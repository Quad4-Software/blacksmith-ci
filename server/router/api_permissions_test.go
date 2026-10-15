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

package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
)

// The guardStore answers the store calls made by the API middleware and panics
// on anything else, which gin.Recovery turns into a 500. A guarded route that
// lets the user through therefore returns something other than 401 or 403.
type guardStore struct{ store.Store }

func (guardStore) UserForgeList(int64) ([]*model.UserForge, error) { return nil, nil }

func newGuardEngine(t *testing.T, user *model.User) *gin.Engine {
	t.Helper()
	// The /debug group is only registered at debug log level.
	prev := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	t.Cleanup(func() { zerolog.SetGlobalLevel(prev) })

	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(gin.Recovery())
	e.Use(func(c *gin.Context) {
		store.ToContext(c, guardStore{})
		if user != nil {
			c.Set("user", user)
		}
		c.Next()
	})
	base := e.Group("")
	apiBase := base.Group("/api/v1")
	apiRoutes(base, apiBase, newHumaAPI(e, apiBase, "test", "test"))
	return e
}

type guardedRoute struct {
	method, path string
	perm         model.Permission
}

var guardedRoutes = []guardedRoute{
	{http.MethodGet, "/api/v1/users", model.PermUsersRead},
	{http.MethodGet, "/api/v1/users/bob", model.PermUsersRead},
	{http.MethodGet, "/api/v1/entities", model.PermUsersRead},
	{http.MethodPost, "/api/v1/users", model.PermUsersWrite},
	{http.MethodPatch, "/api/v1/users/bob", model.PermUsersWrite},
	{http.MethodDelete, "/api/v1/users/bob", model.PermUsersWrite},
	{http.MethodGet, "/api/v1/orgs", model.PermOrgsRead},
	{http.MethodDelete, "/api/v1/orgs/1", model.PermOrgsDelete},
	{http.MethodGet, "/api/v1/repos", model.PermReposRead},
	{http.MethodPost, "/api/v1/repos/repair", model.PermReposRepair},
	{http.MethodGet, "/api/v1/pipelines", model.PermQueueRead},
	{http.MethodGet, "/api/v1/queue/info", model.PermQueueRead},
	{http.MethodGet, "/api/v1/queue/norunningpipelines", model.PermQueueRead},
	{http.MethodPost, "/api/v1/queue/pause", model.PermQueueWrite},
	{http.MethodPost, "/api/v1/queue/resume", model.PermQueueWrite},
	{http.MethodPost, "/api/v1/queue/tasks/1/cancel", model.PermQueueWrite},
	{http.MethodGet, "/api/v1/secrets/s/abbreviated", model.PermSecretsWrite},
	{http.MethodPost, "/api/v1/secrets", model.PermSecretsWrite},
	{http.MethodPatch, "/api/v1/secrets/s", model.PermSecretsWrite},
	{http.MethodDelete, "/api/v1/secrets/s", model.PermSecretsWrite},
	{http.MethodPost, "/api/v1/registries", model.PermSecretsWrite},
	{http.MethodPatch, "/api/v1/registries/r", model.PermSecretsWrite},
	{http.MethodDelete, "/api/v1/registries/r", model.PermSecretsWrite},
	{http.MethodPost, "/api/v1/agents", model.PermAgentsRegister},
	{http.MethodPatch, "/api/v1/agents/1", model.PermAgentsWrite},
	{http.MethodDelete, "/api/v1/agents/1", model.PermAgentsWrite},
	{http.MethodPost, "/api/v1/autoscalers", model.PermAgentsRegister},
	{http.MethodPatch, "/api/v1/autoscalers/1", model.PermAgentsWrite},
	{http.MethodDelete, "/api/v1/autoscalers/1", model.PermAgentsWrite},
	{http.MethodPost, "/api/v1/autoscalers/1/token", model.PermAgentsRegister},
	{http.MethodGet, "/api/v1/agents/1/logs/stream", model.PermAgentsRead},
	{http.MethodGet, "/api/v1/autoscalers/1/logs/stream", model.PermAgentsRead},
	{http.MethodGet, "/api/v1/admin/maintenance/config", model.PermMaintenanceRead},
	{http.MethodGet, "/api/v1/admin/maintenance/config/vacuum", model.PermMaintenanceRead},
	{http.MethodGet, "/api/v1/admin/maintenance/stats", model.PermMaintenanceRead},
	{http.MethodGet, "/api/v1/admin/maintenance/logs", model.PermMaintenanceRead},
	{http.MethodPut, "/api/v1/admin/maintenance/config", model.PermMaintenanceWrite},
	{http.MethodPut, "/api/v1/admin/maintenance/config/vacuum", model.PermMaintenanceWrite},
	{http.MethodPost, "/api/v1/admin/maintenance/run", model.PermMaintenanceWrite},
	{http.MethodPost, "/api/v1/forges", model.PermServerAdmin},
	{http.MethodPatch, "/api/v1/forges/1", model.PermServerAdmin},
	{http.MethodDelete, "/api/v1/forges/1", model.PermServerAdmin},
	{http.MethodGet, "/api/v1/admin/notifications/smtp", model.PermServerAdmin},
	{http.MethodPost, "/api/v1/admin/notifications/test", model.PermServerAdmin},
	{http.MethodGet, "/api/v1/log-level", model.PermServerAdmin},
	{http.MethodPost, "/api/v1/log-level", model.PermServerAdmin},
	{http.MethodGet, "/api/v1/debug/pprof/", model.PermServerAdmin},
}

