// Copyright 2022 Woodpecker Authors
// Copyright 2021 Informatyka Boguslawski sp. z o.o. sp.k., http://www.ib.pl/
// Copyright 2018 Drone.IO Inc.
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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v5/pipeline/frontend/metadata"
	"codeberg.org/crowci/crow/v5/pipeline/frontend/yaml"
	"codeberg.org/crowci/crow/v5/pipeline/frontend/yaml/constraint"
	"codeberg.org/crowci/crow/v5/server"
	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/pipeline"
	"codeberg.org/crowci/crow/v5/server/pipeline/stepbuilder"
	"codeberg.org/crowci/crow/v5/server/router/middleware/session"
	"codeberg.org/crowci/crow/v5/server/store"
	"codeberg.org/crowci/crow/v5/server/store/types"
)

// CreatePipeline
//
//	@Summary	Trigger a manual pipeline
//	@Router		/repos/{repo_id}/pipelines [post]
//	@Produce	json
//	@Success	200	{object}	Pipeline
//	@Tags		Pipelines
//	@Param		Authorization	header	string			true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int				true	"the repository id"
//	@Param		options			body	PipelineOptions	true	"the options for the pipeline to run"
func CreatePipeline(c *gin.Context) {
	_store := store.FromContext(c)
	repo := session.Repo(c)

	if !repo.AllowManual {
		_ = c.AbortWithError(http.StatusForbidden, fmt.Errorf("repo does not allow manual pipelines"))
		return
	}

	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get forge from repo")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// parse create options
	var opts model.PipelineOptions
	err = json.NewDecoder(c.Request.Body).Decode(&opts)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	user := session.User(c)

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	isSecondaryForge := false
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
		isSecondaryForge = !userForge.IsPrimary
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, user, repo.ForgeID, isSecondaryForge)
	if c.IsAborted() {
		return
	}

	lastCommit, err := _forge.BranchHead(c, user, repo, opts.Branch)
	if err != nil {
		handleForgeError(c, err)
		return
	}

	tmpPipeline := createTmpPipeline(model.EventManual, lastCommit, user, &opts)

	createOpts := pipeline.CreateOptions{
		WorkflowFilter: opts.Workflows,
	}
	pl, err := pipeline.Create(c, _store, repo, tmpPipeline, createOpts)
	if err != nil {
		handlePipelineErr(c, err)
	} else {
		c.JSON(http.StatusOK, pl)
	}
}

func createTmpPipeline(event model.WebhookEvent, commit *model.Commit, user *model.User, opts *model.PipelineOptions) *model.Pipeline {
	messageText := "Manual run"
	if len(opts.Variables) > 0 {
		varsText := ""
		first := true
		for k, v := range opts.Variables {
			if !first {
				varsText += ", "
			}
			varsText += k + ":" + fmt.Sprint(v)
			first = false
		}
		messageText += ". Extra vars: " + varsText
	}

	return &model.Pipeline{
		Event:     event,
		Commit:    commit.SHA,
		Branch:    opts.Branch,
		Timestamp: time.Now().UTC().Unix(),

		Avatar:  user.Avatar,
		Message: messageText,

		Ref:                 opts.Branch,
		AdditionalVariables: opts.Variables,

		Author: user.Login,
		Email:  user.Email,

		ForgeURL: commit.ForgeURL,
	}
}

