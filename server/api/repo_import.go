// Copyright 2025 Crow Authors
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
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v5/server"
	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/router/middleware/session"
	"codeberg.org/crowci/crow/v5/server/store"
)

// ImportSettingsRequest represents the request body for importing settings.
type ImportSettingsRequest struct {
	SourceRepoID int64 `json:"source_repo_id" binding:"required"`
	Settings     bool  `json:"settings"`
	Secrets      bool  `json:"secrets"`
	Registries   bool  `json:"registries"`
	Crons        bool  `json:"crons"`
	Pipelines    bool  `json:"pipelines"`
	DeleteSource bool  `json:"delete_source"`
}

// ImportSettingsResponse represents the response after importing settings.
type ImportSettingsResponse struct {
	Message            string `json:"message"`
	SettingsImported   bool   `json:"settings_imported"`
	SecretsImported    int    `json:"secrets_imported"`
	RegistriesImported int    `json:"registries_imported"`
	CronsImported      int    `json:"crons_imported"`
	PipelinesImported  int    `json:"pipelines_imported"`
	SourceDeleted      bool   `json:"source_deleted"`
}

// ImportSettings imports settings from another repository.
//
//	@Summary		Import settings from another repository
//	@Description	Imports settings, secrets, registries, and/or crons from a source repository. User must be admin of both repositories.
//	@Router			/repos/{repo_id}/import-settings [post]
//	@Produce		json
//	@Success		200	{object}	ImportSettingsResponse
//	@Tags			Repositories
//	@Param			Authorization	header	string					true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			repo_id			path	int						true	"the target repository id"
//	@Param			request			body	ImportSettingsRequest	true	"Import options"
func ImportSettings(c *gin.Context) {
	_store := store.FromContext(c)
	user := session.User(c)
	targetRepo := session.Repo(c)

	var req ImportSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "Invalid request body: %s", err.Error())
		return
	}

	// Get source repository
	sourceRepo, err := _store.GetRepo(req.SourceRepoID)
	if err != nil {
		c.String(http.StatusNotFound, "Source repository not found")
		return
	}

	// Check user has admin access to source repo
	sourcePerm, err := _store.PermFind(user, sourceRepo)
	if err != nil || !sourcePerm.Admin {
		c.String(http.StatusForbidden, "You need admin access to the source repository")
		return
	}

	// Check user has admin access to target repo (already done by middleware, but double-check)
	targetPerm, err := _store.PermFind(user, targetRepo)
	if err != nil || !targetPerm.Admin {
		c.String(http.StatusForbidden, "You need admin access to the target repository")
		return
	}

	// Prevent importing from same repo
	if sourceRepo.ID == targetRepo.ID {
		c.String(http.StatusBadRequest, "Cannot import settings from the same repository")
		return
	}

	response := ImportSettingsResponse{}

	// Import settings
	if req.Settings {
		targetRepo.Timeout = sourceRepo.Timeout
		targetRepo.Visibility = sourceRepo.Visibility
		targetRepo.Trusted = sourceRepo.Trusted
		targetRepo.RequireApproval = sourceRepo.RequireApproval
		targetRepo.AllowPull = sourceRepo.AllowPull
		targetRepo.AllowDeploy = sourceRepo.AllowDeploy
		targetRepo.DeployTeam = sourceRepo.DeployTeam
		targetRepo.Config = sourceRepo.Config
		targetRepo.CancelPreviousPipelineEvents = sourceRepo.CancelPreviousPipelineEvents
		targetRepo.NetrcTrustedPlugins = sourceRepo.NetrcTrustedPlugins

		if err := _store.UpdateRepo(targetRepo); err != nil {
			log.Error().Err(err).Msg("Failed to update target repo settings")
			c.String(http.StatusInternalServerError, "Failed to import settings")
			return
		}
		response.SettingsImported = true
	}

	// Import secrets
	if req.Secrets {
		secrets, err := _store.SecretList(sourceRepo, false, &model.ListOptions{All: true})
		if err != nil {
			log.Error().Err(err).Msg("Failed to list source secrets")
			c.String(http.StatusInternalServerError, "Failed to list source secrets")
			return
		}

		for _, secret := range secrets {
			// Check if secret already exists in target
			existing, _ := _store.SecretFind(targetRepo, secret.Name)
			if existing != nil {
				// Update existing secret
				existing.Value = secret.Value
				existing.Images = secret.Images
				existing.Events = secret.Events
				if err := _store.SecretUpdate(existing); err != nil {
					log.Error().Err(err).Str("secret", secret.Name).Msg("Failed to update secret")
					continue
				}
			} else {
				// Create new secret
				newSecret := &model.Secret{
					RepoID: targetRepo.ID,
					Name:   secret.Name,
					Value:  secret.Value,
					Images: secret.Images,
					Events: secret.Events,
				}
				if err := _store.SecretCreate(newSecret); err != nil {
					log.Error().Err(err).Str("secret", secret.Name).Msg("Failed to create secret")
					continue
				}
			}
			response.SecretsImported++
		}
	}

	// Import registries
	if req.Registries {
		registries, err := _store.RegistryList(sourceRepo, false, &model.ListOptions{All: true})
		if err != nil {
			log.Error().Err(err).Msg("Failed to list source registries")
			c.String(http.StatusInternalServerError, "Failed to list source registries")
			return
		}

		for _, registry := range registries {
			// Check if registry already exists in target
			existing, _ := _store.RegistryFind(targetRepo, registry.Address)
			if existing != nil {
				// Update existing registry
				existing.Username = registry.Username
				existing.Password = registry.Password
				if err := _store.RegistryUpdate(existing); err != nil {
					log.Error().Err(err).Str("registry", registry.Address).Msg("Failed to update registry")
					continue
				}
			} else {
				// Create new registry
				newRegistry := &model.Registry{
					RepoID:   targetRepo.ID,
					Address:  registry.Address,
					Username: registry.Username,
					Password: registry.Password,
				}
				if err := _store.RegistryCreate(newRegistry); err != nil {
					log.Error().Err(err).Str("registry", registry.Address).Msg("Failed to create registry")
					continue
				}
			}
			response.RegistriesImported++
		}
	}

	// Import crons
	if req.Crons {
		sourceCrons, err := _store.CronList(sourceRepo, &model.ListOptions{All: true})
		if err != nil {
			log.Error().Err(err).Msg("Failed to list source crons")
			c.String(http.StatusInternalServerError, "Failed to list source crons")
			return
		}

		// Get existing crons in target to check for duplicates by name
		targetCrons, _ := _store.CronList(targetRepo, &model.ListOptions{All: true})
		targetCronsByName := make(map[string]*model.Cron)
		for _, tc := range targetCrons {
			targetCronsByName[tc.Name] = tc
		}

		for _, cron := range sourceCrons {
			// Check if cron already exists in target (by name)
			if existing, found := targetCronsByName[cron.Name]; found {
				// Update existing cron
				existing.Schedule = cron.Schedule
				existing.Branch = cron.Branch
				if err := _store.CronUpdate(targetRepo, existing); err != nil {
					log.Error().Err(err).Str("cron", cron.Name).Msg("Failed to update cron")
					continue
				}
			} else {
				// Create new cron
				newCron := &model.Cron{
					RepoID:    targetRepo.ID,
					CreatorID: user.ID,
					Name:      cron.Name,
					Schedule:  cron.Schedule,
					Branch:    cron.Branch,
				}
				if err := _store.CronCreate(newCron); err != nil {
					log.Error().Err(err).Str("cron", cron.Name).Msg("Failed to create cron")
					continue
				}
			}
			response.CronsImported++
		}
	}

	// Import pipelines
	if req.Pipelines {
		imported, err := importPipelines(_store, sourceRepo, targetRepo)
		if err != nil {
			log.Error().Err(err).Msg("Failed to import pipelines")
			c.String(http.StatusInternalServerError, "Failed to import pipelines: %s", err.Error())
			return
		}
		response.PipelinesImported = imported
	}

	// Delete source repo if requested
	if req.DeleteSource {
		if err := _store.DeleteRepo(sourceRepo); err != nil {
			log.Error().Err(err).Msg("Failed to delete source repo")
			// Don't fail the whole operation, just note that deletion failed
		} else {
			response.SourceDeleted = true
		}
	}

	response.Message = "Import completed successfully"
	c.JSON(http.StatusOK, response)
}

