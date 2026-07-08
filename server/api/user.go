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
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/tink-crypto/tink-go/v2/subtle/random"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/shared/token"
)

type GetSelfInput struct{}

type GetSelfOutput struct {
	Body *model.User
}

func GetSelf(ctx context.Context, _ *GetSelfInput) (*GetSelfOutput, error) {
	c := ginCtx(ctx)
	return &GetSelfOutput{Body: session.User(c)}, nil
}

type GetFeedInput struct {
	Latest bool `query:"latest" doc:"return only the latest pipeline per repo"`
}

type GetFeedOutput struct {
	Body any
}

func GetFeed(ctx context.Context, input *GetFeedInput) (*GetFeedOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)

	if input.Latest {
		feed, err := _store.RepoListLatest(user)
		if err != nil {
			return nil, huma.Error500InternalServerError("Error fetching feed", err)
		}
		return &GetFeedOutput{Body: feed}, nil
	}

	feed, err := _store.UserFeed(user)
	if err != nil {
		return nil, huma.Error500InternalServerError("Error fetching user feed", err)
	}
	return &GetFeedOutput{Body: feed}, nil
}

type GetReposInput struct {
	All bool `query:"all" doc:"query all repos, including inactive ones"`
}

type GetReposOutput struct {
	Body any
}

func GetRepos(ctx context.Context, input *GetReposInput) (*GetReposOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)

	if input.All {
		repos, err := getAllReposFromAllForges(c, _store, user)
		if err != nil {
			return nil, huma.Error500InternalServerError("Error fetching repository list", err)
		}
		return &GetReposOutput{Body: repos}, nil
	}

	repos, err := listActiveReposWithPipelines(_store, user)
	if err != nil {
		return nil, err
	}
	return &GetReposOutput{Body: repos}, nil
}

// listActiveReposWithPipelines returns the user's active repos hydrated with last pipeline info.
// Owned=false so repos the user only has read (pull) access to are included too,
// otherwise read-only org members never see their org repos in the overview (issue #1225).
func listActiveReposWithPipelines(_store store.Store, user *model.User) ([]*model.RepoLastPipeline, error) {
	activeRepos, err := _store.RepoList(user, false, true)
	if err != nil {
		return nil, huma.Error500InternalServerError("Error fetching repository list", err)
	}

	repoIDs := make([]int64, len(activeRepos))
	for i, repo := range activeRepos {
		repoIDs[i] = repo.ID
	}

	pipelines, err := _store.GetRepoLatestPipelines(repoIDs)
	if err != nil {
		return nil, huma.Error500InternalServerError("Error fetching repository list", err)
	}

	latestPipelines := make(map[int64]*model.Pipeline, len(activeRepos))
	for _, pipeline := range pipelines {
		latestPipelines[pipeline.RepoID] = pipeline
	}

	repos := make([]*model.RepoLastPipeline, len(activeRepos))
	for i, repo := range activeRepos {
		repos[i] = &model.RepoLastPipeline{
			Repo:         *repo,
			LastPipeline: latestPipelines[repo.ID],
		}
	}

	return repos, nil
}

type GetActiveReposInput struct{}

type GetActiveReposOutput struct {
	Body []*model.RepoLastPipeline
}

// GetActiveRepos returns the user's active repos with last pipeline info.
// Same data as GET /user/repos (default), but with a concrete OpenAPI schema.
func GetActiveRepos(ctx context.Context, _ *GetActiveReposInput) (*GetActiveReposOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)

	repos, err := listActiveReposWithPipelines(_store, user)
	if err != nil {
		return nil, err
	}
	return &GetActiveReposOutput{Body: repos}, nil
}

type GetAvailableReposInput struct {
	ForgeID       int64 `query:"forge_id"       doc:"only return repos from this forge id (0 = all linked forges)"`
	OnlyUnenabled bool  `query:"only_unenabled" doc:"omit repos already enabled in Crow"`
}

type GetAvailableReposOutput struct {
	Body []*model.Repo
}

// GetAvailableRepos returns repos visible to the user across linked forges,
// including both already-enabled and enableable ones (distinguished by IsActive).
// Optional filters narrow by forge or to unenabled-only.
func GetAvailableRepos(ctx context.Context, input *GetAvailableReposInput) (*GetAvailableReposOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)

	repos, err := getAllReposFromAllForges(c, _store, user)
	if err != nil {
		return nil, huma.Error500InternalServerError("Error fetching repository list", err)
	}

	filtered := make([]*model.Repo, 0, len(repos))
	for _, r := range repos {
		if input.ForgeID != 0 && r.ForgeID != input.ForgeID {
			continue
		}
		if input.OnlyUnenabled && r.IsActive {
			continue
		}
		filtered = append(filtered, r)
	}
	return &GetAvailableReposOutput{Body: filtered}, nil
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

	allRepos := make([]*model.Repo, 0)

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

type PostTokenInput struct{}

type PostTokenOutput struct {
	Body string
}

func PostToken(ctx context.Context, _ *PostTokenInput) (*PostTokenOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)
	t := token.New(token.UserToken)
	t.Set("user-id", strconv.FormatInt(user.ID, 10))
	tokenString, err := t.Sign(user.Hash)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return &PostTokenOutput{Body: tokenString}, nil
}

type DeleteTokenInput struct{}

type DeleteTokenOutput struct {
	Body string
}

func DeleteToken(ctx context.Context, _ *DeleteTokenInput) (*DeleteTokenOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	user := session.User(c)
	user.Hash = base32.StdEncoding.EncodeToString(
		random.GetRandomBytes(32),
	)
	if err := _store.UpdateUser(user); err != nil {
		return nil, huma.Error500InternalServerError("Error revoking tokens", err)
	}

	t := token.New(token.UserToken)
	t.Set("user-id", strconv.FormatInt(user.ID, 10))
	tokenString, err := t.Sign(user.Hash)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return &DeleteTokenOutput{Body: tokenString}, nil
}
