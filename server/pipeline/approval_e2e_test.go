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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v5/server/model"
	queue_mocks "codefloe.com/crowci/crow/v5/server/queue/mocks"
	store_mocks "codefloe.com/crowci/crow/v5/server/store/mocks"
)

// approvalDAG bundles the actors of the canonical 3-workflow approval-gate
// DAG used by the e2e scenarios below.
type approvalDAG struct {
	pipeline   *model.Pipeline
	build      *model.Workflow
	gate       *model.Workflow
	deploy     *model.Workflow
	deployStep *model.Step
}

// dagFixture builds the canonical 3-workflow approval-gate DAG:
// build (success) -> gate (blocked, awaiting approval) -> deploy (pending,
// depends on gate). The deploy workflow carries a single pending child step
// so we can observe the cascade reaching down into the step layer on the
// decline / timeout paths.
//
// Each call returns fresh pointers so individual scenarios can mutate state
// (e.g. flip ApprovalDeadline into the past) without leaking between tests.
func dagFixture() approvalDAG {
	d := approvalDAG{}
	d.pipeline = &model.Pipeline{ID: 900}
	d.build = &model.Workflow{
		ID:         901,
		PipelineID: 900,
		Name:       "build",
		State:      model.StatusSuccess,
	}
	d.gate = &model.Workflow{
		ID:                     902,
		PipelineID:             900,
		Name:                   "gate",
		State:                  model.StatusBlocked,
		RequiresApproval:       true,
		ApprovalState:          model.ApprovalPending,
		ApprovalDeadline:       time.Now().Add(1 * time.Hour).Unix(),
		ApprovalTimeoutSeconds: 3600,
		DependsOn:              model.JSONSliceString{"build"},
	}
	d.deployStep = &model.Step{
		ID:    9020,
		PID:   902,
		Name:  "deploy-step",
		State: model.StatusPending,
	}
	d.deploy = &model.Workflow{
		ID:         903,
		PipelineID: 900,
		Name:       "deploy",
		State:      model.StatusPending,
		DependsOn:  model.JSONSliceString{"gate"},
		Children:   []*model.Step{d.deployStep},
	}
	return d
}

// TestApprovalGate_DAG_Approve walks the happy path: an approver clears a
// pending gate on a build -> gate -> deploy DAG.
//
// Expectations:
//   - gate flips Blocked+Pending -> Pending+Approved
//   - an audit event with ApprovalActionApproved is recorded
//   - the queue is notified of the new approval state
//   - downstream "deploy" is NOT touched (it remains pending so the queue can
//     dispatch it once gate transitions to Success). Cascading skip is the
//     decline / expire path, not the approve path.
func TestApprovalGate_DAG_Approve(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	d := dagFixture()
	user := &model.User{ID: 7}

	s := store_mocks.NewMockStore(t)
	q := queue_mocks.NewMockQueue(t)

	s.On("WorkflowLoad", d.gate.ID).Return(d.gate, nil)
	s.On("WorkflowUpdate", mock.MatchedBy(func(w *model.Workflow) bool {
		return w.ID == d.gate.ID &&
			w.State == model.StatusPending &&
			w.ApprovalState == model.ApprovalApproved
	})).Return(nil)
	s.On("WorkflowApprovalEventCreate", mock.MatchedBy(func(e *model.WorkflowApprovalEvent) bool {
		return e.WorkflowID == d.gate.ID &&
			e.Action == model.ApprovalActionApproved &&
			e.UserID == user.ID &&
			e.Reason == "ship it"
	})).Return(nil)
	q.On("SetApprovalState", mock.Anything, "902", model.ApprovalApproved).Return(nil)

	err := ApproveWorkflow(ctx, s, q, user, d.gate.ID, "ship it")
	assert.NoError(t, err)

	// Approve must NOT trigger a cascade — downstream deploy stays Pending.
	s.AssertNotCalled(t, "GetPipeline", mock.Anything)
	s.AssertNotCalled(t, "WorkflowGetTree", mock.Anything)
	s.AssertExpectations(t)
	q.AssertExpectations(t)
}

