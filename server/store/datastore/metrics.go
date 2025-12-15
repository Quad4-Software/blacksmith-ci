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
	"xorm.io/builder"

	"codeberg.org/crowci/crow/v5/server/model"
)

// GetOrgCount gets a count of all organizations in the system.
func (s storage) GetOrgCount() (int64, error) {
	return s.engine.Count(new(model.Org))
}

// GetWorkflowCountByAgent gets a count of workflows executed by a specific agent with optional time filter.
func (s storage) GetWorkflowCountByAgent(agentID int64, after, before int64) (int64, error) {
	cond := builder.NewCond().And(builder.Eq{"agent_id": agentID})

	if after > 0 {
		cond = cond.And(builder.Gte{"started": after})
	}
	if before > 0 {
		cond = cond.And(builder.Lte{"started": before})
	}

	return s.engine.Where(cond).Count(new(model.Workflow))
}

// GetPipelineCountByOrg gets a count of pipelines for a specific org with optional time filter.
func (s storage) GetPipelineCountByOrg(orgID int64, after, before int64) (int64, error) {
	cond := builder.NewCond()

	if after > 0 {
		cond = cond.And(builder.Gte{"pipelines.created": after})
	}
	if before > 0 {
		cond = cond.And(builder.Lte{"pipelines.created": before})
	}

	return s.engine.
		Table("pipelines").
		Join("INNER", "repos", "pipelines.repo_id = repos.id").
		Where("repos.org_id = ?", orgID).
		And(cond).
		Count(new(model.Pipeline))
}

// GetAveragePipelineDuration gets the average duration of pipelines with optional time and repo filter.
func (s storage) GetAveragePipelineDuration(after, before int64, repoIDs []int64) (float64, error) {
	type result struct {
		AvgDuration float64 `xorm:"avg_duration"`
	}

	cond := builder.NewCond().
		And(builder.Gt{"finished": 0}).
		And(builder.Gt{"started": 0})

	if after > 0 {
		cond = cond.And(builder.Gte{"created": after})
	}
	if before > 0 {
		cond = cond.And(builder.Lte{"created": before})
	}
	if len(repoIDs) > 0 {
		cond = cond.And(builder.In("repo_id", repoIDs))
	}

	var res result
	_, err := s.engine.
		Table("pipelines").
		Where(cond).
		Select("AVG(finished - started) as avg_duration").
		Get(&res)

	return res.AvgDuration, err
}

// GetAverageStepsPerWorkflow gets the average number of steps per workflow with optional time and repo filter.
func (s storage) GetAverageStepsPerWorkflow(after, before int64, repoIDs []int64) (float64, error) {
	type result struct {
		AvgSteps float64 `xorm:"avg_steps"`
	}

	if after > 0 || before > 0 || len(repoIDs) > 0 {
		// Join with pipelines to filter by time and/or repo
		pipelineCond := builder.NewCond()
		if after > 0 {
			pipelineCond = pipelineCond.And(builder.Gte{"pipelines.created": after})
		}
		if before > 0 {
			pipelineCond = pipelineCond.And(builder.Lte{"pipelines.created": before})
		}
		if len(repoIDs) > 0 {
			pipelineCond = pipelineCond.And(builder.In("pipelines.repo_id", repoIDs))
		}

		var res result
		_, err := s.engine.
			Table("steps").
			Join("INNER", "pipelines", "steps.pipeline_id = pipelines.id").
			Where(pipelineCond).
			Select("COUNT(steps.id) * 1.0 / COUNT(DISTINCT steps.pipeline_id) as avg_steps").
			Get(&res)

		return res.AvgSteps, err
	}

	// No filters - simpler query
	var res result
	_, err := s.engine.
		Table("steps").
		Select("COUNT(id) * 1.0 / COUNT(DISTINCT pipeline_id) as avg_steps").
		Get(&res)

	return res.AvgSteps, err
}

