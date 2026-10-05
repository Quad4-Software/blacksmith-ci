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

package datastore

import (
	"context"
	"fmt"
	"strconv"

	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v6/server/model"
)

// GetOrgCount gets a count of all organizations in the system.
func (s storage) GetOrgCount() (int64, error) {
	n, err := s.db.NewSelect().Model((*model.Org)(nil)).Count(context.Background())
	return n, err
}

// GetWorkflowCountByAgent gets a count of workflows executed by a specific agent with optional time filter.
func (s storage) GetWorkflowCountByAgent(agentID int64, after, before int64) (int64, error) {
	q := s.db.NewSelect().Model((*model.Workflow)(nil)).Where("agent_id = ?", agentID)

	if after > 0 {
		q = q.Where("started >= ?", after)
	}
	if before > 0 {
		q = q.Where("started <= ?", before)
	}

	n, err := q.Count(context.Background())
	return n, err
}

// GetPipelineCountByOrg gets a count of pipelines for a specific org with optional time filter.
func (s storage) GetPipelineCountByOrg(orgID int64, after, before int64) (int64, error) {
	q := s.db.NewSelect().
		TableExpr("pipelines").
		Join("INNER JOIN repos ON pipelines.repo_id = repos.id").
		Where("repos.org_id = ?", orgID)

	if after > 0 {
		q = q.Where("pipelines.created >= ?", after)
	}
	if before > 0 {
		q = q.Where("pipelines.created <= ?", before)
	}

	n, err := q.Count(context.Background())
	return n, err
}

// GetAveragePipelineDuration gets the average duration of pipelines with optional time and repo filter.
func (s storage) GetAveragePipelineDuration(after, before int64, repoIDs []int64) (float64, error) {
	ctx := context.Background()

	q := s.db.NewSelect().
		TableExpr("pipelines").
		ColumnExpr("AVG(finished - started) as avg_duration").
		Where("finished > 0").
		Where("started > 0")

	if after > 0 {
		q = q.Where("created >= ?", after)
	}
	if before > 0 {
		q = q.Where("created <= ?", before)
	}
	if len(repoIDs) > 0 {
		q = q.Where("repo_id IN (?)", bun.List(repoIDs))
	}

	var avgDuration float64
	err := q.Scan(ctx, &avgDuration)
	return avgDuration, err
}

// GetAverageStepsPerWorkflow gets the average number of steps per workflow with optional time and repo filter.
func (s storage) GetAverageStepsPerWorkflow(after, before int64, repoIDs []int64) (float64, error) {
	ctx := context.Background()

	if after > 0 || before > 0 || len(repoIDs) > 0 {
		// Join with pipelines to filter by time and/or repo
		q := s.db.NewSelect().
			TableExpr("steps").
			Join("INNER JOIN pipelines ON steps.pipeline_id = pipelines.id").
			ColumnExpr("COUNT(steps.id) * 1.0 / COUNT(DISTINCT steps.pipeline_id) as avg_steps")

		if after > 0 {
			q = q.Where("pipelines.created >= ?", after)
		}
		if before > 0 {
			q = q.Where("pipelines.created <= ?", before)
		}
		if len(repoIDs) > 0 {
			q = q.Where("pipelines.repo_id IN (?)", bun.List(repoIDs))
		}

		var avgSteps float64
		err := q.Scan(ctx, &avgSteps)
		return avgSteps, err
	}

	// No filters - simpler query
	var avgSteps float64
	err := s.db.NewSelect().
		TableExpr("steps").
		ColumnExpr("COUNT(id) * 1.0 / COUNT(DISTINCT pipeline_id) as avg_steps").
		Scan(ctx, &avgSteps)
	return avgSteps, err
}

