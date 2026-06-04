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

package crow

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkflowApprove(t *testing.T) {
	t.Parallel()

	t.Run("success with reason", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/api/v1/repos/1/pipelines/123/workflows/2/approve", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			body, _ := io.ReadAll(r.Body)
			var payload struct {
				Reason string `json:"reason"`
			}
			assert.NoError(t, json.Unmarshal(body, &payload))
			assert.Equal(t, "looks good", payload.Reason)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":42,"pid":2,"name":"deploy","state":"running"}`))
		}))
		defer server.Close()

		client := New(server.URL)
		wf, err := client.WorkflowApprove(1, 123, 2, "looks good")
		assert.NoError(t, err)
		assert.Equal(t, int64(42), wf.ID)
		assert.Equal(t, 2, wf.PID)
		assert.Equal(t, "deploy", wf.Name)
	})

	t.Run("success without reason sends no body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/api/v1/repos/5/pipelines/9/workflows/1/approve", r.URL.Path)

			// No body should be sent when reason is empty.
			body, _ := io.ReadAll(r.Body)
			assert.Empty(t, body)
			assert.Empty(t, r.Header.Get("Content-Type"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":7,"pid":1,"name":"x","state":"running"}`))
		}))
		defer server.Close()

		client := New(server.URL)
		wf, err := client.WorkflowApprove(5, 9, 1, "")
		assert.NoError(t, err)
		assert.Equal(t, int64(7), wf.ID)
	})

	t.Run("conflict returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"detail":"approval gate is not pending"}`))
		}))
		defer server.Close()

		client := New(server.URL)
		_, err := client.WorkflowApprove(1, 123, 2, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "approval gate is not pending")
	})
}

func TestWorkflowDecline(t *testing.T) {
	t.Parallel()

	t.Run("success with reason", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/api/v1/repos/1/pipelines/123/workflows/3/decline", r.URL.Path)

			body, _ := io.ReadAll(r.Body)
			var payload struct {
				Reason string `json:"reason"`
			}
			assert.NoError(t, json.Unmarshal(body, &payload))
			assert.Equal(t, "needs review", payload.Reason)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":42,"pid":3,"name":"deploy","state":"skipped"}`))
		}))
		defer server.Close()

		client := New(server.URL)
		wf, err := client.WorkflowDecline(1, 123, 3, "needs review")
		assert.NoError(t, err)
		assert.Equal(t, int64(42), wf.ID)
		assert.Equal(t, "skipped", wf.State)
	})

	t.Run("forbidden returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"detail":"you are not authorized to approve this gate"}`))
		}))
		defer server.Close()

		client := New(server.URL)
		_, err := client.WorkflowDecline(1, 123, 3, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not authorized")
	})
}
