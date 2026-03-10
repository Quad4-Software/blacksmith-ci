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

package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/services/maintenance"
)

type GetMaintenanceConfigInput struct{}

type GetMaintenanceConfigOutput struct {
	Body *model.MaintenanceConfig
}

func GetMaintenanceConfig(ctx context.Context, _ *GetMaintenanceConfigInput) (*GetMaintenanceConfigOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)
	if !user.Admin {
		return nil, huma.Error403Forbidden("Forbidden")
	}

	config := server.Config.Services.MaintenanceService.GetMaintenanceConfig()
	return &GetMaintenanceConfigOutput{Body: config}, nil
}

type UpdateMaintenanceConfigBody struct {
	Enabled        bool   `json:"enabled" doc:"whether maintenance is enabled"`
	Schedule       string `json:"schedule,omitempty" doc:"cron schedule"`
	DryRun         bool   `json:"dry_run,omitempty" doc:"dry run mode"`
	StaleThreshold int64  `json:"stale_threshold,omitempty" doc:"stale threshold in seconds"`
}

type UpdateMaintenanceConfigInput struct {
	Body UpdateMaintenanceConfigBody
}

type UpdateMaintenanceConfigOutput struct {
	Body *model.MaintenanceConfig
}

func UpdateMaintenanceConfig(ctx context.Context, input *UpdateMaintenanceConfigInput) (*UpdateMaintenanceConfigOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)
	if !user.Admin {
		return nil, huma.Error403Forbidden("Forbidden")
	}

	config := &model.MaintenanceConfig{
		Enabled:        input.Body.Enabled,
		Schedule:       input.Body.Schedule,
		DryRun:         input.Body.DryRun,
		StaleThreshold: input.Body.StaleThreshold,
	}
	if err := server.Config.Services.MaintenanceService.UpdateMaintenanceConfig(config); err != nil {
		log.Error().Err(err).Msg("Failed to update maintenance config")
		return nil, huma.Error500InternalServerError("Failed to update maintenance config")
	}

	// Update scheduler with new configuration
	if server.Config.Services.MaintenanceScheduler != nil {
		if err := server.Config.Services.MaintenanceScheduler.UpdateSchedule(config.Schedule, config.Enabled); err != nil {
			log.Error().Err(err).Msg("Failed to update maintenance schedule")
			return nil, huma.Error500InternalServerError("Failed to update maintenance schedule")
		}
	}

	// Get fresh config with runtime fields populated
	returnConfig := server.Config.Services.MaintenanceService.GetMaintenanceConfig()

	return &UpdateMaintenanceConfigOutput{Body: returnConfig}, nil
}

type GetMaintenanceConfigByActionInput struct {
	Action string `path:"action" doc:"Action type (vacuum, kubernetes_cleanup, stale_agent_cleanup, docker_cleanup)"`
}

type GetMaintenanceConfigByActionOutput struct {
	Body *model.MaintenanceConfig
}

func GetMaintenanceConfigByAction(ctx context.Context, input *GetMaintenanceConfigByActionInput) (*GetMaintenanceConfigByActionOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)
	if !user.Admin {
		return nil, huma.Error403Forbidden("Forbidden")
	}

	if input.Action != "vacuum" && input.Action != "kubernetes_cleanup" && input.Action != "stale_agent_cleanup" && input.Action != "docker_cleanup" {
		return nil, huma.Error400BadRequest("Invalid action type. Must be 'vacuum', 'kubernetes_cleanup', 'stale_agent_cleanup', or 'docker_cleanup'")
	}

	config, err := server.Config.Services.MaintenanceService.GetMaintenanceConfigByAction(input.Action)
	if err != nil {
		log.Error().Err(err).Str("action", input.Action).Msg("Failed to get maintenance config")
		return nil, huma.Error500InternalServerError("Failed to get maintenance config")
	}

	// Set runtime flags based on action type
	switch input.Action {
	case "kubernetes_cleanup":
		config.KubernetesCleanupAvailable = server.Config.Services.MaintenanceService.IsKubernetesCleanupAvailable()
		config.EnvVarsSet = server.Config.Services.MaintenanceService.IsKubernetesEnvVarsConfigured()
	case "vacuum":
		config.EnvVarsSet = server.Config.Services.MaintenanceService.IsVacuumEnvVarsConfigured()
	case "docker_cleanup":
		config.DockerCleanupAvailable = server.Config.Services.MaintenanceService.IsDockerCleanupAvailable()
	}

	return &GetMaintenanceConfigByActionOutput{Body: config}, nil
}

