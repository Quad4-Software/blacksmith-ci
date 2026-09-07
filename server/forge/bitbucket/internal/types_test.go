// Copyright 2026 Crow Authors
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

package internal

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestErrorMessage covers the reporting side of issue #1487: a rejected request
// used to surface as a bare "Bad request" because Bitbucket's validation detail
// was decoded away.
func TestErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		expected string
	}{
		{
			name:     "message only",
			status:   http.StatusBadRequest,
			body:     `{"type":"error","error":{"message":"Bad request"}}`,
			expected: "Bad request",
		},
		{
			name:     "message and detail",
			status:   http.StatusBadRequest,
			body:     `{"type":"error","error":{"message":"Bad request","detail":"url is not publicly reachable"}}`,
			expected: "Bad request: url is not publicly reachable",
		},
		{
			name:   "field validation errors are sorted and kept",
			status: http.StatusBadRequest,
			body: `{"type":"error","error":{"message":"Bad request","fields":` +
				`{"url":["This field may not be blank."],"events":["Unknown event: repo:pushed"]}}}`,
			expected: "Bad request (events: Unknown event: repo:pushed; url: This field may not be blank.)",
		},
		{
			name:     "empty body falls back to the status text",
			status:   http.StatusNotFound,
			body:     "",
			expected: "Not Found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Error{Status: tt.status}
			if tt.body != "" {
				require.NoError(t, json.Unmarshal([]byte(tt.body), &err))
			}
			assert.Equal(t, tt.expected, err.Error())
		})
	}
}