// GetWorkflowCountsByAgent returns workflow counts grouped by agent ID with optional time and repo filter.
func (s storage) GetWorkflowCountsByAgent(after, before int64, repoIDs []int64) (map[int64]int64, error) {
	type result struct {
		AgentID int64 `bun:"agent_id"`
		Count   int64 `bun:"count"`
	}

	ctx := context.Background()
	var results []result

	if len(repoIDs) > 0 {
		q := s.db.NewSelect().
			TableExpr("workflows").
			Join("INNER JOIN pipelines ON workflows.pipeline_id = pipelines.id").
			ColumnExpr("workflows.agent_id, COUNT(*) as count")

		if after > 0 {
			q = q.Where("workflows.started >= ?", after)
		}
		if before > 0 {
			q = q.Where("workflows.started <= ?", before)
		}
		q = q.Where("pipelines.repo_id IN (?)", bun.List(repoIDs))

		if err := q.GroupExpr("workflows.agent_id").Scan(ctx, &results); err != nil {
			return nil, err
		}
	} else {
		q := s.db.NewSelect().
			TableExpr("workflows").
			ColumnExpr("agent_id, COUNT(*) as count")

		if after > 0 {
			q = q.Where("started >= ?", after)
		}
		if before > 0 {
			q = q.Where("started <= ?", before)
		}

		if err := q.GroupExpr("agent_id").Scan(ctx, &results); err != nil {
			return nil, err
		}
	}

	counts := make(map[int64]int64)
	for _, r := range results {
		counts[r.AgentID] = r.Count
	}

	return counts, nil
}

// GetWorkflowCountsByAgentPersistentID returns workflow counts grouped by agent persistent ID with optional time and repo filter.
// This method uses the persistent ID stored on workflows, allowing metrics aggregation even after agent cleanup/ID changes.
// For workflows without a persistent ID (legacy data), it falls back to agent_id grouping.
func (s storage) GetWorkflowCountsByAgentPersistentID(after, before int64, repoIDs []int64) ([]model.AgentMetricData, error) {
	type result struct {
		AgentPersistentID string `bun:"agent_persistent_id"`
		AgentName         string `bun:"agent_name"`
		Platform          string `bun:"platform"`
		Count             int64  `bun:"count"`
	}

	ctx := context.Background()
	var results []result

	if len(repoIDs) > 0 {
		q := s.db.NewSelect().
			TableExpr("workflows").
			Join("INNER JOIN pipelines ON workflows.pipeline_id = pipelines.id").
			ColumnExpr("workflows.agent_persistent_id, MAX(workflows.agent_name) as agent_name, MAX(workflows.platform) as platform, COUNT(*) as count")

		if after > 0 {
			q = q.Where("workflows.started >= ?", after)
		}
		if before > 0 {
			q = q.Where("workflows.started <= ?", before)
		}
		q = q.Where("workflows.agent_persistent_id != ''").
			Where("workflows.agent_persistent_id IS NOT NULL").
			Where("pipelines.repo_id IN (?)", bun.List(repoIDs))

		if err := q.GroupExpr("workflows.agent_persistent_id").Scan(ctx, &results); err != nil {
			return nil, err
		}
	} else {
		q := s.db.NewSelect().
			TableExpr("workflows").
			ColumnExpr("agent_persistent_id, MAX(agent_name) as agent_name, MAX(platform) as platform, COUNT(*) as count")

		if after > 0 {
			q = q.Where("started >= ?", after)
		}
		if before > 0 {
			q = q.Where("started <= ?", before)
		}
		q = q.Where("agent_persistent_id != ''").
			Where("agent_persistent_id IS NOT NULL")

		if err := q.GroupExpr("agent_persistent_id").Scan(ctx, &results); err != nil {
			return nil, err
		}
	}

	data := make([]model.AgentMetricData, 0, len(results))
	for _, r := range results {
		data = append(data, model.AgentMetricData{
			AgentPersistentID: r.AgentPersistentID,
			AgentName:         r.AgentName,
			Platform:          r.Platform,
			Count:             r.Count,
		})
	}

	return data, nil
}

