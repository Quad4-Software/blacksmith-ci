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

func TestScheduler_ConfigChangeDetection(t *testing.T) {
	mockService := mocks_service.NewMockService(t)
	mockStore := mocks_store.NewMockStore(t)

	scheduler := NewScheduler(mockService, mockStore, &mockLockService{})
	scheduler.ctx = t.Context()

	testTime := time.Date(2025, 1, 7, 7, 1, 0, 0, time.UTC)

	initialConfig := &model.MaintenanceConfig{
		ActionType: "kubernetes_cleanup",
		Enabled:    false,
		Schedule:   "0 7 * * *",
		LastRun:    time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC),
	}

	updatedConfig := &model.MaintenanceConfig{
		ActionType: "kubernetes_cleanup",
		Enabled:    true,
		Schedule:   "0 7 * * *",
		LastRun:    time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC),
	}

	mockService.EXPECT().
		GetMaintenanceConfigByAction("kubernetes_cleanup").
		Return(initialConfig, nil).Once()

	mockService.EXPECT().
		GetMaintenanceConfigByAction("kubernetes_cleanup").
		Return(updatedConfig, nil).Once()

	mockService.EXPECT().
		IsKubernetesCleanupAvailable().
		Return(true).Once()

	mockService.EXPECT().
		RunKubernetesCleanup(mock.Anything, false).
		Return(nil).Once()

	scheduler.checkAction("kubernetes_cleanup", testTime)
	scheduler.checkAction("kubernetes_cleanup", testTime)
	mockService.AssertExpectations(t)
}

func TestScheduler_ScheduleChangeDetection(t *testing.T) {
	mockService := mocks_service.NewMockService(t)
	mockStore := mocks_store.NewMockStore(t)

	scheduler := NewScheduler(mockService, mockStore, &mockLockService{})
	scheduler.ctx = t.Context()

	testTime1 := time.Date(2025, 1, 7, 6, 30, 0, 0, time.UTC)

	originalConfig := &model.MaintenanceConfig{
		ActionType: "kubernetes_cleanup",
		Enabled:    true,
		Schedule:   "0 7 * * *",
		LastRun:    time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC),
	}

	updatedConfig := &model.MaintenanceConfig{
		ActionType: "kubernetes_cleanup",
		Enabled:    true,
		Schedule:   "30 6 * * *",
		LastRun:    time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC),
	}

	mockService.EXPECT().
		GetMaintenanceConfigByAction("kubernetes_cleanup").
		Return(originalConfig, nil).Once()

	mockService.EXPECT().
		IsKubernetesCleanupAvailable().
		Return(true).Once()

	mockService.EXPECT().
		GetMaintenanceConfigByAction("kubernetes_cleanup").
		Return(updatedConfig, nil).Once()

	mockService.EXPECT().
		IsKubernetesCleanupAvailable().
		Return(true).Once()

	mockService.EXPECT().
		RunKubernetesCleanup(mock.Anything, false).
		Return(nil).Once()

	scheduler.checkAction("kubernetes_cleanup", testTime1)
	scheduler.checkAction("kubernetes_cleanup", testTime1)
	mockService.AssertExpectations(t)
}

func TestScheduler_LastRunTimeUpdatedAfterExecution(t *testing.T) {
	mockService := mocks_service.NewMockService(t)
	mockStore := mocks_store.NewMockStore(t)

	scheduler := NewScheduler(mockService, mockStore, &mockLockService{})
	scheduler.ctx = t.Context()

	testTime := time.Date(2025, 1, 7, 7, 1, 0, 0, time.UTC)

	configOldLastRun := &model.MaintenanceConfig{
		ActionType: "kubernetes_cleanup",
		Enabled:    true,
		Schedule:   "0 7 * * *",
		LastRun:    time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC),
	}

	configNewLastRun := &model.MaintenanceConfig{
		ActionType: "kubernetes_cleanup",
		Enabled:    true,
		Schedule:   "0 7 * * *",
		LastRun:    testTime,
	}

	mockService.EXPECT().
		GetMaintenanceConfigByAction("kubernetes_cleanup").
		Return(configOldLastRun, nil).Once()

	mockService.EXPECT().
		IsKubernetesCleanupAvailable().
		Return(true).Once()

	mockService.EXPECT().
		RunKubernetesCleanup(mock.Anything, false).
		Return(nil).Once()

	mockService.EXPECT().
		GetMaintenanceConfigByAction("kubernetes_cleanup").
		Return(configNewLastRun, nil).Once()

	mockService.EXPECT().
		IsKubernetesCleanupAvailable().
		Return(true).Once()

	scheduler.checkAction("kubernetes_cleanup", testTime)
	scheduler.checkAction("kubernetes_cleanup", testTime)

	mockService.AssertExpectations(t)
}
