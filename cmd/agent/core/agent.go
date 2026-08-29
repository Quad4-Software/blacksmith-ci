// Copyright 2023 Woodpecker Authors
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

package core

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	grpc_credentials "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	"codefloe.com/crowci/crow/v6/agent"
	"codefloe.com/crowci/crow/v6/agent/proclog"
	agent_rpc "codefloe.com/crowci/crow/v6/agent/rpc"
	"codefloe.com/crowci/crow/v6/pipeline"
	"codefloe.com/crowci/crow/v6/pipeline/backend"
	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
	"codefloe.com/crowci/crow/v6/pipeline/rpc"
	"codefloe.com/crowci/crow/v6/pipeline/rpc/proto"
	"codefloe.com/crowci/crow/v6/shared/logger"
	"codefloe.com/crowci/crow/v6/shared/utils"
	"codefloe.com/crowci/crow/v6/version"
)

// runWithRecover wraps a goroutine body in a deferred recover() so a panic in
// one goroutine doesn't take the process down uncontrolled. The recovered panic
// is converted into an error, logged with its stack trace, and triggers a
// graceful shutdown of the whole agent.
//
// Shutting down is deliberate: a panicking goroutine is gone for good and its
// invariants are broken, so the agent cannot be trusted to keep working. The
// workflow-runner loop treats ordinary errors as transient and retries them, so
// without this an agent whose only runner panicked would keep answering /healthz
// with 200 while never picking up work again — the zombie agent of issue #1396.
// Exiting lets the supervisor restart the agent with clean state.
func runWithRecover(name string, fn func() error) func() error {
	return func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				log.Error().
					Str("goroutine", name).
					Interface("recovered", r).
					Bytes("stack", stack).
					Msg("recover triggered in agent goroutine")
				err = fmt.Errorf("panic in %s: %v", name, r)
				stopAgentFunc(err)
			}
		}()
		return fn()
	}
}

// runnerRetryDelay is the cooldown between non-fatal runner errors, to avoid a
// hot loop when the runner repeatedly returns transient errors.
const runnerRetryDelay = 5 * time.Second

const (
	reportHealthInterval           = time.Second * 10
	authInterceptorRefreshInterval = time.Minute * 30
)

const (
	microsecondsPerCore = 100000
	shutdownTimeout     = time.Second * 5
)

// forceExitTimeout is the maximum time to wait for graceful shutdown before forcing exit.
// This ensures the agent process terminates even if some goroutines are stuck,
// which is critical for container orchestration (Docker, Kubernetes) to detect the exit and restart.
// A variable rather than a constant so tests that run a whole agent in-process
// can push the watchdog out of the way instead of having it exit the test binary.
var forceExitTimeout = time.Second * 15

var (
	stopAgentFunc      context.CancelCauseFunc = func(error) {}
	shutdownCancelFunc context.CancelFunc      = func() {}
	shutdownCtx                                = context.Background()
)

// validateGRPCConfig fails fast on common misconfigurations of the agent's
// gRPC settings. Operators frequently set CROW_GRPC_ADDR by following docs
// for the server or autoscaler; the agent does not read it, so a silent
// fallback to a half-configured CROW_SERVER produces confusing dial errors
// further down. Returning an explicit error here points operators at the
// correct variable before any gRPC client is constructed.
func validateGRPCConfig(server string, grpcSecure bool) error {
	for _, name := range []string{"CROW_GRPC_ADDR", "WOODPECKER_GRPC_ADDR"} {
		if os.Getenv(name) != "" {
			return fmt.Errorf("%s is a server/autoscaler-only variable and is ignored by the agent; "+
				"set CROW_SERVER=<host>:<port> instead (e.g. CROW_SERVER=grpc.example.com:443), "+
				"plus CROW_GRPC_SECURE=true for TLS endpoints", name)
		}
	}

	if !grpcSecure {
		if _, port, err := net.SplitHostPort(server); err == nil &&
			(port == "443" || port == "8443") {
			log.Warn().Str("server", server).
				Msg("CROW_SERVER targets a TLS-typical port but CROW_GRPC_SECURE is not set; " +
					"the agent will use plaintext and will likely fail with 'reading server preface: EOF'. " +
					"Set CROW_GRPC_SECURE=true if the server is behind a TLS-terminating proxy.")
		}
	}

	return nil
}

