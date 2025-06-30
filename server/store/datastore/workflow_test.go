// Copyright 2022 Woodpecker Authors
// Copyright 2018 Drone.IO Inc.
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

package datastore

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/crowci/crow/v3/server/model"
)

func TestWorkflowLoad(t *testing.T) {
	store, closer := newTestStore(t, new(model.Step), new(model.Pipeline), new(model.Workflow))
	defer closer()

	wf := &model.Workflow{
		PipelineID: 1,
		PID:        1,
		Name:       "woodpecker",
		Children: []*model.Step{
			{
				UUID:       "ea6d4008-8ace-4f8a-ad03-53f1756465d9",
				PipelineID: 1,
				PID:        2,
				PPID:       1,
				State:      "success",
			},
			{
				UUID:       "2bf387f7-2913-4907-814c-c9ada88707c0",
				PipelineID: 1,
				PID:        3,
				PPID:       1,
				Name:       "build",
				State:      "success",
			},
		},
	}
	assert.NoError(t, store.WorkflowsCreate([]*model.Workflow{wf}))
	workflowGet, err := store.WorkflowLoad(1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, workflowGet.PipelineID)
	assert.Equal(t, 1, workflowGet.PID)
	assert.Len(t, workflowGet.Children, 0)
}

func TestWorkflowGetTree(t *testing.T) {
	store, closer := newTestStore(t, new(model.Step), new(model.Pipeline), new(model.Workflow))
	defer closer()

	wf := &model.Workflow{
		PipelineID: 1,
		PID:        1,
		Name:       "woodpecker",
		Children: []*model.Step{
			{
				UUID:       "ea6d4008-8ace-4f8a-ad03-53f1756465d9",
				PipelineID: 1,
				PID:        2,
				PPID:       1,
				State:      "success",
			},
			{
				UUID:       "2bf387f7-2913-4907-814c-c9ada88707c0",
				PipelineID: 1,
				PID:        3,
				PPID:       1,
				Name:       "build",
				State:      "success",
			},
		},
	}
	assert.NoError(t, store.WorkflowsCreate([]*model.Workflow{wf}))

	workflowsGet, err := store.WorkflowGetTree(&model.Pipeline{ID: 1})
	assert.NoError(t, err)
	assert.Len(t, workflowsGet, 1)
	workflowGet := workflowsGet[0]
	assert.Equal(t, "woodpecker", workflowGet.Name)
	assert.Len(t, workflowGet.Children, 2)
	assert.Equal(t, 2, workflowGet.Children[0].PID)
	assert.Equal(t, 3, workflowGet.Children[1].PID)
}

func TestWorkflowUpdate(t *testing.T) {
	store, closer := newTestStore(t, new(model.Step), new(model.Pipeline), new(model.Workflow))
	defer closer()

	wf := &model.Workflow{
		PipelineID: 1,
		PID:        1,
		Name:       "woodpecker",
		State:      "pending",
	}
	assert.NoError(t, store.WorkflowsCreate([]*model.Workflow{wf}))
	workflowGet, err := store.WorkflowLoad(1)
	assert.NoError(t, err)

	assert.Equal(t, model.StatusValue("pending"), workflowGet.State)

	wf.State = "success"

	assert.NoError(t, store.WorkflowUpdate(wf))
	workflowGet, err = store.WorkflowLoad(1)
	assert.NoError(t, err)
	assert.Equal(t, model.StatusValue("success"), workflowGet.State)
}

func TestCleanupOrphanedWorkflows(t *testing.T) {
	t.Run("empty workflow IDs", func(t *testing.T) {
		store, closer := newTestStore(t, new(model.Step), new(model.Pipeline), new(model.Workflow))
		defer closer()
		err := store.CleanupOrphanedWorkflows([]string{})
		assert.NoError(t, err)
	})

	t.Run("invalid workflow IDs", func(t *testing.T) {
		store, closer := newTestStore(t, new(model.Step), new(model.Pipeline), new(model.Workflow))
		defer closer()
		err := store.CleanupOrphanedWorkflows([]string{"invalid", "also-invalid"})
		assert.NoError(t, err)
	})

	t.Run("mix of valid and invalid IDs", func(t *testing.T) {
		store, closer := newTestStore(t, new(model.Step), new(model.Pipeline), new(model.Workflow))
		defer closer()
		// Create a workflow in the database
		wf := &model.Workflow{
			PipelineID: 1,
			PID:        1,
			Name:       "test-workflow",
		}
		assert.NoError(t, store.WorkflowsCreate([]*model.Workflow{wf}))

		// Test with mix of existing and non-existing workflow IDs
		workflowIDs := []string{"1", "999", "invalid", "2"}
		err := store.CleanupOrphanedWorkflows(workflowIDs)
		assert.NoError(t, err)
	})

	t.Run("all existing workflows", func(t *testing.T) {
		store, closer := newTestStore(t, new(model.Step), new(model.Pipeline), new(model.Workflow))
		defer closer()
		// Create multiple workflows
		workflows := []*model.Workflow{
			{
				PipelineID: 1,
				PID:        1,
				Name:       "workflow-1",
			},
			{
				PipelineID: 1,
				PID:        2,
				Name:       "workflow-2",
			},
		}
		assert.NoError(t, store.WorkflowsCreate(workflows))

		// Test with all existing workflow IDs
		workflowIDs := []string{"1", "2"}
		err := store.CleanupOrphanedWorkflows(workflowIDs)
		assert.NoError(t, err)
	})

	t.Run("all non-existing workflows", func(t *testing.T) {
		store, closer := newTestStore(t, new(model.Step), new(model.Pipeline), new(model.Workflow))
		defer closer()
		// Test with all non-existing workflow IDs
		workflowIDs := []string{"999", "1000", "1001"}
		err := store.CleanupOrphanedWorkflows(workflowIDs)
		assert.NoError(t, err)
	})
}
