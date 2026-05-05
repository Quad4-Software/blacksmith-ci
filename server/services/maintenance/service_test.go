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

package maintenance

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v5/server/model"
	store "codefloe.com/crowci/crow/v5/server/store/mocks"
)

func TestService_SetKubernetesCleanupFunc(t *testing.T) {
	mockStore := &store.MockStore{}
	service, ok := NewService(mockStore).(*service)
	require.True(t, ok)

	assert.False(t, service.kubernetesCleanupAvailable)
	assert.False(t, service.kubernetesCleanupEnabled)
	assert.Nil(t, service.kubernetesCleanupFunc)

	cleanupFunc := func(ctx context.Context, dryRun bool) error {
		return nil
	}

	service.SetKubernetesCleanupFunc(cleanupFunc)

	assert.True(t, service.kubernetesCleanupAvailable)
	assert.False(t, service.kubernetesCleanupEnabled)
	assert.NotNil(t, service.kubernetesCleanupFunc)

	service.SetKubernetesCleanupEnabled(true)
	assert.True(t, service.kubernetesCleanupEnabled)
}

func TestService_GetMaintenanceConfig_KubernetesFlag(t *testing.T) {
	tests := []struct {
		name                 string
		kubernetesAvailable  bool
		kubernetesEnabled    bool
		expectedK8sAvailable bool
		expectedK8sEnabled   bool
	}{
		{
			name:                 "kubernetes cleanup available and enabled",
			kubernetesAvailable:  true,
			kubernetesEnabled:    true,
			expectedK8sAvailable: true,
			expectedK8sEnabled:   true,
		},
		{
			name:                 "kubernetes cleanup available but disabled",
			kubernetesAvailable:  true,
			kubernetesEnabled:    false,
			expectedK8sAvailable: true,
			expectedK8sEnabled:   false,
		},
		{
			name:                 "kubernetes cleanup not available",
			kubernetesAvailable:  false,
			kubernetesEnabled:    false,
			expectedK8sAvailable: false,
			expectedK8sEnabled:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &store.MockStore{}
			service, ok := NewService(mockStore).(*service)
			require.True(t, ok)

			// Set up mock response
			mockConfig := &model.MaintenanceConfig{
				ID:       1,
				Enabled:  true,
				Schedule: "0 5 * * *",
				Created:  1234567890,
				Updated:  1234567890,
			}
			mockStore.On("MaintenanceConfigGet").Return(mockConfig, nil)

			if tt.kubernetesAvailable {
				service.SetKubernetesCleanupFunc(func(ctx context.Context, dryRun bool) error { return nil })
			}
			if tt.kubernetesEnabled {
				service.SetKubernetesCleanupEnabled(true)
			}

			config := service.GetMaintenanceConfig()
			assert.Equal(t, tt.expectedK8sAvailable, config.KubernetesCleanupAvailable)

			mockStore.AssertExpectations(t)
		})
	}
}

func TestService_GetMaintenanceConfig_ErrorHandling(t *testing.T) {
	mockStore := &store.MockStore{}
	service, ok := NewService(mockStore).(*service)
	require.True(t, ok)

	mockStore.On("MaintenanceConfigGet").Return(nil, errors.New("database error"))

	config := service.GetMaintenanceConfig()

	assert.Equal(t, int64(0), config.ID)
	assert.True(t, config.Enabled)
	assert.Equal(t, "0 5 * * *", config.Schedule)
	assert.False(t, config.KubernetesCleanupAvailable)

	mockStore.AssertExpectations(t)
}