func run(ctx context.Context, c *cli.Command, backends []types.Backend) error {
	if err := validateGRPCConfig(c.String("server"), c.Bool("grpc-secure")); err != nil {
		return err
	}

	agentCtx, ctxCancel := context.WithCancelCause(ctx)
	// Guard the body with a Once rather than having stopAgentFunc overwrite
	// itself: several goroutines (health reporting, backend probe, any panicking
	// goroutine wrapped in runWithRecover) may reach it concurrently, and a
	// self-assignment would be a data race on the package variable.
	var stopAgentOnce sync.Once
	stopAgentFunc = func(err error) {
		stopAgentOnce.Do(func() {
			msg := "shutdown of whole agent"
			if err != nil {
				log.Error().Err(err).Msg(msg)
			} else {
				log.Info().Msg(msg)
			}

			// Signal all gRPC retry loops to stop immediately
			agent_rpc.SignalAgentShutdown()

			// Always preserve workflows on agent shutdown so they can be adopted by a restarted agent.
			// This handles all shutdown scenarios: SIGTERM, SIGKILL, server disconnect, pod eviction, etc.
			// Workflows are only cleaned up when they complete naturally (success/failure), not when agent dies.
			pipeline.SetPreserveWorkflows(true)
			log.Info().Msg("agent shutting down, workflows will be preserved for adoption")

			shutdownCtx, shutdownCancelFunc = context.WithTimeout(shutdownCtx, shutdownTimeout)
			ctxCancel(err)

			// Start force exit timer - if graceful shutdown takes too long, force exit.
			// This is critical for container orchestration to detect the exit and restart the agent.
			// Read the timeout here rather than inside the goroutine: the
			// goroutine outlives the agent, and tests that run several agents
			// in one process rewrite the variable between them.
			forceExitAfter := forceExitTimeout
			go func() {
				time.Sleep(forceExitAfter)
				log.Error().Msg("graceful shutdown timed out, forcing exit")
				os.Exit(1)
			}()
		})
	}
	defer stopAgentFunc(nil)
	defer shutdownCancelFunc()

	serviceWaitingGroup := errgroup.Group{}

	agentConfigPath := c.String("agent-config")
	hostname := c.String("hostname")
	if len(hostname) == 0 {
		hostname, _ = os.Hostname()
	}

	maxWorkflowsConfig := c.Int64("max-workflows")
	log.Trace().
		Int64("max-workflows", maxWorkflowsConfig).
		Int64("max-lifetime-workflows", c.Int64("max-lifetime-workflows")).
		Str("CROW_MAX_WORKFLOWS", os.Getenv("CROW_MAX_WORKFLOWS")).
		Str("WOODPECKER_MAX_WORKFLOWS", os.Getenv("WOODPECKER_MAX_WORKFLOWS")).
		Msg("agent configuration")

	// Lifetime budget for single-use agents: once it is used up the agent stops
	// polling, drains what it is running, unregisters and exits 0.
	lifetime := newWorkflowLifetime(int(c.Int64("max-lifetime-workflows")))
	if lifetime.Enabled() {
		log.Info().
			Int("max_lifetime_workflows", lifetime.limit).
			Msg("agent is single-use: it will retire once its workflow budget is used up")

		// An agent that retires after N workflows can never use more than N
		// slots, so cap its parallelism at the budget. Otherwise it registers a
		// capacity it cannot honor and spawns runners that exit on first poll.
		if int64(lifetime.limit) < maxWorkflowsConfig {
			log.Info().
				Int64("max_workflows", maxWorkflowsConfig).
				Int("max_lifetime_workflows", lifetime.limit).
				Msg("capping parallel workflows to the agent's workflow lifetime budget")
			maxWorkflowsConfig = int64(lifetime.limit)
		}
	}

	counter.Polling = int(maxWorkflowsConfig)
	counter.Running = 0

	if c.Bool("healthcheck") {
		serviceWaitingGroup.Go(runWithRecover("healthcheck-server", func() error {
			server := &http.Server{Addr: c.String("healthcheck-addr")}
			go func() {
				<-agentCtx.Done()
				log.Info().Msg("shutdown healthcheck server ...")
				if err := server.Shutdown(shutdownCtx); err != nil { //nolint:contextcheck
					log.Error().Err(err).Msg("shutdown healthcheck server failed")
				} else {
					log.Info().Msg("healthcheck server stopped")
				}
			}()
			if err := server.ListenAndServe(); err != nil {
				log.Error().Err(err).Msgf("cannot listen on address %s", c.String("healthcheck-addr"))
			}
			return nil
		}))
	}

	var transport grpc.DialOption
	if c.Bool("grpc-secure") {
		log.Trace().Msg("use ssl for grpc")
		transport = grpc.WithTransportCredentials(grpc_credentials.NewTLS(&tls.Config{InsecureSkipVerify: c.Bool("grpc-skip-insecure")}))
	} else {
		transport = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	authConn, err := grpc.NewClient(
		c.String("server"),
		transport,
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                c.Duration("grpc-keepalive-time"),
			Timeout:             c.Duration("grpc-keepalive-timeout"),
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return fmt.Errorf("could not create new gRPC 'channel' for authentication: %w", err)
	}
	defer authConn.Close()

	agentConfig := readAgentConfig(agentConfigPath)

	agentToken := c.String("grpc-token")
	grpcClientCtx, grpcClientCtxCancel := context.WithCancelCause(context.Background())
	defer grpcClientCtxCancel(nil)
	authClient := agent_rpc.NewAuthGrpcClient(authConn, agentToken, agentConfig.AgentID, hostname)
	authInterceptor, err := agent_rpc.NewAuthInterceptor(grpcClientCtx, authClient, authInterceptorRefreshInterval) //nolint:contextcheck
	if err != nil {
		return fmt.Errorf("agent could not auth: %w", err)
	}

	conn, err := grpc.NewClient(
		c.String("server"),
		transport,
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                c.Duration("grpc-keepalive-time"),
			Timeout:             c.Duration("grpc-keepalive-timeout"),
			PermitWithoutStream: true,
		}),
		grpc.WithUnaryInterceptor(authInterceptor.Unary()),
		grpc.WithStreamInterceptor(authInterceptor.Stream()),
	)
	if err != nil {
		return fmt.Errorf("could not create new gRPC 'channel' for normal orchestration: %w", err)
	}
	defer conn.Close()

	client := agent_rpc.NewGrpcClient(ctx, conn)
	client.SetMaxErrorDuration(c.Duration("grpc-max-error-duration"))

	if c.Bool("proclog-ship-enabled") {
		sink := proclog.NewSink(c.Int("proclog-local-buffer"))
		ring := proclog.NewRing(c.Int("proclog-local-buffer"))
		grpcProcLogClient := proclog.NewGrpcClient(proto.NewCrowClient(conn))
		shipper := proclog.NewShipper(sink, ring, grpcProcLogClient, proclog.ShipperOpts{
			BatchInterval: c.Duration("proclog-batch-interval"),
			BatchSize:     c.Int("proclog-batch-size"),
			RateLimit:     c.Int("proclog-rate-limit"),
		})
		go shipper.Run(agentCtx)
		logger.SetGlobalExtraWriter(sink)
	}

	agentConfigPersisted := atomic.Bool{}

	grpcCtx := metadata.NewOutgoingContext(grpcClientCtx, metadata.Pairs("hostname", hostname))

	// check if grpc server version is compatible with agent
	grpcServerVersion, err := client.Version(grpcCtx) //nolint:contextcheck
	if err != nil {
		log.Error().Err(err).Msg("could not get grpc server version")
		return err
	}
	if grpcServerVersion.GrpcVersion != agent_rpc.ClientGrpcVersion {
		err := errors.New("GRPC version mismatch")
		log.Error().Err(err).Msgf("server version %s does report grpc version %d but we only understand %d",
			grpcServerVersion.ServerVersion,
			grpcServerVersion.GrpcVersion,
			agent_rpc.ClientGrpcVersion)
		return err
	}

	// new engine
	backendCtx := context.WithValue(agentCtx, types.CliCommand, c)
	backendName := c.String("backend-engine")
	backendEngine, err := backend.FindBackend(backendCtx, backends, backendName)
	if err != nil {
		log.Error().Err(err).Msgf("cannot find backend engine '%s'", backendName)
		return err
	}
	if !backendEngine.IsAvailable(backendCtx) {
		log.Error().Str("engine", backendEngine.Name()).Msg("selected backend engine is unavailable")
		return fmt.Errorf("selected backend engine %s is unavailable", backendEngine.Name())
	}

	// load engine (e.g. init api client)
	engInfo, err := backendEngine.Load(backendCtx)
	if err != nil {
		log.Error().Err(err).Msg("cannot load backend engine")
		return err
	}
	log.Trace().Msgf("loaded %s backend engine", backendEngine.Name())

	maxWorkflows := int(maxWorkflowsConfig)

	customLabels := make(map[string]string)
	if err := stringSliceAddToMap(c.StringSlice("labels"), customLabels); err != nil {
		return err
	}
	if len(customLabels) != 0 {
		log.Debug().Interface("labels", customLabels).Msg("custom labels detected")
	}

	log.Debug().Msg("registering agent with server...")
	var agentName string

	// Read Docker resource limits if using Docker backend
	var limitMem, limitCPUQuota int64
	if backendEngine.Name() == "docker" {
		// Import the docker package's parsing functions
		memStr := c.String("backend-docker-limit-mem")
		cpuStr := c.String("backend-docker-limit-cpu-quota")

		log.Debug().
			Str("mem_str", memStr).
			Str("cpu_str", cpuStr).
			Msg("reading Docker resource limits from flags")

		if memStr != "" {
			limitMem, err = backend.ParseDockerMemory(memStr)
			if err != nil {
				log.Error().Err(err).Str("value", memStr).Msg("failed to parse backend-docker-limit-mem")
				return err
			}
			log.Info().
				Str("input", memStr).
				Str("limit", humanize.IBytes(uint64(limitMem))).
				Msg("Docker memory limit")
		}

		if cpuStr != "" {
			limitCPUQuota, err = backend.ParseDockerCPUQuota(cpuStr)
			if err != nil {
				log.Error().Err(err).Str("value", cpuStr).Msg("failed to parse backend-docker-limit-cpu-quota")
				return err
			}
			log.Info().
				Str("input", cpuStr).
				Str("quota", fmt.Sprintf("%.2f cores", float64(limitCPUQuota)/microsecondsPerCore)).
				Msg("Docker CPU quota")
		}
	}

	agentConfig.AgentID, agentName, err = client.RegisterAgent(grpcCtx, rpc.AgentInfo{ //nolint:contextcheck
		Version:       version.String(),
		BuildDate:     version.Date(),
		Backend:       backendEngine.Name(),
		Platform:      engInfo.Platform,
		Capacity:      maxWorkflows,
		CustomLabels:  customLabels,
		LimitMem:      limitMem,
		LimitCPUQuota: limitCPUQuota,
		HostCPUs:      engInfo.HostCPUs,
		HostMem:       engInfo.HostMem,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to register agent with server")
		return err
	}
	log.Info().
		Int64("agent_id", agentConfig.AgentID).
		Str("agent_name", agentName).
		Interface("custom_labels", customLabels).
		Msg("agent successfully registered with server")

	// Attempt to discover and adopt any orphaned workflows from a previous agent instance.
	// Store these for later - we'll spawn runners after filter is defined.
	adoptedWorkflows := discoverAndAdoptWorkflows(grpcCtx, client, backendEngine) //nolint:contextcheck
	if len(adoptedWorkflows) > 0 {
		log.Info().Int("count", len(adoptedWorkflows)).Msg("adopted orphaned workflows from previous agent instance")
	}

	// Set up re-registration handler for when server restarts and loses agent data
	client.SetReregistrationHandler(func(ctx context.Context) error {
		// A health report that raced the shutdown comes back "agent not found"
		// right after this agent unregistered itself. Re-registering then would
		// leave a fresh row behind for a process that is already exiting.
		if agentCtx.Err() != nil {
			log.Debug().Msg("skipping re-registration, agent is shutting down")
			return errors.New("agent is shutting down")
		}

		log.Info().Msg("re-registering agent with server")
		newAgentID, newAgentName, err := client.RegisterAgent(ctx, rpc.AgentInfo{
			Version:       version.String(),
			BuildDate:     version.Date(),
			Backend:       backendEngine.Name(),
			Platform:      engInfo.Platform,
			Capacity:      maxWorkflows,
			CustomLabels:  customLabels,
			LimitMem:      limitMem,
			LimitCPUQuota: limitCPUQuota,
			HostCPUs:      engInfo.HostCPUs,
			HostMem:       engInfo.HostMem,
		})
		if err != nil {
			log.Error().Err(err).Msg("failed to re-register agent")
			return err
		}

		// Update agent config with new ID
		agentConfig.AgentID = newAgentID

		// Update auth client with new agent ID so authentication works
		authClient.UpdateAgentID(newAgentID)

		// Persist new agent ID if we have a config path
		if agentConfigPath != "" {
			if err := writeAgentConfig(agentConfig, agentConfigPath); err != nil {
				log.Error().Err(err).Msg("failed to persist new agent ID after re-registration")
				// Don't fail re-registration if we can't persist - agent can continue with new ID
			} else {
				agentConfigPersisted.Store(true)
			}
		}

		log.Info().
			Int64("agent_id", newAgentID).
			Str("agent_name", newAgentName).
			Msg("agent successfully re-registered with server")
		return nil
	})

	serviceWaitingGroup.Go(runWithRecover("agent-unregister", func() error {
		// we close grpc client context once unregister was handled
		defer grpcClientCtxCancel(nil)
		// we wait till agent context is done
		<-agentCtx.Done()
		// Remove stateless agents from server. A retired single-use agent also
		// unregisters even though it persisted its ID: the machine it runs on
		// is about to be discarded, so there is no later start to reuse it.
		if !agentConfigPersisted.Load() || counter.Retiring() {
			log.Debug().Msg("unregister agent from server ...")
			// Bound the call: it has no retry of its own, and a connection stuck
			// in CONNECTING would otherwise hold the shutdown open until the
			// force-exit watchdog fires and turns a clean retirement into exit 1.
			unregisterCtx, cancelUnregister := context.WithTimeout(grpcClientCtx, shutdownTimeout)
			defer cancelUnregister()
			// we want to run it explicit run when context got canceled so run it in background
			err := client.UnregisterAgent(unregisterCtx)
			if err != nil {
				log.Err(err).Msg("failed to unregister agent from server")
			} else {
				log.Info().Msg("agent unregistered from server")
			}
		}
		return nil
	}))

	if agentConfigPath != "" {
		if err := writeAgentConfig(agentConfig, agentConfigPath); err == nil {
			agentConfigPersisted.Store(true)
		}
	}

	// set default labels ...
	labels := make(map[string]string)
	labels[pipeline.LabelFilterHostname] = hostname
	labels[pipeline.LabelFilterPlatform] = engInfo.Platform
	labels[pipeline.LabelFilterBackend] = backendEngine.Name()
	labels[pipeline.LabelFilterAgent] = agentName // Use the agent name from server registration
	labels[pipeline.LabelFilterRepo] = "*"        // allow all repos by default
	// ... and let it overwrite by custom ones
	maps.Copy(labels, customLabels)

	log.Trace().Any("labels", labels).Msgf("agent configured with labels")

	filter := rpc.Filter{
		Labels: labels,
	}

	log.Trace().Msgf("agent registered with ID %d", agentConfig.AgentID)

	// Runners for adopted workflows are spawned together with the polling pool
	// below (after the capacity check) so that adopted workflows occupy capacity
	// slots instead of running on top of a full fresh pool.

	// Docker cleanup config shared between health reporting and cleanup goroutines
	var dockerCleanupConfig atomic.Pointer[rpc.DockerCleanupConfig]
	dockerCleanupRunNow := make(chan struct{}, 1) // signal channel for immediate cleanup

	serviceWaitingGroup.Go(runWithRecover("health-reporting", func() error {
		log.Trace().Msg("health reporting goroutine started")
		for {
			cleanupCfg, err := client.ReportHealth(grpcCtx)
			if cleanupCfg != nil {
				dockerCleanupConfig.Store(cleanupCfg)
				// Wake up cleanup goroutine immediately if RunNow is set
				if cleanupCfg.RunNow {
					select {
					case dockerCleanupRunNow <- struct{}{}:
					default: // already signaled
					}
				}
			}
			if err != nil {
				// Threshold reached: server is unreachable beyond what the
				// supervisor can be expected to wait. Exit for restart.
				if errors.Is(err, agent_rpc.ErrAgentRestartRequired) {
					log.Error().Err(err).Msg("health reporting encountered fatal error, agent will exit for restart")
					stopAgentFunc(err) // Trigger agent shutdown
					return err
				}

				// Check if the error is due to context cancellation
				if grpcCtx.Err() != nil || agentCtx.Err() != nil {
					log.Debug().Msg("terminating health reporting due to context cancellation")
					return nil
				}

				// For other errors, just log and continue
				log.Err(err).Msg("failed to report health")
			}

			select {
			case <-agentCtx.Done():
				log.Debug().Msg("terminating health reporting")
				return nil
			case <-time.After(reportHealthInterval):
			}
		}
	}))

	// Start Docker cleanup goroutine if using Docker backend
	if backendEngine.Name() == "docker" {
		serviceWaitingGroup.Go(runWithRecover("docker-cleanup", func() error {
			return runDockerCleanup(agentCtx, grpcCtx, client, &backendEngine, &dockerCleanupConfig, dockerCleanupRunNow)
		}))
	}

	// Start backend liveness probe if the engine supports it. This catches
	// post-startup failures (e.g. host dockerd restart leaving the agent
	// holding a stale socket) by flipping /healthz to 500 so supervisors
	// and humans can react. After backendMaxUnhealthyDuration of sustained
	// failures, the probe returns ErrBackendUnavailable so the agent exits
	// for restart — same pattern used for gRPC's ErrAgentRestartRequired.
	if _, ok := backendEngine.(types.Pinger); ok {
		serviceWaitingGroup.Go(runWithRecover("backend-probe", func() error {
			err := runBackendProbe(agentCtx, backendEngine)
			if err != nil && errors.Is(err, ErrBackendUnavailable) {
				log.Error().Err(err).Msg("backend probe encountered fatal error, agent will exit for restart")
				stopAgentFunc(err)
			}
			return err
		}))
	}

	if maxWorkflows == 0 {
		log.Error().Msg("agent is configured with max-workflows=0, no pipelines will be processed!")
		log.Error().Msg("set CROW_MAX_WORKFLOWS or WOODPECKER_MAX_WORKFLOWS environment variable to at least 1")
		return fmt.Errorf("max-workflows is 0, agent cannot process any pipelines")
	}

	// Mark workflow polling as healthy once runners start - the Next() call blocks until a task
	// is available, so we can't wait for it to return before setting the flag
	client.SetWorkflowPollingHealthy(true)

	plan := planRunners(len(adoptedWorkflows), maxWorkflows)
	log.Debug().
		Int("adopted", len(adoptedWorkflows)).
		Int("polling_adopted", plan.pollingAdopted).
		Int("polling_plain", plan.pollingPlain).
		Int("drain_adopted", plan.drainAdopted).
		Int("capacity", maxWorkflows).
		Msg("planned workflow runners")

	// workflowRunners tracks every runner goroutine, polling and drain-only
	// alike, so the lifetime supervisor below knows when the agent has finished
	// its last workflow.
	var workflowRunners sync.WaitGroup

	// Long-lived polling pool. Slots [0, pollingAdopted) first resume an adopted
	// workflow, then fall into the normal poll loop; the remaining slots poll
	// immediately. The pool size equals maxWorkflows, so the agent stays within
	// capacity even after reclaiming in-flight workflows on restart.
	pollPool := plan.pollingAdopted + plan.pollingPlain
	for i := range pollPool {
		var adopted *AdoptedWorkflow
		if i < plan.pollingAdopted {
			adopted = adoptedWorkflows[i]
		}
		workflowRunners.Add(1)
		serviceWaitingGroup.Go(runWithRecover(fmt.Sprintf("workflow-runner-%d", i), func() error {
			defer workflowRunners.Done()

			// Track liveness of the polling pool so /healthz can tell an agent
			// that lost all its runners from a healthy idle one.
			counter.RunnerStarted()
			defer counter.RunnerStopped()

			runner := agent.NewRunner(client, filter, hostname, counter, &backendEngine)
			log.Trace().Msgf("created new runner %d", i)

			if adopted != nil {
				// An adopted workflow was claimed before this runner existed
				// and is resumed whatever the budget says, so charge it
				// directly instead of reserving a slot for it. Charging up
				// front, before RunAdopted can fail, is deliberate: adoption
				// only finds workflows whose containers are already on this
				// host, so the machine is tainted by that pipeline whether or
				// not resuming it succeeds, which is exactly what the budget
				// counts.
				lifetime.Charge()
				log.Info().
					Str("workflow_id", adopted.Workflow.ID).
					Str("task_uuid", adopted.TaskUUID).
					Msg("starting runner for adopted workflow")
				if err := runner.RunAdopted(agentCtx, shutdownCtx, adopted.Workflow, adopted.TaskUUID); err != nil {
					log.Error().
						Err(err).
						Str("workflow_id", adopted.Workflow.ID).
						Str("task_uuid", adopted.TaskUUID).
						Msg("adopted workflow runner failed")
				} else {
					log.Info().
						Str("workflow_id", adopted.Workflow.ID).
						Str("task_uuid", adopted.TaskUUID).
						Msg("adopted workflow completed")
				}
			}

			return pollWorkflows(agentCtx, shutdownCtx, i, &runner, lifetime)
		}))
	}

	// Adopted workflows beyond capacity (e.g. capacity was lowered while they
	// were in-flight) get drain-only runners: resume the workflow, then exit
	// without polling, so the agent converges back down to capacity instead of
	// staying above it indefinitely.
	for i := range plan.drainAdopted {
		adopted := adoptedWorkflows[plan.pollingAdopted+i]
		workflowRunners.Add(1)
		serviceWaitingGroup.Go(runWithRecover(fmt.Sprintf("adopted-drain-runner-%s", adopted.Workflow.ID), func() error {
			defer workflowRunners.Done()

			lifetime.Charge()
			runner := agent.NewRunner(client, filter, hostname, counter, &backendEngine)
			log.Info().
				Str("workflow_id", adopted.Workflow.ID).
				Str("task_uuid", adopted.TaskUUID).
				Msg("starting drain-only runner for adopted workflow (over capacity)")
			if err := runner.RunAdopted(agentCtx, shutdownCtx, adopted.Workflow, adopted.TaskUUID); err != nil {
				log.Error().
					Err(err).
					Str("workflow_id", adopted.Workflow.ID).
					Str("task_uuid", adopted.TaskUUID).
					Msg("adopted workflow runner failed")
				return nil
			}
			log.Info().
				Str("workflow_id", adopted.Workflow.ID).
				Str("task_uuid", adopted.TaskUUID).
				Msg("adopted workflow completed (drain-only)")
			return nil
		}))
	}

	// Retire the agent once every runner has stopped. Runners only return on
	// their own when the lifetime budget is used up, so this is the single-use
	// exit path: unregister and exit 0, leaving the supervisor to discard the
	// machine instead of restarting the process on it.
	if lifetime.Enabled() {
		serviceWaitingGroup.Go(runWithRecover("lifetime-supervisor", func() error {
			workflowRunners.Wait()
			if !shouldRetire(agentCtx, lifetime) {
				return nil
			}
			counter.SetRetiring(true)
			log.Info().
				Int("workflows", lifetime.Spent()).
				Int("max_lifetime_workflows", lifetime.limit).
				Msg("agent reached its workflow lifetime budget, retiring")
			stopAgentFunc(nil)
			return nil
		}))
	}

	log.Info().Msgf(
		"starting Crow agent with version '%s' and backend '%s' using platform '%s' running up to %d pipelines in parallel",
		version.String(), backendEngine.Name(), engInfo.Platform, maxWorkflows)

	// Mark agent as healthy now that it has started successfully
	counter.SetHealthy(true)

	// Wait for all services to complete
	err = serviceWaitingGroup.Wait()

	// Every runner has returned, so nothing enqueues log entries any more.
	// Closing the client flushes the batch its log drain is still holding,
	// which is up to a second of the last workflow's output — routine loss for
	// an agent that exits right after finishing a job. rpc.Peer itself has no
	// Close, only the gRPC implementation of it does.
	if closer, ok := client.(interface{ Close() error }); ok {
		if closeErr := closer.Close(); closeErr != nil {
			log.Debug().Err(closeErr).Msg("closing the gRPC client failed")
		}
	}

	// Mark agent as unhealthy when it exits/crashes
	counter.SetHealthy(false)

	return err
}

