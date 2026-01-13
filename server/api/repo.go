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
	"encoding/base32"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/tink-crypto/tink-go/v2/subtle/random"

	"codeberg.org/crowci/crow/v5/server"
	"codeberg.org/crowci/crow/v5/server/forge"
	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/router/middleware/session"
	"codeberg.org/crowci/crow/v5/server/store"
	"codeberg.org/crowci/crow/v5/server/store/types"
	"codeberg.org/crowci/crow/v5/shared/token"
	shared_utils "codeberg.org/crowci/crow/v5/shared/utils"
)

// RepoResponse wraps a Repo with optional messages for the frontend.
type RepoResponse struct {
	*model.Repo
	Messages []string `json:"messages,omitempty"`
} //	@name	RepoResponse

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

// PostRepo
//
//	@Summary	Activate a repository
//	@Router		/repos [post]
//	@Produce	json
//	@Success	200	{object}	Repo
//	@Tags		Repositories
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		forge_remote_id	query	string	true	"the id of a repository at the forge"
//	@Param		forge_id		query	int		false	"the forge id (for multi-forge users)"
func PostRepo(c *gin.Context) {
	_store := store.FromContext(c)
	user := session.User(c)

	// Parse optional forge_id parameter for multi-forge support
	var forgeID int64
	if forgeIDStr := c.Query("forge_id"); forgeIDStr != "" {
		var err error
		forgeID, err = strconv.ParseInt(forgeIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Invalid forge_id parameter. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
			})
			return
		}
	}

	// Get user with appropriate forge credentials
	userWithCreds, forgeID, err := getUserWithForgeCredentials(_store, user, forgeID)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get user credentials for forge")
		c.JSON(http.StatusForbidden, gin.H{
			"error": fmt.Sprintf("Unable to access forge: %s. Endpoint: %s %s", err.Error(), c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	_forge, err := server.Config.Services.Manager.ForgeByID(forgeID)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get forge info")
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf("Unable to get forge info. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	// Refresh tokens before forge API calls
	isSecondaryForge := forgeID != user.ForgeID
	refreshUserTokenForForge(c, userWithCreds, forgeID, isSecondaryForge)
	if c.IsAborted() {
		return
	}

	forgeRemoteID := model.ForgeRemoteID(c.Query("forge_remote_id"))
	if !forgeRemoteID.IsValid() {
		log.Error().Msg("The provided 'forge_remote_id' is either invalid or empty")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("The provided 'forge_remote_id' is either invalid or empty. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	repo, err := _store.GetRepoForgeID(forgeRemoteID)
	enabledOnce := err == nil // if there's no error, the repo was found and enabled once already
	if enabledOnce && repo.IsActive {
		log.Error().Msg("Repository is already active")
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf("Repository is already active. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	} else if err != nil && !errors.Is(err, types.RecordNotExist) {
		log.Error().Err(err).Msg("Unable to get repository by its ID")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Unable to get repository by its ID. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	from, err := _forge.Repo(c, userWithCreds, forgeRemoteID, "", "")
	if err != nil {
		log.Error().Err(err).Msg("Unable to get repo information from forge")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Unable to get repo information from forge. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}
	if !from.Perm.Admin {
		log.Error().Msg("You need to be a repository admin to execute this action")
		c.JSON(http.StatusForbidden, gin.H{
			"error": fmt.Sprintf("You need to be a repository admin to execute this action. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}
	if !server.Config.Permissions.OwnersAllowlist.IsAllowed(from) {
		log.Error().Msg("Repository owner is not in allowlist")
		c.JSON(http.StatusForbidden, gin.H{
			"error": fmt.Sprintf("Repository owner is not in allowlist. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	if enabledOnce {
		repo.Update(from)
	} else {
		repo = from
		repo.RequireApproval = server.Config.Pipeline.DefaultApprovalMode
		repo.AllowPull = server.Config.Pipeline.DefaultAllowPullRequests
		repo.AllowDeploy = false
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Unable to find organization. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	// create an org if it doesn't exist yet
	if errors.Is(err, types.RecordNotExist) {
		org, err = _forge.Org(c, userWithCreds, repo.Owner)
		if err != nil {
			msg := fmt.Sprintf("Organization %s not found in DB. Attempting to create a new one.", repo.Owner)
			log.Error().Err(err).Msg(msg)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Organization %s not found in DB. Attempting to create a new one. Endpoint: %s %s", repo.Owner, c.Request.Method, c.Request.URL.Path),
			})
			return
		}

		org.ForgeID = forgeID
		err = _store.OrgCreate(org)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create organization")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to create organization. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
			})
			return
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to create JWT token. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	hookURL := fmt.Sprintf(
		"%s/api/v1/hook?access_token=%s",
		server.Config.Server.WebhookHost,
		sig,
	)

	err = _forge.Activate(c, userWithCreds, repo, hookURL)
	if err != nil {
		log.Error().Err(err).Msg("Failed to activate repo")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to activate repo. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	if enabledOnce {
		err = _store.UpdateRepo(repo)
	} else {
		err = _store.CreateRepo(repo)
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to create/update repo")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to create/update repo. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	repo.Perm = from.Perm
	repo.Perm.Synced = time.Now().Unix()
	repo.Perm.UserID = user.ID
	repo.Perm.RepoID = repo.ID
	repo.Perm.Repo = repo
	err = _store.PermUpsert(repo.Perm)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update permissions")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to update permissions. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
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

	if len(messages) > 0 {
		c.JSON(http.StatusOK, RepoResponse{Repo: repo, Messages: messages})
	} else {
		c.JSON(http.StatusOK, repo)
	}
}

// PatchRepo
//
//	@Summary	Update a repository
//	@Router		/repos/{repo_id} [patch]
//	@Produce	json
//	@Success	200	{object}	Repo
//	@Tags		Repositories
//	@Param		Authorization	header	string		true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int			true	"the repository id"
//	@Param		repo			body	RepoPatch	true	"the repository's information"
func PatchRepo(c *gin.Context) {
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)

	in := new(model.RepoPatch)
	if err := c.Bind(in); err != nil {
		log.Error().Err(err).Msg("Failed to bind request body")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request body: %v. Endpoint: %s %s", err, c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	valid, err := shared_utils.IsValidKeepMinValue(in.LogsPipelinesKeepMin)
	if err != nil || !valid {
		log.Error().Err(err).Msgf("The provided minimum pipeline log retention value is invalid")
		// Return a JSON response with a specific error field for the frontend
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("The provided minimum pipeline log retention value is invalid. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	err = shared_utils.ValidateDuration(in.LogsDurationKeep)
	if err != nil {
		log.Error().Err(err).Msgf("The provided maximum pipeline log duration retention value is invalid. Did you forget the unit indicator or use an invalid one?")
		// Return a JSON response with a specific error field for the frontend
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("The provided maximum pipeline log duration retention value is invalid. Did you forget the unit indicator or use an invalid one? Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	if in.Timeout != nil && *in.Timeout > server.Config.Pipeline.MaxTimeout {
		log.Error().Msg("Timeout is not allowed to be higher than max. configured timeout")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Timeout is not allowed to be higher than max. configured timeout (%d min). Endpoint: %s %s", server.Config.Pipeline.MaxTimeout, c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	if in.Trusted != nil {
		// Check if any trusted settings are actually being changed
		changingNetwork := in.Trusted.Network != nil && *in.Trusted.Network != repo.Trusted.Network
		changingVolumes := in.Trusted.Volumes != nil && *in.Trusted.Volumes != repo.Trusted.Volumes
		changingSecurity := in.Trusted.Security != nil && *in.Trusted.Security != repo.Trusted.Security

		if (changingNetwork || changingVolumes || changingSecurity) && !user.Admin {
			log.Trace().Msgf("user '%s' wants to change trusted without being an instance admin", user.Login)
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Instance admin privileges required to change trusted repository settings",
			})
			return
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
	if in.DeployTeam != nil {
		repo.DeployTeam = *in.DeployTeam
	}

	if in.RequireApproval != nil {
		if mode := model.ApprovalMode(*in.RequireApproval); mode.Valid() {
			repo.RequireApproval = mode
		} else {
			log.Error().Msg("Invalid require-approval setting")
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Invalid require-approval setting. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
			})
			return
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
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Invalid visibility type. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
			})
			return
		}
	}

	repo.LogsPipelinesKeepMin = in.LogsPipelinesKeepMin
	repo.LogsDurationKeep = in.LogsDurationKeep

	err = _store.UpdateRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update repo")
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, repo)
}

// ChownRepo
//
//	@Summary	Change a repository's owner to the currently authenticated user
//	@Router		/repos/{repo_id}/chown [post]
//	@Produce	json
//	@Success	200	{object}	Repo
//	@Tags		Repositories
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
func ChownRepo(c *gin.Context) {
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)
	repo.UserID = user.ID

	err := _store.UpdateRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update repo")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to update repo. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, repo)
}

// LookupRepo
//
//	@Summary	Lookup a repository by full name
//	@Router		/repos/lookup/{repo_full_name} [get]
//	@Produce	json
//	@Success	200	{object}	Repo
//	@Tags		Repositories
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_full_name	path	string	true	"the repository full name / slug"
func LookupRepo(c *gin.Context) {
	c.JSON(http.StatusOK, session.Repo(c))
}

// GetRepo
//
//	@Summary	Get a repository
//	@Router		/repos/{repo_id} [get]
//	@Produce	json
//	@Success	200	{object}	Repo
//	@Tags		Repositories
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
func GetRepo(c *gin.Context) {
	c.JSON(http.StatusOK, session.Repo(c))
}

// GetRepoPermissions
//
//	@Summary		Check current authenticated users access to the repository
//	@Description	The repository permission, according to the used access token.
//	@Router			/repos/{repo_id}/permissions [get]
//	@Produce		json
//	@Success		200	{object}	Perm
//	@Tags			Repositories
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			repo_id			path	int		true	"the repository id"
func GetRepoPermissions(c *gin.Context) {
	perm := session.Perm(c)
	c.JSON(http.StatusOK, perm)
}

// GetRepoBranches
//
//	@Summary	Get branches of a repository
//	@Router		/repos/{repo_id}/branches [get]
//	@Produce	json
//	@Success	200	{array}	string
//	@Tags		Repositories
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
//	@Param		page			query	int		false	"for response pagination, page offset number"	default(1)
//	@Param		perPage			query	int		false	"for response pagination, max items per page"	default(50)
func GetRepoBranches(c *gin.Context) {
	_store := store.FromContext(c)
	repo := session.Repo(c)
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get forge information")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Unable to get forge information. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	repoUser, err := _store.GetUser(repo.UserID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get user. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		handleDBError(c, err)
		return
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	var isSecondaryForge bool
	if userForge, ufErr := _store.UserForgeGet(repoUser.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		repoUser.AccessToken = userForge.AccessToken
		repoUser.RefreshToken = userForge.RefreshToken
		repoUser.Expiry = userForge.Expiry
		repoUser.ForgeID = userForge.ForgeID
		repoUser.ForgeRemoteID = userForge.ForgeRemoteID
		isSecondaryForge = !userForge.IsPrimary
	}

	var refreshErr error
	if isSecondaryForge {
		refreshErr = forge.RefreshUserForge(c, _forge, _store, repoUser, repo.ForgeID)
	} else {
		refreshErr = forge.Refresh(c, _forge, _store, repoUser)
	}
	if refreshErr != nil {
		log.Error().Err(refreshErr).Msg("Failed to refresh user auth token")
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusUnauthorized, "Your authorization has expired and couldn't be refreshed automatically. Please log in again")
		c.Abort()
		return
	}

	branches, err := _forge.Branches(c, repoUser, repo, session.Pagination(c))
	if err != nil {
		log.Error().Err(err).Msg("Failed to get branches from repo")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get branches from repo. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	c.JSON(http.StatusOK, branches)
}

// GetRepoPullRequests
//
//	@Summary	List active pull requests of a repository
//	@Router		/repos/{repo_id}/pull_requests [get]
//	@Produce	json
//	@Success	200	{array}	PullRequest
//	@Tags		Repositories
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
//	@Param		page			query	int		false	"for response pagination, page offset number"	default(1)
//	@Param		perPage			query	int		false	"for response pagination, max items per page"	default(50)
func GetRepoPullRequests(c *gin.Context) {
	_store := store.FromContext(c)
	repo := session.Repo(c)
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get repo information from forge")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Unable to get repo information from forge. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	repoUser, err := _store.GetUser(repo.UserID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get user. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		handleDBError(c, err)
		return
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	var isSecondaryForge2 bool
	if userForge, ufErr := _store.UserForgeGet(repoUser.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		repoUser.AccessToken = userForge.AccessToken
		repoUser.RefreshToken = userForge.RefreshToken
		repoUser.Expiry = userForge.Expiry
		repoUser.ForgeID = userForge.ForgeID
		repoUser.ForgeRemoteID = userForge.ForgeRemoteID
		isSecondaryForge2 = !userForge.IsPrimary
	}

	var refreshErr2 error
	if isSecondaryForge2 {
		refreshErr2 = forge.RefreshUserForge(c, _forge, _store, repoUser, repo.ForgeID)
	} else {
		refreshErr2 = forge.Refresh(c, _forge, _store, repoUser)
	}
	if refreshErr2 != nil {
		log.Error().Err(refreshErr2).Msg("Failed to refresh user auth token")
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusUnauthorized, "Your authorization has expired and couldn't be refreshed automatically. Please log in again")
		c.Abort()
		return
	}

	prs, err := _forge.PullRequests(c, repoUser, repo, session.Pagination(c))
	if err != nil {
		log.Error().Err(err).Msg("Unable to get pull requests from forge")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Unable to get pull requests from forge. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, prs)
}

// DeleteRepo
//
//	@Summary	Delete a repository
//	@Router		/repos/{repo_id} [delete]
//	@Produce	json
//	@Success	200	{object}	Repo
//	@Tags		Repositories
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
func DeleteRepo(c *gin.Context) {
	remove, _ := strconv.ParseBool(c.Query("remove"))
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get repo information from forge")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Unable to get repo information from forge. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// For multi-forge support: get the user's tokens for this repo's forge
	forgeUser := user
	isSecondaryForge := false
	if repo.ForgeID != user.ForgeID {
		userForge, err := _store.UserForgeGet(user.ID, repo.ForgeID)
		if err != nil {
			log.Warn().Err(err).Int64("forge_id", repo.ForgeID).Msg("User does not have tokens for repo's forge, will try to delete anyway")
		} else {
			// Create a user object with the correct forge tokens
			forgeUser = &model.User{
				ID:           user.ID,
				Login:        userForge.ForgeLogin,
				ForgeID:      repo.ForgeID,
				AccessToken:  userForge.AccessToken,
				RefreshToken: userForge.RefreshToken,
				Expiry:       userForge.Expiry,
			}
			isSecondaryForge = true
		}
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, forgeUser, repo.ForgeID, isSecondaryForge)
	if c.IsAborted() {
		return
	}

	if err := _forge.Deactivate(c, forgeUser, repo, server.Config.Server.WebhookHost); err != nil {
		log.Error().Err(err).Msg("Failed to deactivate repo")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to deactivate repo. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if remove {
		if err := _store.DeleteRepo(repo); err != nil {
			log.Error().Err(err).Msg("Failed to delete repo")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to delete repo. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
			})
			handleDBError(c, err)
			return
		}
	} else {
		repo.IsActive = false
		repo.UserID = 0

		if err := _store.UpdateRepo(repo); err != nil {
			log.Error().Err(err).Msg("Unable to update repo")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Unable to update repo. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
			})
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	c.JSON(http.StatusOK, repo)
}

// RepairRepo
//
//	@Summary	Repair a repository
//	@Router		/repos/{repo_id}/repair [post]
//	@Produce	plain
//	@Success	204
//	@Tags		Repositories
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
func RepairRepo(c *gin.Context) {
	repo := session.Repo(c)
	repairRepo(c, repo, true, false)
	if c.Writer.Written() {
		return
	}
	c.Status(http.StatusNoContent)
}

// MoveRepo
//
//	@Summary	Move a repository to a new owner
//	@Router		/repos/{repo_id}/move [post]
//	@Produce	plain
//	@Success	204
//	@Tags		Repositories
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		repo_id			path	int		true	"the repository id"
//	@Param		to				query	string	true	"the username to move the repository to"
func MoveRepo(c *gin.Context) {
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get repo information from forge")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Unable to get repo information from forge. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// For multi-forge support: get the user's tokens for this repo's forge
	forgeUser := user
	isSecondaryForge := false
	if repo.ForgeID != user.ForgeID {
		userForge, err := _store.UserForgeGet(user.ID, repo.ForgeID)
		if err != nil {
			log.Error().Err(err).Int64("forge_id", repo.ForgeID).Msg("User does not have tokens for repo's forge")
			c.JSON(http.StatusForbidden, gin.H{
				"error": "You don't have access to this repository's forge",
			})
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		forgeUser = &model.User{
			ID:           user.ID,
			Login:        userForge.ForgeLogin,
			ForgeID:      repo.ForgeID,
			AccessToken:  userForge.AccessToken,
			RefreshToken: userForge.RefreshToken,
			Expiry:       userForge.Expiry,
		}
		isSecondaryForge = true
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, forgeUser, repo.ForgeID, isSecondaryForge)
	if c.IsAborted() {
		return
	}

	to, exists := c.GetQuery("to")
	if !exists {
		err := fmt.Errorf("missing required 'to' value in query")
		log.Error().Err(err).Msg("Missing required 'to' value in query")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Missing required 'to' value in query. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	owner, name, errParse := model.ParseRepo(to)
	if errParse != nil {
		log.Error().Err(errParse).Msg("Error parsing repo")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Error parsing repo. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		_ = c.AbortWithError(http.StatusInternalServerError, errParse)
		return
	}

	from, err := _forge.Repo(c, forgeUser, "", owner, name)
	if err != nil {
		log.Error().Err(err).Msg("Error retrieving repo")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Error retrieving repo. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if !from.Perm.Admin {
		log.Error().Msg("Insufficient permissions to move repo")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Insufficient permissions to move repo. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	err = _store.CreateRedirection(&model.Redirection{ForgeID: repo.ForgeID, RepoID: repo.ID, FullName: repo.FullName})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create redirection")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to create redirection. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	repo.Update(from)
	errStore := _store.UpdateRepo(repo)
	if errStore != nil {
		log.Error().Err(errStore).Msg("Failed to update repo")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to update repo. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		_ = c.AbortWithError(http.StatusInternalServerError, errStore)
		return
	}

	repo.Perm = from.Perm
	errStore = _store.PermUpsert(repo.Perm)
	if errStore != nil {
		log.Error().Err(errStore).Msg("Failed to update permissions")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to update permissions. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		_ = c.AbortWithError(http.StatusInternalServerError, errStore)
		return
	}

	// creates the jwt token used to verify the repository
	t := token.New(token.HookToken)
	t.Set("repo-forge-remote-id", string(repo.ForgeRemoteID))
	t.Set("forge-id", strconv.FormatInt(repo.ForgeID, 10))
	sig, err := t.Sign(repo.Hash)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create JWT token")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to create JWT token. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to deactivate repo (during move operation). Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
	}
	if err := _forge.Activate(c, forgeUser, repo, hookURL); err != nil {
		log.Error().Err(err).Msg("Failed to activate repo (during move operation)")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to activate repo (during move operation). Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}
	c.Status(http.StatusNoContent)
}

// GetAllRepos
//
//	@Summary		List all repositories on the server
//	@Description	Returns a list of all repositories. Requires admin rights.
//	@Router			/repos [get]
//	@Produce		json
//	@Success		200	{array}	Repo
//	@Tags			Repositories
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			active			query	bool	false	"only list active repos"
//	@Param			page			query	int		false	"for response pagination, page offset number"	default(1)
//	@Param			perPage			query	int		false	"for response pagination, max items per page"	default(50)
func GetAllRepos(c *gin.Context) {
	_store := store.FromContext(c)

	active, _ := strconv.ParseBool(c.Query("active"))

	repos, err := _store.RepoListAll(active, session.Pagination(c))
	if err != nil {
		log.Error().Err(err).Msg("Error fetching repository list")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Error fetching repository list. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	c.JSON(http.StatusOK, repos)
}

// GetPublicRepos
//
//	@Summary		List public repositories
//	@Description	Returns a list of public repositories (no authentication required)
//	@Router			/repos/public [get]
//	@Produce		json
//	@Success		200	{array}	RepoLastPipeline
//	@Tags			Repositories
//	@Param			page	query	int	false	"for response pagination, page offset number"	default(1)
//	@Param			perPage	query	int	false	"for response pagination, max items per page"	default(50)
func GetPublicRepos(c *gin.Context) {
	_store := store.FromContext(c)

	repos, err := _store.RepoListPublic(session.Pagination(c))
	if err != nil {
		log.Error().Err(err).Msg("Error fetching public repository list")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Error fetching public repository list. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	// Fetch latest pipelines for each repo
	repoIDs := make([]int64, len(repos))
	for i, repo := range repos {
		repoIDs[i] = repo.ID
	}

	pipelines, err := _store.GetRepoLatestPipelines(repoIDs)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching latest pipelines for public repos")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Error fetching latest pipelines. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	latestPipelines := make(map[int64]*model.Pipeline, len(repos))
	for _, pipeline := range pipelines {
		latestPipelines[pipeline.RepoID] = pipeline
	}

	result := make([]*model.RepoLastPipeline, len(repos))
	for i, repo := range repos {
		result[i] = &model.RepoLastPipeline{
			Repo:         repo,
			LastPipeline: latestPipelines[repo.ID],
		}
	}

	c.JSON(http.StatusOK, result)
}

// RepairAllRepos
//
//	@Summary		Repair all repositories on the server
//	@Description	Executes a repair process on all repositories. Requires admin rights.
//	@Router			/repos/repair [post]
//	@Produce		plain
//	@Success		204
//	@Tags			Repositories
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
func RepairAllRepos(c *gin.Context) {
	_store := store.FromContext(c)

	repos, err := _store.RepoListAll(true, &model.ListOptions{All: true})
	if err != nil {
		log.Error().Err(err).Msg("Error fetching repository list")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Error fetching repository list. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	for _, r := range repos {
		repairRepo(c, r, false, true)
		if c.Writer.Written() {
			return
		}
	}

	c.Status(http.StatusNoContent)
}

func repairRepo(c *gin.Context, repo *model.Repo, withPerms, skipOnErr bool) {
	_store := store.FromContext(c)
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get repo information from forge")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Unable to get repo information from forge. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		c.AbortWithStatus(http.StatusInternalServerError)
		return
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
				_ = c.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Could not find repo user with ID %d during repo repair. Setting to repair request user with ID %d. Endpoint: %s %s", oldUserID, user.ID, c.Request.Method, c.Request.URL.Path),
			})
			log.Debug().Msgf("Could not find repo user with ID %d during repo repair. Setting to repair request user with ID %d", oldUserID, user.ID)
		} else {
			log.Error().Err(err).Msg("Failed to get user")
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	isSecondaryForge := false
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
		isSecondaryForge = !userForge.IsPrimary
	}

	// Refresh tokens before forge API call
	refreshUserTokenForForge(c, user, repo.ForgeID, isSecondaryForge)
	if c.IsAborted() {
		return
	}

	// creates the jwt token used to verify the repository
	t := token.New(token.HookToken)
	t.Set("repo-forge-remote-id", string(repo.ForgeRemoteID))
	t.Set("forge-id", strconv.FormatInt(repo.ForgeID, 10))
	sig, err := t.Sign(repo.Hash)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create JWT token")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to create JWT token. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
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
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Unable to get repo information from forge. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
			})
			c.AbortWithStatus(http.StatusInternalServerError)
		}
		return
	}

	if repo.FullName != from.FullName {
		// create a redirection
		err = _store.CreateRedirection(&model.Redirection{RepoID: repo.ID, FullName: repo.FullName})
		if err != nil {
			log.Error().Err(err).Msg("Unable to create redirection")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Unable to create redirection. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
			})
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	repo.Update(from)
	if err := _store.UpdateRepo(repo); err != nil {
		log.Error().Err(err).Msg("Failed to update repo")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to update repo. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if withPerms {
		repo.Perm.Pull = from.Perm.Pull
		repo.Perm.Push = from.Perm.Push
		repo.Perm.Admin = from.Perm.Admin
		if err := _store.PermUpsert(repo.Perm); err != nil {
			log.Error().Err(err).Msg("Unable to update permissions")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Unable to update permissions. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
			})
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	if err := _forge.Deactivate(c, user, repo, host); err != nil {
		log.Error().Err(err).Msgf("Failed to deactivate repo '%s' to repair", repo.FullName)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to deactivate repo. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
	}
	if err := _forge.Activate(c, user, repo, hookURL); err != nil {
		log.Error().Err(err).Msg("Failed to activate repo")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to activate repo. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}
}
