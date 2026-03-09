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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v5/server"
	forge_mocks "codefloe.com/crowci/crow/v5/server/forge/mocks"
	"codefloe.com/crowci/crow/v5/server/model"
	mocks_services "codefloe.com/crowci/crow/v5/server/services/mocks"
	store_mocks "codefloe.com/crowci/crow/v5/server/store/mocks"
	"codefloe.com/crowci/crow/v5/server/store/types"
)

func TestGetUserOrgList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Test user
	user := &model.User{
		ID:      1,
		Login:   "testuser",
		OrgID:   100, // Personal org ID
		ForgeID: 1,
	}

	// Test UserForge entries
	userForge1 := &model.UserForge{
		ID:           1,
		UserID:       1,
		ForgeID:      1,
		IsPrimary:    true,
		AccessToken:  "token1",
		RefreshToken: "refresh1",
	}

	// Test organizations
	personalOrg := &model.Org{
		ID:     100,
		Name:   "testuser",
		IsUser: true, // This is the user's personal org
	}

	memberOrg1 := &model.Org{
		ID:     200,
		Name:   "org1",
		IsUser: false,
	}

	memberOrg2 := &model.Org{
		ID:     300,
		Name:   "org2",
		IsUser: false,
	}

	t.Run("should return orgs the user is a member of", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockForge := new(forge_mocks.MockForge)
		mockManager := new(mocks_services.MockManager)

		// Setup UserForgeList to return one forge
		mockStore.On("UserForgeList", user.ID).Return([]*model.UserForge{userForge1}, nil)
		mockManager.On("ForgeByID", int64(1)).Return(mockForge, nil)

		// Setup forge to return teams
		teams := []*model.Team{
			{Login: "org1"},
			{Login: "org2"},
		}
		mockForge.On("Teams", mock.Anything, mock.AnythingOfType("*model.User")).Return(teams, nil)

		// Setup store to find orgs
		mockStore.On("OrgFindByName", "org1", int64(1)).Return(memberOrg1, nil)
		mockStore.On("OrgFindByName", "org2", int64(1)).Return(memberOrg2, nil)

		server.Config.Services.Manager = mockManager

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
		c.Set("store", mockStore)
		c.Set("user", user)

		output, err := GetUserOrgList(testCtx(c), &GetUserOrgListInput{})
		require.NoError(t, err)
		require.NotNil(t, output)
		assert.Len(t, output.Body, 2)

		// Verify both orgs are returned
		orgNames := make([]string, len(output.Body))
		for i, org := range output.Body {
			orgNames[i] = org.Name
		}
		assert.Contains(t, orgNames, "org1")
		assert.Contains(t, orgNames, "org2")
	})

	t.Run("should filter out personal orgs (IsUser=true)", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockForge := new(forge_mocks.MockForge)
		mockManager := new(mocks_services.MockManager)

		mockStore.On("UserForgeList", user.ID).Return([]*model.UserForge{userForge1}, nil)
		mockManager.On("ForgeByID", int64(1)).Return(mockForge, nil)

		// Forge returns both personal org and member org
		teams := []*model.Team{
			{Login: "testuser"}, // Personal org
			{Login: "org1"},     // Member org
		}
		mockForge.On("Teams", mock.Anything, mock.AnythingOfType("*model.User")).Return(teams, nil)

		// Store finds both orgs
		mockStore.On("OrgFindByName", "testuser", int64(1)).Return(personalOrg, nil)
		mockStore.On("OrgFindByName", "org1", int64(1)).Return(memberOrg1, nil)

		server.Config.Services.Manager = mockManager

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
		c.Set("store", mockStore)
		c.Set("user", user)

		output, err := GetUserOrgList(testCtx(c), &GetUserOrgListInput{})
		require.NoError(t, err)
		require.NotNil(t, output)

		// Only org1 should be returned, not the personal org
		assert.Len(t, output.Body, 1)
		assert.Equal(t, "org1", output.Body[0].Name)
		assert.False(t, output.Body[0].IsUser)
	})

	t.Run("should skip orgs not found in Crow database", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockForge := new(forge_mocks.MockForge)
		mockManager := new(mocks_services.MockManager)

		mockStore.On("UserForgeList", user.ID).Return([]*model.UserForge{userForge1}, nil)
		mockManager.On("ForgeByID", int64(1)).Return(mockForge, nil)

		// Forge returns teams including one that doesn't exist in Crow
		teams := []*model.Team{
			{Login: "org1"},         // Exists in Crow
			{Login: "external-org"}, // Does not exist in Crow
		}
		mockForge.On("Teams", mock.Anything, mock.AnythingOfType("*model.User")).Return(teams, nil)

		// Store finds org1 but not external-org
		mockStore.On("OrgFindByName", "org1", int64(1)).Return(memberOrg1, nil)
		mockStore.On("OrgFindByName", "external-org", int64(1)).Return(nil, types.RecordNotExist)

		server.Config.Services.Manager = mockManager

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
		c.Set("store", mockStore)
		c.Set("user", user)

		output, err := GetUserOrgList(testCtx(c), &GetUserOrgListInput{})
		require.NoError(t, err)
		require.NotNil(t, output)

		// Only org1 should be returned
		assert.Len(t, output.Body, 1)
		assert.Equal(t, "org1", output.Body[0].Name)
	})

	t.Run("should return empty list when user has no team memberships", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockForge := new(forge_mocks.MockForge)
		mockManager := new(mocks_services.MockManager)

		mockStore.On("UserForgeList", user.ID).Return([]*model.UserForge{userForge1}, nil)
		mockManager.On("ForgeByID", int64(1)).Return(mockForge, nil)

		// Forge returns no teams
		mockForge.On("Teams", mock.Anything, mock.AnythingOfType("*model.User")).Return([]*model.Team{}, nil)

		server.Config.Services.Manager = mockManager

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
		c.Set("store", mockStore)
		c.Set("user", user)

		output, err := GetUserOrgList(testCtx(c), &GetUserOrgListInput{})
		require.NoError(t, err)
		require.NotNil(t, output)
		assert.Empty(t, output.Body)
	})

	t.Run("should return 500 when UserForgeList fails", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockManager := new(mocks_services.MockManager)

		// UserForgeList returns error
		mockStore.On("UserForgeList", user.ID).Return(nil, assert.AnError)

		server.Config.Services.Manager = mockManager

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
		c.Set("store", mockStore)
		c.Set("user", user)

		output, err := GetUserOrgList(testCtx(c), &GetUserOrgListInput{})
		assert.Nil(t, output)
		require.Error(t, err)

		var he huma.StatusError
		require.True(t, errors.As(err, &he))
		assert.Equal(t, http.StatusInternalServerError, he.GetStatus())
	})

	t.Run("should skip forge when Teams() fails but continue with others", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockForge := new(forge_mocks.MockForge)
		mockManager := new(mocks_services.MockManager)

		// Single forge that fails on Teams
		mockStore.On("UserForgeList", user.ID).Return([]*model.UserForge{userForge1}, nil)
		mockManager.On("ForgeByID", int64(1)).Return(mockForge, nil)
		mockForge.On("Teams", mock.Anything, mock.AnythingOfType("*model.User")).Return(nil, assert.AnError)

		server.Config.Services.Manager = mockManager

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
		c.Set("store", mockStore)
		c.Set("user", user)

		output, err := GetUserOrgList(testCtx(c), &GetUserOrgListInput{})
		require.NoError(t, err)
		require.NotNil(t, output)
		assert.Empty(t, output.Body)
	})
}

// TestGetOrgs_AdminOnly verifies that GetOrgs (admin endpoint) doesn't have
// its own admin check - it relies on router middleware MustAdmin().
// This test documents the expected behavior when called directly.
func TestGetOrgs_AdminOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("should return orgs when called (middleware handles auth)", func(t *testing.T) {
		orgs := []*model.Org{
			{ID: 1, Name: "org1"},
			{ID: 2, Name: "org2"},
		}

		mockStore := new(store_mocks.MockStore)
		mockStore.On("OrgList", mock.Anything).Return(orgs, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
		c.Set("store", mockStore)
		// Note: No user set - the handler doesn't check admin status
		// because MustAdmin() middleware handles that at router level

		output, err := GetOrgs(testCtx(c), &GetOrgsInput{})
		require.NoError(t, err)
		require.NotNil(t, output)
		assert.Len(t, output.Body, 2)
	})
}
