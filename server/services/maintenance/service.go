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

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store"
	"codefloe.com/crowci/crow/v5/server/store/datastore"
)

// Service defines the interface for database maintenance operations.
type Service interface {
	// RunMaintenance executes database maintenance operations
	RunMaintenance(ctx context.Context) error
	// RunKubernetesCleanup executes Kubernetes resource cleanup.
	RunKubernetesCleanup(ctx context.Context, dryRun bool) error
	// RunStaleAgentCleanup executes stale agent cleanup.
	RunStaleAgentCleanup(ctx context.Context, staleDuration time.Duration) error
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
}

type service struct {
	store                      store.Store
	kubernetesCleanupFunc      func(ctx context.Context, dryRun bool) error
	kubernetesCleanupAvailable bool
	kubernetesCleanupEnabled   bool
	vacuumEnvVarsConfigured    bool
	k8sEnvVarsConfigured       bool
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

// SetKubernetesCleanupFunc sets the function to use for Kubernetes cleanup.
func (s *service) SetKubernetesCleanupFunc(f func(ctx context.Context, dryRun bool) error) {
	s.kubernetesCleanupFunc = f
	s.kubernetesCleanupAvailable = true
}

// SetKubernetesCleanupEnabled sets whether Kubernetes cleanup is enabled for execution.
func (s *service) SetKubernetesCleanupEnabled(enabled bool) {
	s.kubernetesCleanupEnabled = enabled
}

// SetVacuumEnvVarsConfigured sets whether vacuum-specific environment variables are overriding configuration.
func (s *service) SetVacuumEnvVarsConfigured(vacuumEnvVarsSet bool) {
	s.vacuumEnvVarsConfigured = vacuumEnvVarsSet
}

// IsVacuumEnvVarsConfigured returns whether vacuum-specific environment variables are set.
func (s *service) IsVacuumEnvVarsConfigured() bool {
	return s.vacuumEnvVarsConfigured
}

// SetKubernetesEnvVarsConfigured sets whether Kubernetes-specific environment variables are set.
func (s *service) SetKubernetesEnvVarsConfigured(k8sEnvVarsSet bool) {
	s.k8sEnvVarsConfigured = k8sEnvVarsSet
}

// IsKubernetesEnvVarsConfigured returns whether Kubernetes-specific environment variables are set.
func (s *service) IsKubernetesEnvVarsConfigured() bool {
	return s.k8sEnvVarsConfigured
}

// RunMaintenance executes database maintenance operations based on the database type.
func (s *service) RunMaintenance(ctx context.Context) error {
	// Update LastRun immediately to prevent duplicate runs
	s.updateActionLastRun("vacuum")

	start := time.Now()
	log.Info().Msg("Starting database maintenance operations")

	if err := s.store.MaintenanceRun(ctx); err != nil {
		duration := time.Since(start)
		log.Error().Err(err).Dur("duration", duration).Msg("Database maintenance failed")
		s.updateActionStats("vacuum", false, duration, err.Error())

		logEntry := &model.MaintenanceLog{
			RunTime:  start,
			Success:  false,
			Duration: duration.Milliseconds(),
			Details:  fmt.Sprintf("Operations: VACUUM/OPTIMIZE log_entries table, Duration: %v", duration),
			Error:    err.Error(),
			Created:  start.Unix(),
		}
		if logErr := s.store.MaintenanceLogCreate(logEntry); logErr != nil {
			log.Error().Err(logErr).Msg("Failed to create maintenance log entry")
		}

		return fmt.Errorf("database maintenance failed: %w", err)
	}

	duration := time.Since(start)

	// Get size information from the store layer
	beforeSize, afterSize, spaceSaved := datastore.GetMaintenanceSizeInfo()
	details := fmt.Sprintf("Operations: VACUUM/OPTIMIZE log_entries table, Duration: %v", duration)

	if beforeSize > 0 || afterSize > 0 {
		const bytesPerMB = 1024 * 1024
		beforeSizeMB := float64(beforeSize) / bytesPerMB
		afterSizeMB := float64(afterSize) / bytesPerMB
		spaceSavedMB := float64(spaceSaved) / bytesPerMB
		details = fmt.Sprintf("Operations: VACUUM/OPTIMIZE log_entries table, Duration: %v, Before: %.2f MB, After: %.2f MB, Saved: %.2f MB",
			duration, beforeSizeMB, afterSizeMB, spaceSavedMB)
	}

	s.updateActionStats("vacuum", true, duration, "")

	logEntry := &model.MaintenanceLog{
		RunTime:  start,
		Success:  true,
		Duration: duration.Milliseconds(),
		Details:  details,
		Created:  start.Unix(),
	}
	if err := s.store.MaintenanceLogCreate(logEntry); err != nil {
		log.Error().Err(err).Msg("Failed to create maintenance log entry")
	}

	log.Info().Dur("duration", duration).Msg("Database maintenance completed successfully")
	return nil
}

// updateActionLastRun updates just the LastRun timestamp for a maintenance action.
// This is called immediately when starting to prevent duplicate runs.
func (s *service) updateActionLastRun(actionType string) {
	config, err := s.store.MaintenanceConfigGetByActionType(actionType)
	if err != nil {
		log.Error().Err(err).Str("action", actionType).Msg("Failed to get maintenance config for last run update")
		return
	}

	config.LastRun = time.Now()
	config.Updated = time.Now().Unix()

	if err := s.store.MaintenanceConfigUpdateByActionType(actionType, config); err != nil {
		log.Error().Err(err).Str("action", actionType).Msg("Failed to update maintenance action last run")
	}
}

// updateActionStats updates statistics for a specific maintenance action.
func (s *service) updateActionStats(actionType string, success bool, duration time.Duration, errorMsg string) {
	config, err := s.store.MaintenanceConfigGetByActionType(actionType)
	if err != nil {
		log.Error().Err(err).Str("action", actionType).Msg("Failed to get maintenance config for stats update")
		return
	}

	// Update statistics in the config
	config.LastRun = time.Now()
	config.LastRunSuccess = success
	config.LastRunDuration = duration.Milliseconds()
	config.TotalRuns++
	config.Updated = time.Now().Unix()

	if success {
		config.SuccessfulRuns++
		config.LastRunError = ""
	} else {
		config.LastRunError = errorMsg
	}

	if err := s.store.MaintenanceConfigUpdateByActionType(actionType, config); err != nil {
		log.Error().Err(err).Str("action", actionType).Msg("Failed to update maintenance action statistics")
	}
}

// GetMaintenanceConfig returns the current maintenance configuration.
func (s *service) GetMaintenanceConfig() *model.MaintenanceConfig {
	config, err := s.store.MaintenanceConfigGet()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get maintenance config, using defaults")
		config = &model.MaintenanceConfig{
			Enabled:  true,
			Schedule: "0 5 * * *",
		}
	}
	// Set the runtime Kubernetes cleanup availability flag (configuration presence)
	config.KubernetesCleanupAvailable = s.kubernetesCleanupAvailable
	// Set the env vars flag to inform the frontend (this is for vacuum since GetMaintenanceConfig is vacuum-focused)
	config.EnvVarsSet = s.vacuumEnvVarsConfigured
	return config
}

