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
	"fmt"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/store"
)

type GetMetricsInput struct {
	After  int64  `query:"after" doc:"Unix timestamp - filter metrics after this time"`
	Before int64  `query:"before" doc:"Unix timestamp - filter metrics before this time"`
	All    bool   `query:"all" doc:"Show all instance metrics (admin only)"`
	Repos  string `query:"repos" doc:"Comma-separated list of repo IDs to filter by"`
}

type GetMetricsOutput struct {
	Body *model.Metrics
}

func GetMetrics(ctx context.Context, input *GetMetricsInput) (*GetMetricsOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)

	after := input.After
	before := input.Before

	repoIDs, accessibleRepos, isAdminScope, err := resolveScopedRepoIDs(_store, user, input.All, input.Repos)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting user repos: %s", err))
	}
	showAll := isAdminScope

	// Get basic counts (scoped to accessible repos)
	var repoCount, orgCount, pipelineCount int64

	if showAll && len(repoIDs) == 0 {
		// Admin viewing all instance metrics
		var err error
		repoCount, err = _store.GetRepoCount()
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting repo count: %s", err))
		}

		orgCount, err = _store.GetOrgCount()
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting org count: %s", err))
		}

		pipelineCount, err = _store.GetPipelineCount()
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting pipeline count: %s", err))
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
			return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting pipeline count: %s", err))
		}
		for _, r := range pipelinesByRepoID {
			pipelineCount += r.PipelineCount
		}
	}

	// Get average metrics (with repo filtering)
	avgBuildTime, err := _store.GetAveragePipelineDuration(after, before, repoIDs)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting average build time: %s", err))
	}

	avgStepsPerFlow, err := _store.GetAverageStepsPerWorkflow(after, before, repoIDs)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting average steps per workflow: %s", err))
	}

	// Get grouped metrics (with repo filtering)
	// Use persistent ID for new data, fallback to agent ID for legacy data
	workflowsByPersistentID, err := _store.GetWorkflowCountsByAgentPersistentID(after, before, repoIDs)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting workflows by agent persistent ID: %s", err))
	}
	workflowsByAgentID, err := _store.GetWorkflowCountsByAgent(after, before, repoIDs)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting workflows by agent: %s", err))
	}

	pipelinesByRepoID, err := _store.GetPipelineCountsByRepo(after, before, repoIDs)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting pipelines by repo: %s", err))
	}

	pipelinesByOrgID, err := _store.GetPipelineCountsByOrg(after, before, repoIDs)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting pipelines by org: %s", err))
	}

	// Get build time statistics (with repo filtering)
	repoTimeStats, err := _store.GetBuildTimeStatsByRepo(after, before, repoIDs)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting repo build time stats: %s", err))
	}

	orgTimeStats, err := _store.GetBuildTimeStatsByOrg(after, before, repoIDs)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting org build time stats: %s", err))
	}

	// Convert IDs to names and build detailed metrics
	agentDetails, workflowsByAgent, err := buildAgentMetrics(_store, workflowsByPersistentID, workflowsByAgentID)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error building agent metrics: %s", err))
	}

	repoDetails, pipelinesByRepo := buildRepoMetrics(pipelinesByRepoID, repoTimeStats)

	orgDetails, pipelinesByOrg := buildOrgMetrics(pipelinesByOrgID, orgTimeStats)

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

	return &GetMetricsOutput{Body: metrics}, nil
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

// resolveScopedRepoIDs resolves the set of repo IDs a metrics request may query,
// honoring admin instance-scope and an optional comma-separated repo subset. A
// non-admin is always restricted to repos returned by RepoList; the subset can
// only narrow that set, never widen it. AccessibleRepos is the user's repo list
// (nil for admin instance-scope) so callers can reuse it without a second query.
// IsAdminScope is true only when the caller sees instance-wide data.
func resolveScopedRepoIDs(_store store.Store, user *model.User, all bool, reposCSV string) (repoIDs []int64, accessibleRepos []*model.Repo, isAdminScope bool, err error) {
	showAll := all && user.Admin
	isAdminScope = showAll

	if !showAll {
		repos, err := _store.RepoList(user, false, true)
		if err != nil {
			return nil, nil, false, err
		}
		accessibleRepos = repos
		repoIDs = make([]int64, len(repos))
		for i, repo := range repos {
			repoIDs[i] = repo.ID
		}
	}

	if reposCSV != "" {
		requestedIDs := parseRepoIDs(reposCSV)
		if len(requestedIDs) > 0 {
			if showAll {
				repoIDs = requestedIDs
			} else {
				repoIDs = intersectRepoIDs(repoIDs, requestedIDs)
			}
		}
	}

	return repoIDs, accessibleRepos, isAdminScope, nil
}

