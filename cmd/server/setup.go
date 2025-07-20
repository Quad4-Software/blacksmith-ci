// Copyright 2022 Woodpecker Authors
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
	"encoding/base32"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"codeberg.org/crowci/crow/v3/server"
	"codeberg.org/crowci/crow/v3/server/cache"
	"codeberg.org/crowci/crow/v3/server/forge/setup"
	"codeberg.org/crowci/crow/v3/server/logging"
	"codeberg.org/crowci/crow/v3/server/model"
	"codeberg.org/crowci/crow/v3/server/pubsub"
	"codeberg.org/crowci/crow/v3/server/queue"
	"codeberg.org/crowci/crow/v3/server/services"
	"codeberg.org/crowci/crow/v3/server/services/lock"
	logService "codeberg.org/crowci/crow/v3/server/services/log"
	"codeberg.org/crowci/crow/v3/server/services/log/file"
	"codeberg.org/crowci/crow/v3/server/services/maintenance"
	"codeberg.org/crowci/crow/v3/server/services/permissions"
	"codeberg.org/crowci/crow/v3/server/store"
	"codeberg.org/crowci/crow/v3/server/store/datastore"
	"codeberg.org/crowci/crow/v3/server/store/types"
)

const (
	queueInfoRefreshInterval = 500 * time.Millisecond
	storeInfoRefreshInterval = 10 * time.Second
)

func setupStore(ctx context.Context, c *cli.Command) (store.Store, error) {
	datasource := c.String("db-datasource")
	driver := c.String("db-driver")
	xorm := store.XORM{
		Log:             c.Bool("db-log"),
		ShowSQL:         c.Bool("db-log-sql"),
		MaxOpenConns:    int(c.Int64("db-max-open-connections")),
		MaxIdleConns:    int(c.Int64("db-max-idle-connections")),
		ConnMaxLifetime: c.Duration("db-max-connection-timeout"),
	}

	if driver == "sqlite3" {
		if datastore.SupportedDriver("sqlite3") {
			log.Debug().Msg("server has sqlite3 support")
		} else {
			log.Debug().Msg("server was built without sqlite3 support!")
		}
	}

	if !datastore.SupportedDriver(driver) {
		return nil, fmt.Errorf("database driver '%s' not supported", driver)
	}

	if driver == "sqlite3" {
		if err := checkSqliteFileExist(datasource); err != nil {
			return nil, fmt.Errorf("check sqlite file: %w", err)
		}
	}

	opts := &store.Opts{
		Driver: driver,
		Config: datasource,
		XORM:   xorm,
	}
	log.Debug().Str("driver", driver).Any("xorm", xorm).Msg("setting up datastore")
	store, err := datastore.NewEngine(opts)
	if err != nil {
		return nil, fmt.Errorf("could not open datastore: %w", err)
	}

	if err = store.Ping(); err != nil {
		return nil, err
	}

	if err := store.Migrate(ctx, c.Bool("migrations-allow-long")); err != nil {
		return nil, fmt.Errorf("could not migrate datastore: %w", err)
	}

	return store, nil
}

func checkSqliteFileExist(path string) error {
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		log.Warn().Msgf("no sqlite3 file found, will create one at '%s'", path)
		return nil
	}
	return err
}

func setupQueue(ctx context.Context, s store.Store, lockService *lock.Service, c *cli.Command) (queue.Queue, error) {
	// Use database queue for HA deployments, memory queue for single instance
	backend := queue.TypeDatabase
	if server.Config.Server.InstanceID == "" {
		// Default to memory queue if no instance ID is configured
		backend = queue.TypeMemory
	}

	config := queue.Config{
		Backend:       backend,
		Store:         s,
		InstanceID:    server.Config.Server.InstanceID,
		LeaderLockTTL: c.Duration("queue-lock-ttl"),
	}

	if backend == queue.TypeDatabase {
		config.LockService = lockService
	}

	q, err := queue.New(ctx, config)
	if err != nil {
		return nil, err
	}

	// Clean up orphaned workflows on startup
	go func() {
		// Wait a bit for the server to fully start
		time.Sleep(5 * time.Second)

		// Use reflection to check if this is a fifo queue and get running workflow IDs
		if getter, ok := q.(interface{ GetRunningWorkflowIDs() []string }); ok {
			runningIDs := getter.GetRunningWorkflowIDs()
			if len(runningIDs) > 0 {
				log.Info().Strs("workflow_ids", runningIDs).Msg("checking for orphaned workflows on startup")
				if err := s.CleanupOrphanedWorkflows(runningIDs); err != nil {
					log.Error().Err(err).Msg("failed to cleanup orphaned workflows")
				}
			}
		}
	}()

	return q, nil
}