// GetBuildTimeStatsByRepo returns average and total build times grouped by repo ID with optional time and repo filter.
func (s storage) GetBuildTimeStatsByRepo(after, before int64, repoIDs []int64) (map[int64]map[string]float64, error) {
	type result struct {
		RepoID      int64   `bun:"repo_id"`
		AvgDuration float64 `bun:"avg_duration"`
		TotalTime   int64   `bun:"total_time"`
	}

	ctx := context.Background()

	q := s.db.NewSelect().
		TableExpr("pipelines").
		ColumnExpr("repo_id, AVG(finished - started) as avg_duration, SUM(finished - started) as total_time").
		Where("finished > 0").
		Where("started > 0")

	if after > 0 {
		q = q.Where("created >= ?", after)
	}
	if before > 0 {
		q = q.Where("created <= ?", before)
	}
	if len(repoIDs) > 0 {
		q = q.Where("repo_id IN (?)", bun.List(repoIDs))
	}

	var results []result
	if err := q.GroupExpr("repo_id").Scan(ctx, &results); err != nil {
		return nil, err
	}

	stats := make(map[int64]map[string]float64)
	for _, r := range results {
		stats[r.RepoID] = map[string]float64{
			"avg":   r.AvgDuration,
			"total": float64(r.TotalTime),
		}
	}

	return stats, nil
}

// GetBuildTimeStatsByOrg returns average and total build times grouped by org ID with optional time and repo filter.
func (s storage) GetBuildTimeStatsByOrg(after, before int64, repoIDs []int64) (map[int64]map[string]float64, error) {
	type result struct {
		OrgID       int64   `bun:"org_id"`
		AvgDuration float64 `bun:"avg_duration"`
		TotalTime   int64   `bun:"total_time"`
	}

	ctx := context.Background()

	// Grouped by the denormalized org_id on pipelines so stats survive repo/org
	// deletion (no join to repos required).
	q := s.db.NewSelect().
		TableExpr("pipelines").
		ColumnExpr("org_id, AVG(finished - started) as avg_duration, SUM(finished - started) as total_time").
		Where("finished > 0").
		Where("started > 0")

	if after > 0 {
		q = q.Where("created >= ?", after)
	}
	if before > 0 {
		q = q.Where("created <= ?", before)
	}
	if len(repoIDs) > 0 {
		q = q.Where("repo_id IN (?)", bun.List(repoIDs))
	}

	var results []result
	if err := q.GroupExpr("org_id").Scan(ctx, &results); err != nil {
		return nil, err
	}

	stats := make(map[int64]map[string]float64)
	for _, r := range results {
		stats[r.OrgID] = map[string]float64{
			"avg":   r.AvgDuration,
			"total": float64(r.TotalTime),
		}
	}

	return stats, nil
}

// GetPipelineCountsByRepo returns pipeline counts grouped by repo ID with optional time and repo filter.
// The repo full name is carried from the denormalized column so it survives repo deletion.
func (s storage) GetPipelineCountsByRepo(after, before int64, repoIDs []int64) ([]model.RepoMetricData, error) {
	type result struct {
		RepoID       int64  `bun:"repo_id"`
		RepoFullName string `bun:"repo_full_name"`
		Count        int64  `bun:"count"`
	}

	ctx := context.Background()

	q := s.db.NewSelect().
		TableExpr("pipelines").
		ColumnExpr("repo_id, MAX(repo_full_name) as repo_full_name, COUNT(*) as count")

	if after > 0 {
		q = q.Where("created >= ?", after)
	}
	if before > 0 {
		q = q.Where("created <= ?", before)
	}
	if len(repoIDs) > 0 {
		q = q.Where("repo_id IN (?)", bun.List(repoIDs))
	}

	var results []result
	if err := q.GroupExpr("repo_id").Scan(ctx, &results); err != nil {
		return nil, err
	}

	data := make([]model.RepoMetricData, 0, len(results))
	for _, r := range results {
		data = append(data, model.RepoMetricData{
			RepoID:        r.RepoID,
			RepoFullName:  r.RepoFullName,
			PipelineCount: r.Count,
		})
	}

	return data, nil
}

