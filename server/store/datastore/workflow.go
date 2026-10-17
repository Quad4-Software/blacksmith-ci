// Copyright 2023 Woodpecker Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datastore

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/matrix"
	"codefloe.com/crowci/crow/v6/server/model"
)

func (s storage) WorkflowGetTree(pipeline *model.Pipeline) ([]*model.Workflow, error) {
	ctx := context.Background()
	wfList, err := s.workflowList(ctx, s.db, pipeline)
	if err != nil {
		return nil, err
	}

	// Get pipeline configurations to populate pipeline_name field for existing workflows
	var pipelineNameMap map[string]string
	if len(wfList) > 0 {
		// Check if any workflows are missing pipeline_name
		needsPopulation := false
		for _, wf := range wfList {
			if wf.PipelineName == "" {
				needsPopulation = true
				break
			}
		}

		if needsPopulation {
			pipelineNameMap, err = s.getPipelineNameMap(pipeline)
			if err != nil {
				log.Warn().Err(err).Int64("pipeline_id", pipeline.ID).Msg("Failed to get pipeline config for pipeline_name population")
				// For existing workflows without pipeline_name, use a fallback approach
				pipelineNameMap = s.createFallbackPipelineNameMap(wfList)
			}
		}
	}

	for _, wf := range wfList {
		wf.Children, err = s.stepListWorkflow(ctx, s.db, wf)
		if err != nil {
			return nil, err
		}

		// Populate pipeline_name field for existing workflows that don't have it set
		if wf.PipelineName == "" && pipelineNameMap != nil {
			// Try to find the pipeline name from the config map
			if pipelineName, found := pipelineNameMap[wf.Name]; found {
				wf.PipelineName = pipelineName
			} else {
				// If not found, use the workflow name as fallback
				wf.PipelineName = wf.Name
			}
		}
	}

	return wfList, nil
}

// getPipelineNameMap creates a map from workflow names to their pipeline names
// by parsing the original YAML configurations.
func (s storage) getPipelineNameMap(pipeline *model.Pipeline) (map[string]string, error) {
	pipelineNameMap := make(map[string]string)

	// Get all configs for this pipeline
	configs, err := s.ConfigsForPipeline(pipeline.ID)
	if err != nil {
		return nil, err
	}

	for _, config := range configs {
		// Parse the YAML config to extract the pipeline name
		parsed, err := yaml.ParseBytes(config.Data)
		if err != nil {
			log.Warn().Err(err).Str("config_name", config.Name).Msg("Failed to parse config YAML")
			continue
		}

		// Determine the pipeline name (from YAML name field or filename)
		var pipelineName string
		if parsed.Name != "" {
			pipelineName = parsed.Name
		} else {
			// Fallback to sanitized filename
			pipelineName = sanitizePath(config.Name)
		}

		// Parse matrix from YAML data
		axes, err := matrix.ParseString(string(config.Data))
		if err != nil {
			log.Warn().Err(err).Str("config_name", config.Name).Msg("Failed to parse matrix from config YAML")
			continue
		}

		// If no matrix axes, create a default empty axis
		if len(axes) == 0 {
			axes = append(axes, matrix.Axis{})
		}

		// Map each matrix axis to the pipeline name
		for _, axis := range axes {
			// Check for matrix NAME (case-insensitive) - same logic as stepBuilder.go
			matrixName, nameKey, hasMatrixName := getCaseInsensitiveKey(axis, "NAME")
			if hasMatrixName {
				delete(axis, nameKey) // Remove NAME from axis
			}

			// Determine workflow name using the same logic as stepBuilder.go
			var workflowName string
			switch {
			case hasMatrixName && matrixName != "":
				workflowName = matrixName
			case parsed.Name != "":
				workflowName = parsed.Name
			default:
				workflowName = sanitizePath(config.Name)
			}

			pipelineNameMap[workflowName] = pipelineName
		}
	}

	return pipelineNameMap, nil
}

// createFallbackPipelineNameMap creates a fallback pipeline name map when configs are not available
// This helps handle cases where the pipeline_configs table doesn't exist or configurations are missing.
func (s storage) createFallbackPipelineNameMap(workflows []*model.Workflow) map[string]string {
	pipelineNameMap := make(map[string]string)

	// Without access to the original YAML configs, we can't determine the true pipeline names
	// The best we can do is use the workflow names themselves
	// This preserves the existing behavior for legacy workflows
	for _, wf := range workflows {
		pipelineNameMap[wf.Name] = wf.Name
	}

	return pipelineNameMap
}

