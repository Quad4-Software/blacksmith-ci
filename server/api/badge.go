// Copyright 2022 Woodpecker Authors
// Copyright 2021 Informatyka Boguslawski sp. z o.o. sp.k., http://www.ib.pl/
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
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/badges"
	"codefloe.com/crowci/crow/v5/server/ccmenu"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store"
	"codefloe.com/crowci/crow/v5/server/store/types"
)

type GetBadgeInput struct {
	RepoIDOrOwner string `path:"repo_id_or_owner" doc:"the repository id or owner name"`
	RepoName      string `path:"repo_name" doc:"the repository name"`
	Branch        string `query:"branch" doc:"the branch name"`
}

func GetBadge(ctx context.Context, input *GetBadgeInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	var repo *model.Repo
	var err error

	if input.RepoName != "" {
		repo, err = _store.GetRepoName(input.RepoIDOrOwner + "/" + input.RepoName)
	} else {
		var repoID int64
		repoID, err = strconv.ParseInt(input.RepoIDOrOwner, 10, 64)
		if err != nil {
			c.Writer.WriteHeader(http.StatusBadRequest)
			return nil, nil
		}
		repo, err = _store.GetRepo(repoID)
	}

	if err != nil {
		return nil, dbErr(err)
	}

	// if no commit was found then display
	// the 'none' badge, instead of throwing
	// an error response
	branch := input.Branch
	if len(branch) == 0 {
		branch = repo.Branch
	}

	pipeline, err := _store.GetPipelineBadge(repo, branch)
	if err != nil {
		if !errors.Is(err, types.RecordNotExist) {
			log.Warn().Err(err).Msg("could not get last pipeline for badge")
		}
		pipeline = nil
	}

	// we serve an SVG, so set content type appropriately.
	c.Writer.Header().Set("Content-Type", "image/svg+xml")
	c.String(http.StatusOK, badges.Generate(pipeline))
	return nil, nil
}

type GetCCInput struct {
	RepoIDOrOwner string `path:"repo_id_or_owner" doc:"the repository id or owner name"`
	RepoName      string `path:"repo_name" doc:"the repository name"`
}

func GetCC(ctx context.Context, input *GetCCInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	var repo *model.Repo
	var err error

	if input.RepoName != "" {
		repo, err = _store.GetRepoName(input.RepoIDOrOwner + "/" + input.RepoName)
	} else {
		var repoID int64
		repoID, err = strconv.ParseInt(input.RepoIDOrOwner, 10, 64)
		if err != nil {
			c.Writer.WriteHeader(http.StatusBadRequest)
			return nil, nil
		}
		repo, err = _store.GetRepo(repoID)
	}

	if err != nil {
		return nil, dbErr(err)
	}

	pipelines, err := _store.GetPipelineList(repo, &model.ListOptions{Page: 1, PerPage: 1}, nil)
	if err != nil && !errors.Is(err, types.RecordNotExist) {
		log.Warn().Err(err).Msg("could not get pipeline list")
		return nil, huma.Error500InternalServerError("could not get pipeline list")
	}
	if len(pipelines) == 0 {
		return nil, huma.Error404NotFound("no pipelines found")
	}

	url := fmt.Sprintf("%s/repos/%d/pipeline/%d", server.Config.Server.Host, repo.ID, pipelines[0].Number)
	cc := ccmenu.New(repo, pipelines[0], url)
	c.XML(http.StatusOK, cc)
	return nil, nil
}
