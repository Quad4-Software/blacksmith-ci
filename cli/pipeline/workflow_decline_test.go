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

	"codefloe.com/crowci/crow/v5/crow-go/crow"
	crow_mocks "codefloe.com/crowci/crow/v5/crow-go/crow/mocks"
)

func TestWorkflowDecline_CLI(t *testing.T) {
	t.Run("success without reason", func(t *testing.T) {
		mockClient := new(crow_mocks.MockClient)
		mockClient.On("RepoLookup", "owner/repo").Return(&crow.Repo{ID: 1}, nil)
		mockClient.On("Pipeline", int64(1), int64(123)).Return(&crow.Pipeline{
			Workflows: []*crow.Workflow{{PID: 1, Name: "deploy"}},
		}, nil)
		mockClient.On("WorkflowDecline", int64(1), int64(123), 1, "").Return(&crow.Workflow{PID: 1}, nil)

		err := workflowDeclineWithClient(mockClient, "owner/repo", "123", "deploy", "")
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("success with reason", func(t *testing.T) {
		mockClient := new(crow_mocks.MockClient)
		mockClient.On("RepoLookup", "owner/repo").Return(&crow.Repo{ID: 1}, nil)
		mockClient.On("Pipeline", int64(1), int64(123)).Return(&crow.Pipeline{
			Workflows: []*crow.Workflow{{PID: 1, Name: "deploy"}},
		}, nil)
		mockClient.On("WorkflowDecline", int64(1), int64(123), 1, "needs more review").Return(&crow.Workflow{PID: 1}, nil)

		err := workflowDeclineWithClient(mockClient, "owner/repo", "123", "deploy", "needs more review")
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("decline call error", func(t *testing.T) {
		mockClient := new(crow_mocks.MockClient)
		mockClient.On("RepoLookup", "owner/repo").Return(&crow.Repo{ID: 1}, nil)
		mockClient.On("Pipeline", int64(1), int64(123)).Return(&crow.Pipeline{
			Workflows: []*crow.Workflow{{PID: 1, Name: "deploy"}},
		}, nil)
		mockClient.On("WorkflowDecline", int64(1), int64(123), 1, "").Return((*crow.Workflow)(nil), errors.New("conflict"))

		err := workflowDeclineWithClient(mockClient, "owner/repo", "123", "deploy", "")
		assert.ErrorContains(t, err, "conflict")
		mockClient.AssertExpectations(t)
	})

	t.Run("workflow not found", func(t *testing.T) {
		mockClient := new(crow_mocks.MockClient)
		mockClient.On("RepoLookup", "owner/repo").Return(&crow.Repo{ID: 1}, nil)
		mockClient.On("Pipeline", int64(1), int64(123)).Return(&crow.Pipeline{
			Workflows: []*crow.Workflow{{PID: 1, Name: "build"}},
		}, nil)

		err := workflowDeclineWithClient(mockClient, "owner/repo", "123", "missing", "")
		assert.ErrorContains(t, err, "workflow \"missing\" not found")
		mockClient.AssertExpectations(t)
	})
}
