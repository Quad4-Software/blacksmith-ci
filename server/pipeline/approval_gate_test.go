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

	"codefloe.com/crowci/crow/v5/server"
	forge_mocks "codefloe.com/crowci/crow/v5/server/forge/mocks"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/pubsub"
	queue_mocks "codefloe.com/crowci/crow/v5/server/queue/mocks"
	manager_mocks "codefloe.com/crowci/crow/v5/server/services/mocks"
	store_mocks "codefloe.com/crowci/crow/v5/server/store/mocks"
)

// withApprovalPublishServices wires the global server.Config.Services slots
// needed by refreshAndPublishPipeline. Returns a cleanup the caller defers.
// The returned forge mock has Status() pre-wired so tests can assert it was
// (or wasn't) called.
func withApprovalPublishServices(t *testing.T, repo *model.Repo) (*forge_mocks.MockForge, func()) {
	t.Helper()
	mockForge := new(forge_mocks.MockForge)
	mockForge.On("Status", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Maybe()
	mockManager := new(manager_mocks.MockManager)
	mockManager.On("ForgeFromRepo", repo).Return(mockForge, nil).Maybe()

	originalManager := server.Config.Services.Manager
	originalPubsub := server.Config.Services.Pubsub
	originalStore := server.Config.Services.Store
	server.Config.Services.Manager = mockManager
	server.Config.Services.Pubsub = pubsub.New()

	cleanup := func() {
		server.Config.Services.Manager = originalManager
		server.Config.Services.Pubsub = originalPubsub
		server.Config.Services.Store = originalStore
	}
	return mockForge, cleanup
}

func TestBlockOnApprovalSetsDeadlineFromNow(t *testing.T) {
	// Cannot t.Parallel — refreshAndPublishPipeline manipulates server.Config.Services globals.
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
			w.ApprovalState == model.ApprovalPending &&
			w.ApprovalDeadline > time.Now().Add(59*time.Minute).Unix() &&
			w.ApprovalDeadline <= time.Now().Add(time.Hour+5*time.Second).Unix()
	})).Return(nil)
	q.On("SetApprovalState", mock.Anything, "42", model.ApprovalPending).Return(nil)

	// refreshAndPublishPipeline path — system-driven (no actor user), falls
	// back to repo owner for the forge auth. Pipeline rollup over a single
	// Blocked workflow is StatusSuccess (Blocked is not Failing/Killed/Declined),
	// which differs from the seed StatusBlocked, so UpdatePipeline fires.
	s.On("GetPipeline", int64(99)).Return(pl, nil)
	s.On("GetRepo", int64(1)).Return(repo, nil)
	s.On("WorkflowGetTree", pl).Return([]*model.Workflow{wf}, nil)
	s.On("GetUser", int64(7)).Return(repoOwner, nil)
	s.On("UpdatePipeline", mock.MatchedBy(func(p *model.Pipeline) bool {
		return p.ID == 99
	})).Return(nil)

	_, cleanup := withApprovalPublishServices(t, repo)
	defer cleanup()

	defaultTimeout := 24 * time.Hour
	err := BlockOnApproval(context.Background(), s, q, 42, defaultTimeout)
	assert.NoError(t, err)
}

func TestBlockOnApprovalUsesDefaultWhenWorkflowHasZero(t *testing.T) {
	// Cannot t.Parallel — refreshAndPublishPipeline manipulates server.Config.Services globals.
	s := store_mocks.NewMockStore(t)
	q := queue_mocks.NewMockQueue(t)
	repo := &model.Repo{ID: 1, FullName: "test/repo", UserID: 7}
	repoOwner := &model.User{ID: 7, Login: "owner"}
	pl := &model.Pipeline{ID: 99, RepoID: 1, Status: model.StatusBlocked}
	wf := &model.Workflow{
		ID: 42, PipelineID: 99, RequiresApproval: true, ApprovalTimeoutSeconds: 0,
	}

	s.On("WorkflowLoad", int64(42)).Return(wf, nil)
	s.On("WorkflowUpdate", mock.MatchedBy(func(w *model.Workflow) bool {
		want := time.Now().Add(24 * time.Hour).Unix()
		return w.ApprovalDeadline >= want-5 && w.ApprovalDeadline <= want+5
	})).Return(nil)
	q.On("SetApprovalState", mock.Anything, "42", model.ApprovalPending).Return(nil)

	// refreshAndPublishPipeline path — same fallback to repo owner.
	s.On("GetPipeline", int64(99)).Return(pl, nil)
	s.On("GetRepo", int64(1)).Return(repo, nil)
	s.On("WorkflowGetTree", pl).Return([]*model.Workflow{wf}, nil)
	s.On("GetUser", int64(7)).Return(repoOwner, nil)
	s.On("UpdatePipeline", mock.MatchedBy(func(p *model.Pipeline) bool {
		return p.ID == 99
	})).Return(nil)

	_, cleanup := withApprovalPublishServices(t, repo)
	defer cleanup()

	defaultTimeout := 24 * time.Hour
	err := BlockOnApproval(context.Background(), s, q, 42, defaultTimeout)
	assert.NoError(t, err)
}

