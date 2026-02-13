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
	"time"

	"github.com/gin-gonic/gin"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/store"
)

// GetAutoscalers
//
//	@Summary	List autoscalers
//	@Router		/autoscalers [get]
//	@Produce	json
//	@Success	200	{array}	Autoscaler
//	@Tags		Autoscalers
//	@Param		Authorization	header	string	true	"Insert your personal access token"				default(Bearer <personal access token>)
//	@Param		page			query	int		false	"for response pagination, page offset number"	default(1)
//	@Param		perPage			query	int		false	"for response pagination, max items per page"	default(50)
//	@Param		search			query	string	false	"search in name, provider, region, ID"
//	@Param		provider		query	string	false	"filter by provider"
//	@Param		enabled			query	bool	false	"filter by enabled status"
//	@Param		sort			query	string	false	"sort field: id, name, provider, last_contact"	default(id)
//	@Param		order			query	string	false	"sort order: asc or desc"						default(asc)
func GetAutoscalers(c *gin.Context) {
	opts := &model.AutoscalerListOptions{
		ListOptions: *session.Pagination(c),
		Search:      c.Query("search"),
		Provider:    c.Query("provider"),
		Sort:        c.Query("sort"),
		Order:       c.Query("order"),
	}

	// Parse enabled filter
	if enabledStr := c.Query("enabled"); enabledStr != "" {
		enabled := enabledStr == "true"
		opts.Enabled = &enabled
	}

	autoscalers, err := store.FromContext(c).AutoscalerList(opts)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting autoscaler list. %s", err)
		return
	}

	c.JSON(http.StatusOK, autoscalers)
}

