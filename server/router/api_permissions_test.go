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
	"github.com/stretchr/testify/require"

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
	return newGuardEngineWithAutoscaler(t, user, nil)
}

func newGuardEngineWithAutoscaler(t *testing.T, user *model.User, autoscaler *model.Autoscaler) *gin.Engine {
	t.Helper()
	return newTestEngine(t, guardStore{}, user, autoscaler)
}

func newTestEngine(t *testing.T, s store.Store, user *model.User, autoscaler *model.Autoscaler) *gin.Engine {
	t.Helper()
	// The /debug group is only registered at debug log level.
	prev := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	t.Cleanup(func() { zerolog.SetGlobalLevel(prev) })

	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(gin.Recovery())
	e.Use(func(c *gin.Context) {
		store.ToContext(c, s)
		if user != nil {
			c.Set("user", user)
		}
		if autoscaler != nil {
			c.Set("autoscaler", autoscaler)
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
	{http.MethodPost, "/api/v1/agents/1/restart", model.PermAgentsWrite},
	{http.MethodDelete, "/api/v1/agents/1/restart", model.PermAgentsWrite},
	{http.MethodPost, "/api/v1/agents/restarts", model.PermAgentsWrite},
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

// Routes that need a signed-in user; the guard store has no watch data, so a
// handler reached by the user ends in a 500 from the store panic.
var userWatchRoutes = []struct {
	method, path string
}{
	{http.MethodGet, "/api/v1/user/watches"},
	{http.MethodPost, "/api/v1/user/watches/read"},
	{http.MethodPost, "/api/v1/user/watches/1/read"},
	{http.MethodPatch, "/api/v1/user/watches/1"},
	{http.MethodDelete, "/api/v1/user/watches/1"},
	{http.MethodGet, "/api/v1/repos/1/watches"},
	{http.MethodPost, "/api/v1/repos/1/watches"},
	{http.MethodGet, "/api/v1/repos/1/watches/suggestions"},
}

func TestWatchRoutesRequireUser(t *testing.T) {
	for _, r := range userWatchRoutes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			req := httptest.NewRequest(r.method, r.path, strings.NewReader(`{"in_app":true}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			newGuardEngine(t, nil).ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
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

// selfStore records the user saved by PATCH /user.
type selfStore struct {
	guardStore
	saved *model.User
}

func (s *selfStore) UpdateUser(u *model.User) error {
	s.saved = u
	return nil
}

// A user without users:write must be able to save their own profile (issue
// #1644), without being able to grant themselves admin or roles on the way.
func TestPatchSelfRoute(t *testing.T) {
	user := &model.User{ID: 1, Login: "user", Timezone: "UTC"}
	s := &selfStore{}
	body := `{"description":"hi","timezone":"Europe/Berlin","admin":true,"roles":["operator"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/user", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newTestEngine(t, s, user, nil).ServeHTTP(w, req)

	// Huma ignores the admin and roles fields, which PatchSelfBody does not have.
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "hi", s.saved.Description)
	assert.Equal(t, "Europe/Berlin", s.saved.Timezone)
	assert.False(t, s.saved.Admin)
	assert.Empty(t, s.saved.Roles)

	req = httptest.NewRequest(http.MethodPatch, "/api/v1/user", strings.NewReader(`{"description":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	newTestEngine(t, &selfStore{}, nil, nil).ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "anonymous")
}

// The heartbeat route must only accept the token of the autoscaler it reports
// for, regardless of any user session on the request.
func TestAutoscalerHeartbeatRequiresOwnToken(t *testing.T) {
	const heartbeatBody = `{"active_agents":1,"pending_agents":0,"version":"v1","min_agents":0,"max_agents":5,` +
		`"workflows_per_agent":1,"provider":"hetznercloud","instance_type":"cx22","region":"fsn1",` +
		`"agent_labels":{},"limit_cpu_quota":"","limit_mem":""}`
	tests := []struct {
		name       string
		user       *model.User
		autoscaler *model.Autoscaler
		denied     int
	}{
		{name: "anonymous", denied: http.StatusUnauthorized},
		{name: "admin user without autoscaler token", user: &model.User{ID: 1, Admin: true}, denied: http.StatusUnauthorized},
		{name: "token of another autoscaler", autoscaler: &model.Autoscaler{ID: 2}, denied: http.StatusForbidden},
		{name: "own token", autoscaler: &model.Autoscaler{ID: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newGuardEngineWithAutoscaler(t, tt.user, tt.autoscaler)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/autoscalers/1/heartbeat", strings.NewReader(heartbeatBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			e.ServeHTTP(w, req)
			if tt.denied != 0 {
				assert.Equal(t, tt.denied, w.Code)
			} else {
				// The guard store panics on AutoscalerFind, so reaching the
				// handler's store lookup surfaces as a 500.
				assert.Equal(t, http.StatusInternalServerError, w.Code)
			}
		})
	}
}

// A host being provisioned reports progress with its agent token; a user
// session alone must not be enough.
func TestAgentProvisioningRequiresAgentToken(t *testing.T) {
	const body = `{"phase":"Installing Hyper-V"}`
	for _, user := range []*model.User{nil, {ID: 1, Admin: true}} {
		e := newGuardEngineWithAutoscaler(t, user, nil)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/provisioning", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	}

	// With a bearer token the handler looks the agent up; the guard store
	// panics on any store call, which surfaces as a 500.
	e := newGuardEngineWithAutoscaler(t, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/provisioning", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer agent-token")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
