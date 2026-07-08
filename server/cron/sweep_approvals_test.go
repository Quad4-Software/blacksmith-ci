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

	"codefloe.com/crowci/crow/v6/server"
	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pubsub"
	queue_mocks "codefloe.com/crowci/crow/v6/server/queue/mocks"
	manager_mocks "codefloe.com/crowci/crow/v6/server/services/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func TestSweepExpiredApprovalsCallsExpire(t *testing.T) {
	// Cannot t.Parallel — manipulates server.Config.Services globals.
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
	// ExpireWorkflow now skips the expired gate's own pending child steps
	// via StepListFromWorkflowFind + UpdateStepToStatusSkipped (step layer
	// of the decline cascade). The bare-minimum fixture in this test has no
	// children, so we expect an empty list and no StepUpdate calls.
	s.On("StepListFromWorkflowFind", expired).Return([]*model.Step{}, nil)
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
	s.On("TaskListPending").Return([]*model.Task{}, nil).Maybe()
	// After cascade, refreshAndPublishPipeline re-loads the pipeline + repo,
	// recomputes the rollup status and republishes to SSE clients. For a
	// system-driven expire (UserID=0) it also looks up the repo owner for
	// forge auth. All reads are .Maybe() because this sweeper test only
	// asserts that ExpireWorkflow ran end-to-end without error.
	s.On("GetRepo", mock.AnythingOfType("int64")).
		Return(&model.Repo{ID: 1, FullName: "test/repo", UserID: 7}, nil).Maybe()
	s.On("GetUser", mock.AnythingOfType("int64")).
		Return(&model.User{ID: 7, Login: "owner"}, nil).Maybe()
	s.On("UpdatePipeline", mock.AnythingOfType("*model.Pipeline")).
		Return(nil).Maybe()

	// Wire the bare minimum global services that refreshAndPublishPipeline
	// reaches into: a Manager that returns a forge mock, and a pubsub.
	mockForge := new(forge_mocks.MockForge)
	mockForge.On("Status", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Maybe()
	mockManager := new(manager_mocks.MockManager)
	mockManager.On("ForgeFromRepo", mock.AnythingOfType("*model.Repo")).
		Return(mockForge, nil).Maybe()

	originalManager := server.Config.Services.Manager
	originalPubsub := server.Config.Services.Pubsub
	server.Config.Services.Manager = mockManager
	server.Config.Services.Pubsub = pubsub.New()
	defer func() {
		server.Config.Services.Manager = originalManager
		server.Config.Services.Pubsub = originalPubsub
	}()

	SweepExpiredApprovals(context.Background(), s, q)
}
