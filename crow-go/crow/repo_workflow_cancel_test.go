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

package crow

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkflowCancel(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		repoID         int64
		pipelineNumber int64
		workflowID     int64
		statusCode     int
		responseBody   string
		expectedError  error
	}{
		{
			name:           "successful cancel workflow",
			repoID:         1,
			pipelineNumber: 123,
			workflowID:     456,
			statusCode:     http.StatusNoContent,
			responseBody:   "",
			expectedError:  nil,
		},
		{
			name:           "workflow not found",
			repoID:         1,
			pipelineNumber: 123,
			workflowID:     999,
			statusCode:     http.StatusNotFound,
			responseBody:   `{"error": "Workflow with ID 999 not found in pipeline"}`,
			expectedError:  errors.New("Workflow with ID 999 not found in pipeline"),
		},
		{
			name:           "pipeline not found",
			repoID:         1,
			pipelineNumber: 999,
			workflowID:     456,
			statusCode:     http.StatusNotFound,
			responseBody:   `{"error": "Pipeline not found"}`,
			expectedError:  errors.New("Pipeline not found"),
		},
		{
			name:           "permission denied",
			repoID:         1,
			pipelineNumber: 123,
			workflowID:     456,
			statusCode:     http.StatusForbidden,
			responseBody:   `{"error": "Permission denied"}`,
			expectedError:  errors.New("Permission denied"),
		},
		{
			name:           "server error",
			repoID:         1,
			pipelineNumber: 123,
			workflowID:     456,
			statusCode:     http.StatusInternalServerError,
			responseBody:   `{"error": "Internal server error"}`,
			expectedError:  errors.New("Internal server error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and path
				assert.Equal(t, "POST", r.Method)
				expectedPath := "/api/v1/repos/1/pipelines/123/workflows/456/cancel"
				if tc.pipelineNumber == 999 {
					expectedPath = "/api/v1/repos/1/pipelines/999/workflows/456/cancel"
				}
				if tc.workflowID == 999 {
					expectedPath = "/api/v1/repos/1/pipelines/123/workflows/999/cancel"
				}
				assert.Equal(t, expectedPath, r.URL.Path)

				// Set response
				w.WriteHeader(tc.statusCode)
				if tc.responseBody != "" {
					_, err := w.Write([]byte(tc.responseBody))
					if err != nil {
						t.Fatalf("failed to write response: %v", err)
					}
				}
			}))
			defer server.Close()

			// Create client
			client := New(server.URL)

			// Execute
			err := client.WorkflowCancel(tc.repoID, tc.pipelineNumber, tc.workflowID)

			// Assert
			if tc.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWorkflowCancelWithDifferentIDs(t *testing.T) {
	t.Parallel()

	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/repos/42/pipelines/789/workflows/101/cancel", r.URL.Path)

		// Set response
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	// Create client
	client := New(server.URL)

	// Execute with different IDs
	err := client.WorkflowCancel(42, 789, 101)

	// Assert
	assert.NoError(t, err)
}

func TestWorkflowCancelNetworkError(t *testing.T) {
	t.Parallel()

	// Create client with invalid URL
	client := New("http://invalid-url-that-does-not-exist")

	// Execute
	err := client.WorkflowCancel(1, 123, 456)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no such host")
}
