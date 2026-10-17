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

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/store"
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

	return authorizeSecretScope(ctx, _store, user, secret.OrgID, secret.RepoID)
}

// authorizeSecretScope checks that the user may manage secrets owned by the
// given scope: the global scope (both IDs zero), an org, or a repo. It requires
// the same authority as creating a secret there through the scoped endpoints:
// secrets:write for global, org admin for an org, and push access for a repo.
func authorizeSecretScope(ctx context.Context, _store store.Store, user *model.User, orgID, repoID int64) error {
	if orgID != 0 && repoID != 0 {
		return huma.Error422UnprocessableEntity("only one of org_id or repo_id may be set")
	}
	if err := authorizeAccessTokenSecretEntity(ctx, _store, orgID, repoID); err != nil {
		return err
	}

	switch {
	case orgID == 0 && repoID == 0:
		if !user.Can(model.PermSecretsWrite) {
			return huma.Error403Forbidden("secrets:write permission required")
		}
		return nil
	case orgID != 0:
		return authorizeOrgSecretAccess(ctx, _store, user, orgID)
	default:
		repo, err := _store.GetRepo(repoID)
		if err != nil {
			return huma.Error404NotFound("repository not found")
		}
		if user.Admin {
			return nil
		}
		perm, err := _store.PermFind(user, repo)
		if err != nil || !perm.Push {
			return huma.Error403Forbidden("push access required")
		}
		return nil
	}
}

// authorizeAccessTokenSecretEntity checks that the access token the request
// authenticated with, if any, reaches a secret owner or target: the repo when
// repoID is set, else the org when orgID is set, else the instance, which needs
// an unrestricted token with admin:write.
func authorizeAccessTokenSecretEntity(ctx context.Context, _store store.Store, orgID, repoID int64) error {
	c := ginCtx(ctx)
	at := session.AccessToken(c)
	if at == nil {
		return nil
	}
	switch {
	case repoID != 0:
		repo, err := _store.GetRepo(repoID)
		if err != nil {
			return huma.Error404NotFound("repository not found")
		}
		if !at.AllowsRepo(repo) {
			return huma.Error403Forbidden("access token is restricted to another repository")
		}
	case orgID != 0:
		org, err := _store.OrgGet(orgID)
		if err != nil {
			return huma.Error404NotFound("organization not found")
		}
		if !session.AccessTokenAllowsOrg(c, org) {
			return huma.Error403Forbidden("access token is restricted to another organization or repository")
		}
	default:
		if !at.IsUserLevel() || !at.Allows(model.ScopeAdminWrite) {
			return huma.Error403Forbidden("global secrets need an unrestricted access token with admin:write")
		}
	}
	return nil
}

// authorizeOrgSecretAccess mirrors MustOrgMember(admin=true) semantics: the user
// must be an instance admin, own the org (personal org), or be an org admin as
// reported by the forge membership service.
func authorizeOrgSecretAccess(ctx context.Context, _store store.Store, user *model.User, orgID int64) error {
	org, err := _store.OrgGet(orgID)
	if err != nil {
		return huma.Error404NotFound("organization not found")
	}

	if user.Admin {
		return nil
	}

	// User can always manage their own (personal) org secrets.
	if session.IsPersonalOrgOwner(_store, user, org) {
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
	// Sharing makes the secret available to the target's pipelines, so the
	// caller needs the same authority over the target as over the owner.
	if err := authorizeSecretScope(ctx, _store, session.User(c), input.Body.OrgID, input.Body.RepoID); err != nil {
		return nil, err
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
	// The new owner must be a scope the caller could create the secret in;
	// moving to org_id=0, repo_id=0 makes it global and needs secrets:write.
	if err := authorizeSecretScope(ctx, _store, session.User(c), input.Body.OrgID, input.Body.RepoID); err != nil {
		return nil, err
	}

	secretService := server.Config.Services.Manager.SecretService()
	if err := secretService.SecretTransferOwner(input.SecretID, input.Body.OrgID, input.Body.RepoID, input.Body.KeepAsTarget); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error transferring secret ownership: %s", err))
	}

	return nil, nil
}
