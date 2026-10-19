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
	"slices"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/pipeline"
	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/queue"
	queue_mocks "codefloe.com/crowci/crow/v6/server/queue/mocks"
	mocks_manager "codefloe.com/crowci/crow/v6/server/services/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
	"codefloe.com/crowci/crow/v6/server/store/types"
	"codefloe.com/crowci/crow/v6/version"
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

func TestPostAgentScopedToOrgs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("org_ids [5,7] sets OrgIDs on created agent", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("OrgGet", int64(5)).Return(&model.Org{ID: 5}, nil)
		mockStore.On("OrgGet", int64(7)).Return(&model.Org{ID: 7}, nil)
		mockStore.On("AgentCreate", mock.AnythingOfType("*model.Agent")).Return(nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Set("user", &model.User{ID: 1})
		c.Request, _ = http.NewRequest(http.MethodPost, "/", nil)

		output, err := PostAgent(testCtx(c), &PostAgentInput{
			Body: PostAgentBody{Name: "a", OrgIDs: []int64{5, 7}},
		})

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, model.JSONSliceInt64{5, 7}, output.Body.OrgIDs)
		mockStore.AssertCalled(t, "OrgGet", int64(5))
		mockStore.AssertCalled(t, "OrgGet", int64(7))
		mockStore.AssertCalled(t, "AgentCreate", mock.AnythingOfType("*model.Agent"))
	})

	t.Run("no org_ids creates global agent", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentCreate", mock.AnythingOfType("*model.Agent")).Return(nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Set("user", &model.User{ID: 1})
		c.Request, _ = http.NewRequest(http.MethodPost, "/", nil)

		output, err := PostAgent(testCtx(c), &PostAgentInput{
			Body: PostAgentBody{Name: "b"},
		})

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Empty(t, output.Body.OrgIDs)
	})

	t.Run("unknown org id returns 400", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("OrgGet", int64(999)).Return((*model.Org)(nil), errors.New("not found"))

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Set("user", &model.User{ID: 1})
		c.Request, _ = http.NewRequest(http.MethodPost, "/", nil)

		output, err := PostAgent(testCtx(c), &PostAgentInput{
			Body: PostAgentBody{Name: "c", OrgIDs: []int64{999}},
		})

		assert.Nil(t, output)
		assert.Error(t, err)
		var humaErr huma.StatusError
		if errors.As(err, &humaErr) {
			assert.Equal(t, http.StatusBadRequest, humaErr.GetStatus())
		}
	})
}

func TestPatchOrgAgentDoesNotChangeOrgIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("org admin can patch multi-org agent name without changing OrgIDs", func(t *testing.T) {
		// Agent belongs to orgs 5 and 7; org admin for org 5 patches name.
		agentMultiOrg := &model.Agent{
			ID:      42,
			Name:    "old-name",
			OwnerID: 1,
			OrgIDs:  model.JSONSliceInt64{5, 7},
		}

		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentFind", int64(42)).Return(agentMultiOrg, nil)
		mockStore.On("AgentUpdate", mock.AnythingOfType("*model.Agent")).Return(nil)

		mockQueue := new(queue_mocks.MockQueue)
		mockQueue.On("Info", mock.Anything).Return(queue.InfoT{})
		server.Config.Services.Queue = mockQueue

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Set("org", &model.Org{ID: 5})
		c.Request, _ = http.NewRequest(http.MethodPatch, "/", nil)

		output, err := PatchOrgAgent(testCtx(c), &PatchOrgAgentInput{
			OrgID:   5,
			AgentID: 42,
			Body:    PatchAgentBody{Name: "new-name"},
		})

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, "new-name", output.Body.Name)
		// OrgIDs must remain unchanged — org patch must not modify scope.
		assert.Equal(t, model.JSONSliceInt64{5, 7}, output.Body.OrgIDs)
	})
}

