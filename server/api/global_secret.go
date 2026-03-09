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

	"github.com/danielgtaylor/huma/v2"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
)

type GetGlobalSecretListInput struct {
	PaginationParams
}

type GetGlobalSecretListOutput struct {
	Body []*model.Secret
}

func GetGlobalSecretList(ctx context.Context, input *GetGlobalSecretListInput) (*GetGlobalSecretListOutput, error) {
	secretService := server.Config.Services.Manager.SecretService()
	list, err := secretService.GlobalSecretList(&model.ListOptions{
		Page:    input.Page,
		PerPage: input.PerPage,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("Error getting global secret list", err)
	}
	// copy the secret detail to remove the sensitive
	// password and token fields.
	for i, secret := range list {
		list[i] = secret.Copy()
	}
	return &GetGlobalSecretListOutput{Body: list}, nil
}

type GetGlobalSecretInput struct {
	Secret string `path:"secret" doc:"the secret's name"`
}

type GetGlobalSecretOutput struct {
	Body *model.Secret
}

func GetGlobalSecret(ctx context.Context, input *GetGlobalSecretInput) (*GetGlobalSecretOutput, error) {
	secretService := server.Config.Services.Manager.SecretService()
	secret, err := secretService.GlobalSecretFind(input.Secret)
	if err != nil {
		return nil, dbErr(err)
	}
	return &GetGlobalSecretOutput{Body: secret.Copy()}, nil
}

type GetGlobalSecretAbbreviatedInput struct {
	Secret string `path:"secret" doc:"the secret's name"`
}

type GetGlobalSecretAbbreviatedOutput struct {
	Body *model.Secret
}

func GetGlobalSecretAbbreviated(ctx context.Context, input *GetGlobalSecretAbbreviatedInput) (*GetGlobalSecretAbbreviatedOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)

	secretService := server.Config.Services.Manager.SecretService()
	secret, err := secretService.GlobalSecretFind(input.Secret)
	if err != nil {
		return nil, dbErr(err)
	}

	// Create authorizer and check admin permission
	authorizer := NewSecretViewAuthorizerFromContext(c, server.Config.Services.Membership)
	if !authorizer.CanViewGlobalSecret(user) {
		return nil, huma.Error403Forbidden("Instance admin privileges required to view secret value")
	}

	return &GetGlobalSecretAbbreviatedOutput{Body: secret.CopyWithAbbreviatedValue()}, nil
}

type PostGlobalSecretInput struct {
	Body model.Secret
}

type PostGlobalSecretOutput struct {
	Body *model.Secret
}

func PostGlobalSecret(ctx context.Context, input *PostGlobalSecretInput) (*PostGlobalSecretOutput, error) {
	secret := &model.Secret{
		Name:   input.Body.Name,
		Value:  input.Body.Value,
		Events: input.Body.Events,
		Images: input.Body.Images,
	}
	if err := secret.Validate(); err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("Error inserting global secret: %s", err))
	}

	secretService := server.Config.Services.Manager.SecretService()

	// Check if secret already exists
	if _, err := secretService.GlobalSecretFind(input.Body.Name); err == nil {
		return nil, huma.Error409Conflict(fmt.Sprintf("Secret %q already exists as a global secret", input.Body.Name))
	}

	if err := secretService.GlobalSecretCreate(secret); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error inserting global secret %q", input.Body.Name), err)
	}
	return &PostGlobalSecretOutput{Body: secret.Copy()}, nil
}

type PatchGlobalSecretInput struct {
	Secret string `path:"secret" doc:"the secret's name"`
	Body   model.Secret
}

type PatchGlobalSecretOutput struct {
	Body *model.Secret
}

func PatchGlobalSecret(ctx context.Context, input *PatchGlobalSecretInput) (*PatchGlobalSecretOutput, error) {
	secretService := server.Config.Services.Manager.SecretService()
	secret, err := secretService.GlobalSecretFind(input.Secret)
	if err != nil {
		return nil, dbErr(err)
	}
	if input.Body.Value != "" {
		secret.Value = input.Body.Value
	}
	if input.Body.Events != nil {
		secret.Events = input.Body.Events
	}
	if input.Body.Images != nil {
		secret.Images = input.Body.Images
	}

	if err := secret.Validate(); err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("Error updating global secret: %s", err))
	}

	if err := secretService.GlobalSecretUpdate(secret); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error updating global secret %q", input.Body.Name), err)
	}
	return &PatchGlobalSecretOutput{Body: secret.Copy()}, nil
}

type DeleteGlobalSecretInput struct {
	Secret string `path:"secret" doc:"the secret's name"`
}

func DeleteGlobalSecret(ctx context.Context, input *DeleteGlobalSecretInput) (*struct{}, error) {
	secretService := server.Config.Services.Manager.SecretService()
	if err := secretService.GlobalSecretDelete(input.Secret); err != nil {
		return nil, dbErr(err)
	}
	return nil, nil
}
