// Copyright 2025 Woodpecker Authors
// Copyright 2025 Crow CI Authors
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
	"crypto"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/forge"
	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pubsub"
	queue_mocks "codefloe.com/crowci/crow/v6/server/queue/mocks"
	"codefloe.com/crowci/crow/v6/server/services/config"
	"codefloe.com/crowci/crow/v6/server/services/environment"
	"codefloe.com/crowci/crow/v6/server/services/registry"
	"codefloe.com/crowci/crow/v6/server/services/secret"
	"codefloe.com/crowci/crow/v6/server/store"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
	store_types "codefloe.com/crowci/crow/v6/server/store/types"
)

// mockManager is a minimal mock for the Manager interface with just ForgeFromRepo
// implemented for this test.
type mockManager struct {
	ForgeFromRepoFunc func(repo *model.Repo) (forge.Forge, error)
	ForgeFromUserFunc func(user *model.User) (forge.Forge, error)
}

func (m *mockManager) ForgeFromRepo(repo *model.Repo) (forge.Forge, error) {
	return m.ForgeFromRepoFunc(repo)
}

func (m *mockManager) ForgeFromUser(user *model.User) (forge.Forge, error) {
	if m.ForgeFromUserFunc != nil {
		return m.ForgeFromUserFunc(user)
	}
	return nil, nil
}

// Stub implementations for the rest of the Manager interface.
func (m *mockManager) SignaturePublicKey() crypto.PublicKey                   { return nil }
func (m *mockManager) SecretServiceFromRepo(_ *model.Repo) secret.Service     { return nil }
func (m *mockManager) SecretService() secret.Service                          { return nil }
func (m *mockManager) RegistryServiceFromRepo(_ *model.Repo) registry.Service { return nil }
func (m *mockManager) RegistryService() registry.Service                      { return nil }
func (m *mockManager) ConfigServiceFromRepo(_ *model.Repo) config.Service     { return nil }
func (m *mockManager) EnvironmentService() environment.Service                { return nil }
func (m *mockManager) ForgeByID(_ int64) (forge.Forge, error)                 { return nil, nil }
func (m *mockManager) Store() store.Store                                     { return nil }

