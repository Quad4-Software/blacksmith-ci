// Copyright 2025 Crow CI Authors
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

// ── Input / Output types ─────────────────────────────────────────────.

type GetSecretTargetListInput struct {
	SecretID int64 `path:"secret_id" doc:"secret ID"`
}

type GetSecretTargetListOutput struct {
	Body []*model.SecretTarget
}

type PostSecretTargetInput struct {
	SecretID int64 `path:"secret_id" doc:"secret ID"`
	Body     PostSecretTargetBody
}

type PostSecretTargetBody struct {
	OrgID  int64 `json:"org_id"  doc:"organization ID to share with (0 if sharing with repo)"`
	RepoID int64 `json:"repo_id" doc:"repository ID to share with (0 if sharing with org)"`
}

type PostSecretTargetOutput struct {
	Body *model.SecretTarget
}

type DeleteSecretTargetInput struct {
	SecretID int64 `path:"secret_id" doc:"secret ID"`
	TargetID int64 `path:"target_id" doc:"target ID"`
}

type PatchSecretOwnerInput struct {
	SecretID int64 `path:"secret_id" doc:"secret ID"`
	Body     PatchSecretOwnerBody
}

type PatchSecretOwnerBody struct {
	OrgID        int64 `json:"org_id"         doc:"new owner org ID"`
	RepoID       int64 `json:"repo_id"        doc:"new owner repo ID"`
	KeepAsTarget bool  `json:"keep_as_target" doc:"keep old owner as a shared target"`
}

// ── Authorization helper ─────────────────────────────────────────────.

func authorizeSecretAccess(ctx context.Context, _store store.Store, secretID int64) error {
	secret, err := _store.SecretFindByID(secretID)
	if err != nil {
		return huma.Error404NotFound("secret not found")
	}

	user := session.User(ginCtx(ctx))
	if user == nil {
		return huma.Error401Unauthorized("authentication required")
	}

	if secret.IsGlobal() {
		if !user.Admin {
			return huma.Error403Forbidden("admin access required")
		}
		return nil
	}

	if secret.IsOrganization() {
		return authorizeOrgSecretAccess(ctx, _store, user, secret.OrgID)
	}

	if secret.IsRepository() {
		repo, err := _store.GetRepo(secret.RepoID)
		if err != nil {
			return huma.Error404NotFound("repository not found")
		}
		perm, err := _store.PermFind(user, repo)
		if err != nil || !perm.Push {
			return huma.Error403Forbidden("push access required")
		}
		return nil
	}

	return huma.Error403Forbidden("access denied")
}

// authorizeOrgSecretAccess mirrors MustOrgMember(admin=true) semantics: the user
// must be an instance admin, own the org (personal org), or be an org admin as
// reported by the forge membership service.
func authorizeOrgSecretAccess(ctx context.Context, _store store.Store, user *model.User, orgID int64) error {
	if user.Admin {
		return nil
	}

	org, err := _store.OrgGet(orgID)
	if err != nil {
		return huma.Error404NotFound("organization not found")
	}

	// User can always manage their own (personal) org secrets.
	if org.Name == user.Login {
		return nil
	}

	_forge, err := server.Config.Services.Manager.ForgeByID(org.ForgeID)
	if err != nil {
		return huma.Error500InternalServerError("cannot get forge for organization")
	}

	// Build a user with the correct forge credentials for this org's forge.
	forgeUser := user
	if uf, ufErr := _store.UserForgeGet(user.ID, org.ForgeID); ufErr == nil && uf != nil {
		forgeUserCopy := *user
		forgeUserCopy.ForgeID = org.ForgeID
		forgeUserCopy.AccessToken = uf.AccessToken
		forgeUserCopy.RefreshToken = uf.RefreshToken
		forgeUserCopy.Expiry = uf.Expiry
		forgeUser = &forgeUserCopy
	}

	perm, err := server.Config.Services.Membership.Get(ctx, _forge, forgeUser, org.Name)
	if err != nil {
		return huma.Error500InternalServerError("failed to check org membership")
	}
	if perm == nil || !perm.Admin {
		return huma.Error403Forbidden("org admin access required for org secrets")
	}
	return nil
}

// ── Handlers ─────────────────────────────────────────────────────────.

func GetSecretTargetList(ctx context.Context, input *GetSecretTargetListInput) (*GetSecretTargetListOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	if err := authorizeSecretAccess(ctx, _store, input.SecretID); err != nil {
		return nil, err
	}

	secretService := server.Config.Services.Manager.SecretService()
	targets, err := secretService.SecretTargetList(input.SecretID)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error listing secret targets: %s", err))
	}
	if targets == nil {
		targets = []*model.SecretTarget{}
	}
	return &GetSecretTargetListOutput{Body: targets}, nil
}

func PostSecretTarget(ctx context.Context, input *PostSecretTargetInput) (*PostSecretTargetOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	if err := authorizeSecretAccess(ctx, _store, input.SecretID); err != nil {
		return nil, err
	}

	if input.Body.OrgID == 0 && input.Body.RepoID == 0 {
		return nil, huma.Error422UnprocessableEntity("either org_id or repo_id must be set")
	}
	if input.Body.OrgID != 0 && input.Body.RepoID != 0 {
		return nil, huma.Error422UnprocessableEntity("only one of org_id or repo_id may be set")
	}

	secretService := server.Config.Services.Manager.SecretService()
	if err := secretService.SecretTargetCreate(input.SecretID, input.Body.OrgID, input.Body.RepoID); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error creating secret target: %s", err))
	}

	// Fetch the created target to return it
	targets, err := secretService.SecretTargetList(input.SecretID)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error fetching secret targets: %s", err))
	}
	// Find the target we just created
	for _, t := range targets {
		if t.OrgID == input.Body.OrgID && t.RepoID == input.Body.RepoID {
			return &PostSecretTargetOutput{Body: t}, nil
		}
	}

	return &PostSecretTargetOutput{Body: &model.SecretTarget{
		SecretID: input.SecretID,
		OrgID:    input.Body.OrgID,
		RepoID:   input.Body.RepoID,
	}}, nil
}

func DeleteSecretTarget(ctx context.Context, input *DeleteSecretTargetInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	if err := authorizeSecretAccess(ctx, _store, input.SecretID); err != nil {
		return nil, err
	}

	// Find the target by listing and matching the target ID
	secretService := server.Config.Services.Manager.SecretService()
	targets, err := secretService.SecretTargetList(input.SecretID)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error listing secret targets: %s", err))
	}

	for _, t := range targets {
		if t.ID == input.TargetID {
			if err := _store.SecretTargetDelete(t); err != nil {
				return nil, huma.Error500InternalServerError(fmt.Sprintf("Error deleting secret target: %s", err))
			}
			return nil, nil
		}
	}

	return nil, huma.Error404NotFound("secret target not found")
}

func PatchSecretOwner(ctx context.Context, input *PatchSecretOwnerInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	if err := authorizeSecretAccess(ctx, _store, input.SecretID); err != nil {
		return nil, err
	}

	secretService := server.Config.Services.Manager.SecretService()
	if err := secretService.SecretTransferOwner(input.SecretID, input.Body.OrgID, input.Body.RepoID, input.Body.KeepAsTarget); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error transferring secret ownership: %s", err))
	}

	return nil, nil
}
