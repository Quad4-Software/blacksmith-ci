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
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v4/server"
	"codeberg.org/crowci/crow/v4/server/model"
	"codeberg.org/crowci/crow/v4/server/router/middleware/session"
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

	c.JSON(http.StatusOK, config)
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
	Job string `json:"job" binding:"required" example:"vacuumLogTable"` // The maintenance job to run
}

// RunMaintenanceNow runs database maintenance immediately
//
//	@Summary		Run maintenance now
//	@Description	Trigger database maintenance operation immediately
//	@Router			/admin/maintenance/run [post]
//	@Produce		json
//	@Accept			json
//	@Success		200	{object}	gin.H
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

	// Validate job type
	if req.Job != "vacuumLogTable" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job type. Supported jobs: vacuumLogTable"})
		return
	}

	if server.Config.Services.MaintenanceScheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Maintenance scheduler not available"})
		return
	}

	go func() {
		if err := server.Config.Services.MaintenanceScheduler.RunManual(); err != nil {
			log.Error().Err(err).Msg("Manual maintenance run failed")
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Maintenance started", "job": req.Job})
}