func runWithRetry(backendEngines []types.Backend) func(ctx context.Context, c *cli.Command) error {
	return func(ctx context.Context, c *cli.Command) error {
		if err := logger.SetupGlobalLogger(ctx, c, true); err != nil {
			return err
		}

		initHealth()

		// Run once - when agent crashes, healthcheck will fail and container will restart
		return run(ctx, c, backendEngines)
	}
}

func stringSliceAddToMap(sl []string, m map[string]string) error {
	if m == nil {
		m = make(map[string]string)
	}
	for _, v := range utils.StringSliceDeleteEmpty(sl) {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			before, after, found := strings.Cut(part, "=")
			switch {
			case before != "" && found:
				if existing, ok := m[before]; ok {
					m[before] = existing + "|" + after
				} else {
					m[before] = after
				}
			case before != "":
				return fmt.Errorf("key '%s' does not have a value assigned", before)
			default:
				return fmt.Errorf("empty string in slice")
			}
		}
	}
	return nil
}

// shouldRetire decides whether an agent whose workflow runners have all
// stopped should retire: unregister and exit 0 as a single-use agent that has
// done its job.
//
// Runners also disappear for reasons that are not retirement. If the agent is
// already shutting down, something else owns the exit. If the budget is not
// used up, a runner panicked and runWithRecover is on its way to shutting the
// agent down with that error — retiring here would race it and report a crash
// as a clean exit, unregistering an agent whose machine should have been
// restarted instead.
func shouldRetire(agentCtx context.Context, lifetime *workflowLifetime) bool {
	if agentCtx.Err() != nil {
		return false
	}
	if !lifetime.Exhausted() {
		log.Debug().
			Int("workflows", lifetime.Spent()).
			Msg("workflow runners stopped without using up the lifetime budget, not retiring")
		return false
	}
	return true
}