func TestService_RunKubernetesCleanup(t *testing.T) {
	tests := []struct {
		name              string
		setupCleanupFunc  bool
		cleanupFuncError  error
		dryRun            bool
		expectError       bool
		expectLogCreation bool
	}{
		{
			name:              "no cleanup function configured",
			setupCleanupFunc:  false,
			dryRun:            false,
			expectError:       false,
			expectLogCreation: false,
		},
		{
			name:              "cleanup function succeeds",
			setupCleanupFunc:  true,
			cleanupFuncError:  nil,
			dryRun:            false,
			expectError:       false,
			expectLogCreation: true,
		},
		{
			name:              "cleanup function succeeds in dry run",
			setupCleanupFunc:  true,
			cleanupFuncError:  nil,
			dryRun:            true,
			expectError:       false,
			expectLogCreation: true,
		},
		{
			name:              "cleanup function fails",
			setupCleanupFunc:  true,
			cleanupFuncError:  errors.New("cleanup failed"),
			dryRun:            false,
			expectError:       true,
			expectLogCreation: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &store.MockStore{}
			service, ok := NewService(mockStore).(*service)
			require.True(t, ok)

			if tt.setupCleanupFunc {
				cleanupFunc := func(ctx context.Context, dryRun bool) error {
					assert.Equal(t, tt.dryRun, dryRun, "dryRun parameter should match")
					return tt.cleanupFuncError
				}
				service.SetKubernetesCleanupFunc(cleanupFunc)
			}

			if tt.expectLogCreation {
				mockStore.On("MaintenanceLogCreate", mock.AnythingOfType("*model.MaintenanceLog")).Return(nil)
				// Mock statistics tracking calls
				mockConfig := &model.MaintenanceConfig{
					ID:         1,
					ActionType: "kubernetes_cleanup",
					Enabled:    true,
					Schedule:   "0 5 * * 0",
					TotalRuns:  0,
				}
				mockStore.On("MaintenanceConfigGetByActionType", "kubernetes_cleanup").Return(mockConfig, nil)
				mockStore.On("MaintenanceConfigUpdateByActionType", "kubernetes_cleanup", mock.AnythingOfType("*model.MaintenanceConfig")).Return(nil)
			}

			err := service.RunKubernetesCleanup(t.Context(), tt.dryRun)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "kubernetes resource cleanup failed")
			} else {
				assert.NoError(t, err)
			}

			mockStore.AssertExpectations(t)
		})
	}
}

func TestService_RunKubernetesCleanup_LogCreationError(t *testing.T) {
	mockStore := &store.MockStore{}
	service, ok := NewService(mockStore).(*service)
	require.True(t, ok)

	cleanupFunc := func(ctx context.Context, dryRun bool) error {
		return nil
	}
	service.SetKubernetesCleanupFunc(cleanupFunc)

	// Mock log creation to fail
	mockStore.On("MaintenanceLogCreate", mock.AnythingOfType("*model.MaintenanceLog")).Return(errors.New("log creation failed"))
	mockConfig := &model.MaintenanceConfig{
		ID:         1,
		ActionType: "kubernetes_cleanup",
		Enabled:    true,
		Schedule:   "0 5 * * 0",
		TotalRuns:  0,
	}
	mockStore.On("MaintenanceConfigGetByActionType", "kubernetes_cleanup").Return(mockConfig, nil)
	mockStore.On("MaintenanceConfigUpdateByActionType", "kubernetes_cleanup", mock.AnythingOfType("*model.MaintenanceConfig")).Return(nil)

	err := service.RunKubernetesCleanup(t.Context(), false)
	assert.NoError(t, err)

	mockStore.AssertExpectations(t)
}

func TestService_RunOrphanedDataCleanup_DryRun(t *testing.T) {
	mockStore := &store.MockStore{}
	svc, ok := NewService(mockStore).(*service)
	require.True(t, ok)

	dangling := []*model.Cron{{ID: 11, Name: "dangling"}}
	orphanOwner := []*model.Cron{{ID: 22, Name: "orphan_active"}}
	orphanRepos := []*model.Repo{{ID: 5, FullName: "alice/orphan"}}

	mockStore.On("CronListOrphaned").Return(dangling, nil)
	mockStore.On("CronListOrphanedOwner").Return(orphanOwner, nil)
	mockStore.On("RepoListOrphanedOwner").Return(orphanRepos, nil)
	mockStore.On("MaintenanceLogCreate", mock.AnythingOfType("*model.MaintenanceLog")).Return(nil)
	// dry-run skips updateActionLastRun — but updateActionStats still runs
	mockStore.On("MaintenanceConfigGetByActionType", "orphaned_data_cleanup").Return(&model.MaintenanceConfig{ActionType: "orphaned_data_cleanup", Enabled: true}, nil)
	mockStore.On("MaintenanceConfigUpdateByActionType", "orphaned_data_cleanup", mock.AnythingOfType("*model.MaintenanceConfig")).Return(nil)

	err := svc.RunOrphanedDataCleanup(t.Context(), true)
	assert.NoError(t, err)

	// Crucial: no mutation calls in dry-run
	mockStore.AssertNotCalled(t, "CronDeleteByID", mock.Anything)
	mockStore.AssertNotCalled(t, "CronDisableOrphaned", mock.Anything, mock.Anything)
	mockStore.AssertExpectations(t)
}

