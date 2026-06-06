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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v5/pipeline/frontend/yaml"
	"codefloe.com/crowci/crow/v5/pipeline/frontend/yaml/matrix"
	"codefloe.com/crowci/crow/v5/server/model"
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

func TestWorkflowListByAgent(t *testing.T) {
	store, closer := newTestStore(t, new(model.Pipeline), new(model.Workflow))
	defer closer()

	ctx := context.Background()
	p1 := &model.Pipeline{RepoID: 1, Number: 7}
	p2 := &model.Pipeline{RepoID: 1, Number: 8}
	_, err := store.db.NewInsert().Model(p1).Exec(ctx)
	assert.NoError(t, err)
	_, err = store.db.NewInsert().Model(p2).Exec(ctx)
	assert.NoError(t, err)

	wfs := []*model.Workflow{
		{PipelineID: p1.ID, PID: 1, Name: "older", AgentID: 1, Started: 100, Finished: 150, State: model.StatusSuccess},
		{PipelineID: p2.ID, PID: 1, Name: "newer", AgentID: 1, Started: 200, Finished: 250, State: model.StatusFailure},
		{PipelineID: p1.ID, PID: 2, Name: "other-agent", AgentID: 2, Started: 100, Finished: 300, State: model.StatusSuccess},
	}
	assert.NoError(t, store.WorkflowsCreate(wfs))

	// Only agent 1's workflows, most recently finished first, enriched with repo + number.
	got, err := store.WorkflowListByAgent(1, &model.ListOptions{All: true})
	assert.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, "newer", got[0].Name)
	assert.Equal(t, "older", got[1].Name)
	assert.EqualValues(t, 1, got[0].RepoID)
	assert.EqualValues(t, 8, got[0].PipelineNumber)
	assert.EqualValues(t, 7, got[1].PipelineNumber)

	// Pagination splits the two results across pages.
	page1, err := store.WorkflowListByAgent(1, &model.ListOptions{Page: 1, PerPage: 1})
	assert.NoError(t, err)
	assert.Len(t, page1, 1)
	assert.Equal(t, "newer", page1[0].Name)

	page2, err := store.WorkflowListByAgent(1, &model.ListOptions{Page: 2, PerPage: 1})
	assert.NoError(t, err)
	assert.Len(t, page2, 1)
	assert.Equal(t, "older", page2[0].Name)
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

func TestMatrixWorkflowNameMapping(t *testing.T) {
	// Test just the matrix parsing logic without database complexity
	testCases := []struct {
		name        string
		configName  string
		yamlContent string
		expectedMap map[string]string
		description string
	}{
		{
			name:       "User's specific case - mat2.yaml with custom-mat name",
			configName: "mat2.yaml",
			yamlContent: `name: custom-mat
matrix:
  include:
    - NAME: run1
    - NAME: run2
    - NAME: run3
steps:
  - name: test
    commands:
      - echo "testing"`,
			expectedMap: map[string]string{
				"run1": "custom-mat",
				"run2": "custom-mat",
				"run3": "custom-mat",
			},
			description: "User's specific case: mat2.yaml with name: custom-mat should group matrix workflows under custom-mat",
		},
		{
			name:       "Pipeline with name field and matrix",
			configName: "custom-mat.yml",
			yamlContent: `name: custom-mat
matrix:
  include:
    - NAME: run1
    - NAME: run2
    - NAME: run3
steps:
  - name: test
    commands:
      - echo "testing"`,
			expectedMap: map[string]string{
				"run1": "custom-mat",
				"run2": "custom-mat",
				"run3": "custom-mat",
			},
			description: "Pipeline with explicit name field should group matrix workflows under the pipeline name",
		},
		{
			name:       "Pipeline without name field",
			configName: "mat.yml",
			yamlContent: `matrix:
  include:
    - NAME: run1
    - NAME: run2
    - NAME: run3
steps:
  - name: test
    commands:
      - echo "testing"`,
			expectedMap: map[string]string{
				"run1": "mat",
				"run2": "mat",
				"run3": "mat",
			},
			description: "Pipeline without name field should use filename for grouping",
		},
		{
			name:       "Single workflow with name field",
			configName: "single.yml",
			yamlContent: `name: myname
steps:
  - name: test
    commands:
      - echo "testing"`,
			expectedMap: map[string]string{
				"myname": "myname",
			},
			description: "Single workflow with name field should map to itself",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test the matrix parsing logic directly
			result := testGetPipelineNameMapping(tc.configName, tc.yamlContent)
			assert.Equal(t, tc.expectedMap, result, tc.description)
		})
	}
}

