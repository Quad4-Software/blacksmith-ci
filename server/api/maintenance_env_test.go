// Copyright 2025 Crow Authors
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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	mocks_service "codefloe.com/crowci/crow/v6/server/services/maintenance/mocks"
)

func TestGetMaintenanceConfigByAction_EnvVarsSet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a mock maintenance service
	mockService := mocks_service.NewMockService(t)

	// Set up the mock to return a config with env vars set
	k8sConfig := &model.MaintenanceConfig{
		ID:         1,
		ActionType: "kubernetes_cleanup",
		Enabled:    true,
		Schedule:   "0 5 * * 0",
	}

	// Mock expectations
	mockService.EXPECT().
		GetMaintenanceConfigByAction("kubernetes_cleanup").
		Return(k8sConfig, nil)

	mockService.EXPECT().
		IsKubernetesCleanupAvailable().
		Return(true)

	mockService.EXPECT().
		IsKubernetesEnvVarsConfigured().
		Return(true)

	// Set up server config with mock
	originalService := server.Config.Services.MaintenanceService
	server.Config.Services.MaintenanceService = mockService
	defer func() {
		server.Config.Services.MaintenanceService = originalService
	}()

	// Create test context with admin user
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/admin/maintenance/config/kubernetes_cleanup", nil)
	c.Set("user", &model.User{
		ID:    1,
		Login: "admin",
		Admin: true,
	})

	output, err := GetMaintenanceConfigByAction(testCtx(c), &GetMaintenanceConfigByActionInput{
		Action: "kubernetes_cleanup",
	})
	require.NoError(t, err)
	require.NotNil(t, output)

	response := output.Body

	// Verify that env_vars_set is true in the response
	assert.True(t, response.EnvVarsSet, "env_vars_set should be true when environment variables are set")
	assert.Equal(t, "kubernetes_cleanup", response.ActionType)
	assert.True(t, response.Enabled)
	assert.True(t, response.KubernetesCleanupAvailable)
}

func TestGetMaintenanceConfigByAction_EnvVarsNotSet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a mock maintenance service
	mockService := mocks_service.NewMockService(t)

	// Set up the mock to return a config with env vars NOT set
	k8sConfig := &model.MaintenanceConfig{
		ID:         1,
		ActionType: "kubernetes_cleanup",
		Enabled:    false,
		Schedule:   "0 5 * * 0",
	}

	// Mock expectations
	mockService.EXPECT().
		GetMaintenanceConfigByAction("kubernetes_cleanup").
		Return(k8sConfig, nil)

	mockService.EXPECT().
		IsKubernetesCleanupAvailable().
		Return(true)

	mockService.EXPECT().
		IsKubernetesEnvVarsConfigured().
		Return(false)

	// Set up server config with mock
	originalService := server.Config.Services.MaintenanceService
	server.Config.Services.MaintenanceService = mockService
	defer func() {
		server.Config.Services.MaintenanceService = originalService
	}()

	// Create test context with admin user
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/admin/maintenance/config/kubernetes_cleanup", nil)
	c.Set("user", &model.User{
		ID:    1,
		Login: "admin",
		Admin: true,
	})

	output, err := GetMaintenanceConfigByAction(testCtx(c), &GetMaintenanceConfigByActionInput{
		Action: "kubernetes_cleanup",
	})
	require.NoError(t, err)
	require.NotNil(t, output)

	response := output.Body

	// Verify that env_vars_set is false in the response
	assert.False(t, response.EnvVarsSet, "env_vars_set should be false when environment variables are not set")
	assert.Equal(t, "kubernetes_cleanup", response.ActionType)
	assert.False(t, response.Enabled)
	assert.True(t, response.KubernetesCleanupAvailable)
}

func TestGetMaintenanceConfigByAction_StaleAgentEnvVarsSet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := mocks_service.NewMockService(t)

	// Stale agent cleanup disabled via CROW_MAINTENANCE_STALE_AGENT_CLEANUP_ENABLED=false
	staleAgentConfig := &model.MaintenanceConfig{
		ID:         1,
		ActionType: "stale_agent_cleanup",
		Enabled:    false,
		Schedule:   "0 5 * * *",
	}

	mockService.EXPECT().
		GetMaintenanceConfigByAction("stale_agent_cleanup").
		Return(staleAgentConfig, nil)

	mockService.EXPECT().
		IsStaleAgentEnvVarsConfigured().
		Return(true)

	originalService := server.Config.Services.MaintenanceService
	server.Config.Services.MaintenanceService = mockService
	defer func() {
		server.Config.Services.MaintenanceService = originalService
	}()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/admin/maintenance/config/stale_agent_cleanup", nil)
	c.Set("user", &model.User{
		ID:    1,
		Login: "admin",
		Admin: true,
	})

	output, err := GetMaintenanceConfigByAction(testCtx(c), &GetMaintenanceConfigByActionInput{
		Action: "stale_agent_cleanup",
	})
	require.NoError(t, err)
	require.NotNil(t, output)

	response := output.Body

	// env_vars_set must reflect the env override so the UI marks it as env-managed
	assert.True(t, response.EnvVarsSet, "env_vars_set should be true when environment variables are set")
	assert.Equal(t, "stale_agent_cleanup", response.ActionType)
	assert.False(t, response.Enabled, "stale agent cleanup should report disabled when env disables it")
}
