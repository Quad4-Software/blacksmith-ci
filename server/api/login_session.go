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
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/types"
	"codefloe.com/crowci/crow/v6/shared/httputil"
	"codefloe.com/crowci/crow/v6/shared/token"
)

// forgeLinkLocks serializes forge linking per user, so two concurrent link
// callbacks cannot both see no linked forge and both claim the primary forge.
// The lock is process-local, which covers the common single-server setup.
var forgeLinkLocks sync.Map

func lockForgeLink(userID int64) (unlock func()) {
	m, _ := forgeLinkLocks.LoadOrStore(userID, &sync.Mutex{})
	mu, _ := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// needsPrimaryForgeAdoption reports whether a user registered through an
// identity provider still has to adopt their primary forge, because an earlier
// adoption failed after the forge link was stored.
func needsPrimaryForgeAdoption(_store store.Store, user *model.User, userForge *model.UserForge) (bool, error) {
	if !userForge.IsPrimary || user.ForgeRemoteID != "" {
		return false, nil
	}
	identities, err := _store.UserIdentityList(user.ID)
	if err != nil {
		return false, err
	}
	return len(identities) > 0, nil
}

// forgeSignInDisabled reports whether CROW_FORGE_LOGIN_DISABLED switched off
// forge OAuth sign-in. Linking a forge to a signed-in user stays possible.
func forgeSignInDisabled() bool {
	return server.Config.Server.ForgeLoginDisabled
}

// createSession signs a session for user, refreshes their repo permissions and
// sets the session cookies.
func createSession(c *gin.Context, _store store.Store, user *model.User) error {
	exp := time.Now().Add(server.Config.Server.SessionExpires).Unix()
	_token := token.New(token.SessToken)
	_token.Set("user-id", strconv.FormatInt(user.ID, 10))
	tokenString, err := _token.SignExpires(user.Hash, exp)
	if err != nil {
		return fmt.Errorf("cannot create token for user %s: %w", user.Login, err)
	}

	if err := updateRepoPermissions(c, user, _store); err != nil {
		return fmt.Errorf("cannot update repo permissions for user %s: %w", user.Login, err)
	}

	httputil.SetCookie(c.Writer, c.Request, "user_sess", tokenString)

	// Remember this account so it can be quick-switched to later without a full
	// logout/login cycle.
	rememberAccount(c.Writer, c.Request, user.ID, tokenString)
	return nil
}

// issueSession creates a session for user. On failure it redirects to the
// login page and returns false.
func issueSession(c *gin.Context, _store store.Store, user *model.User) bool {
	if err := createSession(c, _store, user); err != nil {
		log.Error().Err(err).Msg("cannot create session")
		c.Redirect(http.StatusSeeOther, server.Config.Server.RootPath+"/login?error=internal_error")
		return false
	}
	return true
}

// ensureUserOrg creates or adopts the user's personal org, or renames it after
// a login change.
func ensureUserOrg(_store store.Store, user *model.User, forgeID int64) error {
	if user.OrgID == 0 {
		// check if an org with the same name exists already and assign it to the user if it does
		org, err := _store.OrgFindByName(user.Login, forgeID)
		if err != nil && !errors.Is(err, types.RecordNotExist) {
			return fmt.Errorf("cannot get org for user %s: %w", user.Login, err)
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
		return nil
	}

	// update org name if necessary
	org, err := _store.OrgGet(user.OrgID)
	if err != nil {
		return fmt.Errorf("cannot get org %d for user %s: %w", user.OrgID, user.Login, err)
	}
	if org != nil && org.Name != user.Login {
		org.Name = user.Login
		if err := _store.OrgUpdate(org); err != nil {
			log.Error().Err(err).Msgf("cannot update org %d name to user name %s", org.ID, user.Login)
		}
	}
	return nil
}

// adoptPrimaryForge makes userForge the primary forge of a user who has none
// yet (a user who signed up through an identity provider), filling the legacy
// forge fields exactly like a first forge registration does.
func adoptPrimaryForge(_store store.Store, user *model.User, userForge *model.UserForge, userFromForge *model.User) error {
	user.PrimaryForgeID = userForge.ID
	user.ForgeID = userForge.ForgeID
	user.ForgeRemoteID = userForge.ForgeRemoteID
	user.AccessToken = userFromForge.AccessToken
	user.RefreshToken = userFromForge.RefreshToken
	user.Expiry = userFromForge.Expiry
	if user.Email == "" {
		user.Email = userFromForge.Email
	}
	if user.Avatar == "" {
		user.Avatar = userFromForge.Avatar
	}
	// Never look an org up by the Crow login here: it comes from the identity
	// provider and is not forge-verified, so a name match could bind the user to
	// someone else's org. Only create a fresh personal org when none exists.
	if user.OrgID == 0 {
		org := &model.Org{Name: user.Login, IsUser: true, ForgeID: userForge.ForgeID}
		if err := _store.OrgCreate(org); err != nil {
			return fmt.Errorf("cannot create org for user %s: %w", user.Login, err)
		}
		user.OrgID = org.ID
	} else if err := movePersonalOrgToForge(_store, user, userForge, userFromForge); err != nil {
		return err
	}
	return _store.UpdateUser(user)
}

// movePersonalOrgToForge moves the forge-less personal org that single sign-on
// gave the user (forge_id 0) onto the first linked forge. The forge login is
// forge-verified, so an existing org of that name on that forge is the user's
// own namespace: it is adopted and receives the placeholder's org secrets and
// registries. Otherwise the placeholder itself becomes that namespace.
func movePersonalOrgToForge(_store store.Store, user *model.User, userForge *model.UserForge, userFromForge *model.User) error {
	placeholder, err := _store.OrgGet(user.OrgID)
	if err != nil {
		return fmt.Errorf("cannot get org %d of user %s: %w", user.OrgID, user.Login, err)
	}
	if placeholder.ForgeID != 0 {
		return nil
	}

	forgeOrg, err := _store.OrgFindByName(userFromForge.Login, userForge.ForgeID)
	if errors.Is(err, types.RecordNotExist) {
		placeholder.ForgeID = userForge.ForgeID
		placeholder.Name = userFromForge.Login
		if err := _store.OrgUpdate(placeholder); err != nil {
			return fmt.Errorf("cannot move org %d of user %s to forge %d: %w", placeholder.ID, user.Login, userForge.ForgeID, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot get org %s on forge %d: %w", userFromForge.Login, userForge.ForgeID, err)
	}

	forgeOrg.IsUser = true
	if err := _store.OrgUpdate(forgeOrg); err != nil {
		return fmt.Errorf("cannot assign org %d to user %s: %w", forgeOrg.ID, user.Login, err)
	}
	user.OrgID = forgeOrg.ID

	if moveOrgSecretsAndRegistries(_store, placeholder.ID, forgeOrg.ID) {
		if err := _store.OrgDelete(placeholder.ID); err != nil {
			log.Error().Err(err).Msgf("cannot delete former personal org %d of user %s", placeholder.ID, user.Login)
		}
	} else {
		log.Warn().Msgf("kept former personal org %d of user %s because not all of its secrets and registries could be moved", placeholder.ID, user.Login)
	}
	return nil
}

// moveOrgSecretsAndRegistries moves the org-level secrets and registries of
// org from to org to. It reports whether everything was moved; items whose
// name is already taken in the target org stay where they are.
func moveOrgSecretsAndRegistries(_store store.Store, from, to int64) bool {
	complete := true

	secrets, err := _store.OrgSecretList(from, &model.ListOptions{All: true})
	if err != nil {
		log.Error().Err(err).Int64("org_id", from).Msg("cannot list org secrets to move")
		return false
	}
	for _, secret := range secrets {
		// OrgSecretList also returns secrets of other orgs shared with this one.
		if secret.OrgID != from {
			continue
		}
		if _, err := _store.OrgSecretFind(to, secret.Name); !errors.Is(err, types.RecordNotExist) {
			log.Warn().Err(err).Int64("org_id", to).Msgf("cannot move secret %s: target org already has it", secret.Name)
			complete = false
			continue
		}
		secret.OrgID = to
		if err := _store.SecretUpdate(secret); err != nil {
			log.Error().Err(err).Int64("org_id", to).Msgf("cannot move secret %s", secret.Name)
			complete = false
		}
	}

	registries, err := _store.OrgRegistryList(from, &model.ListOptions{All: true})
	if err != nil {
		log.Error().Err(err).Int64("org_id", from).Msg("cannot list org registries to move")
		return false
	}
	for _, registry := range registries {
		if _, err := _store.OrgRegistryFind(to, registry.Address); !errors.Is(err, types.RecordNotExist) {
			log.Warn().Err(err).Int64("org_id", to).Msgf("cannot move registry %s: target org already has it", registry.Address)
			complete = false
			continue
		}
		registry.OrgID = to
		if err := _store.RegistryUpdate(registry); err != nil {
			log.Error().Err(err).Int64("org_id", to).Msgf("cannot move registry %s", registry.Address)
			complete = false
		}
	}
	return complete
}
