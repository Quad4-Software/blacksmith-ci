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
	"codefloe.com/crowci/crow/v5/server/store"
)

type GetOrgSecretInput struct {
	OrgID  int64  `path:"org_id" doc:"the org's id"`
	Secret string `path:"secret" doc:"the secret's name"`
}

type GetOrgSecretOutput struct {
	Body *model.Secret
}

func GetOrgSecret(ctx context.Context, input *GetOrgSecretInput) (*GetOrgSecretOutput, error) {
	c := ginCtx(ctx)
	org := session.Org(c)

	secretService := server.Config.Services.Manager.SecretService()
	secret, err := secretService.OrgSecretFind(org.ID, input.Secret)
	if err != nil {
		return nil, dbErr(err)
	}
	return &GetOrgSecretOutput{Body: secret.Copy()}, nil
}

type GetOrgSecretAbbreviatedInput struct {
	OrgID  int64  `path:"org_id" doc:"the org's id"`
	Secret string `path:"secret" doc:"the secret's name"`
}

type GetOrgSecretAbbreviatedOutput struct {
	Body *model.Secret
}

func GetOrgSecretAbbreviated(ctx context.Context, input *GetOrgSecretAbbreviatedInput) (*GetOrgSecretAbbreviatedOutput, error) {
	c := ginCtx(ctx)
	org := session.Org(c)
	user := session.User(c)

	secretService := server.Config.Services.Manager.SecretService()
	secret, err := secretService.OrgSecretFind(org.ID, input.Secret)
	if err != nil {
		return nil, dbErr(err)
	}

	// Get forge by org's forge ID (not user's default forge)
	_forge, err := server.Config.Services.Manager.ForgeByID(org.ForgeID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Cannot get forge for organization")
	}

	// Build a user with the correct forge credentials for this org's forge
	forgeUser := user
	if uf, ufErr := store.FromContext(c).UserForgeGet(user.ID, org.ForgeID); ufErr == nil && uf != nil {
		forgeUserCopy := *user
		forgeUserCopy.ForgeID = org.ForgeID
		forgeUserCopy.AccessToken = uf.AccessToken
		forgeUserCopy.RefreshToken = uf.RefreshToken
		forgeUserCopy.Expiry = uf.Expiry
		forgeUser = &forgeUserCopy
	}

	// Create authorizer and check org admin permission
	authorizer := NewSecretViewAuthorizerFromContext(c, server.Config.Services.Membership)
	if !authorizer.CanViewOrgSecret(c, forgeUser, org, _forge) {
		return nil, huma.Error403Forbidden("Org admin privileges required to view secret value")
	}

	return &GetOrgSecretAbbreviatedOutput{Body: secret.CopyWithAbbreviatedValue()}, nil
}

type GetOrgSecretListInput struct {
	PaginationParams
	OrgID int64 `path:"org_id" doc:"the org's id"`
}

type GetOrgSecretListOutput struct {
	Body []*model.Secret
}

func GetOrgSecretList(ctx context.Context, input *GetOrgSecretListInput) (*GetOrgSecretListOutput, error) {
	c := ginCtx(ctx)
	org := session.Org(c)

	secretService := server.Config.Services.Manager.SecretService()
	list, err := secretService.OrgSecretList(org.ID, &model.ListOptions{
		Page:    input.Page,
		PerPage: input.PerPage,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting secret list for %d", org.ID), err)
	}
	// copy the secret detail to remove the sensitive
	// password and token fields.
	for i, secret := range list {
		list[i] = secret.Copy()
	}
	return &GetOrgSecretListOutput{Body: list}, nil
}

type PostOrgSecretInput struct {
	OrgID int64 `path:"org_id" doc:"the org's id"`
	Body  PostSecretBody
}

type PostOrgSecretOutput struct {
	Body *model.Secret
}

func PostOrgSecret(ctx context.Context, input *PostOrgSecretInput) (*PostOrgSecretOutput, error) {
	c := ginCtx(ctx)
	org := session.Org(c)

	secret := &model.Secret{
		OrgID:  org.ID,
		Name:   input.Body.Name,
		Value:  input.Body.Value,
		Events: input.Body.Events,
		Images: input.Body.Images,
	}
	if err := secret.Validate(); err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("Error inserting org %d secret: %s", org.ID, err))
	}

	secretService := server.Config.Services.Manager.SecretService()

	// Check if secret already exists
	if _, err := secretService.OrgSecretFind(org.ID, input.Body.Name); err == nil {
		if org.IsUser {
			return nil, huma.Error409Conflict(fmt.Sprintf("Secret %q already exists in your personal secrets", input.Body.Name))
		}
		return nil, huma.Error409Conflict(fmt.Sprintf("Secret %q already exists for this organization", input.Body.Name))
	}

	if err := secretService.OrgSecretCreate(org.ID, secret); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error inserting org %d secret %q", org.ID, input.Body.Name), err)
	}
	return &PostOrgSecretOutput{Body: secret.Copy()}, nil
}

type PatchOrgSecretInput struct {
	OrgID  int64  `path:"org_id" doc:"the org's id"`
	Secret string `path:"secret" doc:"the secret's name"`
	Body   PatchSecretBody
}

type PatchOrgSecretOutput struct {
	Body *model.Secret
}

func PatchOrgSecret(ctx context.Context, input *PatchOrgSecretInput) (*PatchOrgSecretOutput, error) {
	c := ginCtx(ctx)
	org := session.Org(c)

	secretService := server.Config.Services.Manager.SecretService()
	secret, err := secretService.OrgSecretFind(org.ID, input.Secret)
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
		return nil, huma.Error400BadRequest(fmt.Sprintf("Error updating org %d secret: %s", org.ID, err))
	}

	if err := secretService.OrgSecretUpdate(org.ID, secret); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error updating org %d secret %q", org.ID, secret.Name), err)
	}
	return &PatchOrgSecretOutput{Body: secret.Copy()}, nil
}

type DeleteOrgSecretInput struct {
	OrgID  int64  `path:"org_id" doc:"the org's id"`
	Secret string `path:"secret" doc:"the secret's name"`
}

func DeleteOrgSecret(ctx context.Context, input *DeleteOrgSecretInput) (*struct{}, error) {
	c := ginCtx(ctx)
	org := session.Org(c)

	secretService := server.Config.Services.Manager.SecretService()
	if err := secretService.OrgSecretDelete(org.ID, input.Secret); err != nil {
		return nil, dbErr(err)
	}
	return nil, nil
}
