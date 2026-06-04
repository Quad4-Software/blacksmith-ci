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

func TestBlockOnApprovalSetsDeadlineFromNow(t *testing.T) {
	t.Parallel()
	s := store_mocks.NewMockStore(t)
	q := queue_mocks.NewMockQueue(t)
	wf := &model.Workflow{ID: 42, RequiresApproval: true, ApprovalTimeoutSeconds: 3600}

	s.On("WorkflowLoad", int64(42)).Return(wf, nil)
	s.On("WorkflowUpdate", mock.MatchedBy(func(w *model.Workflow) bool {
		return w.State == model.StatusBlocked &&
			w.ApprovalState == model.ApprovalPending &&
			w.ApprovalDeadline > time.Now().Add(59*time.Minute).Unix() &&
			w.ApprovalDeadline <= time.Now().Add(time.Hour+5*time.Second).Unix()
	})).Return(nil)
	q.On("SetApprovalState", mock.Anything, "42", model.ApprovalPending).Return(nil)

	defaultTimeout := 24 * time.Hour
	err := BlockOnApproval(context.Background(), s, q, 42, defaultTimeout)
	assert.NoError(t, err)
}

func TestBlockOnApprovalUsesDefaultWhenWorkflowHasZero(t *testing.T) {
	t.Parallel()
	s := store_mocks.NewMockStore(t)
	q := queue_mocks.NewMockQueue(t)
	wf := &model.Workflow{ID: 42, RequiresApproval: true, ApprovalTimeoutSeconds: 0}

	s.On("WorkflowLoad", int64(42)).Return(wf, nil)
	s.On("WorkflowUpdate", mock.MatchedBy(func(w *model.Workflow) bool {
		want := time.Now().Add(24 * time.Hour).Unix()
		return w.ApprovalDeadline >= want-5 && w.ApprovalDeadline <= want+5
	})).Return(nil)
	q.On("SetApprovalState", mock.Anything, "42", model.ApprovalPending).Return(nil)

	defaultTimeout := 24 * time.Hour
	err := BlockOnApproval(context.Background(), s, q, 42, defaultTimeout)
	assert.NoError(t, err)
}

func TestApproveWorkflowTransitionsAndUnblocks(t *testing.T) {
	t.Parallel()
	s := store_mocks.NewMockStore(t)
	q := queue_mocks.NewMockQueue(t)
	user := &model.User{ID: 7}
	wf := &model.Workflow{
		ID: 42, State: model.StatusBlocked,
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

	err := ApproveWorkflow(context.Background(), s, q, user, 42, "lgtm")
	assert.NoError(t, err)
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
	t.Parallel()
	s := store_mocks.NewMockStore(t)
	q := queue_mocks.NewMockQueue(t)
	user := &model.User{ID: 7}
	pipeline := &model.Pipeline{ID: 99}
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
	// No downstream pending workflows here, so no further updates expected.
	s.On("GetPipeline", int64(99)).Return(pipeline, nil)
	s.On("WorkflowGetTree", pipeline).Return([]*model.Workflow{wf}, nil)
	s.On("TaskList").Return([]*model.Task{}, nil)

	err := DeclineWorkflow(context.Background(), s, q, user, 42, "no go")
	assert.NoError(t, err)
}