// GetImportableRepos returns a list of repos the user can import settings from.
//
//	@Summary		Get repos that can be imported from
//	@Description	Returns all repos where the user is admin and which are different from the target repo.
//	@Router			/repos/{repo_id}/importable [get]
//	@Produce		json
//	@Success		200	{array}	Repo
//	@Tags			Repositories
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			repo_id			path	int		true	"the target repository id"
func GetImportableRepos(c *gin.Context) {
	_store := store.FromContext(c)
	user := session.User(c)
	targetRepo := session.Repo(c)

	// Get all repos where user has admin access
	repos, err := _store.RepoList(user, true, true) // owned=true (push or admin), active=true
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to list repositories")
		return
	}

	// Filter to only repos where user is admin and exclude target repo
	importableRepos := make([]*model.Repo, 0)
	for _, repo := range repos {
		if repo.ID == targetRepo.ID {
			continue
		}

		// Check admin permission
		perm, err := _store.PermFind(user, repo)
		if err != nil || !perm.Admin {
			continue
		}

		importableRepos = append(importableRepos, repo)
	}

	c.JSON(http.StatusOK, importableRepos)
}

// GetRepoForImport returns details of a specific repo for import preview.
//
//	@Summary		Get repo details for import preview
//	@Description	Returns details of a source repo including counts of secrets, registries, and crons.
//	@Router			/repos/{repo_id}/import-preview/{source_repo_id} [get]
//	@Produce		json
//	@Success		200	{object}	ImportPreviewResponse
//	@Tags			Repositories
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			repo_id			path	int		true	"the target repository id"
//	@Param			source_repo_id	path	int		true	"the source repository id"
func GetRepoForImport(c *gin.Context) {
	_store := store.FromContext(c)
	user := session.User(c)
	targetRepo := session.Repo(c)

	sourceRepoID, err := strconv.ParseInt(c.Param("source_repo_id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid source repo ID")
		return
	}

	sourceRepo, err := _store.GetRepo(sourceRepoID)
	if err != nil {
		c.String(http.StatusNotFound, "Source repository not found")
		return
	}

	// Check user has admin access
	perm, err := _store.PermFind(user, sourceRepo)
	if err != nil || !perm.Admin {
		c.String(http.StatusForbidden, "You need admin access to the source repository")
		return
	}

	// Count repo-level secrets
	secrets, _ := _store.SecretList(sourceRepo, false, &model.ListOptions{All: true})
	secretCount := len(secrets)

	// Count repo-level registries
	registries, _ := _store.RegistryList(sourceRepo, false, &model.ListOptions{All: true})
	registryCount := len(registries)

	// Count crons
	crons, _ := _store.CronList(sourceRepo, &model.ListOptions{All: true})
	cronCount := len(crons)

	// Count pipelines
	pipelineCount, _ := _store.GetPipelineCountByRepo(sourceRepo.ID)

	// Calculate org-level secrets that would be migrated from same-named orgs on other forges
	var orgSecretCount, orgRegistryCount int
	var orgName, orgSourceForge string

	// Get target repo's org
	targetOrg, err := _store.OrgGet(targetRepo.OrgID)
	if err == nil && targetOrg != nil {
		// Find orgs with same name on other forges
		allOrgs, err := _store.OrgFindByNameAllForges(targetOrg.Name)
		if err == nil {
			for _, org := range allOrgs {
				// Skip the target org itself
				if org.ID == targetOrg.ID {
					continue
				}

				// Count org secrets
				orgSecrets, _ := _store.OrgSecretList(org.ID, &model.ListOptions{All: true})
				// Count org registries
				orgRegistries, _ := _store.OrgRegistryList(org.ID, &model.ListOptions{All: true})

				if len(orgSecrets) > 0 || len(orgRegistries) > 0 {
					orgSecretCount = len(orgSecrets)
					orgRegistryCount = len(orgRegistries)
					orgName = org.Name
					// Get forge URL for display (extract hostname)
					if forge, err := server.Config.Services.Manager.ForgeByID(org.ForgeID); err == nil {
						forgeURL := forge.URL()
						if parsed, err := url.Parse(forgeURL); err == nil {
							orgSourceForge = parsed.Host
						} else {
							// Fallback: strip protocol manually
							orgSourceForge = strings.TrimPrefix(strings.TrimPrefix(forgeURL, "https://"), "http://")
						}
					}
					break // Use first org with data
				}
			}
		}
	}

	c.JSON(http.StatusOK, ImportPreviewResponse{
		Repo:             sourceRepo,
		SecretCount:      secretCount,
		RegistryCount:    registryCount,
		CronCount:        cronCount,
		PipelineCount:    int(pipelineCount),
		OrgSecretCount:   orgSecretCount,
		OrgRegistryCount: orgRegistryCount,
		OrgName:          orgName,
		OrgSourceForge:   orgSourceForge,
	})
}