// workflowRunner is the slice of agent.Runner that the polling loop uses, so
// the loop can be exercised without a gRPC client or a backend engine.
type workflowRunner interface {
	Run(runnerCtx, shutdownCtx context.Context) (bool, error)
}

// pollWorkflows is the body of a long-lived runner goroutine: pull workflows
// off the queue until the agent shuts down, treating errors as transient
// unless the server has been unreachable long enough to warrant a restart.
//
// It also enforces the agent's workflow lifetime budget. A slot is reserved
// before every poll and released again unless a workflow was actually picked
// up, so an agent that waits an hour for its single job does not spend its
// lifetime on empty queues. Once the budget is gone the runner returns, and
// the last runner to do so takes the whole agent down with it.
func pollWorkflows(agentCtx, shutdownCtx context.Context, id int, runner workflowRunner, lifetime *workflowLifetime) error {
	for {
		if agentCtx.Err() != nil {
			return nil
		}

		// Single-use agents stop polling once the lifetime budget is used up.
		// Reserve blocks while a sibling runner still holds an unspent slot, so
		// the pool keeps its width until the budget is really gone.
		if !lifetime.Reserve(agentCtx) {
			if agentCtx.Err() != nil {
				return nil
			}
			// Latch the wind-down before the last runner disappears, so
			// /healthz never reports the empty runner pool of a retiring agent
			// as the zombie pool of a broken one.
			counter.SetRetiring(true)
			log.Info().Msgf("runner %d stops polling, agent workflow lifetime budget is used up", id)
			return nil
		}

		log.Trace().Msg("polling new steps")
		ranWorkflow, err := runner.Run(agentCtx, shutdownCtx)
		if ranWorkflow {
			lifetime.Spend()
		} else {
			// The poll found an empty queue or failed, so waiting for work
			// never costs the agent part of its lifetime.
			lifetime.Release()
		}
		if err == nil {
			continue
		}

		// Threshold reached: server is unreachable beyond what the
		// supervisor can be expected to wait. Exit for restart.
		if errors.Is(err, agent_rpc.ErrAgentRestartRequired) {
			log.Error().Err(err).Msg("workflow polling encountered fatal error, agent will exit for restart")
			stopAgentFunc(err) // Trigger agent shutdown
			return err
		}

		// Check if context is canceled - this is expected during shutdown
		if agentCtx.Err() != nil {
			return nil
		}

		// Other errors are transient. Log and retry; do not exit the
		// whole agent — that's what ErrAgentRestartRequired is for.
		// A short cooldown avoids a hot loop on persistent errors.
		log.Warn().Err(err).Msgf("runner %d returned a transient error, retrying after %s", id, runnerRetryDelay)
		select {
		case <-agentCtx.Done():
			return nil
		case <-time.After(runnerRetryDelay):
		}
	}
}

