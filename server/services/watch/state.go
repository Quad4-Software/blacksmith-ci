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

package watch

import (
	"sort"

	"codefloe.com/crowci/crow/v6/server/model"
)

// workflowExpiryRuns is the number of newer runs a workflow may be absent
// from before its state row is dropped (renamed or removed workflows).
const workflowExpiryRuns = 20

// pipelineLevelWorkflow is the pseudo-workflow name that carries pipeline-level
// failures such as configuration errors.
const pipelineLevelWorkflow = ""

// RunResult holds the outcome per workflow name of one finished, genuine pipeline.
type RunResult struct {
	PipelineID     int64
	PipelineNumber int64
	// Workflows maps a workflow name to whether it is failing.
	Workflows map[string]bool
}

// ResultFromPipeline derives the per-workflow outcome of a pipeline.
// Ok is false when the pipeline status is not a genuine outcome.
// Only success and failure/error workflows are reported; others are omitted.
func ResultFromPipeline(p *model.Pipeline, workflows []*model.Workflow) (RunResult, bool) {
	if !model.IsGenuineOutcome(p.Status) {
		return RunResult{}, false
	}
	res := RunResult{PipelineID: p.ID, PipelineNumber: p.Number, Workflows: map[string]bool{}}
	latest := model.LatestAttempts(workflows)
	for _, wf := range latest {
		switch wf.State {
		case model.StatusSuccess:
			res.Workflows[wf.Name] = false
		case model.StatusFailure, model.StatusError:
			res.Workflows[wf.Name] = true
		}
	}
	// A pipeline error with no finished workflow (none, or only pending ones
	// such as after a config error on approval) is a pipeline-level failure.
	if p.Status == model.StatusError && len(res.Workflows) == 0 {
		res.Workflows[pipelineLevelWorkflow] = true
	}
	return res, true
}

// ApplyRun returns the new workflow states after applying a run to prev.
// The input rows are never mutated; changed rows are copied.
// The result is sorted by workflow name.
func ApplyRun(key model.WatchKey, prev []*model.WatchWorkflowState, run RunResult) []*model.WatchWorkflowState {
	rows := make(map[string]*model.WatchWorkflowState, len(prev)+len(run.Workflows))
	for _, row := range prev {
		cp := *row
		rows[row.WorkflowName] = &cp
	}

	// Any run with a real workflow means the pipeline config parsed.
	outcomes := make(map[string]bool, len(run.Workflows)+1)
	for name, failing := range run.Workflows {
		outcomes[name] = failing
	}
	hasReal := false
	for name := range run.Workflows {
		if name != pipelineLevelWorkflow {
			hasReal = true
			break
		}
	}
	if _, ok := outcomes[pipelineLevelWorkflow]; hasReal && !ok {
		if _, exists := rows[pipelineLevelWorkflow]; exists {
			outcomes[pipelineLevelWorkflow] = false
		}
	}

	for name, failing := range outcomes {
		row, exists := rows[name]
		if !exists {
			rows[name] = &model.WatchWorkflowState{
				RepoID: key.RepoID, Event: key.Event, Target: key.Target,
				WorkflowName: name, Failing: failing,
				PipelineID: run.PipelineID, PipelineNumber: run.PipelineNumber,
			}
			continue
		}
		if run.PipelineNumber >= row.PipelineNumber {
			row.Failing = failing
			row.PipelineID = run.PipelineID
			row.PipelineNumber = run.PipelineNumber
			row.RunsSinceSeen = 0
		}
	}

	for name, row := range rows {
		if _, seen := outcomes[name]; !seen && run.PipelineNumber > row.PipelineNumber {
			row.RunsSinceSeen++
		}
	}

	out := make([]*model.WatchWorkflowState, 0, len(rows))
	for _, row := range rows {
		if row.RunsSinceSeen >= workflowExpiryRuns {
			continue
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].WorkflowName < out[j].WorkflowName })
	return out
}

// IsFailing reports whether any workflow is failing.
func IsFailing(states []*model.WatchWorkflowState) bool {
	for _, s := range states {
		if s.Failing {
			return true
		}
	}
	return false
}

// NewestFailingNumber returns the highest pipeline number among failing
// workflows, or 0 if none is failing.
func NewestFailingNumber(states []*model.WatchWorkflowState) int64 {
	var newest int64
	for _, s := range states {
		if s.Failing && s.PipelineNumber > newest {
			newest = s.PipelineNumber
		}
	}
	return newest
}

// IsUnread reports whether the watch has a failure the user has not read.
func IsUnread(w *model.PipelineWatch, states []*model.WatchWorkflowState) bool {
	return IsFailing(states) && NewestFailingNumber(states) > w.LastReadPipeline
}