func TestDeleteOrgAgentDescopes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("agent scoped to [5,7]: delete from org 5 keeps agent with OrgIDs==[7]", func(t *testing.T) {
		agentTwoOrgs := &model.Agent{
			ID:     10,
			Name:   "multi-org-agent",
			OrgIDs: model.JSONSliceInt64{5, 7},
		}

		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentFind", int64(10)).Return(agentTwoOrgs, nil)
		mockStore.On("AgentUpdate", mock.AnythingOfType("*model.Agent")).Return(nil)

		mockQueue := new(queue_mocks.MockQueue)
		mockQueue.On("Info", mock.Anything).Return(queue.InfoT{})
		server.Config.Services.Queue = mockQueue

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Set("org", &model.Org{ID: 5})
		c.Request, _ = http.NewRequest(http.MethodDelete, "/", nil)

		_, err := DeleteOrgAgent(testCtx(c), &DeleteOrgAgentInput{OrgID: 5, AgentID: 10})

		assert.NoError(t, err)
		mockStore.AssertCalled(t, "AgentUpdate", mock.AnythingOfType("*model.Agent"))
		mockStore.AssertNotCalled(t, "AgentDelete", mock.Anything)
		// Verify remaining OrgIDs on the updated agent.
		updateCall := mockStore.Calls[1]
		updatedAgent, ok := updateCall.Arguments.Get(0).(*model.Agent)
		require.True(t, ok, "expected *model.Agent argument to AgentUpdate")
		assert.Equal(t, model.JSONSliceInt64{7}, updatedAgent.OrgIDs)
	})

	t.Run("agent scoped to [5]: delete from org 5 deletes agent", func(t *testing.T) {
		agentOneOrg := &model.Agent{
			ID:     11,
			Name:   "sole-org-agent",
			OrgIDs: model.JSONSliceInt64{5},
		}

		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentFind", int64(11)).Return(agentOneOrg, nil)
		mockStore.On("AgentDelete", mock.AnythingOfType("*model.Agent")).Return(nil)

		mockQueue := new(queue_mocks.MockQueue)
		mockQueue.On("Info", mock.Anything).Return(queue.InfoT{})
		mockQueue.On("KickAgentWorkers", int64(11)).Return()
		server.Config.Services.Queue = mockQueue

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Set("org", &model.Org{ID: 5})
		c.Request, _ = http.NewRequest(http.MethodDelete, "/", nil)

		_, err := DeleteOrgAgent(testCtx(c), &DeleteOrgAgentInput{OrgID: 5, AgentID: 11})

		assert.NoError(t, err)
		mockStore.AssertCalled(t, "AgentDelete", mock.AnythingOfType("*model.Agent"))
		mockQueue.AssertCalled(t, "KickAgentWorkers", int64(11))
	})

	t.Run("agent scoped to [7]: delete from org 5 returns 400", func(t *testing.T) {
		agentOtherOrg := &model.Agent{
			ID:     12,
			Name:   "other-org-agent",
			OrgIDs: model.JSONSliceInt64{7},
		}

		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentFind", int64(12)).Return(agentOtherOrg, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Set("org", &model.Org{ID: 5})
		c.Request, _ = http.NewRequest(http.MethodDelete, "/", nil)

		_, err := DeleteOrgAgent(testCtx(c), &DeleteOrgAgentInput{OrgID: 5, AgentID: 12})

		assert.Error(t, err)
		var humaErr huma.StatusError
		if errors.As(err, &humaErr) {
			assert.Equal(t, http.StatusBadRequest, humaErr.GetStatus())
		}
		mockStore.AssertNotCalled(t, "AgentDelete", mock.Anything)
		mockStore.AssertNotCalled(t, "AgentUpdate", mock.Anything)
	})
}

func TestPatchAgentOrgScope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	operator := &model.User{ID: 2, Login: "op", Roles: model.JSONSliceString{string(model.RoleOperator)}}
	admin := &model.User{ID: 1, Login: "admin", Admin: true}
	ids := func(i ...int64) *[]int64 { return &i }

	tests := []struct {
		name    string
		user    *model.User
		stored  model.JSONSliceInt64
		orgIDs  *[]int64
		want    model.JSONSliceInt64
		wantErr int
	}{
		{"operator making an org agent global is forbidden", operator, model.JSONSliceInt64{5}, ids(), nil, http.StatusForbidden},
		{"operator adding an org is forbidden", operator, model.JSONSliceInt64{5}, ids(5, 7), nil, http.StatusForbidden},
		{"operator scoping a global agent is forbidden", operator, nil, ids(5), nil, http.StatusForbidden},
		{"operator sending the unchanged org set succeeds", operator, model.JSONSliceInt64{5, 7}, ids(7, 5), model.JSONSliceInt64{5, 7}, 0},
		{"operator sending an empty set for a global agent succeeds", operator, nil, ids(), nil, 0},
		{"operator omitting org_ids keeps the scope", operator, model.JSONSliceInt64{5}, nil, model.JSONSliceInt64{5}, 0},
		{"admin can make an agent global", admin, model.JSONSliceInt64{5}, ids(), model.JSONSliceInt64{}, 0},
		{"admin can change the org set", admin, model.JSONSliceInt64{5}, ids(7), model.JSONSliceInt64{7}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &model.Agent{ID: 42, Name: "a", OwnerID: 1, OrgIDs: slices.Clone(tt.stored)}
			mockStore := store_mocks.NewMockStore(t)
			mockStore.On("AgentFind", int64(42)).Return(agent, nil)
			if tt.wantErr == 0 {
				mockStore.On("OrgGet", mock.Anything).Return(&model.Org{}, nil).Maybe()
				mockStore.On("AgentUpdate", mock.AnythingOfType("*model.Agent")).Return(nil)
			}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("store", mockStore)
			c.Set("user", tt.user)
			c.Request, _ = http.NewRequest(http.MethodPatch, "/", nil)

			output, err := PatchAgent(testCtx(c), &PatchAgentInput{
				AgentID: 42,
				Body:    PatchAgentBody{Name: "a", OrgIDs: tt.orgIDs},
			})
			if tt.wantErr != 0 {
				var humaErr huma.StatusError
				require.ErrorAs(t, err, &humaErr)
				assert.Equal(t, tt.wantErr, humaErr.GetStatus())
				return
			}
			require.NoError(t, err)
			if len(tt.want) == 0 {
				assert.Empty(t, output.Body.OrgIDs)
			} else {
				assert.Equal(t, tt.want, output.Body.OrgIDs)
			}
		})
	}
}

