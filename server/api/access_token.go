// Copyright 2025 Crow Authors
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
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/tink-crypto/tink-go/v2/subtle/random"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/datastore"
	"codefloe.com/crowci/crow/v6/shared/token"
)

// CreateAccessTokenRequest is the request body for creating an access token.
type CreateAccessTokenRequest struct {
	Name      string   `json:"name" minLength:"1" doc:"token name"`
	Scopes    []string `json:"scopes" minItems:"1" doc:"token scopes"`
	OrgID     int64    `json:"org_id,omitempty" doc:"optional org scope"`
	RepoID    int64    `json:"repo_id,omitempty" doc:"optional repo scope"`
	ExpiresAt int64    `json:"expires_at,omitempty" doc:"optional expiry timestamp"`
}

// AccessTokenFields is a method-free copy of model.AccessToken for clean huma schema generation.
type AccessTokenFields model.AccessToken

// AccessTokenResponse is returned when creating a token (includes the plain token once).
type AccessTokenResponse struct {
	AccessTokenFields
	Token string `json:"token"` // Only populated on creation
}

type GetAccessTokenListInput struct {
	PaginationParams
}

type GetAccessTokenListOutput struct {
	Body []*model.AccessToken
}

func GetAccessTokenList(ctx context.Context, input *GetAccessTokenListInput) (*GetAccessTokenListOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)
	_store := store.FromContext(c)

	tokens, err := _store.AccessTokenList(user.ID, &model.ListOptions{
		Page:    input.Page,
		PerPage: input.PerPage,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("Error fetching access tokens", err)
	}

	// Return copies without sensitive data
	result := make([]*model.AccessToken, len(tokens))
	for i, t := range tokens {
		result[i] = t.Copy()
	}

	return &GetAccessTokenListOutput{Body: result}, nil
}

type GetAccessTokenInput struct {
	TokenID int64 `path:"token_id" doc:"the token's id"`
}

type GetAccessTokenOutput struct {
	Body *model.AccessToken
}

func GetAccessToken(ctx context.Context, input *GetAccessTokenInput) (*GetAccessTokenOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)
	_store := store.FromContext(c)

	accessToken, err := _store.AccessTokenFind(input.TokenID)
	if err != nil {
		return nil, dbErr(err)
	}

	// Ensure user owns this token
	if accessToken.UserID != user.ID {
		return nil, huma.Error404NotFound("not found")
	}

	return &GetAccessTokenOutput{Body: accessToken.Copy()}, nil
}

type PostAccessTokenInput struct {
	Body CreateAccessTokenRequest
}

type PostAccessTokenOutput struct {
	Body AccessTokenResponse
}

func PostAccessToken(ctx context.Context, input *PostAccessTokenInput) (*PostAccessTokenOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)
	_store := store.FromContext(c)

	req := input.Body

	// Generate a random token
	plainToken := base32.StdEncoding.EncodeToString(random.GetRandomBytes(32))
	tokenHash := datastore.HashAccessToken(plainToken)

	// Create the access token model
	accessToken := &model.AccessToken{
		UserID:    user.ID,
		Name:      req.Name,
		TokenHash: tokenHash,
		Scopes:    req.Scopes,
		OrgID:     req.OrgID,
		RepoID:    req.RepoID,
		ExpiresAt: req.ExpiresAt,
		CreatedAt: time.Now().Unix(),
	}

	// Validate (checks name, scopes, and admin scope permissions)
	if err := accessToken.Validate(user.Admin); err != nil {
		return nil, huma.Error400BadRequest("Validation error", err)
	}

	// Validate entity scoping permissions
	if err := validateAccessTokenEntityScope(c, user, accessToken); err != nil {
		return nil, huma.Error403Forbidden("Permission denied", err)
	}

	// Create in database
	if err := _store.AccessTokenCreate(accessToken); err != nil {
		return nil, huma.Error500InternalServerError("Error creating access token", err)
	}

	// Create a JWT containing the token hash for authentication
	jwtToken := token.New(token.AccessToken)
	jwtToken.Set("token-id", strconv.FormatInt(accessToken.ID, 10))
	jwtToken.Set("token-hash", tokenHash)

	var signedToken string
	var err error
	if accessToken.ExpiresAt > 0 {
		signedToken, err = jwtToken.SignExpires(user.Hash, accessToken.ExpiresAt)
	} else {
		signedToken, err = jwtToken.Sign(user.Hash)
	}
	if err != nil {
		// Clean up the created token on signing failure
		_ = _store.AccessTokenDelete(accessToken)
		return nil, huma.Error500InternalServerError("Error signing token", err)
	}

	// Return response with the JWT (only time it's shown)
	return &PostAccessTokenOutput{Body: AccessTokenResponse{
		AccessTokenFields: AccessTokenFields(*accessToken.Copy()),
		Token:             signedToken,
	}}, nil
}

type DeleteAccessTokenInput struct {
	TokenID int64 `path:"token_id" doc:"the token's id"`
}

