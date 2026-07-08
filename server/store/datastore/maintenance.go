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

package datastore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v6/server/model"
)

// Global variable to store maintenance size info temporarily.
var (
	maintenanceSizeInfo struct {
		sync.RWMutex
		beforeSize int64
		afterSize  int64
		spaceSaved int64
	}
)

// GetMaintenanceSizeInfo returns the size info from the last maintenance operation.
func GetMaintenanceSizeInfo() (beforeSize, afterSize, spaceSaved int64) {
	maintenanceSizeInfo.RLock()
	defer maintenanceSizeInfo.RUnlock()
	return maintenanceSizeInfo.beforeSize, maintenanceSizeInfo.afterSize, maintenanceSizeInfo.spaceSaved
}

// MaintenanceConfigGet returns the vacuum maintenance configuration with runtime flags.
func (s storage) MaintenanceConfigGet() (*model.MaintenanceConfig, error) {
	ctx := context.Background()

	// Get both vacuum and kubernetes configs
	vacuumConfig := &model.MaintenanceConfig{}
	err := s.db.NewSelect().Model(vacuumConfig).Where("action_type = ?", "vacuum").Scan(ctx)
	hasVacuum := err == nil

	k8sConfig := &model.MaintenanceConfig{}
	err = s.db.NewSelect().Model(k8sConfig).Where("action_type = ?", "kubernetes_cleanup").Scan(ctx)
	hasK8s := err == nil

	// If neither exists, create defaults
	if !hasVacuum && !hasK8s {
		// Create default vacuum config
		vacuumConfig = &model.MaintenanceConfig{
			ActionType: "vacuum",
			Enabled:    true,
			Schedule:   "0 5 * * *", // 5 AM daily
			Created:    time.Now().Unix(),
			Updated:    time.Now().Unix(),
		}
		if _, err := s.db.NewInsert().Model(vacuumConfig).Exec(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to create default vacuum config")
		}

		// Create default kubernetes config
		k8sConfig = &model.MaintenanceConfig{
			ActionType: "kubernetes_cleanup",
			Enabled:    false,
			Schedule:   "0 5 * * *", // Daily at 5 AM
			DryRun:     false,
			Created:    time.Now().Unix(),
			Updated:    time.Now().Unix(),
		}
		if _, err := s.db.NewInsert().Model(k8sConfig).Exec(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to create default kubernetes config")
		}
	}

	// Return vacuum config with runtime flags populated
	combinedConfig := &model.MaintenanceConfig{
		ID: 0, // Use ID 0 to indicate this is a combined config, not a specific action
		// Vacuum settings (default)
		Enabled:  true,
		Schedule: "0 5 * * *",
	}

	// Override with actual vacuum config if it exists
	if hasVacuum {
		combinedConfig.Enabled = vacuumConfig.Enabled
		combinedConfig.Schedule = vacuumConfig.Schedule
		combinedConfig.Created = vacuumConfig.Created
		combinedConfig.Updated = vacuumConfig.Updated
	}

	// Use the most recent update time from k8s config if it exists
	if hasK8s && k8sConfig.Updated > combinedConfig.Updated {
		combinedConfig.Updated = k8sConfig.Updated
	}

	log.Info().
		Bool("vacuum_enabled", combinedConfig.Enabled).
		Str("vacuum_schedule", combinedConfig.Schedule).
		Msg("Store: Returning combined config")

	return combinedConfig, nil
}

// MaintenanceConfigGetByActionType returns config for a specific action type.
func (s storage) MaintenanceConfigGetByActionType(actionType string) (*model.MaintenanceConfig, error) {
	ctx := context.Background()
	config := &model.MaintenanceConfig{}
	err := s.db.NewSelect().Model(config).Where("action_type = ?", actionType).Scan(ctx)
	if err == nil {
		return config, nil
	}

	// Record not found — create default config for this action type
	defaultConfig := &model.MaintenanceConfig{
		ActionType: actionType,
		Created:    time.Now().Unix(),
		Updated:    time.Now().Unix(),
	}

	switch actionType {
	case "vacuum":
		defaultConfig.Enabled = true
		defaultConfig.Schedule = "0 5 * * *" // Daily at 5 AM
	case "kubernetes_cleanup":
		defaultConfig.Enabled = false
		defaultConfig.Schedule = "0 5 * * *" // Daily at 5 AM
		defaultConfig.DryRun = false
	case "stale_agent_cleanup":
		defaultConfig.Enabled = true
		defaultConfig.Schedule = "0 5 * * *" // Daily at 5 AM
	case "docker_cleanup":
		defaultConfig.Enabled = false
		defaultConfig.Schedule = "0 */1 * * *" // Hourly
		defaultConfig.StaleThreshold = 3600    // 1 hour
	case "orphaned_data_cleanup":
		defaultConfig.Enabled = true
		defaultConfig.Schedule = "0 5 * * *" // Daily at 5 AM
	}

	// Use Insert with unique constraint handling to avoid race conditions
	// If another instance already created it, just fetch and return
	if _, err := s.db.NewInsert().Model(defaultConfig).Exec(ctx); err != nil {
		// If insert failed due to unique constraint, try to fetch again
		config = &model.MaintenanceConfig{}
		if fetchErr := s.db.NewSelect().Model(config).Where("action_type = ?", actionType).Scan(ctx); fetchErr == nil {
			return config, nil
		}
		// If still not found, return the original error
		return nil, err
	}
	return defaultConfig, nil
}