func TestPostAgentRejectsReservedCustomLabels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name   string
		labels map[string]string
		reject bool
	}{
		{name: "no labels"},
		{name: "regular labels", labels: map[string]string{"tier": "heavy"}},
		{name: "reserved org-id", labels: map[string]string{"tier": "heavy", pipeline.LabelOrgID: "5"}, reject: true},
		{name: "reserved repo-full-name", labels: map[string]string{pipeline.LabelRepoFullName: "org/repo"}, reject: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := store_mocks.NewMockStore(t)
			if !tt.reject {
				mockStore.On("AgentCreate", mock.Anything).Return(nil).Twice()
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set("store", mockStore)
			c.Set("user", &model.User{ID: 1, Admin: true})
			body := PostAgentBody{Name: "agent", CustomLabels: tt.labels}

			_, errGlobal := PostAgent(testCtx(c), &PostAgentInput{Body: body})
			_, errOrg := PostOrgAgent(testCtx(c), &PostOrgAgentInput{OrgID: 3, Body: body})
			for _, err := range []error{errGlobal, errOrg} {
				if !tt.reject {
					assert.NoError(t, err)
					continue
				}
				var se huma.StatusError
				require.True(t, errors.As(err, &se), "expected a huma status error, got %v", err)
				assert.Equal(t, http.StatusUnprocessableEntity, se.GetStatus())
			}
		})
	}
}

func TestAgentRestart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newCtx := func(mockStore *store_mocks.MockStore) context.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("store", mockStore)
		c.Request, _ = http.NewRequest(http.MethodPost, "/", nil)
		return testCtx(c)
	}

	t.Run("should request a restart and release the agent's polls", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentFind", int64(1)).Return(&model.Agent{ID: 1, RemoteUpdate: "restart"}, nil)
		mockStore.On("AgentSetRestartRequested", int64(1), true).Return(nil)
		mockQueue := new(queue_mocks.MockQueue)
		mockQueue.On("KickAgentWorkers", int64(1)).Return()
		server.Config.Services.Queue = mockQueue

		out, err := PostAgentRestart(newCtx(mockStore), &AgentRestartInput{AgentID: 1})

		assert.NoError(t, err)
		assert.True(t, out.Body.RestartRequested)
		mockStore.AssertCalled(t, "AgentSetRestartRequested", int64(1), true)
		mockQueue.AssertCalled(t, "KickAgentWorkers", int64(1))
	})

	t.Run("should refuse agents that do not accept restart requests", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentFind", int64(1)).Return(&model.Agent{ID: 1}, nil)

		_, err := PostAgentRestart(newCtx(mockStore), &AgentRestartInput{AgentID: 1})

		var humaErr huma.StatusError
		if assert.ErrorAs(t, err, &humaErr) {
			assert.Equal(t, http.StatusConflict, humaErr.GetStatus())
		}
		mockStore.AssertNotCalled(t, "AgentSetRestartRequested", mock.Anything, mock.Anything)
	})

	t.Run("should withdraw a request for any agent", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentFind", int64(1)).Return(&model.Agent{ID: 1, RestartRequested: true}, nil)
		mockStore.On("AgentSetRestartRequested", int64(1), false).Return(nil)

		out, err := DeleteAgentRestart(newCtx(mockStore), &AgentRestartInput{AgentID: 1})

		assert.NoError(t, err)
		assert.False(t, out.Body.RestartRequested)
	})

	t.Run("should queue every opted-in agent on another release", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("AgentList", mock.Anything).Return([]*model.Agent{
			{ID: 1, RemoteUpdate: "restart", Version: "0.0.1"},
			{ID: 2, RemoteUpdate: "restart", Version: version.String()},
			{ID: 3, Version: "0.0.1"},
			{ID: 4, RemoteUpdate: "restart", Version: "0.0.1", RestartRequested: true},
			{ID: 5, RemoteUpdate: "restart", Version: "0.0.1", RestartQueued: true},
		}, nil)
		mockStore.On("AgentQueueRestart", int64(1)).Return(nil)

		out, err := PostAgentRestarts(newCtx(mockStore), nil)

		assert.NoError(t, err)
		assert.Equal(t, 1, out.Body.Queued)
		mockStore.AssertNumberOfCalls(t, "AgentQueueRestart", 1)
	})
}
