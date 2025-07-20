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
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v4/server/model"
	"codeberg.org/crowci/crow/v4/server/store"
	"codeberg.org/crowci/crow/v4/server/store/datastore"
)

// Service defines the interface for database maintenance operations.
type Service interface {
	// RunMaintenance executes database maintenance operations
	RunMaintenance(ctx context.Context) error
	// GetMaintenanceConfig returns the current maintenance configuration
	GetMaintenanceConfig() *model.MaintenanceConfig
	// UpdateMaintenanceConfig updates the maintenance configuration
	UpdateMaintenanceConfig(config *model.MaintenanceConfig) error
	// GetMaintenanceStats returns maintenance statistics
	GetMaintenanceStats() (*model.MaintenanceStats, error)
}

type service struct {
	store store.Store
}

// New creates a new maintenance service.
func New(store store.Store) Service {
	return &service{
		store: store,
	}
}

// NewService creates a new maintenance service with only a store.
func NewService(store store.Store) Service {
	return &service{
		store: store,
	}
}

// RunMaintenance executes database maintenance operations based on the database type.
func (s *service) RunMaintenance(ctx context.Context) error {
	start := time.Now()
	log.Info().Msg("Starting database maintenance operations")

	// Try each database type maintenance operation
	// The store methods will handle the database-specific logic
	var err error
	var driverName string

	// Try PostgreSQL first
	if err = s.store.MaintenanceRunPostgreSQL(ctx); err == nil {
		driverName = "postgres"
	} else if err = s.store.MaintenanceRunMySQL(ctx); err == nil {
		driverName = "mysql"
	} else if err = s.store.MaintenanceRunSQLite(ctx); err == nil {
		driverName = "sqlite3"
	} else {
		log.Warn().Msg("Unable to determine database type or maintenance failed")
		return fmt.Errorf("database maintenance failed: %w", err)
	}

	duration := time.Since(start)

	// Get size information from the store layer
	beforeSize, afterSize, spaceSaved := datastore.GetMaintenanceSizeInfo()
	// Log the maintenance operation with more details
	details := fmt.Sprintf("Database: %s, Operations: VACUUM/OPTIMIZE log_entries table, Duration: %v", driverName, duration)

	// Add size information if available
	log.Info().
		Int64("debug_before_size", beforeSize).
		Int64("debug_after_size", afterSize).
		Int64("debug_space_saved", spaceSaved).
		Str("driver", driverName).
		Msg("Size information retrieved")

	if beforeSize > 0 || afterSize > 0 {
		const bytesPerMB = 1024 * 1024
		beforeSizeMB := float64(beforeSize) / bytesPerMB
		afterSizeMB := float64(afterSize) / bytesPerMB
		spaceSavedMB := float64(spaceSaved) / bytesPerMB
		details = fmt.Sprintf("Database: %s, Operations: VACUUM/OPTIMIZE log_entries table, Duration: %v, Before: %.2f MB, After: %.2f MB, Saved: %.2f MB",
			driverName, duration, beforeSizeMB, afterSizeMB, spaceSavedMB)
	}

	logEntry := &model.MaintenanceLog{
		RunTime:  start,
		Success:  err == nil,
		Duration: duration.Milliseconds(),
		Details:  details,
		Created:  start.Unix(),
	}

	if err != nil {
		logEntry.Error = err.Error()
		log.Error().Err(err).Str("driver", driverName).Dur("duration", duration).Msg("Database maintenance failed")

		// Update statistics
		s.updateMaintenanceStats(false, duration)

		// Log the failure
		if err := s.store.MaintenanceLogCreate(logEntry); err != nil {
			log.Error().Err(err).Msg("Failed to create maintenance log entry")
		}

		return fmt.Errorf("maintenance failed for %s: %w", driverName, err)
	}

	// Update statistics
	s.updateMaintenanceStats(true, duration)

	// Log the success
	if err := s.store.MaintenanceLogCreate(logEntry); err != nil {
		log.Error().Err(err).Msg("Failed to create maintenance log entry")
	}

	log.Info().Str("driver", driverName).Dur("duration", duration).Msg("Database maintenance completed successfully")
	return nil
}

// updateMaintenanceStats updates the maintenance statistics.
func (s *service) updateMaintenanceStats(success bool, duration time.Duration) {
	stats, err := s.store.MaintenanceStatsGet()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get maintenance statistics")
		return
	}

	// Update statistics
	stats.LastRun = time.Now()
	stats.LastRunSuccess = success
	stats.LastRunDuration = duration.Milliseconds()
	stats.TotalRuns++
	stats.Updated = time.Now().Unix()

	if success {
		stats.SuccessfulRuns++
		stats.LastRunError = ""
	} else {
		stats.LastRunError = "Maintenance operation failed"
	}

	if err := s.store.MaintenanceStatsUpdate(stats); err != nil {
		log.Error().Err(err).Msg("Failed to update maintenance statistics")
	}
}

// GetMaintenanceConfig returns the current maintenance configuration.
func (s *service) GetMaintenanceConfig() *model.MaintenanceConfig {
	config, err := s.store.MaintenanceConfigGet()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get maintenance config, using defaults")
		return &model.MaintenanceConfig{
			Enabled:  true,
			Schedule: "0 5 * * *", // 5 AM daily
		}
	}
	return config
}

// UpdateMaintenanceConfig updates the maintenance configuration.
func (s *service) UpdateMaintenanceConfig(config *model.MaintenanceConfig) error {
	return s.store.MaintenanceConfigUpdate(config)
}

// GetMaintenanceStats returns maintenance statistics.
func (s *service) GetMaintenanceStats() (*model.MaintenanceStats, error) {
	return s.store.MaintenanceStatsGet()
}
