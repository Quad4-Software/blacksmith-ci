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
	"strings"
	"time"
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
	ErrAccessTokenAdminScope    = errors.New("admin scopes not allowed for non-admin users")
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
	ID        int64    `json:"id"         xorm:"pk autoincr 'id'"`
	UserID    int64    `json:"user_id"    xorm:"NOT NULL INDEX 'user_id'"`
	OrgID     int64    `json:"org_id"     xorm:"NOT NULL DEFAULT 0 INDEX 'org_id'"`  // 0 = user-level
	RepoID    int64    `json:"repo_id"    xorm:"NOT NULL DEFAULT 0 INDEX 'repo_id'"` // 0 = not repo-scoped
	Name      string   `json:"name"       xorm:"NOT NULL 'name'"`
	TokenHash string   `json:"-"          xorm:"UNIQUE VARCHAR(64) 'token_hash'"`
	Scopes    []string `json:"scopes"     xorm:"json 'scopes'"`
	ExpiresAt int64    `json:"expires_at" xorm:"'expires_at'"` // unix timestamp, 0 = never
	LastUsed  int64    `json:"last_used"  xorm:"'last_used'"`
	CreatedAt int64    `json:"created_at" xorm:"'created_at' created"`
	UpdatedAt int64    `json:"updated_at" xorm:"'updated_at' updated"`
} //	@name	AccessToken

// TableName returns database table name for xorm.
func (AccessToken) TableName() string {
	return "access_tokens"
}

// Validate validates the access token fields.
func (at *AccessToken) Validate(isAdmin bool) error {
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
		if scope.IsAdminScope() && !isAdmin {
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
	scopesCopy := make([]string, len(at.Scopes))
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
