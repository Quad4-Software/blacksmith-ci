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

package model

import (
	"time"

	"github.com/uptrace/bun"
)

// MaintenanceConfig represents the configuration for database maintenance operations.
type MaintenanceConfig struct {
	bun.BaseModel `bun:"table:maintenance_config"`

	ID         int64  `json:"id"       bun:"id,pk,autoincrement"`
	ActionType string `json:"action_type" bun:"action_type,unique"`
	Enabled    bool   `json:"enabled"  bun:"enabled"`
	Schedule   string `json:"schedule" bun:"schedule"`
	DryRun     bool   `json:"dry_run" bun:"dry_run,default:false"`
	// Action-specific statistics
	TotalRuns       int64     `json:"total_runs" bun:"total_runs,default:0"`
	SuccessfulRuns  int64     `json:"successful_runs" bun:"successful_runs,default:0"`
	LastRun         time.Time `json:"last_run" bun:"last_run"`
	LastRunSuccess  bool      `json:"last_run_success" bun:"last_run_success,default:false"`
	LastRunDuration int64     `json:"last_run_duration" bun:"last_run_duration,default:0"`
	LastRunError    string    `json:"last_run_error" bun:"last_run_error"`
	Created         int64     `json:"created"  bun:",nullzero"`
	Updated         int64     `json:"updated"  bun:",nullzero"`
	StaleThreshold  int64     `json:"stale_threshold" bun:"stale_threshold,default:3600"`
	// Runtime flags (not persisted)
	KubernetesCleanupAvailable bool `json:"kubernetes_cleanup_available" bun:"-"`
	DockerCleanupAvailable     bool `json:"docker_cleanup_available" bun:"-"`
	// Environment variable override flags (not persisted)
	EnvVarsSet bool `json:"env_vars_set" bun:"-"`
} //	@name	MaintenanceConfig

// MaintenanceOperationConfig represents the configuration for individual maintenance operations.
type MaintenanceOperationConfig struct {
	bun.BaseModel `bun:"table:maintenance_operation_config"`

	ID        int64     `json:"id"         bun:"id,pk,autoincrement"`
	Operation string    `json:"operation"  bun:"operation,unique"`
	Enabled   bool      `json:"enabled"    bun:"enabled"`
	Schedule  string    `json:"schedule"   bun:"schedule"`
	TotalRuns int64     `json:"total_runs" bun:"total_runs"`
	LastRun   time.Time `json:"last_run"   bun:"last_run"`
	Created   int64     `json:"created"    bun:",nullzero"`
	Updated   int64     `json:"updated"    bun:",nullzero"`
} //	@name	MaintenanceOperationConfig

func (MaintenanceConfig) TableName() string {
	return "maintenance_config"
}

func (MaintenanceOperationConfig) TableName() string {
	return "maintenance_operation_config"
}

// MaintenanceStats represents statistics about maintenance operations.
type MaintenanceStats struct {
	bun.BaseModel `bun:"table:maintenance_stats"`

	ID              int64     `json:"id"              bun:"id,pk,autoincrement"`
	LastRun         time.Time `json:"last_run"        bun:"last_run"`
	LastRunSuccess  bool      `json:"last_run_success" bun:"last_run_success"`
	LastRunError    string    `json:"last_run_error"  bun:"last_run_error"`
	LastRunDuration int64     `json:"last_run_duration" bun:"last_run_duration"`
	TotalRuns       int64     `json:"total_runs"      bun:"total_runs"`
	SuccessfulRuns  int64     `json:"successful_runs" bun:"successful_runs"`
	Created         int64     `json:"created"         bun:",nullzero"`
	Updated         int64     `json:"updated"         bun:",nullzero"`
} //	@name	MaintenanceStats

func (MaintenanceStats) TableName() string {
	return "maintenance_stats"
}

// MaintenanceLog represents a log entry for maintenance operations.
type MaintenanceLog struct {
	bun.BaseModel `bun:"table:maintenance_logs"`

	ID       int64     `json:"id"       bun:"id,pk,autoincrement"`
	RunTime  time.Time `json:"run_time" bun:"run_time"`
	Success  bool      `json:"success"  bun:"success"`
	Error    string    `json:"error"    bun:"error"`
	Duration int64     `json:"duration" bun:"duration"`
	Details  string    `json:"details"  bun:"details"`
	Created  int64     `json:"created"  bun:",nullzero"`
} //	@name	MaintenanceLog

func (MaintenanceLog) TableName() string {
	return "maintenance_logs"
}
