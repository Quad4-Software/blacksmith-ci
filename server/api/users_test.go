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
	"context"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/auth/oidc"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func newUsersTestContext(t *testing.T, s *store_mocks.MockStore) context.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	store.ToContext(c, s)
	return testCtx(c)
}

func setManagedAuditor(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { server.Config.Services.OIDC = nil })
	server.Config.Services.OIDC = oidc.New(oidc.Config{
		Issuer:     "https://idp.example",
		RoleGroups: map[string][]string{"auditor": {"audit"}},
	})
}

func TestPatchUserRoles(t *testing.T) {
	setManagedAuditor(t)

	roles := func(r ...string) *[]string { return &r }
	tests := []struct {
		name    string
		stored  model.JSONSliceString
		body    *[]string
		want    model.JSONSliceString
		wantErr int
	}{
		{"assigns unmanaged role", nil, roles("operator"), model.JSONSliceString{"operator"}, 0},
		{"nil roles keep roles", model.JSONSliceString{"operator"}, nil, model.JSONSliceString{"operator"}, 0},
		{"rejects unknown role", nil, roles("root"), nil, http.StatusBadRequest},
		{"rejects adding a managed role", nil, roles("auditor"), nil, http.StatusBadRequest},
		{"rejects adding a managed role next to held roles", model.JSONSliceString{"operator"}, roles("auditor", "operator"), nil, http.StatusBadRequest},
		{"accepts the current roles including a held managed role", model.JSONSliceString{"auditor", "operator"}, roles("auditor", "operator"), model.JSONSliceString{"auditor", "operator"}, 0},
		{"keeps a held managed role sent back with other changes", model.JSONSliceString{"auditor", "operator"}, roles("auditor", "secrets-manager"), model.JSONSliceString{"auditor", "secrets-manager"}, 0},
		{"removes a held managed role left out of the request", model.JSONSliceString{"auditor", "operator"}, roles("operator"), model.JSONSliceString{"operator"}, 0},
		{"removes every role with an empty list", model.JSONSliceString{"auditor", "operator"}, roles(), model.JSONSliceString{}, 0},
		{"nil roles keep a held managed role", model.JSONSliceString{"auditor", "operator"}, nil, model.JSONSliceString{"auditor", "operator"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := store_mocks.NewMockStore(t)
			s.On("GetUserLogin", "bob").Return(&model.User{ID: 2, Login: "bob", Roles: tt.stored}, nil)
			var saved *model.User
			if tt.wantErr == 0 {
				s.On("UpdateUser", mock.Anything).Run(func(a mock.Arguments) { saved, _ = a.Get(0).(*model.User) }).Return(nil)
			}
			ctx := newUsersTestContext(t, s)
			_, err := PatchUser(ctx, &PatchUserInput{Login: "bob", Body: PatchUserBody{Roles: tt.body}})
			if tt.wantErr != 0 {
				var se huma.StatusError
				require.ErrorAs(t, err, &se)
				assert.Equal(t, tt.wantErr, se.GetStatus())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, saved.Roles)
		})
	}
}

func TestPostUserRoles(t *testing.T) {
	setManagedAuditor(t)

	t.Run("assigns roles", func(t *testing.T) {
		s := store_mocks.NewMockStore(t)
		var saved *model.User
		s.On("CreateUser", mock.Anything).Run(func(a mock.Arguments) { saved, _ = a.Get(0).(*model.User) }).Return(nil)
		ctx := newUsersTestContext(t, s)
		_, err := PostUser(ctx, &PostUserInput{Body: PostUserBody{Login: "bob", Email: "bob@example.com", Roles: []string{"operator"}}})
		require.NoError(t, err)
		assert.Equal(t, model.JSONSliceString{"operator"}, saved.Roles)
	})

	t.Run("rejects managed role", func(t *testing.T) {
		s := store_mocks.NewMockStore(t)
		ctx := newUsersTestContext(t, s)
		_, err := PostUser(ctx, &PostUserInput{Body: PostUserBody{Login: "bob", Email: "bob@example.com", Roles: []string{"auditor"}}})
		var se huma.StatusError
		require.ErrorAs(t, err, &se)
		assert.Equal(t, http.StatusBadRequest, se.GetStatus())
	})
}
