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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v5/server"
	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/router/middleware/session"
	"codeberg.org/crowci/crow/v5/server/store"
	"codeberg.org/crowci/crow/v5/server/store/types"
)

// getForgeTokenStatus determines the status of a forge connection based on token expiry.
func getForgeTokenStatus(expiry int64) UserForgeStatus {
	if expiry == 0 {
		// No expiry set - assume active (some forges don't use expiring tokens)
		return UserForgeStatusActive
	}

	now := time.Now().Unix()
	if now >= expiry {
		return UserForgeStatusExpired
	}
	return UserForgeStatusActive
}

// UserForgeStatus represents the connection status of a linked forge.
type UserForgeStatus string

const (
	// UserForgeStatusActive means the token is valid and not expired.
	UserForgeStatusActive UserForgeStatus = "active"
	// UserForgeStatusExpired means the token has expired and needs re-authentication.
	UserForgeStatusExpired UserForgeStatus = "expired"
)

// UserForgeResponse represents a user's forge connection in API responses.
type UserForgeResponse struct {
	ID          int64           `json:"id"`
	ForgeID     int64           `json:"forge_id"`
	ForgeName   string          `json:"forge_name,omitempty"`
	ForgeType   string          `json:"forge_type,omitempty"`
	ForgeIcon   string          `json:"forge_icon,omitempty"`
	IsPrimary   bool            `json:"is_primary"`
	ForgeLogin  string          `json:"forge_login"`
	ForgeEmail  string          `json:"forge_email,omitempty"`
	ForgeAvatar string          `json:"forge_avatar,omitempty"`
	LinkedAt    int64           `json:"linked_at"`
	LastUsed    int64           `json:"last_used,omitempty"`
	Status      UserForgeStatus `json:"status"`
	Expiry      int64           `json:"expiry,omitempty"`
}

// GetUserForges
//
//	@Summary		Get all linked forges for the current user
//	@Description	Returns all forge connections linked to the authenticated user's account
//	@Router			/user/forges [get]
//	@Produce		json
//	@Success		200	{array}	UserForgeResponse
//	@Tags			User
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
func GetUserForges(c *gin.Context) {
	_store := store.FromContext(c)
	user := session.User(c)

	userForges, err := _store.UserForgeList(user.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error fetching user forges. %s", err)
		return
	}

	// Get forge details for each user forge
	responses := make([]*UserForgeResponse, 0, len(userForges))
	for _, uf := range userForges {
		resp := &UserForgeResponse{
			ID:          uf.ID,
			ForgeID:     uf.ForgeID,
			IsPrimary:   uf.IsPrimary,
			ForgeLogin:  uf.ForgeLogin,
			ForgeEmail:  uf.ForgeEmail,
			ForgeAvatar: uf.ForgeAvatar,
			LinkedAt:    uf.LinkedAt,
			LastUsed:    uf.LastUsed,
			Status:      getForgeTokenStatus(uf.Expiry),
			Expiry:      uf.Expiry,
		}

		// Get forge details
		forge, err := _store.ForgeGet(uf.ForgeID)
		if err == nil && forge != nil {
			resp.ForgeName = forge.URL
			resp.ForgeType = string(forge.Type)
			resp.ForgeIcon = forge.Icon
		}

		responses = append(responses, resp)
	}

	c.JSON(http.StatusOK, responses)
}

