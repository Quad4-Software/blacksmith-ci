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

func TestPostAutoscalerHeartbeatAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		caller     *model.Autoscaler
		wantStatus int
	}{
		{name: "no autoscaler token", wantStatus: http.StatusUnauthorized},
		{name: "token of another autoscaler", caller: &model.Autoscaler{ID: 2}, wantStatus: http.StatusForbidden},
		{name: "own token", caller: &model.Autoscaler{ID: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := store_mocks.NewMockStore(t)
			if tt.wantStatus == 0 {
				mockStore.On("AutoscalerFind", int64(1)).Return(&model.Autoscaler{ID: 1, Provider: "old"}, nil)
				mockStore.On("AutoscalerUpdate", mock.MatchedBy(func(a *model.Autoscaler) bool {
					return a.ID == 1 && a.Provider == "hetznercloud" && a.ActiveAgents == 3
				})).Return(nil)
			}

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set("store", mockStore)
			if tt.caller != nil {
				c.Set("autoscaler", tt.caller)
			}

			out, err := PostAutoscalerHeartbeat(testCtx(c), &PostAutoscalerHeartbeatInput{
				AutoscalerID: 1,
				Body:         HeartbeatBody{ActiveAgents: 3, Provider: "hetznercloud"},
			})
			if tt.wantStatus != 0 {
				var se huma.StatusError
				require.True(t, errors.As(err, &se), "expected a huma status error, got %v", err)
				assert.Equal(t, tt.wantStatus, se.GetStatus())
				assert.Nil(t, out)
				return
			}
			require.NoError(t, err)
			assert.True(t, out.Body.Success)
		})
	}
}
