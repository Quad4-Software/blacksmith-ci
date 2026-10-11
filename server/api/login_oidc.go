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
	"encoding/base32"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/tink-crypto/tink-go/v2/subtle/random"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/auth/oidc"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/types"
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
	if linkingUserID != 0 {
		linkOIDCIdentity(c, _store, provider, linkingUserID, claims)
		return
	}

	identity, err := _store.UserIdentityGetByProviderSubject(oidc.ProviderName, provider.Issuer(), claims.Subject)
	var user *model.User
	switch {
	case err == nil:
		user, err = _store.GetUser(identity.UserID)
		if err != nil {
			log.Error().Err(err).Int64("user_id", identity.UserID).Msg("cannot get user of OIDC identity")
			oidcRedirect(c, "/login?error=internal_error")
			return
		}
		refreshIdentity(identity, claims)
		if err := _store.UserIdentityUpdate(identity); err != nil {
			log.Error().Err(err).Msgf("cannot update OIDC identity of %s", user.Login)
		}
	case errors.Is(err, types.RecordNotExist):
		var ok bool
		if user, ok = registerOIDCUser(c, _store, provider, claims); !ok {
			return
		}
	default:
		log.Error().Err(err).Msg("cannot look up OIDC identity")
		oidcRedirect(c, "/login?error=internal_error")
		return
	}

	if provider.ManagesAdmin() {
		// OIDC sign-in never consults CROW_ADMIN because the username claim is not
		// verified; admin via OIDC comes only from CROW_OIDC_ADMIN_GROUPS.
		admin := provider.IsAdmin(claims.Groups)
		if admin != user.Admin {
			user.Admin = admin
			if err := _store.UpdateUser(user); err != nil {
				log.Error().Err(err).Msgf("cannot update admin status of %s", user.Login)
				oidcRedirect(c, "/login?error=internal_error")
				return
			}
			log.Info().Bool("admin", admin).Msgf("admin status of %s set from OIDC groups", user.Login)
		}
	}

	if !issueSession(c, _store, user) {
		return
	}
	oidcRedirect(c, "/")
}

// Column sizes of user_identities; longer claims are stored truncated, as they
// are informational only.
const (
	maxIdentityEmailLen    = 500
	maxIdentityUsernameLen = 250
	// The OIDC spec limits sub to 255 ASCII characters.
	maxIdentitySubjectLen = 255
)

func truncateClaim(value string, maxLen int) string {
	if len(value) <= maxLen {
		return value
	}
	cut := maxLen
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut]
}

func refreshIdentity(identity *model.UserIdentity, claims *oidc.Claims) {
	identity.Email = truncateClaim(claims.Email, maxIdentityEmailLen)
	identity.Username = truncateClaim(claims.Username, maxIdentityUsernameLen)
	identity.Groups = claims.Groups
	identity.LastLogin = time.Now().Unix()
}

// registerOIDCUser creates a user for an unknown identity. On failure it
// redirects and returns false.
func registerOIDCUser(c *gin.Context, _store store.Store, provider *oidc.Provider, claims *oidc.Claims) (*model.User, bool) {
	login, valid := oidc.NormalizeUsername(claims.Username)

	// OIDC sign-in never consults CROW_ADMIN because the username claim is not
	// verified; admin via OIDC comes only from CROW_OIDC_ADMIN_GROUPS.
	if !server.Config.Permissions.Open && !provider.RestrictsGroups() && !provider.IsAdmin(claims.Groups) {
		log.Warn().Str("subject", claims.Subject).Msg("cannot register OIDC user, registration closed")
		oidcRedirect(c, "/login?error=registration_closed")
		return nil, false
	}
	if strings.TrimSpace(claims.Username) == "" {
		log.Warn().Str("subject", claims.Subject).Msg("OIDC token has no username claim")
		oidcRedirect(c, "/login?error=oidc_missing_username")
		return nil, false
	}
	if !valid {
		log.Warn().Str("username", claims.Username).Msg("OIDC username claim is not a valid Crow login")
		oidcRedirect(c, "/login?error=oidc_invalid_username")
		return nil, false
	}

	// Existing users must link their identity explicitly from user settings, so
	// a matching login never signs anyone in. The username comes from the IdP
	// and is not forge-verified, so it must not collide with any user or any org
	// on any forge (case-insensitive): a forge org of that name would look like
	// the user's personal namespace. The store checks this in the same
	// transaction that creates the user, org and identity.
	user := &model.User{
		Login: login,
		Email: truncateClaim(claims.Email, maxIdentityEmailLen),
		Admin: false,
		Hash:  base32.StdEncoding.EncodeToString(random.GetRandomBytes(32)),
	}
	identity := &model.UserIdentity{Provider: oidc.ProviderName, Issuer: provider.Issuer(), Subject: claims.Subject}
	refreshIdentity(identity, claims)
	if err := _store.UserCreateWithIdentity(user, identity); errors.Is(err, types.ErrNameTaken) {
		log.Warn().Msgf("cannot register OIDC user %s: login or org name already taken", login)
		oidcRedirect(c, "/login?error=oidc_login_taken")
		return nil, false
	} else if err != nil {
		log.Error().Err(err).Msgf("cannot create OIDC user %s", login)
		oidcRedirect(c, "/login?error=internal_error")
		return nil, false
	}
	log.Info().Msgf("registered user %s through OIDC", login)
	return user, true
}