// UpdateMaintenanceConfig updates the maintenance configuration.
func (s *service) UpdateMaintenanceConfig(config *model.MaintenanceConfig) error {
	return s.store.MaintenanceConfigUpdate(config)
}

// GetMaintenanceConfigByAction returns config for a specific action type.
func (s *service) GetMaintenanceConfigByAction(actionType string) (*model.MaintenanceConfig, error) {
	log.Debug().Str("action", actionType).Msg("SERVICE: Getting maintenance config by action")
	config, err := s.store.MaintenanceConfigGetByActionType(actionType)
	if err != nil {
		log.Error().Err(err).Str("action", actionType).Msg("SERVICE ERROR: Failed to get config from store")
		return nil, err
	}
	if config == nil {
		log.Error().Str("action", actionType).Msg("SERVICE ERROR: Store returned nil config")
		return nil, nil
	}
	log.Debug().
		Str("action", actionType).
		Int64("config_id", config.ID).
		Bool("enabled", config.Enabled).
		Str("schedule", config.Schedule).
		Msg("SERVICE: Successfully retrieved config")
	return config, nil
}

// UpdateMaintenanceConfigByAction updates config for a specific action type.
func (s *service) UpdateMaintenanceConfigByAction(actionType string, config *model.MaintenanceConfig) error {
	return s.store.MaintenanceConfigUpdateByActionType(actionType, config)
}

