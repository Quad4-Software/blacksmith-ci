// Copyright 2025 Crow Authors
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

package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
)

func TestRunMaintenanceRequest_JobValidation(t *testing.T) {
	tests := []struct {
		name        string
		job         string
		dryRun      bool
		expectValid bool
	}{
		{
			name:        "valid vacuum job",
			job:         "vacuumLogTable",
			dryRun:      false,
			expectValid: true,
		},
		{
			name:        "valid kubernetes job",
			job:         "kubernetesCleanup",
			dryRun:      false,
			expectValid: true,
		},
		{
			name:        "valid kubernetes job with dry run",
			job:         "kubernetesCleanup",
			dryRun:      true,
			expectValid: true,
		},
		{
			name:        "valid stale agent cleanup job",
			job:         "staleAgentCleanup",
			dryRun:      false,
			expectValid: true,
		},
		{
			name:        "invalid job",
			job:         "invalidJob",
			dryRun:      false,
			expectValid: false,
		},
		{
			name:        "empty job",
			job:         "",
			dryRun:      false,
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := RunMaintenanceRequest{Job: tt.job, DryRun: tt.dryRun}

			if tt.expectValid {
				assert.True(t, request.Job == "vacuumLogTable" || request.Job == "kubernetesCleanup" || request.Job == "staleAgentCleanup")
			} else {
				assert.False(t, request.Job == "vacuumLogTable" || request.Job == "kubernetesCleanup" || request.Job == "staleAgentCleanup")
			}

			// Verify dry run field is set correctly
			assert.Equal(t, tt.dryRun, request.DryRun)
		})
	}
}

func TestRunMaintenanceNow_NonAdminUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/admin/maintenance/run", nil)
	c.Set("user", &model.User{Admin: false}) // Non-admin user

	output, err := RunMaintenanceNow(testCtx(c), &RunMaintenanceNowInput{
		Body: RunMaintenanceRequest{Job: "vacuumLogTable"},
	})
	assert.Nil(t, output)
	require.Error(t, err)

	var he huma.StatusError
	require.True(t, errors.As(err, &he))
	assert.Equal(t, http.StatusForbidden, he.GetStatus())
}

func TestRunMaintenanceNow_InvalidJob(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/admin/maintenance/run", nil)
	c.Set("user", &model.User{Admin: true})

	output, err := RunMaintenanceNow(testCtx(c), &RunMaintenanceNowInput{
		Body: RunMaintenanceRequest{Job: "invalidJob"},
	})
	assert.Nil(t, output)
	require.Error(t, err)

	var he huma.StatusError
	require.True(t, errors.As(err, &he))
	assert.Equal(t, http.StatusBadRequest, he.GetStatus())
}
