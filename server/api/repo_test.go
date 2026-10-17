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
	"errors"
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

// TestDeleteRepo_ForgeDeactivateFailureStillRemoves verifies that a repo whose
// forge counterpart no longer exists can still be removed from Crow. When the
// repo was deleted on the forge, the webhook-removal (Deactivate) call fails,
// but that must not block local removal — otherwise the stale repo row becomes
// permanently un-deletable (see issue #1226).
func TestDeleteRepo_ForgeDeactivateFailureStillRemoves(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := &model.User{ID: 1, Login: "testuser", ForgeID: 42}
	repo := &model.Repo{
		ID:            7,
		ForgeID:       42,
		Owner:         "octocat",
		Name:          "deleted-on-forge",
		FullName:      "octocat/deleted-on-forge",
		IsActive:      true,
		UserID:        1,
		ForgeRemoteID: model.ForgeRemoteID("123"),
	}

	mockStore := new(store_mocks.MockStore)
	mockForge := new(forge_mocks.MockForge)
	mockManager := new(mocks_services.MockManager)

	oldManager := server.Config.Services.Manager
	t.Cleanup(func() { server.Config.Services.Manager = oldManager })
	server.Config.Services.Manager = mockManager

	mockManager.On("ForgeFromRepo", repo).Return(mockForge, nil)
	mockManager.On("ForgeByID", int64(42)).Return(mockForge, nil)
	// User has no linked forge credentials — delete must proceed regardless.
	mockStore.On("UserForgeGet", user.ID, repo.ForgeID).Return(nil, types.RecordNotExist)
	// The repo is gone on the forge, so removing the webhook fails.
	mockForge.On("Deactivate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("404 Not Found"))
	mockStore.On("DeleteRepo", repo).Return(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/", nil)
	c.Set("store", mockStore)
	c.Set("user", user)
	c.Set("repo", repo)

	out, err := DeleteRepo(testCtx(c), &DeleteRepoInput{RepoID: repo.ID, Remove: true})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, repo, out.Body)
	mockStore.AssertCalled(t, "DeleteRepo", repo)
}

// A patch that does not name the log retention settings must leave them alone.
// Resetting them to zero made every pipeline start purge the logs of all
// earlier pipelines, including the ones still running.
func TestPatchRepo_KeepsLogRetentionWhenOmitted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newRepo := func() *model.Repo {
		return &model.Repo{ID: 1, LogsPipelinesKeepMin: 90, LogsDurationKeep: "720h"}
	}
	keepMin := int64(5)
	duration := ""

	tests := []struct {
		name         string
		patch        model.RepoPatch
		wantKeepMin  int64
		wantDuration string
	}{
		{
			name:         "omitted",
			patch:        model.RepoPatch{AllowManual: new(true)},
			wantKeepMin:  90,
			wantDuration: "720h",
		},
		{
			name:         "set",
			patch:        model.RepoPatch{LogsPipelinesKeepMin: &keepMin, LogsDurationKeep: &duration},
			wantKeepMin:  5,
			wantDuration: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRepo()
			mockStore := store_mocks.NewMockStore(t)
			mockStore.On("UpdateRepo", repo).Return(nil)

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set("store", mockStore)
			c.Set("repo", repo)
			c.Set("user", &model.User{ID: 1})

			_, err := PatchRepo(testCtx(c), &PatchRepoInput{RepoID: 1, Body: tt.patch})
			require.NoError(t, err)

			assert.Equal(t, tt.wantKeepMin, repo.LogsPipelinesKeepMin)
			assert.Equal(t, tt.wantDuration, repo.LogsDurationKeep)
		})
	}
}
