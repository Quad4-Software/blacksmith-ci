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

// Local Decline / Timeout helpers reuse withApprovalPublishServices, which
// is defined in approval_gate_test.go in this same package.

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
	// Cannot t.Parallel — manipulates server.Config.Services globals.

	ctx := context.Background()
	d := dagFixture()
	user := &model.User{ID: 7}
	d.pipeline.RepoID = 1
	d.pipeline.Status = model.StatusBlocked
	repo := &model.Repo{ID: 1, FullName: "test/repo", UserID: 7}

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

	// refreshAndPublishPipeline runs after approve to refresh SSE clients.
	// It loads the pipeline + repo and walks the tree. After approval the
	// gate flips Blocked -> Pending, deploy stays Pending, build stays
	// Success; PipelineStatus([success, pending, pending]) rolls up to
	// StatusRunning (pending workflows mean the pipeline is still going),
	// which differs from the seed StatusBlocked, so UpdatePipeline IS
	// expected.
	s.On("GetPipeline", d.pipeline.ID).Return(d.pipeline, nil)
	s.On("GetRepo", int64(1)).Return(repo, nil)
	s.On("WorkflowGetTree", d.pipeline).
		Return([]*model.Workflow{d.build, d.gate, d.deploy}, nil)
	s.On("UpdatePipeline", mock.MatchedBy(func(p *model.Pipeline) bool {
		return p.ID == d.pipeline.ID
	})).Return(nil).Maybe()

	mockForge, cleanup := withApprovalPublishServices(t, repo)
	defer cleanup()

	err := ApproveWorkflow(ctx, s, q, user, d.gate.ID, "ship it")
	assert.NoError(t, err)

	// Approve must NOT trigger a cascade walk — TaskList would be the smoking
	// gun, as the skip cascade reads it; refreshAndPublishPipeline does not.
	s.AssertNotCalled(t, "TaskList")
	s.AssertExpectations(t)
	q.AssertExpectations(t)
	mockForge.AssertExpectations(t)
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
	// Cannot t.Parallel — manipulates server.Config.Services globals.

	ctx := context.Background()
	d := dagFixture()
	d.pipeline.RepoID = 1
	d.pipeline.Status = model.StatusBlocked
	user := &model.User{ID: 7}
	repo := &model.Repo{ID: 1, FullName: "test/repo", UserID: 7}

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

	// refreshAndPublishPipeline runs after the cascade. The recomputed rollup
	// is StatusDeclined, which differs from the prior Blocked, so the pipeline
	// status is persisted via UpdatePipeline.
	s.On("GetRepo", int64(1)).Return(repo, nil)
	s.On("UpdatePipeline", mock.MatchedBy(func(p *model.Pipeline) bool {
		return p.ID == d.pipeline.ID && p.Status == model.StatusDeclined
	})).Return(nil)

	mockForge, cleanup := withApprovalPublishServices(t, repo)
	defer cleanup()

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
	mockForge.AssertExpectations(t)
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
	// Cannot t.Parallel — manipulates server.Config.Services globals.

	ctx := context.Background()
	d := dagFixture()
	d.pipeline.RepoID = 1
	d.pipeline.Status = model.StatusBlocked
	// Move the deadline 60s into the past so the sweeper picks this gate up.
	d.gate.ApprovalDeadline = time.Now().Unix() - 60
	repo := &model.Repo{ID: 1, FullName: "test/repo", UserID: 7}
	repoOwner := &model.User{ID: 7, Login: "owner"}

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

	// refreshAndPublishPipeline: load repo, fall back to repo owner because
	// ExpireWorkflow passes a UserID=0 system stub, then publish.
	s.On("GetRepo", int64(1)).Return(repo, nil)
	s.On("GetUser", int64(7)).Return(repoOwner, nil)
	s.On("UpdatePipeline", mock.MatchedBy(func(p *model.Pipeline) bool {
		return p.ID == d.pipeline.ID && p.Status == model.StatusDeclined
	})).Return(nil)

	mockForge, cleanup := withApprovalPublishServices(t, repo)
	defer cleanup()

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
	mockForge.AssertExpectations(t)
}