func TestService_RunOrphanedDataCleanup_Apply(t *testing.T) {
	mockStore := &store.MockStore{}
	svc, ok := NewService(mockStore).(*service)
	require.True(t, ok)

	dangling := []*model.Cron{{ID: 11}, {ID: 12}}
	orphanOwner := []*model.Cron{{ID: 22}}
	orphanRepos := []*model.Repo{{ID: 5, FullName: "alice/orphan"}}

	mockStore.On("CronListOrphaned").Return(dangling, nil)
	mockStore.On("CronListOrphanedOwner").Return(orphanOwner, nil)
	mockStore.On("RepoListOrphanedOwner").Return(orphanRepos, nil)
	mockStore.On("CronDeleteByID", int64(11)).Return(nil)
	mockStore.On("CronDeleteByID", int64(12)).Return(nil)
	mockStore.On("CronDisableOrphaned", int64(22), "auto-disabled by maintenance: repo owner missing").Return(nil)
	mockStore.On("MaintenanceLogCreate", mock.AnythingOfType("*model.MaintenanceLog")).Return(nil)
	// non-dry-run hits updateActionLastRun (Get + Update) AND updateActionStats (Get + Update)
	mockStore.On("MaintenanceConfigGetByActionType", "orphaned_data_cleanup").Return(&model.MaintenanceConfig{ActionType: "orphaned_data_cleanup", Enabled: true}, nil)
	mockStore.On("MaintenanceConfigUpdateByActionType", "orphaned_data_cleanup", mock.AnythingOfType("*model.MaintenanceConfig")).Return(nil)

	err := svc.RunOrphanedDataCleanup(t.Context(), false)
	assert.NoError(t, err)

	mockStore.AssertExpectations(t)
}

func TestService_RunOrphanedDataCleanup_Empty(t *testing.T) {
	mockStore := &store.MockStore{}
	svc, ok := NewService(mockStore).(*service)
	require.True(t, ok)

	mockStore.On("CronListOrphaned").Return([]*model.Cron{}, nil)
	mockStore.On("CronListOrphanedOwner").Return([]*model.Cron{}, nil)
	mockStore.On("RepoListOrphanedOwner").Return([]*model.Repo{}, nil)
	mockStore.On("MaintenanceLogCreate", mock.AnythingOfType("*model.MaintenanceLog")).Return(nil)
	mockStore.On("MaintenanceConfigGetByActionType", "orphaned_data_cleanup").Return(&model.MaintenanceConfig{ActionType: "orphaned_data_cleanup", Enabled: true}, nil)
	mockStore.On("MaintenanceConfigUpdateByActionType", "orphaned_data_cleanup", mock.AnythingOfType("*model.MaintenanceConfig")).Return(nil)

	err := svc.RunOrphanedDataCleanup(t.Context(), false)
	assert.NoError(t, err)

	mockStore.AssertNotCalled(t, "CronDeleteByID", mock.Anything)
	mockStore.AssertNotCalled(t, "CronDisableOrphaned", mock.Anything, mock.Anything)
	mockStore.AssertExpectations(t)
}

func TestService_RunOrphanedDataCleanup_QueryFailure(t *testing.T) {
	mockStore := &store.MockStore{}
	svc, ok := NewService(mockStore).(*service)
	require.True(t, ok)

	mockStore.On("CronListOrphaned").Return([]*model.Cron(nil), errors.New("db down"))
	mockStore.On("MaintenanceLogCreate", mock.AnythingOfType("*model.MaintenanceLog")).Return(nil)
	mockStore.On("MaintenanceConfigGetByActionType", "orphaned_data_cleanup").Return(&model.MaintenanceConfig{ActionType: "orphaned_data_cleanup"}, nil)
	mockStore.On("MaintenanceConfigUpdateByActionType", "orphaned_data_cleanup", mock.AnythingOfType("*model.MaintenanceConfig")).Return(nil)

	err := svc.RunOrphanedDataCleanup(t.Context(), false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "listing dangling crons")

	mockStore.AssertNotCalled(t, "CronListOrphanedOwner")
	mockStore.AssertExpectations(t)
}

