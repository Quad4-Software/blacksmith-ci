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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func TestResolveScopedRepoIDsAll(t *testing.T) {
	t.Run("auditor sees all", func(t *testing.T) {
		user := &model.User{Roles: model.JSONSliceString{"auditor"}}
		_, _, scope, err := resolveScopedRepoIDs(store_mocks.NewMockStore(t), user, true, "")
		require.NoError(t, err)
		assert.True(t, scope)
	})

	t.Run("secrets-manager does not see all", func(t *testing.T) {
		user := &model.User{Roles: model.JSONSliceString{"secrets-manager"}}
		s := store_mocks.NewMockStore(t)
		s.On("RepoList", user, false, true).Return([]*model.Repo{}, nil)
		_, _, scope, err := resolveScopedRepoIDs(s, user, true, "")
		require.NoError(t, err)
		assert.False(t, scope)
	})
}