// GetWorkflowCountsByAgent returns workflow counts grouped by agent ID with optional time and repo filter.
func (s storage) GetWorkflowCountsByAgent(after, before int64, repoIDs []int64) (map[int64]int64, error) {
	type result struct {
		AgentID int64 `xorm:"agent_id"`
		Count   int64 `xorm:"count"`
	}

	cond := builder.NewCond()

	if after > 0 {
		cond = cond.And(builder.Gte{"workflows.started": after})
	}
	if before > 0 {
		cond = cond.And(builder.Lte{"workflows.started": before})
	}

	var results []result
	var err error

	if len(repoIDs) > 0 {
		// Need to join with pipelines to filter by repo
		cond = cond.And(builder.In("pipelines.repo_id", repoIDs))
		err = s.engine.
			Table("workflows").
			Join("INNER", "pipelines", "workflows.pipeline_id = pipelines.id").
			Where(cond).
			Select("workflows.agent_id, COUNT(*) as count").
			GroupBy("workflows.agent_id").
			Find(&results)
	} else {
		err = s.engine.
			Table("workflows").
			Where(cond).
			Select("agent_id, COUNT(*) as count").
			GroupBy("agent_id").
			Find(&results)
	}
	if err != nil {
		return nil, err
	}

	counts := make(map[int64]int64)
	for _, r := range results {
		counts[r.AgentID] = r.Count
	}

	return counts, nil
}

// GetBuildTimeStatsByRepo returns average and total build times grouped by repo ID with optional time and repo filter.
func (s storage) GetBuildTimeStatsByRepo(after, before int64, repoIDs []int64) (map[int64]map[string]float64, error) {
	type result struct {
		RepoID      int64   `xorm:"repo_id"`
		AvgDuration float64 `xorm:"avg_duration"`
		TotalTime   float64 `xorm:"total_time"`
	}

	cond := builder.NewCond().
		And(builder.Gt{"finished": 0}).
		And(builder.Gt{"started": 0})

	if after > 0 {
		cond = cond.And(builder.Gte{"created": after})
	}
	if before > 0 {
		cond = cond.And(builder.Lte{"created": before})
	}
	if len(repoIDs) > 0 {
		cond = cond.And(builder.In("repo_id", repoIDs))
	}

	var results []result
	err := s.engine.
		Table("pipelines").
		Where(cond).
		Select("repo_id, AVG(finished - started) as avg_duration, SUM(finished - started) as total_time").
		GroupBy("repo_id").
		Find(&results)
	if err != nil {
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
		OrgID       int64   `xorm:"org_id"`
		AvgDuration float64 `xorm:"avg_duration"`
		TotalTime   float64 `xorm:"total_time"`
	}

	cond := builder.NewCond().
		And(builder.Gt{"pipelines.finished": 0}).
		And(builder.Gt{"pipelines.started": 0})

	if after > 0 {
		cond = cond.And(builder.Gte{"pipelines.created": after})
	}
	if before > 0 {
		cond = cond.And(builder.Lte{"pipelines.created": before})
	}
	if len(repoIDs) > 0 {
		cond = cond.And(builder.In("pipelines.repo_id", repoIDs))
	}

	var results []result
	err := s.engine.
		Table("pipelines").
		Join("INNER", "repos", "pipelines.repo_id = repos.id").
		Where(cond).
		Select("repos.org_id, AVG(pipelines.finished - pipelines.started) as avg_duration, SUM(pipelines.finished - pipelines.started) as total_time").
		GroupBy("repos.org_id").
		Find(&results)
	if err != nil {
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
		RepoID int64 `xorm:"repo_id"`
		Count  int64 `xorm:"count"`
	}

	cond := builder.NewCond()

	if after > 0 {
		cond = cond.And(builder.Gte{"created": after})
	}
	if before > 0 {
		cond = cond.And(builder.Lte{"created": before})
	}
	if len(repoIDs) > 0 {
		cond = cond.And(builder.In("repo_id", repoIDs))
	}

	var results []result
	err := s.engine.
		Table("pipelines").
		Where(cond).
		Select("repo_id, COUNT(*) as count").
		GroupBy("repo_id").
		Find(&results)
	if err != nil {
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
		OrgID int64 `xorm:"org_id"`
		Count int64 `xorm:"count"`
	}

	cond := builder.NewCond()

	if after > 0 {
		cond = cond.And(builder.Gte{"pipelines.created": after})
	}
	if before > 0 {
		cond = cond.And(builder.Lte{"pipelines.created": before})
	}
	if len(repoIDs) > 0 {
		cond = cond.And(builder.In("pipelines.repo_id", repoIDs))
	}

	var results []result
	err := s.engine.
		Table("pipelines").
		Join("INNER", "repos", "pipelines.repo_id = repos.id").
		Where(cond).
		Select("repos.org_id, COUNT(*) as count").
		GroupBy("repos.org_id").
		Find(&results)
	if err != nil {
		return nil, err
	}

	counts := make(map[int64]int64)
	for _, r := range results {
		counts[r.OrgID] = r.Count
	}

	return counts, nil
}
