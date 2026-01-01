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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"codeberg.org/crowci/crow/v5/server/model"
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