func setupMembershipService(_ context.Context, _store store.Store) cache.MembershipService {
	// Use distributed cache for HA deployments, memory cache for single instance
	backend := cache.TypeMemory
	if server.Config.Server.InstanceID != "" {
		backend = cache.TypeDistributed
	}

	config := cache.Config{
		Backend: backend,
		Store:   _store,
	}

	return cache.NewMembershipServiceWithConfig(config)
}

func setupPubSub(s store.Store, instanceID string) pubsub.Publisher {
	// Use distributed pub/sub for HA deployments, memory for single instance
	backend := pubsub.TypeDistributed
	if instanceID == "" {
		// Default to memory pub/sub if no instance ID is configured
		backend = pubsub.TypeMemory
	}

	return pubsub.NewPublisher(pubsub.Config{
		Backend:    backend,
		Store:      s,
		InstanceID: instanceID,
	})
}

func setupLogStore(c *cli.Command, s store.Store) (logService.Service, error) {
	switch c.String("log-store") {
	case "file":
		return file.NewLogStore(c.String("log-store-file-path"))
	default:
		return s, nil
	}
}

const jwtSecretID = "jwt-secret"

func setupJWTSecret(_store store.Store) (string, error) {
	jwtSecret, err := _store.ServerConfigGet(jwtSecretID)
	if errors.Is(err, types.RecordNotExist) {
		jwtSecret := base32.StdEncoding.EncodeToString(
			securecookie.GenerateRandomKey(32),
		)
		err = _store.ServerConfigSet(jwtSecretID, jwtSecret)
		if err != nil {
			return "", err
		}
		log.Debug().Msg("created jwt secret")
		return jwtSecret, nil
	}

	if err != nil {
		return "", err
	}

	return jwtSecret, nil
}

