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
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/datastore"
	"codefloe.com/crowci/crow/v6/shared/httputil"
	"codefloe.com/crowci/crow/v6/shared/token"
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

		// Opaque bootstrap-token path: a non-JWT bearer authenticates only if
		// it resolves to the dedicated system admin user (CROW_ADMIN_TOKEN).
		if raw, err := token.ExtractToken(c.Request); err == nil && raw != "" && strings.Count(raw, ".") != 2 {
			if u, at, ok := resolveOpaqueToken(c, raw); ok {
				applyAccessToken(c, u, at)
			}
			c.Next()
			return
		}

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
			if err != nil {
				return "", err
			}
			return user.Hash, nil
		})
		if err != nil {
			// If we had a session cookie but it was invalid (stale token, deleted user, etc.),
			// clear it so the browser stops sending it on every request.
			if _, cookieErr := c.Request.Cookie("user_sess"); cookieErr == nil {
				log.Warn().Err(err).Msg("clearing invalid session cookie")
				httputil.DelCookie(c.Writer, c.Request, "user_sess")
			}
		}
		if err == nil {
			c.Set("user", user)

			// Store access token in context for scope checking
			if accessToken != nil {
				applyAccessToken(c, user, accessToken)
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

// resolveOpaqueToken looks up a raw (non-JWT) bearer by its hash and returns
// the owning user only when the token belongs to the system admin user and is
// not expired. This is the CROW_ADMIN_TOKEN bootstrap credential path.
func resolveOpaqueToken(c *gin.Context, raw string) (*model.User, *model.AccessToken, bool) {
	_store := store.FromContext(c)

	at, err := _store.AccessTokenFindByHash(datastore.HashAccessToken(raw))
	if err != nil {
		return nil, nil, false
	}
	user, err := _store.GetUser(at.UserID)
	if err != nil {
		return nil, nil, false
	}
	if user.Login != model.SystemAdminLogin {
		return nil, nil, false
	}
	if at.IsExpired() {
		return nil, nil, false
	}
	return user, at, true
}

// applyAccessToken sets the authenticated user and access token on the context
// and records the token's last-used timestamp asynchronously.
func applyAccessToken(c *gin.Context, user *model.User, at *model.AccessToken) {
	c.Set("user", user)
	c.Set("access_token", at)

	go func(tokenID int64) {
		if err := store.FromContext(c).AccessTokenUpdateLastUsed(tokenID, time.Now().Unix()); err != nil {
			log.Debug().Err(err).Int64("token_id", tokenID).Msg("Failed to update access token last used")
		}
	}(at.ID)
}

func MustAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := User(c)
		switch {
		case user == nil:
			c.String(http.StatusUnauthorized, "User not authorized")
			c.Abort()
		case !user.Admin:
			c.String(http.StatusForbidden, "Instance admin privileges required")
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
			c.String(http.StatusForbidden, "Repository admin privileges required")
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

		_forge, err := server.Config.Services.Manager.ForgeByID(org.ForgeID)
		if err != nil {
			log.Error().Err(err).Int64("forge_id", org.ForgeID).Msg("Cannot get forge by ID for org")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		// Build a user with the correct forge credentials for this org's forge
		forgeUser := user
		if uf, ufErr := store.FromContext(c).UserForgeGet(user.ID, org.ForgeID); ufErr == nil && uf != nil {
			forgeUserCopy := *user
			forgeUserCopy.ForgeID = org.ForgeID
			forgeUserCopy.AccessToken = uf.AccessToken
			forgeUserCopy.RefreshToken = uf.RefreshToken
			forgeUserCopy.Expiry = uf.Expiry
			forgeUser = &forgeUserCopy
		}

		perm, err := server.Config.Services.Membership.Get(c, _forge, forgeUser, org.Name)
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
