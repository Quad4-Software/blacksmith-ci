// Copyright 2018 Drone.IO Inc.
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
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v5/server"
	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/store"
	"codeberg.org/crowci/crow/v5/server/store/datastore"
	"codeberg.org/crowci/crow/v5/shared/token"
)

func User(c *gin.Context) *model.User {
	v, ok := c.Get("user")
	if !ok {
		return nil
	}
	u, ok := v.(*model.User)
	if !ok {
		return nil
	}
	return u
}

func SetUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		var user *model.User
		var accessToken *model.AccessToken

		t, err := token.ParseRequest([]token.Type{token.UserToken, token.SessToken, token.AccessToken}, c.Request, func(t *token.Token) (string, error) {
			var err error
			_store := store.FromContext(c)

			// Handle access token authentication
			if t.Type == token.AccessToken {
				tokenHash := t.Get("token-hash")
				if tokenHash == "" {
					return "", model.ErrAccessTokenScopesInvalid
				}

				accessToken, err = _store.AccessTokenFindByHash(tokenHash)
				if err != nil {
					return "", err
				}

				// Check expiration
				if accessToken.IsExpired() {
					return "", model.ErrAccessTokenExpired
				}

				// Load the user
				user, err = _store.GetUser(accessToken.UserID)
				if err != nil {
					return "", err
				}

				return user.Hash, nil
			}

			// Handle traditional user/session token authentication
			userID, err := strconv.ParseInt(t.Get("user-id"), 10, 64)
			if err != nil {
				return "", err
			}
			user, err = _store.GetUser(userID)
			return user.Hash, err
		})
		if err == nil {
			c.Set("user", user)

			// Store access token in context for scope checking
			if accessToken != nil {
				c.Set("access_token", accessToken)

				// Update last used timestamp asynchronously
				go func(tokenID int64) {
					// Create a fresh store connection for the goroutine
					if err := store.FromContext(c).AccessTokenUpdateLastUsed(tokenID, time.Now().Unix()); err != nil {
						log.Debug().Err(err).Int64("token_id", tokenID).Msg("Failed to update access token last used")
					}
				}(accessToken.ID)
			}

			// if this is a session token (ie not the API token)
			// this means the user is accessing with a web browser,
			// so we should implement CSRF protection measures.
			if t.Type == token.SessToken {
				err = token.CheckCsrf(c.Request, func(_ *token.Token) (string, error) {
					return user.Hash, nil
				})
				// if csrf token validation fails, exit immediately
				// with a not authorized error.
				if err != nil {
					c.AbortWithStatus(http.StatusUnauthorized)
					return
				}
			}
		}
		c.Next()
	}
}

// AccessToken returns the access token from the context, if present.
func AccessToken(c *gin.Context) *model.AccessToken {
	v, ok := c.Get("access_token")
	if !ok {
		return nil
	}
	t, ok := v.(*model.AccessToken)
	if !ok {
		return nil
	}
	return t
}

// HasAccessTokenScope checks if the current request has an access token with the required scope.
// If no access token is present (regular user/session token), it returns true (full access).
func HasAccessTokenScope(c *gin.Context, required model.AccessTokenScope) bool {
	accessToken := AccessToken(c)
	if accessToken == nil {
		// No access token means regular authentication with full permissions
		return true
	}
	return accessToken.HasScope(required)
}

// MustAccessTokenScope creates middleware that requires a specific scope.
// If using a regular user/session token, the request is allowed.
// If using an access token, it must have the required scope.
func MustAccessTokenScope(required model.AccessTokenScope) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !HasAccessTokenScope(c, required) {
			c.String(http.StatusForbidden, "Access token missing required scope: %s", required)
			c.Abort()
			return
		}
		c.Next()
	}
}

// ValidateAccessTokenEntityScope checks if the access token is allowed to access the given entity.
// Returns true if allowed, false otherwise.
func ValidateAccessTokenEntityScope(c *gin.Context, repoID int64, orgID int64) bool {
	accessToken := AccessToken(c)
	if accessToken == nil {
		// No access token means regular authentication with full permissions
		return true
	}

	// User-level token has access to everything the user has access to
	if accessToken.IsUserLevel() {
		return true
	}

	// Repo-scoped token must match the repo
	if accessToken.IsRepoScoped() {
		return accessToken.RepoID == repoID
	}

	// Org-scoped token must match the org
	if accessToken.IsOrgScoped() {
		return accessToken.OrgID == orgID
	}

	return false
}

// accessTokenVerifier is used for verifying access tokens by hash.
// This avoids needing to look up the token twice.
var _ = datastore.HashAccessToken // Ensure import is used

func MustAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := User(c)
		switch {
		case user == nil:
			c.String(http.StatusUnauthorized, "User not authorized")
			c.Abort()
		case !user.Admin:
			c.String(http.StatusForbidden, "User not authorized")
			c.Abort()
		default:
			c.Next()
		}
	}
}

func MustRepoAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := User(c)
		perm := Perm(c)
		switch {
		case user == nil:
			c.String(http.StatusUnauthorized, "User not authorized")
			c.Abort()
		case !perm.Admin:
			c.String(http.StatusForbidden, "User not authorized")
			c.Abort()
		default:
			c.Next()
		}
	}
}

func MustUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := User(c)
		if user == nil {
			c.String(http.StatusUnauthorized, "User not authorized")
			c.Abort()
			return
		}
		c.Next()
	}
}

func MustOrgMember(admin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := User(c)
		if user == nil {
			c.String(http.StatusUnauthorized, "User not authorized")
			c.Abort()
			return
		}

		org := Org(c)
		if org == nil {
			c.String(http.StatusBadRequest, "Organization not loaded")
			c.Abort()
			return
		}

		// User can access his own, admin can access all
		if (org.Name == user.Login) || user.Admin {
			c.Next()
			return
		}

		_forge, err := server.Config.Services.Manager.ForgeFromUser(user)
		if err != nil {
			log.Error().Err(err).Msg("Cannot get forge from user")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		perm, err := server.Config.Services.Membership.Get(c, _forge, user, org.Name)
		if err != nil {
			log.Error().Err(err).Msg("failed to check membership")
			c.String(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			c.Abort()
			return
		}

		if perm == nil || (!admin && !perm.Member) || (admin && !perm.Admin) {
			c.String(http.StatusForbidden, "user not authorized")
			c.Abort()
			return
		}

		c.Next()
	}
}
