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

package services

import (
	"context"
	"time"

	"codefloe.com/crowci/crow/v5/pipeline/rpc"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store"
)

// MaintenanceService defines the interface for database maintenance operations.
type MaintenanceService interface {
	// RunMaintenance executes database maintenance operations
	RunMaintenance(ctx context.Context) error
	// RunKubernetesCleanup executes Kubernetes resource cleanup
	RunKubernetesCleanup(ctx context.Context, dryRun bool) error
	// RunStaleAgentCleanup executes stale agent cleanup
	RunStaleAgentCleanup(ctx context.Context, staleDuration time.Duration) error
	// RunOrphanedDataCleanup detects and cleans up DB rows that reference
	// deleted forge users or repositories
	RunOrphanedDataCleanup(ctx context.Context, dryRun bool) error
	// GetMaintenanceConfig returns the current maintenance configuration
	GetMaintenanceConfig() *model.MaintenanceConfig
	// UpdateMaintenanceConfig updates the maintenance configuration
	UpdateMaintenanceConfig(config *model.MaintenanceConfig) error
	// GetMaintenanceConfigByAction returns config for a specific action type
	GetMaintenanceConfigByAction(actionType string) (*model.MaintenanceConfig, error)
	// UpdateMaintenanceConfigByAction updates config for a specific action type
	UpdateMaintenanceConfigByAction(actionType string, config *model.MaintenanceConfig) error
	// IsKubernetesCleanupAvailable returns whether Kubernetes cleanup is available
	IsKubernetesCleanupAvailable() bool
	// GetMaintenanceStats returns maintenance statistics
	GetMaintenanceStats() (*model.MaintenanceStats, error)
	// SetKubernetesCleanupFunc sets the function to use for Kubernetes cleanup
	SetKubernetesCleanupFunc(f func(ctx context.Context, dryRun bool) error)
	// SetKubernetesCleanupEnabled sets whether Kubernetes cleanup is enabled for execution
	SetKubernetesCleanupEnabled(enabled bool)
	// SetVacuumEnvVarsConfigured sets whether vacuum-specific environment variables are overriding configuration
	SetVacuumEnvVarsConfigured(vacuumEnvVarsSet bool)
	// IsVacuumEnvVarsConfigured returns whether vacuum-specific environment variables are set
	IsVacuumEnvVarsConfigured() bool
	// SetKubernetesEnvVarsConfigured sets whether Kubernetes-specific environment variables are set
	SetKubernetesEnvVarsConfigured(k8sEnvVarsSet bool)
	// IsKubernetesEnvVarsConfigured returns whether Kubernetes-specific environment variables are set
	IsKubernetesEnvVarsConfigured() bool
	// SetOrphanedDataEnvVarsConfigured sets whether orphaned-data-cleanup-specific environment variables are set
	SetOrphanedDataEnvVarsConfigured(orphanEnvVarsSet bool)
	// IsOrphanedDataEnvVarsConfigured returns whether orphaned-data-cleanup-specific environment variables are set
	IsOrphanedDataEnvVarsConfigured() bool
	// IsDockerCleanupAvailable returns whether Docker cleanup is available (a Docker agent is connected)
	IsDockerCleanupAvailable() bool
	// SetDockerCleanupAvailable sets whether Docker cleanup is available
	SetDockerCleanupAvailable(available bool)
	// GetDockerCleanupConfigForAgent returns Docker cleanup config to send to agents via health response
	GetDockerCleanupConfigForAgent() *rpc.DockerCleanupConfig
	// RecordDockerCleanupResult records a Docker cleanup result from an agent
	RecordDockerCleanupResult(agentID int64, agentName string, result rpc.DockerCleanupResult) error
	// SetDockerCleanupRunNow triggers an immediate Docker cleanup on the next agent health report
	SetDockerCleanupRunNow()
}

// MaintenanceScheduler defines the interface for maintenance scheduling.
type MaintenanceScheduler interface {
	// Start begins the maintenance scheduler
	Start() error
	// Stop stops the maintenance scheduler
	Stop()
	// UpdateSchedule updates the maintenance schedule
	UpdateSchedule(schedule string, enabled bool) error
	// RunManual runs maintenance manually (outside of schedule)
	RunManual() error
}

// Store interface for maintenance service (to avoid circular imports).
type Store interface {
	store.Store
}
