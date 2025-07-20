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

	"codeberg.org/crowci/crow/v4/server/model"
	"codeberg.org/crowci/crow/v4/server/store"
)

// MaintenanceService defines the interface for database maintenance operations.
type MaintenanceService interface {
	// RunMaintenance executes database maintenance operations
	RunMaintenance(ctx context.Context) error
	// GetMaintenanceConfig returns the current maintenance configuration
	GetMaintenanceConfig() *model.MaintenanceConfig
	// UpdateMaintenanceConfig updates the maintenance configuration
	UpdateMaintenanceConfig(config *model.MaintenanceConfig) error
	// GetMaintenanceStats returns maintenance statistics
	GetMaintenanceStats() (*model.MaintenanceStats, error)
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
