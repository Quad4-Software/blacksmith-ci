// Copyright 2026 Crow CI Authors
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

package bootstrap

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store"
	"codefloe.com/crowci/crow/v5/server/store/datastore"
)

const testToken = "0123456789abcdef0123456789abcdef" // 32 chars

func newStore(t *testing.T) store.Store {
	t.Helper()
	s, err := datastore.NewEngine(&store.Opts{Driver: "sqlite3", Config: ":memory:"})
	require.NoError(t, err)
	require.NoError(t, s.Migrate(context.Background(), false))
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestAdminTokenEmptyIsNoop(t *testing.T) {
	s := newStore(t)
	require.NoError(t, AdminToken(s, ""))

	_, err := s.GetUserLogin(model.SystemAdminLogin)
	assert.Error(t, err) // user must not be created when disabled
}

func TestAdminTokenRejectsShort(t *testing.T) {
	s := newStore(t)
	assert.Error(t, AdminToken(s, "tooshort"))
}

func TestAdminTokenCreatesUserAndToken(t *testing.T) {
	s := newStore(t)
	require.NoError(t, AdminToken(s, testToken))

	user, err := s.GetUserLogin(model.SystemAdminLogin)
	require.NoError(t, err)
	assert.True(t, user.Admin)

	at, err := s.AccessTokenFindByHash(datastore.HashAccessToken(testToken))
	require.NoError(t, err)
	assert.Equal(t, user.ID, at.UserID)
	assert.Equal(t, "admin-bootstrap", at.Name)
	assert.True(t, at.HasScope(model.ScopeAdminWrite))
}

func TestAdminTokenIsIdempotent(t *testing.T) {
	s := newStore(t)
	require.NoError(t, AdminToken(s, testToken))
	first, err := s.AccessTokenFindByHash(datastore.HashAccessToken(testToken))
	require.NoError(t, err)

	require.NoError(t, AdminToken(s, testToken))
	user, err := s.GetUserLogin(model.SystemAdminLogin)
	require.NoError(t, err)
	tokens, err := s.AccessTokenList(user.ID, &model.ListOptions{All: true})
	require.NoError(t, err)
	assert.Len(t, tokens, 1)
	assert.Equal(t, first.ID, tokens[0].ID)
}

func TestAdminTokenRotationRevokesOld(t *testing.T) {
	s := newStore(t)
	require.NoError(t, AdminToken(s, testToken))

	const newToken = "ffffffffffffffffffffffffffffffff" // 32 chars
	require.NoError(t, AdminToken(s, newToken))

	_, err := s.AccessTokenFindByHash(datastore.HashAccessToken(testToken))
	assert.Error(t, err) // old token revoked

	_, err = s.AccessTokenFindByHash(datastore.HashAccessToken(newToken))
	assert.NoError(t, err) // new token present

	user, err := s.GetUserLogin(model.SystemAdminLogin)
	require.NoError(t, err)
	tokens, err := s.AccessTokenList(user.ID, &model.ListOptions{All: true})
	require.NoError(t, err)
	assert.Len(t, tokens, 1)
}
