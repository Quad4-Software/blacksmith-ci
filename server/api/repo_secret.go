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

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
)

type GetSecretInput struct {
	RepoID int64  `path:"repo_id" doc:"the repository id"`
	Secret string `path:"secret" doc:"the secret name"`
}

type GetSecretOutput struct {
	Body *model.Secret
}

func GetSecret(ctx context.Context, input *GetSecretInput) (*GetSecretOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)

	secretService := server.Config.Services.Manager.SecretServiceFromRepo(repo)
	secret, err := secretService.SecretFind(repo, input.Secret)
	if err != nil {
		return nil, dbErr(err)
	}
	return &GetSecretOutput{Body: secret.Copy()}, nil
}

type GetSecretAbbreviatedInput struct {
	RepoID int64  `path:"repo_id" doc:"the repository id"`
	Secret string `path:"secret" doc:"the secret name"`
}

type GetSecretAbbreviatedOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         *model.Secret
}

func GetSecretAbbreviated(ctx context.Context, input *GetSecretAbbreviatedInput) (*GetSecretAbbreviatedOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)
	user := session.User(c)

	secretService := server.Config.Services.Manager.SecretServiceFromRepo(repo)
	secret, err := secretService.SecretFind(repo, input.Secret)
	if err != nil {
		return nil, dbErr(err)
	}

	// Create authorizer and check explicit permission
	authorizer := NewSecretViewAuthorizerFromContext(c, server.Config.Services.Membership)
	if !authorizer.CanViewRepoSecret(user, repo) {
		return nil, huma.Error403Forbidden("Insufficient permissions to view secret value. Explicit write access required.")
	}

	return &GetSecretAbbreviatedOutput{
		CacheControl: "no-store, no-cache, must-revalidate, private",
		Body:         secret.CopyWithAbbreviatedValue(),
	}, nil
}

type PostSecretInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Body   model.Secret
}

type PostSecretOutput struct {
	Body *model.Secret
}

func PostSecret(ctx context.Context, input *PostSecretInput) (*PostSecretOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)

	secret := &model.Secret{
		RepoID: repo.ID,
		Name:   input.Body.Name,
		Value:  input.Body.Value,
		Events: input.Body.Events,
		Images: input.Body.Images,
	}
	if err := secret.Validate(); err != nil {
		return nil, huma.Error422UnprocessableEntity(fmt.Sprintf("Error inserting secret. %s", err))
	}

	secretService := server.Config.Services.Manager.SecretServiceFromRepo(repo)

	// Check if secret already exists
	if _, err := secretService.SecretFind(repo, input.Body.Name); err == nil {
		return nil, huma.Error409Conflict(fmt.Sprintf("Secret %q already exists for this repository", input.Body.Name))
	}

	if err := secretService.SecretCreate(repo, secret); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error inserting secret %q. %s", input.Body.Name, err))
	}
	return &PostSecretOutput{Body: secret.Copy()}, nil
}

type PatchSecretInput struct {
	RepoID int64  `path:"repo_id" doc:"the repository id"`
	Secret string `path:"secret" doc:"the secret name"`
	Body   model.Secret
}

type PatchSecretOutput struct {
	Body *model.Secret
}

func PatchSecret(ctx context.Context, input *PatchSecretInput) (*PatchSecretOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)

	secretService := server.Config.Services.Manager.SecretServiceFromRepo(repo)
	secret, err := secretService.SecretFind(repo, input.Secret)
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
		return nil, huma.Error422UnprocessableEntity(fmt.Sprintf("Error updating secret. %s", err))
	}
	if err := secretService.SecretUpdate(repo, secret); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error updating secret %q. %s", input.Body.Name, err))
	}
	return &PatchSecretOutput{Body: secret.Copy()}, nil
}

type GetSecretListInput struct {
	PaginationParams
	RepoID int64 `path:"repo_id" doc:"the repository id"`
}

type GetSecretListOutput struct {
	Body []*model.Secret
}

func GetSecretList(ctx context.Context, input *GetSecretListInput) (*GetSecretListOutput, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)
	secretService := server.Config.Services.Manager.SecretServiceFromRepo(repo)
	list, err := secretService.SecretList(repo, &model.ListOptions{Page: input.Page, PerPage: input.PerPage})
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting secret list. %s", err))
	}
	// copy the secret detail to remove the sensitive
	// password and token fields.
	for i, secret := range list {
		list[i] = secret.Copy()
	}
	return &GetSecretListOutput{Body: list}, nil
}

type DeleteSecretInput struct {
	RepoID int64  `path:"repo_id" doc:"the repository id"`
	Secret string `path:"secret" doc:"the secret name"`
}

func DeleteSecret(ctx context.Context, input *DeleteSecretInput) (*struct{}, error) {
	c := ginCtx(ctx)
	repo := session.Repo(c)

	secretService := server.Config.Services.Manager.SecretServiceFromRepo(repo)
	if err := secretService.SecretDelete(repo, input.Secret); err != nil {
		return nil, dbErr(err)
	}
	return nil, nil
}
