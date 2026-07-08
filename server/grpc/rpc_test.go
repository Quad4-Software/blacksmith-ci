// Copyright 2022 Woodpecker Authors
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

package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/metadata"

	"codefloe.com/crowci/crow/v6/pipeline/rpc"
	"codefloe.com/crowci/crow/v6/server/logging"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pubsub"
	queue_mocks "codefloe.com/crowci/crow/v6/server/queue/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

// newTestGRPCMetrics creates a GRPCMetrics using a fresh registry to avoid
// duplicate registration panics when multiple tests run in the same process.
func newTestGRPCMetrics() *GRPCMetrics {
	f := promauto.With(prometheus.NewRegistry())
	return &GRPCMetrics{
		PipelineTime: f.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "crow", Name: "pipeline_time",
		}, []string{"repo", "branch", "status", "pipeline"}),
		PipelineCount: f.NewCounterVec(prometheus.CounterOpts{
			Namespace: "crow", Name: "pipeline_count",
		}, []string{"repo", "branch", "status", "pipeline"}),
		PipelineDuration: f.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "crow", Name: "pipeline_duration_seconds",
			Buckets: []float64{30, 60, 120, 300, 600, 1200, 1800, 3600},
		}, []string{"repo", "branch", "status"}),
		AgentRegistrations: f.NewCounter(prometheus.CounterOpts{
			Namespace: "crow", Name: "agent_registrations_total",
		}),
		AgentUnregistrations: f.NewCounter(prometheus.CounterOpts{
			Namespace: "crow", Name: "agent_unregistrations_total",
		}),
	}
}

func TestRegisterAgent(t *testing.T) {
	metrics := newTestGRPCMetrics()

	t.Run("When existing agent Name is empty it should update Name with hostname from metadata", func(t *testing.T) {
		store := new(store_mocks.MockStore)
		storeAgent := new(model.Agent)
		storeAgent.ID = 1337
		// Use mock.MatchedBy to verify key fields, as PersistentID is computed during registration
		agentMatcher := mock.MatchedBy(func(a *model.Agent) bool {
			return a.ID == 1337 &&
				a.Name == "hostname" &&
				a.Platform == "platform" &&
				a.Backend == "backend" &&
				a.Capacity == 2 &&
				a.Version == "version" &&
				a.NoSchedule == false &&
				a.PersistentID != "" // PersistentID should be generated
		})

		store.On("AgentFind", int64(1337)).Once().Return(storeAgent, nil)
		store.On("AgentUpdate", agentMatcher).Once().Return(nil)
		grpc := RPC{
			store:   store,
			metrics: metrics,
		}
		ctx := metadata.NewIncomingContext(
			t.Context(),
			metadata.Pairs("hostname", "hostname", "agent_id", "1337"),
		)
		agentID, agentName, err := grpc.RegisterAgent(ctx, rpc.AgentInfo{
			Version:  "version",
			Platform: "platform",
			Backend:  "backend",
			Capacity: 2,
		})
		if !assert.NoError(t, err) {
			return
		}
		_ = agentName // Not testing the name in this test

		assert.EqualValues(t, 1337, agentID)
	})

	t.Run("When existing agent hostname is present it should not update the hostname", func(t *testing.T) {
		store := new(store_mocks.MockStore)
		storeAgent := new(model.Agent)
		storeAgent.ID = 1337
		storeAgent.Name = "originalHostname"
		// Use mock.MatchedBy to verify key fields, as PersistentID is computed during registration
		agentMatcher := mock.MatchedBy(func(a *model.Agent) bool {
			return a.ID == 1337 &&
				a.Name == "originalHostname" && // should NOT be updated
				a.Platform == "platform" &&
				a.Backend == "backend" &&
				a.Capacity == 2 &&
				a.Version == "version" &&
				a.NoSchedule == false &&
				a.PersistentID != "" // PersistentID should be generated
		})

		store.On("AgentFind", int64(1337)).Once().Return(storeAgent, nil)
		store.On("AgentUpdate", agentMatcher).Once().Return(nil)
		grpc := RPC{
			store:   store,
			metrics: metrics,
		}
		ctx := metadata.NewIncomingContext(
			t.Context(),
			metadata.Pairs("hostname", "newHostname", "agent_id", "1337"),
		)
		agentID, agentName, err := grpc.RegisterAgent(ctx, rpc.AgentInfo{
			Version:  "version",
			Platform: "platform",
			Backend:  "backend",
			Capacity: 2,
		})
		if !assert.NoError(t, err) {
			return
		}
		_ = agentName // Not testing the name in this test

		assert.EqualValues(t, 1337, agentID)
	})

	t.Run("It should preserve the server-managed priority across re-registration", func(t *testing.T) {
		store := new(store_mocks.MockStore)
		storeAgent := new(model.Agent)
		storeAgent.ID = 1337
		storeAgent.Priority = 42 // set server-side, must survive registration
		agentMatcher := mock.MatchedBy(func(a *model.Agent) bool {
			return a.ID == 1337 && a.Priority == 42
		})

		store.On("AgentFind", int64(1337)).Once().Return(storeAgent, nil)
		store.On("AgentUpdate", agentMatcher).Once().Return(nil)
		grpc := RPC{
			store:   store,
			metrics: metrics,
		}
		ctx := metadata.NewIncomingContext(
			t.Context(),
			metadata.Pairs("hostname", "hostname", "agent_id", "1337"),
		)
		_, _, err := grpc.RegisterAgent(ctx, rpc.AgentInfo{
			Version:  "version",
			Platform: "platform",
			Backend:  "backend",
			Capacity: 2,
		})
		assert.NoError(t, err)
	})
}