// GetPipelineCountsByOrg returns pipeline counts grouped by org ID with optional time and repo filter.
// The org name and active-repo count are derived from the denormalized pipeline columns so they
// survive org deletion and cover personal-account orgs (which OrgList excludes).
func (s storage) GetPipelineCountsByOrg(after, before int64, repoIDs []int64) ([]model.OrgMetricData, error) {
	type result struct {
		OrgID     int64  `bun:"org_id"`
		OrgName   string `bun:"org_name"`
		Count     int64  `bun:"count"`
		RepoCount int64  `bun:"repo_count"`
	}

	ctx := context.Background()

	q := s.db.NewSelect().
		TableExpr("pipelines").
		ColumnExpr("org_id, MAX(org_name) as org_name, COUNT(*) as count, COUNT(DISTINCT repo_id) as repo_count")

	if after > 0 {
		q = q.Where("created >= ?", after)
	}
	if before > 0 {
		q = q.Where("created <= ?", before)
	}
	if len(repoIDs) > 0 {
		q = q.Where("repo_id IN (?)", bun.List(repoIDs))
	}

	var results []result
	if err := q.GroupExpr("org_id").Scan(ctx, &results); err != nil {
		return nil, err
	}

	data := make([]model.OrgMetricData, 0, len(results))
	for _, r := range results {
		data = append(data, model.OrgMetricData{
			OrgID:         r.OrgID,
			OrgName:       r.OrgName,
			PipelineCount: r.Count,
			RepoCount:     r.RepoCount,
		})
	}

	return data, nil
}

