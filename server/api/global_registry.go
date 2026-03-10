// Copyright 2024 Woodpecker Authors
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

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/model"
)

type GetGlobalRegistryListInput struct {
	PaginationParams
}

type GetGlobalRegistryListOutput struct {
	Body []*model.Registry
}

func GetGlobalRegistryList(ctx context.Context, input *GetGlobalRegistryListInput) (*GetGlobalRegistryListOutput, error) {
	registryService := server.Config.Services.Manager.RegistryService()
	list, err := registryService.GlobalRegistryList(&model.ListOptions{
		Page:    input.Page,
		PerPage: input.PerPage,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("Error getting global registry list", err)
	}
	// copy the registry detail to remove the sensitive
	// password and token fields.
	for i, registry := range list {
		list[i] = registry.Copy()
	}
	return &GetGlobalRegistryListOutput{Body: list}, nil
}

type GetGlobalRegistryInput struct {
	Registry string `path:"registry" doc:"the registry's name"`
}

type GetGlobalRegistryOutput struct {
	Body *model.Registry
}

func GetGlobalRegistry(ctx context.Context, input *GetGlobalRegistryInput) (*GetGlobalRegistryOutput, error) {
	registryService := server.Config.Services.Manager.RegistryService()
	registry, err := registryService.GlobalRegistryFind(input.Registry)
	if err != nil {
		return nil, dbErr(err)
	}
	return &GetGlobalRegistryOutput{Body: registry.Copy()}, nil
}

type PostGlobalRegistryInput struct {
	Body PostRegistryBody
}

type PostGlobalRegistryOutput struct {
	Body *model.Registry
}

func PostGlobalRegistry(ctx context.Context, input *PostGlobalRegistryInput) (*PostGlobalRegistryOutput, error) {
	registry := &model.Registry{
		Address:  input.Body.Address,
		Username: input.Body.Username,
		Password: input.Body.Password,
	}
	if err := registry.Validate(); err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("Error inserting global registry: %s", err))
	}

	registryService := server.Config.Services.Manager.RegistryService()
	if err := registryService.GlobalRegistryCreate(registry); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error inserting global registry %q", input.Body.Address), err)
	}
	return &PostGlobalRegistryOutput{Body: registry.Copy()}, nil
}

type PatchGlobalRegistryInput struct {
	Registry string `path:"registry" doc:"the registry's name"`
	Body     PatchRegistryBody
}

type PatchGlobalRegistryOutput struct {
	Body *model.Registry
}

func PatchGlobalRegistry(ctx context.Context, input *PatchGlobalRegistryInput) (*PatchGlobalRegistryOutput, error) {
	registryService := server.Config.Services.Manager.RegistryService()
	registry, err := registryService.GlobalRegistryFind(input.Registry)
	if err != nil {
		return nil, dbErr(err)
	}
	if input.Body.Address != "" {
		registry.Address = input.Body.Address
	}
	if input.Body.Username != "" {
		registry.Username = input.Body.Username
	}
	if input.Body.Password != "" {
		registry.Password = input.Body.Password
	}

	if err := registry.Validate(); err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("Error updating global registry: %s", err))
	}

	if err := registryService.GlobalRegistryUpdate(registry); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error updating global registry %q", input.Body.Address), err)
	}
	return &PatchGlobalRegistryOutput{Body: registry.Copy()}, nil
}

type DeleteGlobalRegistryInput struct {
	Registry string `path:"registry" doc:"the registry's name"`
}

func DeleteGlobalRegistry(ctx context.Context, input *DeleteGlobalRegistryInput) (*struct{}, error) {
	registryService := server.Config.Services.Manager.RegistryService()
	if err := registryService.GlobalRegistryDelete(input.Registry); err != nil {
		return nil, dbErr(err)
	}
	return nil, nil
}
