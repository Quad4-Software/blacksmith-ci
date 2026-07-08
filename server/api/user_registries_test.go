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

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	"codefloe.com/crowci/crow/v6/server/model"
	mocks_services "codefloe.com/crowci/crow/v6/server/services/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

func TestGetUserRegistryList_UsesUserForgeTokenForOrgScope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := &model.User{
		ID:           1,
		Login:        "testuser",
		OrgID:        100,
		ForgeID:      42,
		AccessToken:  "stale-token",
		RefreshToken: "stale-refresh",
	}

	userForge := &model.UserForge{
		UserID:       1,
		ForgeID:      42,
		AccessToken:  "fresh-token",
		RefreshToken: "fresh-refresh",
		Expiry:       12345,
	}

	org := &model.Org{
		ID:      200,
		Name:    "org-a",
		ForgeID: 42,
	}

	mockStore := new(store_mocks.MockStore)
	mockForge := new(forge_mocks.MockForge)
	mockManager := new(mocks_services.MockManager)

	oldManager := server.Config.Services.Manager
	t.Cleanup(func() {
		server.Config.Services.Manager = oldManager
	})
	server.Config.Services.Manager = mockManager

	mockManager.On("ForgeFromUser", user).Return(mockForge, nil)
	mockStore.On("UserForgeGet", user.ID, user.ForgeID).Return(userForge, nil)
	mockForge.On("Teams", mock.Anything, mock.AnythingOfType("*model.User")).Run(func(args mock.Arguments) {
		forgeUser, ok := args.Get(1).(*model.User)
		require.True(t, ok)
		require.NotNil(t, forgeUser)
		assert.Equal(t, user.ID, forgeUser.ID)
		assert.Equal(t, user.ForgeID, forgeUser.ForgeID)
		assert.Equal(t, "fresh-token", forgeUser.AccessToken)
		assert.Equal(t, "fresh-refresh", forgeUser.RefreshToken)
		assert.Equal(t, int64(12345), forgeUser.Expiry)
	}).Return([]*model.Team{{Login: "org-a"}}, nil)
	mockStore.On("OrgFindByName", "org-a", user.ForgeID).Return(org, nil)
	mockStore.On("UserRegistryList", []int64{org.ID}, false, "", mock.MatchedBy(func(p *model.ListOptions) bool {
		return p != nil && p.Page == 1 && p.PerPage == 50
	})).Return([]*model.Registry{}, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	c.Set("store", mockStore)
	c.Set("user", user)

	output, err := GetUserRegistryList(testCtx(c), &GetUserRegistryListInput{
		PaginationParams: PaginationParams{Page: 1, PerPage: 50},
		Scope:            "org",
	})
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Empty(t, output.Body)
}

func TestGetUserRegistryList_FallsBackToSessionTokenWithoutUserForge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := &model.User{
		ID:           1,
		Login:        "testuser",
		OrgID:        100,
		ForgeID:      42,
		AccessToken:  "session-token",
		RefreshToken: "session-refresh",
	}

	org := &model.Org{
		ID:      201,
		Name:    "org-b",
		ForgeID: 42,
	}

	mockStore := new(store_mocks.MockStore)
	mockForge := new(forge_mocks.MockForge)
	mockManager := new(mocks_services.MockManager)

	oldManager := server.Config.Services.Manager
	t.Cleanup(func() {
		server.Config.Services.Manager = oldManager
	})
	server.Config.Services.Manager = mockManager

	mockManager.On("ForgeFromUser", user).Return(mockForge, nil)
	mockStore.On("UserForgeGet", user.ID, user.ForgeID).Return((*model.UserForge)(nil), types.RecordNotExist)
	mockForge.On("Teams", mock.Anything, mock.AnythingOfType("*model.User")).Run(func(args mock.Arguments) {
		forgeUser, ok := args.Get(1).(*model.User)
		require.True(t, ok)
		require.NotNil(t, forgeUser)
		assert.Same(t, user, forgeUser)
		assert.Equal(t, "session-token", forgeUser.AccessToken)
		assert.Equal(t, "session-refresh", forgeUser.RefreshToken)
	}).Return([]*model.Team{{Login: "org-b"}}, nil)
	mockStore.On("OrgFindByName", "org-b", user.ForgeID).Return(org, nil)
	mockStore.On("UserRegistryList", []int64{org.ID}, false, "", mock.MatchedBy(func(p *model.ListOptions) bool {
		return p != nil && p.Page == 1 && p.PerPage == 50
	})).Return([]*model.Registry{}, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	c.Set("store", mockStore)
	c.Set("user", user)

	output, err := GetUserRegistryList(testCtx(c), &GetUserRegistryListInput{
		PaginationParams: PaginationParams{Page: 1, PerPage: 50},
		Scope:            "org",
	})
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Empty(t, output.Body)
}