// GetMetricSeries runs a flexible chart-builder aggregation: one metric grouped
// by one dimension, optionally filtered by time window and repo set. The caller
// (API layer) is responsible for validating the dimension/metric combination
// and for access-scoping RepoIDs before calling this.
func (s storage) GetMetricSeries(opts model.MetricQueryOptions) ([]model.MetricPoint, error) {
	ctx := context.Background()

	type seriesRow struct {
		ID    int64   `bun:"id"`
		Key   string  `bun:"grp_key"`
		Label string  `bun:"label"`
		Value float64 `bun:"value"`
	}

	agentDim := opts.Dimension == model.MetricDimensionAgent
	// Agent identity lives on workflows; workflow_count also counts workflow rows.
	workflowBased := agentDim || opts.Metric == model.MetricWorkflowCount

	q := s.db.NewSelect()

	var timeCol, startedCol, finishedCol string
	switch {
	case workflowBased:
		q = q.TableExpr("workflows")
		// Join pipelines when we need repo/org columns (repo/org grouping) or a
		// repo-scope filter. Agent/time grouping over workflow rows needs no join.
		needPipelines := opts.Dimension == model.MetricDimensionRepo ||
			opts.Dimension == model.MetricDimensionOrg ||
			len(opts.RepoIDs) > 0
		if needPipelines {
			q = q.Join("INNER JOIN pipelines ON workflows.pipeline_id = pipelines.id")
		}
		timeCol, startedCol, finishedCol = "workflows.started", "workflows.started", "workflows.finished"
	case opts.Metric == model.MetricAvgSteps:
		q = q.TableExpr("steps").Join("INNER JOIN pipelines ON steps.pipeline_id = pipelines.id")
		timeCol = "pipelines.created"
	default:
		q = q.TableExpr("pipelines")
		timeCol, startedCol, finishedCol = "pipelines.created", "pipelines.started", "pipelines.finished"
	}

	// Aggregate expression for the metric.
	var aggExpr string
	switch opts.Metric {
	case model.MetricPipelineCount, model.MetricWorkflowCount:
		aggExpr = "COUNT(*)"
	case model.MetricAvgBuildTime:
		aggExpr = fmt.Sprintf("AVG(%s - %s)", finishedCol, startedCol)
	case model.MetricTotalBuildTime:
		aggExpr = fmt.Sprintf("SUM(%s - %s)", finishedCol, startedCol)
	case model.MetricAvgSteps:
		aggExpr = "COUNT(steps.id) * 1.0 / COUNT(DISTINCT steps.pipeline_id)"
	default:
		return nil, fmt.Errorf("unsupported metric %q", opts.Metric)
	}
	// Force a floating-point result so COUNT/SUM (integer) scan into float64.
	valueExpr := "(" + aggExpr + ") * 1.0"

	// Only completed runs contribute to build-time metrics.
	if opts.Metric == model.MetricAvgBuildTime || opts.Metric == model.MetricTotalBuildTime {
		q = q.Where(finishedCol + " > 0").Where(startedCol + " > 0")
	}

	// Grouping / labels per dimension.
	switch opts.Dimension {
	case model.MetricDimensionRepo:
		q = q.ColumnExpr("pipelines.repo_id AS id, MAX(COALESCE(pipelines.repo_full_name, '')) AS label, " + valueExpr + " AS value").
			GroupExpr("pipelines.repo_id")
	case model.MetricDimensionOrg:
		q = q.ColumnExpr("pipelines.org_id AS id, MAX(COALESCE(pipelines.org_name, '')) AS label, " + valueExpr + " AS value").
			GroupExpr("pipelines.org_id")
	case model.MetricDimensionAgent:
		q = q.ColumnExpr("workflows.agent_persistent_id AS grp_key, MAX(COALESCE(workflows.agent_name, '')) AS label, " + valueExpr + " AS value").
			Where("workflows.agent_persistent_id != ''").
			Where("workflows.agent_persistent_id IS NOT NULL").
			GroupExpr("workflows.agent_persistent_id")
	case model.MetricDimensionTime:
		bucket := s.bucketExpr(timeCol, opts.Bucket)
		q = q.ColumnExpr(bucket + " AS grp_key, " + valueExpr + " AS value").
			GroupExpr(bucket).
			OrderExpr(bucket)
	default:
		return nil, fmt.Errorf("unsupported dimension %q", opts.Dimension)
	}

	if opts.After > 0 {
		q = q.Where(timeCol+" >= ?", opts.After)
	}
	if opts.Before > 0 {
		q = q.Where(timeCol+" <= ?", opts.Before)
	}
	if len(opts.RepoIDs) > 0 {
		q = q.Where("pipelines.repo_id IN (?)", bun.List(opts.RepoIDs))
	}

	var rows []seriesRow
	if err := q.Scan(ctx, &rows); err != nil {
		return nil, err
	}

	points := make([]model.MetricPoint, 0, len(rows))
	for _, r := range rows {
		p := model.MetricPoint{ID: r.ID, Label: r.Label, Value: r.Value}
		switch opts.Dimension {
		case model.MetricDimensionRepo:
			p.Key = strconv.FormatInt(r.ID, 10)
			if p.Label == "" {
				p.Label = fmt.Sprintf("Repo #%d", r.ID)
			}
		case model.MetricDimensionOrg:
			p.Key = strconv.FormatInt(r.ID, 10)
			if p.Label == "" {
				p.Label = fmt.Sprintf("Org #%d", r.ID)
			}
		case model.MetricDimensionAgent:
			p.Key = r.Key
			if p.Label == "" {
				p.Label = r.Key
			}
		case model.MetricDimensionTime:
			p.Key = r.Key
			p.Label = r.Key
		}
		points = append(points, p)
	}

	return points, nil
}

// RepoIDsByOrgIDs returns the IDs of existing repos owned by the given orgs.
// Used to translate a chart-builder org filter into the repo-scoped filter that
// enforces access control uniformly across the metrics endpoints.
func (s storage) RepoIDsByOrgIDs(orgIDs []int64) ([]int64, error) {
	if len(orgIDs) == 0 {
		return nil, nil
	}
	var ids []int64
	err := s.db.NewSelect().
		TableExpr("repos").
		ColumnExpr("id").
		Where("org_id IN (?)", bun.List(orgIDs)).
		Scan(context.Background(), &ids)
	return ids, err
}
