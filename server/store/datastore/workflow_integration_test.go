// Copyright 2025 Crow authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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

	"codefloe.com/crowci/crow/v5/server/model"
)

func TestWorkflowPipelineNameIntegration(t *testing.T) {
	// Test that workflows created with PipelineName set correctly don't get overridden
	store, closer := newTestStore(t, new(model.Step), new(model.Pipeline), new(model.Workflow))
	defer closer()

	// Create test pipeline
	pipeline := &model.Pipeline{
		ID:     1,
		RepoID: 1,
		Number: 1,
	}

	// Create workflows with PipelineName already set (simulating new pipelines)
	workflows := []*model.Workflow{
		{
			ID:           1,
			PipelineID:   1,
			PID:          1,
			Name:         "run1",
			PipelineName: "custom-mat", // Already set correctly
			State:        model.StatusSuccess,
		},
		{
			ID:           2,
			PipelineID:   1,
			PID:          2,
			Name:         "run2",
			PipelineName: "custom-mat", // Already set correctly
			State:        model.StatusSuccess,
		},
		{
			ID:           3,
			PipelineID:   1,
			PID:          3,
			Name:         "run3",
			PipelineName: "custom-mat", // Already set correctly
			State:        model.StatusSuccess,
		},
	}

	// Store workflows
	err := store.WorkflowsCreate(workflows)
	assert.NoError(t, err)

	// Test WorkflowGetTree
	result, err := store.WorkflowGetTree(pipeline)
	assert.NoError(t, err)

	// Verify that PipelineName was preserved correctly
	assert.Len(t, result, 3)
	for _, wf := range result {
		assert.Equal(t, "custom-mat", wf.PipelineName, "Workflow %s should have PipelineName preserved as 'custom-mat'", wf.Name)
	}
}
