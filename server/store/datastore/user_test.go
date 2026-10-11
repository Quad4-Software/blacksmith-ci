// Copyright 2022 Woodpecker Authors
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

package datastore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

func TestUsers(t *testing.T) {
	store, closer := newTestStore(t, new(model.User), new(model.Org), new(model.Secret), new(model.Repo), new(model.Perm), new(model.UserForge), new(model.UserIdentity))
	defer closer()

	count, err := store.GetUserCount()
	assert.NoError(t, err)
	assert.Zero(t, count)

	user := model.User{
		Login:        "joe",
		AccessToken:  "f0b461ca586c27872b43a0685cbc2847",
		RefreshToken: "976f22a5eef7caacb7e678d6c52f49b1",
		Email:        "foo@bar.com",
		Avatar:       "b9015b0857e16ac4d94a0ffd9a0b79c8",
	}
	err = store.CreateUser(&user)
	assert.NoError(t, err)
	assert.NotZero(t, user.ID)

	err2 := store.UpdateUser(&user)
	assert.NoError(t, err2)

	getUser, err := store.GetUser(user.ID)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, getUser.ID)
	assert.Equal(t, user.Login, getUser.Login)
	assert.Equal(t, user.AccessToken, getUser.AccessToken)
	assert.Equal(t, user.RefreshToken, getUser.RefreshToken)
	assert.Equal(t, user.Email, getUser.Email)
	assert.Equal(t, user.Avatar, getUser.Avatar)

	getUser, err = store.GetUserLogin(user.Login)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, getUser.ID)
	assert.Equal(t, user.Login, getUser.Login)

	// check unique login
	user2 := model.User{
		Login:       "Joe",
		Email:       "foo2@bar.com",
		AccessToken: "ab20g0ddaf012c744e136da16aa21ad9",
	}
	err2 = store.CreateUser(&user2)
	assert.Error(t, err2)

	user2 = model.User{
		Login:       "jane",
		Email:       "foo@bar.com",
		AccessToken: "ab20g0ddaf012c744e136da16aa21ad9",
		Hash:        "A",
	}
	assert.NoError(t, store.CreateUser(&user2))
	users, err := store.GetUserList(&model.ListOptions{Page: 1, PerPage: 50})
	assert.NoError(t, err)
	assert.Len(t, users, 2)
	// "jane" user is first due to alphabetic sorting
	assert.Equal(t, user2.Login, users[0].Login)
	assert.Equal(t, user2.Email, users[0].Email)
	assert.Equal(t, user2.AccessToken, users[0].AccessToken)

	count, err = store.GetUserCount()
	assert.NoError(t, err)
	assert.EqualValues(t, 2, count)

	getUser, err1 := store.GetUser(user.ID)
	assert.NoError(t, err1)
	err2 = store.DeleteUser(getUser)
	assert.NoError(t, err2)
	_, err3 := store.GetUser(getUser.ID)
	assert.Error(t, err3)
}