// GetAutoscaler
//
//	@Summary	Get an autoscaler
//	@Router		/autoscalers/{autoscaler_id} [get]
//	@Produce	json
//	@Success	200	{object}	Autoscaler
//	@Tags		Autoscalers
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		autoscaler_id	path	int		true	"the autoscaler's id"
func GetAutoscaler(c *gin.Context) {
	autoscalerID, err := strconv.ParseInt(c.Param("autoscaler_id"), 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	autoscaler, err := store.FromContext(c).AutoscalerFind(autoscalerID)
	if err != nil {
		handleDBError(c, err)
		return
	}
	c.JSON(http.StatusOK, autoscaler)
}

func GetAutoscalerSelf(c *gin.Context) {
	autoscaler := session.Autoscaler(c)
	c.JSON(http.StatusOK, autoscaler)
}

// PostAutoscaler
//
//	@Summary		Create a new autoscaler
//	@Description	Creates a new autoscaler with a random token. Configuration is provided by the autoscaler upon registration.
//	@Router			/autoscalers [post]
//	@Produce		json
//	@Success		200	{object}	AutoscalerWithToken
//	@Tags			Autoscalers
//	@Param			Authorization	header	string		true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			autoscaler		body	Autoscaler	true	"the autoscaler's data (only name is required)"
func PostAutoscaler(c *gin.Context) {
	in := &model.Autoscaler{}
	err := c.Bind(in)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	// Validate required fields
	if in.Name == "" {
		c.String(http.StatusBadRequest, "Name is required")
		return
	}

	user := session.User(c)

	autoscaler := &model.Autoscaler{
		Name:    in.Name,
		OwnerID: user.ID,
		OrgID:   model.IDNotSet,
		Enabled: true,
		Token:   model.GenerateNewAutoscalerToken(),
	}

	if err = store.FromContext(c).AutoscalerCreate(autoscaler); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Return token only on creation
	c.JSON(http.StatusOK, &model.AutoscalerWithToken{
		Autoscaler: autoscaler,
		Token:      autoscaler.Token,
	})
}

// PatchAutoscaler
//
//	@Summary	Update an autoscaler
//	@Router		/autoscalers/{autoscaler_id} [patch]
//	@Produce	json
//	@Success	200	{object}	Autoscaler
//	@Tags		Autoscalers
//	@Param		Authorization	header	string		true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		autoscaler_id	path	int			true	"the autoscaler's id"
//	@Param		autoscalerData	body	Autoscaler	true	"the autoscaler's data (only enabled can be updated)"
func PatchAutoscaler(c *gin.Context) {
	_store := store.FromContext(c)

	in := &model.Autoscaler{}
	err := c.Bind(in)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	autoscalerID, err := strconv.ParseInt(c.Param("autoscaler_id"), 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	autoscaler, err := _store.AutoscalerFind(autoscalerID)
	if err != nil {
		handleDBError(c, err)
		return
	}

	// Only allow updating the enabled status (similar to agent's NoSchedule)
	autoscaler.Enabled = in.Enabled

	err = _store.AutoscalerUpdate(autoscaler)
	if err != nil {
		c.AbortWithStatus(http.StatusConflict)
		return
	}

	c.JSON(http.StatusOK, autoscaler)
}

// DeleteAutoscaler
//
//	@Summary	Delete an autoscaler
//	@Router		/autoscalers/{autoscaler_id} [delete]
//	@Produce	plain
//	@Success	204
//	@Tags		Autoscalers
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		autoscaler_id	path	int		true	"the autoscaler's id"
func DeleteAutoscaler(c *gin.Context) {
	_store := store.FromContext(c)

	autoscalerID, err := strconv.ParseInt(c.Param("autoscaler_id"), 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	autoscaler, err := _store.AutoscalerFind(autoscalerID)
	if err != nil {
		handleDBError(c, err)
		return
	}

	if err = _store.AutoscalerDelete(autoscaler); err != nil {
		c.String(http.StatusInternalServerError, "Error deleting autoscaler. %s", err)
		return
	}
	c.Status(http.StatusNoContent)
}

// PostAutoscalerHeartbeat
//
//	@Summary		Update autoscaler heartbeat
//	@Description	Autoscalers send heartbeats to update their status and configuration
//	@Router			/autoscalers/{autoscaler_id}/heartbeat [post]
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Tags			Autoscalers
//	@Param			Authorization	header	string	true	"Insert autoscaler token"	default(Bearer <autoscaler token>)
//	@Param			autoscaler_id	path	int		true	"the autoscaler's id"
//	@Param			heartbeat		body	object	true	"heartbeat data with active_agents, pending_agents, version, and configuration"
func PostAutoscalerHeartbeat(c *gin.Context) {
	_store := store.FromContext(c)

	autoscalerID, err := strconv.ParseInt(c.Param("autoscaler_id"), 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	// Parse heartbeat data
	var heartbeat struct {
		ActiveAgents      int32             `json:"active_agents"`
		PendingAgents     int32             `json:"pending_agents"`
		Version           string            `json:"version"`
		MinAgents         int32             `json:"min_agents"`
		MaxAgents         int32             `json:"max_agents"`
		WorkflowsPerAgent int32             `json:"workflows_per_agent"`
		Provider          string            `json:"provider"`
		InstanceType      string            `json:"instance_type"`
		Region            string            `json:"region"`
		AgentLabels       map[string]string `json:"agent_labels"`
		LimitCPUQuota     string            `json:"limit_cpu_quota"`
		LimitMem          string            `json:"limit_mem"`
	}
	if err := c.Bind(&heartbeat); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	// Verify autoscaler exists and token matches
	autoscaler, err := _store.AutoscalerFind(autoscalerID)
	if err != nil {
		handleDBError(c, err)
		return
	}

	// TODO: Add middleware to authenticate autoscaler by token
	// For now, we trust that the caller has access to this endpoint

	// Update autoscaler with heartbeat data and configuration
	autoscaler.ActiveAgents = heartbeat.ActiveAgents
	autoscaler.PendingAgents = heartbeat.PendingAgents
	autoscaler.Version = heartbeat.Version
	autoscaler.LastContact = time.Now().Unix()

	// Update configuration fields if provided
	if heartbeat.MinAgents > 0 || heartbeat.MaxAgents > 0 {
		autoscaler.MinAgents = heartbeat.MinAgents
		autoscaler.MaxAgents = heartbeat.MaxAgents
	}
	if heartbeat.WorkflowsPerAgent > 0 {
		autoscaler.WorkflowsPerAgent = heartbeat.WorkflowsPerAgent
	}
	if heartbeat.Provider != "" {
		autoscaler.Provider = heartbeat.Provider
	}
	if heartbeat.InstanceType != "" {
		autoscaler.InstanceType = heartbeat.InstanceType
	}
	if heartbeat.Region != "" {
		autoscaler.Region = heartbeat.Region
	}
	if heartbeat.AgentLabels != nil {
		autoscaler.AgentLabels = heartbeat.AgentLabels
	}
	if heartbeat.LimitCPUQuota != "" {
		autoscaler.LimitCPUQuota = heartbeat.LimitCPUQuota
	}
	if heartbeat.LimitMem != "" {
		autoscaler.LimitMem = heartbeat.LimitMem
	}

	err = _store.AutoscalerUpdate(autoscaler)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error updating heartbeat. %s", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"last_contact": time.Now().Unix(),
	})
}

// PostAutoscalerToken
//
//	@Summary		Regenerate autoscaler token
//	@Description	Generates a new token for the autoscaler and invalidates the old one
//	@Router			/autoscalers/{autoscaler_id}/token [post]
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Tags			Autoscalers
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			autoscaler_id	path	int		true	"the autoscaler's id"
func PostAutoscalerToken(c *gin.Context) {
	_store := store.FromContext(c)

	autoscalerID, err := strconv.ParseInt(c.Param("autoscaler_id"), 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	autoscaler, err := _store.AutoscalerFind(autoscalerID)
	if err != nil {
		handleDBError(c, err)
		return
	}

	// Generate new token
	autoscaler.Token = model.GenerateNewAutoscalerToken()

	err = _store.AutoscalerUpdate(autoscaler)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error updating token. %s", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": autoscaler.Token,
	})
}
