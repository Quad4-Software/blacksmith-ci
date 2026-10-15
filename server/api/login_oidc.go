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

package api

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/auth/oidc"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/shared/httputil"
	"codefloe.com/crowci/crow/v6/shared/token"
)

// oidcAuthCookie carries the state, nonce and PKCE verifier between the two legs
// of an OIDC sign-in. The verifier must never travel through the IdP, so it is
// kept in an HttpOnly cookie instead of the state parameter.
const oidcAuthCookie = "oidc_auth"

func oidcRedirect(c *gin.Context, path string) {
	c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+path)
}

// HandleOIDCAuth starts an OIDC sign-in, or links an identity when a user is
// already signed in.
func HandleOIDCAuth(c *gin.Context) {
	provider := server.Config.Services.OIDC
	if provider == nil {
		oidcRedirect(c, "/login?error=oidc_error")
		return
	}

	state, nonce, verifier := oidc.RandomString(), oidc.RandomString(), oidc.RandomString()
	authToken := token.New(token.OIDCStateToken)
	authToken.Set("state", state)
	authToken.Set("nonce", nonce)
	authToken.Set("verifier", verifier)
	if user := session.User(c); user != nil {
		authToken.Set("linking-user-id", strconv.FormatInt(user.ID, 10))
	}
	signed, err := authToken.SignExpires(server.Config.Server.JWTSecret, time.Now().Add(stateTokenDuration).Unix())
	if err != nil {
		log.Error().Err(err).Msg("cannot create OIDC state token")
		oidcRedirect(c, "/login?error=internal_error")
		return
	}

	authURL, err := provider.AuthCodeURL(c.Request.Context(), state, nonce, verifier)
	if err != nil {
		log.Error().Err(err).Msg("cannot start OIDC sign-in")
		oidcRedirect(c, "/login?error=oidc_error")
		return
	}

	httputil.SetCookie(c.Writer, c.Request, oidcAuthCookie, signed)
	c.Redirect(http.StatusSeeOther, authURL)
}

// HandleOIDCCallback completes an OIDC sign-in or identity link.
func HandleOIDCCallback(c *gin.Context) {
	provider := server.Config.Services.OIDC
	if provider == nil {
		oidcRedirect(c, "/login?error=oidc_error")
		return
	}

	authCookie, cookieErr := c.Request.Cookie(oidcAuthCookie)
	httputil.DelCookie(c.Writer, c.Request, oidcAuthCookie)

	if idpErr := c.Request.FormValue("error"); idpErr != "" {
		log.Warn().Str("error", idpErr).Str("description", c.Request.FormValue("error_description")).Msg("OIDC provider returned an error")
		oidcRedirect(c, "/login?error=oidc_error")
		return
	}
	if cookieErr != nil {
		log.Warn().Msg("OIDC callback without auth cookie")
		oidcRedirect(c, "/login?error=invalid_state")
		return
	}
	authToken, err := token.Parse([]token.Type{token.OIDCStateToken}, authCookie.Value, func(_ *token.Token) (string, error) {
		return server.Config.Server.JWTSecret, nil
	})
	if err != nil || authToken.Get("state") == "" ||
		subtle.ConstantTimeCompare([]byte(authToken.Get("state")), []byte(c.Request.FormValue("state"))) != 1 {
		log.Warn().Err(err).Msg("OIDC callback with invalid state")
		oidcRedirect(c, "/login?error=invalid_state")
		return
	}

	// A link must complete for the same user that started it.
	var linkingUserID int64
	if v := authToken.Get("linking-user-id"); v != "" {
		linkingUserID, _ = strconv.ParseInt(v, 10, 64)
		if current := session.User(c); current == nil || current.ID != linkingUserID {
			log.Warn().Int64("linking_user_id", linkingUserID).Msg("OIDC link completed by a different session")
			oidcRedirect(c, "/login?error=invalid_state")
			return
		}
	}

	// A failed link returns to the user settings, where it was started.
	errorPage := "/login"
	if linkingUserID != 0 {
		errorPage = "/user"
	}

	claims, err := provider.Exchange(c.Request.Context(), c.Request.FormValue("code"), authToken.Get("verifier"), authToken.Get("nonce"))
	if err != nil {
		log.Error().Err(err).Msg("cannot complete OIDC sign-in")
		oidcRedirect(c, errorPage+"?error=oidc_error")
		return
	}
	if len(claims.Subject) > maxIdentitySubjectLen {
		log.Error().Int("length", len(claims.Subject)).Msg("OIDC subject is longer than the OIDC spec allows")
		oidcRedirect(c, errorPage+"?error=oidc_error")
		return
	}
	if !provider.IsAllowed(claims.Groups) {
		log.Warn().Str("subject", claims.Subject).Strs("groups", claims.Groups).Msg("OIDC user is not in an allowed group")
		oidcRedirect(c, errorPage+"?error=oidc_access_denied")
		return
	}

	_store := store.FromContext(c)
	idClaims := &idpClaims{Subject: claims.Subject, Email: claims.Email, Username: claims.Username, Groups: claims.Groups}
	if linkingUserID != 0 {
		linkOIDCIdentity(c, _store, provider, linkingUserID, idClaims)
		return
	}

	user, err := signInIdentity(_store, provider, idClaims)
	if err != nil {
		code := identityErrorCode(err)
		if code == "internal_error" {
			log.Error().Err(err).Msg("cannot complete OIDC sign-in")
		}
		oidcRedirect(c, "/login?error="+code)
		return
	}

	if !issueSession(c, _store, user) {
		return
	}
	oidcRedirect(c, "/")
}

// linkOIDCIdentity attaches the identity to the signed-in user.
func linkOIDCIdentity(c *gin.Context, _store store.Store, provider *oidc.Provider, userID int64, claims *idpClaims) {
	user, err := _store.GetUser(userID)
	if err != nil {
		log.Error().Err(err).Int64("user_id", userID).Msg("cannot get linking user")
		oidcRedirect(c, "/user?error=internal_error")
		return
	}

	if _, err := linkIdentity(_store, provider, user, claims); err != nil {
		code := identityErrorCode(err)
		if code == "internal_error" {
			log.Error().Err(err).Msgf("cannot link OIDC identity to %s", user.Login)
		}
		oidcRedirect(c, "/user?error="+code)
		return
	}

	oidcRedirect(c, "/user?identity_linked=true")
}
