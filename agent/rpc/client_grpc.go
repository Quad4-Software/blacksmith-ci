// Copyright 2023 Woodpecker Authors
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

package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v7"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
	grpcproto "google.golang.org/protobuf/proto"

	backend "codefloe.com/crowci/crow/v6/pipeline/backend/types"
	"codefloe.com/crowci/crow/v6/pipeline/rpc"
	"codefloe.com/crowci/crow/v6/pipeline/rpc/proto"
)

// ErrAgentRestartRequired is returned when the agent has been unable to reach
// the server for too long and should be restarted by its supervisor.
var ErrAgentRestartRequired = errors.New("agent unable to reach server for too long, restart required")

// ErrAgentShuttingDown is returned when operations are aborted because the agent is shutting down.
var ErrAgentShuttingDown = errors.New("agent is shutting down")

const (
	// Set grpc version on compile time to compare against server version response.
	ClientGrpcVersion int32 = proto.Version

	// Maximum size of an outgoing log message.
	// Picked to prevent it from going over GRPC size limit (4 MiB) with a large safety margin.
	maxLogBatchSize int = 1 * 1024 * 1024

	// Maximum amount of time between sending consecutive batched log messages.
	// Controls the delay between the CI job generating a log record, and web users receiving it.
	maxLogFlushPeriod time.Duration = time.Second

	// Retry attempt threshold for escalating log level from INFO to WARN.
	// Attempts 1,2,4 log at INFO, attempts 8+ log at WARN to indicate persistent issues.
	retryWarnThreshold = 8

	// DefaultMaxErrorDuration is the default time the agent will keep retrying
	// gRPC calls before giving up and asking its supervisor to restart it. Sized
	// to comfortably outlast a normal server bounce (kernel auto-reboot, deploy,
	// cert renewal) without giving up so quickly that a transient outage costs
	// us the whole agent process.
	DefaultMaxErrorDuration = 5 * time.Minute
)

// Global flags to track agent connectivity state (shared across all client instances and runners)
//
// Design: An agent is only truly operational when BOTH health reporting AND workflow polling work.
// - workflowPollingHealthy: true when workflow polling can connect to server
// - healthReportingHealthy: true when health heartbeat can connect to server
// - agentOperationalLogged: true after we've logged "agent fully operational" message
// - operationalLogMutex: ensures only one goroutine logs the message (across 100+ workers)
// - agentShuttingDown: true when agent has decided to shut down (stops all retry loops)
//
// For workflow polling, "sql: no rows" (empty queue) is treated as success to prevent log spam.
// For health reporting, "sql: no rows" (agent not found) is treated as an error requiring reconnection.
// This prevents log spam while ensuring admins see when the agent is truly operational.
var (
	workflowPollingHealthy atomic.Bool
	healthReportingHealthy atomic.Bool
	agentOperationalLogged atomic.Bool
	operationalLogMutex    sync.Mutex
	agentShuttingDown      atomic.Bool
)

type client struct {
	client                proto.CrowClient
	conn                  *grpc.ClientConn
	logs                  chan *proto.LogEntry
	reregistrationHandler func(context.Context) error // callback to re-register agent when not found on server
	maxErrorDuration      time.Duration

	// logsDrained is closed by processLogs once it has flushed its last batch,
	// and closeOnce keeps Close idempotent, since closing c.logs twice panics.
	logsDrained chan struct{}
	closeOnce   sync.Once
}

// NewGrpcClient returns a new grpc Client.
func NewGrpcClient(ctx context.Context, conn *grpc.ClientConn) rpc.Peer {
	client := new(client)
	client.client = proto.NewCrowClient(conn)
	client.conn = conn
	client.logs = make(chan *proto.LogEntry, 10) // max memory use: 10 lines * 1 MiB
	client.logsDrained = make(chan struct{})
	client.maxErrorDuration = DefaultMaxErrorDuration
	go client.processLogs(ctx)
	return client
}

// SetMaxErrorDuration overrides the default amount of time the client will
// keep retrying gRPC calls before returning ErrAgentRestartRequired. Zero or
// negative values reset to DefaultMaxErrorDuration.
func (c *client) SetMaxErrorDuration(d time.Duration) {
	if d <= 0 {
		d = DefaultMaxErrorDuration
	}
	c.maxErrorDuration = d
}

// SetReregistrationHandler sets a callback that will be invoked when the agent
// needs to re-register with the server (e.g., after server restart).
func (c *client) SetReregistrationHandler(handler func(context.Context) error) {
	c.reregistrationHandler = handler
}

