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

	"codefloe.com/crowci/crow/v6/server"
	cache_mocks "codefloe.com/crowci/crow/v6/server/cache/mocks"
	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	"codefloe.com/crowci/crow/v6/server/model"
	mocks_services "codefloe.com/crowci/crow/v6/server/services/mocks"
	secret_mocks "codefloe.com/crowci/crow/v6/server/services/secret/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

// The secretTargetFixture models a repo secret (ID 7, owned by repo 1) and the
// scopes it could be shared with or moved to. The developer has push on repos
// 1 and 3 but not 2, owns personal org 10, is a plain member of org 20 and an
// admin of org 30. Repo 99 and org 99 do not exist.
func secretTargetFixture(t *testing.T) (*store_mocks.MockStore, *secret_mocks.MockService) {
	t.Helper()

	s := new(store_mocks.MockStore)
	s.On("SecretFindByID", int64(7)).Return(&model.Secret{ID: 7, Name: "token", RepoID: 1}, nil)
	for _, id := range []int64{1, 2, 3} {
		s.On("GetRepo", id).Return(&model.Repo{ID: id}, nil)
	}
	s.On("GetRepo", int64(99)).Return(nil, types.RecordNotExist)
	s.On("PermFind", mock.Anything, mock.Anything).Return(func(_ *model.User, r *model.Repo) (*model.Perm, error) {
		return &model.Perm{Pull: true, Push: r.ID != 2}, nil
	})
	for _, o := range []*model.Org{
		{ID: 10, Name: "dev", ForgeID: 1},
		{ID: 20, Name: "member-org", ForgeID: 1},
		{ID: 30, Name: "admin-org", ForgeID: 1},
	} {
		s.On("OrgGet", o.ID).Return(o, nil)
	}
	s.On("OrgGet", int64(99)).Return(nil, types.RecordNotExist)
	s.On("UserForgeGet", mock.Anything, mock.Anything).Return(nil, types.RecordNotExist)

	membership := new(cache_mocks.MockMembershipService)
	membership.On("Get", mock.Anything, mock.Anything, mock.Anything, "member-org").Return(&model.OrgPerm{Member: true}, nil)
	membership.On("Get", mock.Anything, mock.Anything, mock.Anything, "admin-org").Return(&model.OrgPerm{Member: true, Admin: true}, nil)

	secretService := new(secret_mocks.MockService)
	secretService.On("SecretTargetCreate", int64(7), mock.Anything, mock.Anything).Return(nil)
	secretService.On("SecretTargetList", int64(7)).Return([]*model.SecretTarget{}, nil)
	secretService.On("SecretTransferOwner", int64(7), mock.Anything, mock.Anything, mock.Anything).Return(nil)

	manager := new(mocks_services.MockManager)
	manager.On("SecretService").Return(secretService)
	manager.On("ForgeByID", mock.Anything).Return(new(forge_mocks.MockForge), nil)

	oldManager, oldMembership := server.Config.Services.Manager, server.Config.Services.Membership
	t.Cleanup(func() {
		server.Config.Services.Manager = oldManager
		server.Config.Services.Membership = oldMembership
	})
	server.Config.Services.Manager = manager
	server.Config.Services.Membership = membership

	return s, secretService
}

func TestSecretTargetDestinationAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)

	developer := &model.User{ID: 1, Login: "dev", OrgID: 10}
	secretsManager := &model.User{ID: 3, Login: "sm", OrgID: 11, Roles: model.JSONSliceString{"secrets-manager"}}
	admin := &model.User{ID: 5, Login: "admin", Admin: true}

	// A status of 0 means the request is allowed. The share endpoint never
	// accepts the global scope, so it has its own expectation.
	tests := []struct {
		name          string
		user          *model.User
		orgID, repoID int64
		share         int
		transfer      int
	}{
		{name: "global scope as developer", user: developer, share: http.StatusUnprocessableEntity, transfer: http.StatusForbidden},
		{name: "global scope as secrets manager", user: secretsManager, share: http.StatusUnprocessableEntity},
		{name: "global scope as admin", user: admin, share: http.StatusUnprocessableEntity},
		{name: "both org and repo", user: developer, orgID: 30, repoID: 3, share: http.StatusUnprocessableEntity, transfer: http.StatusUnprocessableEntity},
		{name: "repo with push", user: developer, repoID: 3},
		{name: "repo without push", user: developer, repoID: 2, share: http.StatusForbidden, transfer: http.StatusForbidden},
		{name: "repo without push as admin", user: admin, repoID: 2},
		{name: "missing repo", user: developer, repoID: 99, share: http.StatusNotFound, transfer: http.StatusNotFound},
		{name: "missing repo as admin", user: admin, repoID: 99, share: http.StatusNotFound, transfer: http.StatusNotFound},
		{name: "personal org", user: developer, orgID: 10},
		{name: "org as plain member", user: developer, orgID: 20, share: http.StatusForbidden, transfer: http.StatusForbidden},
		{name: "org as org admin", user: developer, orgID: 30},
		{name: "missing org", user: developer, orgID: 99, share: http.StatusNotFound, transfer: http.StatusNotFound},
		{name: "missing org as admin", user: admin, orgID: 99, share: http.StatusNotFound, transfer: http.StatusNotFound},
	}

	assertStatus := func(t *testing.T, want int, err error) {
		t.Helper()
		if want == 0 {
			assert.NoError(t, err)
			return
		}
		var se huma.StatusError
		require.True(t, errors.As(err, &se), "expected a huma status error, got %v", err)
		assert.Equal(t, want, se.GetStatus())
	}

	for _, tt := range tests {
		t.Run(tt.name+"/share", func(t *testing.T) {
			s, secretService := secretTargetFixture(t)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set("store", s)
			c.Set("user", tt.user)

			_, err := PostSecretTarget(testCtx(c), &PostSecretTargetInput{
				SecretID: 7,
				Body:     PostSecretTargetBody{OrgID: tt.orgID, RepoID: tt.repoID},
			})
			assertStatus(t, tt.share, err)
			if tt.share != 0 {
				secretService.AssertNotCalled(t, "SecretTargetCreate", mock.Anything, mock.Anything, mock.Anything)
			} else {
				secretService.AssertCalled(t, "SecretTargetCreate", int64(7), tt.orgID, tt.repoID)
			}
		})
		t.Run(tt.name+"/transfer", func(t *testing.T) {
			s, secretService := secretTargetFixture(t)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set("store", s)
			c.Set("user", tt.user)

			_, err := PatchSecretOwner(testCtx(c), &PatchSecretOwnerInput{
				SecretID: 7,
				Body:     PatchSecretOwnerBody{OrgID: tt.orgID, RepoID: tt.repoID},
			})
			assertStatus(t, tt.transfer, err)
			if tt.transfer != 0 {
				secretService.AssertNotCalled(t, "SecretTransferOwner", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			} else {
				secretService.AssertCalled(t, "SecretTransferOwner", int64(7), tt.orgID, tt.repoID, false)
			}
		})
	}
}

// A user without push on the secret's current repo cannot share or move it,
// whatever the destination.
func TestSecretTargetRequiresSourceAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s, secretService := secretTargetFixture(t)
	// Repo 2 is the one the developer lacks push on.
	s.On("SecretFindByID", int64(8)).Return(&model.Secret{ID: 8, Name: "token", RepoID: 2}, nil)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("store", s)
	c.Set("user", &model.User{ID: 1, Login: "dev", OrgID: 10})

	_, err := PatchSecretOwner(testCtx(c), &PatchSecretOwnerInput{SecretID: 8, Body: PatchSecretOwnerBody{RepoID: 3}})
	var se huma.StatusError
	require.True(t, errors.As(err, &se))
	assert.Equal(t, http.StatusForbidden, se.GetStatus())
	secretService.AssertNotCalled(t, "SecretTransferOwner", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