func TestUpdateAgentLastWork(t *testing.T) {
	t.Run("When last work was never updated it should update last work timestamp", func(t *testing.T) {
		agent := model.Agent{
			LastWork: 0,
		}
		store := new(store_mocks.MockStore)
		rpc := RPC{
			store: store,
		}
		store.On("AgentUpdate", mock.Anything).Once().Return(nil)

		err := rpc.updateAgentLastWork(&agent)
		assert.NoError(t, err)

		assert.NotZero(t, agent.LastWork)
	})

	t.Run("When last work was updated over a minute ago it should update last work timestamp", func(t *testing.T) {
		lastWork := time.Now().Add(-time.Hour).Unix()
		agent := model.Agent{
			LastWork: lastWork,
		}
		store := new(store_mocks.MockStore)
		rpc := RPC{
			store: store,
		}
		store.On("AgentUpdate", mock.Anything).Once().Return(nil)

		err := rpc.updateAgentLastWork(&agent)
		assert.NoError(t, err)

		assert.NotEqual(t, lastWork, agent.LastWork)
	})

	t.Run("When last work was updated in the last minute it should not update last work timestamp again", func(t *testing.T) {
		lastWork := time.Now().Add(-time.Second * 30).Unix()
		agent := model.Agent{
			LastWork: lastWork,
		}
		rpc := RPC{}

		err := rpc.updateAgentLastWork(&agent)
		assert.NoError(t, err)

		assert.Equal(t, lastWork, agent.LastWork)
	})
}

func TestRPC_Done_MissingWorkflow(t *testing.T) {
	t.Parallel()

	// Setup
	mockStore := new(store_mocks.MockStore)
	mockQueue := new(queue_mocks.MockQueue)
	mockLogger := logging.New()

	rpcServer := &RPC{
		queue:   mockQueue,
		pubsub:  pubsub.New(),
		logger:  mockLogger,
		store:   mockStore,
		metrics: newTestGRPCMetrics(),
	}

	// Create context with required metadata
	ctx := metadata.NewIncomingContext(
		t.Context(),
		metadata.Pairs("agent_id", "1"),
	)

	workflowID := "12345"
	state := rpc.WorkflowState{}

	// Mock store to return "workflow not found" error
	mockStore.On("WorkflowLoad", int64(12345)).Return(nil, types.RecordNotExist)

	// Mock queue to expect Done call for missing workflow
	mockQueue.On("Done", ctx, workflowID, model.StatusError).Return(nil)

	// Execute
	err := rpcServer.Done(ctx, workflowID, state)

	// Assert
	assert.Error(t, err)
	assert.True(t, errors.Is(err, types.RecordNotExist))
	mockStore.AssertExpectations(t)
	mockQueue.AssertExpectations(t)
}