// MaintenanceConfigUpdate updates or creates the maintenance configuration.
func (s storage) MaintenanceConfigUpdate(config *model.MaintenanceConfig) error {
	log.Info().
		Bool("incoming_enabled", config.Enabled).
		Str("incoming_schedule", config.Schedule).
		Msg("Store: Updating maintenance config")

	// Wrap the read-modify-write in a transaction so a concurrent writer can't
	// slip between the existence check and the update/insert.
	return s.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		// Get existing config - there should only be one after merging.
		// Order deterministically so the "one row" we pick is stable.
		existing := &model.MaintenanceConfig{}
		err := tx.NewSelect().Model(existing).Order("id ASC").Limit(1).Scan(ctx)
		has := err == nil

		if has {
			config.ID = existing.ID
			config.Created = existing.Created
			config.Updated = time.Now().Unix()

			// Update only the vacuum-related columns
			_, err := tx.NewUpdate().Model(config).
				Column("enabled", "schedule", "updated").
				Where("id = ?", existing.ID).
				Exec(ctx)
			return err
		}

		// Create new config
		_, err = tx.NewInsert().Model(config).Exec(ctx)
		return err
	})
}

// MaintenanceConfigUpdateByActionType updates config for a specific action type.
func (s storage) MaintenanceConfigUpdateByActionType(actionType string, updates *model.MaintenanceConfig) error {
	ctx := context.Background()

	log.Info().
		Str("action_type", actionType).
		Bool("enabled", updates.Enabled).
		Str("schedule", updates.Schedule).
		Msg("Store: Updating config by action type")

	// Get existing config for this action type
	existing := &model.MaintenanceConfig{}
	err := s.db.NewSelect().Model(existing).Where("action_type = ?", actionType).Scan(ctx)
	has := err == nil

	if has {
		// Update existing config
		existing.Enabled = updates.Enabled
		existing.Schedule = updates.Schedule
		existing.DryRun = updates.DryRun
		existing.StaleThreshold = updates.StaleThreshold
		existing.Updated = time.Now().Unix()

		// Only update statistics if they were provided (non-zero values indicate stats update)
		updateCols := []string{"enabled", "schedule", "dry_run", "stale_threshold", "updated"}
		if updates.TotalRuns > 0 || !updates.LastRun.IsZero() {
			// This is a statistics update
			existing.TotalRuns = updates.TotalRuns
			existing.SuccessfulRuns = updates.SuccessfulRuns
			existing.LastRun = updates.LastRun
			existing.LastRunSuccess = updates.LastRunSuccess
			existing.LastRunDuration = updates.LastRunDuration
			existing.LastRunError = updates.LastRunError
			updateCols = append(updateCols, "total_runs", "successful_runs", "last_run", "last_run_success", "last_run_duration", "last_run_error")
		}

		res, err := s.db.NewUpdate().Model(existing).
			Column(updateCols...).
			Where("id = ?", existing.ID).
			Exec(ctx)
		affected, _ := res.RowsAffected()
		log.Info().
			Str("action_type", actionType).
			Int64("affected_rows", affected).
			Err(err).
			Msg("Store: Updated existing config")

		return err
	}

	// Create new config
	newConfig := &model.MaintenanceConfig{
		ActionType: actionType,
		Enabled:    updates.Enabled,
		Schedule:   updates.Schedule,
		DryRun:     updates.DryRun,
		Created:    time.Now().Unix(),
		Updated:    time.Now().Unix(),
	}

	_, err = s.db.NewInsert().Model(newConfig).Exec(ctx)
	log.Info().
		Str("action_type", actionType).
		Err(err).
		Msg("Store: Created new config")

	return err
}

