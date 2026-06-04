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

package cron

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v5/server/model"
	queue_mocks "codefloe.com/crowci/crow/v5/server/queue/mocks"
	store_mocks "codefloe.com/crowci/crow/v5/server/store/mocks"
)

func TestSweepExpiredApprovalsCallsExpire(t *testing.T) {
	t.Parallel()
	s := store_mocks.NewMockStore(t)
	q := queue_mocks.NewMockQueue(t)

	expired := &model.Workflow{
		ID: 42, RequiresApproval: true,
		ApprovalState:    model.ApprovalPending,
		ApprovalDeadline: time.Now().Unix() - 60,
	}

	s.On("WorkflowsAwaitingApprovalBefore", mock.AnythingOfType("int64")).
		Return([]*model.Workflow{expired}, nil)
	s.On("WorkflowLoad", int64(42)).Return(expired, nil)
	s.On("WorkflowUpdate", mock.MatchedBy(func(w *model.Workflow) bool {
		return w.ApprovalState == model.ApprovalExpired
	})).Return(nil)
	s.On("WorkflowApprovalEventCreate", mock.MatchedBy(func(e *model.WorkflowApprovalEvent) bool {
		return e.Action == model.ApprovalActionExpired && e.UserID == 0
	})).Return(nil)
	q.On("SetApprovalState", mock.Anything, "42", model.ApprovalExpired).Return(nil)

	// ExpireWorkflow → declineWorkflowInternal → cascadeSkipFromDeclined needs
	// GetPipeline + WorkflowGetTree. For a single declined workflow with no
	// dependents, the cascade is a no-op walk — keep these .Maybe() so we don't
	// over-constrain the test.
	s.On("GetPipeline", expired.PipelineID).
		Return(&model.Pipeline{ID: expired.PipelineID}, nil).Maybe()
	s.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).
		Return([]*model.Workflow{expired}, nil).Maybe()
	s.On("TaskList").Return([]*model.Task{}, nil).Maybe()

	SweepExpiredApprovals(context.Background(), s, q)
}
