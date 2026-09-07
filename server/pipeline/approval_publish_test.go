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

	"codefloe.com/crowci/crow/v6/server/model"
	queue_mocks "codefloe.com/crowci/crow/v6/server/queue/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

// TestApproveWorkflowPublishesPipelineUpdate verifies that after a successful
// approve transition the parent pipeline is reloaded, its rollup status is
// recomputed, and the forge commit status push is invoked — which is the
// observable side effect of publishPipeline. This is the SSE-update path:
// without it, an approval UI sits stale until something else republishes.
func TestApproveWorkflowPublishesPipelineUpdate(t *testing.T) {
	// Cannot t.Parallel — manipulates server.Config.Services globals.
	s := store_mocks.NewMockStore(t)
	q := queue_mocks.NewMockQueue(t)

	user := &model.User{ID: 7}
	repo := &model.Repo{ID: 1, FullName: "test/repo", UserID: 7}
	pl := &model.Pipeline{ID: 99, RepoID: 1, Status: model.StatusBlocked}
	wf := &model.Workflow{
		ID: 42, PipelineID: 99, State: model.StatusBlocked,
		ApprovalState: model.ApprovalPending,
	}

	s.On("WorkflowLoad", int64(42)).Return(wf, nil)
	s.On("WorkflowUpdate", mock.MatchedBy(func(w *model.Workflow) bool {
		return w.ApprovalState == model.ApprovalApproved
	})).Return(nil)
	s.On("WorkflowApprovalEventCreate", mock.AnythingOfType("*model.WorkflowApprovalEvent")).
		Return(nil)
	q.On("SetApprovalState", mock.Anything, "42", model.ApprovalApproved).Return(nil)

	// refreshAndPublishPipeline path: reload + recompute + persist + publish.
	s.On("GetPipeline", int64(99)).Return(pl, nil)
	s.On("GetRepo", int64(1)).Return(repo, nil)
	s.On("WorkflowGetTree", pl).Return([]*model.Workflow{wf}, nil)
	s.On("UpdatePipeline", mock.MatchedBy(func(p *model.Pipeline) bool {
		// Pipeline rollup with a lone approved-but-not-yet-run gate is Success
		// (no Failing() workflows). The pipeline was previously Blocked, so a
		// status change is persisted.
		return p.ID == 99
	})).Return(nil)

	mockForge, cleanup := withApprovalPublishServices(t, repo)
	defer cleanup()
	// Assert publishPipeline reaches the forge commit-status push — this is the
	// downstream effect that proves we went all the way through.
	mockForge.On("Status", mock.Anything, user, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	err := ApproveWorkflow(context.Background(), s, q, user, 42, "lgtm")
	assert.NoError(t, err)

	s.AssertExpectations(t)
	q.AssertExpectations(t)
	// Specifically prove the forge.Status() call happened — that's the visible
	// signal that publishPipeline ran end-to-end.
	mockForge.AssertCalled(t, "Status", mock.Anything, user,
		mock.Anything, mock.Anything, mock.Anything)
}

// TestDeclineWorkflowPublishesPipelineUpdate is the decline twin of the above:
// after the cascade runs we still go through refreshAndPublishPipeline, the
// pipeline status flips to Declined, and the SSE/forge update fires.
func TestDeclineWorkflowPublishesPipelineUpdate(t *testing.T) {
	// Cannot t.Parallel — manipulates server.Config.Services globals.
	s := store_mocks.NewMockStore(t)
	q := queue_mocks.NewMockQueue(t)

	user := &model.User{ID: 7}
	repo := &model.Repo{ID: 1, FullName: "test/repo", UserID: 7}
	pl := &model.Pipeline{ID: 99, RepoID: 1, Status: model.StatusBlocked}
	wf := &model.Workflow{
		ID: 42, PipelineID: 99, State: model.StatusBlocked,
		ApprovalState: model.ApprovalPending,
	}

	s.On("WorkflowLoad", int64(42)).Return(wf, nil)
	s.On("WorkflowUpdate", mock.MatchedBy(func(w *model.Workflow) bool {
		return w.State == model.StatusDeclined &&
			w.ApprovalState == model.ApprovalDeclined
	})).Return(nil)
	// Declined gate has no children in this fixture.
	s.On("StepListFromWorkflowFind", wf).Return([]*model.Step{}, nil)
	s.On("WorkflowApprovalEventCreate", mock.AnythingOfType("*model.WorkflowApprovalEvent")).
		Return(nil)
	q.On("SetApprovalState", mock.Anything, "42", model.ApprovalDeclined).Return(nil)
	q.On("Evict", mock.Anything, "42").Return(nil)

	// Cascade walks the tree from the pipeline (no dependents in this fixture
	// so it's a no-op after the lookup).
	s.On("GetPipeline", int64(99)).Return(pl, nil)
	s.On("WorkflowGetTree", pl).Return([]*model.Workflow{wf}, nil)
	s.On("TaskListPending").Return([]*model.Task{}, nil)

	// refreshAndPublishPipeline: pipeline rollup now collapses to Declined.
	s.On("GetRepo", int64(1)).Return(repo, nil)
	s.On("UpdatePipeline", mock.MatchedBy(func(p *model.Pipeline) bool {
		return p.ID == 99 && p.Status == model.StatusDeclined
	})).Return(nil)

	mockForge, cleanup := withApprovalPublishServices(t, repo)
	defer cleanup()
	mockForge.On("Status", mock.Anything, user, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	err := DeclineWorkflow(context.Background(), s, q, user, 42, "no go")
	assert.NoError(t, err)

	s.AssertExpectations(t)
	q.AssertExpectations(t)
	mockForge.AssertCalled(t, "Status", mock.Anything, user,
		mock.Anything, mock.Anything, mock.Anything)
}

// TestExpireWorkflowFallsBackToRepoOwnerForForgeAuth verifies the
// system-driven path: the timeout sweeper calls ExpireWorkflow with an empty
// system user (ID=0). The refreshAndPublishPipeline helper must look up the
// repo owner to authenticate the forge.Status() push, mirroring the same
// idiom used by other system-driven publishes in the codebase.
func TestExpireWorkflowFallsBackToRepoOwnerForForgeAuth(t *testing.T) {
	// Cannot t.Parallel — manipulates server.Config.Services globals.
	s := store_mocks.NewMockStore(t)
	q := queue_mocks.NewMockQueue(t)

	repo := &model.Repo{ID: 1, FullName: "test/repo", UserID: 7}
	repoOwner := &model.User{ID: 7, Login: "owner"}
	pl := &model.Pipeline{ID: 99, RepoID: 1, Status: model.StatusBlocked}
	wf := &model.Workflow{
		ID: 42, PipelineID: 99, State: model.StatusBlocked,
		ApprovalState: model.ApprovalPending,
	}

	s.On("WorkflowLoad", int64(42)).Return(wf, nil)
	s.On("WorkflowUpdate", mock.MatchedBy(func(w *model.Workflow) bool {
		return w.ApprovalState == model.ApprovalExpired
	})).Return(nil)
	// Expired gate has no children in this fixture.
	s.On("StepListFromWorkflowFind", wf).Return([]*model.Step{}, nil)
	s.On("WorkflowApprovalEventCreate", mock.MatchedBy(func(e *model.WorkflowApprovalEvent) bool {
		// System-driven — UserID stays 0.
		return e.UserID == 0 && e.Action == model.ApprovalActionExpired
	})).Return(nil)
	q.On("SetApprovalState", mock.Anything, "42", model.ApprovalExpired).Return(nil)
	q.On("Evict", mock.Anything, "42").Return(nil)

	// Cascade tree load.
	s.On("GetPipeline", int64(99)).Return(pl, nil)
	s.On("WorkflowGetTree", pl).Return([]*model.Workflow{wf}, nil)
	s.On("TaskListPending").Return([]*model.Task{}, nil)

	// refreshAndPublishPipeline.
	s.On("GetRepo", int64(1)).Return(repo, nil)
	// CRITICAL: with the actor.ID == 0 we must resolve a forge-auth user via
	// the repo owner lookup.
	s.On("GetUser", int64(7)).Return(repoOwner, nil)
	s.On("UpdatePipeline", mock.MatchedBy(func(p *model.Pipeline) bool {
		return p.ID == 99 && p.Status == model.StatusDeclined
	})).Return(nil)

	mockForge, cleanup := withApprovalPublishServices(t, repo)
	defer cleanup()
	// forge.Status receives the repo owner, not the zero-ID system stub.
	mockForge.On("Status", mock.Anything, repoOwner, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	err := ExpireWorkflow(context.Background(), s, q, 42)
	assert.NoError(t, err)

	s.AssertExpectations(t)
	q.AssertExpectations(t)
	mockForge.AssertCalled(t, "Status", mock.Anything, repoOwner,
		mock.Anything, mock.Anything, mock.Anything)
}

// TestBlockOnApprovalPublishesPipelineUpdate is the block-activation twin of
// the approve/decline/expire publish tests. When the queue activates a gate
// (upstream deps cleared), the DB row flips to Blocked + Pending and the
// in-flight SSE topic must see the change immediately — without it, the
// approval panel needs a full page reload to appear. The action is
// system-driven, so the helper falls back to the repo owner for forge auth.
func TestBlockOnApprovalPublishesPipelineUpdate(t *testing.T) {
	// Cannot t.Parallel — manipulates server.Config.Services globals.
	s := store_mocks.NewMockStore(t)
	q := queue_mocks.NewMockQueue(t)

	repo := &model.Repo{ID: 1, FullName: "test/repo", UserID: 7}
	repoOwner := &model.User{ID: 7, Login: "owner"}
	pl := &model.Pipeline{ID: 99, RepoID: 1, Status: model.StatusBlocked}
	wf := &model.Workflow{
		ID: 42, PipelineID: 99, RequiresApproval: true, ApprovalTimeoutSeconds: 3600,
	}

	s.On("WorkflowLoad", int64(42)).Return(wf, nil)
	s.On("WorkflowUpdate", mock.MatchedBy(func(w *model.Workflow) bool {
		return w.State == model.StatusBlocked &&
			w.ApprovalState == model.ApprovalPending
	})).Return(nil)
	q.On("SetApprovalState", mock.Anything, "42", model.ApprovalPending).Return(nil)

	// refreshAndPublishPipeline path: reload + recompute + (maybe) persist + publish.
	s.On("GetPipeline", int64(99)).Return(pl, nil)
	s.On("GetRepo", int64(1)).Return(repo, nil)
	s.On("WorkflowGetTree", pl).Return([]*model.Workflow{wf}, nil)
	// System-driven activation — actor is nil, so the helper looks up the
	// repo owner to authenticate the forge.Status() push.
	s.On("GetUser", int64(7)).Return(repoOwner, nil)
	// The pipeline rollup over a single Blocked workflow is StatusBlocked,
	// matching the seed StatusBlocked, so UpdatePipeline is not called —
	// publish-only. This is the visible signal the gate-activation publish
	// path is now consistent with the rollup precedence.

	mockForge, cleanup := withApprovalPublishServices(t, repo)
	defer cleanup()
	// forge.Status receives the repo owner, not a nil actor.
	mockForge.On("Status", mock.Anything, repoOwner, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	defaultTimeout := 24 * time.Hour
	err := BlockOnApproval(context.Background(), s, q, 42, defaultTimeout)
	assert.NoError(t, err)

	s.AssertExpectations(t)
	q.AssertExpectations(t)
	// Specifically prove the forge.Status() call happened — that's the visible
	// signal that publishPipeline ran end-to-end after gate activation.
	mockForge.AssertCalled(t, "Status", mock.Anything, repoOwner,
		mock.Anything, mock.Anything, mock.Anything)
}
