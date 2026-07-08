// Copyright 2026 Crow CI Authors
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

	"codefloe.com/crowci/crow/v6/crow-go/crow"
	crow_mocks "codefloe.com/crowci/crow/v6/crow-go/crow/mocks"
)

func TestWorkflowApprove_CLI(t *testing.T) {
	t.Run("success without reason", func(t *testing.T) {
		mockClient := new(crow_mocks.MockClient)
		mockClient.On("RepoLookup", "owner/repo").Return(&crow.Repo{ID: 1}, nil)
		mockClient.On("Pipeline", int64(1), int64(123)).Return(&crow.Pipeline{
			Workflows: []*crow.Workflow{
				{PID: 1, Name: "build"},
				{PID: 2, Name: "deploy"},
			},
		}, nil)
		mockClient.On("WorkflowApprove", int64(1), int64(123), 2, "").Return(&crow.Workflow{PID: 2}, nil)

		err := workflowApproveWithClient(mockClient, "owner/repo", "123", "deploy", "")
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("success with reason", func(t *testing.T) {
		mockClient := new(crow_mocks.MockClient)
		mockClient.On("RepoLookup", "owner/repo").Return(&crow.Repo{ID: 1}, nil)
		mockClient.On("Pipeline", int64(1), int64(123)).Return(&crow.Pipeline{
			Workflows: []*crow.Workflow{{PID: 1, Name: "deploy"}},
		}, nil)
		mockClient.On("WorkflowApprove", int64(1), int64(123), 1, "looks good").Return(&crow.Workflow{PID: 1}, nil)

		err := workflowApproveWithClient(mockClient, "owner/repo", "123", "deploy", "looks good")
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("workflow not found", func(t *testing.T) {
		mockClient := new(crow_mocks.MockClient)
		mockClient.On("RepoLookup", "owner/repo").Return(&crow.Repo{ID: 1}, nil)
		mockClient.On("Pipeline", int64(1), int64(123)).Return(&crow.Pipeline{
			Workflows: []*crow.Workflow{{PID: 1, Name: "build"}},
		}, nil)

		err := workflowApproveWithClient(mockClient, "owner/repo", "123", "missing", "")
		assert.ErrorContains(t, err, "workflow \"missing\" not found")
		mockClient.AssertExpectations(t)
	})

	t.Run("repo lookup error", func(t *testing.T) {
		mockClient := new(crow_mocks.MockClient)
		mockClient.On("RepoLookup", "bad/repo").Return(nil, errors.New("repo not found"))

		err := workflowApproveWithClient(mockClient, "bad/repo", "123", "deploy", "")
		assert.ErrorContains(t, err, "repo not found")
		mockClient.AssertExpectations(t)
	})

	t.Run("invalid pipeline number", func(t *testing.T) {
		mockClient := new(crow_mocks.MockClient)
		mockClient.On("RepoLookup", "owner/repo").Return(&crow.Repo{ID: 1}, nil)

		err := workflowApproveWithClient(mockClient, "owner/repo", "not-a-number", "deploy", "")
		assert.ErrorContains(t, err, "invalid syntax")
		mockClient.AssertExpectations(t)
	})

	t.Run("approve call error", func(t *testing.T) {
		mockClient := new(crow_mocks.MockClient)
		mockClient.On("RepoLookup", "owner/repo").Return(&crow.Repo{ID: 1}, nil)
		mockClient.On("Pipeline", int64(1), int64(123)).Return(&crow.Pipeline{
			Workflows: []*crow.Workflow{{PID: 1, Name: "deploy"}},
		}, nil)
		mockClient.On("WorkflowApprove", int64(1), int64(123), 1, "").Return((*crow.Workflow)(nil), errors.New("forbidden"))

		err := workflowApproveWithClient(mockClient, "owner/repo", "123", "deploy", "")
		assert.ErrorContains(t, err, "forbidden")
		mockClient.AssertExpectations(t)
	})

	t.Run("empty workflow name", func(t *testing.T) {
		mockClient := new(crow_mocks.MockClient)

		err := workflowApproveWithClient(mockClient, "owner/repo", "123", "", "")
		assert.ErrorContains(t, err, "workflow name is required")
		mockClient.AssertExpectations(t)
	})
}
