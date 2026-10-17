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
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/bootstrap"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/datastore"
	"codefloe.com/crowci/crow/v6/shared/token"
)

// loadAllRoutes builds the router with every optional route group enabled.
func loadAllRoutes(t *testing.T, s store.Store) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	prevLevel := zerolog.GlobalLevel()
	prevSwagger := server.Config.WebUI.EnableSwagger
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	server.Config.WebUI.EnableSwagger = true
	t.Cleanup(func() {
		zerolog.SetGlobalLevel(prevLevel)
		server.Config.WebUI.EnableSwagger = prevSwagger
	})
	e, ok := Load(http.NotFound, middleware.Store(s)).(*gin.Engine)
	require.True(t, ok)
	return e
}

// TestAccessTokenScopeCoverage makes sure every registered route has an entry
// in the scope table. A route without one is unreachable with an access token,
// so add a rule to scopeRules when this test fails.
func TestAccessTokenScopeCoverage(t *testing.T) {
	e := loadAllRoutes(t, guardStore{})
	routes := e.Routes()
	require.NotEmpty(t, routes)
	for _, r := range routes {
		req, ok := resolveAccessTokenScope(r.Method, r.Path)
		if assert.True(t, ok, "no access token scope rule for %s %s", r.Method, r.Path) && req.Kind == session.ScopeRequired {
			assert.True(t, req.Scope.IsValid(), "%s %s resolves to invalid scope %q", r.Method, r.Path, req.Scope)
		}
	}
	// The web UI is served by the NoRoute handler, which has no route pattern.
	req, ok := resolveAccessTokenScope(http.MethodGet, "")
	assert.True(t, ok)
	assert.Equal(t, session.ScopePublic, req.Kind)
}

