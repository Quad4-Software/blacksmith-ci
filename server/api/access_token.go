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
	"encoding/base32"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tink-crypto/tink-go/v2/subtle/random"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/store"
	"codefloe.com/crowci/crow/v5/server/store/datastore"
	"codefloe.com/crowci/crow/v5/shared/token"
)

// CreateAccessTokenRequest is the request body for creating an access token.
type CreateAccessTokenRequest struct {
	Name      string   `json:"name"       binding:"required"`
	Scopes    []string `json:"scopes"     binding:"required,min=1"`
	OrgID     int64    `json:"org_id"`
	RepoID    int64    `json:"repo_id"`
	ExpiresAt int64    `json:"expires_at"`
}

// AccessTokenResponse is returned when creating a token (includes the plain token once).
type AccessTokenResponse struct {
	*model.AccessToken
	Token string `json:"token"` // Only populated on creation
}

// GetAccessTokenList
//
//	@Summary		List access tokens
//	@Description	Returns all access tokens for the authenticated user.
//	@Router			/user/access-tokens [get]
//	@Produce		json
//	@Success		200	{array}	AccessToken
//	@Tags			User
//	@Param			Authorization	header	string	true	"Insert your personal access token"				default(Bearer <personal access token>)
//	@Param			page			query	int		false	"for response pagination, page offset number"	default(1)
//	@Param			perPage			query	int		false	"for response pagination, max items per page"	default(50)
func GetAccessTokenList(c *gin.Context) {
	user := session.User(c)
	_store := store.FromContext(c)

	tokens, err := _store.AccessTokenList(user.ID, session.Pagination(c))
	if err != nil {
		c.String(http.StatusInternalServerError, "Error fetching access tokens. %s", err)
		return
	}

	// Return copies without sensitive data
	result := make([]*model.AccessToken, len(tokens))
	for i, t := range tokens {
		result[i] = t.Copy()
	}

	c.JSON(http.StatusOK, result)
}

// GetAccessToken
//
//	@Summary		Get an access token
//	@Description	Returns a single access token by ID.
//	@Router			/user/access-tokens/{token_id} [get]
//	@Produce		json
//	@Success		200	{object}	AccessToken
//	@Tags			User
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			token_id		path	int		true	"the token's id"
func GetAccessToken(c *gin.Context) {
	user := session.User(c)
	_store := store.FromContext(c)

	tokenID, err := strconv.ParseInt(c.Param("token_id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid token ID")
		return
	}

	accessToken, err := _store.AccessTokenFind(tokenID)
	if err != nil {
		handleDBError(c, err)
		return
	}

	// Ensure user owns this token
	if accessToken.UserID != user.ID {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, accessToken.Copy())
}

// PostAccessToken
//
//	@Summary		Create an access token
//	@Description	Creates a new personal access token. The token value is only returned once.
//	@Router			/user/access-tokens [post]
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	AccessTokenResponse
//	@Tags			User
//	@Param			Authorization	header	string						true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			token			body	CreateAccessTokenRequest	true	"Token data"
func PostAccessToken(c *gin.Context) {
	user := session.User(c)
	_store := store.FromContext(c)

	var req CreateAccessTokenRequest
	if err := c.BindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "Error parsing request. %s", err)
		return
	}

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
		c.String(http.StatusBadRequest, "Validation error. %s", err)
		return
	}

	// Validate entity scoping permissions
	if err := validateAccessTokenEntityScope(c, user, accessToken); err != nil {
		c.String(http.StatusForbidden, "Permission denied. %s", err)
		return
	}

	// Create in database
	if err := _store.AccessTokenCreate(accessToken); err != nil {
		c.String(http.StatusInternalServerError, "Error creating access token. %s", err)
		return
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
		c.String(http.StatusInternalServerError, "Error signing token. %s", err)
		return
	}

	// Return response with the JWT (only time it's shown)
	c.JSON(http.StatusOK, AccessTokenResponse{
		AccessToken: accessToken.Copy(),
		Token:       signedToken,
	})
}