// getCaseInsensitiveKey returns the value, the actual key, and true if the key exists in the map (case-insensitive).
func getCaseInsensitiveKey(m map[string]string, key string) (string, string, bool) {
	for k, v := range m {
		if equalFold(k, key) {
			return v, k, true
		}
	}
	return "", "", false
}

// equalFold is a simple ASCII case-insensitive comparison.
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		switch {
		case (a[i] >= 'A' && a[i] <= 'Z') && (b[i] >= 'a' && b[i] <= 'z'):
			if a[i]+32 != b[i] {
				return false
			}
		case (a[i] >= 'a' && a[i] <= 'z') && (b[i] >= 'A' && b[i] <= 'Z'):
			if a[i] != b[i]+32 {
				return false
			}
		default:
			if a[i] != b[i] {
				return false
			}
		}
	}
	return true
}

// sanitizePath removes the file extension and leading dots from a path.
func sanitizePath(path string) string {
	path = filepath.Base(path)
	path = strings.TrimSuffix(path, ".yml")
	path = strings.TrimSuffix(path, ".yaml")
	path = strings.TrimPrefix(path, ".")
	return path
}

func (s storage) WorkflowsCreate(workflows []*model.Workflow) error {
	return s.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		return s.workflowsCreate(ctx, tx, workflows)
	})
}

func (s storage) workflowsCreate(ctx context.Context, db bun.IDB, workflows []*model.Workflow) error {
	for i := range workflows {
		// only Insert on single object ref set auto created ID back to object
		if err := s.stepCreate(ctx, db, workflows[i].Children); err != nil {
			return err
		}
		if _, err := db.NewInsert().Model(workflows[i]).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

// WorkflowsReplace performs an atomic replacement of workflows and associated steps by deleting all existing workflows and steps and inserting the new ones.
func (s storage) WorkflowsReplace(pipeline *model.Pipeline, workflows []*model.Workflow) error {
	return s.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		if err := s.workflowsDelete(ctx, tx, pipeline.ID); err != nil {
			return err
		}

		// Delete tasks associated with this pipeline to prevent orphaned tasks referencing deleted steps
		if _, err := tx.NewDelete().Model((*model.Task)(nil)).Where("pipeline_id = ?", pipeline.ID).Exec(ctx); err != nil {
			return err
		}

		return s.workflowsCreate(ctx, tx, workflows)
	})
}

func (s storage) workflowsDelete(ctx context.Context, db bun.IDB, pipelineID int64) error {
	// delete related steps in batches; each batch is deleted before the next
	// select, so always read from the start instead of advancing an offset
	for {
		stepIDs := make([]int64, 0, perPage)
		err := db.NewSelect().
			TableExpr("steps").
			ColumnExpr("id").
			Where("pipeline_id = ?", pipelineID).
			Limit(perPage).
			Scan(ctx, &stepIDs)
		if err != nil {
			return err
		}
		if len(stepIDs) == 0 {
			break
		}

		for i := range stepIDs {
			if err := deleteStep(ctx, db, stepIDs[i]); err != nil {
				return err
			}
		}
	}

	_, err := db.NewDelete().Model((*model.Workflow)(nil)).Where("pipeline_id = ?", pipelineID).Exec(ctx)
	return err
}

func (s storage) WorkflowList(pipeline *model.Pipeline) ([]*model.Workflow, error) {
	return s.workflowList(context.Background(), s.db, pipeline)
}