// metricQueryCombos lists the metrics supported for each chart-builder dimension.
// Aggregating pipelines or step counts per agent is meaningless (a pipeline is
// not tied to a single agent), so those combinations are rejected.
var metricQueryCombos = map[string]map[string]bool{
	model.MetricDimensionRepo: {
		model.MetricPipelineCount: true, model.MetricWorkflowCount: true,
		model.MetricAvgBuildTime: true, model.MetricTotalBuildTime: true, model.MetricAvgSteps: true,
	},
	model.MetricDimensionOrg: {
		model.MetricPipelineCount: true, model.MetricWorkflowCount: true,
		model.MetricAvgBuildTime: true, model.MetricTotalBuildTime: true, model.MetricAvgSteps: true,
	},
	model.MetricDimensionAgent: {
		model.MetricWorkflowCount: true, model.MetricAvgBuildTime: true, model.MetricTotalBuildTime: true,
	},
	model.MetricDimensionTime: {
		model.MetricPipelineCount: true, model.MetricWorkflowCount: true,
		model.MetricAvgBuildTime: true, model.MetricTotalBuildTime: true, model.MetricAvgSteps: true,
	},
}

// GetMetricsQueryInput is the query for the flexible chart-builder endpoint.
type GetMetricsQueryInput struct {
	Dimension string `query:"dimension" enum:"org,repo,agent,time" required:"true" doc:"Group-by dimension"`
	Metric    string `query:"metric" enum:"pipeline_count,workflow_count,avg_build_time,total_build_time,avg_steps" required:"true" doc:"Metric to aggregate"`
	Bucket    string `query:"bucket" enum:"day,week,month" doc:"Time bucket size (time dimension only, default day)"`
	After     int64  `query:"after" doc:"Unix timestamp - lower time bound"`
	Before    int64  `query:"before" doc:"Unix timestamp - upper time bound"`
	All       bool   `query:"all" doc:"Show all instance metrics (admin only)"`
	Repos     string `query:"repos" doc:"Comma-separated list of repo IDs to filter by"`
	Orgs      string `query:"orgs" doc:"Comma-separated list of org IDs to filter by"`
}

type GetMetricsQueryOutput struct {
	Body *model.MetricSeries
}

// GetMetricsQuery runs a single flexible metrics aggregation for the chart builder.
func GetMetricsQuery(ctx context.Context, input *GetMetricsQueryInput) (*GetMetricsQueryOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)

	if !metricQueryCombos[input.Dimension][input.Metric] {
		return nil, huma.Error422UnprocessableEntity(
			fmt.Sprintf("metric %q is not available for dimension %q", input.Metric, input.Dimension))
	}

	repoIDs, _, isAdminScope, err := resolveScopedRepoIDs(_store, user, input.All, input.Repos)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting user repos: %s", err))
	}

	// A restricted request must resolve to a concrete repo set. An org filter can
	// only narrow it (never widen), and it is intersected after access scoping.
	restricted := !isAdminScope || input.Repos != "" || input.Orgs != ""
	if input.Orgs != "" {
		orgIDs := parseRepoIDs(input.Orgs)
		orgRepoIDs, err := _store.RepoIDsByOrgIDs(orgIDs)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("Error resolving org filter: %s", err))
		}
		if isAdminScope && len(repoIDs) == 0 {
			repoIDs = orgRepoIDs
		} else {
			repoIDs = intersectRepoIDs(repoIDs, orgRepoIDs)
		}
	}

	series := &model.MetricSeries{
		Dimension: input.Dimension,
		Metric:    input.Metric,
		Unit:      model.MetricUnit(input.Metric),
		Points:    []model.MetricPoint{},
	}

	// An empty repo set on a restricted request means "nothing accessible/matched",
	// never "all repos" — return an empty series instead of leaking instance data.
	if restricted && len(repoIDs) == 0 {
		return &GetMetricsQueryOutput{Body: series}, nil
	}

	if input.Dimension == model.MetricDimensionTime {
		series.Bucket = input.Bucket
		if series.Bucket == "" {
			series.Bucket = model.MetricBucketDay
		}
	}

	points, err := _store.GetMetricSeries(model.MetricQueryOptions{
		Dimension: input.Dimension,
		Metric:    input.Metric,
		Bucket:    series.Bucket,
		After:     input.After,
		Before:    input.Before,
		RepoIDs:   repoIDs,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error querying metrics: %s", err))
	}
	series.Points = points

	return &GetMetricsQueryOutput{Body: series}, nil
}