func TestRoutePermissions(t *testing.T) {
	users := map[string]*model.User{
		"user":            {ID: 1, Login: "user"},
		"operator":        {ID: 2, Login: "op", Roles: model.JSONSliceString{"operator"}},
		"secrets-manager": {ID: 3, Login: "sm", Roles: model.JSONSliceString{"secrets-manager"}},
		"auditor":         {ID: 4, Login: "aud", Roles: model.JSONSliceString{"auditor"}},
		"admin":           {ID: 5, Login: "admin", Admin: true},
	}
	for name, user := range users {
		e := newGuardEngine(t, user)
		for _, r := range guardedRoutes {
			t.Run(name+" "+r.method+" "+r.path, func(t *testing.T) {
				req := httptest.NewRequest(r.method, r.path, strings.NewReader("{}"))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				e.ServeHTTP(w, req)
				if user.Can(r.perm) {
					assert.NotContains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, w.Code, "allowed route denied")
				} else {
					assert.Equal(t, http.StatusForbidden, w.Code)
				}
			})
		}
	}
}

func TestRoutePermissionsAnonymous(t *testing.T) {
	e := newGuardEngine(t, nil)
	for _, r := range guardedRoutes {
		req := httptest.NewRequest(r.method, r.path, strings.NewReader("{}"))
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code, "%s %s", r.method, r.path)
	}
}

// Routes that need only a signed-in user, and the status a signed-in user gets
// from the guard engine, where no handler dependencies are configured.
var userRoutes = []struct {
	method, path string
	status       int
}{
	// LDAP is not configured in the guard engine.
	{http.MethodPost, "/api/v1/user/identities/ldap", http.StatusNotFound},
}

func TestUserRoutes(t *testing.T) {
	for _, r := range userRoutes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			for _, user := range []*model.User{nil, {ID: 1, Login: "user"}} {
				req := httptest.NewRequest(r.method, r.path, strings.NewReader(`{"username":"jane","password":"pw"}`))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				newGuardEngine(t, user).ServeHTTP(w, req)
				if user == nil {
					assert.Equal(t, http.StatusUnauthorized, w.Code, "anonymous")
				} else {
					assert.Equal(t, r.status, w.Code, "signed in")
				}
			}
		})
	}
}
