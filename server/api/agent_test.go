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

package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/queue"
	queue_mocks "codefloe.com/crowci/crow/v5/server/queue/mocks"
	mocks_manager "codefloe.com/crowci/crow/v5/server/services/mocks"
	store_mocks "codefloe.com/crowci/crow/v5/server/store/mocks"
	"codefloe.com/crowci/crow/v5/server/store/types"
)

var fakeAgent = &model.Agent{
	ID:         1,
	Name:       "test-agent",
	OwnerID:    1,
	NoSchedule: false,
}

// testCtx creates a context.Context that embeds the gin context,
// as expected by the ginCtx() helper used in huma handlers.
func testCtx(c *gin.Context) context.Context {
	if c.Request == nil {
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	}
	return context.WithValue(c.Request.Context(), GinContextKey(), c)
}

func TestGetAgents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("should get agents", func(t *testing.T) {
		agents := []*model.Agent{fakeAgent}

		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentList", mock.Anything).Return(agents, nil)

		mockQueue := new(queue_mocks.MockQueue)
		mockQueue.On("Info", mock.Anything).Return(queue.InfoT{})
		server.Config.Services.Queue = mockQueue

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

		output, err := GetAgents(testCtx(c), &GetAgentsInput{PaginationParams: PaginationParams{Page: 1, PerPage: 50}})

		mockStore.AssertCalled(t, "AgentList", mock.Anything)
		mockQueue.AssertCalled(t, "Info", mock.Anything)
		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Len(t, output.Body, 1)
		assert.Equal(t, fakeAgent.ID, output.Body[0].ID)
		assert.Equal(t, fakeAgent.Name, output.Body[0].Name)
		assert.Equal(t, 0, output.Body[0].RunningWorkflows)
	})
}

func TestGetAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("should get agent", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentFind", int64(1)).Return(fakeAgent, nil)

		mockQueue := new(queue_mocks.MockQueue)
		mockQueue.On("Info", mock.Anything).Return(queue.InfoT{})
		server.Config.Services.Queue = mockQueue

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

		output, err := GetAgent(testCtx(c), &GetAgentInput{AgentID: 1})

		mockStore.AssertCalled(t, "AgentFind", int64(1))
		mockQueue.AssertCalled(t, "Info", mock.Anything)
		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, fakeAgent.ID, output.Body.ID)
		assert.Equal(t, fakeAgent.Name, output.Body.Name)
		assert.Equal(t, 0, output.Body.RunningWorkflows)
	})

	t.Run("should return not found for non-existent agent", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentFind", int64(2)).Return((*model.Agent)(nil), types.RecordNotExist)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

		output, err := GetAgent(testCtx(c), &GetAgentInput{AgentID: 2})

		assert.Nil(t, output)
		assert.Error(t, err)
		var humaErr huma.StatusError
		if errors.As(err, &humaErr) {
			assert.Equal(t, http.StatusNotFound, humaErr.GetStatus())
		}
	})
}

func TestPatchAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("should update agent", func(t *testing.T) {
		updatedAgent := *fakeAgent
		updatedAgent.Name = "updated-agent"

		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentFind", int64(1)).Return(fakeAgent, nil)
		mockStore.On("AgentUpdate", mock.AnythingOfType("*model.Agent")).Return(nil)

		mockManager := new(mocks_manager.MockManager)
		server.Config.Services.Manager = mockManager

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Request, _ = http.NewRequest(http.MethodPatch, "/", nil)
		c.Request.Header.Set("Content-Type", "application/json")

		output, err := PatchAgent(testCtx(c), &PatchAgentInput{
			AgentID: 1,
			Body:    PatchAgentBody{Name: "updated-agent"},
		})

		mockStore.AssertCalled(t, "AgentFind", int64(1))
		mockStore.AssertCalled(t, "AgentUpdate", mock.AnythingOfType("*model.Agent"))
		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, "updated-agent", output.Body.Name)
	})
}

func TestPostAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("should create agent", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentCreate", mock.AnythingOfType("*model.Agent")).Return(nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Set("user", &model.User{ID: 1})
		c.Request, _ = http.NewRequest(http.MethodPost, "/", nil)
		c.Request.Header.Set("Content-Type", "application/json")

		output, err := PostAgent(testCtx(c), &PostAgentInput{
			Body: PostAgentBody{Name: "new-agent"},
		})

		mockStore.AssertCalled(t, "AgentCreate", mock.AnythingOfType("*model.Agent"))
		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, "new-agent", output.Body.Name)
		assert.NotEmpty(t, output.Body.Token)
	})
}

func TestDeleteAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("should delete agent", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentFind", int64(1)).Return(fakeAgent, nil)
		mockStore.On("AgentDelete", mock.AnythingOfType("*model.Agent")).Return(nil)

		mockManager := new(mocks_manager.MockManager)
		server.Config.Services.Manager = mockManager

		mockQueue := new(queue_mocks.MockQueue)
		mockQueue.On("Info", mock.Anything).Return(queue.InfoT{})
		mockQueue.On("KickAgentWorkers", int64(1)).Return()
		server.Config.Services.Queue = mockQueue

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Request, _ = http.NewRequest(http.MethodDelete, "/", nil)

		_, err := DeleteAgent(testCtx(c), &DeleteAgentInput{AgentID: 1})

		mockStore.AssertCalled(t, "AgentFind", int64(1))
		mockStore.AssertCalled(t, "AgentDelete", mock.AnythingOfType("*model.Agent"))
		mockQueue.AssertCalled(t, "KickAgentWorkers", int64(1))
		assert.NoError(t, err)
	})

	t.Run("should not delete agent with running tasks", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentFind", int64(1)).Return(fakeAgent, nil)

		mockManager := new(mocks_manager.MockManager)
		server.Config.Services.Manager = mockManager

		mockQueue := new(queue_mocks.MockQueue)
		mockQueue.On("Info", mock.Anything).Return(queue.InfoT{
			Running: []*model.Task{{AgentID: 1}},
		})
		server.Config.Services.Queue = mockQueue

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Request, _ = http.NewRequest(http.MethodDelete, "/", nil)

		_, err := DeleteAgent(testCtx(c), &DeleteAgentInput{AgentID: 1})

		mockStore.AssertCalled(t, "AgentFind", int64(1))
		mockStore.AssertNotCalled(t, "AgentDelete", mock.Anything)
		assert.Error(t, err)
		var humaErr huma.StatusError
		if errors.As(err, &humaErr) {
			assert.Equal(t, http.StatusConflict, humaErr.GetStatus())
		}
	})
}

func TestPostOrgAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("create org agent should succeed", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentCreate", mock.AnythingOfType("*model.Agent")).Return(nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)

		// Set up a non-admin user
		c.Set("user", &model.User{
			ID:    1,
			Admin: false,
		})

		c.Request, _ = http.NewRequest(http.MethodPost, "/", nil)
		c.Request.Header.Set("Content-Type", "application/json")

		output, err := PostOrgAgent(testCtx(c), &PostOrgAgentInput{
			OrgID: 1,
			Body:  PostAgentBody{Name: "new-agent"},
		})

		assert.NoError(t, err)
		assert.NotNil(t, output)

		// Ensure an agent was created
		mockStore.AssertCalled(t, "AgentCreate", mock.AnythingOfType("*model.Agent"))
	})
}
