// Copyright 2025 Crow Authors
// Copyright 2022 Woodpecker Authors
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
	"context"
	"encoding/base32"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/tink-crypto/tink-go/v2/subtle/random"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/forge"
	forge_types "codefloe.com/crowci/crow/v6/server/forge/types"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/types"
	"codefloe.com/crowci/crow/v6/shared/token"
	shared_utils "codefloe.com/crowci/crow/v6/shared/utils"
)

// RepoFields is a method-free copy of model.Repo for clean huma schema generation.
type RepoFields model.Repo

// RepoResponse wraps a Repo with optional messages for the frontend.
type RepoResponse struct {
	RepoFields
	Messages []string `json:"messages,omitempty"`
}

// getUserWithForgeCredentials returns a copy of the user with credentials from the specified forge.
// If forgeID is 0, it returns the user with their primary/current forge credentials.
// This allows using forge APIs with the correct access tokens for multi-forge users.
func getUserWithForgeCredentials(_store store.Store, user *model.User, forgeID int64) (*model.User, int64, error) {
	// If no forge specified, use the user's current forge
	if forgeID == 0 {
		forgeID = user.ForgeID
	}

	// Try to get UserForge credentials for the specified forge
	userForge, err := _store.UserForgeGet(user.ID, forgeID)
	if err != nil {
		if errors.Is(err, types.RecordNotExist) {
			// User doesn't have this forge linked
			return nil, forgeID, fmt.Errorf("user does not have forge %d linked", forgeID)
		}
		return nil, forgeID, fmt.Errorf("failed to get user forge credentials: %w", err)
	}

	// Create a copy of the user with the forge-specific credentials
	userCopy := *user
	userCopy.ForgeID = forgeID
	userCopy.ForgeRemoteID = userForge.ForgeRemoteID
	userCopy.AccessToken = userForge.AccessToken
	userCopy.RefreshToken = userForge.RefreshToken
	userCopy.Expiry = userForge.Expiry

	return &userCopy, forgeID, nil
}

// migrateOrgDataResult contains the result of org data migration (secrets and registries).
type migrateOrgDataResult struct {
	// SecretsCount is the number of secrets migrated
	SecretsCount int
	// RegistriesCount is the number of registry credentials migrated
	RegistriesCount int
	// MigratedToRepo indicates data was migrated to repo level (user lacks dest org admin)
	MigratedToRepo bool
	// SourceOrgName is the name of the source org the data was migrated from
	SourceOrgName string
	// SourceForgeID is the forge ID of the source org
	SourceForgeID int64
	// SkippedReason explains why migration was skipped (empty if migration occurred)
	SkippedReason string
	// SourceSecretsCount is the number of secrets in the source org (for reporting skipped migrations)
	SourceSecretsCount int
	// SourceRegistriesCount is the number of registries in the source org
	SourceRegistriesCount int
}