func TestRPC_Done_MissingWorkflow_QueueError(t *testing.T) {
	t.Parallel()

	// Setup
	mockStore := new(store_mocks.MockStore)
	mockQueue := new(queue_mocks.MockQueue)
	mockLogger := logging.New()

	rpcServer := &RPC{
		queue:   mockQueue,
		pubsub:  pubsub.New(),
		logger:  mockLogger,
		store:   mockStore,
		metrics: newTestGRPCMetrics(),
	}

	// Create context with required metadata
	ctx := metadata.NewIncomingContext(
		t.Context(),
		metadata.Pairs("agent_id", "1"),
	)

	workflowID := "12345"
	state := rpc.WorkflowState{}

	// Mock store to return "workflow not found" error
	mockStore.On("WorkflowLoad", int64(12345)).Return(nil, types.RecordNotExist)

	// Mock queue to return error when trying to ack missing workflow
	queueErr := errors.New("queue error")
	mockQueue.On("Done", ctx, workflowID, model.StatusError).Return(queueErr)

	// Execute
	err := rpcServer.Done(ctx, workflowID, state)

	// Assert
	assert.Error(t, err)
	assert.True(t, errors.Is(err, types.RecordNotExist))
	mockStore.AssertExpectations(t)
	mockQueue.AssertExpectations(t)
}

func TestMigrateFilterLabels(t *testing.T) {
	t.Run("Should migrate all legacy labels to new format", func(t *testing.T) {
		filter := rpc.Filter{
			Labels: map[string]string{
				"repo":     "my-repo",
				"platform": "linux/amd64",
				"hostname": "my-host",
				"backend":  "docker",
				"org-id":   "123",
			},
		}

		result := migrateFilterLabels(filter)

		// Check that new labels exist
		assert.Equal(t, "my-repo", result.Labels["crowci/repo-full-name"])
		assert.Equal(t, "linux/amd64", result.Labels["platform"])
		assert.Equal(t, "my-host", result.Labels["hostname"])
		assert.Equal(t, "docker", result.Labels["backend"])
		assert.Equal(t, "123", result.Labels["crowci/org-id"])

		// Check that old labels are removed
		assert.NotContains(t, result.Labels, "repo")
		assert.NotContains(t, result.Labels, "org-id")
	})

	t.Run("Should preserve custom labels during migration", func(t *testing.T) {
		filter := rpc.Filter{
			Labels: map[string]string{
				"backend":    "docker",
				"platform":   "linux/arm64",
				"hostname":   "gaia",
				"repo":       "*",
				"agent-name": "gaia",
				"custom-key": "custom-value",
			},
		}

		result := migrateFilterLabels(filter)

		// Check custom labels are preserved
		assert.Equal(t, "gaia", result.Labels["agent-name"])
		assert.Equal(t, "custom-value", result.Labels["custom-key"])

		// Check standard labels are migrated
		assert.Equal(t, "docker", result.Labels["backend"])
		assert.Equal(t, "linux/arm64", result.Labels["platform"])
		assert.Equal(t, "gaia", result.Labels["hostname"])
		assert.Equal(t, "*", result.Labels["crowci/repo-full-name"])

		// Check old names are removed
		assert.NotContains(t, result.Labels, "repo")
	})

	t.Run("Should handle filter with no legacy labels", func(t *testing.T) {
		filter := rpc.Filter{
			Labels: map[string]string{
				"backend":    "kubernetes",
				"platform":   "linux/amd64",
				"hostname":   "k8s-agent",
				"custom-key": "value",
			},
		}

		result := migrateFilterLabels(filter)

		// All labels should remain unchanged
		assert.Equal(t, "kubernetes", result.Labels["backend"])
		assert.Equal(t, "linux/amd64", result.Labels["platform"])
		assert.Equal(t, "k8s-agent", result.Labels["hostname"])
		assert.Equal(t, "value", result.Labels["custom-key"])
	})
}