// MaintenanceStatsGet returns the maintenance statistics.
func (s storage) MaintenanceStatsGet() (*model.MaintenanceStats, error) {
	ctx := context.Background()
	stats := &model.MaintenanceStats{}

	err := s.db.NewSelect().Model(stats).Limit(1).Scan(ctx)
	if err != nil {
		// Return default statistics if none exist
		return &model.MaintenanceStats{
			TotalRuns:      0,
			SuccessfulRuns: 0,
		}, nil
	}

	return stats, nil
}

// MaintenanceStatsUpdate updates or creates the maintenance statistics.
func (s storage) MaintenanceStatsUpdate(stats *model.MaintenanceStats) error {
	ctx := context.Background()

	// Check if stats exist
	existing := &model.MaintenanceStats{}
	err := s.db.NewSelect().Model(existing).Limit(1).Scan(ctx)
	has := err == nil

	if has {
		// Update existing stats
		stats.ID = existing.ID
		stats.Created = existing.Created
		_, err = s.db.NewUpdate().Model(stats).WherePK().Exec(ctx)
		return err
	}

	// Create new stats
	_, err = s.db.NewInsert().Model(stats).Exec(ctx)
	return err
}

// MaintenanceLogCreate creates a new maintenance log entry.
func (s storage) MaintenanceLogCreate(logEntry *model.MaintenanceLog) error {
	_, err := s.db.NewInsert().Model(logEntry).Exec(context.Background())
	return err
}

// MaintenanceLogList returns recent maintenance log entries.
func (s storage) MaintenanceLogList(limit int) ([]*model.MaintenanceLog, error) {
	if limit <= 0 {
		limit = 50
	}

	logs := make([]*model.MaintenanceLog, 0)
	err := s.db.NewSelect().Model(&logs).
		Order("run_time DESC").
		Limit(limit).
		Scan(context.Background())

	return logs, err
}

// MaintenanceRun executes database-specific maintenance operations based on the driver.
func (s storage) MaintenanceRun(ctx context.Context) error {
	switch s.driver {
	case DriverPostgres:
		return s.maintenanceRunPostgreSQL(ctx)
	case DriverMysql:
		return s.maintenanceRunMySQL(ctx)
	default:
		return s.maintenanceRunSQLite(ctx)
	}
}

// maintenanceRunPostgreSQL executes PostgreSQL-specific maintenance operations.
func (s storage) maintenanceRunPostgreSQL(ctx context.Context) error {
	// Get table size before vacuum
	var beforeSize int64
	if err := s.db.NewRaw("SELECT pg_total_relation_size('log_entries')").Scan(ctx, &beforeSize); err != nil {
		log.Error().Err(err).Msg("Failed to get PostgreSQL table size before vacuum")
		beforeSize = 0 // Continue if we can't get size
	} else {
		log.Info().Int64("before_size", beforeSize).Msg("PostgreSQL table size before vacuum")
	}

	// VACUUM FULL log_entries to reclaim space
	if _, err := s.db.ExecContext(ctx, "VACUUM FULL log_entries"); err != nil {
		return fmt.Errorf("failed to vacuum log_entries table: %w", err)
	}

	// ANALYZE to update statistics
	if _, err := s.db.ExecContext(ctx, "ANALYZE"); err != nil {
		return fmt.Errorf("failed to analyze tables: %w", err)
	}

	// Get table size after vacuum
	var afterSize int64
	if err := s.db.NewRaw("SELECT pg_total_relation_size('log_entries')").Scan(ctx, &afterSize); err != nil {
		log.Error().Err(err).Msg("Failed to get PostgreSQL table size after vacuum")
		afterSize = 0 // Continue if we can't get size
	} else {
		log.Info().Int64("after_size", afterSize).Msg("PostgreSQL table size after vacuum")
	}

	// Calculate space saved
	spaceSaved := beforeSize - afterSize
	if spaceSaved < 0 {
		spaceSaved = 0
	}

	// Store size info for service layer
	maintenanceSizeInfo.Lock()
	maintenanceSizeInfo.beforeSize = beforeSize
	maintenanceSizeInfo.afterSize = afterSize
	maintenanceSizeInfo.spaceSaved = spaceSaved
	maintenanceSizeInfo.Unlock()

	// Log size info
	log.Info().
		Int64("before_size", beforeSize).
		Int64("after_size", afterSize).
		Int64("space_saved", spaceSaved).
		Msg("PostgreSQL maintenance completed with size info stored")

	return nil
}