// AdoptedWorkflow represents a workflow that has been claimed and needs to be monitored.
type AdoptedWorkflow struct {
	Workflow *rpc.Workflow
	TaskUUID string
}

// runnerPlan describes how many runner goroutines to spawn on startup, given the
// number of workflows adopted from a previous agent instance and the configured
// capacity (maxWorkflows).
type runnerPlan struct {
	// pollingAdopted runners first resume an adopted workflow, then enter the
	// normal poll loop. pollingPlain runners poll immediately. Together they are
	// the long-lived pool and always sum to maxWorkflows, so the agent never
	// pulls new work beyond its capacity — including right after a restart that
	// reclaimed in-flight workflows.
	pollingAdopted int
	pollingPlain   int
	// drainAdopted runners resume an adopted workflow then exit without polling.
	// Only used when more workflows were reclaimed than capacity allows (e.g.
	// capacity was lowered); they let the agent converge back down to capacity
	// instead of staying over it indefinitely.
	drainAdopted int
}

// planRunners decides how to spawn runners so that the long-lived polling pool
// never exceeds maxWorkflows, while still resuming every adopted workflow.
func planRunners(adoptedCount, maxWorkflows int) runnerPlan {
	if adoptedCount < 0 {
		adoptedCount = 0
	}
	if maxWorkflows < 0 {
		maxWorkflows = 0
	}
	pollingAdopted := min(adoptedCount, maxWorkflows)
	return runnerPlan{
		pollingAdopted: pollingAdopted,
		pollingPlain:   maxWorkflows - pollingAdopted,
		drainAdopted:   adoptedCount - pollingAdopted,
	}
}