// GetUserForge
//
//	@Summary		Get a specific linked forge for the current user
//	@Description	Returns details of a specific forge connection for the authenticated user
//	@Router			/user/forges/{forge_id} [get]
//	@Produce		json
//	@Success		200	{object}	UserForgeResponse
//	@Tags			User
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			forge_id		path	int		true	"the forge's id"
func GetUserForge(c *gin.Context) {
	_store := store.FromContext(c)
	user := session.User(c)

	forgeID, err := strconv.ParseInt(c.Param("forge_id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid forge ID")
		return
	}

	userForge, err := _store.UserForgeGet(user.ID, forgeID)
	if err != nil {
		if errors.Is(err, types.RecordNotExist) {
			c.String(http.StatusNotFound, "Forge not linked to user")
			return
		}
		c.String(http.StatusInternalServerError, "Error fetching user forge. %s", err)
		return
	}

	resp := &UserForgeResponse{
		ID:          userForge.ID,
		ForgeID:     userForge.ForgeID,
		IsPrimary:   userForge.IsPrimary,
		ForgeLogin:  userForge.ForgeLogin,
		ForgeEmail:  userForge.ForgeEmail,
		ForgeAvatar: userForge.ForgeAvatar,
		LinkedAt:    userForge.LinkedAt,
		LastUsed:    userForge.LastUsed,
		Status:      getForgeTokenStatus(userForge.Expiry),
		Expiry:      userForge.Expiry,
	}

	// Get forge details
	forge, err := _store.ForgeGet(userForge.ForgeID)
	if err == nil && forge != nil {
		resp.ForgeName = forge.URL
		resp.ForgeType = string(forge.Type)
		resp.ForgeIcon = forge.Icon
	}

	c.JSON(http.StatusOK, resp)
}

// GetUserForgeAffectedRepos
//
//	@Summary		Get repos that would be affected by unlinking a forge
//	@Description	Returns all active repos owned by the user from the specified forge that would be disabled if the forge is unlinked.
//	@Router			/user/forges/{forge_id}/affected-repos [get]
//	@Produce		json
//	@Success		200	{array}	Repo
//	@Tags			User
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			forge_id		path	int		true	"the forge's id"
func GetUserForgeAffectedRepos(c *gin.Context) {
	_store := store.FromContext(c)
	user := session.User(c)

	forgeID, err := strconv.ParseInt(c.Param("forge_id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid forge ID")
		return
	}

	// Verify the forge is linked to this user
	_, err = _store.UserForgeGet(user.ID, forgeID)
	if err != nil {
		if errors.Is(err, types.RecordNotExist) {
			c.String(http.StatusNotFound, "Forge not linked to user")
			return
		}
		c.String(http.StatusInternalServerError, "Error fetching user forge. %s", err)
		return
	}

	// Get all active repos owned by this user from this forge
	repos, err := _store.RepoListByUserAndForge(user.ID, forgeID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error fetching affected repos. %s", err)
		return
	}

	c.JSON(http.StatusOK, repos)
}