// migrateOrgDataForRepoActivation handles migrating org secrets and registry credentials
// when a repo is activated, checking for same-named orgs on other forges.
//
// Migration rules:
// - User must be admin of SOURCE org to migrate its secrets/registries
// - If user is admin of DEST org: copy to dest org level
// - If user is NOT admin of DEST org: copy to repo level
// - Individual items that already exist (by name) are skipped
//
// Returns a result with details about what was migrated or why it was skipped.
func migrateOrgDataForRepoActivation(
	c *gin.Context,
	_store store.Store,
	_forge forge.Forge,
	user *model.User,
	destOrg *model.Org,
	repo *model.Repo,
) *migrateOrgDataResult {
	result := &migrateOrgDataResult{}

	// Find all orgs with the same name across all forges
	allOrgs, err := _store.OrgFindByNameAllForges(destOrg.Name)
	if err != nil {
		log.Warn().Err(err).Str("org_name", destOrg.Name).Msg("Failed to find orgs by name for migration")
		return result
	}

	log.Info().
		Int("total_orgs_found", len(allOrgs)).
		Str("org_name", destOrg.Name).
		Int64("dest_org_id", destOrg.ID).
		Msg("Org data migration: found orgs with same name across forges")

	// Look for source orgs from other forges that have secrets or registries
	var sourceOrg *model.Org
	var sourceSecrets []*model.Secret
	var sourceRegistries []*model.Registry

	for _, org := range allOrgs {
		// Skip the destination org itself
		if org.ID == destOrg.ID {
			log.Info().Int64("org_id", org.ID).Msg("Skipping destination org in migration search")
			continue
		}

		// Check if this org has secrets
		secrets, err := _store.OrgSecretList(org.ID, &model.ListOptions{All: true})
		if err != nil {
			log.Warn().Err(err).Int64("org_id", org.ID).Msg("Failed to list org secrets during migration check")
		}

		// Check if this org has registry credentials
		registries, err := _store.OrgRegistryList(org.ID, &model.ListOptions{All: true})
		if err != nil {
			log.Warn().Err(err).Int64("org_id", org.ID).Msg("Failed to list org registries during migration check")
		}

		log.Info().
			Int64("org_id", org.ID).
			Int64("forge_id", org.ForgeID).
			Int("secrets_count", len(secrets)).
			Int("registries_count", len(registries)).
			Msg("Checked source org for migratable data")

		if len(secrets) > 0 || len(registries) > 0 {
			sourceOrg = org
			sourceSecrets = secrets
			sourceRegistries = registries
			break // Use the first org with data
		}
	}

	// No source org with data found
	if sourceOrg == nil || (len(sourceSecrets) == 0 && len(sourceRegistries) == 0) {
		log.Info().
			Str("org_name", destOrg.Name).
			Msg("No source org with secrets/registries found, nothing to migrate")
		return result
	}

	result.SourceOrgName = sourceOrg.Name
	result.SourceForgeID = sourceOrg.ForgeID
	result.SourceSecretsCount = len(sourceSecrets)
	result.SourceRegistriesCount = len(sourceRegistries)

	// Get the forge for the source org
	sourceForge, err := server.Config.Services.Manager.ForgeByID(sourceOrg.ForgeID)
	if err != nil {
		log.Warn().Err(err).Int64("forge_id", sourceOrg.ForgeID).Msg("Failed to get source forge for migration")
		result.SkippedReason = "source_forge_unavailable"
		return result
	}

	// Get user's credentials for the source forge
	sourceUserForge, err := _store.UserForgeGet(user.ID, sourceOrg.ForgeID)
	if err != nil {
		log.Debug().Err(err).Int64("forge_id", sourceOrg.ForgeID).Msg("User has no credentials for source forge")
		result.SkippedReason = "no_source_forge_credentials"
		return result
	}

	// Create a user object with source forge credentials
	sourceUser := &model.User{
		ID:            user.ID,
		Login:         sourceUserForge.ForgeLogin,
		ForgeID:       sourceOrg.ForgeID,
		ForgeRemoteID: sourceUserForge.ForgeRemoteID,
		AccessToken:   sourceUserForge.AccessToken,
		RefreshToken:  sourceUserForge.RefreshToken,
		Expiry:        sourceUserForge.Expiry,
	}

	// Check if user is admin of the SOURCE org
	sourceOrgPerm, err := server.Config.Services.Membership.Get(c, sourceForge, sourceUser, sourceOrg.Name)
	isSourceOrgAdmin := err == nil && sourceOrgPerm != nil && sourceOrgPerm.Admin

	// Personal org (user's own namespace) means they are admin
	if sourceOrg.Name == sourceUser.Login {
		isSourceOrgAdmin = true
	}

	if !isSourceOrgAdmin {
		log.Info().
			Str("org_name", sourceOrg.Name).
			Int64("source_forge_id", sourceOrg.ForgeID).
			Int("secrets_count", len(sourceSecrets)).
			Int("registries_count", len(sourceRegistries)).
			Msg("User is not admin of source org, skipping org data migration")
		result.SkippedReason = "not_source_org_admin"
		return result
	}

	// Check if user is admin of the DESTINATION org via forge membership
	destOrgPerm, err := server.Config.Services.Membership.Get(c, _forge, user, destOrg.Name)
	isDestOrgAdmin := err == nil && destOrgPerm != nil && destOrgPerm.Admin

	// Personal org means they are admin
	if destOrg.Name == user.Login {
		isDestOrgAdmin = true
	}

	result.MigratedToRepo = !isDestOrgAdmin

	// Migrate secrets
	for _, secret := range sourceSecrets {
		newSecret := &model.Secret{
			Name:   secret.Name,
			Value:  secret.Value,
			Images: secret.Images,
			Events: secret.Events,
			Source: secret.Source,
		}

		if isDestOrgAdmin {
			newSecret.OrgID = destOrg.ID
			newSecret.RepoID = 0
		} else {
			newSecret.OrgID = 0
			newSecret.RepoID = repo.ID
		}

		// Check if secret already exists at destination
		var existingSecret *model.Secret
		if isDestOrgAdmin {
			existingSecret, _ = _store.OrgSecretFind(destOrg.ID, secret.Name)
		} else {
			existingSecret, _ = _store.SecretFind(repo, secret.Name)
		}

		if existingSecret != nil {
			log.Debug().
				Str("secret_name", secret.Name).
				Bool("to_org", isDestOrgAdmin).
				Msg("Secret already exists at destination, skipping")
			continue
		}

		if err := _store.SecretCreate(newSecret); err != nil {
			log.Warn().Err(err).Str("secret_name", secret.Name).Msg("Failed to migrate secret")
			continue
		}

		result.SecretsCount++
		log.Info().
			Str("secret_name", secret.Name).
			Int64("source_org_id", sourceOrg.ID).
			Bool("migrated_to_org", isDestOrgAdmin).
			Msg("Migrated org secret")
	}

	// Migrate registry credentials
	for _, registry := range sourceRegistries {
		newRegistry := &model.Registry{
			Address:  registry.Address,
			Username: registry.Username,
			Password: registry.Password,
		}

		if isDestOrgAdmin {
			newRegistry.OrgID = destOrg.ID
			newRegistry.RepoID = 0
		} else {
			newRegistry.OrgID = 0
			newRegistry.RepoID = repo.ID
		}

		// Check if registry already exists at destination (by address)
		var existingRegistry *model.Registry
		if isDestOrgAdmin {
			existingRegistry, _ = _store.OrgRegistryFind(destOrg.ID, registry.Address)
		} else {
			existingRegistry, _ = _store.RegistryFind(repo, registry.Address)
		}

		if existingRegistry != nil {
			log.Debug().
				Str("registry_address", registry.Address).
				Bool("to_org", isDestOrgAdmin).
				Msg("Registry already exists at destination, skipping")
			continue
		}

		if err := _store.RegistryCreate(newRegistry); err != nil {
			log.Warn().Err(err).Str("registry_address", registry.Address).Msg("Failed to migrate registry")
			continue
		}

		result.RegistriesCount++
		log.Info().
			Str("registry_address", registry.Address).
			Int64("source_org_id", sourceOrg.ID).
			Bool("migrated_to_org", isDestOrgAdmin).
			Msg("Migrated org registry credential")
	}

	return result
}

