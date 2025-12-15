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

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codeberg.org/crowci/crow/v5/server"
	"codeberg.org/crowci/crow/v5/server/forge"
	forge_mocks "codeberg.org/crowci/crow/v5/server/forge/mocks"
	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/pubsub"
	queue_mocks "codeberg.org/crowci/crow/v5/server/queue/mocks"
	"codeberg.org/crowci/crow/v5/server/services/config"
	"codeberg.org/crowci/crow/v5/server/services/environment"
	"codeberg.org/crowci/crow/v5/server/services/registry"
	"codeberg.org/crowci/crow/v5/server/services/secret"
	store_mocks "codeberg.org/crowci/crow/v5/server/store/mocks"
	store_types "codeberg.org/crowci/crow/v5/server/store/types"
)

// mockManager is a minimal mock for the Manager interface with just ForgeFromRepo
// implemented for this test.
type mockManager struct {
	ForgeFromRepoFunc func(repo *model.Repo) (forge.Forge, error)
}

func (m *mockManager) ForgeFromRepo(repo *model.Repo) (forge.Forge, error) {
	return m.ForgeFromRepoFunc(repo)
}

// Stub implementations for the rest of the Manager interface.
func (m *mockManager) SignaturePublicKey() crypto.PublicKey                   { return nil }
func (m *mockManager) SecretServiceFromRepo(_ *model.Repo) secret.Service     { return nil }
func (m *mockManager) SecretService() secret.Service                          { return nil }
func (m *mockManager) RegistryServiceFromRepo(_ *model.Repo) registry.Service { return nil }
func (m *mockManager) RegistryService() registry.Service                      { return nil }
func (m *mockManager) ConfigServiceFromRepo(_ *model.Repo) config.Service     { return nil }
func (m *mockManager) EnvironmentService() environment.Service                { return nil }
func (m *mockManager) ForgeFromUser(_ *model.User) (forge.Forge, error)       { return nil, nil }
func (m *mockManager) ForgeByID(_ int64) (forge.Forge, error)                 { return nil, nil }

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
		}
		server.Config.Services.Manager = mgr
		defer func() { server.Config.Services = originalServices }()

		// Create request
		req, _ := http.NewRequest("POST", "/api/repos/1/pipelines/123/workflows/456/cancel", nil)
		w := httptest.NewRecorder()

		// Setup gin context
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{
			{Key: "repo_id", Value: "1"},
			{Key: "number", Value: "123"},
			{Key: "workflow_id", Value: "456"},
		}

		// Mock session
		c.Set("repo", repo)
		c.Set("user", user)
		c.Set("store", mockStore)

		// Execute
		CancelWorkflow(c)

		// Assert
		assert.Equal(t, http.StatusNoContent, w.Code)
		mockStore.AssertExpectations(t)
		mockForge.AssertExpectations(t)
	})

	t.Run("pipeline not found", func(t *testing.T) {
		// Setup
		repo := &model.Repo{ID: 1, FullName: "test/repo"}
		user := &model.User{ID: 1, Login: "testuser"}

		// Mock store
		mockStore := new(store_mocks.MockStore)
		mockStore.On("GetPipelineNumber", repo, int64(123)).Return(nil, store_types.RecordNotExist)

		// Mock forge
		mockForge := new(forge_mocks.MockForge)

		// Setup server config
		originalServices := server.Config.Services
		mgr := &mockManager{
			ForgeFromRepoFunc: func(_ *model.Repo) (forge.Forge, error) {
				return mockForge, nil
			},
		}
		server.Config.Services.Manager = mgr
		defer func() { server.Config.Services = originalServices }()

		// Create request
		req, _ := http.NewRequest("POST", "/api/repos/1/pipelines/123/workflows/456/cancel", nil)
		w := httptest.NewRecorder()

		// Setup gin context
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{
			{Key: "repo_id", Value: "1"},
			{Key: "number", Value: "123"},
			{Key: "workflow_id", Value: "456"},
		}

		// Mock session
		c.Set("repo", repo)
		c.Set("user", user)
		c.Set("store", mockStore)

		// Execute
		CancelWorkflow(c)

		// Assert
		assert.Equal(t, http.StatusNotFound, w.Code)
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
		w := httptest.NewRecorder()

		// Setup gin context
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{
			{Key: "repo_id", Value: "1"},
			{Key: "number", Value: "123"},
			{Key: "workflow_id", Value: "456"},
		}

		// Mock session
		c.Set("repo", repo)
		c.Set("user", user)
		c.Set("store", mockStore)

		// Execute
		CancelWorkflow(c)

		// Assert
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockStore.AssertExpectations(t)
		mockForge.AssertExpectations(t)
	})

	t.Run("invalid pipeline number", func(t *testing.T) {
		// Setup
		repo := &model.Repo{ID: 1, FullName: "test/repo"}
		user := &model.User{ID: 1, Login: "testuser"}

		// Mock store
		mockStore := new(store_mocks.MockStore)

		// Mock forge
		mockForge := new(forge_mocks.MockForge)

		// Setup server config
		originalServices := server.Config.Services
		mgr := &mockManager{
			ForgeFromRepoFunc: func(_ *model.Repo) (forge.Forge, error) {
				return mockForge, nil
			},
		}
		server.Config.Services.Manager = mgr
		defer func() { server.Config.Services = originalServices }()

		// Create request
		req, _ := http.NewRequest("POST", "/api/repos/1/pipelines/invalid/workflows/456/cancel", nil)
		w := httptest.NewRecorder()

		// Setup gin context
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{
			{Key: "repo_id", Value: "1"},
			{Key: "number", Value: "invalid"},
			{Key: "workflow_id", Value: "456"},
		}

		// Mock session
		c.Set("repo", repo)
		c.Set("user", user)
		c.Set("store", mockStore)

		// Execute
		CancelWorkflow(c)

		// Assert
		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockStore.AssertExpectations(t)
		mockForge.AssertExpectations(t)
	})

	t.Run("invalid workflow ID", func(t *testing.T) {
		// Setup
		repo := &model.Repo{ID: 1, FullName: "test/repo"}
		user := &model.User{ID: 1, Login: "testuser"}

		// Mock store
		mockStore := new(store_mocks.MockStore)

		// Mock forge
		mockForge := new(forge_mocks.MockForge)

		// Setup server config
		originalServices := server.Config.Services
		mgr := &mockManager{
			ForgeFromRepoFunc: func(_ *model.Repo) (forge.Forge, error) {
				return mockForge, nil
			},
		}
		server.Config.Services.Manager = mgr
		defer func() { server.Config.Services = originalServices }()

		// Create request
		req, _ := http.NewRequest("POST", "/api/repos/1/pipelines/123/workflows/invalid/cancel", nil)
		w := httptest.NewRecorder()

		// Setup gin context
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{
			{Key: "repo_id", Value: "1"},
			{Key: "number", Value: "123"},
			{Key: "workflow_id", Value: "invalid"},
		}

		// Mock session
		c.Set("repo", repo)
		c.Set("user", user)
		c.Set("store", mockStore)

		// Execute
		CancelWorkflow(c)

		// Assert
		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockStore.AssertExpectations(t)
		mockForge.AssertExpectations(t)
	})
}