// ImportPreviewResponse contains repo info and counts for import preview.
type ImportPreviewResponse struct {
	Repo          *model.Repo `json:"repo"`
	SecretCount   int         `json:"secret_count"`
	RegistryCount int         `json:"registry_count"`
	CronCount     int         `json:"cron_count"`
	PipelineCount int         `json:"pipeline_count"`
	// Org-level assets that would be migrated from same-named orgs on other forges
	OrgSecretCount   int    `json:"org_secret_count"`
	OrgRegistryCount int    `json:"org_registry_count"`
	OrgName          string `json:"org_name,omitempty"`
	OrgSourceForge   string `json:"org_source_forge,omitempty"`
}

// userForgeMapping holds the target forge user info for mapping authors.
type userForgeMapping struct {
	Login  string
	Email  string
	Avatar string
}

// buildUserForgeMapping builds a mapping of source forge logins to target forge user info
// for users who have linked both forges.
func buildUserForgeMapping(_store store.Store, sourceForgeID, targetForgeID int64) map[string]*userForgeMapping {
	mapping := make(map[string]*userForgeMapping)

	// Get all user forge connections
	allUserForges, err := _store.UserForgeListAll()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get user forges for mapping")
		return mapping
	}

	// Group by user ID
	userForgesByUser := make(map[int64][]*model.UserForge)
	for _, uf := range allUserForges {
		userForgesByUser[uf.UserID] = append(userForgesByUser[uf.UserID], uf)
	}

	// Find users with both source and target forge links
	for _, userForges := range userForgesByUser {
		var sourceUF, targetUF *model.UserForge
		for _, uf := range userForges {
			if uf.ForgeID == sourceForgeID {
				sourceUF = uf
			}
			if uf.ForgeID == targetForgeID {
				targetUF = uf
			}
		}

		// If user has both forges linked, add to mapping
		if sourceUF != nil && targetUF != nil {
			mapping[sourceUF.ForgeLogin] = &userForgeMapping{
				Login:  targetUF.ForgeLogin,
				Email:  targetUF.ForgeEmail,
				Avatar: targetUF.ForgeAvatar,
			}
		}
	}

	return mapping
}

