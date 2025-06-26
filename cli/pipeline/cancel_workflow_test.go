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

	"codeberg.org/crowci/crow/v3/crow-go/crow"
	crow_mocks "codeberg.org/crowci/crow/v3/crow-go/crow/mocks"
)

func TestCancelWorkflow_CLI(t *testing.T) {
	t.Run("success with repo full name", func(t *testing.T) {
		mockClient := new(crow_mocks.MockClient)
		mockClient.On("RepoLookup", "owner/repo").Return(&crow.Repo{ID: 1}, nil)
		mockClient.On("WorkflowCancel", int64(1), int64(123), int64(456)).Return(nil)

		err := cancelWorkflowWithClient(mockClient, "owner/repo", "123", "456")
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("repo lookup error", func(t *testing.T) {
		mockClient := new(crow_mocks.MockClient)
		mockClient.On("RepoLookup", "bad/repo").Return(nil, errors.New("repo not found"))

		err := cancelWorkflowWithClient(mockClient, "bad/repo", "123", "456")
		assert.ErrorContains(t, err, "repo not found")
		mockClient.AssertExpectations(t)
	})

	t.Run("workflow cancel error", func(t *testing.T) {
		mockClient := new(crow_mocks.MockClient)
		mockClient.On("RepoLookup", "owner/repo").Return(&crow.Repo{ID: 1}, nil)
		mockClient.On("WorkflowCancel", int64(1), int64(123), int64(456)).Return(errors.New("workflow not found"))

		err := cancelWorkflowWithClient(mockClient, "owner/repo", "123", "456")
		assert.ErrorContains(t, err, "workflow not found")
		mockClient.AssertExpectations(t)
	})

	t.Run("invalid pipeline number", func(t *testing.T) {
		mockClient := new(crow_mocks.MockClient)
		mockClient.On("RepoLookup", "owner/repo").Return(&crow.Repo{ID: 1}, nil)

		err := cancelWorkflowWithClient(mockClient, "owner/repo", "invalid", "456")
		assert.ErrorContains(t, err, "invalid syntax")
		mockClient.AssertExpectations(t)
	})

	t.Run("invalid workflow ID", func(t *testing.T) {
		mockClient := new(crow_mocks.MockClient)
		mockClient.On("RepoLookup", "owner/repo").Return(&crow.Repo{ID: 1}, nil)

		err := cancelWorkflowWithClient(mockClient, "owner/repo", "123", "invalid")
		assert.ErrorContains(t, err, "invalid syntax")
		mockClient.AssertExpectations(t)
	})

	t.Run("missing arguments", func(t *testing.T) {
		mockClient := new(crow_mocks.MockClient)
		mockClient.On("RepoLookup", "owner/repo").Return(&crow.Repo{ID: 1}, nil)

		// Simulate missing workflow ID by passing empty string
		err := cancelWorkflowWithClient(mockClient, "owner/repo", "123", "")
		assert.ErrorContains(t, err, "invalid syntax")
		mockClient.AssertExpectations(t)
	})
}

func TestCancelWorkflow_CommandTreeDebug(t *testing.T) {
	cmd := Command
	for _, c := range cmd.Commands {
		t.Logf("subcommand: %s", c.Name)
	}
}