// TestGetUserByForgeRemoteIDIsForgeScoped guards against cross-forge account
// takeover: an account on forge B must never resolve to a user registered on
// forge A, no matter how the two accounts collide.
func TestGetUserByForgeRemoteIDIsForgeScoped(t *testing.T) {
	store, closer := newTestStore(t, new(model.User), new(model.Org), new(model.Perm), new(model.UserIdentity))
	defer closer()

	victim := &model.User{
		Login:         "victim",
		ForgeID:       1,
		ForgeRemoteID: "42",
		Hash:          "A",
		Admin:         true,
	}
	assert.NoError(t, store.CreateUser(victim))

	t.Run("same login on another forge does not match", func(t *testing.T) {
		// Attacker registers "victim" on forge 2 and gets remote ID 7 there.
		_, err := store.GetUserByForgeRemoteID(2, "7", "victim")
		assert.ErrorIs(t, err, types.RecordNotExist)
	})

	t.Run("same remote ID on another forge does not match", func(t *testing.T) {
		// Forge remote IDs are per-forge sequences, so collisions are trivial.
		_, err := store.GetUserByForgeRemoteID(2, "42", "attacker")
		assert.ErrorIs(t, err, types.RecordNotExist)
	})

	t.Run("own forge and remote ID still matches", func(t *testing.T) {
		got, err := store.GetUserByForgeRemoteID(1, "42", "victim")
		assert.NoError(t, err)
		assert.Equal(t, victim.ID, got.ID)
	})

	t.Run("legacy row without remote ID matches by login on its own forge", func(t *testing.T) {
		legacy := &model.User{Login: "legacy", ForgeID: 1, Hash: "B"}
		assert.NoError(t, store.CreateUser(legacy))

		got, err := store.GetUserByForgeRemoteID(1, "99", "legacy")
		assert.NoError(t, err)
		assert.Equal(t, legacy.ID, got.ID)

		// ... but not from a different forge.
		_, err = store.GetUserByForgeRemoteID(2, "99", "legacy")
		assert.ErrorIs(t, err, types.RecordNotExist)
	})

	t.Run("pre-multi-forge row with unset forge ID belongs to the main forge", func(t *testing.T) {
		ancient := &model.User{Login: "ancient", ForgeRemoteID: "5", Hash: "C"}
		assert.NoError(t, store.CreateUser(ancient))

		got, err := store.GetUserByForgeRemoteID(1, "5", "ancient")
		assert.NoError(t, err)
		assert.Equal(t, ancient.ID, got.ID)

		_, err = store.GetUserByForgeRemoteID(2, "5", "ancient")
		assert.ErrorIs(t, err, types.RecordNotExist)
	})

	t.Run("identity provider user never matches by login", func(t *testing.T) {
		// Users registered through single sign-on have no forge identity yet, and
		// their login is not forge-verified: a forge account of that name must
		// not sign in as them.
		sso := &model.User{Login: "sso", Hash: "D"}
		require.NoError(t, store.UserCreateWithIdentity(sso, &model.UserIdentity{Provider: "oidc", Issuer: "https://idp.example", Subject: "sub-sso"}))

		_, err := store.GetUserByForgeRemoteID(1, "77", "sso")
		assert.ErrorIs(t, err, types.RecordNotExist)
	})
}

func TestCreateUserWithExistingOrg(t *testing.T) {
	store, closer := newTestStore(t, new(model.User), new(model.Org), new(model.Perm))
	defer closer()

	existingOrg := &model.Org{
		ForgeID: 1,
		IsUser:  true,
		Name:    "existingOrg",
		Private: false,
	}

	err := store.OrgCreate(existingOrg)
	assert.NoError(t, err)
	assert.EqualValues(t, "existingOrg", existingOrg.Name)

	// Create a new user with the same name as the existing organization
	newUser := &model.User{
		Login:   "existingOrg",
		Hash:    "A",
		ForgeID: 1,
	}
	err = store.CreateUser(newUser)
	assert.NoError(t, err)

	updatedOrg, err := store.OrgGet(existingOrg.ID)
	assert.NoError(t, err)
	assert.Equal(t, "existingOrg", updatedOrg.Name)

	newUser2 := &model.User{
		Login:   "new-user",
		ForgeID: 1,
		Hash:    "B",
	}
	err = store.CreateUser(newUser2)
	assert.NoError(t, err)

	newOrg, err := store.OrgFindByName("new-user", 1)
	assert.NoError(t, err)
	assert.Equal(t, "new-user", newOrg.Name)
}

func TestUserRolesRoundTrip(t *testing.T) {
	store, closer := newTestStore(t, new(model.User), new(model.Org), new(model.Secret), new(model.Repo), new(model.Perm), new(model.UserForge), new(model.UserIdentity))
	defer closer()

	u := &model.User{Login: "roles", Hash: "h-roles", Roles: model.JSONSliceString{"auditor", "operator"}}
	require.NoError(t, store.CreateUser(u))

	got, err := store.GetUser(u.ID)
	require.NoError(t, err)
	assert.Equal(t, model.JSONSliceString{"auditor", "operator"}, got.Roles)

	got.Roles = nil
	require.NoError(t, store.UpdateUser(got))
	got, err = store.GetUser(u.ID)
	require.NoError(t, err)
	assert.Empty(t, got.Roles)
}
