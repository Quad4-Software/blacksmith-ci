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
	"fmt"

	"github.com/danielgtaylor/huma/v2"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
)

type GetRegistryInput struct {
	RepoID   int64  `path:"repo_id" doc:"the repository id"`
	Registry string `path:"registry" doc:"the registry name"`
}

type GetRegistryOutput struct {
	Body *model.Registry
}

func GetRegistry(ctx context.Context, input *GetRegistryInput) (*GetRegistryOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)

	registryService := server.Config.Services.Manager.RegistryServiceFromRepo(repo)
	registry, err := registryService.RegistryFind(repo, input.Registry)
	if err != nil {
		return nil, dbErr(err)
	}
	return &GetRegistryOutput{Body: registry.Copy()}, nil
}

type PostRegistryBody struct {
	Address  string `json:"address" minLength:"1" doc:"registry address"`
	Username string `json:"username" minLength:"1" doc:"registry username"`
	Password string `json:"password" minLength:"1" doc:"registry password"`
}

type PatchRegistryBody struct {
	Address  string `json:"address,omitempty" doc:"registry address"`
	Username string `json:"username,omitempty" doc:"registry username"`
	Password string `json:"password,omitempty" doc:"registry password"`
}

type PostRegistryInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Body   PostRegistryBody
}

type PostRegistryOutput struct {
	Body *model.Registry
}

func PostRegistry(ctx context.Context, input *PostRegistryInput) (*PostRegistryOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)

	registry := &model.Registry{
		RepoID:   repo.ID,
		Address:  input.Body.Address,
		Username: input.Body.Username,
		Password: input.Body.Password,
	}
	if err := registry.Validate(); err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("Error inserting registry. %s", err))
	}

	registryService := server.Config.Services.Manager.RegistryServiceFromRepo(repo)
	if err := registryService.RegistryCreate(repo, registry); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error inserting registry %q. %s", input.Body.Address, err))
	}
	return &PostRegistryOutput{Body: registry.Copy()}, nil
}

type PatchRegistryInput struct {
	RepoID   int64  `path:"repo_id" doc:"the repository id"`
	Registry string `path:"registry" doc:"the registry name"`
	Body     PatchRegistryBody
}

type PatchRegistryOutput struct {
	Body *model.Registry
}

func PatchRegistry(ctx context.Context, input *PatchRegistryInput) (*PatchRegistryOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)

	registryService := server.Config.Services.Manager.RegistryServiceFromRepo(repo)
	registry, err := registryService.RegistryFind(repo, input.Registry)
	if err != nil {
		return nil, dbErr(err)
	}
	if input.Body.Username != "" {
		registry.Username = input.Body.Username
	}
	if input.Body.Password != "" {
		registry.Password = input.Body.Password
	}

	if err := registry.Validate(); err != nil {
		return nil, huma.Error422UnprocessableEntity(fmt.Sprintf("Error updating registry. %s", err))
	}
	if err := registryService.RegistryUpdate(repo, registry); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error updating registry %q. %s", input.Body.Address, err))
	}
	return &PatchRegistryOutput{Body: registry.Copy()}, nil
}

type GetRegistryListInput struct {
	PaginationParams
	RepoID int64 `path:"repo_id" doc:"the repository id"`
}

type GetRegistryListOutput struct {
	Body []*model.Registry
}

func GetRegistryList(ctx context.Context, input *GetRegistryListInput) (*GetRegistryListOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)
	registryService := server.Config.Services.Manager.RegistryServiceFromRepo(repo)
	list, err := registryService.RegistryList(repo, &model.ListOptions{Page: input.Page, PerPage: input.PerPage})
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting registry list. %s", err))
	}
	// copy the registry detail to remove the sensitive
	// password and token fields.
	for i, registry := range list {
		list[i] = registry.Copy()
	}
	return &GetRegistryListOutput{Body: list}, nil
}

type DeleteRegistryInput struct {
	RepoID   int64  `path:"repo_id" doc:"the repository id"`
	Registry string `path:"registry" doc:"the registry name"`
}

func DeleteRegistry(ctx context.Context, input *DeleteRegistryInput) (*struct{}, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)

	registryService := server.Config.Services.Manager.RegistryServiceFromRepo(repo)
	if err := registryService.RegistryDelete(repo, input.Registry); err != nil {
		return nil, dbErr(err)
	}
	return nil, nil
}