type UpdateMaintenanceConfigByActionInput struct {
	Action string `path:"action" doc:"Action type (vacuum, kubernetes_cleanup, docker_cleanup)"`
	Body   UpdateMaintenanceConfigBody
}

type UpdateMaintenanceConfigByActionOutput struct {
	Body *model.MaintenanceConfig
}

func UpdateMaintenanceConfigByAction(ctx context.Context, input *UpdateMaintenanceConfigByActionInput) (*UpdateMaintenanceConfigByActionOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)
	if !user.Admin {
		return nil, huma.Error403Forbidden("Forbidden")
	}

	if input.Action != "vacuum" && input.Action != "kubernetes_cleanup" && input.Action != "docker_cleanup" {
		return nil, huma.Error400BadRequest("Invalid action type. Must be 'vacuum', 'kubernetes_cleanup', or 'docker_cleanup'")
	}

	actionConfig := &model.MaintenanceConfig{
		ActionType:     input.Action,
		Enabled:        input.Body.Enabled,
		Schedule:       input.Body.Schedule,
		DryRun:         input.Body.DryRun,
		StaleThreshold: input.Body.StaleThreshold,
	}

	log.Info().
		Str("action", input.Action).
		Bool("enabled", actionConfig.Enabled).
		Str("schedule", actionConfig.Schedule).
		Bool("dry_run", actionConfig.DryRun).
		Msg("Updating maintenance config by action")

	if err := server.Config.Services.MaintenanceService.UpdateMaintenanceConfigByAction(input.Action, actionConfig); err != nil {
		log.Error().Err(err).Str("action", input.Action).Msg("Failed to update maintenance config")
		return nil, huma.Error500InternalServerError("Failed to update maintenance config")
	}

	// Note: No need to update scheduler - it reloads configs every minute automatically

	updatedConfig, err := server.Config.Services.MaintenanceService.GetMaintenanceConfigByAction(input.Action)
	if err != nil {
		log.Error().Err(err).Str("action", input.Action).Msg("Failed to get updated maintenance config")
		return nil, huma.Error500InternalServerError("Failed to get updated maintenance config")
	}

	switch input.Action {
	case "kubernetes_cleanup":
		updatedConfig.KubernetesCleanupAvailable = server.Config.Services.MaintenanceService.IsKubernetesCleanupAvailable()
	case "docker_cleanup":
		updatedConfig.DockerCleanupAvailable = server.Config.Services.MaintenanceService.IsDockerCleanupAvailable()
	}

	return &UpdateMaintenanceConfigByActionOutput{Body: updatedConfig}, nil
}

type GetMaintenanceStatsInput struct{}

type GetMaintenanceStatsOutput struct {
	Body *model.MaintenanceStats
}

func GetMaintenanceStats(ctx context.Context, _ *GetMaintenanceStatsInput) (*GetMaintenanceStatsOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)
	if !user.Admin {
		return nil, huma.Error403Forbidden("Forbidden")
	}

	stats, err := server.Config.Services.MaintenanceService.GetMaintenanceStats()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get maintenance stats")
		return nil, huma.Error500InternalServerError("Failed to get maintenance stats")
	}

	return &GetMaintenanceStatsOutput{Body: stats}, nil
}

type GetMaintenanceLogsInput struct {
	Limit int `query:"limit" default:"50" doc:"Number of logs to return"`
}

type GetMaintenanceLogsOutput struct {
	Body []*model.MaintenanceLog
}

func GetMaintenanceLogs(ctx context.Context, input *GetMaintenanceLogsInput) (*GetMaintenanceLogsOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)
	if !user.Admin {
		return nil, huma.Error403Forbidden("Forbidden")
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}

	logs, err := server.Config.Services.Store.MaintenanceLogList(limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get maintenance logs")
		return nil, huma.Error500InternalServerError("Failed to get maintenance logs")
	}

	return &GetMaintenanceLogsOutput{Body: logs}, nil
}

