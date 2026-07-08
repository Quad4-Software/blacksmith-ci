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
	cache_mocks "codefloe.com/crowci/crow/v6/server/cache/mocks"
	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	"codefloe.com/crowci/crow/v6/server/model"
	mocks_services "codefloe.com/crowci/crow/v6/server/services/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

// newLookupOrgTestFixtures wires up the mocks that the private-org membership
// branch of LookupOrg / LookupOrgGin needs. It sets up the service manager
// and Membership service and registers the cleanup to restore them.
func newLookupOrgTestFixtures(t *testing.T) (*store_mocks.MockStore, *cache_mocks.MockMembershipService) {
	t.Helper()

	mockStore := new(store_mocks.MockStore)
	mockForge := new(forge_mocks.MockForge)
	mockManager := new(mocks_services.MockManager)
	mockMembership := new(cache_mocks.MockMembershipService)

	oldManager := server.Config.Services.Manager
	oldMembership := server.Config.Services.Membership
	t.Cleanup(func() {
		server.Config.Services.Manager = oldManager
		server.Config.Services.Membership = oldMembership
	})

	server.Config.Services.Manager = mockManager
	server.Config.Services.Membership = mockMembership

	mockManager.On("ForgeFromUser", mock.AnythingOfType("*model.User")).Return(mockForge, nil)

	return mockStore, mockMembership
}

func TestLookupOrg_UsesUserForgeTokenForMembershipCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := &model.User{
		ID:           1,
		Login:        "private-org",
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
		Name:    "private-org",
		ForgeID: 42,
		Private: true,
	}

	mockStore, mockMembership := newLookupOrgTestFixtures(t)

	mockStore.On("OrgFindByName", "private-org", user.ForgeID).Return(org, nil)
	mockStore.On("UserForgeGet", user.ID, org.ForgeID).Return(userForge, nil)
	mockMembership.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*model.User"), org.Name).
		Run(func(args mock.Arguments) {
			forgeUser, ok := args.Get(2).(*model.User)
			require.True(t, ok)
			require.NotNil(t, forgeUser)
			assert.Equal(t, user.ID, forgeUser.ID)
			assert.Equal(t, org.ForgeID, forgeUser.ForgeID)
			assert.Equal(t, "fresh-token", forgeUser.AccessToken)
			assert.Equal(t, "fresh-refresh", forgeUser.RefreshToken)
			assert.Equal(t, int64(12345), forgeUser.Expiry)
		}).
		Return(&model.OrgPerm{Member: true}, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	c.Set("store", mockStore)
	c.Set("user", user)

	output, err := LookupOrg(testCtx(c), &LookupOrgInput{OrgFullName: "private-org"})
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, org, output.Body)
}

func TestLookupOrg_FallsBackToSessionTokenWithoutUserForge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := &model.User{
		ID:           1,
		Login:        "private-org",
		ForgeID:      42,
		AccessToken:  "session-token",
		RefreshToken: "session-refresh",
	}

	org := &model.Org{
		ID:      201,
		Name:    "private-org",
		ForgeID: 42,
		Private: true,
	}

	mockStore, mockMembership := newLookupOrgTestFixtures(t)

	mockStore.On("OrgFindByName", "private-org", user.ForgeID).Return(org, nil)
	mockStore.On("UserForgeGet", user.ID, org.ForgeID).Return((*model.UserForge)(nil), types.RecordNotExist)
	mockMembership.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*model.User"), org.Name).
		Run(func(args mock.Arguments) {
			forgeUser, ok := args.Get(2).(*model.User)
			require.True(t, ok)
			require.NotNil(t, forgeUser)
			assert.Same(t, user, forgeUser)
			assert.Equal(t, "session-token", forgeUser.AccessToken)
		}).
		Return(&model.OrgPerm{Member: true}, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	c.Set("store", mockStore)
	c.Set("user", user)

	output, err := LookupOrg(testCtx(c), &LookupOrgInput{OrgFullName: "private-org"})
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, org, output.Body)
}

func TestLookupOrgGin_UsesUserForgeTokenForMembershipCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := &model.User{
		ID:           1,
		Login:        "private-org",
		ForgeID:      42,
		AccessToken:  "stale-token",
		RefreshToken: "stale-refresh",
	}

	userForge := &model.UserForge{
		UserID:       1,
		ForgeID:      42,
		AccessToken:  "fresh-token",
		RefreshToken: "fresh-refresh",
		Expiry:       54321,
	}

	org := &model.Org{
		ID:      202,
		Name:    "private-org",
		ForgeID: 42,
		Private: true,
	}

	mockStore, mockMembership := newLookupOrgTestFixtures(t)

	mockStore.On("OrgFindByName", "private-org", user.ForgeID).Return(org, nil)
	mockStore.On("UserForgeGet", user.ID, org.ForgeID).Return(userForge, nil)
	mockMembership.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*model.User"), org.Name).
		Run(func(args mock.Arguments) {
			forgeUser, ok := args.Get(2).(*model.User)
			require.True(t, ok)
			require.NotNil(t, forgeUser)
			assert.Equal(t, "fresh-token", forgeUser.AccessToken)
			assert.Equal(t, "fresh-refresh", forgeUser.RefreshToken)
			assert.Equal(t, int64(54321), forgeUser.Expiry)
		}).
		Return(&model.OrgPerm{Member: true}, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	c.Set("store", mockStore)
	c.Set("user", user)
	c.Params = gin.Params{{Key: "org_full_name", Value: "private-org"}}

	LookupOrgGin(c)

	assert.Equal(t, http.StatusOK, w.Code)
}