// GetPipelines
//
//	@Summary		List repository pipelines
//	@Description	Get a list of pipelines for a repository.
//	@Router			/repos/{repo_id}/pipelines [get]
//	@Produce		json
//	@Success		200	{array}	Pipeline
//	@Tags			Pipelines
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			repo_id			path	int		true	"the repository id"
//	@Param			page			query	int		false	"for response pagination, page offset number"	default(1)
//	@Param			perPage			query	int		false	"for response pagination, max items per page"	default(50)
//	@Param			before			query	string	false	"only return pipelines before this RFC3339 date"
//	@Param			after			query	string	false	"only return pipelines after this RFC3339 date"
//	@Param			branch			query	string	false	"filter pipelines by branch"
//	@Param			event			query	string	false	"filter pipelines by webhook events (comma separated)"
//	@Param			ref				query	string	false	"filter pipelines by strings contained in ref"
//	@Param			status			query	string	false	"filter pipelines by status"
//	@Param			search			query	string	false	"search pipelines by number, message, branch, or author"
func GetPipelines(c *gin.Context) {
	repo := session.Repo(c)

	filter := &model.PipelineFilter{
		Branch:      c.Query("branch"),
		RefContains: c.Query("ref"),
		Search:      c.Query("search"),
	}

	if events := c.Query("event"); events != "" {
		eventList := strings.Split(events, ",")
		wel := make(model.WebhookEventList, 0, len(eventList))
		for _, event := range eventList {
			we := model.WebhookEvent(event)
			if err := we.Validate(); err != nil {
				_ = c.AbortWithError(http.StatusBadRequest, err)
				return
			}
			wel = append(wel, we)
		}
		filter.Events = wel
	}

	if status := c.Query("status"); status != "" {
		ps := model.StatusValue(status)
		if err := ps.Validate(); err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		filter.Status = ps
	}

	if before := c.Query("before"); before != "" {
		beforeDt, err := time.Parse(time.RFC3339, before)
		if err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		filter.Before = beforeDt.Unix()
	}

	if after := c.Query("after"); after != "" {
		afterDt, err := time.Parse(time.RFC3339, after)
		if err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		filter.After = afterDt.Unix()
	}

	pipelines, err := store.FromContext(c).GetPipelineList(repo, session.Pagination(c), filter)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, pipelines)
}

// DeletePipeline
//
//	@Summary	Delete a pipeline
//	@Router		/repos/{repo_id}/pipelines/{number} [delete]
//	@Produce	plain
//	@Success	204
//	@Tags		Pipelines
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
//	@Param		number			path	int		true	"the number of the pipeline"
func DeletePipeline(c *gin.Context) {
	_store := store.FromContext(c)

	repo := session.Repo(c)
	num, err := strconv.ParseInt(c.Param("number"), 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	pl, err := _store.GetPipelineNumber(repo, num)
	if err != nil {
		handleDBError(c, err)
		return
	}

	if ok := pipelineDeleteAllowed(pl); !ok {
		c.String(http.StatusUnprocessableEntity, "Cannot delete pipeline with status %s", pl.Status)
		return
	}

	err = store.FromContext(c).DeletePipeline(pl)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error deleting pipeline. %s", err)
		return
	}

	c.Status(http.StatusNoContent)
}

// GetPipeline
//
//	@Summary	Get a repositories pipeline
//	@Router		/repos/{repo_id}/pipelines/{number} [get]
//	@Produce	json
//	@Success	200	{object}	Pipeline
//	@Tags		Pipelines
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
//	@Param		number			path	int		true	"the number of the pipeline, OR 'latest'"
func GetPipeline(c *gin.Context) {
	_store := store.FromContext(c)
	if c.Param("number") == "latest" {
		GetPipelineLast(c)
		return
	}

	repo := session.Repo(c)
	num, err := strconv.ParseInt(c.Param("number"), 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	pl, err := _store.GetPipelineNumber(repo, num)
	if err != nil {
		handleDBError(c, err)
		return
	}
	if pl.Workflows, err = _store.WorkflowGetTree(pl); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, pl)
}

func GetPipelineLast(c *gin.Context) {
	_store := store.FromContext(c)
	repo := session.Repo(c)
	branch := c.DefaultQuery("branch", repo.Branch)

	pl, err := _store.GetPipelineLast(repo, branch)
	if err != nil {
		handleDBError(c, err)
		return
	}

	if pl.Workflows, err = _store.WorkflowGetTree(pl); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, pl)
}