// ErrBackendUnavailable is returned when the backend engine has been
// unreachable through Ping for longer than backendMaxUnhealthyDuration. The
// agent treats this as fatal so its supervisor (Docker/Kubernetes/systemd)
// restarts the container, which is the only way to recover from situations
// like a stale Docker socket bind mount after a host dockerd upgrade.
var ErrBackendUnavailable = errors.New("backend engine unreachable for too long, restart required")

// Backend liveness probe tuning. The interval is short enough that a stale
// socket is detected within ~minute, the timeout is well under
// reportHealthInterval so a stuck probe never blocks the rest of the agent,
// and the max unhealthy duration is long enough to ride out a normal
// daemon restart (apt upgrade, manual systemctl restart) without giving up
// so quickly that we churn through container restarts during routine
// maintenance.
const (
	backendProbeInterval        = 15 * time.Second
	backendProbeTimeout         = 5 * time.Second
	backendMaxUnhealthyDuration = 3 * time.Minute
)

// backendProbeTracker decides, given a sequence of probe results, whether
// the backend has been unhealthy long enough to warrant a fatal agent
// exit. Extracted from the probe loop so the time-based threshold logic
// can be unit-tested without spinning real timers.
type backendProbeTracker struct {
	maxUnhealthy   time.Duration
	unhealthySince time.Time // zero means currently healthy
}

