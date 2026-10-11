// Copyright 2024 Woodpecker Authors
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

package model

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

// AccessTokenScope represents permission scopes for access tokens.
type AccessTokenScope string

const (
	// User scopes - available to all users.
	ScopeRepoRead  AccessTokenScope = "repo:read"
	ScopeRepoWrite AccessTokenScope = "repo:write"
	ScopeRepoAdmin AccessTokenScope = "repo:admin"
	ScopeUserRead  AccessTokenScope = "user:read"
	ScopeUserWrite AccessTokenScope = "user:write"

	// Admin-only scopes.
	ScopeAdminRead  AccessTokenScope = "admin:read"
	ScopeAdminWrite AccessTokenScope = "admin:write"
)

var (
	ErrAccessTokenNameInvalid   = errors.New("invalid access token name")
	ErrAccessTokenNameTooLong   = errors.New("access token name too long")
	ErrAccessTokenScopesInvalid = errors.New("invalid access token scopes")
	ErrAccessTokenScopesEmpty   = errors.New("at least one scope is required")
	ErrAccessTokenExpired       = errors.New("access token expired")
	ErrAccessTokenAdminScope    = errors.New("admin scopes require an instance role that grants them")
)

const maxAccessTokenNameLen = 100

// AllUserScopes returns all scopes available to regular users.
func AllUserScopes() []AccessTokenScope {
	return []AccessTokenScope{
		ScopeRepoRead,
		ScopeRepoWrite,
		ScopeRepoAdmin,
		ScopeUserRead,
		ScopeUserWrite,
	}
}

// AllAdminScopes returns all scopes available only to admins.
func AllAdminScopes() []AccessTokenScope {
	return []AccessTokenScope{
		ScopeAdminRead,
		ScopeAdminWrite,
	}
}

// AdminScopes returns the admin scopes the user may put on a token: admin:read
// with any admin permission, and admin:write with any permission that changes state.
func (u *User) AdminScopes() []AccessTokenScope {
	scopes := []AccessTokenScope{}
	if u.HasAnyReadPermission() || u.HasAnyWritePermission() {
		scopes = append(scopes, ScopeAdminRead)
	}
	if u.HasAnyWritePermission() {
		scopes = append(scopes, ScopeAdminWrite)
	}
	return scopes
}

// AllScopes returns all available scopes.
func AllScopes() []AccessTokenScope {
	return append(AllUserScopes(), AllAdminScopes()...)
}

// IsAdminScope returns true if the scope requires admin privileges.
func (s AccessTokenScope) IsAdminScope() bool {
	return s == ScopeAdminRead || s == ScopeAdminWrite
}

// IsValid returns true if the scope is a valid defined scope.
func (s AccessTokenScope) IsValid() bool {
	for _, valid := range AllScopes() {
		if s == valid {
			return true
		}
	}
	return false
}

// AccessToken represents a user-created API access token.
type AccessToken struct {
	bun.BaseModel `bun:"table:access_tokens"`

	ID        int64           `json:"id"         bun:"id,pk,autoincrement"`
	UserID    int64           `json:"user_id"    bun:"user_id,notnull"`
	OrgID     int64           `json:"org_id"     bun:"org_id,notnull,default:0"`
	RepoID    int64           `json:"repo_id"    bun:"repo_id,notnull,default:0"`
	Name      string          `json:"name"       bun:"name,notnull"`
	TokenHash string          `json:"-"          bun:"token_hash,unique,type:varchar(64)"`
	Scopes    JSONSliceString `json:"scopes"     bun:"scopes,type:text"`
	ExpiresAt int64           `json:"expires_at" bun:"expires_at"`
	LastUsed  int64           `json:"last_used"  bun:"last_used"`
	CreatedAt int64           `json:"created_at" bun:"created_at,nullzero"`
	UpdatedAt int64           `json:"updated_at" bun:"updated_at,nullzero"`
} //	@name	AccessToken

// TableName returns the database table name.
func (AccessToken) TableName() string {
	return "access_tokens"
}

// Validate validates the access token fields for a token owned by user.
func (at *AccessToken) Validate(user *User) error {
	// Validate name
	if strings.TrimSpace(at.Name) == "" {
		return ErrAccessTokenNameInvalid
	}
	if len(at.Name) > maxAccessTokenNameLen {
		return ErrAccessTokenNameTooLong
	}

	// Validate scopes
	if len(at.Scopes) == 0 {
		return ErrAccessTokenScopesEmpty
	}

	for _, scopeStr := range at.Scopes {
		scope := AccessTokenScope(scopeStr)
		if !scope.IsValid() {
			return fmt.Errorf("%w: %s", ErrAccessTokenScopesInvalid, scopeStr)
		}
		if scope.IsAdminScope() && !slices.Contains(user.AdminScopes(), scope) {
			return ErrAccessTokenAdminScope
		}
	}

	return nil
}

// GetScopes returns the scopes as AccessTokenScope slice.
func (at *AccessToken) GetScopes() []AccessTokenScope {
	scopes := make([]AccessTokenScope, len(at.Scopes))
	for i, s := range at.Scopes {
		scopes[i] = AccessTokenScope(s)
	}
	return scopes
}

// HasScope returns true if the token has the specified scope.
func (at *AccessToken) HasScope(required AccessTokenScope) bool {
	for _, s := range at.Scopes {
		if AccessTokenScope(s) == required {
			return true
		}
	}
	return false
}

// IsExpired returns true if the token has expired.
func (at *AccessToken) IsExpired() bool {
	if at.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() > at.ExpiresAt
}

// IsUserLevel returns true if the token is not scoped to an org or repo.
func (at *AccessToken) IsUserLevel() bool {
	return at.OrgID == 0 && at.RepoID == 0
}

// IsOrgScoped returns true if the token is scoped to a specific organization.
func (at *AccessToken) IsOrgScoped() bool {
	return at.OrgID != 0 && at.RepoID == 0
}

// IsRepoScoped returns true if the token is scoped to a specific repository.
func (at *AccessToken) IsRepoScoped() bool {
	return at.RepoID != 0
}

// Copy returns a copy of the access token without sensitive data.
func (at *AccessToken) Copy() *AccessToken {
	scopesCopy := make(JSONSliceString, len(at.Scopes))
	copy(scopesCopy, at.Scopes)

	return &AccessToken{
		ID:        at.ID,
		UserID:    at.UserID,
		OrgID:     at.OrgID,
		RepoID:    at.RepoID,
		Name:      at.Name,
		Scopes:    scopesCopy,
		ExpiresAt: at.ExpiresAt,
		LastUsed:  at.LastUsed,
		CreatedAt: at.CreatedAt,
		UpdatedAt: at.UpdatedAt,
		// TokenHash intentionally omitted
	}
}

// AccessTokenStore defines the interface for access token persistence.
type AccessTokenStore interface {
	AccessTokenCreate(*AccessToken) error
	AccessTokenFind(int64) (*AccessToken, error)
	AccessTokenFindByHash(string) (*AccessToken, error)
	AccessTokenList(userID int64, p *ListOptions) ([]*AccessToken, error)
	AccessTokenDelete(*AccessToken) error
	AccessTokenUpdateLastUsed(int64, int64) error
}