func TestResolveAccessTokenScope(t *testing.T) {
	required := func(s model.AccessTokenScope) session.ScopeRequirement {
		return session.ScopeRequirement{Kind: session.ScopeRequired, Scope: s}
	}
	public := session.ScopeRequirement{Kind: session.ScopePublic}
	// User and admin routes reach beyond a token's org/repo limit.
	unlimited := func(s model.AccessTokenScope) session.ScopeRequirement {
		return session.ScopeRequirement{Kind: session.ScopeRequired, Scope: s, Unrestricted: true}
	}

	tests := []struct {
		method, path string
		want         session.ScopeRequirement
	}{
		// Repositories.
		{http.MethodGet, "/api/v1/repos/:repo_id", required(model.ScopeRepoRead)},
		{http.MethodHead, "/api/v1/repos/:repo_id/pipelines", required(model.ScopeRepoRead)},
		{http.MethodPost, "/api/v1/repos/:repo_id/pipelines", required(model.ScopeRepoWrite)},
		{http.MethodPost, "/api/v1/repos/:repo_id/pipelines/:number/cancel", required(model.ScopeRepoWrite)},
		{http.MethodDelete, "/api/v1/repos/:repo_id/pipelines/:number", required(model.ScopeRepoAdmin)},
		{http.MethodGet, "/api/v1/repos/:repo_id/secrets", required(model.ScopeRepoAdmin)},
		{http.MethodGet, "/api/v1/repos/:repo_id/registries/:registry", required(model.ScopeRepoAdmin)},
		{http.MethodPatch, "/api/v1/repos/:repo_id", required(model.ScopeRepoAdmin)},
		{http.MethodDelete, "/api/v1/repos/:repo_id", required(model.ScopeRepoAdmin)},
		{http.MethodPost, "/api/v1/repos/:repo_id/chown", required(model.ScopeRepoAdmin)},
		{http.MethodGet, "/api/v1/repos/lookup/*repo_full_name", required(model.ScopeRepoRead)},
		{http.MethodPost, "/api/v1/repos", session.ScopeRequirement{Kind: session.ScopeRequired, Scope: model.ScopeRepoAdmin, Unrestricted: true}},
		{http.MethodGet, "/api/v1/stream/logs/:repo_id/:pipeline/:stepId", required(model.ScopeRepoRead)},
		{http.MethodGet, "/api/v1/stream/events", required(model.ScopeRepoRead)},
		// Organizations.
		{http.MethodGet, "/api/v1/orgs/:org_id", required(model.ScopeRepoRead)},
		{http.MethodGet, "/api/v1/orgs/:org_id/secrets", required(model.ScopeRepoAdmin)},
		{http.MethodPost, "/api/v1/orgs/:org_id/agents", required(model.ScopeRepoAdmin)},
		{http.MethodDelete, "/api/v1/orgs/:org_id", unlimited(model.ScopeAdminWrite)},
		{http.MethodGet, "/api/v1/orgs", unlimited(model.ScopeAdminRead)},
		// User.
		{http.MethodGet, "/api/v1/user", required(model.ScopeUserRead)},
		{http.MethodGet, "/api/v1/user/access-tokens", unlimited(model.ScopeUserRead)},
		{http.MethodPost, "/api/v1/user/access-tokens", unlimited(model.ScopeUserWrite)},
		{http.MethodPost, "/api/v1/user/token", session.ScopeRequirement{Kind: session.ScopeDenied}},
		{http.MethodGet, "/api/v1/secrets", unlimited(model.ScopeUserRead)},
		{http.MethodGet, "/api/v1/agents", unlimited(model.ScopeUserRead)},
		// Admin.
		{http.MethodGet, "/api/v1/users", unlimited(model.ScopeAdminRead)},
		{http.MethodPost, "/api/v1/users", unlimited(model.ScopeAdminWrite)},
		{http.MethodGet, "/api/v1/secrets/:secret/abbreviated", unlimited(model.ScopeAdminRead)},
		{http.MethodPost, "/api/v1/secrets", unlimited(model.ScopeAdminWrite)},
		{http.MethodPost, "/api/v1/queue/pause", unlimited(model.ScopeAdminWrite)},
		{http.MethodGet, "/api/v1/agents/:agent_id/logs/stream", unlimited(model.ScopeAdminRead)},
		{http.MethodDelete, "/api/v1/forges/:forgeId", unlimited(model.ScopeAdminWrite)},
		// Public.
		{http.MethodGet, "/api/v1/timezones", public},
		{http.MethodGet, "/api/v1/badges/:repo_id_or_owner/status.svg", public},
		{http.MethodGet, "/api/v1/forges", session.ScopeRequirement{Kind: session.ScopePublic, Scope: model.ScopeAdminRead, Unrestricted: true}},
		{http.MethodPost, "/api/v1/hook", public},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			got, ok := resolveAccessTokenScope(tt.method, tt.path)
			assert.True(t, ok)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("unknown route", func(t *testing.T) {
		got, ok := resolveAccessTokenScope(http.MethodGet, "/api/v1/brand-new")
		assert.False(t, ok)
		assert.Equal(t, session.ScopeDenied, got.Kind)
	})
}

// The scopeFixture holds a router backed by an in-memory store with a user, two
// orgs and a repo in each org.
type scopeFixture struct {
	t            *testing.T
	store        store.Store
	engine       *gin.Engine
	user         *model.User
	repoA, repoB *model.Repo
	minted       int
}

func newScopeFixture(t *testing.T) *scopeFixture {
	t.Helper()
	s, err := datastore.NewEngine(&store.Opts{Driver: "sqlite3", Config: ":memory:"})
	require.NoError(t, err)
	require.NoError(t, s.Migrate(context.Background(), false))
	t.Cleanup(func() { _ = s.Close() })

	// An admin, so that only the token scopes can deny a request.
	user := &model.User{Login: "alice", Hash: "alice-hash", ForgeID: 1, ForgeRemoteID: "1", Admin: true}
	require.NoError(t, s.CreateUser(user))
	orgA := &model.Org{Name: "org-a", ForgeID: 1}
	orgB := &model.Org{Name: "org-b", ForgeID: 1}
	require.NoError(t, s.OrgCreate(orgA))
	require.NoError(t, s.OrgCreate(orgB))
	repoA := &model.Repo{ForgeRemoteID: "10", Owner: "org-a", Name: "a", FullName: "org-a/a", OrgID: orgA.ID, ForgeID: 1, UserID: user.ID}
	repoB := &model.Repo{ForgeRemoteID: "20", Owner: "org-b", Name: "b", FullName: "org-b/b", OrgID: orgB.ID, ForgeID: 1, UserID: user.ID}
	require.NoError(t, s.CreateRepo(repoA))
	require.NoError(t, s.CreateRepo(repoB))

	return &scopeFixture{t: t, store: s, engine: loadAllRoutes(t, s), user: user, repoA: repoA, repoB: repoB}
}

// The mint helper creates an access token for the fixture user and returns its JWT.
func (f *scopeFixture) mint(orgID, repoID int64, scopes ...model.AccessTokenScope) string {
	f.t.Helper()
	f.minted++
	hash := datastore.HashAccessToken(f.t.Name() + "-" + strconv.Itoa(f.minted))
	at := &model.AccessToken{UserID: f.user.ID, Name: "t", TokenHash: hash, OrgID: orgID, RepoID: repoID}
	for _, s := range scopes {
		at.Scopes = append(at.Scopes, string(s))
	}
	require.NoError(f.t, f.store.AccessTokenCreate(at))
	jwt := token.New(token.AccessToken)
	jwt.Set("token-hash", hash)
	signed, err := jwt.Sign(f.user.Hash)
	require.NoError(f.t, err)
	return signed
}

func (f *scopeFixture) do(bearer, method, path, body string) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	f.engine.ServeHTTP(w, req)
	return w
}