// GetStepLogs
//
//	@Summary	Get logs for a pipeline step
//	@Router		/repos/{repo_id}/logs/{number}/{stepID} [get]
//	@Produce	json
//	@Success	200	{array}	LogEntry
//	@Tags		Pipeline logs
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
//	@Param		number			path	int		true	"the number of the pipeline"
//	@Param		stepID			path	int		true	"the step id"
func GetStepLogs(c *gin.Context) {
	_store := store.FromContext(c)
	repo := session.Repo(c)

	// parse the pipeline number and step sequence number from
	// the request parameter.
	num, err := strconv.ParseInt(c.Params.ByName("number"), 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	pl, err := _store.GetPipelineNumber(repo, num)
	if err != nil {
		handleDBError(c, err)
		return
	}

	stepID, err := strconv.ParseInt(c.Params.ByName("stepId"), 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	step, err := _store.StepLoad(stepID)
	if err != nil {
		handleDBError(c, err)
		return
	}

	if step.PipelineID != pl.ID {
		// make sure we cannot read arbitrary logs by id
		_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("step with id %d is not part of repo %s", stepID, repo.FullName))
		return
	}

	logs, err := server.Config.Services.LogStore.LogFind(step)
	if err != nil {
		handleDBError(c, err)
		return
	}

	c.JSON(http.StatusOK, logs)
}

// DeleteStepLogs
//
//	@Summary	Delete step logs of a pipeline
//	@Router		/repos/{repo_id}/logs/{number}/{stepId} [delete]
//	@Produce	plain
//	@Success	204
//	@Tags		Pipeline logs
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
//	@Param		number			path	int		true	"the number of the pipeline"
//	@Param		stepId			path	int		true	"the step id"
func DeleteStepLogs(c *gin.Context) {
	_store := store.FromContext(c)
	repo := session.Repo(c)

	pipelineNumber, err := strconv.ParseInt(c.Params.ByName("number"), 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	_pipeline, err := _store.GetPipelineNumber(repo, pipelineNumber)
	if err != nil {
		handleDBError(c, err)
		return
	}

	stepID, err := strconv.ParseInt(c.Params.ByName("stepId"), 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	_step, err := _store.StepLoad(stepID)
	if err != nil {
		handleDBError(c, err)
		return
	}

	if _step.PipelineID != _pipeline.ID {
		// make sure we cannot read arbitrary logs by id
		_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("step with id %d is not part of repo %s", stepID, repo.FullName))
		return
	}

	switch _step.State {
	case model.StatusRunning, model.StatusPending:
		c.String(http.StatusUnprocessableEntity, "Cannot delete logs for a pending or running step")
		return
	}

	err = _store.LogDelete(_step)
	if err != nil {
		handleDBError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// GetPipelineConfig
//
//	@Summary	Get configuration files for a pipeline
//	@Router		/repos/{repo_id}/pipelines/{number}/config [get]
//	@Produce	json
//	@Success	200	{array}	Config
//	@Tags		Pipelines
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
//	@Param		number			path	int		true	"the number of the pipeline"
func GetPipelineConfig(c *gin.Context) {
	_store := store.FromContext(c)
	repo := session.Repo(c)
	num, err := strconv.ParseInt(c.Param("number"), 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	pl, err := _store.GetPipelineNumber(repo, num)
	if err != nil {
		handleDBError(c, err)
		return
	}

	configs, err := _store.ConfigsForPipeline(pl.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Get workflows to filter configs to only those that were actually executed
	workflows, err := _store.WorkflowGetTree(pl)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Collect unique pipeline names from executed workflows
	executedPipelineNames := make(map[string]struct{})
	for _, wf := range workflows {
		if wf.PipelineName != "" {
			executedPipelineNames[wf.PipelineName] = struct{}{}
		}
	}

	// Filter configs to only include those with executed workflows
	filteredConfigs := filterConfigsByExecutedWorkflows(configs, executedPipelineNames)

	c.JSON(http.StatusOK, filteredConfigs)
}

// filterConfigsByExecutedWorkflows returns only configs that have corresponding executed workflows.
func filterConfigsByExecutedWorkflows(configs []*model.Config, executedPipelineNames map[string]struct{}) []*model.Config {
	if len(executedPipelineNames) == 0 {
		return configs // Return all configs if no workflow info available
	}

	var filtered []*model.Config
	for _, config := range configs {
		pipelineName := getConfigPipelineName(config)
		if _, exists := executedPipelineNames[pipelineName]; exists {
			filtered = append(filtered, config)
		}
	}

	return filtered
}

// getConfigPipelineName determines the pipeline name for a config.
// This matches the logic in stepbuilder: YAML name field if present, otherwise sanitized file name.
func getConfigPipelineName(config *model.Config) string {
	parsed, err := yaml.ParseBytes(config.Data)
	if err == nil && parsed.Name != "" {
		return parsed.Name
	}
	return stepbuilder.SanitizePath(config.Name)
}

// ManualWorkflow represents a workflow that can be manually triggered.
type ManualWorkflow struct {
	Name         string   `json:"name"`
	PipelineName string   `json:"pipeline_name"`
	DependsOn    []string `json:"depends_on,omitempty"`
	HasManual    bool     `json:"has_manual"`
} //	@name	ManualWorkflow

// ManualWorkflowsResponse is the response for the GetManualWorkflows endpoint.
type ManualWorkflowsResponse struct {
	Workflows []ManualWorkflow `json:"workflows"`
	Warnings  []string         `json:"warnings,omitempty"`
} //	@name	ManualWorkflowsResponse

// GetRepoConfigs
//
//	@Summary		Get workflow configurations for a branch
//	@Description	Returns parsed workflow information for a specific branch, optionally filtered by trigger event type.
//	@Router			/repos/{repo_id}/configs [get]
//	@Produce		json
//	@Success		200	{object}	ManualWorkflowsResponse
//	@Tags			Pipelines
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			repo_id			path	int		true	"the repository id"
//	@Param			branch			query	string	true	"the branch to get configs from"
//	@Param			filter			query	string	false	"filter workflows by event type (e.g., 'manual')"
func GetRepoConfigs(c *gin.Context) {
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)

	branch := c.Query("branch")
	if branch == "" {
		c.String(http.StatusBadRequest, "branch parameter is required")
		return
	}

	filter := c.Query("filter")

	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get forge from repo")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	isSecondaryForge := false
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
		isSecondaryForge = !userForge.IsPrimary
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, user, repo.ForgeID, isSecondaryForge)
	if c.IsAborted() {
		return
	}

	// Get the branch head commit
	commit, err := _forge.BranchHead(c, user, repo, branch)
	if err != nil {
		handleForgeError(c, err)
		return
	}

	// Create a temporary pipeline to fetch configs (we won't persist it)
	tmpPipeline := &model.Pipeline{
		Event:  model.EventManual,
		Commit: commit.SHA,
		Branch: branch,
		Ref:    branch,
	}

	// Fetch the configs from the forge
	configService := server.Config.Services.Manager.ConfigServiceFromRepo(repo)
	forgeYamlConfigs, err := configService.Fetch(c, _forge, user, repo, tmpPipeline, nil, false)
	if err != nil {
		log.Debug().Err(err).Str("repo", repo.FullName).Msg("error fetching configs")
		c.String(http.StatusInternalServerError, "Error fetching pipeline configs: %s", err.Error())
		return
	}

	if len(forgeYamlConfigs) == 0 {
		c.JSON(http.StatusOK, ManualWorkflowsResponse{
			Workflows: []ManualWorkflow{},
		})
		return
	}

	// Parse each YAML file and extract workflow information
	workflows := make([]ManualWorkflow, 0, len(forgeYamlConfigs))
	workflowMap := make(map[string]ManualWorkflow) // for dependency resolution
	var warnings []string

	for _, forgeConfig := range forgeYamlConfigs {
		parsed, err := yaml.ParseBytes(forgeConfig.Data)
		if err != nil {
			log.Debug().Err(err).Str("file", forgeConfig.Name).Msg("error parsing yaml")
			continue
		}

		// Determine workflow name
		workflowName := parsed.Name
		if workflowName == "" {
			workflowName = stepbuilder.SanitizePath(forgeConfig.Name)
		}

		// Check if workflow has manual trigger
		hasManual := workflowHasManualTrigger(parsed.When)
		log.Debug().
			Str("workflow", workflowName).
			Bool("hasManual", hasManual).
			Int("constraintCount", len(parsed.When.Constraints)).
			Msg("checking workflow for manual trigger")

		wf := ManualWorkflow{
			Name:         workflowName,
			PipelineName: stepbuilder.SanitizePath(forgeConfig.Name),
			DependsOn:    parsed.DependsOn,
			HasManual:    hasManual,
		}

		workflowMap[workflowName] = wf

		// If filtering by manual, only include workflows with manual trigger
		if filter == "manual" && !hasManual {
			continue
		}

		workflows = append(workflows, wf)
	}

	// Generate warnings for dependencies that don't have manual trigger
	if filter == "manual" {
		for _, wf := range workflows {
			for _, dep := range wf.DependsOn {
				if depWf, exists := workflowMap[dep]; exists && !depWf.HasManual {
					warnings = append(warnings, fmt.Sprintf("Workflow '%s' depends on '%s' which does not have a manual trigger", wf.Name, dep))
				}
			}
		}
	}

	c.JSON(http.StatusOK, ManualWorkflowsResponse{
		Workflows: workflows,
		Warnings:  warnings,
	})
}

// workflowHasManualTrigger checks if a workflow's when constraint includes the manual event.
func workflowHasManualTrigger(when constraint.When) bool {
	// If no constraints, check default behavior
	if when.IsEmpty() {
		log.Debug().Msg("workflowHasManualTrigger: when is empty, returning false")
		return false // Default doesn't include manual
	}

	// Check each constraint for manual event
	for i, c := range when.Constraints {
		log.Debug().
			Int("constraintIndex", i).
			Strs("events", c.Event).
			Msg("workflowHasManualTrigger: checking constraint")

		// If event is not specified in constraint, it doesn't filter by event
		if len(c.Event) == 0 {
			// No event filter means all events match, including manual
			log.Debug().Msg("workflowHasManualTrigger: no event filter, returning true")
			return true
		}
		// Check if manual is in the event list
		for _, event := range c.Event {
			if event == metadata.EventManual {
				log.Debug().Msg("workflowHasManualTrigger: found manual event, returning true")
				return true
			}
		}
	}

	log.Debug().Msg("workflowHasManualTrigger: no manual trigger found, returning false")
	return false
}

// GetPipelineMetadata
//
//	@Summary	Get metadata for a pipeline or a specific workflow, including previous pipeline info
//	@Router		/repos/{repo_id}/pipelines/{number}/metadata [get]
//	@Produce	json
//	@Success	200	{object}	metadata.Metadata
//	@Tags		Pipelines
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
//	@Param		number			path	int		true	"the number of the pipeline"
func GetPipelineMetadata(c *gin.Context) {
	repo := session.Repo(c)
	num, err := strconv.ParseInt(c.Param("number"), 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	_store := store.FromContext(c)
	currentPipeline, err := _store.GetPipelineNumber(repo, num)
	if err != nil {
		handleDBError(c, err)
		return
	}

	forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	prevPipeline, err := _store.GetPipelineLastBefore(repo, currentPipeline.Branch, currentPipeline.ID)
	if err != nil && !errors.Is(err, types.RecordNotExist) {
		handleDBError(c, err)
		return
	}

	metadata := stepbuilder.MetadataFromStruct(forge, repo, currentPipeline, prevPipeline, nil, server.Config.Server.Host)
	c.JSON(http.StatusOK, metadata)
}

// CancelPipeline
//
//	@Summary	Cancel a pipeline
//	@Router		/repos/{repo_id}/pipelines/{number}/cancel [post]
//	@Produce	plain
//	@Success	200
//	@Tags		Pipelines
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
//	@Param		number			path	int		true	"the number of the pipeline"
func CancelPipeline(c *gin.Context) {
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get forge from repo")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	isSecondaryForge := false
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
		isSecondaryForge = !userForge.IsPrimary
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, user, repo.ForgeID, isSecondaryForge)
	if c.IsAborted() {
		return
	}

	num, _ := strconv.ParseInt(c.Params.ByName("number"), 10, 64)

	pl, err := _store.GetPipelineNumber(repo, num)
	if err != nil {
		handleDBError(c, err)
		return
	}

	if err := pipeline.Cancel(c, _forge, _store, repo, user, pl); err != nil {
		handlePipelineErr(c, err)
	} else {
		c.AbortWithStatus(http.StatusNoContent)
	}
}

// PostApproval
//
//	@Summary	Approve and start a pipeline
//	@Router		/repos/{repo_id}/pipelines/{number}/approve [post]
//	@Produce	json
//	@Success	200	{object}	Pipeline
//	@Tags		Pipelines
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
//	@Param		number			path	int		true	"the number of the pipeline"
func PostApproval(c *gin.Context) {
	var (
		_store = store.FromContext(c)
		repo   = session.Repo(c)
		user   = session.User(c)
		num, _ = strconv.ParseInt(c.Params.ByName("number"), 10, 64)
	)

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	isSecondaryForge := false
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
		isSecondaryForge = !userForge.IsPrimary
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, user, repo.ForgeID, isSecondaryForge)
	if c.IsAborted() {
		return
	}

	pl, err := _store.GetPipelineNumber(repo, num)
	if err != nil {
		handleDBError(c, err)
		return
	}

	newPipeline, err := pipeline.Approve(c, _store, pl, user, repo)
	if err != nil {
		handlePipelineErr(c, err)
	} else {
		c.JSON(http.StatusOK, newPipeline)
	}
}

// PostDecline
//
//	@Summary	Decline a pipeline
//	@Router		/repos/{repo_id}/pipelines/{number}/decline [post]
//	@Produce	json
//	@Success	200	{object}	Pipeline
//	@Tags		Pipelines
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
//	@Param		number			path	int		true	"the number of the pipeline"
func PostDecline(c *gin.Context) {
	var (
		_store = store.FromContext(c)
		repo   = session.Repo(c)
		user   = session.User(c)
		num, _ = strconv.ParseInt(c.Params.ByName("number"), 10, 64)
	)

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	isSecondaryForge := false
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
		isSecondaryForge = !userForge.IsPrimary
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, user, repo.ForgeID, isSecondaryForge)
	if c.IsAborted() {
		return
	}

	pl, err := _store.GetPipelineNumber(repo, num)
	if err != nil {
		handleDBError(c, err)
		return
	}

	pl, err = pipeline.Decline(c, _store, pl, user, repo)
	if err != nil {
		handlePipelineErr(c, err)
	} else {
		c.JSON(http.StatusOK, pl)
	}
}

// GetPipelineQueue
//
//	@Summary	List pipelines in queue
//	@Router		/pipelines [get]
//	@Produce	json
//	@Success	200	{array}	Feed
//	@Tags		Pipeline queues
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
func GetPipelineQueue(c *gin.Context) {
	out, err := store.FromContext(c).GetPipelineQueue()
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting pipeline queue. %s", err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// PostPipeline
//
//	@Summary		Restart a pipeline
//	@Description	Restarts a pipeline optional with altered event, deploy or environment
//	@Router			/repos/{repo_id}/pipelines/{number} [post]
//	@Produce		json
//	@Success		200	{object}	Pipeline
//	@Tags			Pipelines
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			repo_id			path	int		true	"the repository id"
//	@Param			number			path	int		true	"the number of the pipeline"
//	@Param			event			query	string	false	"override the event type"
//	@Param			deploy_to		query	string	false	"override the target deploy value"
func PostPipeline(c *gin.Context) {
	_store := store.FromContext(c)
	repo := session.Repo(c)

	num, err := strconv.ParseInt(c.Param("number"), 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	user, err := _store.GetUser(repo.UserID)
	if err != nil {
		handleDBError(c, err)
		return
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	// This ensures we use the correct OAuth tokens for the forge the repo belongs to.
	var isSecondaryForge bool
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
		isSecondaryForge = !userForge.IsPrimary
	}

	pl, err := _store.GetPipelineNumber(repo, num)
	if err != nil {
		handleDBError(c, err)
		return
	}

	// Store the original event type before any modifications
	originalEvent := pl.Event
	log.Debug().
		Str("stored_original_event", string(originalEvent)).
		Str("current_event", string(pl.Event)).
		Int64("pipeline_number", pl.Number).
		Msg("Stored original event before modifications")

	// refresh the token to make sure, pipeline.Restart can still obtain the pipeline config if necessary again
	refreshUserTokenForForge(c, user, repo.ForgeID, isSecondaryForge)

	// make Deploy overridable

	// make Deploy task overridable
	pl.DeployTask = c.DefaultQuery("deploy_task", pl.DeployTask)

	// make Event overridable to deploy
	// TODO: refactor to use own proper API for deploy
	if event, ok := c.GetQuery("event"); ok {
		log.Debug().
			Str("query_event", event).
			Str("original_event", string(originalEvent)).
			Msg("Event query parameter found")
		pl.Event = model.WebhookEvent(event)
		if pl.Event != model.EventDeploy {
			_ = c.AbortWithError(http.StatusBadRequest, model.ErrInvalidWebhookEvent)
			return
		}

		if !repo.AllowDeploy {
			_ = c.AbortWithError(http.StatusForbidden, fmt.Errorf("repo does not allow deployments"))
			return
		}

		// Check team restriction for deployments
		if repo.DeployTeam != "" {
			triggeringUser := session.User(c)
			_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
			if err != nil {
				_ = c.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			teams, err := _forge.Teams(c, triggeringUser)
			if err != nil {
				_ = c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to fetch user teams: %w", err))
				return
			}
			isMember := false
			for _, t := range teams {
				if t.Login == repo.DeployTeam {
					isMember = true
					break
				}
			}
			if !isMember {
				_ = c.AbortWithError(http.StatusForbidden, fmt.Errorf("user not in deploy team %q", repo.DeployTeam))
				return
			}
		}

		pl.DeployTo = c.DefaultQuery("deploy_to", pl.DeployTo)
	}

	// Read query string parameters into pipelineParams, exclude reserved params
	envs := map[string]string{}
	for key, val := range c.Request.URL.Query() {
		switch key {
		// Skip some options of the endpoint
		case "fork", "event", "deploy_to", "deploy_task":
			continue
		default:
			// We only accept string literals, because pipeline parameters will be
			// injected as environment variables
			// TODO: sanitize the value
			envs[key] = val[0]
		}
	}

	// Pass original event in envs to help determine if this is a deployment from a non-deployment
	log.Debug().
		Str("pl.Event", string(pl.Event)).
		Str("originalEvent", string(originalEvent)).
		Str("model.EventDeploy", string(model.EventDeploy)).
		Bool("pl.Event == model.EventDeploy", pl.Event == model.EventDeploy).
		Bool("originalEvent != model.EventDeploy", originalEvent != model.EventDeploy).
		Bool("condition", pl.Event == model.EventDeploy && originalEvent != model.EventDeploy).
		Msg("Checking deployment condition")

	if pl.Event == model.EventDeploy && originalEvent != model.EventDeploy {
		envs["CROW_ORIGINAL_EVENT"] = string(originalEvent)
		log.Debug().
			Str("original_event", string(originalEvent)).
			Str("new_event", string(pl.Event)).
			Int64("pipeline_number", pl.Number).
			Msg("Triggering deployment from non-deployment pipeline")
	} else {
		log.Debug().
			Str("original_event", string(originalEvent)).
			Str("new_event", string(pl.Event)).
			Int64("pipeline_number", pl.Number).
			Bool("is_deployment", pl.Event == model.EventDeploy).
			Bool("original_is_deployment", originalEvent == model.EventDeploy).
			Msg("Not marking as deployment from other event")
	}
	log.Debug().
		Interface("envs_passed_to_restart", envs).
		Str("pl.Event", string(pl.Event)).
		Bool("has_crow_original_event", envs["CROW_ORIGINAL_EVENT"] != "").
		Msg("About to call pipeline.Restart")
	newPipeline, err := pipeline.Restart(c, _store, pl, user, repo, envs)
	if err != nil {
		handlePipelineErr(c, err)
	} else {
		// Notify any active streaming sessions for the old pipeline that it has been restarted
		go notifyPipelineRestart(pl, newPipeline)

		c.JSON(http.StatusOK, newPipeline)
	}
}

// DeletePipelineLogs
//
//	@Summary	Deletes all logs of a pipeline
//	@Router		/repos/{repo_id}/logs/{number} [delete]
//	@Produce	plain
//	@Success	204
//	@Tags		Pipeline logs
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
//	@Param		number			path	int		true	"the number of the pipeline"
func DeletePipelineLogs(c *gin.Context) {
	_store := store.FromContext(c)

	repo := session.Repo(c)
	num, _ := strconv.ParseInt(c.Params.ByName("number"), 10, 64)

	pl, err := _store.GetPipelineNumber(repo, num)
	if err != nil {
		handleDBError(c, err)
		return
	}

	steps, err := _store.StepList(pl)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if ok := pipelineDeleteAllowed(pl); !ok {
		c.String(http.StatusUnprocessableEntity, "Cannot delete logs for pipeline with status %s", pl.Status)
		return
	}

	for _, step := range steps {
		if lErr := server.Config.Services.LogStore.LogDelete(step); err != nil {
			err = errors.Join(err, lErr)
		}
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "Error deleting pipeline logs. %s", err)
		return
	}

	c.Status(http.StatusNoContent)
}

// CancelWorkflow
//
//	@Summary	Cancel a specific workflow in a pipeline
//	@Router		/repos/{repo_id}/pipelines/{number}/workflows/{workflow_id}/cancel [post]
//	@Produce	plain
//	@Success	200
//	@Tags		Pipelines
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
//	@Param		number			path	int		true	"the number of the pipeline"
//	@Param		workflow_id		path	int		true	"the id of the workflow to cancel"
func CancelWorkflow(c *gin.Context) {
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get forge from repo")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	isSecondaryForge := false
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
		isSecondaryForge = !userForge.IsPrimary
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, user, repo.ForgeID, isSecondaryForge)
	if c.IsAborted() {
		return
	}

	num, err := strconv.ParseInt(c.Params.ByName("number"), 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	workflowID, err := strconv.ParseInt(c.Params.ByName("workflow_id"), 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	pl, err := _store.GetPipelineNumber(repo, num)
	if err != nil {
		handleDBError(c, err)
		return
	}

	if err := pipeline.CancelWorkflow(c, _forge, _store, repo, user, pl, workflowID); err != nil {
		handlePipelineErr(c, err)
	} else {
		c.AbortWithStatus(http.StatusNoContent)
	}
}

// notifyPipelineRestart sends redirect signals to any active log streaming sessions
// for the old pipeline, directing them to the new restarted pipeline.
func notifyPipelineRestart(oldPipeline, newPipeline *model.Pipeline) {
	log.Info().Int64("oldPipelineID", oldPipeline.ID).Int64("newPipelineID", newPipeline.ID).Msg("notifying active sessions of pipeline restart")

	// Get workflows for the old pipeline to find step IDs
	oldWorkflows, err := server.Config.Services.Store.WorkflowGetTree(oldPipeline)
	if err != nil {
		log.Error().Err(err).Int64("oldPipelineID", oldPipeline.ID).Msg("failed to get old pipeline workflows for restart notification")
		return
	}

	// Get workflows for the new pipeline to find corresponding step IDs
	newWorkflows, err := server.Config.Services.Store.WorkflowGetTree(newPipeline)
	if err != nil {
		log.Error().Err(err).Int64("newPipelineID", newPipeline.ID).Msg("failed to get new pipeline workflows for restart notification")
		return
	}

	// Create a mapping from old step names to new step IDs
	newStepsByName := make(map[string]int64)
	for _, newWorkflow := range newWorkflows {
		for _, newStep := range newWorkflow.Children {
			newStepsByName[newStep.Name] = newStep.ID
		}
	}

	// Send redirect signals to old step streaming sessions
	for _, oldWorkflow := range oldWorkflows {
		for _, oldStep := range oldWorkflow.Children {
			if newStepID, exists := newStepsByName[oldStep.Name]; exists {
				// Send redirect signal from old step to new step
				redirectData := fmt.Sprintf(`{"redirect":true,"reason":"pipeline_restarted","oldPipelineNumber":%d,"newPipelineNumber":%d,"newStepID":%d}`,
					oldPipeline.Number, newPipeline.Number, newStepID)

				err := server.Config.Services.Logs.Write(context.Background(), oldStep.ID, []*model.LogEntry{
					{
						ID:     -1, // Special ID to indicate this is a redirect signal
						StepID: oldStep.ID,
						Time:   time.Now().Unix(),
						Line:   -1,
						Data:   []byte(redirectData),
						Type:   model.LogEntryMetadata,
					},
				})

				if err != nil {
					log.Error().Err(err).Int64("oldStepID", oldStep.ID).Int64("newStepID", newStepID).Msg("failed to send restart redirect signal")
				} else {
					log.Debug().Int64("oldStepID", oldStep.ID).Int64("newStepID", newStepID).Str("stepName", oldStep.Name).Msg("sent restart redirect signal")
				}
			}
		}
	}
}
