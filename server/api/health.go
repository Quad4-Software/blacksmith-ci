// Copyright 2025 Crow CI Authors
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
	"time"

	"github.com/gin-gonic/gin"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/store"
	"codefloe.com/crowci/crow/v5/version"
)

// HealthStatus represents the health status of the server.
type HealthStatus struct {
	Status     string            `json:"status"`
	InstanceID string            `json:"instance_id"`
	Timestamp  int64             `json:"timestamp"`
	Checks     map[string]string `json:"checks"`
	Version    string            `json:"version"`
	Uptime     float64           `json:"uptime_seconds"`
} //	@name	HealthStatus

var serverStartTime = time.Now()

// GetHealth returns the health status of the server.
func GetHealth(c *gin.Context) {
	store := store.FromContext(c)

	checks := make(map[string]string)

	// Check database connectivity
	if err := store.Ping(); err != nil {
		checks["database"] = "unhealthy: " + err.Error()
		c.JSON(http.StatusServiceUnavailable, &HealthStatus{
			Status:     "unhealthy",
			InstanceID: server.Config.Server.InstanceID,
			Timestamp:  time.Now().Unix(),
			Checks:     checks,
			Version:    version.String(),
			Uptime:     time.Since(serverStartTime).Seconds(),
		})
		return
	}
	checks["database"] = "healthy"

	// All checks passed
	c.JSON(http.StatusOK, &HealthStatus{
		Status:     "healthy",
		InstanceID: server.Config.Server.InstanceID,
		Timestamp:  time.Now().Unix(),
		Checks:     checks,
		Version:    version.String(),
		Uptime:     time.Since(serverStartTime).Seconds(),
	})
}

// GetHealthLive is a lightweight liveness probe (no dependencies checked).
func GetHealthLive(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":      "alive",
		"instance_id": server.Config.Server.InstanceID,
		"timestamp":   time.Now().Unix(),
	})
}

// GetHealthReady is a readiness probe that checks if the server is ready to serve traffic.
func GetHealthReady(c *gin.Context) {
	store := store.FromContext(c)

	// Check if database is accessible and ready
	if err := store.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":      "not_ready",
			"instance_id": server.Config.Server.InstanceID,
			"timestamp":   time.Now().Unix(),
			"reason":      "database not accessible: " + err.Error(),
		})
		return
	}

	// Check if critical services are available
	// Add more readiness checks here as needed

	c.JSON(http.StatusOK, gin.H{
		"status":      "ready",
		"instance_id": server.Config.Server.InstanceID,
		"timestamp":   time.Now().Unix(),
	})
}
