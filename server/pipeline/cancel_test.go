// Copyright 2025 Woodpecker Authors
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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v5/server"
	forge_mocks "codefloe.com/crowci/crow/v5/server/forge/mocks"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/pubsub"
	"codefloe.com/crowci/crow/v5/server/queue"
	queue_mocks "codefloe.com/crowci/crow/v5/server/queue/mocks"
	store_mocks "codefloe.com/crowci/crow/v5/server/store/mocks"
)

func TestCancelWorkflow(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	repo := &model.Repo{ID: 1, FullName: "test/repo"}
	user := &model.User{ID: 1, Login: "testuser"}

	t.Run("successful cancel running workflow", func(t *testing.T) {
		// Setup
		pipeline := &model.Pipeline{
			ID:     1,
			RepoID: 1,
			Status: model.StatusRunning,
		}

		workflow1 := &model.Workflow{
			ID:         1,
			PipelineID: 1,
			State:      model.StatusRunning,
			Children:   []*model.Step{},
		}

		workflow2 := &model.Workflow{
			ID:         2,
			PipelineID: 1,
			State:      model.StatusPending,
			Children:   []*model.Step{},
		}

		workflows := []*model.Workflow{workflow1, workflow2}

		// Mock store
		mockStore := new(store_mocks.MockStore)
		mockStore.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).Return(workflows, nil)
		mockStore.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).Return(workflows, nil) // Called twice
		mockStore.On("WorkflowUpdate", mock.AnythingOfType("*model.Workflow")).Return(nil)
		mockStore.On("UpdatePipeline", pipeline).Return(nil).Maybe() // May be called if all workflows complete

		// Mock forge
		mockForge := new(forge_mocks.MockForge)

		// Mock queue
		originalQueue := server.Config.Services.Queue
		mockQueue := new(queue_mocks.MockQueue)
		mockQueue.On("Error", ctx, "1", queue.ErrCancel).Return(nil)
		server.Config.Services.Queue = mockQueue
		defer func() { server.Config.Services.Queue = originalQueue }()

		// Mock pubsub
		originalPubsub := server.Config.Services.Pubsub
		server.Config.Services.Pubsub = pubsub.New()
		defer func() { server.Config.Services.Pubsub = originalPubsub }()

		// Execute
		err := CancelWorkflow(ctx, mockForge, mockStore, repo, user, pipeline, 1)

		// Assert
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
		mockQueue.AssertExpectations(t)
	})

	t.Run("successful cancel pending workflow", func(t *testing.T) {
		// Setup
		pipeline := &model.Pipeline{
			ID:     1,
			RepoID: 1,
			Status: model.StatusPending,
		}

		workflow1 := &model.Workflow{
			ID:         1,
			PipelineID: 1,
			State:      model.StatusPending,
			Children:   []*model.Step{},
		}

		workflows := []*model.Workflow{workflow1}

		// Mock store
		mockStore := new(store_mocks.MockStore)
		mockStore.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).Return(workflows, nil)
		mockStore.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).Return(workflows, nil) // Called twice
		mockStore.On("WorkflowUpdate", mock.AnythingOfType("*model.Workflow")).Return(nil)

		// Mock forge
		mockForge := new(forge_mocks.MockForge)

		// Mock queue
		originalQueue := server.Config.Services.Queue
		mockQueue := new(queue_mocks.MockQueue)
		mockQueue.On("Evict", ctx, "1").Return(nil)
		mockQueue.On("Error", ctx, "1", queue.ErrCancel).Return(nil)
		server.Config.Services.Queue = mockQueue
		defer func() { server.Config.Services.Queue = originalQueue }()

		// Mock pubsub
		originalPubsub := server.Config.Services.Pubsub
		server.Config.Services.Pubsub = pubsub.New()
		defer func() { server.Config.Services.Pubsub = originalPubsub }()

		// Execute
		err := CancelWorkflow(ctx, mockForge, mockStore, repo, user, pipeline, 1)

		// Assert
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
		mockQueue.AssertExpectations(t)
	})

	t.Run("workflow not found", func(t *testing.T) {
		// Setup
		pipeline := &model.Pipeline{
			ID:     1,
			RepoID: 1,
			Status: model.StatusRunning,
		}

		workflows := []*model.Workflow{}

		// Mock store
		mockStore := new(store_mocks.MockStore)
		mockStore.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).Return(workflows, nil)

		// Mock forge
		mockForge := new(forge_mocks.MockForge)

		// Execute
		err := CancelWorkflow(ctx, mockForge, mockStore, repo, user, pipeline, 999)

		// Assert
		assert.Error(t, err)
		assert.IsType(t, &ErrNotFound{}, err)
		assert.Contains(t, err.Error(), "Workflow with ID 999 not found in pipeline")
	})

	t.Run("workflow already completed", func(t *testing.T) {
		// Setup
		pipeline := &model.Pipeline{
			ID:     1,
			RepoID: 1,
			Status: model.StatusRunning,
		}

		workflow := &model.Workflow{
			ID:         1,
			PipelineID: 1,
			State:      model.StatusSuccess,
			Children:   []*model.Step{},
		}

		workflows := []*model.Workflow{workflow}

		// Mock store
		mockStore := new(store_mocks.MockStore)
		mockStore.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).Return(workflows, nil)

		// Mock forge
		mockForge := new(forge_mocks.MockForge)

		// Execute
		err := CancelWorkflow(ctx, mockForge, mockStore, repo, user, pipeline, 1)

		// Assert
		assert.Error(t, err)
		assert.IsType(t, &ErrBadRequest{}, err)
		assert.Contains(t, err.Error(), "Cannot cancel a workflow with status success")
	})

	t.Run("non-cancellable workflow on terminal pipeline", func(t *testing.T) {
		// Even with the ghost-workflow allowance for terminal pipelines,
		// we still must not let users cancel a workflow that has already
		// reached a terminal state itself. The workflow-level check is
		// the only safety guard for this.
		pipeline := &model.Pipeline{
			ID:     1,
			RepoID: 1,
			Status: model.StatusSuccess,
		}

		workflow := &model.Workflow{
			ID:         1,
			PipelineID: 1,
			State:      model.StatusSuccess,
			Children:   []*model.Step{},
		}

		mockStore := new(store_mocks.MockStore)
		mockStore.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).Return([]*model.Workflow{workflow}, nil)

		mockForge := new(forge_mocks.MockForge)

		err := CancelWorkflow(ctx, mockForge, mockStore, repo, user, pipeline, 1)

		assert.Error(t, err)
		assert.IsType(t, &ErrBadRequest{}, err)
		assert.Contains(t, err.Error(), "Cannot cancel a workflow with status success")
	})

	t.Run("store error getting workflows", func(t *testing.T) {
		// Setup
		pipeline := &model.Pipeline{
			ID:     1,
			RepoID: 1,
			Status: model.StatusRunning,
		}

		// Mock store
		mockStore := new(store_mocks.MockStore)
		mockStore.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).Return(nil, errors.New("database error"))

		// Mock forge
		mockForge := new(forge_mocks.MockForge)

		// Execute
		err := CancelWorkflow(ctx, mockForge, mockStore, repo, user, pipeline, 1)

		// Assert
		assert.Error(t, err)
		assert.IsType(t, &ErrNotFound{}, err)
		assert.Contains(t, err.Error(), "database error")
	})

	t.Run("cancel workflow with pending steps", func(t *testing.T) {
		// Setup
		pipeline := &model.Pipeline{
			ID:     1,
			RepoID: 1,
			Status: model.StatusPending,
		}

		step := &model.Step{
			ID:         1,
			PipelineID: 1,
			State:      model.StatusPending,
		}

		workflow := &model.Workflow{
			ID:         1,
			PipelineID: 1,
			State:      model.StatusPending,
			Children:   []*model.Step{step},
		}

		workflows := []*model.Workflow{workflow}

		// Mock store
		mockStore := new(store_mocks.MockStore)
		mockStore.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).Return(workflows, nil)
		mockStore.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).Return(workflows, nil) // Called twice
		mockStore.On("WorkflowUpdate", mock.AnythingOfType("*model.Workflow")).Return(nil)
		mockStore.On("StepUpdate", mock.AnythingOfType("*model.Step")).Return(nil)

		// Mock forge
		mockForge := new(forge_mocks.MockForge)

		// Mock queue
		originalQueue := server.Config.Services.Queue
		mockQueue := new(queue_mocks.MockQueue)
		mockQueue.On("Evict", ctx, "1").Return(nil)
		mockQueue.On("Error", ctx, "1", queue.ErrCancel).Return(nil)
		server.Config.Services.Queue = mockQueue
		defer func() { server.Config.Services.Queue = originalQueue }()

		// Mock pubsub
		originalPubsub := server.Config.Services.Pubsub
		server.Config.Services.Pubsub = pubsub.New()
		defer func() { server.Config.Services.Pubsub = originalPubsub }()

		// Execute
		err := CancelWorkflow(ctx, mockForge, mockStore, repo, user, pipeline, 1)

		// Assert
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
		mockQueue.AssertExpectations(t)
	})

	t.Run("update pipeline status when all workflows completed", func(t *testing.T) {
		// Setup
		pipeline := &model.Pipeline{
			ID:     1,
			RepoID: 1,
			Status: model.StatusRunning,
		}

		workflow1 := &model.Workflow{
			ID:         1,
			PipelineID: 1,
			State:      model.StatusRunning,
			Children:   []*model.Step{},
		}

		workflow2 := &model.Workflow{
			ID:         2,
			PipelineID: 1,
			State:      model.StatusSuccess,
			Children:   []*model.Step{},
		}

		// After cancellation, workflow1 becomes killed
		workflow1Killed := &model.Workflow{
			ID:         1,
			PipelineID: 1,
			State:      model.StatusKilled,
			Children:   []*model.Step{},
		}

		workflowsBefore := []*model.Workflow{workflow1, workflow2}
		workflowsAfter := []*model.Workflow{workflow1Killed, workflow2}

		// Mock store
		mockStore := new(store_mocks.MockStore)
		mockStore.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).Return(workflowsBefore, nil)
		mockStore.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).Return(workflowsAfter, nil)
		mockStore.On("WorkflowUpdate", mock.AnythingOfType("*model.Workflow")).Return(nil)
		mockStore.On("UpdatePipeline", pipeline).Return(nil).Maybe() // May be called if all workflows complete

		// Mock forge
		mockForge := new(forge_mocks.MockForge)

		// Mock queue
		originalQueue := server.Config.Services.Queue
		mockQueue := new(queue_mocks.MockQueue)
		mockQueue.On("Error", ctx, "1", queue.ErrCancel).Return(nil)
		server.Config.Services.Queue = mockQueue
		defer func() { server.Config.Services.Queue = originalQueue }()

		// Mock pubsub
		originalPubsub := server.Config.Services.Pubsub
		server.Config.Services.Pubsub = pubsub.New()
		defer func() { server.Config.Services.Pubsub = originalPubsub }()

		// Execute
		err := CancelWorkflow(ctx, mockForge, mockStore, repo, user, pipeline, 1)

		// Assert
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
		mockQueue.AssertExpectations(t)
	})

	t.Run("cancel ghost workflow on already-killed pipeline", func(t *testing.T) {
		// Reproduces the autoscaler scenario: a previous Cancel() (or
		// cancelPreviousPipelines()) marked the pipeline as killed and sent
		// queue.Error to the running workflow, but the autoscaler-provisioned
		// agent never acked the cancel signal (agent died, was reaped during
		// scale-down, etc.). The workflow stays in "running" state in DB while
		// the pipeline is already killed. The UI keeps showing the cancel
		// button (it's gated on workflow state, not pipeline state). The user
		// must be able to clean up the ghost workflow.
		pipeline := &model.Pipeline{
			ID:     1,
			RepoID: 1,
			Status: model.StatusKilled,
		}

		ghostWorkflow := &model.Workflow{
			ID:         1,
			PipelineID: 1,
			State:      model.StatusRunning,
			Children:   []*model.Step{},
		}

		workflows := []*model.Workflow{ghostWorkflow}

		mockStore := new(store_mocks.MockStore)
		mockStore.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).Return(workflows, nil)
		mockStore.On("WorkflowUpdate", mock.AnythingOfType("*model.Workflow")).Return(nil)
		// Pipeline is already in a terminal state, so no UpdatePipeline expected
		// (we don't downgrade or refresh terminal pipeline status).

		mockForge := new(forge_mocks.MockForge)

		originalQueue := server.Config.Services.Queue
		mockQueue := new(queue_mocks.MockQueue)
		mockQueue.On("Error", ctx, "1", queue.ErrCancel).Return(nil)
		server.Config.Services.Queue = mockQueue
		defer func() { server.Config.Services.Queue = originalQueue }()

		originalPubsub := server.Config.Services.Pubsub
		server.Config.Services.Pubsub = pubsub.New()
		defer func() { server.Config.Services.Pubsub = originalPubsub }()

		err := CancelWorkflow(ctx, mockForge, mockStore, repo, user, pipeline, 1)

		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
		mockQueue.AssertExpectations(t)
	})

	t.Run("queue error handling", func(t *testing.T) {
		// Setup
		pipeline := &model.Pipeline{
			ID:     1,
			RepoID: 1,
			Status: model.StatusRunning,
		}

		workflow := &model.Workflow{
			ID:         1,
			PipelineID: 1,
			State:      model.StatusRunning,
			Children:   []*model.Step{},
		}

		workflows := []*model.Workflow{workflow}

		// Mock store
		mockStore := new(store_mocks.MockStore)
		mockStore.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).Return(workflows, nil)
		mockStore.On("WorkflowGetTree", mock.AnythingOfType("*model.Pipeline")).Return(workflows, nil) // Called twice
		mockStore.On("WorkflowUpdate", mock.AnythingOfType("*model.Workflow")).Return(nil)

		// Mock forge
		mockForge := new(forge_mocks.MockForge)

		// Mock queue with error
		originalQueue := server.Config.Services.Queue
		mockQueue := new(queue_mocks.MockQueue)
		mockQueue.On("Error", ctx, "1", queue.ErrCancel).Return(errors.New("queue error"))
		server.Config.Services.Queue = mockQueue
		defer func() { server.Config.Services.Queue = originalQueue }()

		// Mock pubsub
		originalPubsub := server.Config.Services.Pubsub
		server.Config.Services.Pubsub = pubsub.New()
		defer func() { server.Config.Services.Pubsub = originalPubsub }()

		// Execute
		err := CancelWorkflow(ctx, mockForge, mockStore, repo, user, pipeline, 1)

		// Assert - should still succeed despite queue error (errors are logged but don't fail the operation)
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
		mockQueue.AssertExpectations(t)
	})
}
