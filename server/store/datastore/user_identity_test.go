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

package datastore

import (
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

func TestUserIdentities(t *testing.T) {
	store, closer := newTestStore(t, new(model.User), new(model.Org), new(model.Perm), new(model.UserForge), new(model.UserIdentity), new(model.PipelineWatch))
	defer closer()

	_, err := store.UserIdentityGetByProviderSubject("oidc", "https://idp.example", "sub-1")
	assert.ErrorIs(t, err, types.RecordNotExist)

	identity := &model.UserIdentity{
		UserID:   7,
		Provider: "oidc",
		Issuer:   "https://idp.example",
		Subject:  "sub-1",
		Email:    "jane@example.com",
		Username: "jane",
		Groups:   model.JSONSliceString{"crow-users", "crow-admins"},
	}
	require.NoError(t, store.UserIdentityCreate(identity))
	assert.NotZero(t, identity.ID)
	assert.NotZero(t, identity.LinkedAt, "LinkedAt defaults to now")

	got, err := store.UserIdentityGetByProviderSubject("oidc", "https://idp.example", "sub-1")
	require.NoError(t, err)
	assert.Equal(t, int64(7), got.UserID)
	assert.Equal(t, model.JSONSliceString{"crow-users", "crow-admins"}, got.Groups)

	// (provider, issuer, subject) is unique across users
	assert.Error(t, store.UserIdentityCreate(&model.UserIdentity{UserID: 8, Provider: "oidc", Issuer: "https://idp.example", Subject: "sub-1"}))

	// The same subject at another issuer is a different account.
	_, err = store.UserIdentityGetByProviderSubject("oidc", "https://other.example", "sub-1")
	assert.ErrorIs(t, err, types.RecordNotExist)
	require.NoError(t, store.UserIdentityCreate(&model.UserIdentity{UserID: 8, Provider: "oidc", Issuer: "https://other.example", Subject: "sub-1"}))

	got.Email = "jane@corp.example"
	got.LastLogin = 1234
	require.NoError(t, store.UserIdentityUpdate(got))

	list, err := store.UserIdentityList(7)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "jane@corp.example", list[0].Email)
	assert.Equal(t, int64(1234), list[0].LastLogin)

	require.NoError(t, store.UserIdentityDelete(list[0]))
	list, err = store.UserIdentityList(7)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestDeleteUserRemovesIdentities(t *testing.T) {
	// Same tables as TestUsers, because DeleteUser also deletes the personal org with its repos and secrets.
	store, closer := newTestStore(t, new(model.User), new(model.Org), new(model.Secret), new(model.Repo), new(model.Perm), new(model.UserForge), new(model.UserIdentity), new(model.PipelineWatch))
	defer closer()

	user := &model.User{Login: "jane", Hash: "hash-jane"}
	require.NoError(t, store.CreateUser(user))
	require.NoError(t, store.UserIdentityCreate(&model.UserIdentity{UserID: user.ID, Provider: "oidc", Subject: "sub-1"}))

	require.NoError(t, store.DeleteUser(user))

	list, err := store.UserIdentityList(user.ID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestUserCreateWithIdentity(t *testing.T) {
	store, closer := newTestStore(t, new(model.User), new(model.Org), new(model.Perm), new(model.UserForge), new(model.UserIdentity), new(model.PipelineWatch))
	defer closer()

	newIdentity := func(subject string) *model.UserIdentity {
		return &model.UserIdentity{Provider: "oidc", Issuer: "https://idp.example", Subject: subject}
	}

	t.Run("creates the user, a fresh personal org and the identity", func(t *testing.T) {
		user := &model.User{Login: "jane", Hash: "A"}
		identity := newIdentity("sub-jane")
		require.NoError(t, store.UserCreateWithIdentity(user, identity))

		assert.NotZero(t, user.ID)
		assert.Equal(t, user.ID, identity.UserID)
		assert.NotZero(t, identity.LinkedAt)
		org, err := store.OrgGet(user.OrgID)
		require.NoError(t, err)
		assert.Equal(t, "jane", org.Name)
		assert.True(t, org.IsUser)
		assert.Zero(t, org.ForgeID)
		got, err := store.UserIdentityGetByProviderSubject("oidc", "https://idp.example", "sub-jane")
		require.NoError(t, err)
		assert.Equal(t, user.ID, got.UserID)
	})

	t.Run("refuses a login that differs only in case from a user", func(t *testing.T) {
		err := store.UserCreateWithIdentity(&model.User{Login: "Jane", Hash: "B"}, newIdentity("sub-other"))
		assert.ErrorIs(t, err, types.ErrNameTaken)
	})

	t.Run("refuses a login that names an org on any forge", func(t *testing.T) {
		require.NoError(t, store.OrgCreate(&model.Org{Name: "Acme", ForgeID: 2}))
		err := store.UserCreateWithIdentity(&model.User{Login: "acme", Hash: "C"}, newIdentity("sub-acme"))
		assert.ErrorIs(t, err, types.ErrNameTaken)
		_, err = store.GetUserLogin("acme")
		assert.ErrorIs(t, err, types.RecordNotExist)
	})

	t.Run("rolls back the user and org when the identity cannot be stored", func(t *testing.T) {
		// The subject is already linked to jane, so the identity insert fails.
		err := store.UserCreateWithIdentity(&model.User{Login: "bob", Hash: "D"}, newIdentity("sub-jane"))
		require.Error(t, err)
		_, err = store.GetUserLogin("bob")
		assert.ErrorIs(t, err, types.RecordNotExist)
		orgs, err := store.OrgFindByNameAllForges("bob")
		require.NoError(t, err)
		assert.Empty(t, orgs)
	})
}

func TestIsTxConflict(t *testing.T) {
	assert.True(t, isTxConflict(fmt.Errorf("commit: %w", &pgconn.PgError{Code: "40001"})))
	assert.True(t, isTxConflict(&pgconn.PgError{Code: "40P01"}))
	assert.True(t, isTxConflict(&mysql.MySQLError{Number: 1213}))
	assert.False(t, isTxConflict(&pgconn.PgError{Code: "23505"}))
	assert.False(t, isTxConflict(&mysql.MySQLError{Number: 1062}))
	assert.False(t, isTxConflict(types.ErrNameTaken))
	assert.False(t, isTxConflict(nil))
}
