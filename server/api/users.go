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
	"encoding/base32"
	"fmt"
	"slices"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"
	"github.com/tink-crypto/tink-go/v2/subtle/random"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/store"
)

// The managed roles are the ones the identity providers set at sign-in.
func managedRoles() []string {
	roles := []string{}
	for _, p := range identityProviders() {
		roles = append(roles, p.ManagedRoles()...)
	}
	slices.Sort(roles)
	return slices.Compact(roles)
}

// ManagedRoles returns the roles set from identity provider groups, sorted and
// never nil; the web UI cannot assign them.
func ManagedRoles() []string { return managedRoles() }

// Assignable roles are the requested roles after validation. The request is
// authoritative, except that a role the identity provider manages can only be
// kept or removed: adding one the user does not already hold is rejected, so
// that an admin can still revoke a role assigned by hand before it was mapped.
func assignableRoles(current model.JSONSliceString, requested []string) (model.JSONSliceString, error) {
	roles, err := model.ParseRoles(requested)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	managed := managedRoles()
	for _, r := range roles {
		if slices.Contains(managed, string(r)) && !slices.Contains(current, string(r)) {
			return nil, huma.Error400BadRequest(fmt.Sprintf("role %s is managed by the identity provider", r))
		}
	}
	return model.RolesToStrings(roles), nil
}

type GetUsersInput struct {
	PaginationParams
}

type GetUsersOutput struct {
	Body []*model.User
}

func GetUsers(ctx context.Context, input *GetUsersInput) (*GetUsersOutput, error) {
	c := ginCtx(ctx)
	users, err := store.FromContext(c).GetUserList(&model.ListOptions{
		Page:    input.Page,
		PerPage: input.PerPage,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("Error getting user list", err)
	}
	return &GetUsersOutput{Body: users}, nil
}

type GetUserInput struct {
	Login string `path:"login" doc:"the user's login name"`
}

type GetUserOutput struct {
	Body *model.User
}

func GetUser(ctx context.Context, input *GetUserInput) (*GetUserOutput, error) {
	c := ginCtx(ctx)
	user, err := store.FromContext(c).GetUserLogin(input.Login)
	if err != nil {
		return nil, dbErr(err)
	}
	return &GetUserOutput{Body: user}, nil
}

type PatchUserBody struct {
	Email       string    `json:"email,omitempty" doc:"user email"`
	Avatar      string    `json:"avatar_url,omitempty" doc:"user avatar URL"`
	Admin       bool      `json:"admin,omitempty" doc:"whether user is admin"`
	Description string    `json:"description,omitempty" doc:"user description"`
	Timezone    string    `json:"timezone,omitempty" doc:"user timezone"`
	Roles       *[]string `json:"roles,omitempty" doc:"instance roles besides admin; omit to keep the current roles"`
}

type PatchUserInput struct {
	Login string `path:"login" doc:"the user's login name"`
	Body  PatchUserBody
}

type PatchUserOutput struct {
	Body *model.User
}

func PatchUser(ctx context.Context, input *PatchUserInput) (*PatchUserOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	user, err := _store.GetUserLogin(input.Login)
	if err != nil {
		return nil, dbErr(err)
	}

	// TODO: allow to change login (currently used as primary key)
	// TODO: disallow to change login, email, avatar if the user is using oauth
	user.Email = input.Body.Email
	user.Avatar = input.Body.Avatar
	user.Admin = input.Body.Admin
	user.Description = input.Body.Description
	user.Timezone = input.Body.Timezone

	if input.Body.Roles != nil {
		roles, err := assignableRoles(user.Roles, *input.Body.Roles)
		if err != nil {
			return nil, err
		}
		if !slices.Equal(roles, user.Roles) {
			by := "unknown"
			if admin := session.User(c); admin != nil {
				by = admin.Login
			}
			log.Info().Str("old", strings.Join(user.Roles, ",")).Str("new", strings.Join(roles, ",")).Msgf("roles of %s changed by %s", user.Login, by)
		}
		user.Roles = roles
	}

	err = _store.UpdateUser(user)
	if err != nil {
		return nil, huma.Error409Conflict("conflict updating user")
	}

	return &PatchUserOutput{Body: user}, nil
}

type PostUserBody struct {
	Login  string   `json:"login" minLength:"1" doc:"user login name"`
	Email  string   `json:"email" doc:"user email"`
	Avatar string   `json:"avatar_url,omitempty" doc:"user avatar URL"`
	Roles  []string `json:"roles,omitempty" doc:"instance roles besides admin"`
}

type PostUserInput struct {
	Body PostUserBody
}

type PostUserOutput struct {
	Body *model.User
}

func PostUser(ctx context.Context, input *PostUserInput) (*PostUserOutput, error) {
	c := ginCtx(ctx)
	user := &model.User{
		Login:  input.Body.Login,
		Email:  input.Body.Email,
		Avatar: input.Body.Avatar,
		Hash: base32.StdEncoding.EncodeToString(
			random.GetRandomBytes(32),
		),
		ForgeID:       1,                        // TODO: replace with forge id when multiple forges are supported
		ForgeRemoteID: model.ForgeRemoteID("0"), // TODO: search for the user in the forge and get the remote id
	}
	roles, err := assignableRoles(nil, input.Body.Roles)
	if err != nil {
		return nil, err
	}
	user.Roles = roles
	if err := user.Validate(); err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	if err := store.FromContext(c).CreateUser(user); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return &PostUserOutput{Body: user}, nil
}

type DeleteUserInput struct {
	Login string `path:"login" doc:"the user's login name"`
}

func DeleteUser(ctx context.Context, input *DeleteUserInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	user, err := _store.GetUserLogin(input.Login)
	if err != nil {
		return nil, dbErr(err)
	}
	if err = _store.DeleteUser(user); err != nil {
		return nil, dbErr(err)
	}
	return nil, nil
}
