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
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v5/server/model"
	mocks_service "codefloe.com/crowci/crow/v5/server/services/maintenance/mocks"
	mocks_store "codefloe.com/crowci/crow/v5/server/store/mocks"
)

func TestScheduler_CheckAction_Vacuum(t *testing.T) {
	mockService := mocks_service.NewMockService(t)
	mockStore := mocks_store.NewMockStore(t)

	scheduler := NewScheduler(mockService, mockStore, &mockLockService{})

	now := time.Date(2025, 1, 7, 7, 0, 0, 0, time.UTC)

	vacuumConfig := &model.MaintenanceConfig{
		ActionType: "vacuum",
		Enabled:    true,
		Schedule:   "0 7 * * *",
		LastRun:    time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC),
	}

	mockService.EXPECT().
		GetMaintenanceConfigByAction("vacuum").
		Return(vacuumConfig, nil)

	mockService.EXPECT().
		RunMaintenance(mock.Anything).
		Return(nil)

	scheduler.checkAction("vacuum", now)

	mockService.AssertExpectations(t)
}

func TestScheduler_CheckAction_KubernetesCleanup(t *testing.T) {
	mockService := mocks_service.NewMockService(t)
	mockStore := mocks_store.NewMockStore(t)

	scheduler := NewScheduler(mockService, mockStore, &mockLockService{})
	scheduler.ctx = t.Context()

	now := time.Date(2025, 1, 7, 7, 0, 0, 0, time.UTC)

	k8sConfig := &model.MaintenanceConfig{
		ActionType: "kubernetes_cleanup",
		Enabled:    true,
		Schedule:   "0 7 * * *",
		LastRun:    time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC),
	}

	mockService.EXPECT().
		GetMaintenanceConfigByAction("kubernetes_cleanup").
		Return(k8sConfig, nil)

	mockService.EXPECT().
		IsKubernetesCleanupAvailable().
		Return(true)

	mockService.EXPECT().
		RunKubernetesCleanup(mock.Anything, false).
		Return(nil)

	scheduler.checkAction("kubernetes_cleanup", now)

	mockService.AssertExpectations(t)
}

func TestScheduler_CheckAction_KubernetesCleanup_NotAvailable(t *testing.T) {
	mockService := mocks_service.NewMockService(t)
	mockStore := mocks_store.NewMockStore(t)

	scheduler := NewScheduler(mockService, mockStore, &mockLockService{})

	now := time.Date(2025, 1, 7, 7, 0, 0, 0, time.UTC)

	k8sConfig := &model.MaintenanceConfig{
		ActionType: "kubernetes_cleanup",
		Enabled:    true,
		Schedule:   "0 7 * * *",
		LastRun:    time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC),
	}

	mockService.EXPECT().
		GetMaintenanceConfigByAction("kubernetes_cleanup").
		Return(k8sConfig, nil)

	mockService.EXPECT().
		IsKubernetesCleanupAvailable().
		Return(false)

	scheduler.checkAction("kubernetes_cleanup", now)
	mockService.AssertExpectations(t)
}

func TestScheduler_CheckAction_Disabled(t *testing.T) {
	mockService := mocks_service.NewMockService(t)
	mockStore := mocks_store.NewMockStore(t)

	scheduler := NewScheduler(mockService, mockStore, &mockLockService{})

	now := time.Date(2025, 1, 7, 7, 0, 0, 0, time.UTC)

	config := &model.MaintenanceConfig{
		ActionType: "vacuum",
		Enabled:    false,
		Schedule:   "0 7 * * *",
	}

	mockService.EXPECT().
		GetMaintenanceConfigByAction("vacuum").
		Return(config, nil)

	scheduler.checkAction("vacuum", now)
	mockService.AssertExpectations(t)
}

func TestScheduler_CheckAction_StaleAgentCleanup(t *testing.T) {
	mockService := mocks_service.NewMockService(t)
	mockStore := mocks_store.NewMockStore(t)

	scheduler := NewScheduler(mockService, mockStore, &mockLockService{})
	scheduler.ctx = t.Context()

	now := time.Date(2025, 1, 7, 7, 0, 0, 0, time.UTC)

	config := &model.MaintenanceConfig{
		ActionType: "stale_agent_cleanup",
		Enabled:    true,
		Schedule:   "0 7 * * *",
		LastRun:    time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC),
	}

	mockService.EXPECT().
		GetMaintenanceConfigByAction("stale_agent_cleanup").
		Return(config, nil)

	mockService.EXPECT().
		RunStaleAgentCleanup(mock.Anything, mock.AnythingOfType("time.Duration")).
		Return(nil)

	scheduler.checkAction("stale_agent_cleanup", now)
	mockService.AssertExpectations(t)
}