// importPipelines imports all pipelines from sourceRepo to targetRepo,
// adapting forge URLs to point to the target forge.
func importPipelines(_store store.Store, sourceRepo, targetRepo *model.Repo) (int, error) {
	// Get all pipelines from source repo
	pipelines, err := _store.GetPipelineList(sourceRepo, &model.ListOptions{All: true}, nil)
	if err != nil {
		return 0, err
	}

	if len(pipelines) == 0 {
		return 0, nil
	}

	// Build user mapping for authors who have linked both forges
	userMapping := buildUserForgeMapping(_store, sourceRepo.ForgeID, targetRepo.ForgeID)

	// Get the current max pipeline number in target repo
	var maxNumber int64
	targetPipelines, err := _store.GetPipelineList(targetRepo, &model.ListOptions{Page: 1, PerPage: 1}, nil)
	if err == nil && len(targetPipelines) > 0 {
		maxNumber = targetPipelines[0].Number
	}

	imported := 0
	for _, srcPipeline := range pipelines {
		maxNumber++

		// Map author to target forge if they have linked both forges
		author := srcPipeline.Author
		sender := srcPipeline.Sender
		email := srcPipeline.Email
		avatar := adaptForgeURL(srcPipeline.Avatar, sourceRepo.ForgeURL, targetRepo.ForgeURL)

		if targetUser, found := userMapping[srcPipeline.Author]; found {
			author = targetUser.Login
			if srcPipeline.Sender == srcPipeline.Author {
				sender = targetUser.Login
			}
			if targetUser.Email != "" {
				email = targetUser.Email
			}
			if targetUser.Avatar != "" {
				avatar = targetUser.Avatar
			}
		}

		// Create new pipeline with adapted URLs and mapped user info
		newPipeline := &model.Pipeline{
			RepoID:              targetRepo.ID,
			Number:              maxNumber,
			Author:              author,
			Parent:              srcPipeline.Parent,
			Event:               srcPipeline.Event,
			Status:              srcPipeline.Status,
			Errors:              srcPipeline.Errors,
			Created:             srcPipeline.Created,
			Updated:             srcPipeline.Updated,
			Started:             srcPipeline.Started,
			Finished:            srcPipeline.Finished,
			DeployTo:            srcPipeline.DeployTo,
			DeployTask:          srcPipeline.DeployTask,
			Commit:              srcPipeline.Commit,
			Branch:              srcPipeline.Branch,
			Ref:                 srcPipeline.Ref,
			Refspec:             srcPipeline.Refspec,
			Title:               srcPipeline.Title,
			Message:             srcPipeline.Message,
			Timestamp:           srcPipeline.Timestamp,
			Sender:              sender,
			Avatar:              avatar,
			Email:               email,
			ForgeURL:            adaptForgeURL(srcPipeline.ForgeURL, sourceRepo.ForgeURL, targetRepo.ForgeURL),
			Reviewer:            srcPipeline.Reviewer,
			Reviewed:            srcPipeline.Reviewed,
			ChangedFiles:        srcPipeline.ChangedFiles,
			AdditionalVariables: srcPipeline.AdditionalVariables,
			PullRequestLabels:   srcPipeline.PullRequestLabels,
			IsPrerelease:        srcPipeline.IsPrerelease,
			FromFork:            srcPipeline.FromFork,
			LogsDeleted:         srcPipeline.LogsDeleted,
		}

		// Get source workflows and steps
		srcWorkflows, err := _store.WorkflowGetTree(srcPipeline)
		if err != nil {
			log.Error().Err(err).Int64("pipeline", srcPipeline.ID).Msg("Failed to get workflows for pipeline")
			continue
		}

		// Collect all steps for the CreatePipeline call, creating new step objects
		// to avoid ID conflicts (source steps have their original IDs set)
		var allSteps []*model.Step
		for _, wf := range srcWorkflows {
			for _, step := range wf.Children {
				newStep := &model.Step{
					// ID is intentionally 0 for auto-increment
					UUID:       "", // Empty for imported steps, will be set if needed
					PipelineID: 0,  // Will be set by CreatePipeline
					PID:        step.PID,
					PPID:       step.PPID,
					Name:       step.Name,
					State:      step.State,
					Error:      step.Error,
					Failure:    step.Failure,
					ExitCode:   step.ExitCode,
					Started:    step.Started,
					Finished:   step.Finished,
					Type:       step.Type,
					DependsOn:  step.DependsOn,
				}
				allSteps = append(allSteps, newStep)
			}
		}

		// Create the pipeline with steps
		if err := _store.CreatePipeline(newPipeline, allSteps...); err != nil {
			log.Error().Err(err).Int64("pipeline", srcPipeline.ID).Msg("Failed to create pipeline")
			continue
		}

		// Create workflows with updated pipeline ID
		var newWorkflows []*model.Workflow
		for _, srcWf := range srcWorkflows {
			newWf := &model.Workflow{
				PipelineID:        newPipeline.ID,
				PID:               srcWf.PID,
				Name:              srcWf.Name,
				PipelineName:      srcWf.PipelineName,
				State:             srcWf.State,
				Error:             srcWf.Error,
				Started:           srcWf.Started,
				Finished:          srcWf.Finished,
				AgentID:           srcWf.AgentID,
				AgentPersistentID: srcWf.AgentPersistentID,
				AgentName:         srcWf.AgentName,
				Platform:          srcWf.Platform,
				Environ:           srcWf.Environ,
				DependsOn:         srcWf.DependsOn,
				AxisID:            srcWf.AxisID,
			}
			newWorkflows = append(newWorkflows, newWf)
		}

		if len(newWorkflows) > 0 {
			if err := _store.WorkflowsCreate(newWorkflows); err != nil {
				log.Error().Err(err).Int64("pipeline", newPipeline.ID).Msg("Failed to create workflows")
			}
		}

		// Copy configs
		srcConfigs, err := _store.ConfigsForPipeline(srcPipeline.ID)
		if err != nil {
			log.Error().Err(err).Int64("pipeline", srcPipeline.ID).Msg("Failed to get configs for pipeline")
		} else {
			for _, srcConfig := range srcConfigs {
				// Create or reuse config in target repo
				newConfig := &model.Config{
					RepoID: targetRepo.ID,
					Hash:   srcConfig.Hash,
					Name:   srcConfig.Name,
					Data:   srcConfig.Data,
				}
				persistedConfig, err := _store.ConfigPersist(newConfig)
				if err != nil {
					log.Error().Err(err).Str("config", srcConfig.Name).Msg("Failed to persist config")
					continue
				}

				// Link config to pipeline
				if err := _store.PipelineConfigCreate(&model.PipelineConfig{
					ConfigID:   persistedConfig.ID,
					PipelineID: newPipeline.ID,
				}); err != nil {
					log.Error().Err(err).Str("config", srcConfig.Name).Msg("Failed to link config to pipeline")
				}
			}
		}

		// Copy logs for each step
		srcSteps, err := _store.StepList(srcPipeline)
		if err != nil {
			log.Error().Err(err).Int64("pipeline", srcPipeline.ID).Msg("Failed to get steps for pipeline")
		} else {
			// Get the new steps to map old step IDs to new ones
			newSteps, _ := _store.StepList(newPipeline)
			stepIDMap := make(map[int]int64) // PID -> new step ID
			for _, newStep := range newSteps {
				stepIDMap[newStep.PID] = newStep.ID
			}

			for _, srcStep := range srcSteps {
				newStepID, ok := stepIDMap[srcStep.PID]
				if !ok {
					continue
				}

				logs, err := _store.LogFind(srcStep)
				if err != nil {
					continue
				}

				if len(logs) > 0 {
					// Update step IDs in log entries
					for _, logEntry := range logs {
						logEntry.ID = 0 // Reset ID for new insert
						logEntry.StepID = newStepID
					}

					newStep := &model.Step{ID: newStepID}
					if err := _store.LogAppend(newStep, logs); err != nil {
						log.Error().Err(err).Int64("step", srcStep.ID).Msg("Failed to copy logs")
					}
				}
			}
		}

		imported++
	}

	return imported, nil
}

// adaptForgeURL adapts a URL from the source forge to the target forge.
// For example, if sourceURL is "https://old.forge/user/repo/commit/abc"
// and we're migrating from "https://old.forge/user/repo" to "https://new.forge/user/repo",
// the result would be "https://new.forge/user/repo/commit/abc".
func adaptForgeURL(sourceURL, sourceRepoURL, targetRepoURL string) string {
	if sourceURL == "" || sourceRepoURL == "" || targetRepoURL == "" {
		return sourceURL
	}

	// Parse the source repo URL to get the base
	srcParsed, err := url.Parse(sourceRepoURL)
	if err != nil {
		return sourceURL
	}
	srcBase := srcParsed.Scheme + "://" + srcParsed.Host

	// Parse the target repo URL to get the base
	tgtParsed, err := url.Parse(targetRepoURL)
	if err != nil {
		return sourceURL
	}
	tgtBase := tgtParsed.Scheme + "://" + tgtParsed.Host

	// If the source URL starts with the source base, replace it with the target base
	if strings.HasPrefix(sourceURL, srcBase) {
		return tgtBase + strings.TrimPrefix(sourceURL, srcBase)
	}

	return sourceURL
}