func TestCancelWorkflow(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	t.Run("successful cancel workflow", func(t *testing.T) {
		// Setup
		repo := &model.Repo{ID: 1, FullName: "test/repo"}
		user := &model.User{ID: 1, Login: "testuser"}
		pipeline := &model.Pipeline{
			ID:     1,
			RepoID: 1,
			Status: model.StatusRunning,
		}

		// Mock store
		mockStore := new(store_mocks.MockStore)
		mockStore.On("UserForgeGet", user.ID, repo.ForgeID).Return(nil, store_types.RecordNotExist)
		mockStore.On("GetPipelineNumber", repo, int64(123)).Return(pipeline, nil)
		mockStore.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).Return([]*model.Workflow{
			{
				ID:         456,
				PipelineID: 1,
				State:      model.StatusRunning,
				Children:   []*model.Step{},
			},
		}, nil)
		mockStore.On("WorkflowUpdate", mock.AnythingOfType("*model.Workflow")).Return(nil)
		mockStore.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).Return([]*model.Workflow{
			{
				ID:         456,
				PipelineID: 1,
				State:      model.StatusKilled,
				Children:   []*model.Step{},
			},
		}, nil)

		// Mock forge
		mockForge := new(forge_mocks.MockForge)

		// Mock queue
		mockQueue := new(queue_mocks.MockQueue)
		mockQueue.On("Error", mock.Anything, "456", mock.Anything).Return(nil)
		originalQueue := server.Config.Services.Queue
		server.Config.Services.Queue = mockQueue
		defer func() { server.Config.Services.Queue = originalQueue }()

		// Mock pubsub
		originalPubsub := server.Config.Services.Pubsub
		server.Config.Services.Pubsub = pubsub.New()
		defer func() { server.Config.Services.Pubsub = originalPubsub }()

		// Setup server config
		originalServices := server.Config.Services
		mgr := &mockManager{
			ForgeFromRepoFunc: func(_ *model.Repo) (forge.Forge, error) {
				return mockForge, nil
			},
			ForgeFromUserFunc: func(_ *model.User) (forge.Forge, error) {
				return mockForge, nil
			},
		}
		server.Config.Services.Manager = mgr
		defer func() { server.Config.Services = originalServices }()

		// Create request
		req, _ := http.NewRequest("POST", "/api/repos/1/pipelines/123/workflows/456/cancel", nil)

		// Setup gin context
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = req

		// Mock session
		c.Set("repo", repo)
		c.Set("user", user)
		c.Set("store", mockStore)

		// Execute
		_, err := CancelWorkflow(testCtx(c), &CancelWorkflowInput{
			RepoID:     1,
			Number:     123,
			WorkflowID: 456,
		})

		// Assert
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
		mockForge.AssertExpectations(t)
	})

	t.Run("pipeline not found", func(t *testing.T) {
		// Setup
		repo := &model.Repo{ID: 1, FullName: "test/repo"}
		user := &model.User{ID: 1, Login: "testuser"}

		// Mock store
		mockStore := new(store_mocks.MockStore)
		mockStore.On("UserForgeGet", user.ID, repo.ForgeID).Return(nil, store_types.RecordNotExist)
		mockStore.On("GetPipelineNumber", repo, int64(123)).Return(nil, store_types.RecordNotExist)

		// Mock forge
		mockForge := new(forge_mocks.MockForge)

		// Setup server config
		originalServices := server.Config.Services
		mgr := &mockManager{
			ForgeFromRepoFunc: func(_ *model.Repo) (forge.Forge, error) {
				return mockForge, nil
			},
			ForgeFromUserFunc: func(_ *model.User) (forge.Forge, error) {
				return mockForge, nil
			},
		}
		server.Config.Services.Manager = mgr
		defer func() { server.Config.Services = originalServices }()

		// Create request
		req, _ := http.NewRequest("POST", "/api/repos/1/pipelines/123/workflows/456/cancel", nil)

		// Setup gin context
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = req

		// Mock session
		c.Set("repo", repo)
		c.Set("user", user)
		c.Set("store", mockStore)

		// Execute
		_, err := CancelWorkflow(testCtx(c), &CancelWorkflowInput{
			RepoID:     1,
			Number:     123,
			WorkflowID: 456,
		})

		// Assert
		var humaErr huma.StatusError
		assert.ErrorAs(t, err, &humaErr)
		assert.Equal(t, http.StatusNotFound, humaErr.GetStatus())
		mockStore.AssertExpectations(t)
		mockForge.AssertExpectations(t)
	})

	t.Run("forge error", func(t *testing.T) {
		// Setup
		repo := &model.Repo{ID: 1, FullName: "test/repo"}
		user := &model.User{ID: 1, Login: "testuser"}

		// Mock store
		mockStore := new(store_mocks.MockStore)

		// Mock forge with error
		mockForge := new(forge_mocks.MockForge)

		// Setup server config
		originalServices := server.Config.Services
		mgr := &mockManager{
			ForgeFromRepoFunc: func(_ *model.Repo) (forge.Forge, error) {
				return nil, errors.New("forge error")
			},
		}
		server.Config.Services.Manager = mgr
		defer func() { server.Config.Services = originalServices }()

		// Create request
		req, _ := http.NewRequest("POST", "/api/repos/1/pipelines/123/workflows/456/cancel", nil)

		// Setup gin context
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = req

		// Mock session
		c.Set("repo", repo)
		c.Set("user", user)
		c.Set("store", mockStore)

		// Execute
		_, err := CancelWorkflow(testCtx(c), &CancelWorkflowInput{
			RepoID:     1,
			Number:     123,
			WorkflowID: 456,
		})

		// Assert
		var humaErr huma.StatusError
		assert.ErrorAs(t, err, &humaErr)
		assert.Equal(t, http.StatusInternalServerError, humaErr.GetStatus())
		mockStore.AssertExpectations(t)
		mockForge.AssertExpectations(t)
	})

	// Note: The old "invalid pipeline number" and "invalid workflow ID" tests validated
	// gin param parsing of string path params like "invalid". With huma, Number and
	// WorkflowID are int64 fields validated by the framework before the handler is
	// called, so those cases are no longer testable at the handler level.
}
