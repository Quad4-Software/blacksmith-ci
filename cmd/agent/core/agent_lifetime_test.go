// Copyright 2025 Crow Authors
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
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
	"google.golang.org/grpc"

	agent_rpc "codefloe.com/crowci/crow/v6/agent/rpc"
	"codefloe.com/crowci/crow/v6/pipeline"
	"codefloe.com/crowci/crow/v6/pipeline/backend/dummy"
	backend "codefloe.com/crowci/crow/v6/pipeline/backend/types"
	"codefloe.com/crowci/crow/v6/pipeline/rpc/proto"
	"codefloe.com/crowci/crow/v6/shared/utils"
	"codefloe.com/crowci/crow/v6/version"
)

// fakeAuthServer answers the agent's authentication handshake. The agent only
// stores the returned access token and sends it back as metadata, so any
// non-empty string will do.
type fakeAuthServer struct {
	proto.UnimplementedCrowAuthServer
}

func (fakeAuthServer) Auth(_ context.Context, _ *proto.AuthRequest) (*proto.AuthResponse, error) {
	return &proto.AuthResponse{Status: "ok", AgentId: 7, AccessToken: "test-token"}, nil
}

// fakeCrowServer is the smallest server an agent can actually work against: it
// hands out a fixed list of workflows, one per Next, and records what the agent
// did with them.
type fakeCrowServer struct {
	proto.UnimplementedCrowServer

	// onDone, when set, is called after a workflow was reported done, so a test
	// can stop the agent at a chosen point.
	onDone func(workflowID string)

	mu           sync.Mutex
	queue        []*proto.Workflow
	nextCalls    int
	doneIDs      []string
	logEntries   int
	unregistered int
}

func (f *fakeCrowServer) Version(context.Context, *proto.Empty) (*proto.VersionResponse, error) {
	return &proto.VersionResponse{
		GrpcVersion:             proto.LegacyVersion,
		ServerVersion:           version.String(),
		ProtocolVersion:         proto.Version,
		MinAgentProtocolVersion: proto.MinAgentVersion,
	}, nil
}

func (f *fakeCrowServer) RegisterAgent(context.Context, *proto.RegisterAgentRequest) (*proto.RegisterAgentResponse, error) {
	return &proto.RegisterAgentResponse{AgentId: 7, AgentName: "test-agent"}, nil
}

func (f *fakeCrowServer) UnregisterAgent(context.Context, *proto.Empty) (*proto.Empty, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unregistered++
	return &proto.Empty{}, nil
}

func (f *fakeCrowServer) ReportHealth(context.Context, *proto.ReportHealthRequest) (*proto.ReportHealthResponse, error) {
	return &proto.ReportHealthResponse{}, nil
}

func (f *fakeCrowServer) Next(ctx context.Context, _ *proto.NextRequest) (*proto.NextResponse, error) {
	f.mu.Lock()
	f.nextCalls++
	var workflow *proto.Workflow
	if len(f.queue) > 0 {
		workflow, f.queue = f.queue[0], f.queue[1:]
	}
	f.mu.Unlock()

	if workflow == nil {
		// Mimic the real server's long poll on an empty queue rather than
		// answering in a hot loop.
		select {
		case <-ctx.Done():
		case <-time.After(20 * time.Millisecond):
		}
		return &proto.NextResponse{}, nil
	}
	return &proto.NextResponse{Workflow: workflow}, nil
}

func (f *fakeCrowServer) Init(context.Context, *proto.InitRequest) (*proto.Empty, error) {
	return &proto.Empty{}, nil
}

func (f *fakeCrowServer) Update(context.Context, *proto.UpdateRequest) (*proto.Empty, error) {
	return &proto.Empty{}, nil
}

func (f *fakeCrowServer) Log(_ context.Context, req *proto.LogRequest) (*proto.Empty, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logEntries += len(req.GetLogEntries())
	return &proto.Empty{}, nil
}

func (f *fakeCrowServer) Extend(context.Context, *proto.ExtendRequest) (*proto.Empty, error) {
	return &proto.Empty{}, nil
}

func (f *fakeCrowServer) Done(_ context.Context, req *proto.DoneRequest) (*proto.Empty, error) {
	f.mu.Lock()
	f.doneIDs = append(f.doneIDs, req.GetId())
	f.mu.Unlock()

	if f.onDone != nil {
		f.onDone(req.GetId())
	}
	return &proto.Empty{}, nil
}

// Wait is the cancel channel: the real server blocks until the workflow is
// canceled, so block until the agent drops the call.
func (f *fakeCrowServer) Wait(ctx context.Context, _ *proto.WaitRequest) (*proto.Empty, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (f *fakeCrowServer) state() (nextCalls int, doneIDs []string, unregistered int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.nextCalls, append([]string(nil), f.doneIDs...), f.unregistered
}

func (f *fakeCrowServer) receivedLogEntries() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.logEntries
}

func testWorkflow(id string) *proto.Workflow {
	payload, err := json.Marshal(&backend.Config{
		Stages: []*backend.Stage{{
			Steps: []*backend.Step{{
				Name: "step",
				UUID: "step-uuid-" + id,
				Type: backend.StepTypeCommands,
				Environment: map[string]string{
					"CI_REPO":            "octocat/hello-world",
					"CI_PIPELINE_NUMBER": id,
				},
				OnSuccess: true,
			}},
		}},
	})
	if err != nil {
		panic(err) //nolint:forbidigo // unreachable: a static config always marshals
	}
	return &proto.Workflow{Id: id, Payload: payload}
}