// RunMaintenanceRequest represents the request payload for running maintenance.
type RunMaintenanceRequest struct {
	Job    string `json:"job" doc:"The maintenance job to run"`
	DryRun bool   `json:"dry_run,omitempty" doc:"Run in dry-run mode (only for kubernetesCleanup)"`
}

type RunMaintenanceNowInput struct {
	Body RunMaintenanceRequest
}

type RunMaintenanceResponse struct {
	Message string `json:"message"`
	Job     string `json:"job"`
}

type RunMaintenanceNowOutput struct {
	Body RunMaintenanceResponse
}

func RunMaintenanceNow(ctx context.Context, input *RunMaintenanceNowInput) (*RunMaintenanceNowOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)
	if !user.Admin {
		return nil, huma.Error403Forbidden("Forbidden")
	}

	// Validate job type and execute appropriate maintenance job
	switch input.Body.Job {
	case "vacuumLogTable":
		if server.Config.Services.MaintenanceScheduler == nil {
			return nil, huma.Error500InternalServerError("Maintenance scheduler not available")
		}

		go func() {
			if err := server.Config.Services.MaintenanceScheduler.RunManual(); err != nil {
				log.Error().Err(err).Msg("Manual maintenance run failed")
			}
		}()

	case "kubernetesCleanup":
		if server.Config.Services.MaintenanceService == nil {
			return nil, huma.Error500InternalServerError("Maintenance service not available")
		}

		if !server.Config.Services.MaintenanceService.IsKubernetesCleanupAvailable() {
			return nil, huma.Error400BadRequest("Kubernetes cleanup is not configured. Set CROW_MAINTENANCE_KUBERNETES_CLEANUP_AGE to configure.")
		}

		k8sConfig, err := server.Config.Services.MaintenanceService.GetMaintenanceConfigByAction("kubernetes_cleanup")
		if err != nil || !k8sConfig.Enabled {
			return nil, huma.Error400BadRequest("Kubernetes cleanup is not enabled. Enable it in the maintenance settings.")
		}

		go func() {
			if err := server.Config.Services.MaintenanceService.RunKubernetesCleanup(context.Background(), input.Body.DryRun); err != nil {
				log.Error().Err(err).Bool("dry_run", input.Body.DryRun).Msg("Kubernetes cleanup failed")
			}
		}()

	case "staleAgentCleanup":
		if server.Config.Services.MaintenanceService == nil {
			return nil, huma.Error500InternalServerError("Maintenance service not available")
		}

		staleAgentConfig, err := server.Config.Services.MaintenanceService.GetMaintenanceConfigByAction("stale_agent_cleanup")
		if err != nil || !staleAgentConfig.Enabled {
			return nil, huma.Error400BadRequest("Stale agent cleanup is not enabled. Enable it in the maintenance settings.")
		}

		go func() {
			staleDuration := maintenance.GetStaleAgentDuration()
			if err := server.Config.Services.MaintenanceService.RunStaleAgentCleanup(context.Background(), staleDuration); err != nil {
				log.Error().Err(err).Dur("stale_duration", staleDuration).Msg("Stale agent cleanup failed")
			}
		}()

	case "dockerCleanup":
		if server.Config.Services.MaintenanceService == nil {
			return nil, huma.Error500InternalServerError("Maintenance service not available")
		}

		if !server.Config.Services.MaintenanceService.IsDockerCleanupAvailable() {
			return nil, huma.Error400BadRequest("Docker cleanup is not available. No Docker agent is connected.")
		}

		dockerConfig, err := server.Config.Services.MaintenanceService.GetMaintenanceConfigByAction("docker_cleanup")
		if err != nil || !dockerConfig.Enabled {
			return nil, huma.Error400BadRequest("Docker cleanup is not enabled. Enable it in the maintenance settings.")
		}

		server.Config.Services.MaintenanceService.SetDockerCleanupRunNow()

	default:
		return nil, huma.Error400BadRequest("Invalid job type. Supported jobs: vacuumLogTable, kubernetesCleanup, staleAgentCleanup, dockerCleanup")
	}

	return &RunMaintenanceNowOutput{Body: RunMaintenanceResponse{
		Message: "Maintenance started",
		Job:     input.Body.Job,
	}}, nil
}