func setupEvilGlobals(ctx context.Context, c *cli.Command, s store.Store) (err error) {
	// services
	server.Config.Services.Logs = logging.New()
	server.Config.Services.Membership = setupMembershipService(ctx, s)
	// Setup lock service for distributed coordination
	lockService := lock.NewService(s, server.Config.Server.InstanceID)

	// Setup pub/sub system - use distributed for HA deployments
	server.Config.Services.Pubsub = setupPubSub(s, server.Config.Server.InstanceID)

	server.Config.Services.Queue, err = setupQueue(ctx, s, lockService, c)
	if err != nil {
		return fmt.Errorf("could not setup queue: %w", err)
	}
	server.Config.Services.Manager, err = services.NewManager(c, s, setup.Forge)
	if err != nil {
		return fmt.Errorf("could not setup service manager: %w", err)
	}
	server.Config.Services.LogStore, err = setupLogStore(c, s)
	if err != nil {
		return fmt.Errorf("could not setup log store: %w", err)
	}

	// maintenance service
	server.Config.Services.Store = s
	server.Config.Services.MaintenanceService = maintenance.NewService(s)
	server.Config.Services.MaintenanceScheduler = maintenance.NewScheduler(server.Config.Services.MaintenanceService, s)
	if err := server.Config.Services.MaintenanceScheduler.Start(); err != nil {
		log.Error().Err(err).Msg("failed to start maintenance scheduler")
	}

	// agents
	server.Config.Agent.DisableUserRegisteredAgentRegistration = c.Bool("disable-user-agent-registration")

	// authentication
	server.Config.Pipeline.AuthenticatePublicRepos = c.Bool("authenticate-public-repos")

	// Pull requests
	server.Config.Pipeline.DefaultAllowPullRequests = c.Bool("default-allow-pull-requests")

	// Cloning
	server.Config.Pipeline.DefaultClonePlugin = c.String("default-clone-plugin")
	server.Config.Pipeline.TrustedClonePlugins = c.StringSlice("plugins-trusted-clone")
	server.Config.Pipeline.TrustedClonePlugins = append(server.Config.Pipeline.TrustedClonePlugins, server.Config.Pipeline.DefaultClonePlugin)

	// Logs
	server.Config.Pipeline.LogsDurationKeep = c.String("logs-duration-keep")
	server.Config.Pipeline.LogsPipelinesKeepMin = c.Int64("logs-pipelines-keep-min")

	// Execution
	_events := c.StringSlice("default-cancel-previous-pipeline-events")
	events := make([]model.WebhookEvent, 0, len(_events))
	for _, v := range _events {
		events = append(events, model.WebhookEvent(v))
	}
	server.Config.Pipeline.DefaultCancelPreviousPipelineEvents = events
	server.Config.Pipeline.DefaultTimeout = c.Int64("default-pipeline-timeout")
	server.Config.Pipeline.MaxTimeout = c.Int64("max-pipeline-timeout")

	_labels := c.StringSlice("default-workflow-labels")
	labels := make(map[string]string, len(_labels))
	for _, v := range _labels {
		name, value, ok := strings.Cut(v, "=")
		if !ok {
			return fmt.Errorf("invalid label filter: %s", v)
		}
		labels[name] = value
	}
	server.Config.Pipeline.DefaultWorkflowLabels = labels

	// backend options for pipeline compiler
	server.Config.Pipeline.Proxy.No = c.String("backend-no-proxy")
	server.Config.Pipeline.Proxy.HTTP = c.String("backend-http-proxy")
	server.Config.Pipeline.Proxy.HTTPS = c.String("backend-https-proxy")
	server.Config.Pipeline.ContainerNameScheme = c.String("container-name-scheme")

	// server configuration
	server.Config.Server.JWTSecret, err = setupJWTSecret(s)
	if err != nil {
		return fmt.Errorf("could not setup jwt secret: %w", err)
	}
	server.Config.Server.Cert = c.String("server-cert")
	server.Config.Server.Key = c.String("server-key")
	server.Config.Server.AgentToken = c.String("agent-secret")
	serverHost := strings.TrimSuffix(c.String("server-host"), "/")
	server.Config.Server.Host = serverHost
	if c.IsSet("server-webhook-host") {
		server.Config.Server.WebhookHost = c.String("server-webhook-host")
	} else {
		server.Config.Server.WebhookHost = serverHost
	}
	server.Config.Server.OAuthHost = serverHost
	server.Config.Server.Port = c.String("server-addr")
	server.Config.Server.PortTLS = c.String("server-addr-tls")
	server.Config.Server.StatusContext = c.String("status-context")
	server.Config.Server.StatusContextFormat = c.String("status-context-format")
	server.Config.Server.SessionExpires = c.Duration("session-expires")
	u, _ := url.Parse(server.Config.Server.Host)
	rootPath := strings.TrimSuffix(u.Path, "/")

	// Set instance ID for HA deployments - defaults to hostname if not provided
	server.Config.Server.InstanceID = c.String("instance-id")
	if server.Config.Server.InstanceID == "" {
		if hostname, err := os.Hostname(); err == nil {
			server.Config.Server.InstanceID = hostname
			log.Info().Str("instanceID", server.Config.Server.InstanceID).Msg("Using hostname as instance ID for HA deployment")
		} else {
			server.Config.Server.InstanceID = "default"
			log.Warn().Err(err).Str("instanceID", server.Config.Server.InstanceID).Msg("Failed to get hostname, using default instance ID")
		}
	} else {
		log.Info().Str("instanceID", server.Config.Server.InstanceID).Msg("Using configured instance ID for HA deployment")
	}
	if rootPath != "" && !strings.HasPrefix(rootPath, "/") {
		rootPath = "/" + rootPath
	}
	server.Config.Server.RootPath = rootPath
	server.Config.Server.CustomCSSFile = strings.TrimSpace(c.String("custom-css-file"))
	server.Config.Server.CustomJsFile = strings.TrimSpace(c.String("custom-js-file"))
	server.Config.Pipeline.Networks = c.StringSlice("network")
	server.Config.Pipeline.Volumes = c.StringSlice("volume")
	server.Config.WebUI.EnableSwagger = c.Bool("enable-swagger")
	server.Config.WebUI.SkipVersionCheck = c.Bool("skip-version-check")
	server.Config.Pipeline.PrivilegedPlugins = c.StringSlice("plugins-privileged")
	server.Config.Pipeline.PrivilegedPluginsMatchType = c.String("plugins-privileged-match-type")

	// prometheus
	server.Config.Prometheus.AuthToken = c.String("prometheus-auth-token")

	// permissions
	server.Config.Permissions.Open = c.Bool("open")
	server.Config.Permissions.Admins = permissions.NewAdmins(c.StringSlice("admin"))
	server.Config.Permissions.Orgs = permissions.NewOrgs(c.StringSlice("orgs"))
	server.Config.Permissions.OwnersAllowlist = permissions.NewOwnersAllowlist(c.StringSlice("repo-owners"))
	return nil
}
