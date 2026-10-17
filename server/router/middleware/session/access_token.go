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
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
)

const (
	accessTokenKey      = "access_token"
	accessTokenScopeKey = "access_token_scope"
)

// ScopeRequirementKind tells how a route treats requests authenticated with an
// access token.
type ScopeRequirementKind int

const (
	// ScopeDenied rejects access tokens on the route. It is also what routes
	// without an entry in the scope table get.
	ScopeDenied ScopeRequirementKind = iota
	// ScopeRequired needs a token scope that satisfies ScopeRequirement.Scope.
	ScopeRequired
	// ScopePublic serves the route without the token's identity: the request
	// continues as an anonymous one, unless the token satisfies
	// ScopeRequirement.Scope (when set) and, if Unrestricted is set, is not
	// limited to an org or a repo, in which case the user is kept.
	ScopePublic
)

// ScopeRequirement describes what an access token needs to use a route.
type ScopeRequirement struct {
	Kind  ScopeRequirementKind
	Scope model.AccessTokenScope
	// Unrestricted additionally rejects tokens limited to an org or a repo.
	Unrestricted bool
}

// ScopeResolver returns the requirement of a route, given the request method
// and the gin route pattern relative to the server root path. The boolean is
// false when the route is unknown.
type ScopeResolver func(method, path string) (ScopeRequirement, bool)

// AccessToken returns the access token the request authenticated with, or nil
// for every other kind of authentication.
func AccessToken(c *gin.Context) *model.AccessToken {
	v, ok := c.Get(accessTokenKey)
	if !ok {
		return nil
	}
	at, ok := v.(*model.AccessToken)
	if !ok {
		return nil
	}
	return at
}

// AccessTokenAllows reports whether the request may use scope. It is true for
// every request not authenticated with an access token.
func AccessTokenAllows(c *gin.Context, scope model.AccessTokenScope) bool {
	at := AccessToken(c)
	return at == nil || at.Allows(scope)
}

// AccessTokenAllowsRepo reports whether the org/repo restriction of the
// request's access token admits repo. It is true for every request not
// authenticated with an access token.
func AccessTokenAllowsRepo(c *gin.Context, repo *model.Repo) bool {
	at := AccessToken(c)
	return at == nil || at.AllowsRepo(repo)
}

// AccessTokenAllowsOrg reports whether the org/repo restriction of the
// request's access token admits org. A token limited to one org admits only
// that org. A token limited to one repo admits only the org owning the repo,
// and only for routes that need no more than repo:read: it must not reach the
// org's secrets, registries or agents.
func AccessTokenAllowsOrg(c *gin.Context, org *model.Org) bool {
	at := AccessToken(c)
	if at == nil || at.IsUserLevel() {
		return true
	}
	if org == nil {
		return false
	}
	if at.OrgID != 0 && org.ID != at.OrgID {
		return false
	}
	if at.RepoID == 0 {
		return true
	}
	if scope, _ := c.Get(accessTokenScopeKey); scope != model.ScopeRepoRead {
		return false
	}
	repo, err := store.FromContext(c).GetRepo(at.RepoID)
	if err != nil {
		return false
	}
	return repo.OrgID == org.ID
}

// EnforceAccessTokenScopes applies the scopes and the org/repo restriction of
// access tokens. Requests authenticated any other way pass unchanged. The route
// pattern of the request, without rootPath, is looked up with resolve; routes it
// does not know are denied, so a new route is unreachable with an access token
// until it is given a requirement.
func EnforceAccessTokenScopes(rootPath string, resolve ScopeResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		at := AccessToken(c)
		if at == nil {
			c.Next()
			return
		}

		path := strings.TrimPrefix(c.FullPath(), rootPath)
		req, ok := resolve(c.Request.Method, path)
		if !ok {
			req = ScopeRequirement{Kind: ScopeDenied}
		}

		switch req.Kind {
		case ScopePublic:
			if req.Scope == "" || !at.Allows(req.Scope) || (req.Unrestricted && !at.IsUserLevel()) {
				// Continue as an anonymous request.
				c.Set("user", nil)
				c.Set(accessTokenKey, nil)
			}
		case ScopeRequired:
			if !at.Allows(req.Scope) {
				log.Debug().Int64("token_id", at.ID).Str("path", path).Str("scope", string(req.Scope)).Msg("access token lacks scope")
				c.String(http.StatusForbidden, "Access token scope %s required", req.Scope)
				c.Abort()
				return
			}
			if req.Unrestricted && !at.IsUserLevel() {
				c.String(http.StatusForbidden, "Access token restricted to an organization or repository cannot use this endpoint")
				c.Abort()
				return
			}
			c.Set(accessTokenScopeKey, req.Scope)
		default:
			c.String(http.StatusForbidden, "Access tokens cannot use this endpoint")
			c.Abort()
			return
		}
		c.Next()
	}
}
