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

	"github.com/danielgtaylor/huma/v2"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/store"
)

type GetForgesInput struct {
	PaginationParams
}

type GetForgesOutput struct {
	Body []*model.Forge
}

func GetForges(ctx context.Context, input *GetForgesInput) (*GetForgesOutput, error) {
	c := ginCtx(ctx)
	forges, err := store.FromContext(c).ForgeList(&model.ListOptions{
		Page:    input.Page,
		PerPage: input.PerPage,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("Error getting forge list", err)
	}

	user := session.User(c)
	if user != nil && user.Admin {
		return &GetForgesOutput{Body: forges}, nil
	}

	// copy forges data without sensitive information
	for i, forge := range forges {
		forges[i] = forge.PublicCopy()
	}

	return &GetForgesOutput{Body: forges}, nil
}

type GetForgeInput struct {
	ForgeID int64 `path:"forgeId" doc:"the forge's id"`
}

type GetForgeOutput struct {
	Body *model.Forge
}

func GetForge(ctx context.Context, input *GetForgeInput) (*GetForgeOutput, error) {
	c := ginCtx(ctx)
	forge, err := store.FromContext(c).ForgeGet(input.ForgeID)
	if err != nil {
		return nil, dbErr(err)
	}

	user := session.User(c)
	if user != nil && user.Admin {
		return &GetForgeOutput{Body: forge}, nil
	}
	return &GetForgeOutput{Body: forge.PublicCopy()}, nil
}

type PatchForgeInput struct {
	ForgeID int64 `path:"forgeId" doc:"the forge's id"`
	Body    struct {
		model.Forge
		ClientSecret string `json:"client_secret"`
	}
}

type PatchForgeOutput struct {
	Body *model.Forge
}

func PatchForge(ctx context.Context, input *PatchForgeInput) (*PatchForgeOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	forge, err := _store.ForgeGet(input.ForgeID)
	if err != nil {
		return nil, dbErr(err)
	}
	forge.URL = input.Body.URL
	forge.Type = input.Body.Type
	forge.Client = input.Body.Client
	forge.OAuthHost = input.Body.OAuthHost
	forge.SkipVerify = input.Body.SkipVerify
	forge.AdditionalOptions = input.Body.AdditionalOptions
	if input.Body.ClientSecret != "" {
		forge.ClientSecret = input.Body.ClientSecret
	}

	err = _store.ForgeUpdate(forge)
	if err != nil {
		return nil, huma.Error409Conflict("conflict updating forge")
	}

	return &PatchForgeOutput{Body: forge}, nil
}

type PostForgeInput struct {
	Body model.Forge
}

type PostForgeOutput struct {
	Body *model.Forge
}

func PostForge(ctx context.Context, input *PostForgeInput) (*PostForgeOutput, error) {
	c := ginCtx(ctx)

	forge := &model.Forge{
		URL:               input.Body.URL,
		Type:              input.Body.Type,
		Client:            input.Body.Client,
		ClientSecret:      input.Body.ClientSecret,
		OAuthHost:         input.Body.OAuthHost,
		SkipVerify:        input.Body.SkipVerify,
		AdditionalOptions: input.Body.AdditionalOptions,
	}
	if err := store.FromContext(c).ForgeCreate(forge); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return &PostForgeOutput{Body: forge}, nil
}

type DeleteForgeInput struct {
	ForgeID int64 `path:"forgeId" doc:"the forge's id"`
}

func DeleteForge(ctx context.Context, input *DeleteForgeInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	forge, err := _store.ForgeGet(input.ForgeID)
	if err != nil {
		return nil, dbErr(err)
	}

	if err = _store.ForgeDelete(forge); err != nil {
		return nil, huma.Error500InternalServerError("Error deleting forge", err)
	}
	return nil, nil
}