func buildAgentMetrics(_store store.Store, workflowsByPersistentID []model.AgentMetricData, workflowsByAgentID map[int64]int64) ([]model.AgentMetric, map[string]int64, error) {
	agentDetails := make([]model.AgentMetric, 0)
	workflowsByAgent := make(map[string]int64)

	// Track which persistent IDs we've seen to avoid double-counting
	seenPersistentIDs := make(map[string]bool)

	// First, process workflows with persistent IDs (new data)
	// These have agent name stored directly, no need to look up
	for _, data := range workflowsByPersistentID {
		seenPersistentIDs[data.AgentPersistentID] = true

		agentName := data.AgentName
		if agentName == "" {
			agentName = "Unknown"
		}

		agentDetails = append(agentDetails, model.AgentMetric{
			AgentPersistentID: data.AgentPersistentID,
			AgentName:         agentName,
			WorkflowCount:     data.Count,
			AvgBuildTime:      0.0, // Would require additional store method
			Platform:          data.Platform,
			Backend:           "", // Not stored on workflow, would need agent lookup
		})

		workflowsByAgent[agentName] += data.Count
	}

	// Then, process legacy workflows by agent ID (data without persistent ID)
	// These need agent lookup to get names
	agents, err := _store.AgentList(&model.AgentListOptions{})
	if err != nil {
		return nil, nil, err
	}

	agentMap := make(map[int64]*model.Agent)
	for _, agent := range agents {
		agentMap[agent.ID] = agent
	}

	for agentID, count := range workflowsByAgentID {
		agent, exists := agentMap[agentID]

		// If this agent has a persistent ID that we've already counted via
		// the persistent ID query, skip to avoid double-counting
		if exists && agent.PersistentID != "" && seenPersistentIDs[agent.PersistentID] {
			continue
		}

		// Legacy workflows (no persistent ID) whose agent has since been deleted
		// keep a distinct "Agent #<id>" label instead of collapsing every deleted
		// agent into a single "Unknown" bucket. The backfill migration recovers
		// names for agents that still exist.
		agentName := fmt.Sprintf("Agent #%d", agentID)
		platform := ""
		backend := ""
		persistentID := ""

		if exists {
			agentName = agent.Name
			platform = agent.Platform
			backend = agent.Backend
			persistentID = agent.PersistentID
		}

		agentDetails = append(agentDetails, model.AgentMetric{
			AgentID:           agentID,
			AgentPersistentID: persistentID,
			AgentName:         agentName,
			WorkflowCount:     count,
			AvgBuildTime:      0.0,
			Platform:          platform,
			Backend:           backend,
		})

		workflowsByAgent[agentName] += count
	}

	return agentDetails, workflowsByAgent, nil
}

func buildRepoMetrics(pipelinesByRepo []model.RepoMetricData, repoTimeStats map[int64]map[string]float64) ([]model.RepoMetric, map[string]int64) {
	repoDetails := make([]model.RepoMetric, 0, len(pipelinesByRepo))
	pipelinesByRepoName := make(map[string]int64)

	for _, r := range pipelinesByRepo {
		// The full name is denormalized onto the pipeline rows, so it is present
		// even for renamed or deleted repos. Fall back to a synthetic label only
		// if the column was never populated (pre-migration orphaned rows).
		repoFullName := r.RepoFullName
		if repoFullName == "" {
			repoFullName = fmt.Sprintf("Repo #%d", r.RepoID)
		}
		repoName := repoFullName
		if _, name, ok := strings.Cut(repoFullName, "/"); ok && name != "" {
			repoName = name
		}

		// Get build time statistics for this repo
		avgTime := 0.0
		totalTime := 0.0
		if stats, hasStats := repoTimeStats[r.RepoID]; hasStats {
			avgTime = stats["avg"]
			totalTime = stats["total"]
		}

		repoDetails = append(repoDetails, model.RepoMetric{
			RepoID:         r.RepoID,
			RepoName:       repoName,
			RepoFullName:   repoFullName,
			PipelineCount:  r.PipelineCount,
			AvgBuildTime:   avgTime,
			TotalBuildTime: totalTime,
		})

		pipelinesByRepoName[repoFullName] = r.PipelineCount
	}

	return repoDetails, pipelinesByRepoName
}

func buildOrgMetrics(pipelinesByOrg []model.OrgMetricData, orgTimeStats map[int64]map[string]float64) ([]model.OrgMetric, map[string]int64) {
	orgDetails := make([]model.OrgMetric, 0, len(pipelinesByOrg))
	pipelinesByOrgName := make(map[string]int64)

	for _, o := range pipelinesByOrg {
		// The org name is denormalized onto the pipeline rows (from the repo
		// owner), so it is present even for personal-account orgs and deleted
		// orgs. Fall back to a synthetic label only for pre-migration rows.
		orgName := o.OrgName
		if orgName == "" {
			orgName = fmt.Sprintf("Org #%d", o.OrgID)
		}

		// Get build time statistics for this org
		avgTime := 0.0
		totalTime := 0.0
		if stats, hasStats := orgTimeStats[o.OrgID]; hasStats {
			avgTime = stats["avg"]
			totalTime = stats["total"]
		}

		orgDetails = append(orgDetails, model.OrgMetric{
			OrgID:          o.OrgID,
			OrgName:        orgName,
			PipelineCount:  o.PipelineCount,
			RepoCount:      o.RepoCount,
			AvgBuildTime:   avgTime,
			TotalBuildTime: totalTime,
		})

		pipelinesByOrgName[orgName] = o.PipelineCount
	}

	return orgDetails, pipelinesByOrgName
}
