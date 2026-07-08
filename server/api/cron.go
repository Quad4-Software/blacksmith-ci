// Copyright 2022 Woodpecker Authors
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

	"codefloe.com/crowci/crow/v6/server"
	cronScheduler "codefloe.com/crowci/crow/v6/server/cron"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pipeline"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/store"
)

type GetCronInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Cron   int64 `path:"cron" doc:"the cron job id"`
}

type GetCronOutput struct {
	Body *model.Cron
}

func GetCron(ctx context.Context, input *GetCronInput) (*GetCronOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)

	cron, err := store.FromContext(c).CronFind(repo, input.Cron)
	if err != nil {
		return nil, dbErr(err)
	}
	return &GetCronOutput{Body: cron}, nil
}

type RunCronInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Cron   int64 `path:"cron" doc:"the cron job id"`
}

type RunCronOutput struct {
	Body *model.Pipeline
}

func RunCron(ctx context.Context, input *RunCronInput) (*RunCronOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)
	_store := store.FromContext(c)

	cron, err := _store.CronFind(repo, input.Cron)
	if err != nil {
		return nil, dbErr(err)
	}

	// Reset failure state on manual trigger so the user can re-enable a disabled cron
	if cron.Disabled || cron.FailCount > 0 {
		cron.Disabled = false
		cron.FailCount = 0
		cron.FailMsg = ""
		if err := _store.CronUpdate(repo, cron); err != nil {
			log.Error().Err(err).Int64("cronID", cron.ID).Msg("failed to reset cron failure state on manual run")
		}
	}

	repo, newPipeline, err := cronScheduler.CreatePipeline(c, _store, cron)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error creating pipeline for cron %d. %s", input.Cron, err))
	}

	pl, err := pipeline.Create(c, _store, repo, newPipeline)
	if err != nil {
		return nil, pipelineErr(err)
	}

	return &RunCronOutput{Body: pl}, nil
}

type PostCronBody struct {
	Name     string `json:"name" minLength:"1" doc:"cron job name"`
	Schedule string `json:"schedule" minLength:"1" doc:"cron schedule expression"`
	Branch   string `json:"branch,omitempty" doc:"branch to run cron on"`
}

type PatchCronBody struct {
	Name     string `json:"name,omitempty" doc:"cron job name"`
	Schedule string `json:"schedule,omitempty" doc:"cron schedule expression"`
	Branch   string `json:"branch,omitempty" doc:"branch to run cron on"`
}

type PostCronInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Body   PostCronBody
}

type PostCronOutput struct {
	Body *model.Cron
}

func PostCron(ctx context.Context, input *PostCronInput) (*PostCronOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)
	user := session.User(c)
	_store := store.FromContext(c)
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get forge from repo")
		return nil, huma.Error500InternalServerError("Cannot get forge from repo")
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
		return nil, huma.Error500InternalServerError("Failed to refresh user token")
	}

	cron := &model.Cron{
		RepoID:    repo.ID,
		Name:      input.Body.Name,
		CreatorID: user.ID,
		Schedule:  input.Body.Schedule,
		Branch:    input.Body.Branch,
	}
	if err := cron.Validate(); err != nil {
		return nil, huma.Error422UnprocessableEntity(fmt.Sprintf("Error inserting cron. validate failed: %s", err))
	}

	nextExec, err := cronScheduler.CalcNewNext(input.Body.Schedule, time.Now())
	if err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("Error inserting cron. schedule could not parsed: %s", err))
	}
	cron.NextExec = nextExec.Unix()

	if input.Body.Branch != "" {
		// check if branch exists on forge
		_, err := _forge.BranchHead(c, user, repo, input.Body.Branch)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("Error inserting cron. branch not resolved: %s", err))
		}
	}

	if err := _store.CronCreate(cron); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error inserting cron %q. %s", input.Body.Name, err))
	}
	return &PostCronOutput{Body: cron}, nil
}

type PatchCronInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Cron   int64 `path:"cron" doc:"the cron job id"`
	Body   PatchCronBody
}

type PatchCronOutput struct {
	Body *model.Cron
}

func PatchCron(ctx context.Context, input *PatchCronInput) (*PatchCronOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)
	user := session.User(c)
	_store := store.FromContext(c)
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get forge from repo")
		return nil, huma.Error500InternalServerError("Cannot get forge from repo")
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
		return nil, huma.Error500InternalServerError("Failed to refresh user token")
	}

	cron, err := _store.CronFind(repo, input.Cron)
	if err != nil {
		return nil, dbErr(err)
	}
	if input.Body.Branch != "" {
		// check if branch exists on forge
		_, err := _forge.BranchHead(c, user, repo, input.Body.Branch)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("Error inserting cron. branch not resolved: %s", err))
		}
		cron.Branch = input.Body.Branch
	}
	if input.Body.Schedule != "" {
		cron.Schedule = input.Body.Schedule
		nextExec, err := cronScheduler.CalcNewNext(input.Body.Schedule, time.Now())
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("Error inserting cron. schedule could not parsed: %s", err))
		}
		cron.NextExec = nextExec.Unix()
	}
	if input.Body.Name != "" {
		cron.Name = input.Body.Name
	}
	cron.CreatorID = user.ID

	if err := cron.Validate(); err != nil {
		return nil, huma.Error422UnprocessableEntity(fmt.Sprintf("Error inserting cron. validate failed: %s", err))
	}
	if err := _store.CronUpdate(repo, cron); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error updating cron %q. %s", input.Body.Name, err))
	}
	return &PatchCronOutput{Body: cron}, nil
}

type GetCronListInput struct {
	PaginationParams
	RepoID int64 `path:"repo_id" doc:"the repository id"`
}

type GetCronListOutput struct {
	Body []*model.Cron
}

func GetCronList(ctx context.Context, input *GetCronListInput) (*GetCronListOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)
	list, err := store.FromContext(c).CronList(repo, &model.ListOptions{Page: input.Page, PerPage: input.PerPage})
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting cron list. %s", err))
	}
	return &GetCronListOutput{Body: list}, nil
}

type DeleteCronInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Cron   int64 `path:"cron" doc:"the cron job id"`
}

func DeleteCron(ctx context.Context, input *DeleteCronInput) (*struct{}, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)
	if err := store.FromContext(c).CronDelete(repo, input.Cron); err != nil {
		return nil, dbErr(err)
	}
	return nil, nil
}
