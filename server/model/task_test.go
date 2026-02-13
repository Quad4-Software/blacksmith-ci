// Copyright 2024 Woodpecker Authors
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

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v5/pipeline"
)

func TestTask_GetLabels(t *testing.T) {
	t.Run("Nil Repo", func(t *testing.T) {
		task := &Task{}
		err := task.ApplyLabelsFromRepo(nil)

		assert.Error(t, err)
		assert.Nil(t, task.Labels)
		assert.EqualError(t, err, "repo is nil but needed to get task labels")
	})

	t.Run("Empty Repo", func(t *testing.T) {
		task := &Task{}
		repo := &Repo{}

		err := task.ApplyLabelsFromRepo(repo)

		assert.NoError(t, err)
		assert.NotNil(t, task.Labels)
		assert.Equal(t, map[string]string{
			pipeline.LabelRepoFullName: "",
			pipeline.LabelOrgID:        "0",
		}, task.Labels)
	})

	t.Run("Empty Labels", func(t *testing.T) {
		task := &Task{}
		repo := &Repo{
			FullName: "test/repo",
			ID:       123,
			OrgID:    456,
		}

		err := task.ApplyLabelsFromRepo(repo)

		assert.NoError(t, err)
		assert.NotNil(t, task.Labels)
		assert.Equal(t, map[string]string{
			pipeline.LabelRepoFullName: "test/repo",
			pipeline.LabelOrgID:        "456",
		}, task.Labels)
	})

	t.Run("Existing Labels", func(t *testing.T) {
		task := &Task{
			Labels: map[string]string{
				"existing": "label",
			},
		}
		repo := &Repo{
			FullName: "test/repo",
			ID:       123,
			OrgID:    456,
		}

		err := task.ApplyLabelsFromRepo(repo)

		assert.NoError(t, err)
		assert.NotNil(t, task.Labels)
		assert.Equal(t, map[string]string{
			"existing":                 "label",
			pipeline.LabelRepoFullName: "test/repo",
			pipeline.LabelOrgID:        "456",
		}, task.Labels)
	})
}

func TestTaskShouldRun(t *testing.T) {
	tests := []struct {
		name         string
		dependencies []string
		depStatus    map[string]StatusValue
		runOn        []string
		expected     bool
	}{
		{
			name:         "no dependencies",
			dependencies: []string{},
			depStatus:    map[string]StatusValue{},
			runOn:        []string{},
			expected:     true,
		},
		{
			name:         "dependencies but empty status - should wait",
			dependencies: []string{"1"},
			depStatus:    map[string]StatusValue{},
			runOn:        []string{},
			expected:     false,
		},
		{
			name:         "dependencies with successful status",
			dependencies: []string{"1"},
			depStatus:    map[string]StatusValue{"1": StatusSuccess},
			runOn:        []string{},
			expected:     true,
		},
		{
			name:         "dependencies with failed status - default runs on success",
			dependencies: []string{"1"},
			depStatus:    map[string]StatusValue{"1": StatusFailure},
			runOn:        []string{},
			expected:     false,
		},
		{
			name:         "multiple dependencies, partial completion - should wait",
			dependencies: []string{"1", "2"},
			depStatus:    map[string]StatusValue{"1": StatusSuccess},
			runOn:        []string{},
			expected:     false,
		},
		{
			name:         "multiple dependencies, all successful",
			dependencies: []string{"1", "2"},
			depStatus:    map[string]StatusValue{"1": StatusSuccess, "2": StatusSuccess},
			runOn:        []string{},
			expected:     true,
		},
		{
			name:         "runs on failure - should wait for deps to complete",
			dependencies: []string{"1"},
			depStatus:    map[string]StatusValue{},
			runOn:        []string{"failure"},
			expected:     false,
		},
		{
			name:         "runs on failure - dependency failed",
			dependencies: []string{"1"},
			depStatus:    map[string]StatusValue{"1": StatusFailure},
			runOn:        []string{"failure"},
			expected:     true,
		},
		{
			name:         "runs on failure - dependency succeeded",
			dependencies: []string{"1"},
			depStatus:    map[string]StatusValue{"1": StatusSuccess},
			runOn:        []string{"failure"},
			expected:     false,
		},
		{
			name:         "runs on both success and failure",
			dependencies: []string{"1"},
			depStatus:    map[string]StatusValue{"1": StatusFailure},
			runOn:        []string{"success", "failure"},
			expected:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			task := &Task{
				ID:           "test",
				Dependencies: test.dependencies,
				DepStatus:    test.depStatus,
				RunOn:        test.runOn,
			}

			result := task.ShouldRun()
			assert.Equal(t, test.expected, result,
				"Dependencies: %v, DepStatus: %v, RunOn: %v",
				test.dependencies, test.depStatus, test.runOn)
		})
	}
}
