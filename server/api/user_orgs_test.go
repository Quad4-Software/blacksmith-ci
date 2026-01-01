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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codeberg.org/crowci/crow/v5/server"
	forge_mocks "codeberg.org/crowci/crow/v5/server/forge/mocks"
	"codeberg.org/crowci/crow/v5/server/model"
	mocks_services "codeberg.org/crowci/crow/v5/server/services/mocks"
	store_mocks "codeberg.org/crowci/crow/v5/server/store/mocks"
	"codeberg.org/crowci/crow/v5/server/store/types"
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

		// Setup forge to return teams
		teams := []*model.Team{
			{Login: "org1"},
			{Login: "org2"},
		}
		mockManager.On("ForgeFromUser", user).Return(mockForge, nil)
		mockForge.On("Teams", mock.Anything, user).Return(teams, nil)

		// Setup store to find orgs
		mockStore.On("OrgFindByName", "org1", user.ForgeID).Return(memberOrg1, nil)
		mockStore.On("OrgFindByName", "org2", user.ForgeID).Return(memberOrg2, nil)

		server.Config.Services.Manager = mockManager

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Set("user", user)

		GetUserOrgList(c)
		c.Writer.WriteHeaderNow()

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*model.Org
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Len(t, response, 2)

		// Verify both orgs are returned
		orgNames := make([]string, len(response))
		for i, org := range response {
			orgNames[i] = org.Name
		}
		assert.Contains(t, orgNames, "org1")
		assert.Contains(t, orgNames, "org2")
	})

	t.Run("should filter out personal orgs (IsUser=true)", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockForge := new(forge_mocks.MockForge)
		mockManager := new(mocks_services.MockManager)

		// Forge returns both personal org and member org
		teams := []*model.Team{
			{Login: "testuser"}, // Personal org
			{Login: "org1"},     // Member org
		}
		mockManager.On("ForgeFromUser", user).Return(mockForge, nil)
		mockForge.On("Teams", mock.Anything, user).Return(teams, nil)

		// Store finds both orgs
		mockStore.On("OrgFindByName", "testuser", user.ForgeID).Return(personalOrg, nil)
		mockStore.On("OrgFindByName", "org1", user.ForgeID).Return(memberOrg1, nil)

		server.Config.Services.Manager = mockManager

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Set("user", user)

		GetUserOrgList(c)
		c.Writer.WriteHeaderNow()

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*model.Org
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Only org1 should be returned, not the personal org
		assert.Len(t, response, 1)
		assert.Equal(t, "org1", response[0].Name)
		assert.False(t, response[0].IsUser)
	})

	t.Run("should skip orgs not found in Crow database", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockForge := new(forge_mocks.MockForge)
		mockManager := new(mocks_services.MockManager)

		// Forge returns teams including one that doesn't exist in Crow
		teams := []*model.Team{
			{Login: "org1"},         // Exists in Crow
			{Login: "external-org"}, // Does not exist in Crow
		}
		mockManager.On("ForgeFromUser", user).Return(mockForge, nil)
		mockForge.On("Teams", mock.Anything, user).Return(teams, nil)

		// Store finds org1 but not external-org
		mockStore.On("OrgFindByName", "org1", user.ForgeID).Return(memberOrg1, nil)
		mockStore.On("OrgFindByName", "external-org", user.ForgeID).Return(nil, types.RecordNotExist)

		server.Config.Services.Manager = mockManager

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Set("user", user)

		GetUserOrgList(c)
		c.Writer.WriteHeaderNow()

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*model.Org
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Only org1 should be returned
		assert.Len(t, response, 1)
		assert.Equal(t, "org1", response[0].Name)
	})

	t.Run("should return empty list when user has no team memberships", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockForge := new(forge_mocks.MockForge)
		mockManager := new(mocks_services.MockManager)

		// Forge returns no teams
		mockManager.On("ForgeFromUser", user).Return(mockForge, nil)
		mockForge.On("Teams", mock.Anything, user).Return([]*model.Team{}, nil)

		server.Config.Services.Manager = mockManager

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Set("user", user)

		GetUserOrgList(c)
		c.Writer.WriteHeaderNow()

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*model.Org
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Empty(t, response)
	})

	t.Run("should return 500 when forge Teams() fails", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockForge := new(forge_mocks.MockForge)
		mockManager := new(mocks_services.MockManager)

		// Forge returns error
		mockManager.On("ForgeFromUser", user).Return(mockForge, nil)
		mockForge.On("Teams", mock.Anything, user).Return(nil, assert.AnError)

		server.Config.Services.Manager = mockManager

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Set("user", user)

		GetUserOrgList(c)
		c.Writer.WriteHeaderNow()

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("should return 500 when ForgeFromUser fails", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockManager := new(mocks_services.MockManager)

		// Manager returns error
		mockManager.On("ForgeFromUser", user).Return(nil, assert.AnError)

		server.Config.Services.Manager = mockManager

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Set("user", user)

		GetUserOrgList(c)
		c.Writer.WriteHeaderNow()

		assert.Equal(t, http.StatusInternalServerError, w.Code)
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
		c.Set("store", mockStore)
		// Note: No user set - the handler doesn't check admin status
		// because MustAdmin() middleware handles that at router level

		GetOrgs(c)
		c.Writer.WriteHeaderNow()

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*model.Org
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Len(t, response, 2)
	})
}