// Helper function to test the pipeline name mapping logic.
func testGetPipelineNameMapping(configName, yamlContent string) map[string]string {
	pipelineNameMap := make(map[string]string)

	// Parse the YAML config to extract the pipeline name
	parsed, err := yaml.ParseString(yamlContent)
	if err != nil {
		return pipelineNameMap
	}

	// Determine the pipeline name (from YAML name field or filename)
	var pipelineName string
	if parsed.Name != "" {
		pipelineName = parsed.Name
	} else {
		// Fallback to sanitized filename
		pipelineName = sanitizePath(configName)
	}

	// Parse matrix from YAML data
	axes, err := matrix.ParseString(yamlContent)
	if err != nil {
		return pipelineNameMap
	}

	// If no matrix axes, create a default empty axis
	if len(axes) == 0 {
		axes = append(axes, matrix.Axis{})
	}

	// Map each matrix axis to the pipeline name
	for _, axis := range axes {
		// Check for matrix NAME (case-insensitive)
		matrixName, nameKey, hasMatrixName := getCaseInsensitiveKey(axis, "NAME")
		if hasMatrixName {
			delete(axis, nameKey) // Remove NAME from axis
		}

		// Determine workflow name using the same logic as stepBuilder.go
		var workflowName string
		switch {
		case hasMatrixName && matrixName != "":
			workflowName = matrixName
		case parsed.Name != "":
			workflowName = parsed.Name
		default:
			workflowName = sanitizePath(configName)
		}

		pipelineNameMap[workflowName] = pipelineName
	}

	return pipelineNameMap
}

// Helper functions are now available from workflow.go.

func TestWorkflowGetTreePipelineNamePopulation(t *testing.T) {
	// Test the actual WorkflowGetTree function with fallback logic
	store, closer := newTestStore(t, new(model.Step), new(model.Pipeline), new(model.Workflow))
	defer closer()

	// Create test pipeline
	pipeline := &model.Pipeline{
		ID:     1,
		RepoID: 1,
		Number: 1,
	}

	// Create workflows without pipeline_name field (simulating existing data)
	workflows := []*model.Workflow{
		{
			ID:           1,
			PipelineID:   1,
			PID:          1,
			Name:         "run1",
			PipelineName: "", // Empty - should be populated by fallback
			State:        model.StatusSuccess,
		},
		{
			ID:           2,
			PipelineID:   1,
			PID:          2,
			Name:         "run2",
			PipelineName: "", // Empty - should be populated by fallback
			State:        model.StatusSuccess,
		},
		{
			ID:           3,
			PipelineID:   1,
			PID:          3,
			Name:         "run3",
			PipelineName: "", // Empty - should be populated by fallback
			State:        model.StatusSuccess,
		},
	}

	// Store workflows
	err := store.WorkflowsCreate(workflows)
	assert.NoError(t, err)

	// Test WorkflowGetTree - this should trigger the fallback logic
	result, err := store.WorkflowGetTree(pipeline)
	assert.NoError(t, err)

	// Verify that pipeline_name was populated by fallback logic
	assert.Len(t, result, 3)
	for _, wf := range result {
		assert.NotEmpty(t, wf.PipelineName, "Workflow %s should have pipeline_name populated by fallback", wf.Name)
		// With the simplified fallback logic, each workflow uses its own name
		assert.Equal(t, wf.Name, wf.PipelineName, "Workflow %s should use its own name as pipeline_name", wf.Name)
	}
}

func TestCreateFallbackPipelineNameMap(t *testing.T) {
	store := storage{}

	testCases := []struct {
		name        string
		workflows   []*model.Workflow
		expectedMap map[string]string
		description string
	}{
		{
			name: "Matrix workflows without pipeline_name",
			workflows: []*model.Workflow{
				{Name: "run1"},
				{Name: "run2"},
				{Name: "run3"},
			},
			expectedMap: map[string]string{
				"run1": "run1",
				"run2": "run2",
				"run3": "run3",
			},
			description: "Workflows without pipeline_name should use their own names",
		},
		{
			name: "Workflows without config name fallback to own name",
			workflows: []*model.Workflow{
				{Name: "custom-mat-1"},
				{Name: "custom-mat-2"},
				{Name: "custom-mat-3"},
			},
			expectedMap: map[string]string{
				"custom-mat-1": "custom-mat-1",
				"custom-mat-2": "custom-mat-2",
				"custom-mat-3": "custom-mat-3",
			},
			description: "Workflows without config name should use their own name as pipeline_name",
		},
		{
			name: "Single workflow",
			workflows: []*model.Workflow{
				{Name: "single-workflow"},
			},
			expectedMap: map[string]string{
				"single-workflow": "single-workflow",
			},
			description: "Single workflow should map to itself",
		},
		{
			name: "Mixed workflows",
			workflows: []*model.Workflow{
				{Name: "run1"},
				{Name: "run2"},
				{Name: "single-workflow"},
				{Name: "test-workflow"},
			},
			expectedMap: map[string]string{
				"run1":            "run1",
				"run2":            "run2",
				"single-workflow": "single-workflow",
				"test-workflow":   "test-workflow",
			},
			description: "Each workflow should use its own name as pipeline_name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := store.createFallbackPipelineNameMap(tc.workflows)
			assert.Equal(t, tc.expectedMap, result, tc.description)
		})
	}
}

func TestSanitizePath(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"test.yml", "test"},
		{"test.yaml", "test"},
		{".hidden.yml", "hidden"},
		{"path/to/file.yml", "file"},
		{"custom-mat.yml", "custom-mat"},
		{"simple", "simple"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := sanitizePath(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