// startFakeServer serves both agent-facing gRPC services on a loopback port and
// returns the address to dial.
func startFakeServer(t *testing.T, srv *fakeCrowServer) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	proto.RegisterCrowServer(grpcServer, srv)
	proto.RegisterCrowAuthServer(grpcServer, fakeAuthServer{})
	go func() {
		_ = grpcServer.Serve(listener)
	}()
	t.Cleanup(grpcServer.Stop)

	return listener.Addr().String()
}

// runTestAgent runs a full agent against the fake server, exactly as the binary
// would, and returns what run() returned — which is what the process exits with.
func runTestAgent(t *testing.T, addr string, args ...string) error {
	t.Helper()

	// Well under the package's own -timeout, so a regression surfaces as this
	// error rather than as the test binary's goroutine dump.
	ctx, cancel := context.WithTimeoutCause(t.Context(), 20*time.Second, errTestAgentTimeout)
	defer cancel()

	return runTestAgentWithContext(ctx, t, addr, args...)
}

// errTestAgentTimeout marks an agent that never retired on its own.
var errTestAgentTimeout = errors.New("test agent did not exit in time")

// runTestAgentWithContext is runTestAgent for tests that stop the agent
// themselves rather than letting it retire.
func runTestAgentWithContext(ctx context.Context, t *testing.T, addr string, args ...string) error {
	t.Helper()

	// run() mutates package state that outlives a single agent, so put it back
	// for whatever test comes next.
	agent_rpc.ResetAgentShutdown()
	t.Cleanup(agent_rpc.ResetAgentShutdown)
	// Every shutdown latches the preserve-workflows flag, which would stop the
	// next agent in this binary from reporting its workflows as done.
	pipeline.SetPreserveWorkflows(false)
	t.Cleanup(func() { pipeline.SetPreserveWorkflows(false) })
	t.Cleanup(func() { shutdownCtx = context.Background() })
	t.Cleanup(func() { counter.SetRetiring(false) })

	// The watchdog would take the test binary down with os.Exit while later
	// tests are still running. Long enough that it never fires, short enough
	// that the goroutine it leaks per agent is gone well before a developer
	// running this package in a loop notices.
	previousForceExitTimeout := forceExitTimeout
	forceExitTimeout = 2 * time.Minute
	t.Cleanup(func() { forceExitTimeout = previousForceExitTimeout })

	engine := dummy.New()
	command := &cli.Command{
		Name:  "crow-agent",
		Flags: utils.MergeSlices(flags, engine.Flags()),
		Action: func(ctx context.Context, c *cli.Command) error {
			return run(ctx, c, []backend.Backend{engine})
		},
	}

	return command.Run(ctx, append([]string{
		"crow-agent",
		"--server", addr,
		"--grpc-token", "test-secret",
		"--backend-engine", "dummy",
		"--healthcheck=false",
		"--proclog-ship-enabled=false",
	}, args...))
}

// A single-use agent must run exactly the workflows it is budgeted for, then
// unregister and exit cleanly so its machine can be discarded.
func TestAgentRetiresAfterLifetimeBudget(t *testing.T) {
	srv := &fakeCrowServer{queue: []*proto.Workflow{testWorkflow("1"), testWorkflow("2")}}
	addr := startFakeServer(t, srv)

	// The persisted agent config is what normally suppresses unregistering; a
	// retiring agent unregisters anyway.
	configPath := filepath.Join(t.TempDir(), "agent.conf")

	require.NoError(t, runTestAgent(t, addr,
		"--agent-config", configPath,
		"--max-workflows", "4",
		"--max-lifetime-workflows", "1",
	), "a retiring agent exits without error")

	nextCalls, doneIDs, unregistered := srv.state()
	assert.Equal(t, 1, nextCalls, "the agent must stop polling after its only workflow")
	assert.Equal(t, []string{"1"}, doneIDs, "the second queued workflow belongs to the next agent")
	assert.Equal(t, 1, unregistered, "a retiring agent unregisters even though it persisted its ID")

	// The agent exits within milliseconds of the workflow finishing, well
	// inside the log drain's batch interval, so the output only reaches the
	// server if shutdown flushes the drain.
	assert.NotZero(t, srv.receivedLogEntries(), "the last workflow's logs must be flushed before the agent exits")
}

func TestAgentRetiresAfterSeveralWorkflows(t *testing.T) {
	srv := &fakeCrowServer{queue: []*proto.Workflow{
		testWorkflow("1"), testWorkflow("2"), testWorkflow("3"), testWorkflow("4"),
	}}
	addr := startFakeServer(t, srv)

	require.NoError(t, runTestAgent(t, addr,
		"--agent-config", "",
		"--max-workflows", "1",
		"--max-lifetime-workflows", "3",
	))

	nextCalls, doneIDs, unregistered := srv.state()
	assert.Equal(t, 3, nextCalls)
	assert.Equal(t, []string{"1", "2", "3"}, doneIDs)
	assert.Equal(t, 1, unregistered)
}

// The default of an unlimited lifetime must keep an agent polling for as long
// as it is up, however many workflows it has already run.
func TestAgentWithoutLifetimeBudgetKeepsPolling(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)

	srv := &fakeCrowServer{queue: []*proto.Workflow{
		testWorkflow("1"), testWorkflow("2"), testWorkflow("3"), testWorkflow("4"),
	}}
	srv.onDone = func(workflowID string) {
		if workflowID == "4" {
			cancel(nil)
		}
	}
	addr := startFakeServer(t, srv)

	require.NoError(t, runTestAgentWithContext(ctx, t, addr,
		"--agent-config", "",
		"--max-workflows", "1",
	))

	_, doneIDs, _ := srv.state()
	assert.Equal(t, []string{"1", "2", "3", "4"}, doneIDs)
}
