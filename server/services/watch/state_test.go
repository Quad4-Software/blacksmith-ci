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
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
)

var testKey = model.WatchKey{RepoID: 1, Event: model.EventPush, Target: "main"}

func state(name string, failing bool, number int64) *model.WatchWorkflowState {
	return &model.WatchWorkflowState{
		RepoID: 1, Event: model.EventPush, Target: "main",
		WorkflowName: name, Failing: failing, PipelineID: number * 100, PipelineNumber: number,
	}
}

func run(number int64, wfs map[string]bool) RunResult {
	return RunResult{PipelineID: number * 100, PipelineNumber: number, Workflows: wfs}
}

func find(states []*model.WatchWorkflowState, name string) *model.WatchWorkflowState {
	for _, s := range states {
		if s.WorkflowName == name {
			return s
		}
	}
	return nil
}

func TestApplyRun_DocsOnlyPassKeepsBuildFailing(t *testing.T) {
	next := ApplyRun(testKey, []*model.WatchWorkflowState{state("build", true, 10)}, run(11, map[string]bool{"docs": false}))
	assert.True(t, find(next, "build").Failing)
	assert.False(t, find(next, "docs").Failing)
	assert.True(t, IsFailing(next))
}

func TestApplyRun_PassClears(t *testing.T) {
	next := ApplyRun(testKey, []*model.WatchWorkflowState{state("build", true, 10)}, run(12, map[string]bool{"build": false}))
	assert.False(t, IsFailing(next))
}

func TestApplyRun_IgnoresOlderRun(t *testing.T) {
	next := ApplyRun(testKey, []*model.WatchWorkflowState{state("build", false, 12)}, run(11, map[string]bool{"build": true}))
	assert.False(t, find(next, "build").Failing)
	assert.Equal(t, int64(12), find(next, "build").PipelineNumber)
}

func TestApplyRun_SamePipelineRetryClears(t *testing.T) {
	next := ApplyRun(testKey, []*model.WatchWorkflowState{state("build", true, 10)}, run(10, map[string]bool{"build": false}))
	assert.False(t, IsFailing(next))
}

func TestApplyRun_ConfigErrorThenFixed(t *testing.T) {
	next := ApplyRun(testKey, nil, run(5, map[string]bool{pipelineLevelWorkflow: true}))
	assert.True(t, IsFailing(next))
	assert.Equal(t, int64(5), NewestFailingNumber(next))
	next = ApplyRun(testKey, next, run(6, map[string]bool{"build": false}))
	assert.False(t, find(next, pipelineLevelWorkflow).Failing)
	assert.False(t, IsFailing(next))
}

func TestApplyRun_ExpiresVanishedWorkflow(t *testing.T) {
	states := []*model.WatchWorkflowState{state("old", true, 1)}
	for n := int64(2); n <= 21; n++ {
		states = ApplyRun(testKey, states, run(n, map[string]bool{"build": false}))
	}
	assert.Nil(t, find(states, "old"))
	assert.False(t, IsFailing(states))
}

func TestApplyRun_DoesNotMutatePrevAndFillsNewRows(t *testing.T) {
	prev := []*model.WatchWorkflowState{state("build", true, 10)}
	next := ApplyRun(testKey, prev, run(11, map[string]bool{"build": false, "docs": true}))
	assert.True(t, prev[0].Failing)
	assert.Equal(t, int64(10), prev[0].PipelineNumber)
	docs := find(next, "docs")
	assert.Equal(t, model.WatchKey{RepoID: docs.RepoID, Event: docs.Event, Target: docs.Target}, testKey)
	assert.Equal(t, int64(11), docs.PipelineNumber)
	assert.Equal(t, int64(1100), docs.PipelineID)
	assert.Equal(t, "build", next[0].WorkflowName)
}

func TestApplyRun_EmptyRunOnlyBookkeeping(t *testing.T) {
	prev := []*model.WatchWorkflowState{state("build", true, 10)}
	next := ApplyRun(testKey, prev, run(11, nil))
	assert.True(t, find(next, "build").Failing)
	assert.Equal(t, int64(1), find(next, "build").RunsSinceSeen)
}

func TestResultFromPipeline(t *testing.T) {
	_, ok := ResultFromPipeline(&model.Pipeline{Status: model.StatusKilled}, nil)
	assert.False(t, ok)

	res, ok := ResultFromPipeline(&model.Pipeline{ID: 7, Number: 3, Status: model.StatusSuccess}, []*model.Workflow{
		{ID: 1, Name: "build", State: model.StatusSuccess},
		{ID: 2, Name: "lint", State: model.StatusSkipped},
	})
	assert.True(t, ok)
	assert.Equal(t, int64(7), res.PipelineID)
	assert.Equal(t, int64(3), res.PipelineNumber)
	assert.Equal(t, map[string]bool{"build": false}, res.Workflows)

	res, ok = ResultFromPipeline(&model.Pipeline{Status: model.StatusFailure}, []*model.Workflow{
		{ID: 1, Name: "build", State: model.StatusFailure, Attempt: 1},
		{ID: 2, Name: "build", State: model.StatusSuccess, Attempt: 2, OriginalWorkflowID: 1},
	})
	assert.True(t, ok)
	assert.Equal(t, map[string]bool{"build": false}, res.Workflows)

	res, ok = ResultFromPipeline(&model.Pipeline{Status: model.StatusError}, nil)
	assert.True(t, ok)
	assert.Equal(t, map[string]bool{"": true}, res.Workflows)
}

func TestIsUnread(t *testing.T) {
	failing := []*model.WatchWorkflowState{state("build", true, 11), state("lint", true, 8)}
	assert.True(t, IsUnread(&model.PipelineWatch{LastReadPipeline: 10}, failing))
	assert.False(t, IsUnread(&model.PipelineWatch{LastReadPipeline: 11}, failing))
	assert.False(t, IsUnread(&model.PipelineWatch{LastReadPipeline: 0}, []*model.WatchWorkflowState{state("build", false, 11)}))
}

func TestResultFromPipeline_ErrorWithPendingWorkflows(t *testing.T) {
	p := &model.Pipeline{ID: 100, Number: 1, Status: model.StatusError}
	res, ok := ResultFromPipeline(p, []*model.Workflow{{Name: "a", State: model.StatusPending}, {Name: "b", State: model.StatusBlocked}})
	assert.True(t, ok)
	assert.Equal(t, map[string]bool{"": true}, res.Workflows)
}
