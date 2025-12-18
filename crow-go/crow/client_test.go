// Copyright 2022 Woodpecker Authors
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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_LogLevel(t *testing.T) {
	logLevel := "warn"
	fixtureHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var ll LogLevel
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&ll)) {
				return
			}
			logLevel = ll.Level
		}

		_, err := fmt.Fprintf(w, `{
			"log-level": "%s"
	}`, logLevel)
		assert.NoError(t, err)
	}

	ts := httptest.NewServer(http.HandlerFunc(fixtureHandler))
	defer ts.Close()

	client := NewClient(ts.URL, http.DefaultClient)

	curLvl, err := client.LogLevel()
	assert.NoError(t, err)
	assert.True(t, strings.EqualFold(curLvl.Level, logLevel))

	newLvl, err := client.SetLogLevel(&LogLevel{Level: "trace"})
	assert.NoError(t, err)
	assert.True(t, strings.EqualFold(newLvl.Level, logLevel))
}

// TestHTMLResponseError tests that when the server returns HTML instead of JSON,
// the client provides a clear error message instead of a confusing JSON parse error.
// This reproduces the issue where CLI commands fail with:
// "invalid character '<' looking for beginning of value".
func TestHTMLResponseError(t *testing.T) {
	htmlResponse := `<!DOCTYPE html>
<html>
<head><title>Crow CI</title></head>
<body><div id="app"></div></body>
</html>`

	tests := []struct {
		name          string
		statusCode    int
		contentType   string
		responseBody  string
		expectError   bool
		errorContains string
	}{
		{
			name:          "HTML with 200 OK (NoRoute handler serving SPA)",
			statusCode:    http.StatusOK,
			contentType:   "text/html; charset=utf-8",
			responseBody:  htmlResponse,
			expectError:   true,
			errorContains: "HTML",
		},
		{
			name:          "HTML with 404 Not Found",
			statusCode:    http.StatusNotFound,
			contentType:   "text/html",
			responseBody:  htmlResponse,
			expectError:   true,
			errorContains: "404",
		},
		{
			name:          "HTML with 401 Unauthorized",
			statusCode:    http.StatusUnauthorized,
			contentType:   "text/html",
			responseBody:  htmlResponse,
			expectError:   true,
			errorContains: "401",
		},
		{
			name:          "valid JSON response",
			statusCode:    http.StatusOK,
			contentType:   "application/json",
			responseBody:  `{"id":1,"login":"test"}`,
			expectError:   false,
			errorContains: "",
		},
		{
			name:          "JSON error response",
			statusCode:    http.StatusBadRequest,
			contentType:   "application/json",
			responseBody:  `{"error":"bad request"}`,
			expectError:   true,
			errorContains: "400",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := New(server.URL)
			user, err := client.Self()

			if tt.expectError {
				assert.Error(t, err, "Expected an error for test case: %s", tt.name)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains,
						"Error message should contain '%s', got: %s", tt.errorContains, err.Error())
				}
				// The error should NOT contain the raw JSON parse error for HTML responses
				if strings.HasPrefix(tt.contentType, "text/html") {
					assert.NotContains(t, err.Error(), "invalid character '<'",
						"Error should not expose raw JSON parse error")
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, user)
				assert.NotEmpty(t, user.Login, "User login should not be empty")
			}
		})
	}
}

// TestPipelineCreateHTMLResponse specifically tests the pipeline create scenario
// that was reported in the bug:
// crow pipeline create --var ARCH=amd64 --branch=main 5
// error: "invalid character '<' looking for beginning of value"
func TestPipelineCreateHTMLResponse(t *testing.T) {
	htmlResponse := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Crow CI</title>
</head>
<body>
  <div id="app"></div>
  <script src="/assets/index.js"></script>
</body>
</html>`

	tests := []struct {
		name          string
		statusCode    int
		contentType   string
		responseBody  string
		expectError   bool
		errorContains string
	}{
		{
			name:          "server returns HTML instead of JSON (route mismatch)",
			statusCode:    http.StatusOK,
			contentType:   "text/html; charset=utf-8",
			responseBody:  htmlResponse,
			expectError:   true,
			errorContains: "HTML",
		},
		{
			name:          "server returns valid pipeline JSON",
			statusCode:    http.StatusOK,
			contentType:   "application/json",
			responseBody:  `{"id":123,"number":1,"status":"pending","branch":"main"}`,
			expectError:   false,
			errorContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify the request looks correct
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Contains(t, r.URL.Path, "/pipelines")

				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := New(server.URL)
			options := &PipelineOptions{
				Branch:    "main",
				Variables: map[string]string{"ARCH": "amd64"},
			}
			pipeline, err := client.PipelineCreate(5, options)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, pipeline)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains,
						"Error message should contain '%s', got: %s", tt.errorContains, err.Error())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pipeline)
				assert.Equal(t, int64(123), pipeline.ID)
			}
		})
	}
}
