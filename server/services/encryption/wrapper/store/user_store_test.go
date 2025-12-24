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

package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codeberg.org/crowci/crow/v5/server/model"
	encryption_types "codeberg.org/crowci/crow/v5/server/services/encryption/types"
	encryption_mocks "codeberg.org/crowci/crow/v5/server/services/encryption/types/mocks"
	store_mocks "codeberg.org/crowci/crow/v5/server/store/mocks"
)

func TestEncryptedUserStore_GetUser(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewUserStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	user := &model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "encrypted-access",
		RefreshToken: "encrypted-refresh",
	}

	mockStore.On("GetUser", int64(1)).Return(user, nil)
	mockEncryption.On("Decrypt", "encrypted-access", "1:access").Return("decrypted-access", nil)
	mockEncryption.On("Decrypt", "encrypted-refresh", "1:refresh").Return("decrypted-refresh", nil)

	result, err := wrapper.GetUser(1)
	require.NoError(t, err)
	assert.Equal(t, "decrypted-access", result.AccessToken)
	assert.Equal(t, "decrypted-refresh", result.RefreshToken)
	assert.Equal(t, "testuser", result.Login)
}

func TestEncryptedUserStore_GetUserWithOnlyAccessToken(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewUserStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	user := &model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "encrypted-access",
		RefreshToken: "", // No refresh token
	}

	mockStore.On("GetUser", int64(1)).Return(user, nil)
	mockEncryption.On("Decrypt", "encrypted-access", "1:access").Return("decrypted-access", nil)

	result, err := wrapper.GetUser(1)
	require.NoError(t, err)
	assert.Equal(t, "decrypted-access", result.AccessToken)
	assert.Empty(t, result.RefreshToken)
}

func TestEncryptedUserStore_GetUserLogin(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewUserStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	user := &model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "encrypted-access",
		RefreshToken: "encrypted-refresh",
	}

	mockStore.On("GetUserLogin", "testuser").Return(user, nil)
	mockEncryption.On("Decrypt", "encrypted-access", "1:access").Return("decrypted-access", nil)
	mockEncryption.On("Decrypt", "encrypted-refresh", "1:refresh").Return("decrypted-refresh", nil)

	result, err := wrapper.GetUserLogin("testuser")
	require.NoError(t, err)
	assert.Equal(t, "decrypted-access", result.AccessToken)
	assert.Equal(t, "decrypted-refresh", result.RefreshToken)
}

func TestEncryptedUserStore_GetUserList(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewUserStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	users := []*model.User{
		{ID: 1, Login: "user1", AccessToken: "encrypted1", RefreshToken: "refresh1"},
		{ID: 2, Login: "user2", AccessToken: "encrypted2", RefreshToken: ""},
	}

	mockStore.On("GetUserList", mock.Anything).Return(users, nil)
	mockEncryption.On("Decrypt", "encrypted1", "1:access").Return("decrypted1", nil)
	mockEncryption.On("Decrypt", "refresh1", "1:refresh").Return("decrypted-refresh1", nil)
	mockEncryption.On("Decrypt", "encrypted2", "2:access").Return("decrypted2", nil)

	result, err := wrapper.GetUserList(nil)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "decrypted1", result[0].AccessToken)
	assert.Equal(t, "decrypted-refresh1", result[0].RefreshToken)
	assert.Equal(t, "decrypted2", result[1].AccessToken)
}

func TestEncryptedUserStore_CreateUser(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewUserStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	user := &model.User{
		Login:        "newuser",
		AccessToken:  "plain-access",
		RefreshToken: "plain-refresh",
	}

	// CreateUser sets the ID
	mockStore.On("CreateUser", mock.AnythingOfType("*model.User")).Run(func(args mock.Arguments) {
		u, ok := args.Get(0).(*model.User)
		require.True(t, ok)
		u.ID = 1
	}).Return(nil)

	// Then encrypt and update
	mockEncryption.On("Encrypt", "plain-access", "1:access").Return("encrypted-access", nil)
	mockEncryption.On("Encrypt", "plain-refresh", "1:refresh").Return("encrypted-refresh", nil)
	mockStore.On("UpdateUser", mock.MatchedBy(func(u *model.User) bool {
		return u.AccessToken == "encrypted-access" && u.RefreshToken == "encrypted-refresh"
	})).Return(nil)

	// Then decrypt back for the caller
	mockEncryption.On("Decrypt", "encrypted-access", "1:access").Return("plain-access", nil)
	mockEncryption.On("Decrypt", "encrypted-refresh", "1:refresh").Return("plain-refresh", nil)

	err = wrapper.CreateUser(user)
	require.NoError(t, err)
	assert.Equal(t, int64(1), user.ID)
	assert.Equal(t, "plain-access", user.AccessToken)
	assert.Equal(t, "plain-refresh", user.RefreshToken)
}

func TestEncryptedUserStore_UpdateUser(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewUserStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	user := &model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "new-plain-access",
		RefreshToken: "new-plain-refresh",
	}

	mockEncryption.On("Encrypt", "new-plain-access", "1:access").Return("new-encrypted-access", nil)
	mockEncryption.On("Encrypt", "new-plain-refresh", "1:refresh").Return("new-encrypted-refresh", nil)
	mockStore.On("UpdateUser", mock.MatchedBy(func(u *model.User) bool {
		return u.AccessToken == "new-encrypted-access" && u.RefreshToken == "new-encrypted-refresh"
	})).Return(nil)
	mockEncryption.On("Decrypt", "new-encrypted-access", "1:access").Return("new-plain-access", nil)
	mockEncryption.On("Decrypt", "new-encrypted-refresh", "1:refresh").Return("new-plain-refresh", nil)

	err = wrapper.UpdateUser(user)
	require.NoError(t, err)
	assert.Equal(t, "new-plain-access", user.AccessToken)
	assert.Equal(t, "new-plain-refresh", user.RefreshToken)
}

func TestEncryptedUserStore_DeleteUser(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)

	wrapper := NewUserStore(mockStore)

	user := &model.User{ID: 1}
	mockStore.On("DeleteUser", user).Return(nil)

	err := wrapper.DeleteUser(user)
	require.NoError(t, err)
}

func TestEncryptedUserStore_GetUserRemoteID(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewUserStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	user := &model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "encrypted-access",
		RefreshToken: "",
	}

	mockStore.On("GetUserRemoteID", model.ForgeRemoteID("123"), "testuser").Return(user, nil)
	mockEncryption.On("Decrypt", "encrypted-access", "1:access").Return("decrypted-access", nil)

	result, err := wrapper.GetUserRemoteID("123", "testuser")
	require.NoError(t, err)
	assert.Equal(t, "decrypted-access", result.AccessToken)
}

func TestEncryptedUserStore_SetEncryptionServiceTwice(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewUserStore(mockStore)

	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	// Setting encryption service twice should fail
	err = wrapper.SetEncryptionService(mockEncryption)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "more than once")
}

// Test helper to verify interface compliance.
var _ encryption_types.EncryptionClient = (*EncryptedUserStore)(nil)
