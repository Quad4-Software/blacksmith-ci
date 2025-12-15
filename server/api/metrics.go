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
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/router/middleware/session"
	"codeberg.org/crowci/crow/v5/server/store"
)

// GetMetrics
//
//	@Summary		Get metrics
//	@Description	Returns comprehensive metrics including builds per agent, repo, org, average build time, etc. Non-admins see only their accessible repos.
//	@Router			/metrics [get]
//	@Produce		json
//	@Success		200	{object}	model.Metrics
//	@Tags			Metrics
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			after			query	int		false	"Unix timestamp - filter metrics after this time"
//	@Param			before			query	int		false	"Unix timestamp - filter metrics before this time"
//	@Param			all				query	bool	false	"Show all instance metrics (admin only)"
//	@Param			repos			query	string	false	"Comma-separated list of repo IDs to filter by"
func GetMetrics(c *gin.Context) {
	_store := store.FromContext(c)
	user := session.User(c)

	// Parse time filters from query parameters
	var after, before int64
	if afterStr := c.Query("after"); afterStr != "" {
		if val, err := strconv.ParseInt(afterStr, 10, 64); err == nil {
			after = val
		}
	}
	if beforeStr := c.Query("before"); beforeStr != "" {
		if val, err := strconv.ParseInt(beforeStr, 10, 64); err == nil {
			before = val
		}
	}

	// Determine scope: admin can see all, non-admin sees only their repos
	showAll := c.Query("all") == "true" && user.Admin
	isAdminScope := showAll

	// Get user's accessible repos
	var repoIDs []int64
	var accessibleRepos []*model.Repo

	if !showAll {
		// Get repos user has access to (via perms table)
		repos, err := _store.RepoList(user, false, true)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error getting user repos: %s", err)
			return
		}
		accessibleRepos = repos
		repoIDs = make([]int64, len(repos))
		for i, repo := range repos {
			repoIDs[i] = repo.ID
		}
	}

	// Apply additional repo subset filter if specified
	if repoSubsetStr := c.Query("repos"); repoSubsetStr != "" {
		requestedIDs := parseRepoIDs(repoSubsetStr)
		if len(requestedIDs) > 0 {
			if showAll {
				// Admin viewing all repos but wants subset
				repoIDs = requestedIDs
			} else {
				// Non-admin: intersect with accessible repos
				repoIDs = intersectRepoIDs(repoIDs, requestedIDs)
			}
		}
	}

	// Get basic counts (scoped to accessible repos)
	var repoCount, orgCount, pipelineCount int64

	if showAll && len(repoIDs) == 0 {
		// Admin viewing all instance metrics
		var err error
		repoCount, err = _store.GetRepoCount()
		if err != nil {
			c.String(http.StatusInternalServerError, "Error getting repo count: %s", err)
			return
		}

		orgCount, err = _store.GetOrgCount()
		if err != nil {
			c.String(http.StatusInternalServerError, "Error getting org count: %s", err)
			return
		}

		pipelineCount, err = _store.GetPipelineCount()
		if err != nil {
			c.String(http.StatusInternalServerError, "Error getting pipeline count: %s", err)
			return
		}
	} else {
		// Scoped metrics
		repoCount = int64(len(repoIDs))

		// Count unique orgs from accessible repos
		orgSet := make(map[int64]bool)
		if len(accessibleRepos) > 0 {
			for _, repo := range accessibleRepos {
				if repo.OrgID > 0 {
					orgSet[repo.OrgID] = true
				}
			}
		} else if len(repoIDs) > 0 {
			// Need to fetch repos for org counting
			for _, repoID := range repoIDs {
				repo, err := _store.GetRepo(repoID)
				if err == nil && repo.OrgID > 0 {
					orgSet[repo.OrgID] = true
				}
			}
		}
		orgCount = int64(len(orgSet))

		// Count pipelines for accessible repos
		pipelinesByRepoID, err := _store.GetPipelineCountsByRepo(0, 0, repoIDs)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error getting pipeline count: %s", err)
			return
		}
		for _, count := range pipelinesByRepoID {
			pipelineCount += count
		}
	}

	// Get average metrics (with repo filtering)
	avgBuildTime, err := _store.GetAveragePipelineDuration(after, before, repoIDs)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting average build time: %s", err)
		return
	}

	avgStepsPerFlow, err := _store.GetAverageStepsPerWorkflow(after, before, repoIDs)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting average steps per workflow: %s", err)
		return
	}

	// Get grouped metrics (with repo filtering)
	workflowsByAgentID, err := _store.GetWorkflowCountsByAgent(after, before, repoIDs)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting workflows by agent: %s", err)
		return
	}

	pipelinesByRepoID, err := _store.GetPipelineCountsByRepo(after, before, repoIDs)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting pipelines by repo: %s", err)
		return
	}

	pipelinesByOrgID, err := _store.GetPipelineCountsByOrg(after, before, repoIDs)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting pipelines by org: %s", err)
		return
	}

	// Get build time statistics (with repo filtering)
	repoTimeStats, err := _store.GetBuildTimeStatsByRepo(after, before, repoIDs)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting repo build time stats: %s", err)
		return
	}

	orgTimeStats, err := _store.GetBuildTimeStatsByOrg(after, before, repoIDs)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting org build time stats: %s", err)
		return
	}

	// Convert IDs to names and build detailed metrics
	agentDetails, workflowsByAgent, err := buildAgentMetrics(_store, workflowsByAgentID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error building agent metrics: %s", err)
		return
	}

	repoDetails, pipelinesByRepo, err := buildRepoMetrics(_store, pipelinesByRepoID, repoTimeStats)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error building repo metrics: %s", err)
		return
	}

	orgDetails, pipelinesByOrg, err := buildOrgMetrics(_store, pipelinesByOrgID, orgTimeStats)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error building org metrics: %s", err)
		return
	}

	metrics := &model.Metrics{
		TotalRepos:       repoCount,
		TotalOrgs:        orgCount,
		TotalPipelines:   pipelineCount,
		AvgBuildTime:     avgBuildTime,
		AvgStepsPerFlow:  avgStepsPerFlow,
		PipelinesByRepo:  pipelinesByRepo,
		PipelinesByOrg:   pipelinesByOrg,
		WorkflowsByAgent: workflowsByAgent,
		RepoDetails:      repoDetails,
		OrgDetails:       orgDetails,
		AgentDetails:     agentDetails,
		IsAdminScope:     isAdminScope,
	}

	c.JSON(http.StatusOK, metrics)
}