// IsKubernetesCleanupAvailable returns whether Kubernetes cleanup is available.
func (s *service) IsKubernetesCleanupAvailable() bool {
	return s.kubernetesCleanupAvailable
}

// GetMaintenanceStats returns maintenance statistics.
func (s *service) GetMaintenanceStats() (*model.MaintenanceStats, error) {
	return s.store.MaintenanceStatsGet()
}

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const cleanupStatsKey contextKey = "cleanup_stats"

// RunKubernetesCleanup executes Kubernetes resource cleanup.
func (s *service) RunKubernetesCleanup(ctx context.Context, dryRun bool) error {
	if s.kubernetesCleanupFunc == nil {
		log.Info().Msg("Kubernetes cleanup function not configured, skipping")
		return nil
	}

	// Update LastRun immediately to prevent duplicate runs (unless in dry-run mode)
	if !dryRun {
		s.updateActionLastRun("kubernetes_cleanup")
	}

	start := time.Now()
	log.Info().Bool("dry_run", dryRun).Msg("Starting Kubernetes resource cleanup")

	// Store the original cleanup stats to capture what was cleaned up
	cleanupStats := &KubernetesCleanupStats{}
	ctx = context.WithValue(ctx, cleanupStatsKey, cleanupStats)

	err := s.kubernetesCleanupFunc(ctx, dryRun)
	duration := time.Since(start)

	details := fmt.Sprintf("Operation: Kubernetes resource cleanup, Duration: %v, DryRun: %v", duration, dryRun)

	totalFound := cleanupStats.ServicesFound + cleanupStats.PVCsFound + cleanupStats.SecretsFound
	totalDeleted := cleanupStats.ServicesDeleted + cleanupStats.PVCsDeleted + cleanupStats.SecretsDeleted

	if totalFound > 0 {
		resourceDetails := fmt.Sprintf(", Resources found: %d (Services: %d, PVCs: %d, Secrets: %d)",
			totalFound, cleanupStats.ServicesFound, cleanupStats.PVCsFound, cleanupStats.SecretsFound)

		if dryRun {
			resourceDetails += fmt.Sprintf(", Would delete: %d", totalFound)
		} else {
			resourceDetails += fmt.Sprintf(", Deleted: %d", totalDeleted)
			if totalDeleted < totalFound {
				resourceDetails += fmt.Sprintf(" (Failed: %d)", totalFound-totalDeleted)
			}
		}
		details += resourceDetails
	} else {
		details += ", No stale resources found"
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
		log.Error().Err(err).Dur("duration", duration).Msg("Kubernetes resource cleanup failed")

		s.updateActionStats("kubernetes_cleanup", false, duration, err.Error())

		if logErr := s.store.MaintenanceLogCreate(logEntry); logErr != nil {
			log.Error().Err(logErr).Msg("Failed to create maintenance log entry for Kubernetes cleanup")
		}

		return fmt.Errorf("kubernetes resource cleanup failed: %w", err)
	}

	s.updateActionStats("kubernetes_cleanup", true, duration, "")

	if err := s.store.MaintenanceLogCreate(logEntry); err != nil {
		log.Error().Err(err).Msg("Failed to create maintenance log entry for Kubernetes cleanup")
	}

	log.Info().Dur("duration", duration).Msg("Kubernetes resource cleanup completed successfully")
	return nil
}

