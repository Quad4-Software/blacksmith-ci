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
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	grpc_credentials "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	"codefloe.com/crowci/crow/v5/agent"
	agent_rpc "codefloe.com/crowci/crow/v5/agent/rpc"
	"codefloe.com/crowci/crow/v5/pipeline"
	"codefloe.com/crowci/crow/v5/pipeline/backend"
	"codefloe.com/crowci/crow/v5/pipeline/backend/types"
	"codefloe.com/crowci/crow/v5/pipeline/rpc"
	"codefloe.com/crowci/crow/v5/shared/logger"
	"codefloe.com/crowci/crow/v5/shared/utils"
	"codefloe.com/crowci/crow/v5/version"
)

const (
	reportHealthInterval           = time.Second * 10
	authInterceptorRefreshInterval = time.Minute * 30
)

const (
	shutdownTimeout = time.Second * 5

	// ForceExitTimeout is the maximum time to wait for graceful shutdown before forcing exit.
	// This ensures the agent process terminates even if some goroutines are stuck,
	// which is critical for container orchestration (Docker, Kubernetes) to detect the exit and restart.
	forceExitTimeout = time.Second * 15
)

var (
	stopAgentFunc      context.CancelCauseFunc = func(error) {}
	shutdownCancelFunc context.CancelFunc      = func() {}
	shutdownCtx                                = context.Background()
)

func run(ctx context.Context, c *cli.Command, backends []types.Backend) error {
	agentCtx, ctxCancel := context.WithCancelCause(ctx)
	stopAgentFunc = func(err error) {
		msg := "shutdown of whole agent"
		if err != nil {
			log.Error().Err(err).Msg(msg)
		} else {
			log.Info().Msg(msg)
		}
		stopAgentFunc = func(error) {}

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
		go func() {
			time.Sleep(forceExitTimeout)
			log.Error().Msg("graceful shutdown timed out, forcing exit")
			os.Exit(1)
		}()
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
		Str("CROW_MAX_WORKFLOWS", os.Getenv("CROW_MAX_WORKFLOWS")).
		Str("WOODPECKER_MAX_WORKFLOWS", os.Getenv("WOODPECKER_MAX_WORKFLOWS")).
		Msg("agent configuration")

	counter.Polling = int(maxWorkflowsConfig)
	counter.Running = 0

	if c.Bool("healthcheck") {
		serviceWaitingGroup.Go(
			func() error {
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
			})
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
	authClient := agent_rpc.NewAuthGrpcClient(authConn, agentToken, agentConfig.AgentID)
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

	maxWorkflows := int(c.Int64("max-workflows"))

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
				Int64("bytes", limitMem).
				Msg("parsed Docker memory limit")
		}

		if cpuStr != "" {
			limitCPUQuota, err = backend.ParseDockerCPUQuota(cpuStr)
			if err != nil {
				log.Error().Err(err).Str("value", cpuStr).Msg("failed to parse backend-docker-limit-cpu-quota")
				return err
			}
			log.Info().
				Str("input", cpuStr).
				Int64("quota", limitCPUQuota).
				Msg("parsed Docker CPU quota")
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

	serviceWaitingGroup.Go(func() error {
		// we close grpc client context once unregister was handled
		defer grpcClientCtxCancel(nil)
		// we wait till agent context is done
		<-agentCtx.Done()
		// Remove stateless agents from server
		if !agentConfigPersisted.Load() {
			log.Debug().Msg("unregister agent from server ...")
			// we want to run it explicit run when context got canceled so run it in background
			err := client.UnregisterAgent(grpcClientCtx)
			if err != nil {
				log.Err(err).Msg("failed to unregister agent from server")
			} else {
				log.Info().Msg("agent unregistered from server")
			}
		}
		return nil
	})

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

	// Spawn runners for adopted workflows (discovered earlier, now that filter is defined)
	for _, adopted := range adoptedWorkflows {
		adopted := adopted // capture loop variable
		serviceWaitingGroup.Go(func() error {
			runner := agent.NewRunner(client, filter, hostname, counter, &backendEngine)
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
				return nil // Don't fail the whole agent for one adopted workflow
			}

			log.Info().
				Str("workflow_id", adopted.Workflow.ID).
				Str("task_uuid", adopted.TaskUUID).
				Msg("adopted workflow completed")
			return nil
		})
	}

	serviceWaitingGroup.Go(func() error {
		log.Trace().Msg("health reporting goroutine started")
		for {
			err := client.ReportHealth(grpcCtx)
			if err != nil {
				// Check for fatal consecutive error threshold FIRST - this must always trigger restart
				if errors.Is(err, agent_rpc.ErrMaxConsecutiveErrors) {
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
	})

	if maxWorkflows == 0 {
		log.Error().Msg("agent is configured with max-workflows=0, no pipelines will be processed!")
		log.Error().Msg("set CROW_MAX_WORKFLOWS or WOODPECKER_MAX_WORKFLOWS environment variable to at least 1")
		return fmt.Errorf("max-workflows is 0, agent cannot process any pipelines")
	}

	// Mark workflow polling as healthy once runners start - the Next() call blocks until a task
	// is available, so we can't wait for it to return before setting the flag
	client.SetWorkflowPollingHealthy(true)

	for i := 0; i < maxWorkflows; i++ {
		i := i
		serviceWaitingGroup.Go(func() error {
			runner := agent.NewRunner(client, filter, hostname, counter, &backendEngine)
			log.Trace().Msgf("created new runner %d", i)

			for {
				if agentCtx.Err() != nil {
					return nil
				}

				log.Trace().Msg("polling new steps")
				if err := runner.Run(agentCtx, shutdownCtx); err != nil {
					// Check for fatal consecutive error threshold FIRST - this must always trigger restart
					if errors.Is(err, agent_rpc.ErrMaxConsecutiveErrors) {
						log.Error().Err(err).Msg("workflow polling encountered fatal error, agent will exit for restart")
						stopAgentFunc(err) // Trigger agent shutdown
						return err
					}

					// Check if context is canceled - this is expected during shutdown
					if agentCtx.Err() != nil {
						return nil
					}

					// For any other error, log and exit to trigger pod restart
					// This ensures a clean slate and follows the pattern that errors should cause restarts
					log.Error().Err(err).Msg("runner encountered an error, agent will exit for restart")
					stopAgentFunc(err) // Trigger agent shutdown
					return err
				}
			}
		})
	}

	log.Info().Msgf(
		"starting Crow agent with version '%s' and backend '%s' using platform '%s' running up to %d pipelines in parallel",
		version.String(), backendEngine.Name(), engInfo.Platform, maxWorkflows)

	// Mark agent as healthy now that it has started successfully
	counter.SetHealthy(true)

	// Wait for all services to complete
	err = serviceWaitingGroup.Wait()

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
		before, after, _ := strings.Cut(v, "=")
		switch {
		case before != "" && after != "":
			m[before] = after
		case before != "":
			return fmt.Errorf("key '%s' does not have a value assigned", before)
		default:
			return fmt.Errorf("empty string in slice")
		}
	}
	return nil
}

// AdoptedWorkflow represents a workflow that has been claimed and needs to be monitored.
type AdoptedWorkflow struct {
	Workflow *rpc.Workflow
	TaskUUID string
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
