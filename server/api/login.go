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

package api

import (
	"encoding/base32"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/tink-crypto/tink-go/v2/subtle/random"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/forge"
	forge_types "codefloe.com/crowci/crow/v5/server/forge/types"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/store"
	"codefloe.com/crowci/crow/v5/server/store/types"
	"codefloe.com/crowci/crow/v5/shared/httputil"
	"codefloe.com/crowci/crow/v5/shared/token"
)

const stateTokenDuration = time.Minute * 5

// getLinkingUser returns the authenticated user if they are already logged in.
// This is used to detect "linking mode" where a logged-in user wants to connect an additional forge.
// The session.SetUser() middleware has already parsed and validated the session cookie,
// so we just retrieve the user from the context.
func getLinkingUser(c *gin.Context) *model.User {
	return session.User(c)
}

func HandleAuth(c *gin.Context) {
	// TODO: check if this is really needed
	c.Writer.Header().Del("Content-Type")

	// redirect when getting oauth error from forge to login page
	if err := c.Request.FormValue("error"); err != "" {
		query := url.Values{}
		query.Set("error", err)
		if errorDescription := c.Request.FormValue("error_description"); errorDescription != "" {
			query.Set("error_description", errorDescription)
		}
		if errorURI := c.Request.FormValue("error_uri"); errorURI != "" {
			query.Set("error_uri", errorURI)
		}
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("%s/login?%s", server.Config.Server.RootPath, query.Encode()))
		return
	}

	_store := store.FromContext(c)

	code := c.Request.FormValue("code")
	state := c.Request.FormValue("state")
	isCallback := code != "" && state != ""
	var forgeID int64
	var linkingUserID int64 // non-zero if we're linking a new forge to an existing user

	var isReconnect bool  // true if user is reconnecting (refreshing tokens for) an already-linked forge
	var redirectTo string // custom redirect URL after successful auth

	if isCallback { // validate the state token
		stateToken, err := token.Parse([]token.Type{token.OAuthStateToken}, state, func(_ *token.Token) (string, error) {
			return server.Config.Server.JWTSecret, nil
		})
		if err != nil {
			log.Error().Err(err).Msg("cannot verify state token")
			c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=invalid_state")
			return
		}

		_forgeID := stateToken.Get("forge-id")
		forgeID, err = strconv.ParseInt(_forgeID, 10, 64)
		if err != nil {
			log.Error().Err(err).Msg("forge-id of state token invalid")
			c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=invalid_state")
			return
		}

		// Check if this is a linking request
		if linkingStr := stateToken.Get("linking-user-id"); linkingStr != "" {
			linkingUserID, _ = strconv.ParseInt(linkingStr, 10, 64)
		}

		// Check if this is a reconnect request
		isReconnect = stateToken.Get("reconnect") == "true"

		// Get custom redirect URL if set
		redirectTo = stateToken.Get("redirect-to")
	} else { // only generate a state token if not a callback
		var err error

		_forgeID := c.Request.FormValue("forge_id")
		if _forgeID == "" {
			forgeID = 1 // fallback to main forge
		} else {
			forgeID, err = strconv.ParseInt(_forgeID, 10, 64)
			if err != nil {
				log.Error().Err(err).Msg("forge-id of state token invalid")
				c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=invalid_state")
				return
			}
		}

		// Check if user is already logged in (linking mode)
		linkingUser := getLinkingUser(c)

		// Check if this is a reconnect request (refreshing tokens for existing link)
		isReconnect = c.Request.FormValue("reconnect") == "true"

		// Get custom redirect URL (for returning to current page after auth)
		redirectTo = c.Request.FormValue("redirect_to")

		jwtSecret := server.Config.Server.JWTSecret
		exp := time.Now().Add(stateTokenDuration).Unix()
		stateToken := token.New(token.OAuthStateToken)
		stateToken.Set("forge-id", strconv.FormatInt(forgeID, 10))
		if linkingUser != nil {
			stateToken.Set("linking-user-id", strconv.FormatInt(linkingUser.ID, 10))

			// Verify user doesn't already have this forge linked (unless reconnecting)
			existingUserForge, err := _store.UserForgeGet(linkingUser.ID, forgeID)
			if err == nil && existingUserForge != nil {
				if isReconnect {
					// Mark as reconnect in the state token
					stateToken.Set("reconnect", "true")
				} else {
					log.Warn().Msgf("user %s already has forge %d linked", linkingUser.Login, forgeID)
					c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/user?error=forge_already_linked")
					return
				}
			}
		}

		// Store redirect URL in state token if provided
		if redirectTo != "" {
			stateToken.Set("redirect-to", redirectTo)
		}

		state, err = stateToken.SignExpires(jwtSecret, exp)
		if err != nil {
			log.Error().Err(err).Msg("cannot create state token")
			c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=internal_error")
			return
		}
	}

	_forge, err := server.Config.Services.Manager.ForgeByID(forgeID)
	if err != nil {
		log.Error().Err(err).Msgf("cannot get forge by id %d", forgeID)
		c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=internal_error")
		return
	}

	userFromForge, redirectURL, err := _forge.Login(c, &forge_types.OAuthRequest{
		Code:  c.Request.FormValue("code"),
		State: state,
	})
	if err != nil {
		log.Error().Err(err).Msg("cannot authenticate user")
		c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=oauth_error")
		return
	}
	// The user is not authorized yet -> redirect
	if userFromForge == nil {
		http.Redirect(c.Writer, c.Request, redirectURL, http.StatusSeeOther)
		return
	}

	// if organization filter is enabled, we need to check if the user is a member of one
	// of the configured organizations
	if server.Config.Permissions.Orgs.IsConfigured {
		teams, terr := _forge.Teams(c, userFromForge)
		if terr != nil || !server.Config.Permissions.Orgs.IsMember(teams) {
			log.Error().Err(terr).Msgf("cannot verify team membership for %s", userFromForge.Login)
			c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=org_access_denied")
			return
		}
	}

	var user *model.User
	var userForge *model.UserForge

	// Handle linking mode: user is already authenticated and linking a new forge
	if linkingUserID != 0 {
		user, err = _store.GetUser(linkingUserID)
		if err != nil {
			log.Error().Err(err).Msgf("cannot get linking user %d", linkingUserID)
			c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=internal_error")
			return
		}

		// Check if this forge remote ID is already linked to another user
		existingUserForge, err := _store.UserForgeGetByRemoteID(forgeID, userFromForge.ForgeRemoteID)
		if err == nil && existingUserForge != nil && existingUserForge.UserID != user.ID {
			log.Error().Msgf("forge account %s is already linked to another user", userFromForge.Login)
			c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/user?error=forge_account_taken")
			return
		}

		// Handle reconnect: update existing UserForge tokens
		if isReconnect {
			existingUserForge, err := _store.UserForgeGet(user.ID, forgeID)
			if err != nil {
				log.Error().Err(err).Msgf("cannot get user forge for reconnect: user %d, forge %d", user.ID, forgeID)
				c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/user?error=internal_error")
				return
			}

			// Update tokens
			existingUserForge.AccessToken = userFromForge.AccessToken
			existingUserForge.RefreshToken = userFromForge.RefreshToken
			existingUserForge.Expiry = userFromForge.Expiry
			existingUserForge.ForgeLogin = userFromForge.Login
			existingUserForge.ForgeEmail = userFromForge.Email
			existingUserForge.ForgeAvatar = userFromForge.Avatar

			if err := _store.UserForgeUpdate(existingUserForge); err != nil {
				log.Error().Err(err).Msgf("cannot update user forge tokens for user %s", user.Login)
				c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/user?error=internal_error")
				return
			}
			log.Info().Msgf("user %s reconnected forge %d (%s)", user.Login, forgeID, userFromForge.Login)

			// Redirect back to custom URL or user settings
			if redirectTo != "" {
				c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+redirectTo)
			} else {
				c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/user?forge_reconnected=true")
			}
			return
		}

		// Create new UserForge entry for the linked forge
		userForge = &model.UserForge{
			UserID:        user.ID,
			ForgeID:       forgeID,
			ForgeRemoteID: userFromForge.ForgeRemoteID,
			IsPrimary:     false, // Linked forges are never primary
			AccessToken:   userFromForge.AccessToken,
			RefreshToken:  userFromForge.RefreshToken,
			Expiry:        userFromForge.Expiry,
			ForgeLogin:    userFromForge.Login,
			ForgeEmail:    userFromForge.Email,
			ForgeAvatar:   userFromForge.Avatar,
			LinkedAt:      time.Now().Unix(),
		}
		if err := _store.UserForgeCreate(userForge); err != nil {
			log.Error().Err(err).Msgf("cannot create user forge link for user %s", user.Login)
			c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/user?error=internal_error")
			return
		}
		log.Info().Msgf("user %s linked forge %d (%s)", user.Login, forgeID, userFromForge.Login)

		// Redirect back to custom URL or user settings
		if redirectTo != "" {
			c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+redirectTo)
		} else {
			c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/user?forge_linked=true")
		}
		return
	}

	// Try to find existing user via UserForge (new multi-forge lookup)
	userForge, err = _store.UserForgeGetByRemoteID(forgeID, userFromForge.ForgeRemoteID)
	if err != nil && !errors.Is(err, types.RecordNotExist) {
		log.Error().Err(err).Msgf("cannot get user forge for %s", userFromForge.Login)
		c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=internal_error")
		return
	}

	if userForge != nil {
		// User exists via UserForge - get the user
		user, err = _store.GetUser(userForge.UserID)
		if err != nil {
			log.Error().Err(err).Msgf("cannot get user %d", userForge.UserID)
			c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=internal_error")
			return
		}
	} else {
		// Fall back to legacy lookup via User.ForgeRemoteID for backward compatibility
		user, err = _store.GetUserRemoteID(userFromForge.ForgeRemoteID, userFromForge.Login)
		if err != nil && !errors.Is(err, types.RecordNotExist) {
			log.Error().Err(err).Msgf("cannot get user %s", userFromForge.Login)
			c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=internal_error")
			return
		}

		// If user exists but has no UserForge entry, create one (migration from legacy)
		if user != nil {
			userForge = &model.UserForge{
				UserID:        user.ID,
				ForgeID:       forgeID,
				ForgeRemoteID: userFromForge.ForgeRemoteID,
				IsPrimary:     true,
				AccessToken:   userFromForge.AccessToken,
				RefreshToken:  userFromForge.RefreshToken,
				Expiry:        userFromForge.Expiry,
				ForgeLogin:    userFromForge.Login,
				ForgeEmail:    userFromForge.Email,
				ForgeAvatar:   userFromForge.Avatar,
				LinkedAt:      time.Now().Unix(),
			}
			if err := _store.UserForgeCreate(userForge); err != nil {
				log.Error().Err(err).Msgf("cannot create user forge for legacy user %s", user.Login)
				// Non-fatal - continue with login
			} else {
				log.Info().Msgf("created UserForge entry for legacy user %s", user.Login)
				// Update user's PrimaryForgeID if not set
				if user.PrimaryForgeID == 0 {
					user.PrimaryForgeID = userForge.ID
				}
			}
		}
	}

	if user == nil || errors.Is(err, types.RecordNotExist) {
		// if self-registration is disabled we should return a not authorized error
		if !server.Config.Permissions.Open && !server.Config.Permissions.Admins.IsAdmin(userFromForge) {
			log.Error().Msgf("cannot register %s. registration closed", userFromForge.Login)
			c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=registration_closed")
			return
		}

		// create the user account
		user = &model.User{
			ForgeID:       forgeID,
			ForgeRemoteID: userFromForge.ForgeRemoteID,
			Login:         userFromForge.Login,
			AccessToken:   userFromForge.AccessToken,
			RefreshToken:  userFromForge.RefreshToken,
			Expiry:        userFromForge.Expiry,
			Email:         userFromForge.Email,
			Avatar:        userFromForge.Avatar,
			Hash: base32.StdEncoding.EncodeToString(
				random.GetRandomBytes(32),
			),
		}

		// insert the user into the database
		if err := _store.CreateUser(user); err != nil {
			log.Error().Err(err).Msgf("cannot insert %s", user.Login)
			c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=internal_error")
			return
		}

		// Create the primary UserForge entry for the new user
		userForge = &model.UserForge{
			UserID:        user.ID,
			ForgeID:       forgeID,
			ForgeRemoteID: userFromForge.ForgeRemoteID,
			IsPrimary:     true,
			AccessToken:   userFromForge.AccessToken,
			RefreshToken:  userFromForge.RefreshToken,
			Expiry:        userFromForge.Expiry,
			ForgeLogin:    userFromForge.Login,
			ForgeEmail:    userFromForge.Email,
			ForgeAvatar:   userFromForge.Avatar,
			LinkedAt:      time.Now().Unix(),
		}
		if err := _store.UserForgeCreate(userForge); err != nil {
			log.Error().Err(err).Msgf("cannot create primary user forge for %s", user.Login)
			c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=internal_error")
			return
		}

		// Set the user's PrimaryForgeID
		user.PrimaryForgeID = userForge.ID
		if err := _store.UpdateUser(user); err != nil {
			log.Error().Err(err).Msgf("cannot update user %s with primary forge ID", user.Login)
			// Non-fatal, continue
		}
	}

	// create or set the user's organization if it isn't linked yet
	if user.OrgID == 0 {
		// check if an org with the same name exists already and assign it to the user if it does
		org, err := _store.OrgFindByName(user.Login, forgeID)
		if err != nil && !errors.Is(err, types.RecordNotExist) {
			log.Error().Err(err).Msgf("cannot get org for user %s", user.Login)
			c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=internal_error")
			return
		}

		// if an org with the same name exists => assign org to the user
		if err == nil && org != nil {
			org.IsUser = true
			user.OrgID = org.ID

			if err := _store.OrgUpdate(org); err != nil {
				log.Error().Err(err).Msgf("cannot assign user %s to existing org %d", user.Login, org.ID)
			}
		}

		// if still no org with the same name exists => create a new org
		if user.OrgID == 0 || errors.Is(err, types.RecordNotExist) {
			org := &model.Org{
				Name:    user.Login,
				IsUser:  true,
				Private: false,
				ForgeID: user.ForgeID,
			}
			if err := _store.OrgCreate(org); err != nil {
				log.Error().Err(err).Msgf("cannot create org for user %s", user.Login)
			}
			user.OrgID = org.ID
		}
	} else {
		// update org name if necessary
		org, err := _store.OrgGet(user.OrgID)
		if err != nil {
			log.Error().Err(err).Msgf("cannot get org %d for user %s", user.OrgID, user.Login)
			c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=internal_error")
			return
		}
		if org != nil && org.Name != user.Login {
			org.Name = user.Login
			if err := _store.OrgUpdate(org); err != nil {
				log.Error().Err(err).Msgf("cannot update org %d name to user name %s", org.ID, user.Login)
			}
		}
	}

	// Update UserForge tokens if we have an existing UserForge
	if userForge != nil {
		userForge.AccessToken = userFromForge.AccessToken
		userForge.RefreshToken = userFromForge.RefreshToken
		userForge.Expiry = userFromForge.Expiry
		userForge.ForgeLogin = userFromForge.Login
		userForge.ForgeEmail = userFromForge.Email
		userForge.ForgeAvatar = userFromForge.Avatar
		if err := _store.UserForgeUpdate(userForge); err != nil {
			log.Error().Err(err).Msgf("cannot update user forge for %s", user.Login)
			// Non-fatal, continue
		}
	}

	// update the user meta data and authorization data (keep deprecated fields in sync)
	user.AccessToken = userFromForge.AccessToken
	user.RefreshToken = userFromForge.RefreshToken
	user.Email = userFromForge.Email
	user.Avatar = userFromForge.Avatar
	user.ForgeID = forgeID
	user.ForgeRemoteID = userFromForge.ForgeRemoteID
	user.Login = userFromForge.Login
	user.Admin = user.Admin || server.Config.Permissions.Admins.IsAdmin(userFromForge)

	if err := _store.UpdateUser(user); err != nil {
		log.Error().Err(err).Msgf("cannot update user %s", user.Login)
		c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=internal_error")
		return
	}

	exp := time.Now().Add(server.Config.Server.SessionExpires).Unix()
	_token := token.New(token.SessToken)
	_token.Set("user-id", strconv.FormatInt(user.ID, 10))
	tokenString, err := _token.SignExpires(user.Hash, exp)
	if err != nil {
		log.Error().Msgf("cannot create token for user %s", user.Login)
		c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=internal_error")
		return
	}

	err = updateRepoPermissions(c, user, _store, _forge)
	if err != nil {
		log.Error().Err(err).Msgf("cannot update repo permissions for user %s", user.Login)
		c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=internal_error")
		return
	}

	httputil.SetCookie(c.Writer, c.Request, "user_sess", tokenString)

	c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/")
}

func updateRepoPermissions(c *gin.Context, user *model.User, _store store.Store, _forge forge.Forge) error {
	repos, _ := _forge.Repos(c, user)

	for _, forgeRepo := range repos {
		dbRepo, err := _store.GetRepoForgeID(forgeRepo.ForgeRemoteID)
		if err != nil && errors.Is(err, types.RecordNotExist) {
			continue
		}
		if err != nil {
			return err
		}

		if !dbRepo.IsActive {
			continue
		}

		log.Debug().Msgf("synced user permission for user %s and repo %s", user.Login, dbRepo.FullName)
		perm := forgeRepo.Perm
		perm.Repo = dbRepo
		perm.RepoID = dbRepo.ID
		perm.UserID = user.ID
		perm.Synced = time.Now().Unix()
		if err := _store.PermUpsert(perm); err != nil {
			return err
		}
	}

	return nil
}

func GetLogout(c *gin.Context) {
	httputil.DelCookie(c.Writer, c.Request, "user_sess")
	httputil.DelCookie(c.Writer, c.Request, "user_last")
	c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/")
}
