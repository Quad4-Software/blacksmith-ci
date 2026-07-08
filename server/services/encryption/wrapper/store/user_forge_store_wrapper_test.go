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

package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	encryption_mocks "codefloe.com/crowci/crow/v6/server/services/encryption/types/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

// TestEncryptedUserForgeStore_UserForgeCreate verifies the bug fix for
// codefloe.com/crowci/crow#974: tokens must be encrypted *after* the row is
// inserted so that the AAD (which includes the row ID) matches the real
// auto-incremented ID rather than the placeholder zero. Otherwise every
// subsequent UserForgeGet decrypt would fail with a Tink AEAD error.
func TestEncryptedUserForgeStore_UserForgeCreate(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewUserForgeStore(mockStore)
	require.NoError(t, wrapper.SetEncryptionService(mockEncryption))

	userForge := &model.UserForge{
		UserID:       1,
		ForgeID:      2,
		AccessToken:  "plain-access",
		RefreshToken: "plain-refresh",
	}

	// UserForgeCreate inserts first and assigns the auto-increment ID.
	mockStore.On("UserForgeCreate", mock.AnythingOfType("*model.UserForge")).Run(func(args mock.Arguments) {
		uf, ok := args.Get(0).(*model.UserForge)
		require.True(t, ok)
		// Pre-condition: tokens must still be plaintext at this point
		// (i.e. encryption has not happened yet) so that we can prove the
		// fix is in place.
		assert.Equal(t, "plain-access", uf.AccessToken)
		assert.Equal(t, "plain-refresh", uf.RefreshToken)
		uf.ID = 42
	}).Return(nil)

	// Encrypt with the *real* ID (42), not 0.
	mockEncryption.On("Encrypt", "plain-access", "42:access").Return("encrypted-access", nil)
	mockEncryption.On("Encrypt", "plain-refresh", "42:refresh").Return("encrypted-refresh", nil)

	// Then update the row with the encrypted tokens.
	mockStore.On("UserForgeUpdate", mock.MatchedBy(func(uf *model.UserForge) bool {
		return uf.ID == 42 && uf.AccessToken == "encrypted-access" && uf.RefreshToken == "encrypted-refresh"
	})).Return(nil)

	// Decrypt back so the caller continues to see plaintext.
	mockEncryption.On("Decrypt", "encrypted-access", "42:access").Return("plain-access", nil)
	mockEncryption.On("Decrypt", "encrypted-refresh", "42:refresh").Return("plain-refresh", nil)

	require.NoError(t, wrapper.UserForgeCreate(userForge))

	assert.Equal(t, int64(42), userForge.ID)
	assert.Equal(t, "plain-access", userForge.AccessToken)
	assert.Equal(t, "plain-refresh", userForge.RefreshToken)
}

func TestEncryptedUserForgeStore_UserForgeCreate_NoTokens(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewUserForgeStore(mockStore)
	require.NoError(t, wrapper.SetEncryptionService(mockEncryption))

	userForge := &model.UserForge{UserID: 1, ForgeID: 2}

	// With no tokens, only the bare insert should happen — no encrypt/update.
	mockStore.On("UserForgeCreate", userForge).Run(func(args mock.Arguments) {
		uf, ok := args.Get(0).(*model.UserForge)
		require.True(t, ok)
		uf.ID = 7
	}).Return(nil)

	require.NoError(t, wrapper.UserForgeCreate(userForge))
	assert.Equal(t, int64(7), userForge.ID)
}

func TestEncryptedUserForgeStore_UserForgeCreate_NoEncryption(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)

	wrapper := NewUserForgeStore(mockStore)
	// Intentionally no SetEncryptionService — wrapper should be a pass-through.

	userForge := &model.UserForge{UserID: 1, ForgeID: 2, AccessToken: "plain"}

	mockStore.On("UserForgeCreate", userForge).Return(nil)

	require.NoError(t, wrapper.UserForgeCreate(userForge))
	assert.Equal(t, "plain", userForge.AccessToken)
}

func TestEncryptedUserForgeStore_UserForgeUpdate(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewUserForgeStore(mockStore)
	require.NoError(t, wrapper.SetEncryptionService(mockEncryption))

	userForge := &model.UserForge{
		ID:           42,
		UserID:       1,
		ForgeID:      2,
		AccessToken:  "new-plain-access",
		RefreshToken: "new-plain-refresh",
	}

	mockEncryption.On("Encrypt", "new-plain-access", "42:access").Return("new-encrypted-access", nil)
	mockEncryption.On("Encrypt", "new-plain-refresh", "42:refresh").Return("new-encrypted-refresh", nil)
	mockStore.On("UserForgeUpdate", mock.MatchedBy(func(uf *model.UserForge) bool {
		return uf.AccessToken == "new-encrypted-access" && uf.RefreshToken == "new-encrypted-refresh"
	})).Return(nil)
	mockEncryption.On("Decrypt", "new-encrypted-access", "42:access").Return("new-plain-access", nil)
	mockEncryption.On("Decrypt", "new-encrypted-refresh", "42:refresh").Return("new-plain-refresh", nil)

	require.NoError(t, wrapper.UserForgeUpdate(userForge))

	// Caller must see plaintext after update so subsequent in-memory uses
	// of the same struct don't accidentally send ciphertext to the forge.
	assert.Equal(t, "new-plain-access", userForge.AccessToken)
	assert.Equal(t, "new-plain-refresh", userForge.RefreshToken)
}

func TestEncryptedUserForgeStore_UserForgeUpdate_NoEncryption(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)

	wrapper := NewUserForgeStore(mockStore)

	userForge := &model.UserForge{ID: 42, AccessToken: "plain"}

	mockStore.On("UserForgeUpdate", userForge).Return(nil)

	require.NoError(t, wrapper.UserForgeUpdate(userForge))
	assert.Equal(t, "plain", userForge.AccessToken)
}

func TestEncryptedUserForgeStore_UserForgeGet(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewUserForgeStore(mockStore)
	require.NoError(t, wrapper.SetEncryptionService(mockEncryption))

	stored := &model.UserForge{
		ID:           42,
		UserID:       1,
		ForgeID:      2,
		AccessToken:  "encrypted-access",
		RefreshToken: "encrypted-refresh",
	}

	mockStore.On("UserForgeGet", int64(1), int64(2)).Return(stored, nil)
	mockEncryption.On("Decrypt", "encrypted-access", "42:access").Return("plain-access", nil)
	mockEncryption.On("Decrypt", "encrypted-refresh", "42:refresh").Return("plain-refresh", nil)

	result, err := wrapper.UserForgeGet(1, 2)
	require.NoError(t, err)
	assert.Equal(t, "plain-access", result.AccessToken)
	assert.Equal(t, "plain-refresh", result.RefreshToken)
}

func TestEncryptedUserForgeStore_SetEncryptionServiceTwice(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewUserForgeStore(mockStore)
	require.NoError(t, wrapper.SetEncryptionService(mockEncryption))

	err := wrapper.SetEncryptionService(mockEncryption)
	require.Error(t, err)
}
