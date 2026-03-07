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
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/services/maintenance"
)

// GetMaintenanceConfig returns the current maintenance configuration
//
//	@Summary		Get maintenance configuration
//	@Description	Get the current database maintenance configuration
//	@Router			/admin/maintenance/config [get]
//	@Produce		json
//	@Success		200	{object}	MaintenanceConfig
//	@Tags			Admin
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Security		ApiKeyAuth
func GetMaintenanceConfig(c *gin.Context) {
	user := session.User(c)
	if !user.Admin {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	config := server.Config.Services.MaintenanceService.GetMaintenanceConfig()
	c.JSON(http.StatusOK, config)
}

// UpdateMaintenanceConfig updates the maintenance configuration
//
//	@Summary		Update maintenance configuration
//	@Description	Update the database maintenance configuration
//	@Router			/admin/maintenance/config [put]
//	@Produce		json
//	@Accept			json
//	@Success		200	{object}	MaintenanceConfig
//	@Tags			Admin
//	@Param			Authorization	header	string				true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			config			body	MaintenanceConfig	true	"Maintenance configuration"
//	@Security		ApiKeyAuth
func UpdateMaintenanceConfig(c *gin.Context) {
	user := session.User(c)
	if !user.Admin {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	var config model.MaintenanceConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := server.Config.Services.MaintenanceService.UpdateMaintenanceConfig(&config); err != nil {
		log.Error().Err(err).Msg("Failed to update maintenance config")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update maintenance config"})
		return
	}

	// Update scheduler with new configuration
	if server.Config.Services.MaintenanceScheduler != nil {
		if err := server.Config.Services.MaintenanceScheduler.UpdateSchedule(config.Schedule, config.Enabled); err != nil {
			log.Error().Err(err).Msg("Failed to update maintenance schedule")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update maintenance schedule"})
			return
		}
	}

	// Get fresh config with runtime fields populated
	returnConfig := server.Config.Services.MaintenanceService.GetMaintenanceConfig()

	c.JSON(http.StatusOK, returnConfig)
}

// GetMaintenanceConfigByAction returns maintenance configuration for a specific action
//
//	@Summary		Get maintenance configuration by action type
//	@Description	Get maintenance configuration for a specific action (vacuum, kubernetes_cleanup)
//	@Router			/admin/maintenance/config/{action} [get]
//	@Produce		json
//	@Success		200	{object}	MaintenanceConfig
//	@Tags			Admin
//	@Param			Authorization	header	string	true	"Insert your personal access token"			default(Bearer <personal access token>)
//	@Param			action			path	string	true	"Action type (vacuum, kubernetes_cleanup)"	Enums(vacuum, kubernetes_cleanup)
//	@Security		ApiKeyAuth
func GetMaintenanceConfigByAction(c *gin.Context) {
	user := session.User(c)
	if !user.Admin {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	actionType := c.Param("action")
	if actionType != "vacuum" && actionType != "kubernetes_cleanup" && actionType != "stale_agent_cleanup" && actionType != "docker_cleanup" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action type. Must be 'vacuum', 'kubernetes_cleanup', 'stale_agent_cleanup', or 'docker_cleanup'"})
		return
	}

	config, err := server.Config.Services.MaintenanceService.GetMaintenanceConfigByAction(actionType)
	if err != nil {
		log.Error().Err(err).Str("action", actionType).Msg("Failed to get maintenance config")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get maintenance config"})
		return
	}

	// Set runtime flags based on action type
	switch actionType {
	case "kubernetes_cleanup":
		config.KubernetesCleanupAvailable = server.Config.Services.MaintenanceService.IsKubernetesCleanupAvailable()
		config.EnvVarsSet = server.Config.Services.MaintenanceService.IsKubernetesEnvVarsConfigured()
	case "vacuum":
		config.EnvVarsSet = server.Config.Services.MaintenanceService.IsVacuumEnvVarsConfigured()
	case "docker_cleanup":
		config.DockerCleanupAvailable = server.Config.Services.MaintenanceService.IsDockerCleanupAvailable()
	}

	c.JSON(http.StatusOK, config)
}

// UpdateMaintenanceConfigByAction updates maintenance configuration for a specific action
//
//	@Summary		Update maintenance configuration by action type
//	@Description	Update maintenance configuration for a specific action (vacuum, kubernetes_cleanup)
//	@Router			/admin/maintenance/config/{action} [put]
//	@Produce		json
//	@Accept			json
//	@Success		200	{object}	MaintenanceConfig
//	@Tags			Admin
//	@Param			Authorization	header	string				true	"Insert your personal access token"			default(Bearer <personal access token>)
//	@Param			action			path	string				true	"Action type (vacuum, kubernetes_cleanup)"	Enums(vacuum, kubernetes_cleanup)
//	@Param			config			body	MaintenanceConfig	true	"Maintenance configuration"
//	@Security		ApiKeyAuth
func UpdateMaintenanceConfigByAction(c *gin.Context) {
	user := session.User(c)
	if !user.Admin {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	actionType := c.Param("action")
	if actionType != "vacuum" && actionType != "kubernetes_cleanup" && actionType != "docker_cleanup" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action type. Must be 'vacuum', 'kubernetes_cleanup', or 'docker_cleanup'"})
		return
	}

	var config model.MaintenanceConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Ensure the action type matches the URL parameter
	config.ActionType = actionType

	log.Info().
		Str("action", actionType).
		Bool("enabled", config.Enabled).
		Str("schedule", config.Schedule).
		Bool("dry_run", config.DryRun).
		Msg("Updating maintenance config by action")

	if err := server.Config.Services.MaintenanceService.UpdateMaintenanceConfigByAction(actionType, &config); err != nil {
		log.Error().Err(err).Str("action", actionType).Msg("Failed to update maintenance config")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update maintenance config"})
		return
	}

	// Note: No need to update scheduler - it reloads configs every minute automatically

	updatedConfig, err := server.Config.Services.MaintenanceService.GetMaintenanceConfigByAction(actionType)
	if err != nil {
		log.Error().Err(err).Str("action", actionType).Msg("Failed to get updated maintenance config")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get updated maintenance config"})
		return
	}

	switch actionType {
	case "kubernetes_cleanup":
		updatedConfig.KubernetesCleanupAvailable = server.Config.Services.MaintenanceService.IsKubernetesCleanupAvailable()
	case "docker_cleanup":
		updatedConfig.DockerCleanupAvailable = server.Config.Services.MaintenanceService.IsDockerCleanupAvailable()
	}

	c.JSON(http.StatusOK, updatedConfig)
}

// GetMaintenanceStats returns maintenance statistics
//
//	@Summary		Get maintenance statistics
//	@Description	Get database maintenance statistics
//	@Router			/admin/maintenance/stats [get]
//	@Produce		json
//	@Success		200	{object}	MaintenanceStats
//	@Tags			Admin
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Security		ApiKeyAuth
func GetMaintenanceStats(c *gin.Context) {
	user := session.User(c)
	if !user.Admin {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	stats, err := server.Config.Services.MaintenanceService.GetMaintenanceStats()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get maintenance stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get maintenance stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetMaintenanceLogs returns recent maintenance logs
//
//	@Summary		Get maintenance logs
//	@Description	Get recent database maintenance logs
//	@Router			/admin/maintenance/logs [get]
//	@Produce		json
//	@Success		200	{array}	MaintenanceLog
//	@Tags			Admin
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			limit			query	int		false	"Number of logs to return"			default(50)
//	@Security		ApiKeyAuth
func GetMaintenanceLogs(c *gin.Context) {
	user := session.User(c)
	if !user.Admin {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 50
	}

	logs, err := server.Config.Services.Store.MaintenanceLogList(limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get maintenance logs")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get maintenance logs"})
		return
	}

	c.JSON(http.StatusOK, logs)
}

// RunMaintenanceRequest represents the request payload for running maintenance.
type RunMaintenanceRequest struct {
	Job    string `json:"job" binding:"required" example:"vacuumLogTable"` // The maintenance job to run
	DryRun bool   `json:"dry_run,omitempty"`                               // Run in dry-run mode (only for kubernetesCleanup)
}

// RunMaintenanceNow runs database maintenance immediately
//
//	@Summary		Run maintenance now
//	@Description	Trigger database maintenance operation immediately
//	@Router			/admin/maintenance/run [post]
//	@Produce		json
//	@Accept			json
//	@Success		200	{object}	map[string]string
//	@Tags			Admin
//	@Param			Authorization	header	string					true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			request			body	RunMaintenanceRequest	true	"Maintenance job request"
//	@Security		ApiKeyAuth
func RunMaintenanceNow(c *gin.Context) {
	user := session.User(c)
	if !user.Admin {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	var req RunMaintenanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate job type and execute appropriate maintenance job
	switch req.Job {
	case "vacuumLogTable":
		if server.Config.Services.MaintenanceScheduler == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Maintenance scheduler not available"})
			return
		}

		go func() {
			if err := server.Config.Services.MaintenanceScheduler.RunManual(); err != nil {
				log.Error().Err(err).Msg("Manual maintenance run failed")
			}
		}()

	case "kubernetesCleanup":
		if server.Config.Services.MaintenanceService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Maintenance service not available"})
			return
		}

		if !server.Config.Services.MaintenanceService.IsKubernetesCleanupAvailable() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Kubernetes cleanup is not configured. Set CROW_MAINTENANCE_KUBERNETES_CLEANUP_AGE to configure."})
			return
		}

		k8sConfig, err := server.Config.Services.MaintenanceService.GetMaintenanceConfigByAction("kubernetes_cleanup")
		if err != nil || !k8sConfig.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Kubernetes cleanup is not enabled. Enable it in the maintenance settings."})
			return
		}

		go func() {
			if err := server.Config.Services.MaintenanceService.RunKubernetesCleanup(context.Background(), req.DryRun); err != nil {
				log.Error().Err(err).Bool("dry_run", req.DryRun).Msg("Kubernetes cleanup failed")
			}
		}()

	case "staleAgentCleanup":
		if server.Config.Services.MaintenanceService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Maintenance service not available"})
			return
		}

		staleAgentConfig, err := server.Config.Services.MaintenanceService.GetMaintenanceConfigByAction("stale_agent_cleanup")
		if err != nil || !staleAgentConfig.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Stale agent cleanup is not enabled. Enable it in the maintenance settings."})
			return
		}

		go func() {
			staleDuration := maintenance.GetStaleAgentDuration()
			if err := server.Config.Services.MaintenanceService.RunStaleAgentCleanup(context.Background(), staleDuration); err != nil {
				log.Error().Err(err).Dur("stale_duration", staleDuration).Msg("Stale agent cleanup failed")
			}
		}()

	case "dockerCleanup":
		if server.Config.Services.MaintenanceService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Maintenance service not available"})
			return
		}

		if !server.Config.Services.MaintenanceService.IsDockerCleanupAvailable() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Docker cleanup is not available. No Docker agent is connected."})
			return
		}

		dockerConfig, err := server.Config.Services.MaintenanceService.GetMaintenanceConfigByAction("docker_cleanup")
		if err != nil || !dockerConfig.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Docker cleanup is not enabled. Enable it in the maintenance settings."})
			return
		}

		server.Config.Services.MaintenanceService.SetDockerCleanupRunNow()

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job type. Supported jobs: vacuumLogTable, kubernetesCleanup, staleAgentCleanup, dockerCleanup"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Maintenance started", "job": req.Job})
}
