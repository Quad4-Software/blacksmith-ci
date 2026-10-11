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
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func roleTestUsers() map[string]*model.User {
	return map[string]*model.User{
		"user":            {ID: 1, Login: "user"},
		"operator":        {ID: 2, Login: "op", Roles: model.JSONSliceString{"operator"}},
		"secrets-manager": {ID: 3, Login: "sm", Roles: model.JSONSliceString{"secrets-manager"}},
		"auditor":         {ID: 4, Login: "aud", Roles: model.JSONSliceString{"auditor"}},
		"admin":           {ID: 5, Login: "admin", Admin: true},
	}
}

func TestCanViewGlobalSecret(t *testing.T) {
	want := map[string]bool{
		"user":            false,
		"operator":        false,
		"secrets-manager": true,
		"auditor":         false,
		"admin":           true,
	}
	a := NewSecretViewAuthorizer(nil, nil)
	assert.False(t, a.CanViewGlobalSecret(nil))
	for name, user := range roleTestUsers() {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, want[name], a.CanViewGlobalSecret(user))
		})
	}
}

func TestAuthorizeSecretAccessGlobal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		user   string
		status int
	}{
		{"secrets-manager", 0},
		{"admin", 0},
		{"user", http.StatusForbidden},
		{"auditor", http.StatusForbidden},
		{"operator", http.StatusForbidden},
	}
	users := roleTestUsers()
	for _, tt := range tests {
		t.Run(tt.user, func(t *testing.T) {
			s := store_mocks.NewMockStore(t)
			s.On("SecretFindByID", int64(7)).Return(&model.Secret{ID: 7, Name: "global"}, nil)

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set("user", users[tt.user])

			err := authorizeSecretAccess(testCtx(c), s, 7)
			if tt.status == 0 {
				assert.NoError(t, err)
				return
			}
			var humaErr huma.StatusError
			require.True(t, errors.As(err, &humaErr), "expected huma error, got %v", err)
			assert.Equal(t, tt.status, humaErr.GetStatus())
		})
	}
}
