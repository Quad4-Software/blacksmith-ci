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

import "time"

// MaintenanceConfig represents the configuration for database maintenance operations.
type MaintenanceConfig struct {
	ID       int64  `json:"id"       xorm:"pk autoincr 'id'"`
	Enabled  bool   `json:"enabled"  xorm:"'enabled'"`
	Schedule string `json:"schedule" xorm:"'schedule'"` // Cron format: "0 5 * * *" for 5 AM daily
	Created  int64  `json:"created"  xorm:"created"`
	Updated  int64  `json:"updated"  xorm:"updated"`
} //	@name	MaintenanceConfig

// MaintenanceOperationConfig represents the configuration for individual maintenance operations.
type MaintenanceOperationConfig struct {
	ID        int64     `json:"id"         xorm:"pk autoincr 'id'"`
	Operation string    `json:"operation"  xorm:"'operation' unique"`
	Enabled   bool      `json:"enabled"    xorm:"'enabled'"`
	Schedule  string    `json:"schedule"   xorm:"'schedule'"` // Cron format: "0 5 * * *" for 5 AM daily
	TotalRuns int64     `json:"total_runs" xorm:"'total_runs'"`
	LastRun   time.Time `json:"last_run"   xorm:"'last_run'"`
	Created   int64     `json:"created"    xorm:"created"`
	Updated   int64     `json:"updated"    xorm:"updated"`
} //	@name	MaintenanceOperationConfig

func (MaintenanceConfig) TableName() string {
	return "maintenance_config"
}

func (MaintenanceOperationConfig) TableName() string {
	return "maintenance_operation_config"
}

// MaintenanceStats represents statistics about maintenance operations.
type MaintenanceStats struct {
	ID              int64     `json:"id"              xorm:"pk autoincr 'id'"`
	LastRun         time.Time `json:"last_run"        xorm:"'last_run'"`
	LastRunSuccess  bool      `json:"last_run_success" xorm:"'last_run_success'"`
	LastRunError    string    `json:"last_run_error"  xorm:"'last_run_error'"`
	LastRunDuration int64     `json:"last_run_duration" xorm:"'last_run_duration'"` // Duration in milliseconds
	TotalRuns       int64     `json:"total_runs"      xorm:"'total_runs'"`
	SuccessfulRuns  int64     `json:"successful_runs" xorm:"'successful_runs'"`
	Created         int64     `json:"created"         xorm:"created"`
	Updated         int64     `json:"updated"         xorm:"updated"`
} //	@name	MaintenanceStats

func (MaintenanceStats) TableName() string {
	return "maintenance_stats"
}

// MaintenanceLog represents a log entry for maintenance operations.
type MaintenanceLog struct {
	ID       int64     `json:"id"       xorm:"pk autoincr 'id'"`
	RunTime  time.Time `json:"run_time" xorm:"'run_time'"`
	Success  bool      `json:"success"  xorm:"'success'"`
	Error    string    `json:"error"    xorm:"'error'"`
	Duration int64     `json:"duration" xorm:"'duration'"` // Duration in milliseconds
	Details  string    `json:"details"  xorm:"'details'"`  // JSON formatted details
	Created  int64     `json:"created"  xorm:"created"`
} //	@name	MaintenanceLog

func (MaintenanceLog) TableName() string {
	return "maintenance_logs"
}