// record applies a single probe result at the given moment and returns a
// non-nil error iff the backend has been unreachable for longer than the
// configured threshold. The error wraps ErrBackendUnavailable so callers
// can identify the cause via errors.Is.
func (t *backendProbeTracker) record(now time.Time, probeErr error) error {
	if probeErr == nil {
		t.unhealthySince = time.Time{}
		return nil
	}
	if t.unhealthySince.IsZero() {
		t.unhealthySince = now
	}
	if now.Sub(t.unhealthySince) > t.maxUnhealthy {
		return fmt.Errorf("%w (unreachable for %s)", ErrBackendUnavailable, now.Sub(t.unhealthySince).Round(time.Second))
	}
	return nil
}

// runBackendProbe periodically calls Ping on the backend engine, flips the
// package-level health flag consumed by /healthz, and returns a fatal
// error after the engine has been unreachable for longer than
// backendMaxUnhealthyDuration. The probe is started only for backends
// that implement types.Pinger; for backends that do not, the default-true
// health flag means /healthz behaves exactly as before.
func runBackendProbe(ctx context.Context, backendEngine types.Backend) error {
	pinger, ok := backendEngine.(types.Pinger)
	if !ok {
		return nil
	}

	log.Trace().
		Str("backend", backendEngine.Name()).
		Dur("interval", backendProbeInterval).
		Dur("max_unhealthy_duration", backendMaxUnhealthyDuration).
		Msg("backend liveness probe started")

	tracker := backendProbeTracker{maxUnhealthy: backendMaxUnhealthyDuration}

	// Returns a non-nil error only when the threshold was crossed; per-call
	// failures merely flip the /healthz flag and log a warning.
	probe := func() error {
		probeCtx, cancel := context.WithTimeout(ctx, backendProbeTimeout)
		defer cancel()
		probeErr := pinger.Ping(probeCtx)
		// Don't act on probe errors that are really just context cancellation
		// from agent shutdown — those are not backend-unavailability.
		if probeErr != nil && ctx.Err() != nil {
			return nil
		}
		if probeErr != nil {
			SetBackendHealthy(false)
			log.Warn().
				Err(probeErr).
				Str("backend", backendEngine.Name()).
				Msg("backend liveness probe failed; marking agent unhealthy")
		} else {
			SetBackendHealthy(true)
		}
		return tracker.record(time.Now(), probeErr)
	}

	// Run the first probe immediately so a backend that is already broken at
	// agent startup is reflected in /healthz without waiting a full interval.
	if err := probe(); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backendProbeInterval):
			if err := probe(); err != nil {
				return err
			}
		}
	}
}

