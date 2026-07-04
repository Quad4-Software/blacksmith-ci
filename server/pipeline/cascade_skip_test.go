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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v5/server/model"
	queue_mocks "codefloe.com/crowci/crow/v5/server/queue/mocks"
	store_mocks "codefloe.com/crowci/crow/v5/server/store/mocks"
)

// TestCascadeSkipBlockedWorkflows_SingleDepFailure verifies that a pending
// workflow whose dependency just failed gets marked as Skipped, evicted from
// the queue, and its child steps get skipped as well.
func TestCascadeSkipBlockedWorkflows_SingleDepFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	pipeline := &model.Pipeline{ID: 100}

	failedWorkflow := &model.Workflow{
		ID:         1,
		PipelineID: 100,
		Name:       "build",
		State:      model.StatusFailure,
	}
	pendingStep := &model.Step{
		ID:    20,
		PID:   1,
		Name:  "deploy-step",
		State: model.StatusPending,
	}
	pendingWorkflow := &model.Workflow{
		ID:         2,
		PipelineID: 100,
		Name:       "deploy",
		State:      model.StatusPending,
		DependsOn:  model.JSONSliceString{"build"},
		Children:   []*model.Step{pendingStep},
	}

	mockStore := new(store_mocks.MockStore)
	mockStore.On("WorkflowGetTree", pipeline).Return([]*model.Workflow{failedWorkflow, pendingWorkflow}, nil)
	// Default RunOn behavior (success only) — no need to look up tasks if RunOn is empty,
	// but the helper still calls TaskList for correctness.
	mockStore.On("TaskListPending").Return([]*model.Task{
		{ID: "2", Dependencies: model.JSONSliceString{"1"}},
	}, nil)
	mockStore.On("WorkflowUpdate", mock.MatchedBy(func(wf *model.Workflow) bool {
		return wf.ID == 2 && wf.State == model.StatusSkipped
	})).Return(nil)
	mockStore.On("StepUpdate", mock.MatchedBy(func(s *model.Step) bool {
		return s.ID == 20 && s.State == model.StatusSkipped
	})).Return(nil)

	mockQueue := new(queue_mocks.MockQueue)
	mockQueue.On("EvictAtOnce", mock.Anything, []string{"2"}).Return(nil)

	err := CascadeSkipBlockedWorkflows(ctx, mockStore, mockQueue, pipeline)
	assert.NoError(t, err)

	mockStore.AssertExpectations(t)
	mockQueue.AssertExpectations(t)
}

// TestCascadeSkipBlockedWorkflows_TransitiveCascade verifies that when A fails,
// B (depends on A) is skipped, and C (depends on B) is also skipped — so the
// whole sequential chain is unblocked in one call.
func TestCascadeSkipBlockedWorkflows_TransitiveCascade(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	pipeline := &model.Pipeline{ID: 200}

	wfA := &model.Workflow{
		ID:         10,
		PipelineID: 200,
		Name:       "a",
		State:      model.StatusFailure,
	}
	wfB := &model.Workflow{
		ID:         11,
		PipelineID: 200,
		Name:       "b",
		State:      model.StatusPending,
		DependsOn:  model.JSONSliceString{"a"},
	}
	wfC := &model.Workflow{
		ID:         12,
		PipelineID: 200,
		Name:       "c",
		State:      model.StatusPending,
		DependsOn:  model.JSONSliceString{"b"},
	}

	mockStore := new(store_mocks.MockStore)
	mockStore.On("WorkflowGetTree", pipeline).Return([]*model.Workflow{wfA, wfB, wfC}, nil)
	mockStore.On("TaskListPending").Return([]*model.Task{
		{ID: "11", Dependencies: model.JSONSliceString{"10"}},
		{ID: "12", Dependencies: model.JSONSliceString{"11"}},
	}, nil)

	skippedIDs := make(map[int64]bool)
	mockStore.On("WorkflowUpdate", mock.MatchedBy(func(wf *model.Workflow) bool {
		if wf.State == model.StatusSkipped {
			skippedIDs[wf.ID] = true
			return true
		}
		return false
	})).Return(nil)

	mockQueue := new(queue_mocks.MockQueue)
	// Both B and C should be evicted (separate calls allowed since cascade is iterative).
	mockQueue.On("EvictAtOnce", mock.Anything, mock.Anything).Return(nil)

	err := CascadeSkipBlockedWorkflows(ctx, mockStore, mockQueue, pipeline)
	assert.NoError(t, err)

	assert.True(t, skippedIDs[11], "workflow B should be skipped")
	assert.True(t, skippedIDs[12], "workflow C should be skipped (transitive)")
}

