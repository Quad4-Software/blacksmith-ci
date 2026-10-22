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

package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func TestPostAgentProvisioning(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := ProvisioningBody{Phase: "Building the base image", Detail: "DISM failed", Failed: true}

	tests := []struct {
		name       string
		auth       string
		known      bool
		wantStatus int
	}{
		{name: "no token", wantStatus: http.StatusUnauthorized},
		{name: "not a bearer token", auth: "Basic dXNlcjpwYXNz", wantStatus: http.StatusUnauthorized},
		{name: "empty bearer token", auth: "Bearer ", wantStatus: http.StatusUnauthorized},
		{name: "unknown token", auth: "Bearer nope", wantStatus: http.StatusUnauthorized},
		{name: "agent token", auth: "Bearer agent-token", known: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := store_mocks.NewMockStore(t)
			if tt.auth == "Bearer nope" {
				mockStore.On("AgentFindByToken", "nope").Return(nil, errors.New("not found"))
			}
			if tt.known {
				mockStore.On("AgentFindByToken", "agent-token").Return(&model.Agent{ID: 3, Provisioning: model.JSONSliceProvisioningPhase{
					{Phase: "Installing Hyper-V", At: 1},
				}}, nil)
				mockStore.On("AgentUpdate", mock.MatchedBy(func(a *model.Agent) bool {
					if a.ID != 3 || len(a.Provisioning) != 2 {
						return false
					}
					p := a.Provisioning[1]
					return p.Phase == body.Phase && p.Detail == body.Detail && p.Failed && p.At > 1
				})).Return(nil)
			}

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/agent/provisioning", nil)
			if tt.auth != "" {
				c.Request.Header.Set("Authorization", tt.auth)
			}
			c.Set("store", mockStore)

			_, err := PostAgentProvisioning(testCtx(c), &PostAgentProvisioningInput{Body: body})
			if tt.wantStatus != 0 {
				var se huma.StatusError
				require.True(t, errors.As(err, &se), "expected a huma status error, got %v", err)
				assert.Equal(t, tt.wantStatus, se.GetStatus())
				return
			}
			require.NoError(t, err)
		})
	}
}
