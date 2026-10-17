// Copyright 2022 Woodpecker Authors
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

package common_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/forge/common"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func Test_Netrc(t *testing.T) {
	host, err := common.ExtractHostFromCloneURL("https://git.example.com/foo/bar.git")
	assert.NoError(t, err)
	assert.Equal(t, "git.example.com", host)
}

func TestUserTokenForgeFallback(t *testing.T) {
	errNotFound := errors.New("not found")

	tests := []struct {
		name      string
		userForge int64
		repoForge int64
		linked    *model.UserForge
		want      string
	}{
		{name: "linked forge token wins", userForge: 1, repoForge: 2, linked: &model.UserForge{AccessToken: "forge-2"}, want: "forge-2"},
		{name: "primary token for the same forge", userForge: 2, repoForge: 2, want: "primary"},
		{name: "legacy user without forge id", userForge: 0, repoForge: 2, want: "primary"},
		{name: "primary token never goes to another forge", userForge: 1, repoForge: 2, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := store_mocks.NewMockStore(t)
			if tt.linked != nil {
				mockStore.On("UserForgeGet", int64(7), tt.repoForge).Return(tt.linked, nil)
			} else {
				mockStore.On("UserForgeGet", int64(7), tt.repoForge).Return(nil, errNotFound)
			}
			ctx := store.InjectToContext(context.Background(), mockStore)

			user := &model.User{ID: 7, ForgeID: tt.userForge, AccessToken: "primary"}
			repo := &model.Repo{ForgeID: tt.repoForge}
			assert.Equal(t, tt.want, common.UserToken(ctx, repo, user))
		})
	}
}

func TestRepoUserClearsTokenForOtherForge(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockStore.On("GetUser", int64(7)).Return(&model.User{ID: 7, ForgeID: 1, AccessToken: "primary", RefreshToken: "refresh", Expiry: 10}, nil)
	mockStore.On("UserForgeGet", int64(7), int64(2)).Return(nil, errors.New("not found"))
	ctx := store.InjectToContext(context.Background(), mockStore)

	user, err := common.RepoUser(ctx, &model.Repo{UserID: 7, ForgeID: 2})
	require.NoError(t, err)
	assert.Empty(t, user.AccessToken)
	assert.Empty(t, user.RefreshToken)
	assert.Zero(t, user.Expiry)
}