type PostRepoInput struct {
	ForgeRemoteID string `query:"forge_remote_id" doc:"the id of a repository at the forge"`
	ForgeID       int64  `query:"forge_id" doc:"the forge id (for multi-forge users)"`
}

type PostRepoOutput struct {
	Body *RepoResponse
}

func PostRepo(ctx context.Context, input *PostRepoInput) (*PostRepoOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)

	forgeID := input.ForgeID

	// Get user with appropriate forge credentials
	userWithCreds, forgeID, err := getUserWithForgeCredentials(_store, user, forgeID)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get user credentials for forge")
		return nil, huma.Error403Forbidden(fmt.Sprintf("Unable to access forge: %s", err.Error()))
	}

	_forge, err := server.Config.Services.Manager.ForgeByID(forgeID)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get forge info")
		return nil, huma.Error409Conflict("Unable to get forge info")
	}

	// Refresh tokens before forge API calls
	refreshUserTokenForForge(c, userWithCreds, forgeID)
	if c.IsAborted() {
		return nil, huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
	}

	forgeRemoteID := model.ForgeRemoteID(input.ForgeRemoteID)
	if !forgeRemoteID.IsValid() {
		log.Error().Msg("The provided 'forge_remote_id' is either invalid or empty")
		return nil, huma.Error400BadRequest("The provided 'forge_remote_id' is either invalid or empty")
	}

	repo, err := _store.GetRepoByForgeAndRemoteID(forgeID, forgeRemoteID)
	enabledOnce := err == nil // if there's no error, the repo was found and enabled once already
	if enabledOnce && repo.IsActive {
		log.Error().Msg("Repository is already active")
		return nil, huma.Error409Conflict("Repository is already active")
	} else if err != nil && !errors.Is(err, types.RecordNotExist) {
		log.Error().Err(err).Msg("Unable to get repository by its ID")
		return nil, huma.Error500InternalServerError("Unable to get repository by its ID")
	}

	from, err := _forge.Repo(c, userWithCreds, forgeRemoteID, "", "")
	if err != nil {
		log.Error().Err(err).Msg("Unable to get repo information from forge")
		return nil, huma.Error500InternalServerError("Unable to get repo information from forge")
	}
	if !from.Perm.Admin {
		log.Error().Msg("You need to be a repository admin to execute this action")
		return nil, huma.Error403Forbidden("You need to be a repository admin to execute this action")
	}
	if !server.Config.Permissions.OwnersAllowlist.IsAllowed(from) {
		log.Error().Msg("Repository owner is not in allowlist")
		return nil, huma.Error403Forbidden("Repository owner is not in allowlist")
	}

	// If forge_remote_id missed, fall back to (forge_id, owner, name) to handle
	// repos that were deleted+recreated on the forge (the new forge_remote_id
	// differs but the unique (forge_id, owner, name) row still exists).
	if !enabledOnce {
		existing, lookupErr := _store.GetRepoByForgeOwnerName(forgeID, from.Owner, from.Name)
		if lookupErr == nil {
			if existing.IsActive {
				log.Error().Msg("Repository is already active")
				return nil, huma.Error409Conflict("Repository is already active")
			}
			repo = existing
			enabledOnce = true
		} else if !errors.Is(lookupErr, types.RecordNotExist) {
			log.Error().Err(lookupErr).Msg("Unable to look up repository by name")
			return nil, huma.Error500InternalServerError("Unable to look up repository by name")
		}
	}

	if enabledOnce {
		repo.Update(from)
	} else {
		repo = from
		repo.RequireApproval = server.Config.Pipeline.DefaultApprovalMode
		repo.AllowPull = server.Config.Pipeline.DefaultAllowPullRequests
		repo.AllowDeploy = false
		repo.AllowManual = true
		repo.CancelPreviousPipelineEvents = server.Config.Pipeline.DefaultCancelPreviousPipelineEvents
		repo.LogsDurationKeep = server.Config.Pipeline.LogsDurationKeep
		repo.LogsPipelinesKeepMin = server.Config.Pipeline.LogsPipelinesKeepMin
		repo.ForgeID = forgeID
	}
	repo.IsActive = true
	repo.UserID = user.ID

	if repo.Visibility == "" {
		repo.Visibility = model.VisibilityPublic
		if repo.IsSCMPrivate {
			repo.Visibility = model.VisibilityPrivate
		}
	}

	if repo.Timeout == 0 {
		repo.Timeout = server.Config.Pipeline.DefaultTimeout
	} else if repo.Timeout > server.Config.Pipeline.MaxTimeout {
		repo.Timeout = server.Config.Pipeline.MaxTimeout
	}

	repo.LogsPipelinesKeepMin = server.Config.Pipeline.LogsPipelinesKeepMin
	repo.LogsDurationKeep = server.Config.Pipeline.LogsDurationKeep

	if repo.Hash == "" {
		repo.Hash = base32.StdEncoding.EncodeToString(
			random.GetRandomBytes(32),
		)
	}

	// find org of repo
	var org *model.Org
	var newOrgCreated bool
	org, err = _store.OrgFindByName(repo.Owner, forgeID)
	if err != nil && !errors.Is(err, types.RecordNotExist) {
		log.Error().Err(err).Msg("Unable to find organization")
		return nil, huma.Error500InternalServerError("Unable to find organization")
	}

	// create an org if it doesn't exist yet
	if errors.Is(err, types.RecordNotExist) {
		org, err = _forge.Org(c, userWithCreds, repo.Owner)
		if err != nil {
			log.Error().Err(err).Msgf("Organization %s not found in DB. Attempting to create a new one.", repo.Owner)
			return nil, huma.Error500InternalServerError(fmt.Sprintf("Organization %s not found in DB. Attempting to create a new one.", repo.Owner))
		}

		org.ForgeID = forgeID
		err = _store.OrgCreate(org)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create organization")
			return nil, huma.Error500InternalServerError("Failed to create organization")
		}
		newOrgCreated = true
	}

	repo.OrgID = org.ID

	// creates the jwt token used to verify the repository
	t := token.New(token.HookToken)
	t.Set("repo-forge-remote-id", string(forgeRemoteID))
	t.Set("forge-id", strconv.FormatInt(repo.ForgeID, 10))
	sig, err := t.Sign(repo.Hash)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create JWT token")
		return nil, huma.Error500InternalServerError("Failed to create JWT token")
	}

	hookURL := fmt.Sprintf(
		"%s/api/v1/hook?access_token=%s",
		server.Config.Server.WebhookHost,
		sig,
	)

	err = _forge.Activate(c, userWithCreds, repo, hookURL)
	if err != nil {
		log.Error().Err(err).Msg("Failed to activate repo")
		return nil, huma.Error500InternalServerError("Failed to activate repo")
	}

	if enabledOnce {
		err = _store.UpdateRepo(repo)
	} else {
		err = _store.CreateRepo(repo)
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to create/update repo")
		return nil, huma.Error500InternalServerError("Failed to create/update repo")
	}

	repo.Perm = from.Perm
	repo.Perm.Synced = time.Now().Unix()
	repo.Perm.UserID = user.ID
	repo.Perm.RepoID = repo.ID
	repo.Perm.Repo = repo
	err = _store.PermUpsert(repo.Perm)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update permissions")
		return nil, huma.Error500InternalServerError("Failed to update permissions")
	}

	// Attempt to migrate org secrets and registry credentials from same-named orgs on other forges
	var messages []string
	log.Info().
		Int64("org_id", org.ID).
		Str("org_name", org.Name).
		Int64("forge_id", forgeID).
		Bool("new_org", newOrgCreated).
		Msg("Checking for org data migration from other forges")

	migrationResult := migrateOrgDataForRepoActivation(c, _store, _forge, userWithCreds, org, repo)

	log.Info().
		Int("secrets_migrated", migrationResult.SecretsCount).
		Int("registries_migrated", migrationResult.RegistriesCount).
		Str("skipped_reason", migrationResult.SkippedReason).
		Int("source_secrets_count", migrationResult.SourceSecretsCount).
		Int("source_registries_count", migrationResult.SourceRegistriesCount).
		Bool("migrated_to_repo", migrationResult.MigratedToRepo).
		Msg("Org data migration result")

	// Generate user-facing messages based on migration result
	if migrationResult.SkippedReason != "" {
		// Migration was skipped - inform user if there was data to migrate
		totalSourceData := migrationResult.SourceSecretsCount + migrationResult.SourceRegistriesCount
		if totalSourceData > 0 {
			switch migrationResult.SkippedReason {
			case "not_source_org_admin":
				messages = append(messages, fmt.Sprintf(
					"Found %d secret(s) and %d registry credential(s) in organization '%s' on another forge, "+
						"but they could not be migrated because you are not an admin of that organization.",
					migrationResult.SourceSecretsCount, migrationResult.SourceRegistriesCount, migrationResult.SourceOrgName))
			case "no_source_forge_credentials":
				messages = append(messages, fmt.Sprintf(
					"Found %d secret(s) and %d registry credential(s) in organization '%s' on another forge, "+
						"but they could not be migrated because you are not logged into that forge.",
					migrationResult.SourceSecretsCount, migrationResult.SourceRegistriesCount, migrationResult.SourceOrgName))
			case "source_forge_unavailable":
				messages = append(messages, fmt.Sprintf(
					"Found %d secret(s) and %d registry credential(s) in organization '%s' on another forge, "+
						"but they could not be migrated because the source forge is unavailable.",
					migrationResult.SourceSecretsCount, migrationResult.SourceRegistriesCount, migrationResult.SourceOrgName))
			}
		}
	} else {
		// Migration was attempted - report what was migrated
		totalMigrated := migrationResult.SecretsCount + migrationResult.RegistriesCount
		if totalMigrated > 0 {
			if migrationResult.MigratedToRepo {
				// Migrated to repo level because user is not dest org admin
				if migrationResult.SecretsCount > 0 {
					messages = append(messages, fmt.Sprintf(
						"Migrated %d organization secret(s) to repository level because you are not an admin of '%s' on this forge.",
						migrationResult.SecretsCount, org.Name))
				}
				if migrationResult.RegistriesCount > 0 {
					messages = append(messages, fmt.Sprintf(
						"Migrated %d registry credential(s) to repository level because you are not an admin of '%s' on this forge.",
						migrationResult.RegistriesCount, org.Name))
				}
			} else {
				// Migrated to org level
				if migrationResult.SecretsCount > 0 {
					messages = append(messages, fmt.Sprintf(
						"Migrated %d organization secret(s) from your organization on another forge.",
						migrationResult.SecretsCount))
				}
				if migrationResult.RegistriesCount > 0 {
					messages = append(messages, fmt.Sprintf(
						"Migrated %d registry credential(s) from your organization on another forge.",
						migrationResult.RegistriesCount))
				}
			}
		}
	}

	// Return response with optional messages
	log.Info().
		Int("message_count", len(messages)).
		Strs("messages", messages).
		Msg("Returning repo activation response")

	resp := &RepoResponse{RepoFields: RepoFields(*repo)}
	if len(messages) > 0 {
		resp.Messages = messages
	}
	return &PostRepoOutput{Body: resp}, nil
}

type PatchRepoInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Body   model.RepoPatch
}

type PatchRepoOutput struct {
	Body *model.Repo
}

func PatchRepo(ctx context.Context, input *PatchRepoInput) (*PatchRepoOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)

	in := &input.Body

	valid, err := shared_utils.IsValidKeepMinValue(in.LogsPipelinesKeepMin)
	if err != nil || !valid {
		log.Error().Err(err).Msgf("The provided minimum pipeline log retention value is invalid")
		return nil, huma.Error400BadRequest("The provided minimum pipeline log retention value is invalid")
	}

	err = shared_utils.ValidateDuration(in.LogsDurationKeep)
	if err != nil {
		log.Error().Err(err).Msgf("The provided maximum pipeline log duration retention value is invalid. Did you forget the unit indicator or use an invalid one?")
		return nil, huma.Error400BadRequest("The provided maximum pipeline log duration retention value is invalid. Did you forget the unit indicator or use an invalid one?")
	}

	if in.Timeout != nil && *in.Timeout > server.Config.Pipeline.MaxTimeout {
		log.Error().Msg("Timeout is not allowed to be higher than max. configured timeout")
		return nil, huma.Error400BadRequest(fmt.Sprintf("Timeout is not allowed to be higher than max. configured timeout (%d min)", server.Config.Pipeline.MaxTimeout))
	}

	if in.Trusted != nil {
		// Check if any trusted settings are actually being changed
		changingNetwork := in.Trusted.Network != nil && *in.Trusted.Network != repo.Trusted.Network
		changingVolumes := in.Trusted.Volumes != nil && *in.Trusted.Volumes != repo.Trusted.Volumes
		changingSecurity := in.Trusted.Security != nil && *in.Trusted.Security != repo.Trusted.Security

		if (changingNetwork || changingVolumes || changingSecurity) && !user.Admin {
			log.Trace().Msgf("user '%s' wants to change trusted without being an instance admin", user.Login)
			return nil, huma.Error403Forbidden("Instance admin privileges required to change trusted repository settings")
		}

		if in.Trusted.Network != nil {
			repo.Trusted.Network = *in.Trusted.Network
		}
		if in.Trusted.Security != nil {
			repo.Trusted.Security = *in.Trusted.Security
		}
		if in.Trusted.Volumes != nil {
			repo.Trusted.Volumes = *in.Trusted.Volumes
		}
	}

	if in.AllowPull != nil {
		repo.AllowPull = *in.AllowPull
	}
	if in.AllowDeploy != nil {
		repo.AllowDeploy = *in.AllowDeploy
	}
	if in.AllowManual != nil {
		repo.AllowManual = *in.AllowManual
	}
	if in.DeployTeam != nil {
		repo.DeployTeam = *in.DeployTeam
	}

	if in.RequireApproval != nil {
		if mode := model.ApprovalMode(*in.RequireApproval); mode.Valid() {
			repo.RequireApproval = mode
		} else {
			log.Error().Msg("Invalid require-approval setting")
			return nil, huma.Error400BadRequest("Invalid require-approval setting")
		}
	}
	if in.Timeout != nil {
		repo.Timeout = *in.Timeout
	}
	if in.Config != nil {
		repo.Config = *in.Config
	}
	if in.CancelPreviousPipelineEvents != nil {
		repo.CancelPreviousPipelineEvents = *in.CancelPreviousPipelineEvents
	}
	if in.NetrcTrusted != nil {
		repo.NetrcTrustedPlugins = *in.NetrcTrusted
	}
	if in.Visibility != nil {
		switch *in.Visibility {
		case string(model.VisibilityInternal), string(model.VisibilityPrivate), string(model.VisibilityPublic):
			repo.Visibility = model.RepoVisibility(*in.Visibility)
		default:
			log.Error().Msg("Invalid visibility type")
			return nil, huma.Error400BadRequest("Invalid visibility type")
		}
	}

	repo.LogsPipelinesKeepMin = in.LogsPipelinesKeepMin
	repo.LogsDurationKeep = in.LogsDurationKeep

	err = _store.UpdateRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update repo")
		return nil, huma.Error500InternalServerError("Failed to update repo", err)
	}

	return &PatchRepoOutput{Body: repo}, nil
}

type ChownRepoInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
}

type ChownRepoOutput struct {
	Body *model.Repo
}

func ChownRepo(ctx context.Context, input *ChownRepoInput) (*ChownRepoOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)
	repo.UserID = user.ID

	err := _store.UpdateRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update repo")
		return nil, huma.Error500InternalServerError("Failed to update repo", err)
	}
	return &ChownRepoOutput{Body: repo}, nil
}

type LookupRepoInput struct {
	RepoFullName string `path:"repo_full_name" doc:"the repository full name / slug"`
}

type LookupRepoOutput struct {
	Body *model.Repo
}

func LookupRepo(ctx context.Context, input *LookupRepoInput) (*LookupRepoOutput, error) {
	c := ginCtx(ctx)
	return &LookupRepoOutput{Body: session.Repo(c)}, nil
}

// LookupRepoGin is a gin handler wrapper for LookupRepo (needed because the
// route uses a wildcard path param that huma doesn't support).
func LookupRepoGin(c *gin.Context) {
	c.JSON(http.StatusOK, session.Repo(c))
}

type GetRepoInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
}

type GetRepoOutput struct {
	Body *model.Repo
}

func GetRepo(ctx context.Context, input *GetRepoInput) (*GetRepoOutput, error) {
	c := ginCtx(ctx)
	return &GetRepoOutput{Body: session.Repo(c)}, nil
}

type GetRepoPermissionsInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
}

type GetRepoPermissionsOutput struct {
	Body *model.Perm
}

func GetRepoPermissions(ctx context.Context, input *GetRepoPermissionsInput) (*GetRepoPermissionsOutput, error) {
	c := ginCtx(ctx)
	perm := session.Perm(c)
	return &GetRepoPermissionsOutput{Body: perm}, nil
}

type GetRepoBranchesInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	PaginationParams
}

type GetRepoBranchesOutput struct {
	Body []string
}

// repoForgeSession resolves the forge for a repo along with the repo owner
// carrying a refreshed token for that forge, which every ref-listing endpoint
// needs before it can call the forge.
func repoForgeSession(c *gin.Context, repo *model.Repo) (forge.Forge, *model.User, error) {
	_store := store.FromContext(c)

	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get forge information")
		return nil, nil, huma.Error500InternalServerError("Unable to get forge information")
	}

	repoUser, err := _store.GetUser(repo.UserID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user")
		return nil, nil, dbErr(err)
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	if userForge, ufErr := _store.UserForgeGet(repoUser.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		repoUser.AccessToken = userForge.AccessToken
		repoUser.RefreshToken = userForge.RefreshToken
		repoUser.Expiry = userForge.Expiry
		repoUser.ForgeID = userForge.ForgeID
		repoUser.ForgeRemoteID = userForge.ForgeRemoteID
	}

	if refreshErr := forge.RefreshUserForge(c, _forge, _store, repoUser, repo.ForgeID); refreshErr != nil {
		log.Error().Err(refreshErr).Msg("Failed to refresh user auth token")
		return nil, nil, huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
	}

	return _forge, repoUser, nil
}

// forgeRefErr maps a ref-listing failure to an API error, distinguishing an
// expired authorization from a genuine forge problem.
func forgeRefErr(err error, what string) error {
	log.Error().Err(err).Msgf("Failed to get %s from repo", what)
	if errors.Is(err, &forge_types.ErrTokenRefreshFailed{}) {
		return huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
	}
	return huma.Error500InternalServerError(err.Error())
}

func GetRepoBranches(ctx context.Context, input *GetRepoBranchesInput) (*GetRepoBranchesOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)

	_forge, repoUser, err := repoForgeSession(c, repo)
	if err != nil {
		return nil, err
	}

	branches, err := _forge.Branches(c, repoUser, repo, &model.ListOptions{Page: input.Page, PerPage: input.PerPage})
	if err != nil {
		return nil, forgeRefErr(err, "branches")
	}

	return &GetRepoBranchesOutput{Body: branches}, nil
}

type GetRepoTagsInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	PaginationParams
}

type GetRepoTagsOutput struct {
	Body []string
}

// GetRepoTags lists a repository's tags. Tag support is optional per forge, so
// this returns 501 rather than an error for forges that cannot list tags.
func GetRepoTags(ctx context.Context, input *GetRepoTagsInput) (*GetRepoTagsOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)

	_forge, repoUser, err := repoForgeSession(c, repo)
	if err != nil {
		return nil, err
	}

	tagForge, ok := _forge.(forge.TagForge)
	if !ok {
		return nil, huma.Error501NotImplemented("this forge does not support listing tags")
	}

	tags, err := tagForge.Tags(c, repoUser, repo, &model.ListOptions{Page: input.Page, PerPage: input.PerPage})
	if err != nil {
		return nil, forgeRefErr(err, "tags")
	}

	return &GetRepoTagsOutput{Body: tags}, nil
}

type GetRepoPullRequestsInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	PaginationParams
}

type GetRepoPullRequestsOutput struct {
	Body []*model.PullRequest
}

func GetRepoPullRequests(ctx context.Context, input *GetRepoPullRequestsInput) (*GetRepoPullRequestsOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get repo information from forge")
		return nil, huma.Error500InternalServerError("Unable to get repo information from forge")
	}

	repoUser, err := _store.GetUser(repo.UserID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user")
		return nil, dbErr(err)
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	if userForge, ufErr := _store.UserForgeGet(repoUser.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		repoUser.AccessToken = userForge.AccessToken
		repoUser.RefreshToken = userForge.RefreshToken
		repoUser.Expiry = userForge.Expiry
		repoUser.ForgeID = userForge.ForgeID
		repoUser.ForgeRemoteID = userForge.ForgeRemoteID
	}

	if refreshErr := forge.RefreshUserForge(c, _forge, _store, repoUser, repo.ForgeID); refreshErr != nil {
		log.Error().Err(refreshErr).Msg("Failed to refresh user auth token")
		return nil, huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
	}

	prs, err := _forge.PullRequests(c, repoUser, repo, &model.ListOptions{Page: input.Page, PerPage: input.PerPage})
	if err != nil {
		log.Error().Err(err).Msg("Unable to get pull requests from forge")
		if errors.Is(err, &forge_types.ErrTokenRefreshFailed{}) {
			return nil, huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
		}
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &GetRepoPullRequestsOutput{Body: prs}, nil
}

type DeleteRepoInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Remove bool  `query:"remove" doc:"permanently delete the repository"`
}

type DeleteRepoOutput struct {
	Body *model.Repo
}