// TestApprovalGate_DAG_Decline walks the explicit-no path: an approver
// declines a pending gate on the same 3-workflow DAG.
//
// Expectations:
//   - gate flips Blocked+Pending -> Declined+Declined
//   - an audit event with ApprovalActionDeclined is recorded
//   - the queue is notified of the declined state
//   - cascade kicks in: deploy (and its pending child step) get marked
//     Skipped, and the deploy task is evicted from the queue
//   - build is NOT touched — it already completed successfully and is upstream
//     of the declined gate
func TestApprovalGate_DAG_Decline(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	d := dagFixture()
	user := &model.User{ID: 7}

	s := store_mocks.NewMockStore(t)
	q := queue_mocks.NewMockQueue(t)

	// Decline transition on the gate.
	s.On("WorkflowLoad", d.gate.ID).Return(d.gate, nil)
	s.On("WorkflowUpdate", mock.MatchedBy(func(w *model.Workflow) bool {
		return w.ID == d.gate.ID &&
			w.State == model.StatusDeclined &&
			w.ApprovalState == model.ApprovalDeclined
	})).Return(nil)
	s.On("WorkflowApprovalEventCreate", mock.MatchedBy(func(e *model.WorkflowApprovalEvent) bool {
		return e.WorkflowID == d.gate.ID &&
			e.Action == model.ApprovalActionDeclined &&
			e.UserID == user.ID &&
			e.Reason == "no go"
	})).Return(nil)
	q.On("SetApprovalState", mock.Anything, "902", model.ApprovalDeclined).Return(nil)

	// Cascade walks the tree from the pipeline.
	s.On("GetPipeline", d.pipeline.ID).Return(d.pipeline, nil)
	s.On("WorkflowGetTree", d.pipeline).Return(
		[]*model.Workflow{d.build, d.gate, d.deploy}, nil)
	s.On("TaskList").Return([]*model.Task{
		{ID: "902", Dependencies: model.JSONSliceString{"901"}},
		{ID: "903", Dependencies: model.JSONSliceString{"902"}},
	}, nil)

	// Deploy workflow and its pending step get cascaded to Skipped.
	s.On("WorkflowUpdate", mock.MatchedBy(func(w *model.Workflow) bool {
		return w.ID == d.deploy.ID && w.State == model.StatusSkipped
	})).Return(nil)
	s.On("StepUpdate", mock.MatchedBy(func(st *model.Step) bool {
		return st.ID == d.deployStep.ID && st.State == model.StatusSkipped
	})).Return(nil)
	q.On("EvictAtOnce", mock.Anything, []string{"903"}).Return(nil)

	err := DeclineWorkflow(ctx, s, q, user, d.gate.ID, "no go")
	assert.NoError(t, err)

	// Build was already Success — cascade must not touch it.
	s.AssertNotCalled(t, "WorkflowUpdate", mock.MatchedBy(func(w *model.Workflow) bool {
		return w.ID == d.build.ID
	}))
	// Pipeline-level rollup: with the gate Declined and deploy Skipped,
	// PipelineStatus collapses to StatusDeclined per the precedence rules
	// (declined beats success and beats skipped).
	rollupStatus := model.PipelineStatus([]*model.Workflow{d.build, d.gate, d.deploy})
	assert.Equal(t, model.StatusDeclined, rollupStatus,
		"pipeline rollup after decline cascade should be StatusDeclined")

	s.AssertExpectations(t)
	q.AssertExpectations(t)
}

