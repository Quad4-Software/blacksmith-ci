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

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/queue"
	queue_mocks "codefloe.com/crowci/crow/v6/server/queue/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

const secretAgentToken = "SECRET-AGENT-TOKEN-DO-NOT-LEAK"

func agentWithSecretToken() *model.Agent {
	return &model.Agent{
		ID:      7,
		Name:    "token-agent",
		OwnerID: 1,
		OrgIDs:  model.JSONSliceInt64{5},
		Token:   secretAgentToken,
	}
}

func newAgentTestContext(t *testing.T, method string, mockStore *store_mocks.MockStore) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("store", mockStore)
	c.Set("user", &model.User{ID: 1})
	c.Set("org", &model.Org{ID: 5})
	c.Request, _ = http.NewRequest(method, "/", nil)
	return c
}

func mockAgentQueue() {
	mockQueue := new(queue_mocks.MockQueue)
	mockQueue.On("Info", mock.Anything).Return(queue.InfoT{})
	mockQueue.On("KickAgentWorkers", mock.Anything).Return()
	server.Config.Services.Queue = mockQueue
}

// assertNoAgentToken marshals a handler response body the way huma does and
// asserts that neither a "token" key nor the token value is present.
func assertNoAgentToken(t *testing.T, body any) {
	t.Helper()
	data, err := json.Marshal(body)
	require.NoError(t, err)
	assert.NotContains(t, string(data), secretAgentToken)
	assert.NotContains(t, string(data), `"token"`)
}

func TestAgentTokenNotExposed(t *testing.T) {
	t.Run("GET /agents", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentList", mock.Anything).Return([]*model.Agent{agentWithSecretToken()}, nil)
		mockAgentQueue()

		output, err := GetAgents(testCtx(newAgentTestContext(t, http.MethodGet, mockStore)), &GetAgentsInput{PaginationParams: PaginationParams{Page: 1, PerPage: 50}})
		require.NoError(t, err)
		require.Len(t, output.Body, 1)
		assertNoAgentToken(t, output.Body)
	})

	t.Run("GET /agents/{id}", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentFind", int64(7)).Return(agentWithSecretToken(), nil)
		mockAgentQueue()

		output, err := GetAgent(testCtx(newAgentTestContext(t, http.MethodGet, mockStore)), &GetAgentInput{AgentID: 7})
		require.NoError(t, err)
		assertNoAgentToken(t, output.Body)
	})

	t.Run("PATCH /agents/{id}", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentFind", int64(7)).Return(agentWithSecretToken(), nil)
		mockStore.On("AgentUpdate", mock.AnythingOfType("*model.Agent")).Return(nil)
		mockAgentQueue()

		output, err := PatchAgent(testCtx(newAgentTestContext(t, http.MethodPatch, mockStore)), &PatchAgentInput{
			AgentID: 7,
			Body:    PatchAgentBody{Name: "renamed"},
		})
		require.NoError(t, err)
		assertNoAgentToken(t, output.Body)
	})

	t.Run("GET /orgs/{id}/agents", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentListForOrg", int64(5), mock.Anything).Return([]*model.Agent{agentWithSecretToken()}, nil)
		mockAgentQueue()

		output, err := GetOrgAgents(testCtx(newAgentTestContext(t, http.MethodGet, mockStore)), &GetOrgAgentsInput{OrgID: 5})
		require.NoError(t, err)
		require.Len(t, output.Body, 1)
		assertNoAgentToken(t, output.Body)
	})

	t.Run("PATCH /orgs/{id}/agents/{id}", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentFind", int64(7)).Return(agentWithSecretToken(), nil)
		mockStore.On("AgentUpdate", mock.AnythingOfType("*model.Agent")).Return(nil)
		mockAgentQueue()

		output, err := PatchOrgAgent(testCtx(newAgentTestContext(t, http.MethodPatch, mockStore)), &PatchOrgAgentInput{
			OrgID:   5,
			AgentID: 7,
			Body:    PatchAgentBody{Name: "renamed"},
		})
		require.NoError(t, err)
		assertNoAgentToken(t, output.Body)
	})
}

func TestAgentTokenReturnedOnCreate(t *testing.T) {
	assertCreateResponseHasToken := func(t *testing.T, body *model.AgentWithToken) {
		t.Helper()
		require.NotNil(t, body)
		require.NotEmpty(t, body.Token)

		data, err := json.Marshal(body)
		require.NoError(t, err)
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(data, &decoded))
		assert.Equal(t, body.Token, decoded["token"])
		assert.Equal(t, body.Name, decoded["name"])
	}

	t.Run("POST /agents", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentCreate", mock.AnythingOfType("*model.Agent")).Return(nil)

		output, err := PostAgent(testCtx(newAgentTestContext(t, http.MethodPost, mockStore)), &PostAgentInput{
			Body: PostAgentBody{Name: "new-agent"},
		})
		require.NoError(t, err)
		assertCreateResponseHasToken(t, output.Body)
	})

	t.Run("POST /orgs/{id}/agents", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentCreate", mock.AnythingOfType("*model.Agent")).Return(nil)

		output, err := PostOrgAgent(testCtx(newAgentTestContext(t, http.MethodPost, mockStore)), &PostOrgAgentInput{
			OrgID: 5,
			Body:  PostAgentBody{Name: "new-org-agent"},
		})
		require.NoError(t, err)
		assertCreateResponseHasToken(t, output.Body)
	})
}