func TestFilterLabelsIntegrity(t *testing.T) {
	t.Run("Agent filter labels should preserve backend, platform, hostname, and custom labels after server enforcement", func(t *testing.T) {
		// Simulate agent sending filter with old label names and custom labels
		agentFilter := rpc.Filter{
			Labels: map[string]string{
				"backend":    "docker",
				"platform":   "linux/arm64",
				"hostname":   "gaia",
				"repo":       "*",
				"agent-name": "gaia", // custom label
			},
		}

		// Step 1: Migrate labels (as done in Next())
		agentFilter = migrateFilterLabels(agentFilter)

		// Verify migration worked correctly
		assert.Equal(t, "docker", agentFilter.Labels["backend"], "backend should be preserved after migration")
		assert.Equal(t, "linux/arm64", agentFilter.Labels["platform"], "platform should be preserved after migration")
		assert.Equal(t, "gaia", agentFilter.Labels["hostname"], "hostname should be preserved after migration")
		assert.Equal(t, "*", agentFilter.Labels["crowci/repo-full-name"], "repo should be migrated to crowci/repo-full-name")
		assert.Equal(t, "gaia", agentFilter.Labels["agent-name"], "custom label agent-name should be preserved after migration")
		assert.NotContains(t, agentFilter.Labels, "repo", "old repo label should be removed")

		// Step 2: Get server labels (as done in Next())
		agent := &model.Agent{
			OrgIDs: nil, // Global agent
		}
		serverLabels, err := agent.GetServerLabels()
		assert.NoError(t, err)

		// Step 3: Enforce server labels (as done in Next())
		for k, v := range serverLabels {
			agentFilter.Labels[k] = v
		}

		// Verify all labels are preserved after server enforcement
		assert.Equal(t, "docker", agentFilter.Labels["backend"], "backend should still be present after server label enforcement")
		assert.Equal(t, "linux/arm64", agentFilter.Labels["platform"], "platform should still be present after server label enforcement")
		assert.Equal(t, "gaia", agentFilter.Labels["hostname"], "hostname should still be present after server label enforcement")
		assert.Equal(t, "gaia", agentFilter.Labels["agent-name"], "custom label agent-name should still be present after server label enforcement")
		assert.Equal(t, "*", agentFilter.Labels["crowci/org-id"], "org-id should be set by server")
		assert.Equal(t, "*", agentFilter.Labels["crowci/repo-full-name"], "repo-full-name should still be present")

		// Verify we have all expected labels and no extras were removed
		assert.Len(t, agentFilter.Labels, 6, "should have exactly 6 labels: backend, platform, hostname, agent-name, crowci/org-id, crowci/repo-full-name")
	})

	t.Run("Organization agent should have org-id restriction while preserving other labels", func(t *testing.T) {
		agentFilter := rpc.Filter{
			Labels: map[string]string{
				"backend":    "kubernetes",
				"platform":   "linux/amd64",
				"hostname":   "k8s-agent",
				"repo":       "*",
				"agent-name": "production",
			},
		}

		// Migrate labels
		agentFilter = migrateFilterLabels(agentFilter)

		// Organization agent
		agent := &model.Agent{
			OrgIDs: model.JSONSliceInt64{42},
		}
		serverLabels, err := agent.GetServerLabels()
		assert.NoError(t, err)

		// Enforce server labels
		for k, v := range serverLabels {
			agentFilter.Labels[k] = v
		}

		// Verify org-id is enforced
		assert.Equal(t, "42", agentFilter.Labels["crowci/org-id"], "org-id should be enforced for organization agent")

		// Verify other labels are preserved
		assert.Equal(t, "kubernetes", agentFilter.Labels["backend"])
		assert.Equal(t, "linux/amd64", agentFilter.Labels["platform"])
		assert.Equal(t, "k8s-agent", agentFilter.Labels["hostname"])
		assert.Equal(t, "production", agentFilter.Labels["agent-name"])
	})
}

