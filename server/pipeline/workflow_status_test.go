package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codeberg.org/crowci/crow/v5/pipeline/rpc"
	"codeberg.org/crowci/crow/v5/server/model"
	store_mocks "codeberg.org/crowci/crow/v5/server/store/mocks"
)

func TestUpdateWorkflowStatusToDonePreservesKilledState(t *testing.T) {
	t.Parallel()

	// Test that a workflow marked as killed stays killed even when UpdateWorkflowStatusToDone is called
	workflow := model.Workflow{
		ID:    1,
		State: model.StatusKilled, // Already marked as killed
	}

	state := rpc.WorkflowState{
		Started:  1000,
		Finished: 2000,
		Error:    "some error", // This would normally set state to failure
	}

	mockStore := new(store_mocks.MockStore)
	mockStore.On("WorkflowUpdate", mock.AnythingOfType("*model.Workflow")).Return(nil)

	result, err := UpdateWorkflowStatusToDone(mockStore, workflow, state)

	assert.NoError(t, err)
	assert.Equal(t, model.StatusKilled, result.State) // Should remain killed
	assert.Equal(t, int64(2000), result.Finished)
	assert.Equal(t, "some error", result.Error)
	mockStore.AssertExpectations(t)
}

func TestUpdateWorkflowStatusToDoneNormalFlow(t *testing.T) {
	t.Parallel()

	// Test normal flow when workflow is not killed
	workflow := model.Workflow{
		ID:    1,
		State: model.StatusRunning,
	}

	state := rpc.WorkflowState{
		Started:  1000,
		Finished: 2000,
		Error:    "some error",
	}

	mockStore := new(store_mocks.MockStore)
	mockStore.On("WorkflowUpdate", mock.AnythingOfType("*model.Workflow")).Return(nil)

	result, err := UpdateWorkflowStatusToDone(mockStore, workflow, state)

	assert.NoError(t, err)
	assert.Equal(t, model.StatusFailure, result.State) // Should be set to failure due to error
	assert.Equal(t, int64(2000), result.Finished)
	assert.Equal(t, "some error", result.Error)
	mockStore.AssertExpectations(t)
}

func TestUpdateWorkflowStatusToDoneNoError(t *testing.T) {
	t.Parallel()

	// Test when there's no error
	workflow := model.Workflow{
		ID:    1,
		State: model.StatusRunning,
	}

	state := rpc.WorkflowState{
		Started:  1000,
		Finished: 2000,
		Error:    "", // No error
	}

	mockStore := new(store_mocks.MockStore)
	mockStore.On("WorkflowUpdate", mock.AnythingOfType("*model.Workflow")).Return(nil)

	result, err := UpdateWorkflowStatusToDone(mockStore, workflow, state)

	assert.NoError(t, err)
	assert.Equal(t, model.StatusSuccess, result.State) // Should be set to success
	assert.Equal(t, int64(2000), result.Finished)
	assert.Equal(t, "", result.Error)
	mockStore.AssertExpectations(t)
}