// TestCascadeSkipBlockedWorkflows_RunsOnFailureNotSkipped verifies that a
// workflow with run_on: [failure] is NOT skipped when its dep fails — it
// should run.
func TestCascadeSkipBlockedWorkflows_RunsOnFailureNotSkipped(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	pipeline := &model.Pipeline{ID: 300}

	wfA := &model.Workflow{
		ID:         20,
		PipelineID: 300,
		Name:       "a",
		State:      model.StatusFailure,
	}
	wfB := &model.Workflow{
		ID:         21,
		PipelineID: 300,
		Name:       "b",
		State:      model.StatusPending,
		DependsOn:  model.JSONSliceString{"a"},
	}

	mockStore := new(store_mocks.MockStore)
	mockStore.On("WorkflowGetTree", pipeline).Return([]*model.Workflow{wfA, wfB}, nil)
	mockStore.On("TaskListPending").Return([]*model.Task{
		{
			ID:           "21",
			Dependencies: model.JSONSliceString{"20"},
			RunOn:        model.JSONSliceString{string(model.StatusFailure)},
		},
	}, nil)

	mockQueue := new(queue_mocks.MockQueue)

	err := CascadeSkipBlockedWorkflows(ctx, mockStore, mockQueue, pipeline)
	assert.NoError(t, err)

	// No WorkflowUpdate / EvictAtOnce expectations — neither should be called.
	mockStore.AssertNotCalled(t, "WorkflowUpdate", mock.Anything)
	mockStore.AssertNotCalled(t, "StepUpdate", mock.Anything)
	mockQueue.AssertNotCalled(t, "EvictAtOnce", mock.Anything, mock.Anything)
}

// TestCascadeSkipBlockedWorkflows_RunsOnBothNotSkipped verifies that a
// workflow with run_on: [success, failure] is NOT skipped when its dep fails —
// it should run regardless of dep outcome.
func TestCascadeSkipBlockedWorkflows_RunsOnBothNotSkipped(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	pipeline := &model.Pipeline{ID: 400}

	wfA := &model.Workflow{
		ID:         30,
		PipelineID: 400,
		Name:       "a",
		State:      model.StatusFailure,
	}
	wfB := &model.Workflow{
		ID:         31,
		PipelineID: 400,
		Name:       "b",
		State:      model.StatusPending,
		DependsOn:  model.JSONSliceString{"a"},
	}

	mockStore := new(store_mocks.MockStore)
	mockStore.On("WorkflowGetTree", pipeline).Return([]*model.Workflow{wfA, wfB}, nil)
	mockStore.On("TaskListPending").Return([]*model.Task{
		{
			ID:           "31",
			Dependencies: model.JSONSliceString{"30"},
			RunOn:        model.JSONSliceString{string(model.StatusSuccess), string(model.StatusFailure)},
		},
	}, nil)

	mockQueue := new(queue_mocks.MockQueue)

	err := CascadeSkipBlockedWorkflows(ctx, mockStore, mockQueue, pipeline)
	assert.NoError(t, err)

	mockStore.AssertNotCalled(t, "WorkflowUpdate", mock.Anything)
	mockQueue.AssertNotCalled(t, "EvictAtOnce", mock.Anything, mock.Anything)
}

// TestCascadeSkipBlockedWorkflows_NoFailedDepsNoChanges verifies that when no
// workflow has failed (e.g. one is still running), no pending workflow is
// skipped.
func TestCascadeSkipBlockedWorkflows_NoFailedDepsNoChanges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	pipeline := &model.Pipeline{ID: 500}

	wfA := &model.Workflow{
		ID:         40,
		PipelineID: 500,
		Name:       "a",
		State:      model.StatusRunning, // still running
	}
	wfB := &model.Workflow{
		ID:         41,
		PipelineID: 500,
		Name:       "b",
		State:      model.StatusPending,
		DependsOn:  model.JSONSliceString{"a"},
	}

	mockStore := new(store_mocks.MockStore)
	mockStore.On("WorkflowGetTree", pipeline).Return([]*model.Workflow{wfA, wfB}, nil)
	mockStore.On("TaskListPending").Return([]*model.Task{
		{ID: "41", Dependencies: model.JSONSliceString{"40"}},
	}, nil)

	mockQueue := new(queue_mocks.MockQueue)

	err := CascadeSkipBlockedWorkflows(ctx, mockStore, mockQueue, pipeline)
	assert.NoError(t, err)

	mockStore.AssertNotCalled(t, "WorkflowUpdate", mock.Anything)
	mockQueue.AssertNotCalled(t, "EvictAtOnce", mock.Anything, mock.Anything)
}

