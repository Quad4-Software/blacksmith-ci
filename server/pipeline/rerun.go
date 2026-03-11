// Copyright 2026 Crow Authors
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

package pipeline

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server/forge"
	forge_types "codefloe.com/crowci/crow/v5/server/forge/types"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/pipeline/stepbuilder"
	"codefloe.com/crowci/crow/v5/server/store"
)

// RerunWorkflow reruns a specific workflow within an existing pipeline.
// It creates new workflow+step records (preserving the original) and queues them for execution.
// If the rerun'd workflow has dependents that failed/were skipped, those are also rerun.
func RerunWorkflow(ctx context.Context, _forge forge.Forge, _store store.Store, currentPipeline *model.Pipeline, workflowID int64, user *model.User, repo *model.Repo) error {
	// Load all workflows for the pipeline
	workflows, err := _store.WorkflowGetTree(currentPipeline)
	if err != nil {
		return &ErrNotFound{Msg: err.Error()}
	}

	// Find the target workflow
	var targetWorkflow *model.Workflow
	for _, wf := range workflows {
		if wf.ID == workflowID {
			targetWorkflow = wf
			break
		}
	}

	if targetWorkflow == nil {
		return &ErrNotFound{Msg: fmt.Sprintf("workflow with ID %d not found in pipeline", workflowID)}
	}

	// Validate the workflow is completed (not running or pending)
	if targetWorkflow.Running() {
		return &ErrBadRequest{Msg: "cannot rerun a workflow that is still running or pending"}
	}

	// Fetch pipeline configs
	configs, err := _store.ConfigsForPipeline(currentPipeline.ID)
	if err != nil {
		return &ErrNotFound{Msg: fmt.Sprintf("failure to get pipeline config for %s: %s", repo.FullName, err)}
	}

	var pipelineFiles []*forge_types.FileMeta
	for _, y := range configs {
		pipelineFiles = append(pipelineFiles, &forge_types.FileMeta{Data: y.Data, Name: y.Name})
	}

	// Build the workflow filter: the target workflow and any dependents that should cascade
	workflowFilter := collectRerunWorkflows(targetWorkflow, workflows)

	log.Debug().
		Int64("pipeline_id", currentPipeline.ID).
		Int64("workflow_id", workflowID).
		Strs("workflow_filter", workflowFilter).
		Msg("rerunning workflows")

	// Parse the pipeline with the workflow filter to get only the relevant items
	pipelineItems, err := parsePipeline(_forge, _store, currentPipeline, user, repo, pipelineFiles, nil, workflowFilter) //nolint:contextcheck
	if err != nil {
		return fmt.Errorf("failure to parse pipeline for rerun: %w", err)
	}

	// For matrix builds, parsePipeline returns ALL variants matching the name filter.
	// We only want to rerun the specific variant that was clicked (matched by environ),
	// plus all variants of any cascading dependents.
	targetEnvKey := environKey(targetWorkflow.Environ)
	var filteredItems []*stepbuilder.Item
	for _, item := range pipelineItems {
		if item.Workflow.Name == targetWorkflow.Name {
			// For the target workflow name, only keep the variant with matching environ
			if environKey(item.Workflow.Environ) == targetEnvKey {
				filteredItems = append(filteredItems, item)
			}
		} else {
			// For cascading dependents, check if the corresponding existing variant was failing/skipped
			latestExisting := findLatestWorkflowByVariant(workflows, item.Workflow.Name, item.Workflow.Environ)
			if latestExisting != nil && (latestExisting.Failing() || latestExisting.State == model.StatusSkipped) {
				filteredItems = append(filteredItems, item)
			}
		}
	}
	pipelineItems = filteredItems

	if len(pipelineItems) == 0 {
		return &ErrBadRequest{Msg: "no workflows matched for rerun"}
	}

	// Find the current max PID in the pipeline
	maxPID := 0
	for _, wf := range workflows {
		if wf.PID > maxPID {
			maxPID = wf.PID
		}
		for _, step := range wf.Children {
			if step.PID > maxPID {
				maxPID = step.PID
			}
		}
	}

	// Build a map from variant key (name+environ) to original workflow ID and max attempt
	attemptInfo := buildAttemptInfo(workflows)

	// Assign new PIDs and set rerun metadata on each item
	for _, item := range pipelineItems {
		maxPID++
		item.Workflow.PID = maxPID
		item.Workflow.PipelineID = currentPipeline.ID
		item.Workflow.State = model.StatusPending

		// Set rerun fields using variant key (name+environ) for correct matrix matching
		vKey := variantKey(item.Workflow.Name, item.Workflow.Environ)
		info := attemptInfo[vKey]
		item.Workflow.OriginalWorkflowID = info.originalID
		item.Workflow.Attempt = info.maxAttempt + 1

		// Reset timing and agent fields
		item.Workflow.Started = 0
		item.Workflow.Finished = 0
		item.Workflow.Error = ""
		item.Workflow.AgentID = 0
		item.Workflow.AgentPersistentID = ""
		item.Workflow.AgentName = ""

		// Build child steps from the compiled config
		for _, stage := range item.Config.Stages {
			for _, step := range stage.Steps {
				maxPID++
				childStep := &model.Step{
					Name:       step.Name,
					UUID:       step.UUID,
					PipelineID: currentPipeline.ID,
					PID:        maxPID,
					PPID:       item.Workflow.PID,
					State:      model.StatusPending,
					Failure:    step.Failure,
					Type:       model.StepType(step.Type),
					DependsOn:  step.DependsOn,
				}
				item.Workflow.Children = append(item.Workflow.Children, childStep)
			}
		}
	}

	// Persist the new workflows and steps
	newWorkflows := make([]*model.Workflow, 0, len(pipelineItems))
	for _, item := range pipelineItems {
		newWorkflows = append(newWorkflows, item.Workflow)
	}

	if err := _store.WorkflowsCreate(newWorkflows); err != nil {
		return fmt.Errorf("failure to persist rerun workflows: %w", err)
	}

	// Update pipeline status back to running if it was finished
	if currentPipeline.Status != model.StatusRunning && currentPipeline.Status != model.StatusPending {
		currentPipeline.Status = model.StatusRunning
		currentPipeline.Finished = 0
		if err := _store.UpdatePipeline(currentPipeline); err != nil {
			return fmt.Errorf("failure to update pipeline status: %w", err)
		}
	}

	// Reload all workflows to get fresh state including the new ones
	currentPipeline.Workflows, err = _store.WorkflowGetTree(currentPipeline)
	if err != nil {
		return err
	}

	// Queue the new workflows
	if err := queuePipeline(ctx, repo, pipelineItems); err != nil {
		return fmt.Errorf("failure to queue rerun workflows: %w", err)
	}

	// Publish update for real-time UI
	updatePipelineStatus(ctx, _forge, currentPipeline, repo, user)
	publishToTopic(currentPipeline, repo)

	return nil
}

