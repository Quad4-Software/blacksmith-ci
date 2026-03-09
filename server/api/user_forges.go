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
	"context"
	"fmt"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/store"
)

// getForgeTokenStatus determines the status of a forge connection based on token state.
// A forge is considered expired if the access token is empty (tokens were cleared)
// or if the token expiry time has passed.
func getForgeTokenStatus(accessToken string, expiry int64) UserForgeStatus {
	if accessToken == "" {
		return UserForgeStatusExpired
	}

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

type GetUserForgesInput struct{}

type GetUserForgesOutput struct {
	Body []*UserForgeResponse
}

func GetUserForges(ctx context.Context, _ *GetUserForgesInput) (*GetUserForgesOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)

	userForges, err := _store.UserForgeList(user.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Error fetching user forges", err)
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
			Status:      getForgeTokenStatus(uf.AccessToken, uf.Expiry),
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

	return &GetUserForgesOutput{Body: responses}, nil
}

type GetUserForgeInput struct {
	ForgeID int64 `path:"forge_id" doc:"the forge's id"`
}

type GetUserForgeOutput struct {
	Body *UserForgeResponse
}

func GetUserForge(ctx context.Context, input *GetUserForgeInput) (*GetUserForgeOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)

	userForge, err := _store.UserForgeGet(user.ID, input.ForgeID)
	if err != nil {
		return nil, dbErr(err)
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
		Status:      getForgeTokenStatus(userForge.AccessToken, userForge.Expiry),
		Expiry:      userForge.Expiry,
	}

	// Get forge details
	forge, err := _store.ForgeGet(userForge.ForgeID)
	if err == nil && forge != nil {
		resp.ForgeName = forge.URL
		resp.ForgeType = string(forge.Type)
		resp.ForgeIcon = forge.Icon
	}

	return &GetUserForgeOutput{Body: resp}, nil
}

type GetUserForgeAffectedReposInput struct {
	ForgeID int64 `path:"forge_id" doc:"the forge's id"`
}

type GetUserForgeAffectedReposOutput struct {
	Body []*model.Repo
}

func GetUserForgeAffectedRepos(ctx context.Context, input *GetUserForgeAffectedReposInput) (*GetUserForgeAffectedReposOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)

	// Verify the forge is linked to this user
	_, err := _store.UserForgeGet(user.ID, input.ForgeID)
	if err != nil {
		return nil, dbErr(err)
	}

	// Get all active repos owned by this user from this forge
	repos, err := _store.RepoListByUserAndForge(user.ID, input.ForgeID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Error fetching affected repos", err)
	}

	return &GetUserForgeAffectedReposOutput{Body: repos}, nil
}

type DeleteUserForgeInput struct {
	ForgeID int64 `path:"forge_id" doc:"the forge's id"`
}

type DeleteUserForgeOutput struct {
	Body UnlinkForgeResponse
}

func DeleteUserForge(ctx context.Context, input *DeleteUserForgeInput) (*DeleteUserForgeOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)

	userForge, err := _store.UserForgeGet(user.ID, input.ForgeID)
	if err != nil {
		return nil, dbErr(err)
	}

	// Prevent unlinking the primary forge
	if userForge.IsPrimary {
		return nil, huma.Error403Forbidden("Cannot unlink primary forge")
	}

	// Get all active repos owned by this user from this forge
	affectedRepos, err := _store.RepoListByUserAndForge(user.ID, input.ForgeID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Error fetching affected repos", err)
	}

	// Get the forge to remove webhooks
	_forge, err := server.Config.Services.Manager.ForgeByID(input.ForgeID)
	if err != nil {
		log.Error().Err(err).Int64("forge_id", input.ForgeID).Msg("Cannot get forge for webhook removal")
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
		return nil, huma.Error500InternalServerError("Error unlinking forge", err)
	}

	return &DeleteUserForgeOutput{Body: UnlinkForgeResponse{
		Message:       fmt.Sprintf("Forge unlinked successfully. %d repos were disabled.", disabledCount),
		DisabledRepos: disabledCount,
	}}, nil
}

// UnlinkForgeResponse represents the response when unlinking a forge.
type UnlinkForgeResponse struct {
	Message       string `json:"message"`
	DisabledRepos int    `json:"disabled_repos"`
}

type GetAvailableForgesInput struct{}

type GetAvailableForgesOutput struct {
	Body []*AvailableForgeResponse
}

func GetAvailableForges(ctx context.Context, _ *GetAvailableForgesInput) (*GetAvailableForgesOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)

	// Get all forges
	forges, err := _store.ForgeListAll()
	if err != nil {
		return nil, huma.Error500InternalServerError("Error fetching forges", err)
	}

	// Get user's linked forges
	userForges, err := _store.UserForgeList(user.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Error fetching user forges", err)
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

	return &GetAvailableForgesOutput{Body: responses}, nil
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
