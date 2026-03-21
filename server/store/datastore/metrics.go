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

	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v5/server/model"
)

// GetOrgCount gets a count of all organizations in the system.
func (s storage) GetOrgCount() (int64, error) {
	n, err := s.db.NewSelect().Model((*model.Org)(nil)).Count(context.Background())
	return int64(n), err
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
	return int64(n), err
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
	return int64(n), err
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
		TotalTime   float64 `bun:"total_time"`
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
			"total": r.TotalTime,
		}
	}

	return stats, nil
}

// GetBuildTimeStatsByOrg returns average and total build times grouped by org ID with optional time and repo filter.
func (s storage) GetBuildTimeStatsByOrg(after, before int64, repoIDs []int64) (map[int64]map[string]float64, error) {
	type result struct {
		OrgID       int64   `bun:"org_id"`
		AvgDuration float64 `bun:"avg_duration"`
		TotalTime   float64 `bun:"total_time"`
	}

	ctx := context.Background()

	q := s.db.NewSelect().
		TableExpr("pipelines").
		Join("INNER JOIN repos ON pipelines.repo_id = repos.id").
		ColumnExpr("repos.org_id, AVG(pipelines.finished - pipelines.started) as avg_duration, SUM(pipelines.finished - pipelines.started) as total_time").
		Where("pipelines.finished > 0").
		Where("pipelines.started > 0")

	if after > 0 {
		q = q.Where("pipelines.created >= ?", after)
	}
	if before > 0 {
		q = q.Where("pipelines.created <= ?", before)
	}
	if len(repoIDs) > 0 {
		q = q.Where("pipelines.repo_id IN (?)", bun.List(repoIDs))
	}

	var results []result
	if err := q.GroupExpr("repos.org_id").Scan(ctx, &results); err != nil {
		return nil, err
	}

	stats := make(map[int64]map[string]float64)
	for _, r := range results {
		stats[r.OrgID] = map[string]float64{
			"avg":   r.AvgDuration,
			"total": r.TotalTime,
		}
	}

	return stats, nil
}

// GetPipelineCountsByRepo returns pipeline counts grouped by repo ID with optional time and repo filter.
func (s storage) GetPipelineCountsByRepo(after, before int64, repoIDs []int64) (map[int64]int64, error) {
	type result struct {
		RepoID int64 `bun:"repo_id"`
		Count  int64 `bun:"count"`
	}

	ctx := context.Background()

	q := s.db.NewSelect().
		TableExpr("pipelines").
		ColumnExpr("repo_id, COUNT(*) as count")

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

	counts := make(map[int64]int64)
	for _, r := range results {
		counts[r.RepoID] = r.Count
	}

	return counts, nil
}

// GetPipelineCountsByOrg returns pipeline counts grouped by org ID with optional time and repo filter.
func (s storage) GetPipelineCountsByOrg(after, before int64, repoIDs []int64) (map[int64]int64, error) {
	type result struct {
		OrgID int64 `bun:"org_id"`
		Count int64 `bun:"count"`
	}

	ctx := context.Background()

	q := s.db.NewSelect().
		TableExpr("pipelines").
		Join("INNER JOIN repos ON pipelines.repo_id = repos.id").
		ColumnExpr("repos.org_id, COUNT(*) as count")

	if after > 0 {
		q = q.Where("pipelines.created >= ?", after)
	}
	if before > 0 {
		q = q.Where("pipelines.created <= ?", before)
	}
	if len(repoIDs) > 0 {
		q = q.Where("pipelines.repo_id IN (?)", bun.List(repoIDs))
	}

	var results []result
	if err := q.GroupExpr("repos.org_id").Scan(ctx, &results); err != nil {
		return nil, err
	}

	counts := make(map[int64]int64)
	for _, r := range results {
		counts[r.OrgID] = r.Count
	}

	return counts, nil
}
