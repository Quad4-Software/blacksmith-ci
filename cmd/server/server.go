// Copyright 2025 Crow authors
// Copyright 2018 Drone.IO Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v7"
	"github.com/gin-gonic/gin"
	prometheus_http "github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/bootstrap"
	"codefloe.com/crowci/crow/v6/server/cron"
	"codefloe.com/crowci/crow/v6/server/router"
	"codefloe.com/crowci/crow/v6/server/router/middleware"
	"codefloe.com/crowci/crow/v6/server/services/lock"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/web"
	"codefloe.com/crowci/crow/v6/shared/logger"
	"codefloe.com/crowci/crow/v6/version"
)

const (
	shutdownTimeout = time.Second * 5
	// How often expired build caches are removed.
	buildCacheCleanupInterval = 15 * time.Minute
)

var (
	stopServerFunc     context.CancelCauseFunc = func(error) {}
	shutdownCancelFunc context.CancelFunc      = func() {}
	shutdownCtx                                = context.Background()
)

func run(ctx context.Context, c *cli.Command) error {
	if err := logger.SetupGlobalLogger(ctx, c, true); err != nil {
		return err
	}

	ctx, ctxCancel := context.WithCancelCause(ctx)
	stopServerFunc = func(err error) {
		if err != nil {
			log.Error().Err(err).Msg("shutdown of whole server")
		}
		stopServerFunc = func(error) {}
		shutdownCtx, shutdownCancelFunc = context.WithTimeout(shutdownCtx, shutdownTimeout)
		ctxCancel(err)
	}
	defer stopServerFunc(nil)
	defer shutdownCancelFunc()

	// set gin mode based on log level
	// GIN debug output ([GIN-debug]) only shows at trace level
	if zerolog.GlobalLevel() > zerolog.TraceLevel {
		gin.SetMode(gin.ReleaseMode)
	}

	if c.String("server-host") == "" {
		return fmt.Errorf("CROW_HOST is not properly configured")
	}

	if !strings.Contains(c.String("server-host"), "://") {
		return fmt.Errorf("CROW_HOST must be <scheme>://<hostname> format")
	}

	if _, err := url.Parse(c.String("server-host")); err != nil {
		return fmt.Errorf("could not parse CROW_HOST: %w", err)
	}

	if strings.Contains(c.String("server-host"), "://localhost") {
		log.Warn().Msg(
			"CROW_HOST should probably be publicly accessible (not localhost)",
		)
	}

	_store, err := backoff.Retry(ctx,
		func() (store.Store, error) {
			return setupStore(ctx, c)
		},
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithMaxTries(uint(c.Uint64("db-max-retries"))),
		backoff.WithNotify(func(err error, delay time.Duration) {
			log.Error().Msgf("failed to setup store: '%v'. Retry in %v", err, delay)
		}))
	if err != nil {
		return err
	}

	defer func() {
		if err := _store.Close(); err != nil {
			log.Error().Err(err).Msg("could not close store")
		}
	}()

	err = setupEvilGlobals(ctx, c, _store)
	if err != nil {
		return fmt.Errorf("can't setup globals: %w", err)
	}

	// setupEvilGlobals built the service manager which owns the encryption
	// wrapper around the store. From this point on every consumer (gRPC, cron,
	// metrics) MUST use the wrapped store; otherwise reads of users / user
	// forges return raw ciphertext and that ciphertext ends up being sent to
	// forges as a Bearer token (crowci/crow#974).
	_store = server.Config.Services.Manager.Store()

	if err := bootstrap.AdminToken(_store, c.String("admin-token")); err != nil {
		return fmt.Errorf("admin token bootstrap failed: %w", err)
	}

	// wait for all services until one do stops with an error
	serviceWaitingGroup := errgroup.Group{}

	log.Info().Msgf("starting Crow server with version '%s'", version.String())

	serviceWaitingGroup.Go(func() error {
		log.Info().Msg("starting cron service ...")
		// Use the same lock service for cron leader election
		lockService := lock.NewService(_store, server.Config.Server.InstanceID)
		if err := cron.Run(ctx, _store, lockService, c.Duration("cron-lock-ttl")); err != nil {
			go stopServerFunc(err)
			return err
		}
		log.Info().Msg("cron service stopped")
		return nil
	})

	// approval gate timeout sweeper — transitions workflows whose approval
	// deadline has passed to ApprovalExpired. Runs in every server instance
	// (the underlying UPDATE is idempotent and the per-row state check
	// short-circuits double-processing).
	serviceWaitingGroup.Go(func() error {
		log.Info().Msg("starting approval gate sweeper ...")
		cron.RunSweeper(ctx, _store, server.Config.Services.Queue, server.Config.Pipeline.ApprovalGateSweepInterval)
		log.Info().Msg("approval gate sweeper stopped")
		return nil
	})

	// build cache cleanup: expires unused entries and abandoned uploads and
	// enforces the total quota. Runs in every server instance; deleting an
	// entry twice is harmless.
	if cacheService := server.Config.Services.BuildCache; cacheService != nil {
		serviceWaitingGroup.Go(func() error {
			log.Info().Msg("starting build cache cleanup ...")
			cacheService.RunCleanup(ctx, buildCacheCleanupInterval)
			log.Info().Msg("build cache cleanup stopped")
			return nil
		})
	}

	// start maintenance scheduler shutdown watcher
	serviceWaitingGroup.Go(func() error {
		<-ctx.Done()
		if server.Config.Services.MaintenanceScheduler != nil {
			log.Info().Msg("stopping maintenance scheduler ...")
			server.Config.Services.MaintenanceScheduler.Stop()
			log.Info().Msg("maintenance scheduler stopped")
		}
		return nil
	})

	// start the proclog janitor
	if server.Config.Services.ProcLogJanitor != nil {
		serviceWaitingGroup.Go(func() error {
			log.Info().Msg("starting proclog janitor ...")
			if err := server.Config.Services.ProcLogJanitor.Run(ctx); err != nil {
				go stopServerFunc(err)
				return err
			}
			log.Info().Msg("proclog janitor stopped")
			return nil
		})
	}

	// start the grpc server
	serviceWaitingGroup.Go(func() error {
		log.Info().Msg("starting grpc server ...")
		if err := runGrpcServer(ctx, c, _store); err != nil {
			// stop whole server as grpc is essential
			go stopServerFunc(err)
			return err
		}
		return nil
	})

	proxyWebUI := c.String("www-proxy")
	var webUIServe func(w http.ResponseWriter, r *http.Request)

	if proxyWebUI == "" {
		webEngine, err := web.New()
		if err != nil {
			log.Error().Err(err).Msg("failed to create web engine")
			return err
		}
		webUIServe = webEngine.ServeHTTP
	} else {
		origin, _ := url.Parse(proxyWebUI)

		rewrite := func(req *httputil.ProxyRequest) {
			req.SetURL(origin)
			req.Out.Header.Add("X-Forwarded-Host", req.In.Host)
			req.Out.Header.Add("X-Origin-Host", origin.Host)
			req.SetXForwarded()
		}

		proxy := &httputil.ReverseProxy{Rewrite: rewrite}
		webUIServe = proxy.ServeHTTP
	}

	// setup the server and start the listener
	// Use the Manager's store which is the encrypted store when encryption is configured.
	// This ensures all API endpoints that use store.FromContext() get the encrypted store.
	handler := router.Load(
		webUIServe,
		middleware.Logger(time.RFC3339, true),
		middleware.Version,
		middleware.Store(server.Config.Services.Manager.Store()),
	)

	if c.String("server-cert") != "" {
		// start the server with tls enabled
		serviceWaitingGroup.Go(func() error {
			tlsServer := &http.Server{
				Addr:    server.Config.Server.PortTLS,
				Handler: handler,
				TLSConfig: &tls.Config{
					NextProtos: []string{"h2", "http/1.1"},
				},
			}

			go func() {
				<-ctx.Done()
				log.Info().Msg("shutdown tls server ...")
				if err := tlsServer.Shutdown(shutdownCtx); err != nil { //nolint:contextcheck
					log.Error().Err(err).Msg("shutdown tls server failed")
				} else {
					log.Info().Msg("tls server stopped")
				}
			}()

			log.Info().Msg("starting tls server ...")
			err := tlsServer.ListenAndServeTLS(
				c.String("server-cert"),
				c.String("server-key"),
			)
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error().Err(err).Msg("TLS server failed")
				stopServerFunc(fmt.Errorf("TLS server failed: %w", err))
			}
			return err
		})

		// http to https redirect
		redirect := func(w http.ResponseWriter, req *http.Request) {
			serverURL, _ := url.Parse(server.Config.Server.Host)
			req.URL.Scheme = "https"
			req.URL.Host = serverURL.Host

			w.Header().Set("Strict-Transport-Security", "max-age=31536000")

			http.Redirect(w, req, req.URL.String(), http.StatusMovedPermanently)
		}

		serviceWaitingGroup.Go(func() error {
			redirectServer := &http.Server{
				Addr:    server.Config.Server.Port,
				Handler: http.HandlerFunc(redirect),
			}
			go func() {
				<-ctx.Done()
				log.Info().Msg("shutdown redirect server ...")
				if err := redirectServer.Shutdown(shutdownCtx); err != nil { //nolint:contextcheck
					log.Error().Err(err).Msg("shutdown redirect server failed")
				} else {
					log.Info().Msg("redirect server stopped")
				}
			}()

			log.Info().Msg("starting redirect server ...")
			if err := redirectServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error().Err(err).Msg("redirect server failed")
				stopServerFunc(fmt.Errorf("redirect server failed: %w", err))
			}
			return nil
		})
	} else {
		// start the server without tls
		serviceWaitingGroup.Go(func() error {
			httpServer := &http.Server{
				Addr:    c.String("server-addr"),
				Handler: handler,
			}

			go func() {
				<-ctx.Done()
				log.Info().Msg("shutdown http server ...")
				if err := httpServer.Shutdown(shutdownCtx); err != nil { //nolint:contextcheck
					log.Error().Err(err).Msg("shutdown http server failed")
				} else {
					log.Info().Msg("http server stopped")
				}
			}()

			log.Info().Msg("starting http server ...")
			if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error().Err(err).Msg("http server failed")
				stopServerFunc(fmt.Errorf("http server failed: %w", err))
			}
			return err
		})
	}

	if metricsServerAddr := c.String("metrics-server-addr"); metricsServerAddr != "" {
		startMetricsCollector(ctx, _store)

		serviceWaitingGroup.Go(func() error {
			metricsRouter := gin.New()
			metricsRouter.GET("/metrics", gin.WrapH(prometheus_http.Handler()))

			metricsServer := &http.Server{
				Addr:    metricsServerAddr,
				Handler: metricsRouter,
			}

			go func() {
				<-ctx.Done()
				log.Info().Msg("shutdown metrics server ...")
				if err := metricsServer.Shutdown(shutdownCtx); err != nil { //nolint:contextcheck
					log.Error().Err(err).Msg("shutdown metrics server failed")
				} else {
					log.Info().Msg("metrics server stopped")
				}
			}()

			log.Info().Msg("starting metrics server ...")
			if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error().Err(err).Msg("metrics server failed")
				stopServerFunc(fmt.Errorf("metrics server failed: %w", err))
			}
			return err
		})
	}

	return serviceWaitingGroup.Wait()
}