// TestCascadeSkipBlockedWorkflows_SkippedDepCascades verifies that a workflow
// whose dependency was Skipped (not Failed) is also skipped — Skipped means
// "did not produce a success" and the default semantics are run-on-success.
func TestCascadeSkipBlockedWorkflows_SkippedDepCascades(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	pipeline := &model.Pipeline{ID: 600}

	wfA := &model.Workflow{
		ID:         50,
		PipelineID: 600,
		Name:       "a",
		State:      model.StatusSkipped,
	}
	wfB := &model.Workflow{
		ID:         51,
		PipelineID: 600,
		Name:       "b",
		State:      model.StatusPending,
		DependsOn:  model.JSONSliceString{"a"},
	}

	mockStore := new(store_mocks.MockStore)
	mockStore.On("WorkflowGetTree", pipeline).Return([]*model.Workflow{wfA, wfB}, nil)
	mockStore.On("TaskListPending").Return([]*model.Task{
		{ID: "51", Dependencies: model.JSONSliceString{"50"}},
	}, nil)
	mockStore.On("WorkflowUpdate", mock.MatchedBy(func(wf *model.Workflow) bool {
		return wf.ID == 51 && wf.State == model.StatusSkipped
	})).Return(nil)

	mockQueue := new(queue_mocks.MockQueue)
	mockQueue.On("EvictAtOnce", mock.Anything, []string{"51"}).Return(nil)

	err := CascadeSkipBlockedWorkflows(ctx, mockStore, mockQueue, pipeline)
	assert.NoError(t, err)

	mockStore.AssertExpectations(t)
	mockQueue.AssertExpectations(t)
}

// TestCascadeSkipFromDeclined verifies that when a workflow gate is declined,
// its transitively-dependent workflows are marked Skipped — same as the
// failure cascade but triggered by StatusDeclined.
//
// 3-workflow DAG: build (success) -> gate (declined) -> deploy (pending).
// After the cascade, build is unchanged, gate stays declined, deploy is
// Skipped.
func TestCascadeSkipFromDeclined(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	pipeline := &model.Pipeline{ID: 700}

	buildWF := &model.Workflow{
		ID:         60,
		PipelineID: 700,
		Name:       "build",
		State:      model.StatusSuccess,
	}
	gateWF := &model.Workflow{
		ID:            61,
		PipelineID:    700,
		Name:          "gate",
		State:         model.StatusDeclined,
		ApprovalState: model.ApprovalDeclined,
		DependsOn:     model.JSONSliceString{"build"},
	}
	deployStep := &model.Step{
		ID:    62,
		PID:   61,
		Name:  "deploy-step",
		State: model.StatusPending,
	}
	deployWF := &model.Workflow{
		ID:         62,
		PipelineID: 700,
		Name:       "deploy",
		State:      model.StatusPending,
		DependsOn:  model.JSONSliceString{"gate"},
		Children:   []*model.Step{deployStep},
	}

	mockStore := new(store_mocks.MockStore)
	mockStore.On("GetPipeline", int64(700)).Return(pipeline, nil)
	mockStore.On("WorkflowGetTree", pipeline).Return(
		[]*model.Workflow{buildWF, gateWF, deployWF}, nil)
	mockStore.On("TaskListPending").Return([]*model.Task{
		{ID: "61", Dependencies: model.JSONSliceString{"60"}},
		{ID: "62", Dependencies: model.JSONSliceString{"61"}},
	}, nil)
	mockStore.On("WorkflowUpdate", mock.MatchedBy(func(wf *model.Workflow) bool {
		return wf.ID == 62 && wf.State == model.StatusSkipped
	})).Return(nil)
	mockStore.On("StepUpdate", mock.MatchedBy(func(s *model.Step) bool {
		return s.ID == 62 && s.State == model.StatusSkipped
	})).Return(nil)

	mockQueue := new(queue_mocks.MockQueue)
	mockQueue.On("EvictAtOnce", mock.Anything, []string{"62"}).Return(nil)

	err := cascadeSkipFromDeclined(ctx, mockStore, mockQueue, gateWF)
	assert.NoError(t, err)

	// build was not touched.
	mockStore.AssertNotCalled(t, "WorkflowUpdate", mock.MatchedBy(func(wf *model.Workflow) bool {
		return wf.ID == 60
	}))
	// gate stays declined — cascade must not flip a declined dep to skipped.
	mockStore.AssertNotCalled(t, "WorkflowUpdate", mock.MatchedBy(func(wf *model.Workflow) bool {
		return wf.ID == 61
	}))
	mockStore.AssertExpectations(t)
	mockQueue.AssertExpectations(t)
}

