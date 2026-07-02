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
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/store"
)

type GetOrgsInput struct {
	PaginationParams
}

type GetOrgsOutput struct {
	Body []*model.Org
}

func GetOrgs(ctx context.Context, input *GetOrgsInput) (*GetOrgsOutput, error) {
	c := ginCtx(ctx)
	orgs, err := store.FromContext(c).OrgList(&model.ListOptions{
		Page:    input.Page,
		PerPage: input.PerPage,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("Error getting user list", err)
	}
	return &GetOrgsOutput{Body: orgs}, nil
}

type GetOrgInput struct {
	OrgID int64 `path:"org_id" doc:"the organization's id"`
}

type GetOrgOutput struct {
	Body *model.Org
}

func GetOrg(ctx context.Context, input *GetOrgInput) (*GetOrgOutput, error) {
	c := ginCtx(ctx)
	org := session.Org(c)
	return &GetOrgOutput{Body: org}, nil
}

type GetOrgPermissionsInput struct {
	OrgID int64 `path:"org_id" doc:"the organization's id"`
}

type GetOrgPermissionsOutput struct {
	Body *model.OrgPerm
}

func GetOrgPermissions(ctx context.Context, input *GetOrgPermissionsInput) (*GetOrgPermissionsOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)
	org := session.Org(c)

	if user == nil {
		return &GetOrgPermissionsOutput{Body: &model.OrgPerm{}}, nil
	}

	if (org.IsUser && org.Name == user.Login) || (user.Admin && !org.IsUser) {
		return &GetOrgPermissionsOutput{Body: &model.OrgPerm{
			Member: true,
			Admin:  true,
		}}, nil
	} else if org.IsUser {
		return &GetOrgPermissionsOutput{Body: &model.OrgPerm{}}, nil
	}

	// Get forge by org's forge ID (not user's default forge)
	_forge, err := server.Config.Services.Manager.ForgeByID(org.ForgeID)
	if err != nil {
		log.Error().Err(err).Int64("forge_id", org.ForgeID).Msg("Cannot get forge for org")
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

	perm, err := server.Config.Services.Membership.Get(c, _forge, forgeUser, org.Name)
	if err != nil {
		return nil, huma.Error500InternalServerError("Error getting membership", err)
	}

	return &GetOrgPermissionsOutput{Body: perm}, nil
}

type LookupOrgInput struct {
	OrgFullName string `path:"org_full_name" doc:"the organization's full name / slug"`
}

type LookupOrgOutput struct {
	Body *model.Org
}

func LookupOrg(ctx context.Context, input *LookupOrgInput) (*LookupOrgOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)
	_forge, err := server.Config.Services.Manager.ForgeFromUser(user)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get forge from user")
		return nil, huma.Error500InternalServerError("Cannot get forge from user")
	}

	orgFullName := strings.TrimLeft(input.OrgFullName, "/")

	org, err := _store.OrgFindByName(orgFullName, user.ForgeID)
	if err != nil {
		return nil, dbErr(err)
	}

	// don't leak private org infos
	if org.Private {
		user := session.User(c)
		if user == nil {
			return nil, huma.Error404NotFound("not found")
		}

		if !user.Admin && org.Name != user.Login {
			return nil, huma.Error404NotFound("not found")
		} else if !user.Admin {
			// Build a user with the latest forge credentials to avoid using
			// the deprecated/stale users.access_token when calling the forge.
			forgeUser := user
			if uf, ufErr := _store.UserForgeGet(user.ID, org.ForgeID); ufErr == nil && uf != nil {
				forgeUserCopy := *user
				forgeUserCopy.ForgeID = org.ForgeID
				forgeUserCopy.AccessToken = uf.AccessToken
				forgeUserCopy.RefreshToken = uf.RefreshToken
				forgeUserCopy.Expiry = uf.Expiry
				forgeUser = &forgeUserCopy
			}

			perm, err := server.Config.Services.Membership.Get(c, _forge, forgeUser, org.Name)
			if err != nil {
				log.Error().Err(err).Msg("failed to check membership")
				return nil, huma.Error500InternalServerError("failed to check membership")
			}

			if perm == nil || !perm.Member {
				return nil, huma.Error404NotFound("not found")
			}
		}
	}

	return &LookupOrgOutput{Body: org}, nil
}

// LookupOrgGin is a gin handler wrapper for LookupOrg (needed because the
// route uses a wildcard path param that huma doesn't support).
func LookupOrgGin(c *gin.Context) {
	_store := store.FromContext(c)
	user := session.User(c)
	// This route has no auth middleware, so the session user may be nil.
	// ForgeFromUser and the private-org check below both dereference it.
	if user == nil {
		c.String(http.StatusUnauthorized, "authentication required")
		return
	}
	_forge, err := server.Config.Services.Manager.ForgeFromUser(user)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get forge from user")
		c.String(http.StatusInternalServerError, "Cannot get forge from user")
		return
	}

	orgFullName := strings.TrimLeft(c.Param("org_full_name"), "/")

	org, err := _store.OrgFindByName(orgFullName, user.ForgeID)
	if err != nil {
		handleDBError(c, err)
		return
	}

	if org.Private {
		if !user.Admin && org.Name != user.Login {
			c.String(http.StatusNotFound, "not found")
			return
		} else if !user.Admin {
			// Build a user with the latest forge credentials to avoid using
			// the deprecated/stale users.access_token when calling the forge.
			forgeUser := user
			if uf, ufErr := _store.UserForgeGet(user.ID, org.ForgeID); ufErr == nil && uf != nil {
				forgeUserCopy := *user
				forgeUserCopy.ForgeID = org.ForgeID
				forgeUserCopy.AccessToken = uf.AccessToken
				forgeUserCopy.RefreshToken = uf.RefreshToken
				forgeUserCopy.Expiry = uf.Expiry
				forgeUser = &forgeUserCopy
			}

			perm, err := server.Config.Services.Membership.Get(c, _forge, forgeUser, org.Name)
			if err != nil {
				log.Error().Err(err).Msg("failed to check membership")
				c.String(http.StatusInternalServerError, "failed to check membership")
				return
			}

			if perm == nil || !perm.Member {
				c.String(http.StatusNotFound, "not found")
				return
			}
		}
	}

	c.JSON(http.StatusOK, org)
}

type DeleteOrgInput struct {
	OrgID int64 `path:"org_id" doc:"the org's id"`
}

func DeleteOrg(ctx context.Context, input *DeleteOrgInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	org := session.Org(c)

	if err := _store.OrgDelete(org.ID); err != nil {
		return nil, dbErr(err)
	}

	return nil, nil
}