// linkOIDCIdentity attaches the identity to the signed-in user.
func linkOIDCIdentity(c *gin.Context, _store store.Store, provider *oidc.Provider, userID int64, claims *oidc.Claims) {
	user, err := _store.GetUser(userID)
	if err != nil {
		log.Error().Err(err).Int64("user_id", userID).Msg("cannot get linking user")
		oidcRedirect(c, "/user?error=internal_error")
		return
	}

	existing, err := _store.UserIdentityGetByProviderSubject(oidc.ProviderName, provider.Issuer(), claims.Subject)
	switch {
	case err == nil && existing.UserID != user.ID:
		log.Warn().Msgf("OIDC identity %s is already linked to user %d", claims.Subject, existing.UserID)
		oidcRedirect(c, "/user?error=identity_taken")
		return
	case err == nil:
		refreshIdentity(existing, claims)
		if err := _store.UserIdentityUpdate(existing); err != nil {
			log.Error().Err(err).Msgf("cannot update OIDC identity of %s", user.Login)
		}
	case errors.Is(err, types.RecordNotExist):
		identities, err := _store.UserIdentityList(user.ID)
		if err != nil {
			log.Error().Err(err).Msgf("cannot list identities of %s", user.Login)
			oidcRedirect(c, "/user?error=internal_error")
			return
		}
		for _, id := range identities {
			if id.Provider == oidc.ProviderName {
				oidcRedirect(c, "/user?error=identity_already_linked")
				return
			}
		}
		// A legacy forge user with neither a forge remote ID nor a forge link is
		// only found at forge sign-in by login, and that lookup skips users with
		// an identity. They must sign in with their forge once first, which
		// records the forge link.
		if len(identities) == 0 && user.ForgeRemoteID == "" {
			forges, err := _store.UserForgeList(user.ID)
			if err != nil {
				log.Error().Err(err).Msgf("cannot list forges of %s", user.Login)
				oidcRedirect(c, "/user?error=internal_error")
				return
			}
			if len(forges) == 0 {
				log.Warn().Msgf("user %s must sign in with their forge before linking an OIDC identity", user.Login)
				oidcRedirect(c, "/user?error=forge_sign_in_required")
				return
			}
		}
		identity := &model.UserIdentity{UserID: user.ID, Provider: oidc.ProviderName, Issuer: provider.Issuer(), Subject: claims.Subject}
		refreshIdentity(identity, claims)
		if err := _store.UserIdentityCreate(identity); err != nil {
			log.Error().Err(err).Msgf("cannot link OIDC identity to %s", user.Login)
			oidcRedirect(c, "/user?error=internal_error")
			return
		}
		log.Info().Msgf("user %s linked an OIDC identity", user.Login)
	default:
		log.Error().Err(err).Msg("cannot look up OIDC identity")
		oidcRedirect(c, "/user?error=internal_error")
		return
	}

	oidcRedirect(c, "/user?identity_linked=true")
}