// parseRepoIDs parses a comma-separated string of repo IDs.
func parseRepoIDs(s string) []int64 {
	var ids []int64
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if id, err := strconv.ParseInt(part, 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

// intersectRepoIDs returns the intersection of two repo ID slices.
func intersectRepoIDs(a, b []int64) []int64 {
	set := make(map[int64]bool)
	for _, id := range a {
		set[id] = true
	}

	var result []int64
	for _, id := range b {
		if set[id] {
			result = append(result, id)
		}
	}
	return result
}

func buildAgentMetrics(_store store.Store, workflowsByAgentID map[int64]int64) ([]model.AgentMetric, map[string]int64, error) {
	agents, err := _store.AgentList(&model.AgentListOptions{})
	if err != nil {
		return nil, nil, err
	}

	agentMap := make(map[int64]*model.Agent)
	for _, agent := range agents {
		agentMap[agent.ID] = agent
	}

	agentDetails := make([]model.AgentMetric, 0)
	workflowsByAgent := make(map[string]int64)

	for agentID, count := range workflowsByAgentID {
		agent, exists := agentMap[agentID]
		agentName := "Unknown"
		platform := ""
		backend := ""

		if exists {
			agentName = agent.Name
			platform = agent.Platform
			backend = agent.Backend
		}

		// Calculate average build time for this agent
		avgTime := 0.0
		// Note: This would require additional store methods to calculate per-agent avg build time
		// For now, we'll leave it as 0.0

		agentDetails = append(agentDetails, model.AgentMetric{
			AgentID:       agentID,
			AgentName:     agentName,
			WorkflowCount: count,
			AvgBuildTime:  avgTime,
			Platform:      platform,
			Backend:       backend,
		})

		workflowsByAgent[agentName] = count
	}

	return agentDetails, workflowsByAgent, nil
}

func buildRepoMetrics(_store store.Store, pipelinesByRepoID map[int64]int64, repoTimeStats map[int64]map[string]float64) ([]model.RepoMetric, map[string]int64, error) {
	// Get all repos for lookup (including inactive ones to ensure we have names for all pipelines)
	repos, err := _store.RepoListAll(false, &model.ListOptions{All: true})
	if err != nil {
		return nil, nil, err
	}

	repoMap := make(map[int64]*model.Repo)
	for _, repo := range repos {
		repoMap[repo.ID] = repo
	}

	repoDetails := make([]model.RepoMetric, 0)
	pipelinesByRepo := make(map[string]int64)

	for repoID, count := range pipelinesByRepoID {
		repo, exists := repoMap[repoID]
		repoName := "Unknown"
		repoFullName := ""

		if exists {
			repoName = repo.Name
			repoFullName = repo.FullName
		} else {
			// Fallback: try to fetch the specific repo (might be deleted but still have pipelines)
			if specificRepo, err := _store.GetRepo(repoID); err == nil && specificRepo != nil {
				repoName = specificRepo.Name
				repoFullName = specificRepo.FullName
			}
		}

		// Ensure we always have a displayable name
		if repoFullName == "" {
			if repoName != "" && repoName != "Unknown" {
				repoFullName = repoName
			} else {
				repoFullName = fmt.Sprintf("Repo #%d", repoID)
			}
		}

		// Get build time statistics for this repo
		avgTime := 0.0
		totalTime := 0.0
		if stats, hasStats := repoTimeStats[repoID]; hasStats {
			avgTime = stats["avg"]
			totalTime = stats["total"]
		}

		repoDetails = append(repoDetails, model.RepoMetric{
			RepoID:         repoID,
			RepoName:       repoName,
			RepoFullName:   repoFullName,
			PipelineCount:  count,
			AvgBuildTime:   avgTime,
			TotalBuildTime: totalTime,
		})

		pipelinesByRepo[repoFullName] = count
	}

	return repoDetails, pipelinesByRepo, nil
}

func buildOrgMetrics(_store store.Store, pipelinesByOrgID map[int64]int64, orgTimeStats map[int64]map[string]float64) ([]model.OrgMetric, map[string]int64, error) {
	// Get all orgs for lookup
	orgs, err := _store.OrgList(&model.ListOptions{All: true})
	if err != nil {
		return nil, nil, err
	}

	orgMap := make(map[int64]*model.Org)
	for _, org := range orgs {
		orgMap[org.ID] = org
	}

	orgDetails := make([]model.OrgMetric, 0)
	pipelinesByOrg := make(map[string]int64)

	for orgID, count := range pipelinesByOrgID {
		org, exists := orgMap[orgID]
		orgName := "Unknown"

		if exists {
			orgName = org.Name
		}

		// Get repo count for this org
		repoCount := int64(0)
		if exists {
			repos, err := _store.OrgRepoList(org, &model.ListOptions{All: true})
			if err == nil {
				repoCount = int64(len(repos))
			}
		}

		// Get build time statistics for this org
		avgTime := 0.0
		totalTime := 0.0
		if stats, hasStats := orgTimeStats[orgID]; hasStats {
			avgTime = stats["avg"]
			totalTime = stats["total"]
		}

		orgDetails = append(orgDetails, model.OrgMetric{
			OrgID:          orgID,
			OrgName:        orgName,
			PipelineCount:  count,
			RepoCount:      repoCount,
			AvgBuildTime:   avgTime,
			TotalBuildTime: totalTime,
		})

		pipelinesByOrg[orgName] = count
	}

	return orgDetails, pipelinesByOrg, nil
}