// RunStaleAgentCleanup executes stale agent cleanup.
func (s *service) RunStaleAgentCleanup(ctx context.Context, staleDuration time.Duration) error {
	// Update LastRun immediately to prevent duplicate runs
	s.updateActionLastRun("stale_agent_cleanup")

	start := time.Now()
	log.Info().Dur("stale_duration", staleDuration).Msg("Starting stale agent cleanup")

	// Calculate the cutoff time for stale agents
	cutoffTime := time.Now().Add(-staleDuration).Unix()

	// Get all agents
	agents, err := s.store.AgentList(&model.AgentListOptions{ListOptions: model.ListOptions{All: true}})
	if err != nil {
		log.Error().Err(err).Msg("Failed to list agents")
		s.updateActionStats("stale_agent_cleanup", false, time.Since(start), err.Error())
		return fmt.Errorf("failed to list agents: %w", err)
	}

	staleCount := 0
	deletedCount := 0
	var errors []string

	log.Debug().
		Int64("cutoff_time", cutoffTime).
		Time("cutoff_time_formatted", time.Unix(cutoffTime, 0)).
		Dur("stale_duration", staleDuration).
		Int("total_agents", len(agents)).
		Msg("Stale agent cleanup - checking agents")

	for _, agent := range agents {
		isStale := false
		var reason string

		// Check if agent is stale
		if agent.LastContact > 0 && agent.LastContact < cutoffTime {
			// Agent has connected before but hasn't connected recently
			isStale = true
			reason = "no contact since " + time.Unix(agent.LastContact, 0).Format(time.RFC3339)
		} else if agent.LastContact == 0 && agent.Created < cutoffTime {
			// Agent was created but has never connected, and creation time is older than threshold
			isStale = true
			reason = "never connected, created " + time.Unix(agent.Created, 0).Format(time.RFC3339)
		}

		// Debug log for every agent to understand what's happening
		if agent.LastContact == 0 {
			log.Debug().
				Int64("agent_id", agent.ID).
				Str("agent_name", agent.Name).
				Int64("created", agent.Created).
				Int64("cutoff_time", cutoffTime).
				Bool("is_stale", isStale).
				Str("reason", reason).
				Msg("Agent never connected - checking if stale")
		}

		if isStale {
			staleCount++
			log.Info().
				Int64("agent_id", agent.ID).
				Str("agent_name", agent.Name).
				Int64("last_contact", agent.LastContact).
				Int64("created", agent.Created).
				Str("reason", reason).
				Msg("Deleting stale agent")

			if err := s.store.AgentDelete(agent); err != nil {
				errMsg := fmt.Sprintf("failed to delete agent %d (%s): %v", agent.ID, agent.Name, err)
				errors = append(errors, errMsg)
				log.Error().Err(err).Int64("agent_id", agent.ID).Msg("Failed to delete stale agent")
			} else {
				deletedCount++
			}
		}
	}

	duration := time.Since(start)
	details := fmt.Sprintf("Operation: Stale agent cleanup, Duration: %v, Stale threshold: %v, Total agents: %d, Stale found: %d, Deleted: %d",
		duration, staleDuration, len(agents), staleCount, deletedCount)

	if len(errors) > 0 {
		details += fmt.Sprintf(", Errors: %d", len(errors))
	}

	logEntry := &model.MaintenanceLog{
		RunTime:  start,
		Success:  len(errors) == 0,
		Duration: duration.Milliseconds(),
		Details:  details,
		Created:  start.Unix(),
	}

	if len(errors) > 0 {
		logEntry.Error = fmt.Sprintf("%d errors occurred: %v", len(errors), errors[0])
		log.Error().Strs("errors", errors).Msg("Stale agent cleanup completed with errors")
		s.updateActionStats("stale_agent_cleanup", false, duration, logEntry.Error)

		if logErr := s.store.MaintenanceLogCreate(logEntry); logErr != nil {
			log.Error().Err(logErr).Msg("Failed to create maintenance log entry for stale agent cleanup")
		}

		return fmt.Errorf("stale agent cleanup completed with %d errors", len(errors))
	}

	s.updateActionStats("stale_agent_cleanup", true, duration, "")

	if err := s.store.MaintenanceLogCreate(logEntry); err != nil {
		log.Error().Err(err).Msg("Failed to create maintenance log entry for stale agent cleanup")
	}

	log.Info().
		Int("stale_count", staleCount).
		Int("deleted_count", deletedCount).
		Dur("duration", duration).
		Msg("Stale agent cleanup completed successfully")
	return nil
}