// runDockerCleanup runs a goroutine that periodically cleans up stale Docker resources.
// It reads config from the atomic pointer (set by the health reporting goroutine) and
// reports results back to the server.
func runDockerCleanup(
	agentCtx context.Context,
	grpcCtx context.Context,
	client rpc.Peer,
	backendEngine *types.Backend,
	configPtr *atomic.Pointer[rpc.DockerCleanupConfig],
	runNowCh <-chan struct{},
) error {
	log.Trace().Msg("Docker cleanup goroutine started, waiting for config from server")

	// Wait for initial config
	var cfg *rpc.DockerCleanupConfig
	for {
		cfg = configPtr.Load()
		if cfg != nil {
			break
		}
		select {
		case <-agentCtx.Done():
			return nil
		case <-time.After(reportHealthInterval):
		}
	}

	if !cfg.Enabled {
		log.Debug().Msg("Docker cleanup is disabled by server config")
	}

	// Check if backend supports cleanup
	cleaner, ok := (*backendEngine).(types.ResourceCleaner)
	if !ok {
		log.Debug().Msg("backend does not support resource cleanup")
		return nil
	}

	interval := time.Duration(cfg.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = time.Hour
	}

	log.Info().
		Bool("enabled", cfg.Enabled).
		Dur("interval", interval).
		Int64("stale_threshold_seconds", cfg.StaleThresholdSeconds).
		Msg("Docker cleanup goroutine configured")

	// If run_now was set in the initial config, run immediately
	if cfg.RunNow && cfg.Enabled {
		executeDockerCleanup(grpcCtx, client, cleaner, cfg)
	}

	for {
		select {
		case <-agentCtx.Done():
			log.Debug().Msg("terminating Docker cleanup goroutine")
			return nil
		case <-time.After(interval):
			// Scheduled tick
		case <-runNowCh:
			// Immediate trigger from server
			log.Info().Msg("Docker cleanup triggered by server run-now request")
		}

		// Re-read config in case it changed
		latestCfg := configPtr.Load()
		if latestCfg != nil {
			cfg = latestCfg
			newInterval := time.Duration(cfg.IntervalSeconds) * time.Second
			if newInterval > 0 {
				interval = newInterval
			}
		}

		if !cfg.Enabled && !cfg.RunNow {
			continue
		}

		executeDockerCleanup(grpcCtx, client, cleaner, cfg)
	}
}

func executeDockerCleanup(
	grpcCtx context.Context,
	client rpc.Peer,
	cleaner types.ResourceCleaner,
	cfg *rpc.DockerCleanupConfig,
) {
	staleThreshold := time.Duration(cfg.StaleThresholdSeconds) * time.Second
	if staleThreshold <= 0 {
		staleThreshold = time.Hour
	}

	start := time.Now()
	log.Info().Dur("stale_threshold", staleThreshold).Msg("starting Docker resource cleanup")

	cleanupCtx, cancel := context.WithTimeout(grpcCtx, staleThreshold)
	defer cancel()
	// Snapshot the workflows currently running on this agent so the reaper never
	// reclaims resources belonging to an in-flight build, even one that has been
	// running longer than the stale threshold.
	result, err := cleaner.CleanupStaleResources(cleanupCtx, staleThreshold, counter.ActiveTaskUUIDs())

	duration := time.Since(start)

	report := rpc.DockerCleanupResult{
		DurationMs: duration.Milliseconds(),
		Success:    err == nil,
		Manual:     cfg.RunNow,
	}

	if result != nil {
		report.ContainersFound = result.ContainersFound
		report.ContainersRemoved = result.ContainersRemoved
		report.NetworksFound = result.NetworksFound
		report.NetworksRemoved = result.NetworksRemoved
		report.VolumesFound = result.VolumesFound
		report.VolumesRemoved = result.VolumesRemoved
	}

	if err != nil {
		report.Error = err.Error()
		log.Error().Err(err).Msg("Docker resource cleanup failed")
	} else {
		log.Info().
			Int64("containers_removed", report.ContainersRemoved).
			Int64("networks_removed", report.NetworksRemoved).
			Int64("volumes_removed", report.VolumesRemoved).
			Dur("duration", duration).
			Msg("Docker resource cleanup completed")
	}

	if reportErr := client.ReportDockerCleanup(grpcCtx, report); reportErr != nil {
		log.Warn().Err(reportErr).Msg("failed to report Docker cleanup results to server")
	}
}

// discoverAndAdoptWorkflows checks if the backend supports workflow discovery,
// and if so, discovers orphaned workflows and attempts to claim them from the server.
// Returns the list of successfully adopted workflows that need to be monitored.
func discoverAndAdoptWorkflows(ctx context.Context, client rpc.Peer, backendEngine types.Backend) []*AdoptedWorkflow {
	// Check if backend supports workflow discovery
	discoverer, ok := backendEngine.(types.WorkflowDiscoverer)
	if !ok {
		log.Debug().
			Str("backend", backendEngine.Name()).
			Msg("backend does not support workflow discovery, skipping adoption")
		return nil
	}

	// Discover orphaned workflows
	workflows, err := discoverer.DiscoverWorkflows(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("failed to discover orphaned workflows")
		return nil
	}

	if len(workflows) == 0 {
		log.Debug().Msg("no orphaned workflows found for adoption")
		return nil
	}

	log.Info().
		Int("count", len(workflows)).
		Msg("discovered orphaned workflows, attempting to adopt")

	var adopted []*AdoptedWorkflow
	for _, workflow := range workflows {
		// Attempt to claim the task from the server
		// This is HA-safe: the server will only return the task if it was previously
		// assigned to this agent (matched by agent_id in the database)
		claimedWorkflow, err := client.ClaimTask(ctx, workflow.TaskUUID)
		if err != nil {
			log.Warn().
				Err(err).
				Str("task_uuid", workflow.TaskUUID).
				Msg("failed to claim orphaned workflow task")
			continue
		}

		if claimedWorkflow == nil {
			log.Debug().
				Str("task_uuid", workflow.TaskUUID).
				Msg("task not claimable (may belong to different agent or already completed)")
			continue
		}

		log.Info().
			Str("task_uuid", workflow.TaskUUID).
			Str("workflow_id", claimedWorkflow.ID).
			Int("steps", len(workflow.Steps)).
			Msg("successfully adopted orphaned workflow")

		adopted = append(adopted, &AdoptedWorkflow{
			Workflow: claimedWorkflow,
			TaskUUID: workflow.TaskUUID,
		})
	}

	return adopted
}
