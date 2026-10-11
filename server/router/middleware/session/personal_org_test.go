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

package session_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

func TestIsPersonalOrgOwner(t *testing.T) {
	user := &model.User{ID: 42, Login: "jane", OrgID: 5}

	t.Run("owns the org the user points at", func(t *testing.T) {
		_store := store_mocks.NewMockStore(t)
		assert.True(t, session.IsPersonalOrgOwner(_store, user, &model.Org{ID: 5, Name: "anything", ForgeID: 1}))
	})

	t.Run("owns an org named like the linked forge account", func(t *testing.T) {
		_store := store_mocks.NewMockStore(t)
		_store.On("UserForgeGet", int64(42), int64(2)).Return(&model.UserForge{ForgeLogin: "Jane-Forge"}, nil)
		assert.True(t, session.IsPersonalOrgOwner(_store, user, &model.Org{ID: 9, Name: "jane-forge", ForgeID: 2}))
	})

	t.Run("does not own an org that only matches the Crow login", func(t *testing.T) {
		_store := store_mocks.NewMockStore(t)
		_store.On("UserForgeGet", int64(42), int64(1)).Return(nil, types.RecordNotExist)
		assert.False(t, session.IsPersonalOrgOwner(_store, user, &model.Org{ID: 9, Name: "jane", ForgeID: 1}))
	})

	t.Run("does not own an org named unlike the linked forge account", func(t *testing.T) {
		_store := store_mocks.NewMockStore(t)
		_store.On("UserForgeGet", int64(42), int64(1)).Return(&model.UserForge{ForgeLogin: "jane-forge"}, nil)
		assert.False(t, session.IsPersonalOrgOwner(_store, user, &model.Org{ID: 9, Name: "jane", ForgeID: 1}))
	})

	t.Run("handles missing values", func(t *testing.T) {
		assert.False(t, session.IsPersonalOrgOwner(nil, nil, &model.Org{ID: 5}))
		assert.False(t, session.IsPersonalOrgOwner(nil, user, nil))
		assert.False(t, session.IsPersonalOrgOwner(nil, &model.User{ID: 1}, &model.Org{ID: 0, Name: "x"}))
	})
}

func TestMustOrgMemberPersonalOrg(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("user", &model.User{ID: 42, Login: "jane", OrgID: 5})
	c.Set("org", &model.Org{ID: 5, Name: "jane"})

	session.MustOrgMember(true)(c)

	assert.False(t, c.IsAborted(), "the owner of a personal org passes without a forge membership check")
}
