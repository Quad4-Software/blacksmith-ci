// Copyright 2025 Crow CI Authors
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

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/queue"
	"codefloe.com/crowci/crow/v5/server/store"
)

// CascadeSkipBlockedWorkflows synchronously marks pending workflows as skipped
// when their dependencies have completed in a way that prevents them from ever
// running (failure / killed / error / skipped, given the workflow's run_on
// semantics).
//
// Without this, blocked pending workflows would linger in the DB as Pending
// until an agent eventually polls them and the queue's lazy ShouldRun()
// evaluation discards them. That delay leaves the parent pipeline stuck in
// "running" state — sometimes indefinitely on idle agents — even though no
// further work can ever execute.
//
// The cascade is iterative so transitive chains (A fails → B skipped → C
// skipped) collapse in a single call. Each iteration:
//  1. Re-reads the workflow tree from the store.
//  2. For every pending workflow, looks up its task to honor run_on rules.
//  3. Marks the workflow + its pending children as skipped and evicts the
//     queue entry.
//
// Errors are logged per workflow but do not abort the cascade: it's better to
// skip as many blocked workflows as possible than to leave the pipeline stuck
// because one update failed.
func CascadeSkipBlockedWorkflows(ctx context.Context, _store store.Store, q queue.Queue, pipeline *model.Pipeline) error {
	workflows, err := _store.WorkflowGetTree(pipeline)
	if err != nil {
		return fmt.Errorf("cascade skip: load workflow tree: %w", err)
	}

	// Only tasks of pending workflows are consulted below, so the filtered
	// query suffices. The unfiltered TaskList() re-read every row in the
	// tasks table on each non-success workflow completion, which turned
	// leaked stale rows into repeated whole-table loads (678 MB per call in
	// the production incident this fixes).
	tasks, err := _store.TaskListPending()
	if err != nil {
		return fmt.Errorf("cascade skip: load tasks: %w", err)
	}
	tasksByID := make(map[string]*model.Task, len(tasks))
	for _, t := range tasks {
		tasksByID[t.ID] = t
	}

	// Bound the loop by len(workflows): a fresh skip can only unblock
	// downstream workflows we haven't visited yet, so one pass per workflow
	// is the worst case for a strictly DAG-shaped pipeline.
	maxIterations := len(workflows)
	for range maxIterations {
		wfByName := make(map[string]*model.Workflow, len(workflows))
		for _, wf := range workflows {
			wfByName[wf.Name] = wf
		}

		var (
			taskIDsToEvict  []string
			workflowsToSkip []*model.Workflow
		)

		for _, wf := range workflows {
			if wf.State != model.StatusPending {
				continue
			}
			task := tasksByID[fmt.Sprint(wf.ID)]
			if !shouldSkipDueToBlockedDep(wf, task, wfByName) {
				continue
			}
			taskIDsToEvict = append(taskIDsToEvict, fmt.Sprint(wf.ID))
			workflowsToSkip = append(workflowsToSkip, wf)
		}

		if len(workflowsToSkip) == 0 {
			return nil
		}

		if err := q.EvictAtOnce(ctx, taskIDsToEvict); err != nil {
			log.Error().Err(err).Strs("taskIDs", taskIDsToEvict).
				Msg("cascade skip: failed to evict blocked tasks from queue")
		}

		for _, wf := range workflowsToSkip {
			if _, err := UpdateWorkflowToStatusSkipped(_store, *wf); err != nil {
				log.Error().Err(err).Int64("workflowID", wf.ID).
					Msg("cascade skip: failed to mark blocked workflow as skipped")
				continue
			}
			// Reflect the new state on the in-memory tree so the next
			// iteration sees this workflow as Skipped (which can in turn
			// unblock downstream workflows that depend on it).
			wf.State = model.StatusSkipped

			for _, step := range wf.Children {
				if step.State != model.StatusPending {
					continue
				}
				if _, err := UpdateStepToStatusSkipped(_store, *step, 0); err != nil {
					log.Error().Err(err).Int64("stepID", step.ID).
						Msg("cascade skip: failed to mark blocked step as skipped")
				}
			}
		}
	}

	return nil
}

// shouldSkipDueToBlockedDep returns true when wf is pending and its
// dependencies have completed in a way that means wf will never run.
//
// Semantics mirror Task.ShouldRun() but evaluate against current workflow
// state from the store (not just task.DepStatus, which is updated lazily as
// agents poll).
//
// Rules:
//   - If wf has no dependencies recorded in the workflow tree, don't skip:
//     it's either a root workflow or its deps haven't been linked yet.
//   - If any dep is still Running/Pending, don't skip yet — we don't know
//     the outcome.
//   - If task RunOn includes both success and failure, never skip.
//   - If task RunOn includes only failure, skip when every dep succeeded.
//   - Otherwise (default: runs on success), skip when any dep finished
//     non-successfully (Failure / Killed / Error / Skipped).
func shouldSkipDueToBlockedDep(wf *model.Workflow, task *model.Task, wfByName map[string]*model.Workflow) bool {
	if len(wf.DependsOn) == 0 {
		return false
	}

	// Resolve each dependency to its current workflow state. If we can't
	// find it (dangling dep name), be conservative and don't skip.
	depStates := make([]model.StatusValue, 0, len(wf.DependsOn))
	for _, depName := range wf.DependsOn {
		dep, ok := wfByName[depName]
		if !ok {
			return false
		}
		if dep.Running() {
			return false
		}
		depStates = append(depStates, dep.State)
	}

	runsOnFailure := false
	runsOnSuccess := true // default when RunOn is empty
	if task != nil && len(task.RunOn) > 0 {
		runsOnSuccess = false
		for _, s := range task.RunOn {
			switch model.StatusValue(s) {
			case model.StatusSuccess:
				runsOnSuccess = true
			case model.StatusFailure:
				runsOnFailure = true
			}
		}
	}

	// Runs unconditionally — never skip due to dep outcome.
	if runsOnSuccess && runsOnFailure {
		return false
	}

	// Runs only on failure: skip if every dep succeeded.
	if runsOnFailure && !runsOnSuccess {
		for _, s := range depStates {
			if s != model.StatusSuccess {
				return false
			}
		}
		return true
	}

	// Default (runs on success): skip if any dep did not succeed.
	for _, s := range depStates {
		if s != model.StatusSuccess {
			return true
		}
	}
	return false
}
