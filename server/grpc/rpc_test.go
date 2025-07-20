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
