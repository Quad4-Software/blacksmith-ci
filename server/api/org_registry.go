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

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
)

type GetOrgRegistryInput struct {
	OrgID    int64  `path:"org_id" doc:"the org's id"`
	Registry string `path:"registry" doc:"the registry's address"`
}

type GetOrgRegistryOutput struct {
	Body *model.Registry
}

func GetOrgRegistry(ctx context.Context, input *GetOrgRegistryInput) (*GetOrgRegistryOutput, error) {
	c := ginCtx(ctx)
	org := session.Org(c)

	registryService := server.Config.Services.Manager.RegistryService()
	registry, err := registryService.OrgRegistryFind(org.ID, input.Registry)
	if err != nil {
		return nil, dbErr(err)
	}
	return &GetOrgRegistryOutput{Body: registry.Copy()}, nil
}

type GetOrgRegistryListInput struct {
	PaginationParams
	OrgID int64 `path:"org_id" doc:"the org's id"`
}

type GetOrgRegistryListOutput struct {
	Body []*model.Registry
}

func GetOrgRegistryList(ctx context.Context, input *GetOrgRegistryListInput) (*GetOrgRegistryListOutput, error) {
	c := ginCtx(ctx)
	org := session.Org(c)

	registryService := server.Config.Services.Manager.RegistryService()
	list, err := registryService.OrgRegistryList(org.ID, &model.ListOptions{
		Page:    input.Page,
		PerPage: input.PerPage,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting registry list for %d", org.ID), err)
	}
	// copy the registry detail to remove the sensitive
	// password and token fields.
	for i, registry := range list {
		list[i] = registry.Copy()
	}
	return &GetOrgRegistryListOutput{Body: list}, nil
}

type PostOrgRegistryInput struct {
	OrgID int64 `path:"org_id" doc:"the org's id"`
	Body  PostRegistryBody
}

type PostOrgRegistryOutput struct {
	Body *model.Registry
}

func PostOrgRegistry(ctx context.Context, input *PostOrgRegistryInput) (*PostOrgRegistryOutput, error) {
	c := ginCtx(ctx)
	org := session.Org(c)

	registry := &model.Registry{
		OrgID:    org.ID,
		Address:  input.Body.Address,
		Username: input.Body.Username,
		Password: input.Body.Password,
	}
	if err := registry.Validate(); err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("Error inserting org %d registry: %s", org.ID, err))
	}

	registryService := server.Config.Services.Manager.RegistryService()
	if err := registryService.OrgRegistryCreate(org.ID, registry); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error inserting org %d registry %q", org.ID, input.Body.Address), err)
	}
	return &PostOrgRegistryOutput{Body: registry.Copy()}, nil
}

type PatchOrgRegistryInput struct {
	OrgID    int64  `path:"org_id" doc:"the org's id"`
	Registry string `path:"registry" doc:"the registry's name"`
	Body     PatchRegistryBody
}

type PatchOrgRegistryOutput struct {
	Body *model.Registry
}

func PatchOrgRegistry(ctx context.Context, input *PatchOrgRegistryInput) (*PatchOrgRegistryOutput, error) {
	c := ginCtx(ctx)
	org := session.Org(c)

	registryService := server.Config.Services.Manager.RegistryService()
	registry, err := registryService.OrgRegistryFind(org.ID, input.Registry)
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
		return nil, huma.Error400BadRequest(fmt.Sprintf("Error updating org %d registry: %s", org.ID, err))
	}

	if err := registryService.OrgRegistryUpdate(org.ID, registry); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error updating org %d registry %q", org.ID, input.Body.Address), err)
	}
	return &PatchOrgRegistryOutput{Body: registry.Copy()}, nil
}

type DeleteOrgRegistryInput struct {
	OrgID    int64  `path:"org_id" doc:"the org's id"`
	Registry string `path:"registry" doc:"the registry's name"`
}

func DeleteOrgRegistry(ctx context.Context, input *DeleteOrgRegistryInput) (*struct{}, error) {
	c := ginCtx(ctx)
	org := session.Org(c)

	registryService := server.Config.Services.Manager.RegistryService()
	if err := registryService.OrgRegistryDelete(org.ID, input.Registry); err != nil {
		return nil, dbErr(err)
	}
	return nil, nil
}
