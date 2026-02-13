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
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/tink-crypto/tink-go/v2/subtle/random"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/store"
	"codefloe.com/crowci/crow/v5/shared/token"
)

// GetSelf
//
//	@Summary	Get the currently authenticated user
//	@Router		/user [get]
//	@Produce	json
//	@Success	200	{object}	User
//	@Tags		User
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
func GetSelf(c *gin.Context) {
	c.JSON(http.StatusOK, session.User(c))
}

// GetFeed
//
//	@Summary		Get the currently authenticated users pipeline feed
//	@Description	The feed lists the most recent pipeline for the currently authenticated user.
//	@Router			/user/feed [get]
//	@Produce		json
//	@Success		200	{array}	Feed
//	@Tags			User
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
func GetFeed(c *gin.Context) {
	_store := store.FromContext(c)

	user := session.User(c)
	latest, _ := strconv.ParseBool(c.Query("latest"))

	if latest {
		feed, err := _store.RepoListLatest(user)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error fetching feed. %s", err)
		} else {
			c.JSON(http.StatusOK, feed)
		}
		return
	}

	feed, err := _store.UserFeed(user)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error fetching user feed. %s", err)
		return
	}
	c.JSON(http.StatusOK, feed)
}

// GetRepos
//
//	@Summary		Get user's repositories
//	@Description	Retrieve the currently authenticated User's Repository list from all linked forges
//	@Router			/user/repos [get]
//	@Produce		json
//	@Success		200	{array}	RepoLastPipeline
//	@Tags			User
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			all				query	bool	false	"query all repos, including inactive ones"
func GetRepos(c *gin.Context) {
	_store := store.FromContext(c)
	user := session.User(c)

	all, _ := strconv.ParseBool(c.Query("all"))

	if all {
		// Get all repos from all linked forges
		repos, err := getAllReposFromAllForges(c, _store, user)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error fetching repository list. %s", err)
			return
		}
		c.JSON(http.StatusOK, repos)
		return
	}

	activeRepos, err := _store.RepoList(user, true, true)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error fetching repository list. %s", err)
		return
	}

	repoIDs := make([]int64, len(activeRepos))
	for i, repo := range activeRepos {
		repoIDs[i] = repo.ID
	}

	pipelines, err := _store.GetRepoLatestPipelines(repoIDs)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error fetching repository list. %s", err)
		return
	}

	latestPipelines := make(map[int64]*model.Pipeline, len(activeRepos))
	for _, pipeline := range pipelines {
		latestPipelines[pipeline.RepoID] = pipeline
	}

	repos := make([]*model.RepoLastPipeline, len(activeRepos))
	for i, repo := range activeRepos {
		repos[i] = &model.RepoLastPipeline{
			Repo:         repo,
			LastPipeline: latestPipelines[repo.ID],
		}
	}

	c.JSON(http.StatusOK, repos)
}

// getAllReposFromAllForges fetches repos from all forges the user has linked.
func getAllReposFromAllForges(c *gin.Context, _store store.Store, user *model.User) ([]*model.Repo, error) {
	// Get all forge connections for this user
	userForges, err := _store.UserForgeList(user.ID)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get user forges")
		return nil, err
	}

	log.Debug().Int("count", len(userForges)).Int64("user_id", user.ID).Msg("Found user forges")

	// Get all repos from database that user has access to
	dbRepos, err := _store.RepoList(user, true, false)
	if err != nil {
		return nil, err
	}

	// Create a map of active repos by forge_id + forge_remote_id for quick lookup
	type repoKey struct {
		forgeID       int64
		forgeRemoteID model.ForgeRemoteID
	}
	active := make(map[repoKey]*model.Repo)
	for _, r := range dbRepos {
		key := repoKey{forgeID: r.ForgeID, forgeRemoteID: r.ForgeRemoteID}
		active[key] = r
	}

	var allRepos []*model.Repo

	// Query repos from each linked forge
	for _, uf := range userForges {
		log.Debug().
			Int64("forge_id", uf.ForgeID).
			Str("forge_login", uf.ForgeLogin).
			Bool("has_token", uf.AccessToken != "").
			Msg("Processing user forge")

		_forge, err := server.Config.Services.Manager.ForgeByID(uf.ForgeID)
		if err != nil {
			log.Error().Err(err).Int64("forge_id", uf.ForgeID).Msg("Cannot get forge")
			continue // Skip this forge but continue with others
		}

		// Create a temporary user with this forge's tokens
		forgeUser := &model.User{
			ID:            user.ID,
			Login:         uf.ForgeLogin,
			Email:         uf.ForgeEmail,
			ForgeID:       uf.ForgeID,
			ForgeRemoteID: uf.ForgeRemoteID,
			AccessToken:   uf.AccessToken,
			RefreshToken:  uf.RefreshToken,
			Expiry:        uf.Expiry,
		}

		forgeRepos, err := _forge.Repos(c, forgeUser)
		if err != nil {
			log.Error().Err(err).Int64("forge_id", uf.ForgeID).Msg("Cannot get repos from forge")
			continue // Skip this forge but continue with others
		}

		log.Debug().Int64("forge_id", uf.ForgeID).Int("repo_count", len(forgeRepos)).Msg("Got repos from forge")

		for _, r := range forgeRepos {
			if r.Perm.Push && server.Config.Permissions.OwnersAllowlist.IsAllowed(r) {
				key := repoKey{forgeID: uf.ForgeID, forgeRemoteID: r.ForgeRemoteID}
				if existingRepo := active[key]; existingRepo != nil {
					existingRepo.Update(r)
					// IsActive is already set from the database
					allRepos = append(allRepos, existingRepo)
				} else if r.Perm.Admin {
					// you must be admin to enable the repo
					// Set ForgeID for repos from the forge API (not yet in database)
					r.ForgeID = uf.ForgeID
					allRepos = append(allRepos, r)
				}
			}
		}
	}

	return allRepos, nil
}

// PostToken
//
//	@Summary		Return the token of the current user as string
//	@Description	Deprecated: Use POST /user/access-tokens instead for better security and scope control.
//	@Router			/user/token [post]
//	@Produce		plain
//	@Success		200
//	@Tags			User
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Deprecated
func PostToken(c *gin.Context) {
	user := session.User(c)
	t := token.New(token.UserToken)
	t.Set("user-id", strconv.FormatInt(user.ID, 10))
	tokenString, err := t.Sign(user.Hash)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.String(http.StatusOK, tokenString)
}

// DeleteToken
//
//	@Summary		Reset a token
//	@Description	Deprecated: Use DELETE /user/access-tokens/:token_id instead. This endpoint rotates the user hash which invalidates ALL tokens.
//	@Router			/user/token [delete]
//	@Produce		plain
//	@Success		200
//	@Tags			User
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Deprecated
func DeleteToken(c *gin.Context) {
	_store := store.FromContext(c)

	user := session.User(c)
	user.Hash = base32.StdEncoding.EncodeToString(
		random.GetRandomBytes(32),
	)
	if err := _store.UpdateUser(user); err != nil {
		c.String(http.StatusInternalServerError, "Error revoking tokens. %s", err)
		return
	}

	t := token.New(token.UserToken)
	t.Set("user-id", strconv.FormatInt(user.ID, 10))
	tokenString, err := t.Sign(user.Hash)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.String(http.StatusOK, tokenString)
}
