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

	"github.com/danielgtaylor/huma/v2"
	"github.com/tink-crypto/tink-go/v2/subtle/random"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store"
)

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

type PatchUserInput struct {
	Login string `path:"login" doc:"the user's login name"`
	Body  model.User
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

	err = _store.UpdateUser(user)
	if err != nil {
		return nil, huma.Error409Conflict("conflict updating user")
	}

	return &PatchUserOutput{Body: user}, nil
}

type PostUserInput struct {
	Body model.User
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
