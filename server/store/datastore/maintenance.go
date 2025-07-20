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

	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v4/server/model"
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

// MaintenanceConfigGet returns the maintenance configuration.
func (s storage) MaintenanceConfigGet() (*model.MaintenanceConfig, error) {
	config := &model.MaintenanceConfig{}

	has, err := s.engine.Get(config)
	if err != nil {
		return nil, err
	}

	if !has {
		// Return default configuration if none exists
		return &model.MaintenanceConfig{
			Enabled:  true,
			Schedule: "0 5 * * *", // 5 AM daily
		}, nil
	}

	return config, nil
}

// MaintenanceConfigUpdate updates or creates the maintenance configuration.
func (s storage) MaintenanceConfigUpdate(config *model.MaintenanceConfig) error {
	// Check if config exists
	existing := &model.MaintenanceConfig{}
	has, err := s.engine.Get(existing)
	if err != nil {
		return err
	}

	if has {
		// Update existing config
		config.ID = existing.ID
		config.Created = existing.Created
		_, err = s.engine.ID(existing.ID).Update(config)
		return err
	}

	// Create new config
	_, err = s.engine.Insert(config)
	return err
}

// MaintenanceStatsGet returns the maintenance statistics.
func (s storage) MaintenanceStatsGet() (*model.MaintenanceStats, error) {
	stats := &model.MaintenanceStats{}

	has, err := s.engine.Get(stats)
	if err != nil {
		return nil, err
	}

	if !has {
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
	// Check if stats exist
	existing := &model.MaintenanceStats{}
	has, err := s.engine.Get(existing)
	if err != nil {
		return err
	}

	if has {
		// Update existing stats
		stats.ID = existing.ID
		stats.Created = existing.Created
		_, err = s.engine.ID(existing.ID).Update(stats)
		return err
	}

	// Create new stats
	_, err = s.engine.Insert(stats)
	return err
}

// MaintenanceLogCreate creates a new maintenance log entry.
func (s storage) MaintenanceLogCreate(log *model.MaintenanceLog) error {
	_, err := s.engine.Insert(log)
	return err
}

// MaintenanceLogList returns recent maintenance log entries.
func (s storage) MaintenanceLogList(limit int) ([]*model.MaintenanceLog, error) {
	if limit <= 0 {
		limit = 50
	}

	logs := make([]*model.MaintenanceLog, 0)
	err := s.engine.
		Desc("run_time").
		Limit(limit).
		Find(&logs)

	return logs, err
}

// MaintenanceRunPostgreSQL executes PostgreSQL-specific maintenance operations.
func (s storage) MaintenanceRunPostgreSQL(ctx context.Context) error {
	// Get table size before vacuum
	var beforeSize int64
	if _, err := s.engine.Context(ctx).SQL("SELECT pg_total_relation_size('log_entries')").Get(&beforeSize); err != nil {
		log.Error().Err(err).Msg("Failed to get PostgreSQL table size before vacuum")
		beforeSize = 0 // Continue if we can't get size
	} else {
		log.Info().Int64("before_size", beforeSize).Msg("PostgreSQL table size before vacuum")
	}

	// VACUUM FULL log_entries to reclaim space
	if _, err := s.engine.Context(ctx).Exec("VACUUM FULL log_entries"); err != nil {
		return fmt.Errorf("failed to vacuum log_entries table: %w", err)
	}

	// ANALYZE to update statistics
	if _, err := s.engine.Context(ctx).Exec("ANALYZE"); err != nil {
		return fmt.Errorf("failed to analyze tables: %w", err)
	}

	// Get table size after vacuum
	var afterSize int64
	if _, err := s.engine.Context(ctx).SQL("SELECT pg_total_relation_size('log_entries')").Get(&afterSize); err != nil {
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

// MaintenanceRunMySQL executes MySQL-specific maintenance operations.
func (s storage) MaintenanceRunMySQL(ctx context.Context) error {
	// Get table size before optimize
	var beforeSize int64
	if _, err := s.engine.Context(ctx).SQL("SELECT (data_length + index_length) as size FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'log_entries'").Get(&beforeSize); err != nil {
		log.Error().Err(err).Msg("Failed to get MySQL table size before optimize")
		beforeSize = 0 // Continue if we can't get size
	} else {
		log.Info().Int64("before_size", beforeSize).Msg("MySQL table size before optimize")
	}

	// OPTIMIZE TABLE log_entries
	if _, err := s.engine.Context(ctx).Exec("OPTIMIZE TABLE log_entries"); err != nil {
		return fmt.Errorf("failed to optimize log_entries table: %w", err)
	}

	// ANALYZE TABLE to update statistics
	if _, err := s.engine.Context(ctx).Exec("ANALYZE TABLE log_entries"); err != nil {
		return fmt.Errorf("failed to analyze log_entries table: %w", err)
	}

	// Get table size after optimize
	var afterSize int64
	if _, err := s.engine.Context(ctx).SQL("SELECT (data_length + index_length) as size FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'log_entries'").Get(&afterSize); err != nil {
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

// MaintenanceRunSQLite executes SQLite-specific maintenance operations.
func (s storage) MaintenanceRunSQLite(ctx context.Context) error {
	// Get database size before vacuum
	var beforeSize int64
	var pageCount, pageSize int64

	// Try to get page count and page size
	if _, err := s.engine.Context(ctx).SQL("PRAGMA page_count").Get(&pageCount); err == nil {
		if _, err := s.engine.Context(ctx).SQL("PRAGMA page_size").Get(&pageSize); err == nil {
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
	if _, err := s.engine.Context(ctx).Exec("VACUUM"); err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}

	// PRAGMA optimize to update statistics
	if _, err := s.engine.Context(ctx).Exec("PRAGMA optimize"); err != nil {
		return fmt.Errorf("failed to optimize database: %w", err)
	}

	// Get database size after vacuum
	var afterSize int64
	var pageCountAfter, pageSizeAfter int64

	// Try to get page count and page size after vacuum
	if _, err := s.engine.Context(ctx).SQL("PRAGMA page_count").Get(&pageCountAfter); err == nil {
		if _, err := s.engine.Context(ctx).SQL("PRAGMA page_size").Get(&pageSizeAfter); err == nil {
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