// TestScheduler_CheckAction_StaleAgentCleanup_Disabled guards the fix for
// https://codefloe.com/crowci/crow/issues/1131: when the config is disabled
// (e.g. via CROW_MAINTENANCE_STALE_AGENT_CLEANUP_ENABLED=false reconciled into
// the DB at startup), the scheduler must not run the cleanup and delete agents.
func TestScheduler_CheckAction_StaleAgentCleanup_Disabled(t *testing.T) {
	mockService := mocks_service.NewMockService(t)
	mockStore := mocks_store.NewMockStore(t)

	scheduler := NewScheduler(mockService, mockStore, &mockLockService{})

	now := time.Date(2025, 1, 7, 7, 0, 0, 0, time.UTC)

	config := &model.MaintenanceConfig{
		ActionType: "stale_agent_cleanup",
		Enabled:    false,
		Schedule:   "0 7 * * *",
	}

	mockService.EXPECT().
		GetMaintenanceConfigByAction("stale_agent_cleanup").
		Return(config, nil)

	scheduler.checkAction("stale_agent_cleanup", now)

	// RunStaleAgentCleanup must never be called when disabled.
	mockService.AssertNotCalled(t, "RunStaleAgentCleanup", mock.Anything, mock.Anything)
	mockService.AssertExpectations(t)
}

func TestScheduler_CheckAction_NotTimeYet(t *testing.T) {
	mockService := mocks_service.NewMockService(t)
	mockStore := mocks_store.NewMockStore(t)

	scheduler := NewScheduler(mockService, mockStore, &mockLockService{})

	now := time.Date(2025, 1, 7, 6, 0, 0, 0, time.UTC)

	config := &model.MaintenanceConfig{
		ActionType: "vacuum",
		Enabled:    true,
		Schedule:   "0 7 * * *",
		LastRun:    time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC),
	}

	mockService.EXPECT().
		GetMaintenanceConfigByAction("vacuum").
		Return(config, nil)

	scheduler.checkAction("vacuum", now)
	mockService.AssertExpectations(t)
}

func TestScheduler_CheckAction_TooFrequent(t *testing.T) {
	mockService := mocks_service.NewMockService(t)
	mockStore := mocks_store.NewMockStore(t)

	scheduler := NewScheduler(mockService, mockStore, &mockLockService{})

	now := time.Date(2025, 1, 7, 7, 0, 0, 0, time.UTC)

	config := &model.MaintenanceConfig{
		ActionType: "vacuum",
		Enabled:    true,
		Schedule:   "* * * * *",
		LastRun:    now.Add(-10 * time.Second),
	}

	mockService.EXPECT().
		GetMaintenanceConfigByAction("vacuum").
		Return(config, nil)

	scheduler.checkAction("vacuum", now)
	mockService.AssertExpectations(t)
}

func TestScheduler_CheckAction_AllActionsRun(t *testing.T) {
	mockService := mocks_service.NewMockService(t)
	mockStore := mocks_store.NewMockStore(t)

	scheduler := NewScheduler(mockService, mockStore, &mockLockService{})
	scheduler.ctx = t.Context()

	vacuumConfig := &model.MaintenanceConfig{
		ActionType: "vacuum",
		Enabled:    true,
		Schedule:   "0 7 * * *",
		LastRun:    time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC),
	}

	k8sConfig := &model.MaintenanceConfig{
		ActionType: "kubernetes_cleanup",
		Enabled:    true,
		Schedule:   "0 7 * * *",
		LastRun:    time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC),
	}

	staleAgentConfig := &model.MaintenanceConfig{
		ActionType: "stale_agent_cleanup",
		Enabled:    true,
		Schedule:   "0 7 * * *",
		LastRun:    time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC),
	}

	orphanedDataConfig := &model.MaintenanceConfig{
		ActionType: "orphaned_data_cleanup",
		Enabled:    true,
		Schedule:   "0 7 * * *",
		LastRun:    time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC),
	}

	// Setup expectations - all four should run
	mockService.EXPECT().
		GetMaintenanceConfigByAction("vacuum").
		Return(vacuumConfig, nil)

	mockService.EXPECT().
		RunMaintenance(mock.Anything).
		Return(nil)

	mockService.EXPECT().
		GetMaintenanceConfigByAction("kubernetes_cleanup").
		Return(k8sConfig, nil)

	mockService.EXPECT().
		IsKubernetesCleanupAvailable().
		Return(true)

	mockService.EXPECT().
		RunKubernetesCleanup(mock.Anything, false).
		Return(nil)

	mockService.EXPECT().
		GetMaintenanceConfigByAction("stale_agent_cleanup").
		Return(staleAgentConfig, nil)

	mockService.EXPECT().
		RunStaleAgentCleanup(mock.Anything, mock.AnythingOfType("time.Duration")).
		Return(nil)

	mockService.EXPECT().
		GetMaintenanceConfigByAction("orphaned_data_cleanup").
		Return(orphanedDataConfig, nil)

	mockService.EXPECT().
		RunOrphanedDataCleanup(mock.Anything, false).
		Return(nil)

	scheduler.checkMaintenance()

	mockService.AssertExpectations(t)
}

func TestScheduler_CheckAction_UnknownActionType(t *testing.T) {
	mockService := mocks_service.NewMockService(t)
	mockStore := mocks_store.NewMockStore(t)

	scheduler := NewScheduler(mockService, mockStore, &mockLockService{})
	scheduler.ctx = t.Context()

	now := time.Date(2025, 1, 7, 7, 0, 0, 0, time.UTC)

	config := &model.MaintenanceConfig{
		ActionType: "unknown_action",
		Enabled:    true,
		Schedule:   "0 7 * * *",
		LastRun:    time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC),
	}

	mockService.EXPECT().
		GetMaintenanceConfigByAction("unknown_action").
		Return(config, nil)

	scheduler.checkAction("unknown_action", now)
	mockService.AssertExpectations(t)
}