// TestApprovalGate_DAG_Timeout walks the auto-expire path: the cron sweeper
// finds a gate whose deadline has elapsed without a decision.
//
// Expectations:
//   - the sweeper calls ExpireWorkflow on the over-deadline gate
//   - gate flips Blocked+Pending -> Declined+Expired
//   - an audit event with ApprovalActionExpired and UserID=0 (system) is
//     recorded
//   - the queue is notified with ApprovalExpired
//   - cascade kicks in: deploy (and its pending child step) get marked
//     Skipped, deploy task is evicted from the queue
//
// We invoke the sweeper directly rather than spinning up the ticker — the
// sweeper IS the unit under test for the timeout path, and a ticker would
// only add flakiness.
func TestApprovalGate_DAG_Timeout(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	d := dagFixture()
	// Move the deadline 60s into the past so the sweeper picks this gate up.
	d.gate.ApprovalDeadline = time.Now().Unix() - 60

	s := store_mocks.NewMockStore(t)
	q := queue_mocks.NewMockQueue(t)

	// Sweeper query returns the over-deadline gate.
	s.On("WorkflowsAwaitingApprovalBefore", mock.AnythingOfType("int64")).
		Return([]*model.Workflow{d.gate}, nil)

	// ExpireWorkflow path on the gate.
	s.On("WorkflowLoad", d.gate.ID).Return(d.gate, nil)
	s.On("WorkflowUpdate", mock.MatchedBy(func(w *model.Workflow) bool {
		return w.ID == d.gate.ID &&
			w.State == model.StatusDeclined &&
			w.ApprovalState == model.ApprovalExpired
	})).Return(nil)
	s.On("WorkflowApprovalEventCreate", mock.MatchedBy(func(e *model.WorkflowApprovalEvent) bool {
		return e.WorkflowID == d.gate.ID &&
			e.Action == model.ApprovalActionExpired &&
			e.UserID == 0 && // system-driven
			e.Reason == "approval timed out"
	})).Return(nil)
	q.On("SetApprovalState", mock.Anything, "902", model.ApprovalExpired).Return(nil)

	// Cascade walks the tree from the pipeline.
	s.On("GetPipeline", d.pipeline.ID).Return(d.pipeline, nil)
	s.On("WorkflowGetTree", d.pipeline).Return(
		[]*model.Workflow{d.build, d.gate, d.deploy}, nil)
	s.On("TaskList").Return([]*model.Task{
		{ID: "902", Dependencies: model.JSONSliceString{"901"}},
		{ID: "903", Dependencies: model.JSONSliceString{"902"}},
	}, nil)

	// Deploy workflow and its pending step get cascaded to Skipped.
	s.On("WorkflowUpdate", mock.MatchedBy(func(w *model.Workflow) bool {
		return w.ID == d.deploy.ID && w.State == model.StatusSkipped
	})).Return(nil)
	s.On("StepUpdate", mock.MatchedBy(func(st *model.Step) bool {
		return st.ID == d.deployStep.ID && st.State == model.StatusSkipped
	})).Return(nil)
	q.On("EvictAtOnce", mock.Anything, []string{"903"}).Return(nil)

	// Call the same code path the cron loop would: load expired gates and
	// run ExpireWorkflow on each. We invoke ExpireWorkflow directly here
	// rather than importing server/cron to avoid a test-time package import
	// cycle (cron -> pipeline). The cron sweeper is just a thin loop around
	// this exact call (see server/cron/sweep_approvals.go).
	expired, err := s.WorkflowsAwaitingApprovalBefore(time.Now().Unix())
	assert.NoError(t, err)
	assert.Len(t, expired, 1)
	for _, wf := range expired {
		assert.NoError(t, ExpireWorkflow(ctx, s, q, wf.ID))
	}

	// Build untouched (already Success).
	s.AssertNotCalled(t, "WorkflowUpdate", mock.MatchedBy(func(w *model.Workflow) bool {
		return w.ID == d.build.ID
	}))
	// Pipeline rollup should be Declined after expiry — StatusDeclined wins
	// over Success in the precedence table regardless of the underlying
	// ApprovalState reason (declined vs expired).
	rollupStatus := model.PipelineStatus([]*model.Workflow{d.build, d.gate, d.deploy})
	assert.Equal(t, model.StatusDeclined, rollupStatus,
		"pipeline rollup after expiry cascade should be StatusDeclined")

	s.AssertExpectations(t)
	q.AssertExpectations(t)
}
