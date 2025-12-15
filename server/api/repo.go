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

// PostRepo
//
//	@Summary	Activate a repository
//	@Router		/repos [post]
//	@Produce	json
//	@Success	200	{object}	Repo
//	@Tags		Repositories
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		forge_remote_id	query	string	true	"the id of a repository at the forge"
func PostRepo(c *gin.Context) {
	_store := store.FromContext(c)
	user := session.User(c)
	_forge, err := server.Config.Services.Manager.ForgeFromUser(user)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get forge info from user information")
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf("Unable to get forge info from user information. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
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

	from, err := _forge.Repo(c, user, forgeRemoteID, "", "")
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
		repo.ForgeID = user.ForgeID // TODO: allow to use other connected forges of the user
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
	org, err = _store.OrgFindByName(repo.Owner, user.ForgeID)
	if err != nil && !errors.Is(err, types.RecordNotExist) {
		log.Error().Err(err).Msg("Unable to find organization")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Unable to find organization. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
		return
	}

	// create an org if it doesn't exist yet
	if errors.Is(err, types.RecordNotExist) {
		org, err = _forge.Org(c, user, repo.Owner)
		if err != nil {
			msg := fmt.Sprintf("Organization %s not found in DB. Attempting to create a new one.", repo.Owner)
			log.Error().Err(err).Msg(msg)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Organization %s not found in DB. Attempting to create a new one. Endpoint: %s %s", repo.Owner, c.Request.Method, c.Request.URL.Path),
			})
			return
		}

		org.ForgeID = user.ForgeID
		err = _store.OrgCreate(org)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create organization")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to create organization. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
			})
			return
		}
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
		"%s/api/hook?access_token=%s",
		server.Config.Server.WebhookHost,
		sig,
	)

	err = _forge.Activate(c, user, repo, hookURL)
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

	c.JSON(http.StatusOK, repo)
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
		_ = c.AbortWithError(http.StatusBadRequest, err)
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
		if (*in.Trusted.Network != repo.Trusted.Network || *in.Trusted.Volumes != repo.Trusted.Volumes || *in.Trusted.Security != repo.Trusted.Security) && !user.Admin {
			log.Trace().Msgf("user '%s' wants to change trusted without being an instance admin", user.Login)
			c.JSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("Insufficient privileges to change security settings. Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
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

	refreshErr := forge.Refresh(c, _forge, _store, repoUser)
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

	refreshErr := forge.Refresh(c, _forge, _store, repoUser)
	if refreshErr != nil {
		log.Error().Err(refreshErr).Msg("Failed to refresh user auth token")
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

	if err := _forge.Deactivate(c, user, repo, server.Config.Server.WebhookHost); err != nil {
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

	from, err := _forge.Repo(c, user, "", owner, name)
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

	err = _store.CreateRedirection(&model.Redirection{RepoID: repo.ID, FullName: repo.FullName})
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
		"%s/api/hook?access_token=%s",
		host,
		sig,
	)

	if err := _forge.Deactivate(c, user, repo, host); err != nil {
		log.Error().Err(err).Msg("Failed to deactivate repo (during move operation)")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to deactivate repo (during move operation). Endpoint: %s %s", c.Request.Method, c.Request.URL.Path),
		})
	}
	if err := _forge.Activate(c, user, repo, hookURL); err != nil {
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
		"%s/api/hook?access_token=%s",
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