type attemptData struct {
	originalID int64
	maxAttempt int
}

// environKey returns a canonical string representation of an environ map for comparison.
func environKey(environ map[string]string) string {
	if len(environ) == 0 {
		return ""
	}
	keys := make([]string, 0, len(environ))
	for k := range environ {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+environ[k])
	}
	return strings.Join(parts, ",")
}

// variantKey returns a composite key of workflow name + environ for matching matrix variants.
func variantKey(name string, environ map[string]string) string {
	return name + "|" + environKey(environ)
}

// findLatestWorkflowByVariant finds the latest attempt of a workflow matching by name and environ.
func findLatestWorkflowByVariant(workflows []*model.Workflow, name string, environ map[string]string) *model.Workflow {
	envKey := environKey(environ)
	var latest *model.Workflow
	for _, wf := range workflows {
		if wf.Name == name && environKey(wf.Environ) == envKey {
			if latest == nil || wf.Attempt > latest.Attempt {
				latest = wf
			}
		}
	}
	return latest
}

// buildAttemptInfo creates a map from variant key (name+environ) to its original workflow ID and highest attempt number.
func buildAttemptInfo(workflows []*model.Workflow) map[string]attemptData {
	// First pass: find the true original (OriginalWorkflowID=0) for each variant
	originals := make(map[string]int64)
	for _, wf := range workflows {
		if wf.OriginalWorkflowID == 0 {
			key := variantKey(wf.Name, wf.Environ)
			originals[key] = wf.ID
		}
	}

	// Second pass: find max attempt per variant, using the true original's ID
	info := make(map[string]attemptData)
	for _, wf := range workflows {
		key := variantKey(wf.Name, wf.Environ)
		existing, found := info[key]

		// Prefer the true original's ID found in the first pass
		originalID := originals[key]
		if originalID == 0 {
			// Fallback: no original found (shouldn't happen), use best guess
			originalID = wf.OriginalWorkflowID
			if originalID == 0 {
				originalID = wf.ID
			}
		}

		if !found || wf.Attempt > existing.maxAttempt {
			info[key] = attemptData{
				originalID: originalID,
				maxAttempt: wf.Attempt,
			}
		}
	}
	return info
}

// collectRerunWorkflows returns the list of workflow names to include in the
// pipeline parse filter: the target workflow, any downstream dependents
// (cascading reruns), and all upstream dependencies.
//
// Upstream dependencies must be included in the filter so that
// filterItemsWithMissingDependencies (in the step builder) does not strip
// the target workflow for having "missing" deps. The post-filter in
// RerunWorkflow ensures only the target and its failing/skipped dependents
// are actually rerun — successful upstream deps are excluded there.
//
// Note: this returns names for the WorkflowFilter. For matrix builds,
// individual variant filtering is done after parsePipeline.
func collectRerunWorkflows(target *model.Workflow, allWorkflows []*model.Workflow) []string {
	rerunNames := map[string]bool{target.Name: true}

	// Iteratively find downstream dependents that should be rerun
	changed := true
	for changed {
		changed = false
		for _, wf := range allWorkflows {
			if rerunNames[wf.Name] {
				continue
			}
			for _, dep := range wf.DependsOn {
				if rerunNames[dep] {
					rerunNames[wf.Name] = true
					changed = true
					break
				}
			}
		}
	}

	// Also include upstream dependencies (transitively) so that the step
	// builder's filterItemsWithMissingDependencies doesn't remove the target.
	depNames := make(map[string][]string)
	for _, wf := range allWorkflows {
		depNames[wf.Name] = wf.DependsOn
	}
	changed = true
	for changed {
		changed = false
		for name := range rerunNames {
			for _, dep := range depNames[name] {
				if !rerunNames[dep] {
					rerunNames[dep] = true
					changed = true
				}
			}
		}
	}

	result := make([]string, 0, len(rerunNames))
	for name := range rerunNames {
		result = append(result, name)
	}
	return result
}
