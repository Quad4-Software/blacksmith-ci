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
