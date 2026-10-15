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

	"github.com/danielgtaylor/huma/v2"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/store"
)

type GetUserIdentitiesInput struct{}

type GetUserIdentitiesOutput struct {
	Body []*model.UserIdentity
}

// GetUserIdentities lists the identity provider accounts linked to the current user.
func GetUserIdentities(ctx context.Context, _ *GetUserIdentitiesInput) (*GetUserIdentitiesOutput, error) {
	c := ginCtx(ctx)
	identities, err := store.FromContext(c).UserIdentityList(session.User(c).ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Error fetching identities", err)
	}
	return &GetUserIdentitiesOutput{Body: identities}, nil
}

type DeleteUserIdentityInput struct {
	ID int64 `path:"identity_id" doc:"the identity's id"`
}

type DeleteUserIdentityOutput struct{}

// DeleteUserIdentity unlinks an identity provider account from the current user.
func DeleteUserIdentity(ctx context.Context, input *DeleteUserIdentityInput) (*DeleteUserIdentityOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)
	identities, err := _store.UserIdentityList(user.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Error fetching identities", err)
	}

	var target *model.UserIdentity
	for _, identity := range identities {
		if identity.ID == input.ID {
			target = identity
			break
		}
	}
	if target == nil {
		return nil, huma.Error404NotFound("identity not found")
	}
	if len(identities) == 1 {
		// Unlinking the last identity must leave the user a way to sign in.
		if forgeSignInDisabled() {
			return nil, huma.Error409Conflict("Cannot unlink the last identity while forge sign-in is disabled")
		}
		if user.ForgeRemoteID == "" {
			forges, err := _store.UserForgeList(user.ID)
			if err != nil {
				return nil, huma.Error500InternalServerError("Error fetching forges", err)
			}
			if len(forges) == 0 {
				return nil, huma.Error409Conflict("Link a forge account before unlinking your last identity provider account")
			}
		}
	}

	if err := _store.UserIdentityDelete(target); err != nil {
		return nil, huma.Error500InternalServerError("Error unlinking identity", err)
	}
	return &DeleteUserIdentityOutput{}, nil
}