func DeleteRepo(ctx context.Context, input *DeleteRepoInput) (*DeleteRepoOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get repo information from forge")
		return nil, huma.Error500InternalServerError("Unable to get repo information from forge")
	}

	// For multi-forge support: get the user's tokens for this repo's forge
	forgeUser, _, err := getUserWithForgeCredentials(_store, user, repo.ForgeID)
	if err != nil {
		log.Warn().Err(err).Int64("forge_id", repo.ForgeID).Msg("User does not have tokens for repo's forge, will try to delete anyway")
		forgeUser = user
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, forgeUser, repo.ForgeID)
	if c.IsAborted() {
		return nil, huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
	}

	// Removing the webhook on the forge is best-effort: if the repo was deleted
	// on the forge the call fails (e.g. 404), but that must not block local
	// removal — otherwise the stale repo row becomes permanently un-deletable
	// (see issue #1226). A leftover webhook on a missing/recreated forge repo is
	// harmless: its hook calls no longer resolve to an active Crow repo.
	if err := _forge.Deactivate(c, forgeUser, repo, server.Config.Server.WebhookHost); err != nil {
		log.Warn().Err(err).Str("repo", repo.FullName).Msg("Failed to deactivate repo on forge, removing from Crow anyway")
	}

	if input.Remove {
		if err := _store.DeleteRepo(repo); err != nil {
			log.Error().Err(err).Msg("Failed to delete repo")
			return nil, dbErr(err)
		}
	} else {
		repo.IsActive = false
		repo.UserID = 0

		if err := _store.UpdateRepo(repo); err != nil {
			log.Error().Err(err).Msg("Unable to update repo")
			return nil, huma.Error500InternalServerError("Unable to update repo", err)
		}
	}

	return &DeleteRepoOutput{Body: repo}, nil
}

type RepairRepoInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
}

func RepairRepo(ctx context.Context, input *RepairRepoInput) (*struct{}, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)
	if err := repairRepo(c, repo, true, false); err != nil {
		return nil, err
	}
	return nil, nil
}

type MoveRepoInput struct {
	RepoID int64  `path:"repo_id" doc:"the repository id"`
	To     string `query:"to" required:"true" doc:"the username to move the repository to"`
}

func MoveRepo(ctx context.Context, input *MoveRepoInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get repo information from forge")
		return nil, huma.Error500InternalServerError("Unable to get repo information from forge")
	}

	// For multi-forge support: get the user's tokens for this repo's forge
	forgeUser, _, err := getUserWithForgeCredentials(_store, user, repo.ForgeID)
	if err != nil {
		log.Error().Err(err).Int64("forge_id", repo.ForgeID).Msg("User does not have tokens for repo's forge")
		return nil, huma.Error403Forbidden("You don't have access to this repository's forge")
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, forgeUser, repo.ForgeID)
	if c.IsAborted() {
		return nil, huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
	}

	owner, name, errParse := model.ParseRepo(input.To)
	if errParse != nil {
		log.Error().Err(errParse).Msg("Error parsing repo")
		return nil, huma.Error500InternalServerError("Error parsing repo", errParse)
	}

	from, err := _forge.Repo(c, forgeUser, "", owner, name)
	if err != nil {
		log.Error().Err(err).Msg("Error retrieving repo")
		return nil, huma.Error500InternalServerError("Error retrieving repo", err)
	}

	if !from.Perm.Admin {
		log.Error().Msg("Insufficient permissions to move repo")
		return nil, huma.Error401Unauthorized("Insufficient permissions to move repo")
	}

	err = _store.CreateRedirection(&model.Redirection{ForgeID: repo.ForgeID, RepoID: repo.ID, FullName: repo.FullName})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create redirection")
		return nil, huma.Error500InternalServerError("Failed to create redirection", err)
	}

	repo.Update(from)
	errStore := _store.UpdateRepo(repo)
	if errStore != nil {
		log.Error().Err(errStore).Msg("Failed to update repo")
		return nil, huma.Error500InternalServerError("Failed to update repo", errStore)
	}

	repo.Perm = from.Perm
	errStore = _store.PermUpsert(repo.Perm)
	if errStore != nil {
		log.Error().Err(errStore).Msg("Failed to update permissions")
		return nil, huma.Error500InternalServerError("Failed to update permissions", errStore)
	}

	// creates the jwt token used to verify the repository
	t := token.New(token.HookToken)
	t.Set("repo-forge-remote-id", string(repo.ForgeRemoteID))
	t.Set("forge-id", strconv.FormatInt(repo.ForgeID, 10))
	sig, err := t.Sign(repo.Hash)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create JWT token")
		return nil, huma.Error500InternalServerError("Failed to create JWT token")
	}

	// reconstruct the hook url
	host := server.Config.Server.WebhookHost
	hookURL := fmt.Sprintf(
		"%s/api/v1/hook?access_token=%s",
		host,
		sig,
	)

	if err := _forge.Deactivate(c, forgeUser, repo, host); err != nil {
		log.Error().Err(err).Msg("Failed to deactivate repo (during move operation)")
	}
	if err := _forge.Activate(c, forgeUser, repo, hookURL); err != nil {
		log.Error().Err(err).Msg("Failed to activate repo (during move operation)")
		return nil, huma.Error500InternalServerError("Failed to activate repo (during move operation)")
	}
	return nil, nil
}

type GetAllReposInput struct {
	PaginationParams
	Active bool `query:"active" doc:"only list active repos"`
}

type GetAllReposOutput struct {
	Body []*model.Repo
}

