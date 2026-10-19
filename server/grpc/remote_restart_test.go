// Copyright 2026 Crow Authors
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"codefloe.com/crowci/crow/v6/pipeline/rpc"
	"codefloe.com/crowci/crow/v6/server/model"
	queue_mocks "codefloe.com/crowci/crow/v6/server/queue/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func agentCtx(t *testing.T) context.Context {
	return metadata.NewIncomingContext(t.Context(), metadata.Pairs("hostname", "host", "agent_id", "1"))
}

func TestNextHoldsBackWorkWhileDrainingForRestart(t *testing.T) {
	db := store_mocks.NewMockStore(t)
	q := queue_mocks.NewMockQueue(t)
	db.On("AgentFind", int64(1)).Return(&model.Agent{ID: 1, RestartRequested: true}, nil)

	s := RPC{store: db, queue: q}
	wf, err := s.Next(agentCtx(t), rpc.Filter{Labels: map[string]string{}})

	require.NoError(t, err)
	assert.Nil(t, wf)
	q.AssertNotCalled(t, "Poll", mock.Anything, mock.Anything, mock.Anything)
}

func TestReportHealthForwardsRestartRequest(t *testing.T) {
	for _, requested := range []bool{false, true} {
		db := store_mocks.NewMockStore(t)
		db.On("AgentFind", int64(1)).Return(&model.Agent{ID: 1, Backend: "local", RestartRequested: requested}, nil)
		db.On("AgentUpdate", mock.Anything).Return(nil)

		s := RPC{store: db}
		health, err := s.ReportHealth(agentCtx(t), "I am alive!", true)

		require.NoError(t, err)
		assert.Equal(t, requested, health.RestartRequested)
		assert.Nil(t, health.DockerCleanup)
	}
}

func TestRegisterAgentAfterRequestedRestart(t *testing.T) {
	db := store_mocks.NewMockStore(t)
	db.On("AgentFind", int64(1)).Return(&model.Agent{ID: 1, RestartRequested: true}, nil)
	db.On("AgentUpdate", mock.MatchedBy(func(a *model.Agent) bool {
		return a.RemoteUpdate == rpc.RemoteUpdateRestart
	})).Return(nil)
	db.On("AgentSetRestartRequested", int64(1), false).Return(nil)

	s := RPC{store: db, metrics: newTestGRPCMetrics()}
	_, _, err := s.RegisterAgent(agentCtx(t), rpc.AgentInfo{Version: "6.13.0", RemoteUpdate: rpc.RemoteUpdateRestart})

	require.NoError(t, err)
	db.AssertCalled(t, "AgentSetRestartRequested", int64(1), false)
}

func TestRegisterAgentDropsQueuedRestart(t *testing.T) {
	db := store_mocks.NewMockStore(t)
	db.On("AgentFind", int64(1)).Return(&model.Agent{ID: 1, RestartQueued: true}, nil)
	db.On("AgentUpdate", mock.Anything).Return(nil)
	db.On("AgentSetRestartRequested", int64(1), false).Return(nil)

	s := RPC{store: db, metrics: newTestGRPCMetrics()}
	_, _, err := s.RegisterAgent(agentCtx(t), rpc.AgentInfo{Version: "6.13.0", RemoteUpdate: rpc.RemoteUpdateRestart})

	require.NoError(t, err)
	db.AssertCalled(t, "AgentSetRestartRequested", int64(1), false)
}

func TestRegisterAgentDropsUnknownRemoteUpdateModes(t *testing.T) {
	db := store_mocks.NewMockStore(t)
	db.On("AgentFind", int64(1)).Return(&model.Agent{ID: 1, RemoteUpdate: rpc.RemoteUpdateRestart}, nil)
	db.On("AgentUpdate", mock.MatchedBy(func(a *model.Agent) bool { return a.RemoteUpdate == "" })).Return(nil)

	s := RPC{store: db, metrics: newTestGRPCMetrics()}
	_, _, err := s.RegisterAgent(agentCtx(t), rpc.AgentInfo{RemoteUpdate: "self"})

	require.NoError(t, err)
}