// SetWorkflowPollingHealthy marks workflow polling as healthy or unhealthy.
// This should be called when the agent starts polling, since Next() is a blocking
// call that won't return until a task is available.
func (c *client) SetWorkflowPollingHealthy(healthy bool) {
	workflowPollingHealthy.Store(healthy)
	if healthy {
		log.Trace().Msg("workflow polling marked as healthy")
	} else {
		log.Trace().Msg("workflow polling marked as unhealthy")
	}
}

// SignalAgentShutdown signals that the agent is shutting down.
// This causes all retry loops in Wait(), Extend(), etc. to stop immediately.
// This is a package-level function since shutdown affects all client instances.
func SignalAgentShutdown() {
	agentShuttingDown.Store(true)
	log.Debug().Msg("agent shutdown signal sent to all gRPC operations")
}

// IsAgentShuttingDown returns true if the agent has been signaled to shut down.
func IsAgentShuttingDown() bool {
	return agentShuttingDown.Load()
}

// ResetAgentShutdown resets the shutdown flag. This should only be used in tests.
func ResetAgentShutdown() {
	agentShuttingDown.Store(false)
}

// logDrainCloseTimeout bounds how long Close waits for the log drain to flush
// what it is still batching. Long enough for one Log call against a healthy
// server, short enough to stay well inside the agent's force-exit watchdog.
const logDrainCloseTimeout = 2 * time.Second

// Close stops the log drain and closes the connection.
//
// It waits for the drain to flush before tearing the connection down. Without
// that wait the last batch — up to maxLogFlushPeriod of a workflow's output —
// is lost whenever the agent exits right after a workflow finishes, which for
// a single-use agent is every workflow it runs.
func (c *client) Close() error {
	c.closeOnce.Do(func() {
		close(c.logs)
	})

	select {
	case <-c.logsDrained:
	case <-time.After(logDrainCloseTimeout):
		log.Warn().Msg("log drain did not flush in time, queued logs are lost")
	}

	return c.conn.Close()
}

func (c *client) newBackOff() backoff.BackOff {
	b := backoff.NewExponentialBackOff()
	b.MaxInterval = 30 * time.Second           //nolint:mnd
	b.InitialInterval = 100 * time.Millisecond //nolint:mnd
	return b
}

// cleanGrpcError extracts a human-readable error message from verbose gRPC errors.
func cleanGrpcError(err error) string {
	errMsg := err.Error()
	code := status.Code(err)

	// Extract the core error message from nested gRPC error strings
	// Example: "rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing: dial tcp 10.43.78.5:9000: connect: connection refused\""
	// Result: "connection refused (is the server reachable from this network?)"

	// Network connectivity errors
	if strings.Contains(errMsg, "connection refused") {
		return "connection refused (is the server running and accessible from this network?)"
	}
	if strings.Contains(errMsg, "no such host") {
		// Extract hostname if possible
		if strings.Contains(errMsg, "dial tcp") {
			parts := strings.Split(errMsg, "dial tcp ")
			if len(parts) > 1 {
				host := strings.Split(parts[1], ":")[0]
				return fmt.Sprintf("DNS resolution failed for '%s' (check hostname/DNS configuration)", host)
			}
		}
		return "DNS resolution failed (check server hostname/address)"
	}
	if strings.Contains(errMsg, "i/o timeout") {
		return "connection timeout (server unreachable - check firewall/network routing)"
	}
	if strings.Contains(errMsg, "no route to host") {
		return "no route to host (network routing problem or server offline)"
	}
	if strings.Contains(errMsg, "network is unreachable") {
		return "network unreachable (check network configuration)"
	}

	// Authentication/authorization errors
	if code == codes.Unauthenticated {
		return "authentication failed (invalid or expired agent token)"
	}
	if code == codes.PermissionDenied {
		return "permission denied (agent token lacks required permissions)"
	}

	// Context errors
	if strings.Contains(errMsg, "context deadline exceeded") {
		return "deadline exceeded (server too slow to respond)"
	}
	if strings.Contains(errMsg, "context canceled") {
		return "context canceled"
	}

	// TLS/certificate errors
	if strings.Contains(errMsg, "certificate") || strings.Contains(errMsg, "tls") {
		return "TLS/certificate error (check --grpc-secure and --grpc-skip-insecure settings)"
	}

	// Server-side errors
	if code == codes.Unavailable {
		return "server unavailable (check if server is running and network allows connection)"
	}
	if code == codes.Internal {
		return "internal server error (check server logs for details)"
	}
	if code == codes.Unknown && strings.Contains(errMsg, "sql: no rows") {
		// This is not an error - queue is empty
		return "queue empty"
	}

	// For other errors, return the status code description with the raw error
	if code != codes.Unknown {
		return fmt.Sprintf("%s: %s", code.String(), errMsg)
	}

	return errMsg
}