func DeleteAccessToken(ctx context.Context, input *DeleteAccessTokenInput) (*struct{}, error) {
	c := ginCtx(ctx)
	user := session.User(c)
	_store := store.FromContext(c)

	accessToken, err := _store.AccessTokenFind(input.TokenID)
	if err != nil {
		return nil, dbErr(err)
	}

	// Ensure user owns this token
	if accessToken.UserID != user.ID {
		return nil, huma.Error404NotFound("not found")
	}

	if err := _store.AccessTokenDelete(accessToken); err != nil {
		return nil, huma.Error500InternalServerError("Error deleting access token", err)
	}

	return nil, nil
}

// UpdateAccessTokenRequest is the request body for updating an access token.
type UpdateAccessTokenRequest struct {
	Name   string   `json:"name,omitempty" doc:"token name"`
	Scopes []string `json:"scopes,omitempty" doc:"token scopes"`
}

type PatchAccessTokenInput struct {
	TokenID int64 `path:"token_id" doc:"the token's id"`
	Body    UpdateAccessTokenRequest
}

type PatchAccessTokenOutput struct {
	Body *model.AccessToken
}

func PatchAccessToken(ctx context.Context, input *PatchAccessTokenInput) (*PatchAccessTokenOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)
	_store := store.FromContext(c)

	accessToken, err := _store.AccessTokenFind(input.TokenID)
	if err != nil {
		return nil, dbErr(err)
	}

	// Ensure user owns this token
	if accessToken.UserID != user.ID {
		return nil, huma.Error404NotFound("not found")
	}

	req := input.Body

	// Update fields if provided
	if req.Name != "" {
		accessToken.Name = req.Name
	}
	if len(req.Scopes) > 0 {
		accessToken.Scopes = req.Scopes
	}

	// Validate the updated token
	if err := accessToken.Validate(user.Admin); err != nil {
		return nil, huma.Error400BadRequest("Validation error", err)
	}

	// Update in database
	if err := _store.AccessTokenUpdate(accessToken); err != nil {
		return nil, huma.Error500InternalServerError("Error updating access token", err)
	}

	return &PatchAccessTokenOutput{Body: accessToken.Copy()}, nil
}

// DeleteAccessTokenBatchRequest is the request body for batch deleting access tokens.
type DeleteAccessTokenBatchRequest struct {
	IDs []int64 `json:"ids" minItems:"1" doc:"token IDs to delete"`
}

type DeleteAccessTokenBatchInput struct {
	Body DeleteAccessTokenBatchRequest
}

func DeleteAccessTokenBatch(ctx context.Context, input *DeleteAccessTokenBatchInput) (*struct{}, error) {
	c := ginCtx(ctx)
	user := session.User(c)
	_store := store.FromContext(c)

	// Verify ownership and delete each token
	for _, tokenID := range input.Body.IDs {
		accessToken, err := _store.AccessTokenFind(tokenID)
		if err != nil {
			// Skip tokens that don't exist
			continue
		}

		// Only delete tokens owned by the user
		if accessToken.UserID != user.ID {
			continue
		}

		if err := _store.AccessTokenDelete(accessToken); err != nil {
			return nil, huma.Error500InternalServerError("Error deleting access token", err)
		}
	}

	return nil, nil
}

type GetAvailableScopesInput struct{}

type GetAvailableScopesOutput struct {
	Body map[string][]model.AccessTokenScope
}

func GetAvailableScopes(ctx context.Context, _ *GetAvailableScopesInput) (*GetAvailableScopesOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)

	scopes := make(map[string][]model.AccessTokenScope)
	scopes["user"] = model.AllUserScopes()

	if user.Admin {
		scopes["admin"] = model.AllAdminScopes()
	}

	return &GetAvailableScopesOutput{Body: scopes}, nil
}

// validateAccessTokenEntityScope validates that the user has permission for the specified entity scope.
func validateAccessTokenEntityScope(c *gin.Context, user *model.User, accessToken *model.AccessToken) error {
	_store := store.FromContext(c)

	// User-level token - no additional validation needed
	if accessToken.IsUserLevel() {
		return nil
	}

	// Org-scoped token
	if accessToken.IsOrgScoped() {
		// Check if user is an admin of this org
		org, err := _store.OrgGet(accessToken.OrgID)
		if err != nil {
			return err
		}

		// Personal org - user must own it
		if org.IsUser {
			if user.OrgID != org.ID {
				return model.ErrAccessTokenAdminScope
			}
			return nil
		}

		// For non-personal orgs, we need to check org membership
		// This would require calling the forge API, so for now we allow if user is system admin
		// or if the org is the user's personal org
		if !user.Admin && user.OrgID != org.ID {
			return model.ErrAccessTokenAdminScope
		}

		return nil
	}

	// Repo-scoped token
	if accessToken.IsRepoScoped() {
		repo, err := _store.GetRepo(accessToken.RepoID)
		if err != nil {
			return err
		}

		// Check user has at least push permissions on the repo
		perm, err := _store.PermFind(user, repo)
		if err != nil {
			return err
		}

		if !perm.Push {
			return model.ErrAccessTokenAdminScope
		}

		return nil
	}

	return nil
}
