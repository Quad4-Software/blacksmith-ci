// Copyright 2026 Crow Authors
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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccessTokenValidateAdminScopesByRole(t *testing.T) {
	read := &AccessToken{Name: "t", Scopes: JSONSliceString{"admin:read"}}
	write := &AccessToken{Name: "t", Scopes: JSONSliceString{"admin:write"}}

	plain := &User{}
	auditor := &User{Roles: JSONSliceString{"auditor"}}
	secrets := &User{Roles: JSONSliceString{"secrets-manager"}}
	admin := &User{Admin: true}

	assert.ErrorIs(t, read.Validate(plain), ErrAccessTokenAdminScope)
	assert.NoError(t, read.Validate(auditor))
	assert.ErrorIs(t, write.Validate(auditor), ErrAccessTokenAdminScope)
	assert.NoError(t, write.Validate(secrets))
	assert.NoError(t, write.Validate(admin))
}

func TestUserAdminScopes(t *testing.T) {
	assert.Equal(t, []AccessTokenScope{}, (&User{}).AdminScopes())
	assert.Equal(t, []AccessTokenScope{ScopeAdminRead}, (&User{Roles: JSONSliceString{"auditor"}}).AdminScopes())
	assert.Equal(t, []AccessTokenScope{ScopeAdminRead, ScopeAdminWrite}, (&User{Roles: JSONSliceString{"secrets-manager"}}).AdminScopes())
	assert.Equal(t, []AccessTokenScope{ScopeAdminRead, ScopeAdminWrite}, (&User{Roles: JSONSliceString{"operator"}}).AdminScopes())
	assert.Equal(t, []AccessTokenScope{ScopeAdminRead, ScopeAdminWrite}, (&User{Admin: true}).AdminScopes())
}

func TestAccessTokenValidateBasics(t *testing.T) {
	user := &User{}
	assert.ErrorIs(t, (&AccessToken{Name: " ", Scopes: JSONSliceString{"repo:read"}}).Validate(user), ErrAccessTokenNameInvalid)
	assert.ErrorIs(t, (&AccessToken{Name: "t"}).Validate(user), ErrAccessTokenScopesEmpty)
	assert.ErrorIs(t, (&AccessToken{Name: "t", Scopes: JSONSliceString{"nope"}}).Validate(user), ErrAccessTokenScopesInvalid)
	assert.NoError(t, (&AccessToken{Name: "t", Scopes: JSONSliceString{"repo:read"}}).Validate(user))
}

func TestAccessTokenScopeSatisfies(t *testing.T) {
	tests := []struct {
		have, need AccessTokenScope
		want       bool
	}{
		{ScopeRepoRead, ScopeRepoRead, true},
		{ScopeRepoWrite, ScopeRepoRead, true},
		{ScopeRepoAdmin, ScopeRepoRead, true},
		{ScopeRepoAdmin, ScopeRepoWrite, true},
		{ScopeRepoRead, ScopeRepoWrite, false},
		{ScopeRepoWrite, ScopeRepoAdmin, false},
		{ScopeUserWrite, ScopeUserRead, true},
		{ScopeUserRead, ScopeUserWrite, false},
		{ScopeAdminWrite, ScopeAdminRead, true},
		{ScopeAdminRead, ScopeAdminWrite, false},
		// Families never imply each other.
		{ScopeAdminWrite, ScopeRepoRead, false},
		{ScopeRepoAdmin, ScopeUserRead, false},
		{ScopeUserWrite, ScopeAdminRead, false},
		{AccessTokenScope("bogus"), ScopeRepoRead, false},
		{ScopeRepoAdmin, AccessTokenScope("bogus"), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.have)+">"+string(tt.need), func(t *testing.T) {
			assert.Equal(t, tt.want, tt.have.Satisfies(tt.need))
		})
	}
}

func TestAccessTokenAllows(t *testing.T) {
	at := &AccessToken{Scopes: JSONSliceString{"repo:write", "user:read"}}
	assert.True(t, at.Allows(ScopeRepoRead))
	assert.True(t, at.Allows(ScopeRepoWrite))
	assert.False(t, at.Allows(ScopeRepoAdmin))
	assert.True(t, at.Allows(ScopeUserRead))
	assert.False(t, at.Allows(ScopeUserWrite))
	assert.False(t, at.Allows(ScopeAdminRead))
}

func TestAccessTokenAllowsRepo(t *testing.T) {
	repo := &Repo{ID: 5, OrgID: 2}
	tests := []struct {
		name  string
		token AccessToken
		repo  *Repo
		want  bool
	}{
		{"user level", AccessToken{}, repo, true},
		{"same repo", AccessToken{RepoID: 5}, repo, true},
		{"other repo", AccessToken{RepoID: 6}, repo, false},
		{"same org", AccessToken{OrgID: 2}, repo, true},
		{"other org", AccessToken{OrgID: 3}, repo, false},
		{"repo and org match", AccessToken{OrgID: 2, RepoID: 5}, repo, true},
		{"repo matches, org does not", AccessToken{OrgID: 3, RepoID: 5}, repo, false},
		{"no repo, user level", AccessToken{}, nil, true},
		{"no repo, restricted", AccessToken{RepoID: 5}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.token.AllowsRepo(tt.repo))
		})
	}
}
