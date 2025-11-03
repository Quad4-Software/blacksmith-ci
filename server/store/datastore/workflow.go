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
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"xorm.io/xorm"

	"codeberg.org/crowci/crow/v4/pipeline/frontend/yaml"
	"codeberg.org/crowci/crow/v4/pipeline/frontend/yaml/matrix"
	"codeberg.org/crowci/crow/v4/server/model"
)

func (s storage) WorkflowGetTree(pipeline *model.Pipeline) ([]*model.Workflow, error) {
	sess := s.engine.NewSession()
	wfList, err := s.workflowList(sess, pipeline)
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
		wf.Children, err = s.stepListWorkflow(sess, wf)
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

	return wfList, sess.Commit()
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
	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := s.workflowsCreate(sess, workflows); err != nil {
		return err
	}

	return sess.Commit()
}

func (s storage) workflowsCreate(sess *xorm.Session, workflows []*model.Workflow) error {
	for i := range workflows {
		// only Insert on single object ref set auto created ID back to object
		if err := s.stepCreate(sess, workflows[i].Children); err != nil {
			return err
		}
		if _, err := sess.Insert(workflows[i]); err != nil {
			return err
		}
	}
	return nil
}

// WorkflowsReplace performs an atomic replacement of workflows and associated steps by deleting all existing workflows and steps and inserting the new ones.
func (s storage) WorkflowsReplace(pipeline *model.Pipeline, workflows []*model.Workflow) error {
	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := s.workflowsDelete(sess, pipeline.ID); err != nil {
		return err
	}

	// Delete tasks associated with this pipeline to prevent orphaned tasks referencing deleted steps
	if _, err := sess.Where("pipeline_id = ?", pipeline.ID).Delete(new(model.Task)); err != nil {
		return err
	}

	if err := s.workflowsCreate(sess, workflows); err != nil {
		return err
	}

	return sess.Commit()
}

func (s storage) workflowsDelete(sess *xorm.Session, pipelineID int64) error {
	// delete related steps
	for startSteps := 0; ; startSteps += perPage {
		stepIDs := make([]int64, 0, perPage)
		if err := sess.Limit(perPage, startSteps).Table("steps").Cols("id").Where("pipeline_id = ?", pipelineID).Find(&stepIDs); err != nil {
			return err
		}
		if len(stepIDs) == 0 {
			break
		}

		for i := range stepIDs {
			if err := deleteStep(sess, stepIDs[i]); err != nil {
				return err
			}
		}
	}

	_, err := sess.Where("pipeline_id = ?", pipelineID).Delete(new(model.Workflow))
	return err
}

func (s storage) WorkflowList(pipeline *model.Pipeline) ([]*model.Workflow, error) {
	return s.workflowList(s.engine.NewSession(), pipeline)
}

// workflowList lists workflows without child steps.
func (s storage) workflowList(sess *xorm.Session, pipeline *model.Pipeline) ([]*model.Workflow, error) {
	var wfList []*model.Workflow
	err := sess.Where("pipeline_id = ?", pipeline.ID).
		OrderBy("pid").
		Find(&wfList)
	if err != nil {
		return nil, err
	}

	return wfList, nil
}

func (s storage) WorkflowLoad(id int64) (*model.Workflow, error) {
	workflow := new(model.Workflow)
	return workflow, wrapGet(s.engine.ID(id).Get(workflow))
}

func (s storage) WorkflowUpdate(workflow *model.Workflow) error {
	_, err := s.engine.ID(workflow.ID).AllCols().Update(workflow)
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
	err := s.engine.In("id", ids).Find(&existingWorkflows)
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