func TestApproveWorkflowTransitionsAndUnblocks(t *testing.T) {
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
		return w.State == model.StatusPending && w.ApprovalState == model.ApprovalApproved
	})).Return(nil)
	s.On("WorkflowApprovalEventCreate", mock.MatchedBy(func(e *model.WorkflowApprovalEvent) bool {
		return e.WorkflowID == 42 && e.Action == model.ApprovalActionApproved &&
			e.UserID == 7 && e.Reason == "lgtm"
	})).Return(nil)
	q.On("SetApprovalState", mock.Anything, "42", model.ApprovalApproved).Return(nil)

	// refreshAndPublishPipeline reloads + republishes the parent pipeline so
	// SSE clients see the approval immediately. The rollup for a lone
	// just-approved gate is Success (no Failing workflows), which differs
	// from the prior Blocked status, so UpdatePipeline is expected.
	s.On("GetPipeline", int64(99)).Return(pl, nil)
	s.On("GetRepo", int64(1)).Return(repo, nil)
	s.On("WorkflowGetTree", pl).Return([]*model.Workflow{wf}, nil)
	s.On("UpdatePipeline", mock.MatchedBy(func(p *model.Pipeline) bool {
		return p.ID == 99
	})).Return(nil)

	mockForge, cleanup := withApprovalPublishServices(t, repo)
	defer cleanup()

	err := ApproveWorkflow(context.Background(), s, q, user, 42, "lgtm")
	assert.NoError(t, err)
	mockForge.AssertExpectations(t)
}

func TestApproveWorkflowRejectsNonPending(t *testing.T) {
	t.Parallel()
	s := store_mocks.NewMockStore(t)
	q := queue_mocks.NewMockQueue(t)
	user := &model.User{ID: 7}
	wf := &model.Workflow{
		ID: 42, State: model.StatusBlocked,
		ApprovalState: model.ApprovalApproved,
	}

	s.On("WorkflowLoad", int64(42)).Return(wf, nil)

	err := ApproveWorkflow(context.Background(), s, q, user, 42, "")
	assert.ErrorIs(t, err, ErrApprovalNotPending)
}

func TestDeclineWorkflowMarksDeclined(t *testing.T) {
	// Cannot t.Parallel — manipulates server.Config.Services globals.
	s := store_mocks.NewMockStore(t)
	q := queue_mocks.NewMockQueue(t)
	user := &model.User{ID: 7}
	repo := &model.Repo{ID: 1, FullName: "test/repo", UserID: 7}
	pipeline := &model.Pipeline{ID: 99, RepoID: 1, Status: model.StatusBlocked}
	wf := &model.Workflow{
		ID: 42, PipelineID: 99, State: model.StatusBlocked,
		ApprovalState: model.ApprovalPending,
	}

	s.On("WorkflowLoad", int64(42)).Return(wf, nil)
	s.On("WorkflowUpdate", mock.MatchedBy(func(w *model.Workflow) bool {
		return w.State == model.StatusDeclined && w.ApprovalState == model.ApprovalDeclined
	})).Return(nil)
	s.On("WorkflowApprovalEventCreate", mock.MatchedBy(func(e *model.WorkflowApprovalEvent) bool {
		return e.Action == model.ApprovalActionDeclined
	})).Return(nil)
	q.On("SetApprovalState", mock.Anything, "42", model.ApprovalDeclined).Return(nil)
	// cascadeSkipFromDeclined loads the parent pipeline and walks the tree.
	// refreshAndPublishPipeline does the same — both reuse GetPipeline +
	// WorkflowGetTree which are idempotent reads, so a single .On(...) is fine.
	s.On("GetPipeline", int64(99)).Return(pipeline, nil)
	s.On("WorkflowGetTree", pipeline).Return([]*model.Workflow{wf}, nil)
	s.On("TaskList").Return([]*model.Task{}, nil)
	// refreshAndPublishPipeline picks up the now-declined gate and rolls the
	// pipeline status to Declined.
	s.On("GetRepo", int64(1)).Return(repo, nil)
	s.On("UpdatePipeline", mock.MatchedBy(func(p *model.Pipeline) bool {
		return p.ID == 99 && p.Status == model.StatusDeclined
	})).Return(nil)

	mockForge, cleanup := withApprovalPublishServices(t, repo)
	defer cleanup()

	err := DeclineWorkflow(context.Background(), s, q, user, 42, "no go")
	assert.NoError(t, err)
	mockForge.AssertExpectations(t)
}