// DeleteUserForge
//
//	@Summary		Unlink a forge from the current user
//	@Description	Removes a forge connection from the authenticated user's account. The primary forge cannot be unlinked. All active repos from this forge owned by the user will be disabled and their webhooks removed.
//	@Router			/user/forges/{forge_id} [delete]
//	@Produce		json
//	@Success		200	{object}	UnlinkForgeResponse
//	@Tags			User
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			forge_id		path	int		true	"the forge's id"
func DeleteUserForge(c *gin.Context) {
	_store := store.FromContext(c)
	user := session.User(c)

	forgeID, err := strconv.ParseInt(c.Param("forge_id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid forge ID")
		return
	}

	userForge, err := _store.UserForgeGet(user.ID, forgeID)
	if err != nil {
		if errors.Is(err, types.RecordNotExist) {
			c.String(http.StatusNotFound, "Forge not linked to user")
			return
		}
		c.String(http.StatusInternalServerError, "Error fetching user forge. %s", err)
		return
	}

	// Prevent unlinking the primary forge
	if userForge.IsPrimary {
		c.String(http.StatusForbidden, "Cannot unlink primary forge")
		return
	}

	// Get all active repos owned by this user from this forge
	affectedRepos, err := _store.RepoListByUserAndForge(user.ID, forgeID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error fetching affected repos. %s", err)
		return
	}

	// Get the forge to remove webhooks
	_forge, err := server.Config.Services.Manager.ForgeByID(forgeID)
	if err != nil {
		log.Error().Err(err).Int64("forge_id", forgeID).Msg("Cannot get forge for webhook removal")
		// Continue anyway - we still want to disable repos even if we can't remove webhooks
	}

	// Disable each affected repo and remove its webhook
	disabledCount := 0
	for _, repo := range affectedRepos {
		// Try to remove the webhook from the forge
		if _forge != nil {
			// Create a temporary user with the forge tokens for webhook removal
			forgeUser := &model.User{
				ID:           user.ID,
				Login:        userForge.ForgeLogin,
				AccessToken:  userForge.AccessToken,
				RefreshToken: userForge.RefreshToken,
				Expiry:       userForge.Expiry,
			}

			if err := _forge.Deactivate(c, forgeUser, repo, server.Config.Server.WebhookHost); err != nil {
				log.Warn().Err(err).Str("repo", repo.FullName).Msg("Failed to remove webhook when unlinking forge")
				// Continue anyway - the repo will be disabled
			}
		}

		// Disable the repo
		repo.IsActive = false
		repo.UserID = 0 // Clear the owner since they no longer have valid tokens
		if err := _store.UpdateRepo(repo); err != nil {
			log.Error().Err(err).Str("repo", repo.FullName).Msg("Failed to disable repo when unlinking forge")
			continue
		}
		disabledCount++
	}

	// Delete the user forge connection
	if err := _store.UserForgeDelete(userForge); err != nil {
		c.String(http.StatusInternalServerError, "Error unlinking forge. %s", err)
		return
	}

	c.JSON(http.StatusOK, UnlinkForgeResponse{
		Message:       fmt.Sprintf("Forge unlinked successfully. %d repos were disabled.", disabledCount),
		DisabledRepos: disabledCount,
	})
}

// UnlinkForgeResponse represents the response when unlinking a forge.
type UnlinkForgeResponse struct {
	Message       string `json:"message"`
	DisabledRepos int    `json:"disabled_repos"`
}

// GetAvailableForges
//
//	@Summary		Get all available forges that can be linked
//	@Description	Returns all configured forges, indicating which ones are already linked to the user
//	@Router			/user/forges/available [get]
//	@Produce		json
//	@Success		200	{array}	AvailableForgeResponse
//	@Tags			User
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
func GetAvailableForges(c *gin.Context) {
	_store := store.FromContext(c)
	user := session.User(c)

	// Get all forges
	forges, err := _store.ForgeListAll()
	if err != nil {
		c.String(http.StatusInternalServerError, "Error fetching forges. %s", err)
		return
	}

	// Get user's linked forges
	userForges, err := _store.UserForgeList(user.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error fetching user forges. %s", err)
		return
	}

	// Build a map of linked forge IDs
	linkedForges := make(map[int64]*model.UserForge)
	for _, uf := range userForges {
		linkedForges[uf.ForgeID] = uf
	}

	responses := make([]*AvailableForgeResponse, 0, len(forges))
	for _, forge := range forges {
		resp := &AvailableForgeResponse{
			ID:   forge.ID,
			URL:  forge.URL,
			Type: string(forge.Type),
		}

		if uf, linked := linkedForges[forge.ID]; linked {
			resp.IsLinked = true
			resp.IsPrimary = uf.IsPrimary
			resp.LinkedAs = uf.ForgeLogin
		}

		responses = append(responses, resp)
	}

	c.JSON(http.StatusOK, responses)
}

// AvailableForgeResponse represents a forge that can potentially be linked.
type AvailableForgeResponse struct {
	ID        int64  `json:"id"`
	URL       string `json:"url"`
	Type      string `json:"type"`
	IsLinked  bool   `json:"is_linked"`
	IsPrimary bool   `json:"is_primary,omitempty"`
	LinkedAs  string `json:"linked_as,omitempty"`
}
