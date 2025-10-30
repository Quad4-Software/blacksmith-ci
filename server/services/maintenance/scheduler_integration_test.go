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

//go:build integration

package maintenance

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/crowci/crow/v4/server/model"
	"codeberg.org/crowci/crow/v4/server/store"
	"codeberg.org/crowci/crow/v4/server/store/datastore"
)

func setupTestStore(t *testing.T) store.Store {
	opts := &store.Opts{
		Driver: "sqlite3",
		Config: ":memory:",
	}

	store, err := datastore.NewEngine(opts)
	require.NoError(t, err)

	err = store.Migrate(context.Background(), false)
	require.NoError(t, err)

	return store
}

func TestSchedulerIntegration_BothActionsScheduled(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	service := NewService(testStore)

	k8sCleanupCalled := false
	service.SetKubernetesCleanupFunc(func(ctx context.Context, dryRun bool) error {
		k8sCleanupCalled = true
		return nil
	})
	service.SetKubernetesCleanupEnabled(true)

	// Configure both actions to run immediately
	currentTime := time.Now().UTC()

	// Setup vacuum action - should run every minute for testing
	vacuumConfig := &model.MaintenanceConfig{
		Enabled:  true,
		Schedule: "* * * * *",                       // Every minute
		LastRun:  currentTime.Add(-2 * time.Minute), // Last run 2 minutes ago
	}
	err := service.UpdateMaintenanceConfigByAction("vacuum", vacuumConfig)
	require.NoError(t, err)

	// Setup kubernetes_cleanup action - should run every minute for testing
	k8sConfig := &model.MaintenanceConfig{
		Enabled:  true,
		Schedule: "* * * * *",                       // Every minute
		LastRun:  currentTime.Add(-2 * time.Minute), // Last run 2 minutes ago
	}
	err = service.UpdateMaintenanceConfigByAction("kubernetes_cleanup", k8sConfig)
	require.NoError(t, err)

	scheduler := NewScheduler(service, testStore)
	scheduler.ctx = ctx

	// Run check at a time when both should execute
	testTime := currentTime.Add(1 * time.Second)
	scheduler.checkAction("vacuum", testTime)
	scheduler.checkAction("kubernetes_cleanup", testTime)

	// Verify vacuum was called (check logs)
	logs, err := testStore.MaintenanceLogList(10)
	require.NoError(t, err)

	// Should have at least one log entry for vacuum
	vacuumExecuted := false
	for _, log := range logs {
		if log.Details != "" && log.Success {
			vacuumExecuted = true
			break
		}
	}
	assert.True(t, vacuumExecuted, "Vacuum action should have been executed")

	assert.True(t, k8sCleanupCalled, "Kubernetes cleanup should have been called")

	// Verify action configs were updated with last run times
	updatedVacuumConfig, err := service.GetMaintenanceConfigByAction("vacuum")
	require.NoError(t, err)
	assert.True(t, updatedVacuumConfig.LastRun.After(currentTime.Add(-1*time.Minute)), "Vacuum LastRun should be updated")

	updatedK8sConfig, err := service.GetMaintenanceConfigByAction("kubernetes_cleanup")
	require.NoError(t, err)
	assert.True(t, updatedK8sConfig.LastRun.After(currentTime.Add(-1*time.Minute)), "Kubernetes cleanup LastRun should be updated")
}

func TestSchedulerIntegration_OnlyEnabledActionsRun(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	service := NewService(testStore)

	k8sCleanupCalled := false
	service.SetKubernetesCleanupFunc(func(ctx context.Context, dryRun bool) error {
		k8sCleanupCalled = true
		return nil
	})
	service.SetKubernetesCleanupEnabled(true)

	currentTime := time.Now().UTC()

	// Setup vacuum action - DISABLED
	vacuumConfig := &model.MaintenanceConfig{
		Enabled:  false,
		Schedule: "* * * * *",
		LastRun:  currentTime.Add(-2 * time.Minute),
	}
	err := service.UpdateMaintenanceConfigByAction("vacuum", vacuumConfig)
	require.NoError(t, err)

	// Setup kubernetes_cleanup action - ENABLED
	k8sConfig := &model.MaintenanceConfig{
		Enabled:  true,
		Schedule: "* * * * *",
		LastRun:  currentTime.Add(-2 * time.Minute),
	}
	err = service.UpdateMaintenanceConfigByAction("kubernetes_cleanup", k8sConfig)
	require.NoError(t, err)

	scheduler := NewScheduler(service, testStore)
	scheduler.ctx = ctx

	testTime := currentTime.Add(1 * time.Second)
	scheduler.checkAction("vacuum", testTime)
	scheduler.checkAction("kubernetes_cleanup", testTime)

	// Vacuum should NOT have created a log (it's disabled)
	// Only kubernetes cleanup should have run
	assert.True(t, k8sCleanupCalled, "Kubernetes cleanup should have been called")

	// Check that vacuum config was NOT updated (since it didn't run)
	updatedVacuumConfig, err := service.GetMaintenanceConfigByAction("vacuum")
	require.NoError(t, err)
	assert.Equal(t, currentTime.Add(-2*time.Minute).Unix(), updatedVacuumConfig.LastRun.Unix(), "Vacuum LastRun should NOT be updated")
}

func TestSchedulerIntegration_RespectsCronSchedule(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	service := NewService(testStore)

	k8sCleanupCalled := false
	service.SetKubernetesCleanupFunc(func(ctx context.Context, dryRun bool) error {
		k8sCleanupCalled = true
		return nil
	})
	service.SetKubernetesCleanupEnabled(true)

	// Setup kubernetes_cleanup to run at 7:00 daily
	k8sConfig := &model.MaintenanceConfig{
		Enabled:  true,
		Schedule: "0 7 * * *",                                 // Daily at 07:00
		LastRun:  time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC), // Yesterday at 07:00
	}
	err := service.UpdateMaintenanceConfigByAction("kubernetes_cleanup", k8sConfig)
	require.NoError(t, err)

	scheduler := NewScheduler(service, testStore)
	scheduler.ctx = ctx

	k8sCleanupCalled = false
	testTime := time.Date(2025, 1, 7, 6, 59, 0, 0, time.UTC)
	scheduler.checkAction("kubernetes_cleanup", testTime)
	assert.False(t, k8sCleanupCalled, "Kubernetes cleanup should NOT run at 06:59")

	k8sCleanupCalled = false
	testTime = time.Date(2025, 1, 7, 7, 0, 0, 0, time.UTC)
	scheduler.checkAction("kubernetes_cleanup", testTime)
	assert.True(t, k8sCleanupCalled, "Kubernetes cleanup SHOULD run at 07:00")

	k8sCleanupCalled = false
	testTime = time.Date(2025, 1, 7, 7, 1, 0, 0, time.UTC)
	scheduler.checkAction("kubernetes_cleanup", testTime)
	assert.True(t, k8sCleanupCalled, "Kubernetes cleanup SHOULD run at 07:01 (still after scheduled time)")
}