// TestCascadeSkipFromDeclined_RunsOnFailureNotSkipped verifies that a
// workflow with run_on: [failure] is NOT skipped when its upstream gate is
// declined — declined counts as "did not succeed" so failure-only handlers
// should still run.
func TestCascadeSkipFromDeclined_RunsOnFailureNotSkipped(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	pipeline := &model.Pipeline{ID: 800}

	gateWF := &model.Workflow{
		ID:            70,
		PipelineID:    800,
		Name:          "gate",
		State:         model.StatusDeclined,
		ApprovalState: model.ApprovalDeclined,
	}
	notifyWF := &model.Workflow{
		ID:         71,
		PipelineID: 800,
		Name:       "notify",
		State:      model.StatusPending,
		DependsOn:  model.JSONSliceString{"gate"},
	}

	mockStore := new(store_mocks.MockStore)
	mockStore.On("GetPipeline", int64(800)).Return(pipeline, nil)
	mockStore.On("WorkflowGetTree", pipeline).Return(
		[]*model.Workflow{gateWF, notifyWF}, nil)
	mockStore.On("TaskListPending").Return([]*model.Task{
		{
			ID:           "71",
			Dependencies: model.JSONSliceString{"70"},
			RunOn:        model.JSONSliceString{string(model.StatusFailure)},
		},
	}, nil)

	mockQueue := new(queue_mocks.MockQueue)

	err := cascadeSkipFromDeclined(ctx, mockStore, mockQueue, gateWF)
	assert.NoError(t, err)

	// notify should still be allowed to run — no skip cascade.
	mockStore.AssertNotCalled(t, "WorkflowUpdate", mock.Anything)
	mockStore.AssertNotCalled(t, "StepUpdate", mock.Anything)
	mockQueue.AssertNotCalled(t, "EvictAtOnce", mock.Anything, mock.Anything)
}

// TestShouldSkipDueToBlockedDep covers the small predicate directly to lock in
// its semantics independently of orchestration.
func TestShouldSkipDueToBlockedDep(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		parent   *model.Workflow
		runOn    model.JSONSliceString
		expected bool
	}{
		{
			name:     "default runs-on-success and dep failed → skip",
			parent:   &model.Workflow{Name: "parent", State: model.StatusFailure},
			runOn:    nil,
			expected: true,
		},
		{
			name:     "runs-on-failure and dep failed → don't skip",
			parent:   &model.Workflow{Name: "parent", State: model.StatusFailure},
			runOn:    model.JSONSliceString{string(model.StatusFailure)},
			expected: false,
		},
		{
			name:     "runs-on-failure and dep succeeded → skip",
			parent:   &model.Workflow{Name: "parent", State: model.StatusSuccess},
			runOn:    model.JSONSliceString{string(model.StatusFailure)},
			expected: true,
		},
		{
			name:     "runs-on-both → never skip",
			parent:   &model.Workflow{Name: "parent", State: model.StatusFailure},
			runOn:    model.JSONSliceString{string(model.StatusSuccess), string(model.StatusFailure)},
			expected: false,
		},
		{
			name:     "default runs-on-success and dep skipped → skip",
			parent:   &model.Workflow{Name: "parent", State: model.StatusSkipped},
			runOn:    nil,
			expected: true,
		},
		{
			name:     "default runs-on-success and dep still running → don't skip yet",
			parent:   &model.Workflow{Name: "parent", State: model.StatusRunning},
			runOn:    nil,
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			parentMap := map[string]*model.Workflow{"parent": tc.parent}
			task := &model.Task{
				ID:           "99",
				Dependencies: model.JSONSliceString{fmt.Sprint(tc.parent.ID)},
				RunOn:        tc.runOn,
			}
			wf := &model.Workflow{
				ID:        99,
				Name:      "child",
				State:     model.StatusPending,
				DependsOn: model.JSONSliceString{"parent"},
			}
			result := shouldSkipDueToBlockedDep(wf, task, parentMap)
			assert.Equal(t, tc.expected, result)
		})
	}
}
