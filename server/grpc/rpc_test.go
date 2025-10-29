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
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/metadata"

	"codeberg.org/crowci/crow/v4/pipeline/rpc"
	"codeberg.org/crowci/crow/v4/server/logging"
	"codeberg.org/crowci/crow/v4/server/model"
	"codeberg.org/crowci/crow/v4/server/pubsub"
	queue_mocks "codeberg.org/crowci/crow/v4/server/queue/mocks"
	store_mocks "codeberg.org/crowci/crow/v4/server/store/mocks"
	"codeberg.org/crowci/crow/v4/server/store/types"
)

func TestRegisterAgent(t *testing.T) {
	t.Run("When existing agent Name is empty it should update Name with hostname from metadata", func(t *testing.T) {
		store := new(store_mocks.MockStore)
		storeAgent := new(model.Agent)
		storeAgent.ID = 1337
		updatedAgent := model.Agent{
			ID:          1337,
			Created:     0,
			Updated:     0,
			Name:        "hostname",
			OwnerID:     0,
			Token:       "",
			LastContact: 0,
			Platform:    "platform",
			Backend:     "backend",
			Capacity:    2,
			Version:     "version",
			NoSchedule:  false,
		}

		store.On("AgentFind", int64(1337)).Once().Return(storeAgent, nil)
		store.On("AgentUpdate", &updatedAgent).Once().Return(nil)
		grpc := RPC{
			store: store,
		}
		ctx := metadata.NewIncomingContext(
			t.Context(),
			metadata.Pairs("hostname", "hostname", "agent_id", "1337"),
		)
		agentID, err := grpc.RegisterAgent(ctx, rpc.AgentInfo{
			Version:  "version",
			Platform: "platform",
			Backend:  "backend",
			Capacity: 2,
		})
		if !assert.NoError(t, err) {
			return
		}

		assert.EqualValues(t, 1337, agentID)
	})

	t.Run("When existing agent hostname is present it should not update the hostname", func(t *testing.T) {
		store := new(store_mocks.MockStore)
		storeAgent := new(model.Agent)
		storeAgent.ID = 1337
		storeAgent.Name = "originalHostname"
		updatedAgent := model.Agent{
			ID:          1337,
			Created:     0,
			Updated:     0,
			Name:        "originalHostname",
			OwnerID:     0,
			Token:       "",
			LastContact: 0,
			Platform:    "platform",
			Backend:     "backend",
			Capacity:    2,
			Version:     "version",
			NoSchedule:  false,
		}

		store.On("AgentFind", int64(1337)).Once().Return(storeAgent, nil)
		store.On("AgentUpdate", &updatedAgent).Once().Return(nil)
		grpc := RPC{
			store: store,
		}
		ctx := metadata.NewIncomingContext(
			t.Context(),
			metadata.Pairs("hostname", "newHostname", "agent_id", "1337"),
		)
		agentID, err := grpc.RegisterAgent(ctx, rpc.AgentInfo{
			Version:  "version",
			Platform: "platform",
			Backend:  "backend",
			Capacity: 2,
		})
		if !assert.NoError(t, err) {
			return
		}

		assert.EqualValues(t, 1337, agentID)
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
		queue:         mockQueue,
		pubsub:        pubsub.New(),
		logger:        mockLogger,
		store:         mockStore,
		pipelineTime:  prometheus.NewGaugeVec(prometheus.GaugeOpts{}, []string{"repo", "branch", "status", "type"}),
		pipelineCount: prometheus.NewCounterVec(prometheus.CounterOpts{}, []string{"repo", "branch", "status", "type"}),
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
		queue:         mockQueue,
		pubsub:        pubsub.New(),
		logger:        mockLogger,
		store:         mockStore,
		pipelineTime:  prometheus.NewGaugeVec(prometheus.GaugeOpts{}, []string{"repo", "branch", "status", "type"}),
		pipelineCount: prometheus.NewCounterVec(prometheus.CounterOpts{}, []string{"repo", "branch", "status", "type"}),
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
			OrgID: model.IDNotSet, // Global agent
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
			OrgID: 42,
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