// DeleteAccessToken
//
//	@Summary		Delete an access token
//	@Description	Deletes an access token by ID.
//	@Router			/user/access-tokens/{token_id} [delete]
//	@Produce		plain
//	@Success		204
//	@Tags			User
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			token_id		path	int		true	"the token's id"
func DeleteAccessToken(c *gin.Context) {
	user := session.User(c)
	_store := store.FromContext(c)

	tokenID, err := strconv.ParseInt(c.Param("token_id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid token ID")
		return
	}

	accessToken, err := _store.AccessTokenFind(tokenID)
	if err != nil {
		handleDBError(c, err)
		return
	}

	// Ensure user owns this token
	if accessToken.UserID != user.ID {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if err := _store.AccessTokenDelete(accessToken); err != nil {
		c.String(http.StatusInternalServerError, "Error deleting access token. %s", err)
		return
	}

	c.Status(http.StatusNoContent)
}

// UpdateAccessTokenRequest is the request body for updating an access token.
type UpdateAccessTokenRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

// PatchAccessToken
//
//	@Summary		Update an access token
//	@Description	Updates an access token's name and/or scopes.
//	@Router			/user/access-tokens/{token_id} [patch]
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	AccessToken
//	@Tags			User
//	@Param			Authorization	header	string						true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			token_id		path	int							true	"the token's id"
//	@Param			token			body	UpdateAccessTokenRequest	true	"Token update data"
func PatchAccessToken(c *gin.Context) {
	user := session.User(c)
	_store := store.FromContext(c)

	tokenID, err := strconv.ParseInt(c.Param("token_id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid token ID")
		return
	}

	accessToken, err := _store.AccessTokenFind(tokenID)
	if err != nil {
		handleDBError(c, err)
		return
	}

	// Ensure user owns this token
	if accessToken.UserID != user.ID {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	var req UpdateAccessTokenRequest
	if err := c.BindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "Error parsing request. %s", err)
		return
	}

	// Update fields if provided
	if req.Name != "" {
		accessToken.Name = req.Name
	}
	if len(req.Scopes) > 0 {
		accessToken.Scopes = req.Scopes
	}

	// Validate the updated token
	if err := accessToken.Validate(user.Admin); err != nil {
		c.String(http.StatusBadRequest, "Validation error. %s", err)
		return
	}

	// Update in database
	if err := _store.AccessTokenUpdate(accessToken); err != nil {
		c.String(http.StatusInternalServerError, "Error updating access token. %s", err)
		return
	}

	c.JSON(http.StatusOK, accessToken.Copy())
}

// DeleteAccessTokenBatchRequest is the request body for batch deleting access tokens.
type DeleteAccessTokenBatchRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

// DeleteAccessTokenBatch
//
//	@Summary		Delete multiple access tokens
//	@Description	Deletes multiple access tokens by their IDs.
//	@Router			/user/access-tokens/batch [delete]
//	@Accept			json
//	@Produce		plain
//	@Success		204
//	@Tags			User
//	@Param			Authorization	header	string							true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			ids				body	DeleteAccessTokenBatchRequest	true	"Token IDs to delete"
func DeleteAccessTokenBatch(c *gin.Context) {
	user := session.User(c)
	_store := store.FromContext(c)

	var req DeleteAccessTokenBatchRequest
	if err := c.BindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "Error parsing request. %s", err)
		return
	}

	// Verify ownership and delete each token
	for _, tokenID := range req.IDs {
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
			c.String(http.StatusInternalServerError, "Error deleting access token %d. %s", tokenID, err)
			return
		}
	}

	c.Status(http.StatusNoContent)
}

// GetAvailableScopes
//
//	@Summary		Get available scopes
//	@Description	Returns the available scopes for access tokens based on user role.
//	@Router			/user/access-tokens/scopes [get]
//	@Produce		json
//	@Success		200	{object}	map[string][]string
//	@Tags			User
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
func GetAvailableScopes(c *gin.Context) {
	user := session.User(c)

	scopes := make(map[string][]model.AccessTokenScope)
	scopes["user"] = model.AllUserScopes()

	if user.Admin {
		scopes["admin"] = model.AllAdminScopes()
	}

	c.JSON(http.StatusOK, scopes)
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