// workflowList lists workflows without child steps.
func (s storage) workflowList(ctx context.Context, db bun.IDB, pipeline *model.Pipeline) ([]*model.Workflow, error) {
	var wfList []*model.Workflow
	err := db.NewSelect().Model(&wfList).
		Where("pipeline_id = ?", pipeline.ID).
		Order("pid ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return wfList, nil
}

func (s storage) WorkflowLoad(id int64) (*model.Workflow, error) {
	workflow := new(model.Workflow)
	return workflow, wrapGet(s.db.NewSelect().Model(workflow).Where("id = ?", id).Scan(context.Background()))
}

func (s storage) WorkflowByPID(pipelineID int64, pid int) (*model.Workflow, error) {
	workflow := new(model.Workflow)
	return workflow, wrapGet(s.db.NewSelect().Model(workflow).
		Where("pipeline_id = ?", pipelineID).
		Where("pid = ?", pid).
		Scan(context.Background()))
}

// WorkflowListByAgent returns historic workflows processed by the given agent,
// most recently finished first, enriched with the owning pipeline's repo and
// number for deep-linking.
func (s storage) WorkflowListByAgent(agentID int64, p *model.ListOptions) ([]*model.AgentWorkflow, error) {
	var list []*model.AgentWorkflow
	q := s.db.NewSelect().
		ColumnExpr("w.*").
		ColumnExpr("p.repo_id AS repo_id").
		ColumnExpr("p.number AS pipeline_number").
		TableExpr("workflows AS w").
		Join("JOIN pipelines AS p ON p.id = w.pipeline_id").
		Where("w.agent_id = ?", agentID).
		OrderExpr("w.finished DESC, w.started DESC, w.id DESC")
	if !p.All {
		q = q.Limit(int64(p.PerPage)).Offset(int64(p.PerPage * (p.Page - 1)))
	}
	if err := q.Scan(context.Background(), &list); err != nil {
		return nil, err
	}
	return list, nil
}

// recentPipelineWindow bounds how many of a repo's most recent pipelines are
// scanned for active workflow names. A window (rather than a single pipeline)
// covers repos whose workflows run across separate pipelines — e.g. cron/manual
// or matrix configs where any one pipeline only runs a subset of the workflows —
// while still letting workflows removed from the config age out of the list.
const recentPipelineWindow = 100

// WorkflowNamesByRepo returns the active workflow names for a repo, i.e. the
// distinct workflow names seen across its most recent pipelines (see
// recentPipelineWindow). Workflows long removed from the config eventually drop
// out of the window and so disappear from the filter list.
func (s storage) WorkflowNamesByRepo(repoID int64) ([]string, error) {
	ctx := context.Background()

	// Resolve the recent pipeline IDs first, then filter workflows by them.
	// MySQL does not support LIMIT inside an IN (...) subquery, so this is done
	// in two steps rather than a single nested query.
	var pipelineIDs []int64
	if err := s.db.NewSelect().
		TableExpr("pipelines").
		ColumnExpr("id").
		Where("repo_id = ?", repoID).
		OrderExpr("number DESC").
		Limit(recentPipelineWindow).
		Scan(ctx, &pipelineIDs); err != nil {
		return nil, err
	}
	if len(pipelineIDs) == 0 {
		// No pipelines yet: nothing to filter by.
		return []string{}, nil
	}

	names := []string{}
	if err := s.db.NewSelect().
		TableExpr("workflows").
		ColumnExpr("DISTINCT name").
		Where("pipeline_id IN (?)", bun.List(pipelineIDs)).
		Where("name != ''").
		OrderExpr("name ASC").
		Scan(ctx, &names); err != nil {
		return nil, err
	}
	return names, nil
}

func (s storage) WorkflowUpdate(workflow *model.Workflow) error {
	_, err := s.db.NewUpdate().Model(workflow).WherePK().Exec(context.Background())
	return err
}

// CleanupOrphanedWorkflows removes workflows that exist in the queue but not in the database
// This helps prevent "sql: no rows in result set" errors.
func (s storage) CleanupOrphanedWorkflows(workflowIDs []string) error {
	if len(workflowIDs) == 0 {
		return nil
	}

	// Convert string IDs to int64 for database query
	var ids []int64
	for _, idStr := range workflowIDs {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}

	if len(ids) == 0 {
		return nil
	}

	// Check which workflows actually exist in the database
	var existingWorkflows []*model.Workflow
	err := s.db.NewSelect().Model(&existingWorkflows).
		Where("id IN (?)", bun.List(ids)).
		Scan(context.Background())
	if err != nil {
		return err
	}

	// Create a map of existing workflow IDs
	existingMap := make(map[int64]bool)
	for _, wf := range existingWorkflows {
		existingMap[wf.ID] = true
	}

	// Log orphaned workflows for debugging
	var orphanedIDs []string
	for _, id := range ids {
		if !existingMap[id] {
			orphanedIDs = append(orphanedIDs, strconv.FormatInt(id, 10))
		}
	}

	if len(orphanedIDs) > 0 {
		log.Warn().Strs("orphaned_workflow_ids", orphanedIDs).Msg("found orphaned workflows in queue that don't exist in database")
	}

	return nil
}