// maintenanceRunMySQL executes MySQL-specific maintenance operations.
func (s storage) maintenanceRunMySQL(ctx context.Context) error {
	// Get table size before optimize
	var beforeSize int64
	if err := s.db.NewRaw("SELECT (data_length + index_length) as size FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'log_entries'").Scan(ctx, &beforeSize); err != nil {
		log.Error().Err(err).Msg("Failed to get MySQL table size before optimize")
		beforeSize = 0 // Continue if we can't get size
	} else {
		log.Info().Int64("before_size", beforeSize).Msg("MySQL table size before optimize")
	}

	// OPTIMIZE TABLE log_entries
	if _, err := s.db.ExecContext(ctx, "OPTIMIZE TABLE log_entries"); err != nil {
		return fmt.Errorf("failed to optimize log_entries table: %w", err)
	}

	// ANALYZE TABLE to update statistics
	if _, err := s.db.ExecContext(ctx, "ANALYZE TABLE log_entries"); err != nil {
		return fmt.Errorf("failed to analyze log_entries table: %w", err)
	}

	// Get table size after optimize
	var afterSize int64
	if err := s.db.NewRaw("SELECT (data_length + index_length) as size FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'log_entries'").Scan(ctx, &afterSize); err != nil {
		log.Error().Err(err).Msg("Failed to get MySQL table size after optimize")
		afterSize = 0 // Continue if we can't get size
	} else {
		log.Info().Int64("after_size", afterSize).Msg("MySQL table size after optimize")
	}

	// Calculate space saved
	spaceSaved := beforeSize - afterSize
	if spaceSaved < 0 {
		spaceSaved = 0
	}

	// Store size info for service layer
	maintenanceSizeInfo.Lock()
	maintenanceSizeInfo.beforeSize = beforeSize
	maintenanceSizeInfo.afterSize = afterSize
	maintenanceSizeInfo.spaceSaved = spaceSaved
	maintenanceSizeInfo.Unlock()

	// Log detailed results
	log.Info().
		Int64("before_size", beforeSize).
		Int64("after_size", afterSize).
		Int64("space_saved", spaceSaved).
		Msg("MySQL maintenance completed with size info stored")

	return nil
}

// maintenanceRunSQLite executes SQLite-specific maintenance operations.
func (s storage) maintenanceRunSQLite(ctx context.Context) error {
	// Get database size before vacuum
	var beforeSize int64
	var pageCount, pageSize int64

	// Try to get page count and page size
	if err := s.db.NewRaw("PRAGMA page_count").Scan(ctx, &pageCount); err == nil {
		if err := s.db.NewRaw("PRAGMA page_size").Scan(ctx, &pageSize); err == nil {
			beforeSize = pageCount * pageSize
			log.Info().Int64("page_count", pageCount).Int64("page_size", pageSize).Int64("total_size", beforeSize).Msg("SQLite size before vacuum")
		} else {
			log.Error().Err(err).Msg("Failed to get page size before vacuum")
			beforeSize = 0
		}
	} else {
		log.Error().Err(err).Msg("Failed to get page count before vacuum")
		beforeSize = 0
	}

	// VACUUM to reclaim space
	if _, err := s.db.ExecContext(ctx, "VACUUM"); err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}

	// PRAGMA optimize to update statistics
	if _, err := s.db.ExecContext(ctx, "PRAGMA optimize"); err != nil {
		return fmt.Errorf("failed to optimize database: %w", err)
	}

	// Get database size after vacuum
	var afterSize int64
	var pageCountAfter, pageSizeAfter int64

	// Try to get page count and page size after vacuum
	if err := s.db.NewRaw("PRAGMA page_count").Scan(ctx, &pageCountAfter); err == nil {
		if err := s.db.NewRaw("PRAGMA page_size").Scan(ctx, &pageSizeAfter); err == nil {
			afterSize = pageCountAfter * pageSizeAfter
			log.Info().Int64("page_count", pageCountAfter).Int64("page_size", pageSizeAfter).Int64("total_size", afterSize).Msg("SQLite size after vacuum")
		} else {
			log.Error().Err(err).Msg("Failed to get page size after vacuum")
			afterSize = 0
		}
	} else {
		log.Error().Err(err).Msg("Failed to get page count after vacuum")
		afterSize = 0
	}

	// Calculate space saved
	spaceSaved := beforeSize - afterSize
	if spaceSaved < 0 {
		spaceSaved = 0
	}

	// Store size info for service layer
	maintenanceSizeInfo.Lock()
	maintenanceSizeInfo.beforeSize = beforeSize
	maintenanceSizeInfo.afterSize = afterSize
	maintenanceSizeInfo.spaceSaved = spaceSaved
	maintenanceSizeInfo.Unlock()

	// Log detailed results
	log.Info().
		Int64("before_size", beforeSize).
		Int64("after_size", afterSize).
		Int64("space_saved", spaceSaved).
		Msg("SQLite maintenance completed with size info stored")

	return nil
}