func repoPath(r *model.Repo, suffix string) string {
	return "/api/v1/repos/" + strconv.FormatInt(r.ID, 10) + suffix
}

// isDenied reports whether the response is a scope or restriction denial. Allowed
// requests may still fail further down (e.g. with no forge configured), but
// never with 401 or 403.
func isDenied(w *httptest.ResponseRecorder) bool {
	return w.Code == http.StatusForbidden || w.Code == http.StatusUnauthorized
}

func TestAccessTokenScopeEnforcement(t *testing.T) {
	f := newScopeFixture(t)

	repoRead := f.mint(0, 0, model.ScopeRepoRead)
	userRead := f.mint(0, 0, model.ScopeUserRead)
	repoAdmin := f.mint(0, 0, model.ScopeRepoAdmin)
	repoWrite := f.mint(0, 0, model.ScopeRepoWrite)
	repoReadOnA := f.mint(0, f.repoA.ID, model.ScopeRepoRead)
	userReadOnA := f.mint(0, f.repoA.ID, model.ScopeUserRead, model.ScopeRepoRead)
	repoAdminOnOrgA := f.mint(f.repoA.OrgID, 0, model.ScopeRepoAdmin)

	tests := []struct {
		name         string
		bearer       string
		method, path string
		denied       bool
	}{
		{"repo:read cannot read the user", repoRead, http.MethodGet, "/api/v1/user", true},
		{"user:read reads the user", userRead, http.MethodGet, "/api/v1/user", false},
		{"user:read cannot read a repo", userRead, http.MethodGet, repoPath(f.repoA, ""), true},
		{"repo:read cannot create a pipeline", repoRead, http.MethodPost, repoPath(f.repoA, "/pipelines"), true},
		{"repo:write cannot list secrets", repoWrite, http.MethodGet, repoPath(f.repoA, "/secrets"), true},
		{"repo:write cannot change settings", repoWrite, http.MethodPatch, repoPath(f.repoA, ""), true},
		{"repo:admin cannot list users", repoAdmin, http.MethodGet, "/api/v1/users", true},
		{"repo token reads its repo", repoReadOnA, http.MethodGet, repoPath(f.repoA, ""), false},
		{"repo token cannot read another repo", repoReadOnA, http.MethodGet, repoPath(f.repoB, ""), true},
		{"repo token cannot look up another repo", repoReadOnA, http.MethodGet, "/api/v1/repos/lookup/org-b/b", true},
		{"repo token cannot stream another repo's logs", repoReadOnA, http.MethodGet, "/api/v1/stream/logs/" + strconv.FormatInt(f.repoB.ID, 10) + "/1/1", true},
		{"repo token reads its org", repoReadOnA, http.MethodGet, "/api/v1/orgs/" + strconv.FormatInt(f.repoA.OrgID, 10), false},
		{"repo token cannot read another org", repoReadOnA, http.MethodGet, "/api/v1/orgs/" + strconv.FormatInt(f.repoB.OrgID, 10), true},
		{"org token reads a repo of its org", repoAdminOnOrgA, http.MethodGet, repoPath(f.repoA, ""), false},
		{"org token cannot read a repo of another org", repoAdminOnOrgA, http.MethodGet, repoPath(f.repoB, ""), true},
		{"org token cannot reach another org's secrets", repoAdminOnOrgA, http.MethodGet, "/api/v1/orgs/" + strconv.FormatInt(f.repoB.OrgID, 10) + "/secrets", true},
		{"restricted token cannot activate repos", repoAdminOnOrgA, http.MethodPost, "/api/v1/repos?forge_remote_id=30", true},
		{"limited token reads its own user", userReadOnA, http.MethodGet, "/api/v1/user", false},
		{"limited token cannot list the user's repos", userReadOnA, http.MethodGet, "/api/v1/user/repos", true},
		{"limited token cannot list agents", userReadOnA, http.MethodGet, "/api/v1/agents", true},
		{"no token can mint a legacy API token", f.mint(0, 0, model.ScopeUserWrite), http.MethodPost, "/api/v1/user/token", true},
		{"public route works with any token", repoRead, http.MethodGet, "/api/v1/timezones", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := f.do(tt.bearer, tt.method, tt.path, "")
			assert.Equal(t, tt.denied, isDenied(w), "status %d: %s", w.Code, w.Body.String())
		})
	}
}