// TestRPC_Update_PublishesPipelineWithUpdatedStepState is the regression test
// for the "step state stuck pending" UI bug: when an agent reports a step has
// started, the gRPC Update handler must publish a pubsub message carrying the
// freshly-reloaded pipeline tree (with the step now in Running state) so the
// SSE-subscribed UI flips out of the stale pending payload it received from
// the initial GET /pipelines/N response.
//
// If a future refactor drops the publish, or stops reloading the workflow
// tree after persisting the step transition, the UI silently stops receiving
// step-progress updates until the workflow finishes — exactly the bug this
// guards against.
func TestRPC_Update_PublishesPipelineWithUpdatedStepState(t *testing.T) {
	t.Parallel()

	mockStore := new(store_mocks.MockStore)
	broker := pubsub.New()

	rpcServer := &RPC{
		pubsub:  broker,
		logger:  logging.New(),
		store:   mockStore,
		metrics: newTestGRPCMetrics(),
	}

	const (
		agentID    int64 = 1
		workflowID int64 = 42
		pipelineID int64 = 99
		repoID     int64 = 7
		stepUUID         = "step-uuid-1"
	)

	// Subscribe before invoking Update so we capture the publish.
	subCtx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)

	var (
		mu       sync.Mutex
		received []pubsub.Message
	)
	subscribed := make(chan struct{})
	go func() {
		close(subscribed)
		broker.Subscribe(subCtx, func(m pubsub.Message) {
			mu.Lock()
			defer mu.Unlock()
			received = append(received, m)
		})
	}()
	<-subscribed
	// Give the goroutine a tick to register with the broker before publishing.
	time.Sleep(50 * time.Millisecond)

	// Fixture: agent allowed (global agent), pipeline running, workflow
	// running, step starts running mid-execution.
	agent := &model.Agent{ID: agentID}
	repo := &model.Repo{ID: repoID, FullName: "owner/repo"}
	pipeline := &model.Pipeline{ID: pipelineID, RepoID: repoID, Status: model.StatusRunning}
	workflow := &model.Workflow{ID: workflowID, PipelineID: pipelineID, State: model.StatusRunning}
	// Step row as it stands in the DB before the agent reports Started.
	stepBefore := &model.Step{
		ID: 5, UUID: stepUUID, PipelineID: pipelineID,
		State: model.StatusPending,
	}
	// Workflow tree the publish should observe — i.e., AFTER UpdateStepStatus
	// has persisted the Running state. This is the load that the handler does
	// via WorkflowGetTree right before constructing the pubsub message.
	stepAfter := &model.Step{
		ID: 5, UUID: stepUUID, PipelineID: pipelineID,
		State: model.StatusRunning, Started: 100,
	}
	wfTree := []*model.Workflow{{
		ID: workflowID, PipelineID: pipelineID, State: model.StatusRunning,
		Children: []*model.Step{stepAfter},
	}}

	mockStore.On("WorkflowLoad", workflowID).Return(workflow, nil)
	mockStore.On("GetPipeline", pipelineID).Return(pipeline, nil)
	mockStore.On("AgentFind", agentID).Return(agent, nil)
	mockStore.On("StepByUUID", stepUUID).Return(stepBefore, nil)
	mockStore.On("GetRepo", repoID).Return(repo, nil)
	// UpdateStepStatus persists the transition; the matcher proves we wrote
	// Running back to the store (the input the publish reads via the tree).
	mockStore.On("StepUpdate", mock.MatchedBy(func(s *model.Step) bool {
		return s.UUID == stepUUID && s.State == model.StatusRunning && s.Started == 100
	})).Return(nil)
	mockStore.On("WorkflowGetTree", pipeline).Return(wfTree, nil)

	ctx := metadata.NewIncomingContext(
		t.Context(),
		metadata.Pairs("agent_id", "1"),
	)

	err := rpcServer.Update(ctx, "42", rpc.StepState{
		StepUUID: stepUUID,
		Started:  100,
		Exited:   false,
	})
	assert.NoError(t, err)

	// pubsub delivery is async (broker.Publish spawns a goroutine per
	// subscriber); wait until the receiver captures at least one message.
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		got := len(received)
		mu.Unlock()
		if got >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Update handler never published to pubsub — UI would stay stuck on stale step state")
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	msg := received[0]
	assert.Equal(t, "owner/repo", msg.Labels["repo"])

	var ev model.Event
	if assert.NoError(t, json.Unmarshal(msg.Data, &ev)) {
		assert.Equal(t, pipelineID, ev.Pipeline.ID)
		// The freshly-reloaded tree must be in the payload — that's the whole
		// point of WorkflowGetTree being called between the step update and
		// the publish. Without it, the UI sees a Pipeline with no Workflows.
		if assert.Len(t, ev.Pipeline.Workflows, 1) {
			wf := ev.Pipeline.Workflows[0]
			if assert.Len(t, wf.Children, 1) {
				step := wf.Children[0]
				assert.Equal(t, model.StatusRunning, step.State,
					"published pipeline must carry the post-update step state — "+
						"if this is Pending the UI sits frozen on the stale value")
			}
		}
	}

	mockStore.AssertExpectations(t)
}
