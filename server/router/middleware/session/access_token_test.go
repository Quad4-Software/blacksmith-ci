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

package session

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
)

// The fixedResolver serves one requirement for the route "/r" and knows no other route.
func fixedResolver(req ScopeRequirement) ScopeResolver {
	return func(_, path string) (ScopeRequirement, bool) {
		if path == "/r" {
			return req, true
		}
		return ScopeRequirement{}, false
	}
}

// The serveScoped helper routes method /path through EnforceAccessTokenScopes
// and reports the status and whether the handler ran with a user.
func serveScoped(t *testing.T, resolve ScopeResolver, at *model.AccessToken, method, path string) (int, bool, bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(func(c *gin.Context) {
		c.Set("user", &model.User{ID: 1, Login: "alice"})
		if at != nil {
			c.Set(accessTokenKey, at)
		}
	})
	e.Use(EnforceAccessTokenScopes("/root", resolve))
	ran, hasUser := false, false
	handler := func(c *gin.Context) {
		ran = true
		hasUser = User(c) != nil
		c.Status(http.StatusOK)
	}
	e.Handle(method, "/root/r", handler)
	e.Handle(method, "/root/unknown", handler)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w.Code, ran, hasUser
}

func TestEnforceAccessTokenScopes(t *testing.T) {
	required := func(s model.AccessTokenScope) ScopeRequirement {
		return ScopeRequirement{Kind: ScopeRequired, Scope: s}
	}
	repoRead := &model.AccessToken{Scopes: model.JSONSliceString{"repo:read"}}
	repoAdmin := &model.AccessToken{Scopes: model.JSONSliceString{"repo:admin"}}
	restricted := &model.AccessToken{RepoID: 5, Scopes: model.JSONSliceString{"repo:admin"}}

	tests := []struct {
		name     string
		req      ScopeRequirement
		token    *model.AccessToken
		path     string
		status   int
		withUser bool
	}{
		{"no access token passes unchanged", ScopeRequirement{Kind: ScopeDenied}, nil, "/root/r", http.StatusOK, true},
		{"scope satisfied", required(model.ScopeRepoRead), repoRead, "/root/r", http.StatusOK, true},
		{"higher scope satisfies lower", required(model.ScopeRepoWrite), repoAdmin, "/root/r", http.StatusOK, true},
		{"scope missing", required(model.ScopeRepoWrite), repoRead, "/root/r", http.StatusForbidden, false},
		{"other family", required(model.ScopeUserRead), repoAdmin, "/root/r", http.StatusForbidden, false},
		{"denied route", ScopeRequirement{Kind: ScopeDenied}, repoAdmin, "/root/r", http.StatusForbidden, false},
		{"unknown route is denied", required(model.ScopeRepoRead), repoAdmin, "/root/unknown", http.StatusForbidden, false},
		{"unrestricted route, user-level token", ScopeRequirement{Kind: ScopeRequired, Scope: model.ScopeRepoAdmin, Unrestricted: true}, repoAdmin, "/root/r", http.StatusOK, true},
		{"unrestricted route, restricted token", ScopeRequirement{Kind: ScopeRequired, Scope: model.ScopeRepoAdmin, Unrestricted: true}, restricted, "/root/r", http.StatusForbidden, false},
		{"public route runs anonymously", ScopeRequirement{Kind: ScopePublic}, repoAdmin, "/root/r", http.StatusOK, false},
		{"public route keeps user with scope", ScopeRequirement{Kind: ScopePublic, Scope: model.ScopeRepoRead}, repoRead, "/root/r", http.StatusOK, true},
		{"public route drops user without scope", ScopeRequirement{Kind: ScopePublic, Scope: model.ScopeAdminRead}, repoRead, "/root/r", http.StatusOK, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, ran, hasUser := serveScoped(t, fixedResolver(tt.req), tt.token, http.MethodGet, tt.path)
			assert.Equal(t, tt.status, status)
			assert.Equal(t, tt.status == http.StatusOK, ran)
			assert.Equal(t, tt.withUser, hasUser)
		})
	}
}

func TestSetRepoAccessTokenRestriction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newSessionStore(t)
	repo := &model.Repo{ForgeRemoteID: "1", Owner: "org", Name: "a", FullName: "org/a", OrgID: 2, ForgeID: 1}
	require.NoError(t, s.CreateRepo(repo))

	tests := []struct {
		name   string
		token  *model.AccessToken
		status int
	}{
		{"no token", nil, http.StatusOK},
		{"user-level token", &model.AccessToken{}, http.StatusOK},
		{"same repo", &model.AccessToken{RepoID: repo.ID}, http.StatusOK},
		{"other repo", &model.AccessToken{RepoID: repo.ID + 1}, http.StatusForbidden},
		{"same org", &model.AccessToken{OrgID: 2}, http.StatusOK},
		{"other org", &model.AccessToken{OrgID: 3}, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := gin.New()
			e.Use(func(c *gin.Context) {
				store.ToContext(c, s)
				c.Set("user", &model.User{ID: 1})
				if tt.token != nil {
					c.Set(accessTokenKey, tt.token)
				}
			})
			e.GET("/repos/:repo_id", SetRepo(), func(c *gin.Context) { c.Status(http.StatusOK) })
			w := httptest.NewRecorder()
			e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/repos/1", nil))
			assert.Equal(t, tt.status, w.Code)
		})
	}
}

func TestAccessTokenAllowsOrg(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newSessionStore(t)
	repo := &model.Repo{ForgeRemoteID: "1", Owner: "org", Name: "a", FullName: "org/a", OrgID: 2, ForgeID: 1}
	require.NoError(t, s.CreateRepo(repo))
	org := &model.Org{ID: 2}
	other := &model.Org{ID: 3}

	tests := []struct {
		name  string
		token *model.AccessToken
		scope model.AccessTokenScope
		org   *model.Org
		want  bool
	}{
		{"no token", nil, model.ScopeRepoAdmin, org, true},
		{"user-level token", &model.AccessToken{}, model.ScopeRepoAdmin, other, true},
		{"org token, same org", &model.AccessToken{OrgID: 2}, model.ScopeRepoAdmin, org, true},
		{"org token, other org", &model.AccessToken{OrgID: 2}, model.ScopeRepoRead, other, false},
		{"repo token, owning org, read", &model.AccessToken{RepoID: repo.ID}, model.ScopeRepoRead, org, true},
		{"repo token, owning org, admin route", &model.AccessToken{RepoID: repo.ID}, model.ScopeRepoAdmin, org, false},
		{"repo token, other org", &model.AccessToken{RepoID: repo.ID}, model.ScopeRepoRead, other, false},
		{"restricted token, no org", &model.AccessToken{OrgID: 2}, model.ScopeRepoRead, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			store.ToContext(c, s)
			if tt.token != nil {
				c.Set(accessTokenKey, tt.token)
			}
			c.Set(accessTokenScopeKey, tt.scope)
			assert.Equal(t, tt.want, AccessTokenAllowsOrg(c, tt.org))
		})
	}
}
