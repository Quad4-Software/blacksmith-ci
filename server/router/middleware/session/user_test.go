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

package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/datastore"
)

func TestMustAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("should allow admin user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user", &model.User{ID: 1, Admin: true})

		handlerCalled := false
		handler := MustAdmin()
		handler(c)

		// If not aborted, handler allows continuation
		if !c.IsAborted() {
			handlerCalled = true
		}

		assert.True(t, handlerCalled)
		assert.NotEqual(t, http.StatusForbidden, w.Code)
		assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("should return 403 for non-admin user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user", &model.User{ID: 1, Admin: false})

		handler := MustAdmin()
		handler(c)
		c.Writer.WriteHeaderNow()

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("should return 401 when no user in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		// No user set in context

		handler := MustAdmin()
		handler(c)
		c.Writer.WriteHeaderNow()

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestMustUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("should allow authenticated user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user", &model.User{ID: 1, Admin: false})

		handlerCalled := false
		handler := MustUser()
		handler(c)

		if !c.IsAborted() {
			handlerCalled = true
		}

		assert.True(t, handlerCalled)
		assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("should allow admin user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user", &model.User{ID: 1, Admin: true})

		handlerCalled := false
		handler := MustUser()
		handler(c)

		if !c.IsAborted() {
			handlerCalled = true
		}

		assert.True(t, handlerCalled)
		assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("should return 401 when no user in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		// No user set in context

		handler := MustUser()
		handler(c)
		c.Writer.WriteHeaderNow()

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("should return user from context", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		expectedUser := &model.User{ID: 1, Login: "testuser"}
		c.Set("user", expectedUser)

		user := User(c)

		assert.Equal(t, expectedUser, user)
	})

	t.Run("should return nil when no user in context", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())

		user := User(c)

		assert.Nil(t, user)
	})

	t.Run("should return nil when context value is wrong type", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("user", "not a user") // Wrong type

		user := User(c)

		assert.Nil(t, user)
	})
}

func seedToken(t *testing.T, s store.Store, login, raw string, admin bool, expiresAt int64) { //nolint:unparam
	t.Helper()
	user := &model.User{Login: login, Admin: admin, Hash: "hash-" + login, ForgeID: 1}
	require.NoError(t, s.CreateUser(user))
	require.NoError(t, s.AccessTokenCreate(&model.AccessToken{
		UserID:    user.ID,
		Name:      "admin-bootstrap",
		TokenHash: datastore.HashAccessToken(raw),
		Scopes:    model.JSONSliceString{string(model.ScopeAdminWrite)},
		ExpiresAt: expiresAt,
	}))
}

func opaqueRequest(s store.Store, bearer string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/user", nil)
	c.Request.Header.Set("Authorization", "Bearer "+bearer)
	store.ToContext(c, s)
	return c
}

func newSessionStore(t *testing.T) store.Store {
	t.Helper()
	s, err := datastore.NewEngine(&store.Opts{Driver: "sqlite3", Config: ":memory:"})
	require.NoError(t, err)
	require.NoError(t, s.Migrate(context.Background(), false))
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSetUserOpaqueToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const raw = "0123456789abcdef0123456789abcdef"

	t.Run("authenticates system admin", func(t *testing.T) {
		s := newSessionStore(t)
		seedToken(t, s, model.SystemAdminLogin, raw, true, 0)
		c := opaqueRequest(s, raw)

		SetUser()(c)

		u := User(c)
		require.NotNil(t, u)
		assert.Equal(t, model.SystemAdminLogin, u.Login)
		assert.True(t, u.Admin)
	})

	t.Run("rejects unknown opaque token", func(t *testing.T) {
		s := newSessionStore(t)
		seedToken(t, s, model.SystemAdminLogin, raw, true, 0)
		c := opaqueRequest(s, "ffffffffffffffffffffffffffffffff")

		SetUser()(c)

		assert.Nil(t, User(c))
	})

	t.Run("rejects expired opaque token", func(t *testing.T) {
		s := newSessionStore(t)
		seedToken(t, s, model.SystemAdminLogin, raw, true, 1) // expired (unix 1)
		c := opaqueRequest(s, raw)

		SetUser()(c)

		assert.Nil(t, User(c))
	})

	t.Run("rejects opaque token owned by non-system user", func(t *testing.T) {
		s := newSessionStore(t)
		seedToken(t, s, "alice", raw, false, 0)
		c := opaqueRequest(s, raw)

		SetUser()(c)

		assert.Nil(t, User(c))
	})
}