func TestAccessTokenScopesLeaveOtherAuthAlone(t *testing.T) {
	f := newScopeFixture(t)

	t.Run("legacy user token keeps full access", func(t *testing.T) {
		jwt := token.New(token.UserToken)
		jwt.Set("user-id", strconv.FormatInt(f.user.ID, 10))
		signed, err := jwt.Sign(f.user.Hash)
		require.NoError(t, err)
		w := f.do(signed, http.MethodGet, "/api/v1/user", "")
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("bootstrap admin token keeps admin access", func(t *testing.T) {
		const raw = "0123456789abcdef0123456789abcdef"
		require.NoError(t, bootstrap.AdminToken(f.store, raw))
		w := f.do(raw, http.MethodGet, "/api/v1/users", "")
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	})
}

func TestAccessTokenCannotMintBroaderToken(t *testing.T) {
	f := newScopeFixture(t)
	manager := f.mint(0, 0, model.ScopeUserWrite, model.ScopeRepoRead)
	restricted := f.mint(0, f.repoA.ID, model.ScopeUserWrite, model.ScopeRepoRead)

	tests := []struct {
		name   string
		bearer string
		body   string
		status int
	}{
		{"same scope", manager, `{"name":"n","scopes":["repo:read"]}`, http.StatusOK},
		{"broader scope", manager, `{"name":"n","scopes":["repo:write"]}`, http.StatusForbidden},
		{"other family", manager, `{"name":"n","scopes":["user:write","repo:read"]}`, http.StatusOK},
		{"restricted token, unrestricted target", restricted, `{"name":"n","scopes":["repo:read"]}`, http.StatusForbidden},
		{"restricted token, other repo", restricted, `{"name":"n","scopes":["repo:read"],"repo_id":` + strconv.FormatInt(f.repoB.ID, 10) + `}`, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := f.do(tt.bearer, http.MethodPost, "/api/v1/user/access-tokens", tt.body)
			assert.Equal(t, tt.status, w.Code, w.Body.String())
		})
	}

	t.Run("cannot widen itself", func(t *testing.T) {
		tokens, err := f.store.AccessTokenList(f.user.ID, &model.ListOptions{All: true})
		require.NoError(t, err)
		require.NotEmpty(t, tokens)
		self := tokens[0]
		for _, at := range tokens {
			if len(at.Scopes) == 2 && at.RepoID == 0 {
				self = at
			}
		}
		w := f.do(manager, http.MethodPatch, "/api/v1/user/access-tokens/"+strconv.FormatInt(self.ID, 10), `{"scopes":["repo:admin","user:write"]}`)
		assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	})
}
