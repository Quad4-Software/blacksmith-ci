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

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/auth/oidc"
	"codefloe.com/crowci/crow/v6/server/model"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func identityCtx(t *testing.T, s *store_mocks.MockStore) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	c.Set("store", s)
	c.Set("user", &model.User{ID: 42, Login: "jane"})
	return c
}

func TestUserIdentities(t *testing.T) {
	identity := &model.UserIdentity{ID: 3, UserID: 42, Provider: "oidc", Subject: "sub-jane"}

	t.Run("lists the user's identities", func(t *testing.T) {
		s := new(store_mocks.MockStore)
		s.On("UserIdentityList", int64(42)).Return([]*model.UserIdentity{identity}, nil)
		out, err := GetUserIdentities(testCtx(identityCtx(t, s)), &GetUserIdentitiesInput{})
		require.NoError(t, err)
		assert.Equal(t, []*model.UserIdentity{identity}, out.Body)
	})

	t.Run("deletes the last identity of a user with a forge", func(t *testing.T) {
		s := new(store_mocks.MockStore)
		s.On("UserIdentityList", int64(42)).Return([]*model.UserIdentity{identity}, nil)
		s.On("UserForgeList", int64(42)).Return([]*model.UserForge{{ID: 1, UserID: 42, ForgeID: 1}}, nil)
		s.On("UserIdentityDelete", identity).Return(nil)
		_, err := DeleteUserIdentity(testCtx(identityCtx(t, s)), &DeleteUserIdentityInput{ID: 3})
		require.NoError(t, err)
		s.AssertExpectations(t)
	})

	t.Run("deletes the last identity of a user with a legacy forge identity", func(t *testing.T) {
		s := new(store_mocks.MockStore)
		s.On("UserIdentityList", int64(42)).Return([]*model.UserIdentity{identity}, nil)
		s.On("UserIdentityDelete", identity).Return(nil)
		c := identityCtx(t, s)
		c.Set("user", &model.User{ID: 42, Login: "jane", ForgeRemoteID: "1001"})
		_, err := DeleteUserIdentity(testCtx(c), &DeleteUserIdentityInput{ID: 3})
		require.NoError(t, err)
		s.AssertExpectations(t)
	})

	t.Run("keeps the last identity of a user without a forge", func(t *testing.T) {
		s := new(store_mocks.MockStore)
		s.On("UserIdentityList", int64(42)).Return([]*model.UserIdentity{identity}, nil)
		s.On("UserForgeList", int64(42)).Return([]*model.UserForge{}, nil)
		_, err := DeleteUserIdentity(testCtx(identityCtx(t, s)), &DeleteUserIdentityInput{ID: 3})
		var se huma.StatusError
		require.ErrorAs(t, err, &se)
		assert.Equal(t, http.StatusConflict, se.GetStatus())
		assert.Contains(t, err.Error(), "Link a forge account")
		s.AssertNotCalled(t, "UserIdentityDelete", identity)
	})

	t.Run("does not delete another user's identity", func(t *testing.T) {
		s := new(store_mocks.MockStore)
		s.On("UserIdentityList", int64(42)).Return([]*model.UserIdentity{identity}, nil)
		_, err := DeleteUserIdentity(testCtx(identityCtx(t, s)), &DeleteUserIdentityInput{ID: 99})
		var se huma.StatusError
		require.ErrorAs(t, err, &se)
		assert.Equal(t, http.StatusNotFound, se.GetStatus())
	})

	t.Run("keeps the last identity when forge sign-in is disabled", func(t *testing.T) {
		server.Config.Services.OIDC = oidc.New(oidc.Config{Issuer: "https://idp.example", Only: true})
		t.Cleanup(func() { server.Config.Services.OIDC = nil })
		s := new(store_mocks.MockStore)
		s.On("UserIdentityList", int64(42)).Return([]*model.UserIdentity{identity}, nil)
		_, err := DeleteUserIdentity(testCtx(identityCtx(t, s)), &DeleteUserIdentityInput{ID: 3})
		var se huma.StatusError
		require.ErrorAs(t, err, &se)
		assert.Equal(t, http.StatusConflict, se.GetStatus())
		s.AssertNotCalled(t, "UserIdentityDelete", identity)
	})
}