func TestService_RunOrphanedDataCleanup_PartialFailure(t *testing.T) {
	mockStore := &store.MockStore{}
	svc, ok := NewService(mockStore).(*service)
	require.True(t, ok)

	dangling := []*model.Cron{{ID: 11}, {ID: 12}}
	orphanOwner := []*model.Cron{{ID: 22}}

	mockStore.On("CronListOrphaned").Return(dangling, nil)
	mockStore.On("CronListOrphanedOwner").Return(orphanOwner, nil)
	mockStore.On("RepoListOrphanedOwner").Return([]*model.Repo{}, nil)
	// ID 11 fails, ID 12 succeeds
	mockStore.On("CronDeleteByID", int64(11)).Return(errors.New("locked"))
	mockStore.On("CronDeleteByID", int64(12)).Return(nil)
	mockStore.On("CronDisableOrphaned", int64(22), "auto-disabled by maintenance: repo owner missing").Return(nil)

	var capturedLog *model.MaintenanceLog
	mockStore.On("MaintenanceLogCreate", mock.AnythingOfType("*model.MaintenanceLog")).Run(func(args mock.Arguments) {
		entry, ok := args.Get(0).(*model.MaintenanceLog)
		require.True(t, ok)
		capturedLog = entry
	}).Return(nil)

	// non-dry-run hits updateActionLastRun (Get + Update) AND updateActionStats (Get + Update)
	mockStore.On("MaintenanceConfigGetByActionType", "orphaned_data_cleanup").Return(&model.MaintenanceConfig{ActionType: "orphaned_data_cleanup", Enabled: true}, nil)
	var capturedConfig *model.MaintenanceConfig
	mockStore.On("MaintenanceConfigUpdateByActionType", "orphaned_data_cleanup", mock.AnythingOfType("*model.MaintenanceConfig")).Run(func(args mock.Arguments) {
		cfg, ok := args.Get(1).(*model.MaintenanceConfig)
		require.True(t, ok)
		capturedConfig = cfg
	}).Return(nil)

	err := svc.RunOrphanedDataCleanup(t.Context(), false)
	// Partial failures do not abort: function must return nil
	require.NoError(t, err)

	require.NotNil(t, capturedLog)
	assert.False(t, capturedLog.Success)
	assert.Contains(t, capturedLog.Error, "partial failure")

	// The last captured config update (from updateActionStats) should reflect failure
	require.NotNil(t, capturedConfig)
	assert.False(t, capturedConfig.LastRunSuccess)

	mockStore.AssertExpectations(t)
}

func TestService_ActionStatsTracking(t *testing.T) {
	tests := []struct {
		name        string
		actionType  string
		success     bool
		expectError bool
	}{
		{
			name:        "vacuum statistics tracking success",
			actionType:  "vacuum",
			success:     true,
			expectError: false,
		},
		{
			name:        "kubernetes cleanup statistics tracking success",
			actionType:  "kubernetes_cleanup",
			success:     true,
			expectError: false,
		},
		{
			name:        "vacuum statistics tracking failure",
			actionType:  "vacuum",
			success:     false,
			expectError: false,
		},
		{
			name:        "kubernetes cleanup statistics tracking failure",
			actionType:  "kubernetes_cleanup",
			success:     false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &store.MockStore{}
			service, ok := NewService(mockStore).(*service)
			require.True(t, ok)

			// Mock the config retrieval for statistics update
			mockConfig := &model.MaintenanceConfig{
				ID:         1,
				ActionType: tt.actionType,
				Enabled:    true,
				Schedule:   "0 5 * * *",
				TotalRuns:  5, // Previous run count
			}
			mockStore.On("MaintenanceConfigGetByActionType", tt.actionType).Return(mockConfig, nil)

			// Mock the config update
			mockStore.On("MaintenanceConfigUpdateByActionType", tt.actionType, mock.MatchedBy(func(config *model.MaintenanceConfig) bool {
				if config.TotalRuns != 6 { // Should increment from 5 to 6
					t.Errorf("Expected TotalRuns to be 6, got %d", config.TotalRuns)
					return false
				}
				if config.LastRunSuccess != tt.success {
					t.Errorf("Expected LastRunSuccess to be %v, got %v", tt.success, config.LastRunSuccess)
					return false
				}
				if tt.success && config.SuccessfulRuns != 1 {
					t.Errorf("Expected SuccessfulRuns to be incremented on success")
					return false
				}
				return true
			})).Return(nil)

			errorMsg := ""
			if !tt.success {
				errorMsg = "test error"
			}
			service.updateActionStats(tt.actionType, tt.success, 0, errorMsg)

			mockStore.AssertExpectations(t)
		})
	}
}
