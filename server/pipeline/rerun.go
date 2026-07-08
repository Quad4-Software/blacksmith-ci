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
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	backend_types "codefloe.com/crowci/crow/v6/pipeline/backend/types"
	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/forge"
	forge_types "codefloe.com/crowci/crow/v6/server/forge/types"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pipeline/stepbuilder"
	"codefloe.com/crowci/crow/v6/server/store"
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
			// For cascading dependents, check if the corresponding existing variant was failing/skipped/pending.
			// Pending dependents are included because they are stuck waiting on a failed dependency
			// and will never become eligible without being re-queued with the new dependency IDs.
			latestExisting := findLatestWorkflowByVariant(workflows, item.Workflow.Name, item.Workflow.Environ)
			if latestExisting != nil && (latestExisting.Failing() || latestExisting.State == model.StatusSkipped || latestExisting.State == model.StatusPending) {
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

		// Update CI_WORKFLOW_NUMBER in step environments to match the new PID.
		// parsePipeline bakes in sequential PIDs (1, 2, ...) during compilation,
		// but we've reassigned PIDs to avoid clashes with existing workflows.
		// Without this, restarted containers get the same name as the originals.
		updateWorkflowNumberInConfig(item.Config, maxPID)

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

	// Evict old pending tasks from the queue and mark superseded workflows as skipped.
	// When a dependent workflow was stuck pending (its dep failed), the old task is in
	// the queue with stale dependency IDs that can never be satisfied.
	evictSupersededWorkflows(ctx, _store, workflows, pipelineItems)

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
// RerunWorkflow ensures only the target and its failing/skipped/pending dependents
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

// RerunStep reruns a specific step (and its dependents) within an existing pipeline.
// It creates a new workflow attempt with a filtered config containing only the target step,
// its dependent steps, and any clone/service steps needed for the workspace.
func RerunStep(ctx context.Context, _forge forge.Forge, _store store.Store, currentPipeline *model.Pipeline, workflowID, stepID int64, user *model.User, repo *model.Repo) error {
	// Load all workflows for the pipeline
	workflows, err := _store.WorkflowGetTree(currentPipeline)
	if err != nil {
		return &ErrNotFound{Msg: err.Error()}
	}

	// Find the parent workflow
	var parentWorkflow *model.Workflow
	for _, wf := range workflows {
		if wf.ID == workflowID {
			parentWorkflow = wf
			break
		}
	}
	if parentWorkflow == nil {
		return &ErrNotFound{Msg: fmt.Sprintf("workflow with ID %d not found in pipeline", workflowID)}
	}

	// Find the target step
	var targetStep *model.Step
	for _, step := range parentWorkflow.Children {
		if step.ID == stepID {
			targetStep = step
			break
		}
	}
	if targetStep == nil {
		return &ErrNotFound{Msg: fmt.Sprintf("step with ID %d not found in workflow %d", stepID, workflowID)}
	}

	// Validate the step is completed (not running or pending)
	if targetStep.Running() {
		return &ErrBadRequest{Msg: "cannot rerun a step that is still running or pending"}
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

	// Build workflow filter: parent workflow + upstream dependencies
	// (upstream deps are needed so the step builder doesn't strip the workflow for missing deps)
	depNames := make(map[string][]string)
	for _, wf := range workflows {
		depNames[wf.Name] = wf.DependsOn
	}
	included := map[string]bool{parentWorkflow.Name: true}
	changed := true
	for changed {
		changed = false
		for name := range included {
			for _, dep := range depNames[name] {
				if !included[dep] {
					included[dep] = true
					changed = true
				}
			}
		}
	}
	workflowFilter := make([]string, 0, len(included))
	for name := range included {
		workflowFilter = append(workflowFilter, name)
	}

	log.Debug().
		Int64("pipeline_id", currentPipeline.ID).
		Int64("workflow_id", workflowID).
		Int64("step_id", stepID).
		Str("step_name", targetStep.Name).
		Strs("workflow_filter", workflowFilter).
		Msg("rerunning step")

	pipelineItems, err := parsePipeline(_forge, _store, currentPipeline, user, repo, pipelineFiles, nil, workflowFilter) //nolint:contextcheck
	if err != nil {
		return fmt.Errorf("failure to parse pipeline for step rerun: %w", err)
	}

	// Find the item matching the parent workflow (by name and environ)
	targetEnvKey := environKey(parentWorkflow.Environ)
	var targetItem *stepbuilder.Item
	for _, item := range pipelineItems {
		if item.Workflow.Name == parentWorkflow.Name && environKey(item.Workflow.Environ) == targetEnvKey {
			targetItem = item
			break
		}
	}
	if targetItem == nil {
		return &ErrBadRequest{Msg: "could not find matching workflow config for step rerun"}
	}

	// Determine which steps to keep: target + dependents + clone + service
	keepNames := collectRerunSteps(targetStep.Name, targetItem.Config)
	filterConfigInPlace(targetItem.Config, keepNames)

	// If config is now empty, nothing to do
	hasSteps := false
	for _, stage := range targetItem.Config.Stages {
		if len(stage.Steps) > 0 {
			hasSteps = true
			break
		}
	}
	if !hasSteps {
		return &ErrBadRequest{Msg: "no steps matched for rerun"}
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

	// Build attempt info for the workflow
	attemptInfo := buildAttemptInfo(workflows)
	vKey := variantKey(parentWorkflow.Name, parentWorkflow.Environ)
	wfInfo := attemptInfo[vKey]

	// Set up new workflow
	maxPID++
	targetItem.Workflow.PID = maxPID
	targetItem.Workflow.PipelineID = currentPipeline.ID
	targetItem.Workflow.State = model.StatusPending

	// Update CI_WORKFLOW_NUMBER in step environments to match the new PID
	updateWorkflowNumberInConfig(targetItem.Config, maxPID)
	targetItem.Workflow.OriginalWorkflowID = wfInfo.originalID
	targetItem.Workflow.Attempt = wfInfo.maxAttempt + 1
	targetItem.Workflow.Started = 0
	targetItem.Workflow.Finished = 0
	targetItem.Workflow.Error = ""
	targetItem.Workflow.AgentID = 0
	targetItem.Workflow.AgentPersistentID = ""
	targetItem.Workflow.AgentName = ""

	// Build step attempt info from all workflow attempts for this variant
	stepAttemptInfo := buildStepAttemptInfo(workflows, parentWorkflow.Name, parentWorkflow.Environ)

	// Build child steps from the filtered config
	targetItem.Workflow.Children = nil
	for _, stage := range targetItem.Config.Stages {
		for _, step := range stage.Steps {
			maxPID++
			childStep := &model.Step{
				Name:       step.Name,
				UUID:       step.UUID,
				PipelineID: currentPipeline.ID,
				PID:        maxPID,
				PPID:       targetItem.Workflow.PID,
				State:      model.StatusPending,
				Failure:    step.Failure,
				Type:       model.StepType(step.Type),
				DependsOn:  step.DependsOn,
			}
			// Set step attempt tracking
			if info, ok := stepAttemptInfo[step.Name]; ok {
				childStep.OriginalStepID = info.originalID
				childStep.Attempt = info.maxAttempt + 1
			}
			targetItem.Workflow.Children = append(targetItem.Workflow.Children, childStep)
		}
	}

	// Persist the new workflow and steps
	if err := _store.WorkflowsCreate([]*model.Workflow{targetItem.Workflow}); err != nil {
		return fmt.Errorf("failure to persist step rerun workflow: %w", err)
	}

	// Update pipeline status back to running if it was finished
	if currentPipeline.Status != model.StatusRunning && currentPipeline.Status != model.StatusPending {
		currentPipeline.Status = model.StatusRunning
		currentPipeline.Finished = 0
		if err := _store.UpdatePipeline(currentPipeline); err != nil {
			return fmt.Errorf("failure to update pipeline status: %w", err)
		}
	}

	// Reload all workflows to get fresh state
	currentPipeline.Workflows, err = _store.WorkflowGetTree(currentPipeline)
	if err != nil {
		return err
	}

	// Queue the new workflow
	if err := queuePipeline(ctx, repo, []*stepbuilder.Item{targetItem}); err != nil {
		return fmt.Errorf("failure to queue step rerun workflow: %w", err)
	}

	// Publish update for real-time UI
	updatePipelineStatus(ctx, _forge, currentPipeline, repo, user)
	publishToTopic(currentPipeline, repo)

	return nil
}

// collectRerunSteps determines which steps to keep for a step rerun:
// the target step, any steps that depend on it (transitively), plus all clone and service steps.
// Clone/service steps are added after the cascade to avoid pulling in all steps
// (since the compiler sets implicit DependsOn=[clone] on every step).
func collectRerunSteps(targetStepName string, cfg *backend_types.Config) map[string]bool {
	keepNames := map[string]bool{targetStepName: true}

	// Collect all steps from the config
	var allSteps []*backend_types.Step
	for _, stage := range cfg.Stages {
		allSteps = append(allSteps, stage.Steps...)
	}

	// Forward-cascade from target only: steps that depend on anything in keepNames
	changed := true
	for changed {
		changed = false
		for _, step := range allSteps {
			if keepNames[step.Name] {
				continue
			}
			for _, dep := range step.DependsOn {
				if keepNames[dep] {
					keepNames[step.Name] = true
					changed = true
					break
				}
			}
		}
	}

	// Add clone and service steps after the cascade so they don't drive inclusion
	for _, step := range allSteps {
		if step.Type == backend_types.StepTypeClone || step.Type == backend_types.StepTypeService {
			keepNames[step.Name] = true
		}
	}

	return keepNames
}

// filterConfigInPlace removes steps not in keepNames from the config's stages,
// and drops stages that become empty.
func filterConfigInPlace(cfg *backend_types.Config, keepNames map[string]bool) {
	var filteredStages []*backend_types.Stage
	for _, stage := range cfg.Stages {
		var filteredSteps []*backend_types.Step
		for _, step := range stage.Steps {
			if keepNames[step.Name] {
				filteredSteps = append(filteredSteps, step)
			}
		}
		if len(filteredSteps) > 0 {
			stage.Steps = filteredSteps
			filteredStages = append(filteredStages, stage)
		}
	}
	cfg.Stages = filteredStages
}

// updateWorkflowNumberInConfig updates CI_WORKFLOW_NUMBER in all step
// environments within a compiled config. This is necessary because parsePipeline
// bakes in sequential PIDs during compilation, but rerun assigns new PIDs to
// avoid clashes with existing workflows. Without this update, restarted
// containers in matrix workflows get the same name as the originals.
func updateWorkflowNumberInConfig(cfg *backend_types.Config, newPID int) {
	pidStr := strconv.Itoa(newPID)
	for _, stage := range cfg.Stages {
		for _, step := range stage.Steps {
			if step.Environment != nil {
				step.Environment["CI_WORKFLOW_NUMBER"] = pidStr
			}
		}
	}
}

type stepAttemptData struct {
	originalID int64
	maxAttempt int
}

// buildStepAttemptInfo builds a map from step name to its original step ID and highest attempt
// across all workflow attempts for the given workflow variant.
func buildStepAttemptInfo(workflows []*model.Workflow, workflowName string, workflowEnviron map[string]string) map[string]stepAttemptData {
	envKey := environKey(workflowEnviron)
	info := make(map[string]stepAttemptData)

	for _, wf := range workflows {
		if wf.Name != workflowName || environKey(wf.Environ) != envKey {
			continue
		}
		for _, step := range wf.Children {
			existing, found := info[step.Name]
			originalID := step.OriginalStepID
			if originalID == 0 {
				originalID = step.ID
			}
			if !found || step.Attempt > existing.maxAttempt {
				info[step.Name] = stepAttemptData{
					originalID: originalID,
					maxAttempt: step.Attempt,
				}
			}
		}
	}
	return info
}

// evictSupersededWorkflows removes old pending tasks from the queue and marks
// old workflows as skipped when they are being superseded by new rerun attempts.
// This prevents stale tasks (with outdated dependency IDs) from lingering in the queue.
func evictSupersededWorkflows(ctx context.Context, _store store.Store, existingWorkflows []*model.Workflow, newItems []*stepbuilder.Item) {
	rerunVariants := make(map[string]bool, len(newItems))
	for _, item := range newItems {
		rerunVariants[variantKey(item.Workflow.Name, item.Workflow.Environ)] = true
	}

	var taskIDsToEvict []string
	var workflowsToSkip []*model.Workflow

	for _, wf := range existingWorkflows {
		if wf.State != model.StatusPending {
			continue
		}
		if !rerunVariants[variantKey(wf.Name, wf.Environ)] {
			continue
		}
		taskIDsToEvict = append(taskIDsToEvict, fmt.Sprint(wf.ID))
		workflowsToSkip = append(workflowsToSkip, wf)
	}

	if len(taskIDsToEvict) > 0 {
		if err := server.Config.Services.Queue.EvictAtOnce(ctx, taskIDsToEvict); err != nil {
			log.Error().Err(err).Strs("taskIDs", taskIDsToEvict).Msg("failed to evict superseded tasks from queue")
		}
	}

	for _, wf := range workflowsToSkip {
		if _, err := UpdateWorkflowToStatusSkipped(_store, *wf); err != nil {
			log.Error().Err(err).Int64("workflowID", wf.ID).Msg("failed to mark superseded workflow as skipped")
		}
		for _, step := range wf.Children {
			if step.State == model.StatusPending {
				if _, err := UpdateStepToStatusSkipped(_store, *step, 0); err != nil {
					log.Error().Err(err).Int64("stepID", step.ID).Msg("failed to mark superseded step as skipped")
				}
			}
		}
	}
}
