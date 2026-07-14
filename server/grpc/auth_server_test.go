// Copyright 2024 Woodpecker Authors
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
	"google.golang.org/grpc/metadata"

	"codefloe.com/crowci/crow/v6/pipeline/rpc/proto"
	"codefloe.com/crowci/crow/v6/server/model"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

const testMasterToken = "master-secret"

func newTestAuthServer(t *testing.T) (*CrowAuthServer, *store_mocks.MockStore) {
	t.Helper()
	store := store_mocks.NewMockStore(t)
	server := NewCrowAuthServer(NewJWTManager("jwt-secret"), testMasterToken, store)
	return server, store
}

func authCtx(hostname string) context.Context {
	if hostname == "" {
		return context.Background()
	}
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("hostname", hostname))
}

// A fresh master-token agent (id == -1) with a known hostname re-binds to its
// existing system row instead of creating a new orphaned one.
func TestAuthMasterTokenRebindsByHostname(t *testing.T) {
	server, store := newTestAuthServer(t)

	existing := &model.Agent{ID: 7, OwnerID: model.IDNotSet, Token: testMasterToken, Hostname: "runner-1"}
	store.On("AgentFindSystemByHostname", "runner-1").Return(existing, nil).Once()

	res, err := server.Auth(authCtx("runner-1"), &proto.AuthRequest{AgentToken: testMasterToken, AgentId: -1})
	assert.NoError(t, err)
	assert.Equal(t, int64(7), res.AgentId)
	store.AssertNotCalled(t, "AgentCreate", mock.Anything)
}

// Without a matching system row for the hostname, a new system agent is created.
func TestAuthMasterTokenCreatesWhenNoHostnameMatch(t *testing.T) {
	server, store := newTestAuthServer(t)

	store.On("AgentFindSystemByHostname", "runner-1").Return(nil, types.RecordNotExist).Once()
	store.On("AgentCreate", mock.Anything).Run(func(args mock.Arguments) {
		agent, ok := args.Get(0).(*model.Agent)
		assert.True(t, ok)
		agent.ID = 42
	}).Return(nil).Once()

	res, err := server.Auth(authCtx("runner-1"), &proto.AuthRequest{AgentToken: testMasterToken, AgentId: -1})
	assert.NoError(t, err)
	assert.Equal(t, int64(42), res.AgentId)
}

// When no hostname is supplied the lookup is skipped and a new agent is created,
// preserving the previous behavior for agents that do not report a hostname.
func TestAuthMasterTokenCreatesWhenNoHostname(t *testing.T) {
	server, store := newTestAuthServer(t)

	store.On("AgentCreate", mock.Anything).Run(func(args mock.Arguments) {
		agent, ok := args.Get(0).(*model.Agent)
		assert.True(t, ok)
		agent.ID = 99
	}).Return(nil).Once()

	res, err := server.Auth(authCtx(""), &proto.AuthRequest{AgentToken: testMasterToken, AgentId: -1})
	assert.NoError(t, err)
	assert.Equal(t, int64(99), res.AgentId)
	store.AssertNotCalled(t, "AgentFindSystemByHostname", mock.Anything)
}

// A master-token agent presenting a known ID binds to that existing row.
func TestAuthMasterTokenFindsByID(t *testing.T) {
	server, store := newTestAuthServer(t)

	existing := &model.Agent{ID: 3, OwnerID: model.IDNotSet, Token: testMasterToken, Hostname: "runner-1"}
	store.On("AgentFind", int64(3)).Return(existing, nil).Once()

	res, err := server.Auth(authCtx("runner-1"), &proto.AuthRequest{AgentToken: testMasterToken, AgentId: 3})
	assert.NoError(t, err)
	assert.Equal(t, int64(3), res.AgentId)
}

// A stale persisted ID falls back to hostname re-binding rather than failing.
func TestAuthMasterTokenStaleIDRebindsByHostname(t *testing.T) {
	server, store := newTestAuthServer(t)

	existing := &model.Agent{ID: 8, OwnerID: model.IDNotSet, Token: testMasterToken, Hostname: "runner-1"}
	store.On("AgentFind", int64(123)).Return(nil, types.RecordNotExist).Once()
	store.On("AgentFindSystemByHostname", "runner-1").Return(existing, nil).Once()

	res, err := server.Auth(authCtx("runner-1"), &proto.AuthRequest{AgentToken: testMasterToken, AgentId: 123})
	assert.NoError(t, err)
	assert.Equal(t, int64(8), res.AgentId)
	store.AssertNotCalled(t, "AgentCreate", mock.Anything)
}

// A per-agent token binds to its existing row by token and never creates a
// duplicate system agent, regardless of the hostname sent.
func TestAuthPerAgentTokenBindsByToken(t *testing.T) {
	server, store := newTestAuthServer(t)

	existing := &model.Agent{ID: 5, OwnerID: 2, Token: "per-agent-token", Hostname: "runner-1"}
	store.On("AgentFindByToken", "per-agent-token").Return(existing, nil).Once()

	res, err := server.Auth(authCtx("runner-1"), &proto.AuthRequest{AgentToken: "per-agent-token", AgentId: -1})
	assert.NoError(t, err)
	assert.Equal(t, int64(5), res.AgentId)
	store.AssertNotCalled(t, "AgentCreate", mock.Anything)
	store.AssertNotCalled(t, "AgentFindSystemByHostname", mock.Anything)
}
