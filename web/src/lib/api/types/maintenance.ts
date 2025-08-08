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

// MaintenanceConfig represents the configuration for database maintenance operations
export interface MaintenanceConfig {
  id: number;
  action_type: string; // "vacuum" or "kubernetes_cleanup"
  enabled: boolean;
  schedule: string; // Cron format
  // Action-specific statistics
  total_runs: number;
  successful_runs: number;
  last_run: string; // ISO date string
  last_run_success: boolean;
  last_run_duration: number; // Duration in milliseconds
  last_run_error: string;
  created: number;
  updated: number;
  // Runtime flags (not persisted)
  kubernetes_cleanup_available: boolean;
  env_vars_set: boolean; // True if maintenance env vars are set
}

// MaintenanceOperationConfig represents the configuration for individual maintenance operations
export interface MaintenanceOperationConfig {
  id: number;
  operation: string;
  enabled: boolean;
  schedule: string; // Cron format
  total_runs: number;
  last_run: string; // ISO date string
  created: number;
  updated: number;
}

// MaintenanceStats represents maintenance statistics
export interface MaintenanceStats {
  id: number;
  last_run: string; // ISO date string
  last_run_success: boolean;
  last_run_error: string;
  last_run_duration: number; // Duration in milliseconds
  total_runs: number;
  successful_runs: number;
  created: number;
  updated: number;
}

// MaintenanceLog represents a log entry for maintenance operations
export interface MaintenanceLog {
  id: number;
  run_time: string; // ISO date string
  success: boolean;
  error: string;
  duration: number; // Duration in milliseconds
  details: string; // JSON formatted details
  created: number;
}