// Version returns the server- & grpc-version. It retries transient gRPC
// errors (e.g. the server isn't listening yet) using exponential backoff,
// giving up after c.maxErrorDuration so a supervisor can restart the agent
// during prolonged outages.
func (c *client) Version(ctx context.Context) (*rpc.Version, error) {
	retry := c.newBackOff()
	var firstErrorAt time.Time
	attemptCount := 0
	for {
		if agentShuttingDown.Load() {
			return nil, ErrAgentShuttingDown
		}

		res, err := c.client.Version(ctx, &proto.Empty{})
		if err == nil {
			return &rpc.Version{
				GrpcVersion:   res.GrpcVersion,
				ServerVersion: res.ServerVersion,
			}, nil
		}

		switch status.Code(err) {
		case codes.Canceled:
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, err
		case codes.Aborted,
			codes.DataLoss,
			codes.DeadlineExceeded,
			codes.Internal,
			codes.Unavailable,
			codes.Unknown:
			// transient — retry
		default:
			return nil, err
		}

		attemptCount++
		if firstErrorAt.IsZero() {
			firstErrorAt = time.Now()
		}
		if time.Since(firstErrorAt) >= c.maxErrorDuration {
			log.Error().
				Str("error", cleanGrpcError(err)).
				Int("attempts", attemptCount).
				Dur("threshold", c.maxErrorDuration).
				Msg("could not reach grpc server, giving up")
			return nil, ErrAgentRestartRequired
		}

		next := retry.NextBackOff()
		log.Warn().
			Str("error", cleanGrpcError(err)).
			Int("attempt", attemptCount).
			Dur("retry_in", next).
			Msg("grpc server version check failed, retrying")

		select {
		case <-time.After(next):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Next returns the next workflow in the queue.
func (c *client) Next(ctx context.Context, filter rpc.Filter) (*rpc.Workflow, error) {
	var res *proto.NextResponse
	var err error
	retry := c.newBackOff()
	req := new(proto.NextRequest)
	req.Filter = new(proto.Filter)
	req.Filter.Labels = filter.Labels

	attemptCount := 0
	var lastBackoff time.Duration
	var firstErrorAt time.Time

	for {
		// Log connection state before attempting Next() call
		connState := c.conn.GetState()
		log.Trace().Str("connection_state", connState.String()).Msg("attempting Next() call")

		// If connection is in a bad state, wait for it to become ready
		if connState != connectivity.Ready && connState != connectivity.Idle {
			log.Warn().
				Str("connection_state", connState.String()).
				Msg("connection not ready, waiting for state change")
			c.conn.Connect()
		}

		res, err = c.client.Next(ctx, req)
		if err == nil {
			// Mark workflow polling as healthy
			workflowPollingHealthy.Store(true)

			// Check if BOTH services are now healthy and log if we're the first
			// Use mutex to guarantee only one goroutine logs
			operationalLogMutex.Lock()
			if healthReportingHealthy.Load() && !agentOperationalLogged.Load() {
				agentOperationalLogged.Store(true)
				log.Info().Msg("agent fully operational (health reporting and workflow polling connected)")
			}
			operationalLogMutex.Unlock()
			break
		}

		// "sql: no rows in result set" is not an error - it means the queue is empty
		// Treat it as success - don't reset flags or increment attempt count
		if status.Code(err) == codes.Unknown && strings.Contains(err.Error(), "sql: no rows in result set") {
			// Mark workflow polling as healthy (same as success case)
			workflowPollingHealthy.Store(true)

			// Check if BOTH services are now healthy and log if we're the first
			// Use mutex to guarantee only one goroutine logs
			operationalLogMutex.Lock()
			if healthReportingHealthy.Load() && !agentOperationalLogged.Load() {
				agentOperationalLogged.Store(true)
				log.Info().Msg("agent fully operational (health reporting and workflow polling connected)")
			}
			operationalLogMutex.Unlock()
			return nil, nil
		}

		attemptCount++
		if firstErrorAt.IsZero() {
			firstErrorAt = time.Now()
		}

		// Mark workflow polling as unhealthy and reset operational flag
		// Only do this for ACTUAL errors, not "sql: no rows"
		workflowPollingHealthy.Store(false)
		agentOperationalLogged.Store(false)

		// Give up if we've been failing for longer than the supervisor can be
		// expected to wait. Returning this signals the agent to exit for restart.
		if errSpan := time.Since(firstErrorAt); errSpan >= c.maxErrorDuration {
			log.Error().
				Str("error", cleanGrpcError(err)).
				Int("consecutive_errors", attemptCount).
				Dur("error_duration", errSpan).
				Dur("threshold", c.maxErrorDuration).
				Msg("workflow polling unable to reach server, agent will exit for restart")
			return nil, ErrAgentRestartRequired
		}

		switch status.Code(err) {
		case codes.Canceled:
			if ctx.Err() != nil {
				// expected as context was canceled
				log.Debug().Err(err).Msgf("grpc error: next(): context canceled")
				return nil, nil
			}
			log.Error().Err(err).Msgf("grpc error: next(): code: %v", status.Code(err))
			return nil, err
		case
			codes.Aborted,
			codes.DataLoss,
			codes.DeadlineExceeded,
			codes.Internal,
			codes.Unavailable,
			codes.Unknown:
			// non-fatal errors that should be retried

			// If we get Unavailable but connection shows READY, the connection is stale
			// (likely closed by proxy/load balancer). Force reconnection.
			if status.Code(err) == codes.Unavailable && c.conn.GetState() == connectivity.Ready {
				log.Warn().
					Str("error", cleanGrpcError(err)).
					Msg("connection shows READY but RPC failed with Unavailable - forcing reconnection")
				c.conn.Connect()
			}

			lastBackoff = retry.NextBackOff()
			// TODO: remove after adding continuous data exchange by something like #536
			if strings.Contains(err.Error(), "\"too_many_pings\"") {
				// https://github.com/woodpecker-ci/woodpecker/issues/717#issuecomment-1049365104
				log.Trace().Err(err).Msg("grpc: to many keepalive pings without sending data")
			} else {
				// Log retry attempts at different levels to reduce noise:
				// - attempts 1, 2, 4: INFO (normal reconnection attempts)
				// - attempts 8+: WARN (persistent connection issues)
				// - all others: DEBUG (detailed troubleshooting only)
				isPowerOfTwo := attemptCount > 0 && (attemptCount&(attemptCount-1)) == 0
				cleanErr := cleanGrpcError(err)
				if isPowerOfTwo {
					if attemptCount >= retryWarnThreshold {
						log.Warn().
							Str("error", cleanErr).
							Int("attempt", attemptCount).
							Dur("retry_in", lastBackoff).
							Msg("workflow polling error, retrying")
					} else {
						log.Info().
							Str("error", cleanErr).
							Int("attempt", attemptCount).
							Dur("retry_in", lastBackoff).
							Msg("workflow polling error, retrying")
					}
				} else {
					log.Debug().
						Str("error", cleanErr).
						Int("attempt", attemptCount).
						Dur("retry_in", lastBackoff).
						Msg("workflow polling error, retrying")
				}
			}
		default:
			log.Error().Err(err).Msgf("grpc error: next(): code: %v", status.Code(err))
			return nil, err
		}

		select {
		case <-time.After(lastBackoff):
		case <-ctx.Done():
			return nil, nil
		}
	}

	if res.GetWorkflow() == nil {
		return nil, nil
	}

	w := new(rpc.Workflow)
	w.ID = res.GetWorkflow().GetId()
	w.Timeout = res.GetWorkflow().GetTimeout()
	w.Config = new(backend.Config)
	if err := json.Unmarshal(res.GetWorkflow().GetPayload(), w.Config); err != nil {
		log.Error().Err(err).Msgf("could not unmarshal workflow config of '%s'", w.ID)
	}
	return w, nil
}

// Wait blocks until the workflow is complete.
func (c *client) Wait(ctx context.Context, workflowID string) (err error) {
	retry := c.newBackOff()
	req := new(proto.WaitRequest)
	req.Id = workflowID
	for {
		// Check if agent is shutting down before attempting RPC
		if agentShuttingDown.Load() {
			log.Debug().Str("workflow_id", workflowID).Msg("wait(): aborting due to agent shutdown")
			return ErrAgentShuttingDown
		}

		_, err = c.client.Wait(ctx, req)
		if err == nil {
			break
		}

		switch status.Code(err) {
		case codes.Canceled:
			if ctx.Err() != nil {
				// expected as context was canceled
				log.Debug().Err(err).Msgf("grpc error: wait(): context canceled")
				return nil
			}
			log.Error().Err(err).Msgf("grpc error: wait(): code: %v", status.Code(err))
			return err
		case
			codes.Aborted,
			codes.DataLoss,
			codes.DeadlineExceeded,
			codes.Internal,
			codes.Unavailable:
			// non-fatal errors
			log.Warn().Err(err).Msgf("grpc error: wait(): code: %v", status.Code(err))
		default:
			log.Error().Err(err).Msgf("grpc error: wait(): code: %v", status.Code(err))
			return err
		}

		// Check shutdown before sleeping
		if agentShuttingDown.Load() {
			log.Debug().Str("workflow_id", workflowID).Msg("wait(): aborting retry due to agent shutdown")
			return ErrAgentShuttingDown
		}

		select {
		case <-time.After(retry.NextBackOff()):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Init signals the workflow is initialized.
func (c *client) Init(ctx context.Context, workflowID string, state rpc.WorkflowState) (err error) {
	retry := c.newBackOff()
	req := new(proto.InitRequest)
	req.Id = workflowID
	req.State = new(proto.WorkflowState)
	req.State.Started = state.Started
	req.State.Finished = state.Finished
	req.State.Error = state.Error
	for {
		// Check if agent is shutting down before attempting RPC
		if agentShuttingDown.Load() {
			log.Debug().Str("workflow_id", workflowID).Msg("init(): aborting due to agent shutdown")
			return ErrAgentShuttingDown
		}

		_, err = c.client.Init(ctx, req)
		if err == nil {
			break
		}

		log.Error().Err(err).Msgf("grpc error: init(): code: %v", status.Code(err))

		switch status.Code(err) {
		case codes.Canceled:
			if ctx.Err() != nil {
				// expected as context was canceled
				log.Debug().Err(err).Msgf("grpc error: init(): context canceled")
				return nil
			}
			log.Error().Err(err).Msgf("grpc error: init(): code: %v", status.Code(err))
			return err
		case
			codes.Aborted,
			codes.DataLoss,
			codes.DeadlineExceeded,
			codes.Internal,
			codes.Unavailable:
			// non-fatal errors
			log.Warn().Err(err).Msgf("grpc error: init(): code: %v", status.Code(err))
		default:
			log.Error().Err(err).Msgf("grpc error: init(): code: %v", status.Code(err))
			return err
		}

		// Check shutdown before sleeping
		if agentShuttingDown.Load() {
			log.Debug().Str("workflow_id", workflowID).Msg("init(): aborting retry due to agent shutdown")
			return ErrAgentShuttingDown
		}

		select {
		case <-time.After(retry.NextBackOff()):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Done signals the workflow is complete.
func (c *client) Done(ctx context.Context, workflowID string, state rpc.WorkflowState) (err error) {
	retry := c.newBackOff()
	req := new(proto.DoneRequest)
	req.Id = workflowID
	req.State = new(proto.WorkflowState)
	req.State.Started = state.Started
	req.State.Finished = state.Finished
	req.State.Error = state.Error
	for {
		// Check if agent is shutting down before attempting RPC
		if agentShuttingDown.Load() {
			log.Debug().Str("workflow_id", workflowID).Msg("done(): aborting due to agent shutdown")
			return ErrAgentShuttingDown
		}

		_, err = c.client.Done(ctx, req)
		if err == nil {
			break
		}

		log.Error().Err(err).Msgf("grpc error: done(): code: %v", status.Code(err))

		switch status.Code(err) {
		case codes.Canceled:
			if ctx.Err() != nil {
				// expected as context was canceled
				log.Debug().Err(err).Msgf("grpc error: done(): context canceled")
				return nil
			}
			log.Error().Err(err).Msgf("grpc error: done(): code: %v", status.Code(err))
			return err
		case
			codes.Aborted,
			codes.DataLoss,
			codes.DeadlineExceeded,
			codes.Internal,
			codes.Unavailable:
			// non-fatal errors
			log.Warn().Err(err).Msgf("grpc error: done(): code: %v", status.Code(err))
		default:
			log.Error().Err(err).Msgf("grpc error: done(): code: %v", status.Code(err))
			return err
		}

		// Check shutdown before sleeping
		if agentShuttingDown.Load() {
			log.Debug().Str("workflow_id", workflowID).Msg("done(): aborting retry due to agent shutdown")
			return ErrAgentShuttingDown
		}

		select {
		case <-time.After(retry.NextBackOff()):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Extend extends the workflow deadline.
func (c *client) Extend(ctx context.Context, workflowID string) (err error) {
	retry := c.newBackOff()
	req := new(proto.ExtendRequest)
	req.Id = workflowID
	for {
		// Check if agent is shutting down before attempting RPC
		if agentShuttingDown.Load() {
			log.Debug().Str("workflow_id", workflowID).Msg("extend(): aborting due to agent shutdown")
			return ErrAgentShuttingDown
		}

		_, err = c.client.Extend(ctx, req)
		if err == nil {
			break
		}

		log.Error().Err(err).Msgf("grpc error: extend(): code: %v", status.Code(err))

		switch status.Code(err) {
		case codes.Canceled:
			if ctx.Err() != nil {
				// expected as context was canceled
				log.Debug().Err(err).Msgf("grpc error: extend(): context canceled")
				return nil
			}
			log.Error().Err(err).Msgf("grpc error: extend(): code: %v", status.Code(err))
			return err
		case
			codes.Aborted,
			codes.DataLoss,
			codes.DeadlineExceeded,
			codes.Internal,
			codes.Unavailable:
			// non-fatal errors
			log.Warn().Err(err).Msgf("grpc error: extend(): code: %v", status.Code(err))
		default:
			log.Error().Err(err).Msgf("grpc error: extend(): code: %v", status.Code(err))
			return err
		}

		// Check shutdown before sleeping
		if agentShuttingDown.Load() {
			log.Debug().Str("workflow_id", workflowID).Msg("extend(): aborting retry due to agent shutdown")
			return ErrAgentShuttingDown
		}

		select {
		case <-time.After(retry.NextBackOff()):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Update updates the workflow state.
func (c *client) Update(ctx context.Context, workflowID string, state rpc.StepState) (err error) {
	retry := c.newBackOff()
	req := new(proto.UpdateRequest)
	req.Id = workflowID
	req.State = new(proto.StepState)
	req.State.StepUuid = state.StepUUID
	req.State.Started = state.Started
	req.State.Finished = state.Finished
	req.State.Exited = state.Exited
	req.State.ExitCode = int32(state.ExitCode)
	req.State.Error = state.Error
	for {
		// Check if agent is shutting down before attempting RPC
		if agentShuttingDown.Load() {
			log.Debug().Str("workflow_id", workflowID).Msg("update(): aborting due to agent shutdown")
			return ErrAgentShuttingDown
		}

		_, err = c.client.Update(ctx, req)
		if err == nil {
			break
		}

		log.Error().Err(err).Msgf("grpc error: update(): code: %v", status.Code(err))

		switch status.Code(err) {
		case codes.Canceled:
			if ctx.Err() != nil {
				// expected as context was canceled
				log.Debug().Err(err).Msgf("grpc error: update(): context canceled")
				return nil
			}
			log.Error().Err(err).Msgf("grpc error: update(): code: %v", status.Code(err))
			return err
		case
			codes.Aborted,
			codes.DataLoss,
			codes.DeadlineExceeded,
			codes.Internal,
			codes.Unavailable:
			// non-fatal errors
			log.Warn().Err(err).Msgf("grpc error: update(): code: %v", status.Code(err))
		default:
			log.Error().Err(err).Msgf("grpc error: update(): code: %v", status.Code(err))
			return err
		}

		// Check shutdown before sleeping
		if agentShuttingDown.Load() {
			log.Debug().Str("workflow_id", workflowID).Msg("update(): aborting retry due to agent shutdown")
			return ErrAgentShuttingDown
		}

		select {
		case <-time.After(retry.NextBackOff()):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// EnqueueLog queues the log entry to be written in a batch later.
func (c *client) EnqueueLog(logEntry *rpc.LogEntry) {
	c.logs <- &proto.LogEntry{
		StepUuid: logEntry.StepUUID,
		Data:     logEntry.Data,
		Line:     int32(logEntry.Line),
		Time:     logEntry.Time,
		Type:     int32(logEntry.Type),
	}
}

func (c *client) processLogs(ctx context.Context) {
	// Signals Close that everything queued has been flushed.
	defer close(c.logsDrained)

	var entries []*proto.LogEntry
	var bytes int

	send := func() {
		if len(entries) == 0 {
			return
		}

		log.Debug().
			Int("entries", len(entries)).
			Int("bytes", bytes).
			Msg("log drain: sending queued logs")

		if err := c.sendLogs(ctx, entries); err != nil {
			log.Error().Err(err).Msg("log drain: could not send logs to server")
		}

		// even if send failed, we don't have infinite memory; retry has already been used
		entries = entries[:0]
		bytes = 0
	}

	// ctx.Done() is covered by the log channel being closed
	for {
		select {
		case entry, ok := <-c.logs:
			if !ok {
				log.Info().Msg("log drain: channel closed")
				send()
				return
			}

			entries = append(entries, entry)
			bytes += grpcproto.Size(entry) // cspell:words grpcproto

			if bytes >= maxLogBatchSize {
				send()
			}

		case <-time.After(maxLogFlushPeriod):
			send()
		}
	}
}

func (c *client) sendLogs(ctx context.Context, entries []*proto.LogEntry) error {
	req := &proto.LogRequest{LogEntries: entries}
	retry := c.newBackOff()

	for {
		_, err := c.client.Log(ctx, req)
		if err == nil {
			break
		}

		switch status.Code(err) {
		case codes.Canceled:
			if ctx.Err() != nil {
				// expected as context was canceled
				log.Debug().Err(err).Msgf("grpc error: log(): context canceled")
				return nil
			}
			log.Error().Err(err).Msgf("grpc error: log(): code: %v", status.Code(err))
			return err
		case
			codes.Aborted,
			codes.DataLoss,
			codes.DeadlineExceeded,
			codes.Internal,
			codes.Unavailable:
			// non-fatal errors
			log.Warn().Err(err).Msgf("grpc error: log(): code: %v", status.Code(err))
		default:
			log.Error().Err(err).Msgf("grpc error: log(): code: %v", status.Code(err))
			return err
		}

		select {
		case <-time.After(retry.NextBackOff()):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (c *client) RegisterAgent(ctx context.Context, info rpc.AgentInfo) (int64, string, error) {
	req := new(proto.RegisterAgentRequest)
	req.Info = &proto.AgentInfo{
		Platform:      info.Platform,
		Backend:       info.Backend,
		Version:       info.Version,
		BuildDate:     info.BuildDate,
		Capacity:      int32(info.Capacity),
		CustomLabels:  info.CustomLabels,
		LimitMem:      info.LimitMem,
		LimitCpuQuota: info.LimitCPUQuota,
		HostCpus:      info.HostCPUs,
		HostMem:       info.HostMem,
	}

	res, err := c.client.RegisterAgent(ctx, req)
	if err != nil {
		return 0, "", err
	}
	return res.GetAgentId(), res.GetAgentName(), nil
}

func (c *client) UnregisterAgent(ctx context.Context) error {
	_, err := c.client.UnregisterAgent(ctx, &proto.Empty{})
	return err
}

func (c *client) ReportHealth(ctx context.Context) (cleanupConfig *rpc.DockerCleanupConfig, err error) {
	retry := c.newBackOff()
	req := new(proto.ReportHealthRequest)
	req.Status = "I am alive!"
	req.WorkflowPollingHealthy = workflowPollingHealthy.Load()

	log.Trace().
		Bool("workflow_polling_healthy", req.WorkflowPollingHealthy).
		Msg("reporting health to server")

	attemptCount := 0
	var lastBackoff time.Duration
	var firstErrorAt time.Time

	for {
		var res *proto.ReportHealthResponse
		res, err = c.client.ReportHealth(ctx, req)
		if err == nil {
			// Mark health reporting as healthy
			healthReportingHealthy.Store(true)

			// Check if BOTH services are now healthy and log if we're the first
			// Use mutex to guarantee only one goroutine logs
			operationalLogMutex.Lock()
			if workflowPollingHealthy.Load() && !agentOperationalLogged.Load() {
				agentOperationalLogged.Store(true)
				log.Info().Msg("agent fully operational (health reporting and workflow polling connected)")
			}
			operationalLogMutex.Unlock()

			// Extract Docker cleanup config from response
			if res.GetDockerCleanupConfig() != nil {
				cfg := res.GetDockerCleanupConfig()
				cleanupConfig = &rpc.DockerCleanupConfig{
					Enabled:               cfg.GetEnabled(),
					StaleThresholdSeconds: cfg.GetStaleThresholdSeconds(),
					IntervalSeconds:       cfg.GetIntervalSeconds(),
					RunNow:                cfg.GetRunNow(),
				}
			}
			return cleanupConfig, nil
		}

		// NOTE: Unlike workflow polling, "sql: no rows" is a REAL error for health reporting.
		// It means the agent doesn't exist in the database, which should trigger reconnection.
		// Check if this is an "agent not found" error and trigger re-registration if handler is set.
		if status.Code(err) == codes.Unknown && strings.Contains(err.Error(), "sql: no rows") {
			if c.reregistrationHandler != nil {
				log.Info().Msg("agent not found on server, attempting re-registration")
				if reregErr := c.reregistrationHandler(ctx); reregErr != nil {
					log.Error().Err(reregErr).Msg("re-registration failed, will retry")
					// Don't return - continue with retry logic below
				} else {
					log.Info().Msg("re-registration successful, resuming health reporting")
					// Reset attempt count and backoff after successful re-registration
					attemptCount = 0
					retry = c.newBackOff()
					continue
				}
			}
		}

		attemptCount++
		if firstErrorAt.IsZero() {
			firstErrorAt = time.Now()
		}

		// Mark health reporting as unhealthy and reset operational flag
		healthReportingHealthy.Store(false)
		agentOperationalLogged.Store(false)

		// Give up if we've been failing for longer than the supervisor can be
		// expected to wait. Returning this signals the agent to exit for restart.
		if errSpan := time.Since(firstErrorAt); errSpan >= c.maxErrorDuration {
			log.Error().
				Str("error", cleanGrpcError(err)).
				Int("consecutive_errors", attemptCount).
				Dur("error_duration", errSpan).
				Dur("threshold", c.maxErrorDuration).
				Msg("health reporting unable to reach server, agent will exit for restart")
			return nil, ErrAgentRestartRequired
		}

		switch status.Code(err) {
		case codes.Canceled:
			if ctx.Err() != nil {
				// expected as context was canceled
				log.Debug().Err(err).Msgf("grpc error: report_health(): context canceled")
				return nil, nil
			}
			log.Error().Err(err).Msgf("grpc error: report_health(): code: %v", status.Code(err))
			return nil, err
		case
			codes.Aborted,
			codes.DataLoss,
			codes.DeadlineExceeded,
			codes.Internal,
			codes.Unavailable,
			codes.Unknown:
			// non-fatal errors that should be retried
			lastBackoff = retry.NextBackOff()
			// Log retry attempts at different levels to reduce noise:
			// - attempts 1, 2, 4: INFO (normal reconnection attempts)
			// - attempts 8+: WARN (persistent connection issues)
			// - all others: DEBUG (detailed troubleshooting only)
			isPowerOfTwo := attemptCount > 0 && (attemptCount&(attemptCount-1)) == 0
			cleanErr := cleanGrpcError(err)
			if isPowerOfTwo {
				if attemptCount >= retryWarnThreshold {
					log.Warn().
						Str("error", cleanErr).
						Int("attempt", attemptCount).
						Dur("retry_in", lastBackoff).
						Msg("health reporting error, retrying")
				} else {
					log.Info().
						Str("error", cleanErr).
						Int("attempt", attemptCount).
						Dur("retry_in", lastBackoff).
						Msg("health reporting error, retrying")
				}
			} else {
				log.Debug().
					Str("error", cleanErr).
					Int("attempt", attemptCount).
					Dur("retry_in", lastBackoff).
					Msg("health reporting error, retrying")
			}
		default:
			log.Error().Err(err).
				Int("attempts", attemptCount).
				Msgf("grpc error: report_health(): code: %v (fatal, giving up)", status.Code(err))
			return nil, err
		}

		select {
		case <-time.After(lastBackoff):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// ReportDockerCleanup sends Docker cleanup results to the server.
func (c *client) ReportDockerCleanup(ctx context.Context, result rpc.DockerCleanupResult) error {
	req := &proto.DockerCleanupResultRequest{
		ContainersFound:   result.ContainersFound,
		ContainersRemoved: result.ContainersRemoved,
		NetworksFound:     result.NetworksFound,
		NetworksRemoved:   result.NetworksRemoved,
		VolumesFound:      result.VolumesFound,
		VolumesRemoved:    result.VolumesRemoved,
		Success:           result.Success,
		Error:             result.Error,
		DurationMs:        result.DurationMs,
		Manual:            result.Manual,
	}
	_, err := c.client.ReportDockerCleanup(ctx, req)
	return err
}

// ClaimTask attempts to claim an orphaned workflow task for adoption after agent restart.
// Returns the workflow if successfully claimed, nil if the task couldn't be claimed.
func (c *client) ClaimTask(ctx context.Context, taskID string) (*rpc.Workflow, error) {
	req := &proto.ClaimTaskRequest{TaskId: taskID}
	res, err := c.client.ClaimTask(ctx, req)
	if err != nil {
		return nil, err
	}

	if res.GetWorkflow() == nil {
		return nil, nil
	}

	w := new(rpc.Workflow)
	w.ID = res.GetWorkflow().GetId()
	w.Timeout = res.GetWorkflow().GetTimeout()
	w.Config = new(backend.Config)
	if err := json.Unmarshal(res.GetWorkflow().GetPayload(), w.Config); err != nil {
		log.Error().Err(err).Msgf("could not unmarshal workflow config of '%s'", w.ID)
		return nil, err
	}
	return w, nil
}