func GetAllRepos(ctx context.Context, input *GetAllReposInput) (*GetAllReposOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	repos, err := _store.RepoListAll(input.Active, &model.ListOptions{Page: input.Page, PerPage: input.PerPage})
	if err != nil {
		log.Error().Err(err).Msg("Error fetching repository list")
		return nil, huma.Error500InternalServerError("Error fetching repository list")
	}

	return &GetAllReposOutput{Body: repos}, nil
}

type GetPublicReposInput struct {
	PaginationParams
}

type GetPublicReposOutput struct {
	Body []*model.RepoLastPipeline
}

func GetPublicRepos(ctx context.Context, input *GetPublicReposInput) (*GetPublicReposOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	repos, err := _store.RepoListPublic(&model.ListOptions{Page: input.Page, PerPage: input.PerPage})
	if err != nil {
		log.Error().Err(err).Msg("Error fetching public repository list")
		return nil, huma.Error500InternalServerError("Error fetching public repository list")
	}

	// Fetch latest pipelines for each repo
	repoIDs := make([]int64, len(repos))
	for i, repo := range repos {
		repoIDs[i] = repo.ID
	}

	pipelines, err := _store.GetRepoLatestPipelines(repoIDs)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching latest pipelines for public repos")
		return nil, huma.Error500InternalServerError("Error fetching latest pipelines")
	}

	latestPipelines := make(map[int64]*model.Pipeline, len(repos))
	for _, pipeline := range pipelines {
		latestPipelines[pipeline.RepoID] = pipeline
	}

	result := make([]*model.RepoLastPipeline, len(repos))
	for i, repo := range repos {
		result[i] = &model.RepoLastPipeline{
			Repo:         *repo,
			LastPipeline: latestPipelines[repo.ID],
		}
	}

	return &GetPublicReposOutput{Body: result}, nil
}

type RepairAllReposInput struct{}

func RepairAllRepos(ctx context.Context, input *RepairAllReposInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	repos, err := _store.RepoListAll(true, &model.ListOptions{All: true})
	if err != nil {
		log.Error().Err(err).Msg("Error fetching repository list")
		return nil, huma.Error500InternalServerError("Error fetching repository list")
	}

	for _, r := range repos {
		if err := repairRepo(c, r, false, true); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func repairRepo(c *gin.Context, repo *model.Repo, withPerms, skipOnErr bool) error {
	_store := store.FromContext(c)
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get repo information from forge")
		return huma.Error500InternalServerError("Unable to get repo information from forge")
	}

	user, err := _store.GetUser(repo.UserID)
	if err != nil {
		if errors.Is(err, types.RecordNotExist) {
			oldUserID := repo.UserID
			user = session.User(c)
			repo.UserID = user.ID
			err = _store.UpdateRepo(repo)
			if err != nil {
				log.Error().Err(err).Msg("Failed to update repo user ID")
				return huma.Error500InternalServerError("Failed to update repo user ID", err)
			}
			log.Debug().Msgf("Could not find repo user with ID %d during repo repair. Setting to repair request user with ID %d", oldUserID, user.ID)
		} else {
			log.Error().Err(err).Msg("Failed to get user")
			return huma.Error500InternalServerError("Failed to get user", err)
		}
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, user, repo.ForgeID)
	if c.IsAborted() {
		return huma.Error401Unauthorized("Your authorization has expired and couldn't be refreshed automatically. Please log in again")
	}

	// creates the jwt token used to verify the repository
	t := token.New(token.HookToken)
	t.Set("repo-forge-remote-id", string(repo.ForgeRemoteID))
	t.Set("forge-id", strconv.FormatInt(repo.ForgeID, 10))
	sig, err := t.Sign(repo.Hash)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create JWT token")
		return huma.Error500InternalServerError("Failed to create JWT token")
	}

	// reconstruct the hook url
	host := server.Config.Server.WebhookHost
	hookURL := fmt.Sprintf(
		"%s/api/v1/hook?access_token=%s",
		host,
		sig,
	)

	from, err := _forge.Repo(c, user, repo.ForgeRemoteID, repo.Owner, repo.Name)
	if err != nil {
		log.Error().Err(err).Msgf("get repo '%s/%s' from forge", repo.Owner, repo.Name)
		if !skipOnErr {
			return huma.Error500InternalServerError("Unable to get repo information from forge")
		}
		return nil
	}

	if repo.FullName != from.FullName {
		// create a redirection
		err = _store.CreateRedirection(&model.Redirection{ForgeID: repo.ForgeID, RepoID: repo.ID, FullName: repo.FullName})
		if err != nil {
			log.Error().Err(err).Msg("Unable to create redirection")
			return huma.Error500InternalServerError("Unable to create redirection", err)
		}
	}

	repo.Update(from)
	if err := _store.UpdateRepo(repo); err != nil {
		log.Error().Err(err).Msg("Failed to update repo")
		return huma.Error500InternalServerError("Failed to update repo", err)
	}
	if withPerms {
		repo.Perm.Pull = from.Perm.Pull
		repo.Perm.Push = from.Perm.Push
		repo.Perm.Admin = from.Perm.Admin
		if err := _store.PermUpsert(repo.Perm); err != nil {
			log.Error().Err(err).Msg("Unable to update permissions")
			return huma.Error500InternalServerError("Unable to update permissions", err)
		}
	}

	if err := _forge.Deactivate(c, user, repo, host); err != nil {
		log.Error().Err(err).Msgf("Failed to deactivate repo '%s' to repair", repo.FullName)
	}
	if err := _forge.Activate(c, user, repo, hookURL); err != nil {
		log.Error().Err(err).Msg("Failed to activate repo")
		return huma.Error500InternalServerError("Failed to activate repo")
	}
	return nil
}
