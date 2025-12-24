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

func TestEncryptedForgeStore_ForgeGet(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewForgeStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	forge := &model.Forge{
		ID:           1,
		Type:         "github",
		URL:          "https://github.com",
		ClientSecret: "encrypted-secret",
	}

	mockStore.On("ForgeGet", int64(1)).Return(forge, nil)
	mockEncryption.On("Decrypt", "encrypted-secret", "1").Return("decrypted-secret", nil)

	result, err := wrapper.ForgeGet(1)
	require.NoError(t, err)
	assert.Equal(t, "decrypted-secret", result.ClientSecret)
	assert.Equal(t, model.ForgeType("github"), result.Type)
}

func TestEncryptedForgeStore_ForgeGetEmptyClientSecret(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)

	wrapper := NewForgeStore(mockStore)

	forge := &model.Forge{
		ID:           1,
		Type:         "github",
		URL:          "https://github.com",
		ClientSecret: "", // No secret
	}

	mockStore.On("ForgeGet", int64(1)).Return(forge, nil)

	result, err := wrapper.ForgeGet(1)
	require.NoError(t, err)
	assert.Empty(t, result.ClientSecret)
}

func TestEncryptedForgeStore_ForgeList(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewForgeStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	forges := []*model.Forge{
		{ID: 1, Type: "github", ClientSecret: "encrypted1"},
		{ID: 2, Type: "gitlab", ClientSecret: "encrypted2"},
	}

	mockStore.On("ForgeList", mock.Anything).Return(forges, nil)
	mockEncryption.On("Decrypt", "encrypted1", "1").Return("decrypted1", nil)
	mockEncryption.On("Decrypt", "encrypted2", "2").Return("decrypted2", nil)

	result, err := wrapper.ForgeList(nil)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "decrypted1", result[0].ClientSecret)
	assert.Equal(t, "decrypted2", result[1].ClientSecret)
}

func TestEncryptedForgeStore_ForgeCreate(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewForgeStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	forge := &model.Forge{
		Type:         "github",
		URL:          "https://github.com",
		ClientSecret: "plaintext-secret",
	}

	// ForgeCreate sets the ID
	mockStore.On("ForgeCreate", mock.AnythingOfType("*model.Forge")).Run(func(args mock.Arguments) {
		f, ok := args.Get(0).(*model.Forge)
		require.True(t, ok)
		f.ID = 1
	}).Return(nil)

	// Then encrypt and update
	mockEncryption.On("Encrypt", "plaintext-secret", "1").Return("encrypted-secret", nil)
	mockStore.On("ForgeUpdate", mock.MatchedBy(func(f *model.Forge) bool {
		return f.ClientSecret == "encrypted-secret"
	})).Return(nil)

	// Then decrypt back for the caller
	mockEncryption.On("Decrypt", "encrypted-secret", "1").Return("plaintext-secret", nil)

	err = wrapper.ForgeCreate(forge)
	require.NoError(t, err)
	assert.Equal(t, int64(1), forge.ID)
	assert.Equal(t, "plaintext-secret", forge.ClientSecret)
}

func TestEncryptedForgeStore_ForgeCreateEmptySecret(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)

	wrapper := NewForgeStore(mockStore)

	forge := &model.Forge{
		Type:         "github",
		URL:          "https://github.com",
		ClientSecret: "", // Empty secret
	}

	// Should just create without encryption
	mockStore.On("ForgeCreate", mock.AnythingOfType("*model.Forge")).Run(func(args mock.Arguments) {
		f, ok := args.Get(0).(*model.Forge)
		require.True(t, ok)
		f.ID = 1
	}).Return(nil)

	err := wrapper.ForgeCreate(forge)
	require.NoError(t, err)
	assert.Equal(t, int64(1), forge.ID)
}

func TestEncryptedForgeStore_ForgeUpdate(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewForgeStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	forge := &model.Forge{
		ID:           1,
		Type:         "github",
		ClientSecret: "new-plaintext",
	}

	mockEncryption.On("Encrypt", "new-plaintext", "1").Return("new-encrypted", nil)
	mockStore.On("ForgeUpdate", mock.MatchedBy(func(f *model.Forge) bool {
		return f.ClientSecret == "new-encrypted"
	})).Return(nil)
	mockEncryption.On("Decrypt", "new-encrypted", "1").Return("new-plaintext", nil)

	err = wrapper.ForgeUpdate(forge)
	require.NoError(t, err)
	assert.Equal(t, "new-plaintext", forge.ClientSecret)
}

func TestEncryptedForgeStore_ForgeDelete(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)

	wrapper := NewForgeStore(mockStore)

	forge := &model.Forge{ID: 1}
	mockStore.On("ForgeDelete", forge).Return(nil)

	err := wrapper.ForgeDelete(forge)
	require.NoError(t, err)
}

func TestEncryptedForgeStore_ForgeListAll(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewForgeStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	forges := []*model.Forge{
		{ID: 1, Type: "github", ClientSecret: "encrypted1"},
		{ID: 2, Type: "gitlab", ClientSecret: ""},
		{ID: 3, Type: "forgejo", ClientSecret: "encrypted3"},
	}

	mockStore.On("ForgeListAll").Return(forges, nil)
	mockEncryption.On("Decrypt", "encrypted1", "1").Return("decrypted1", nil)
	mockEncryption.On("Decrypt", "encrypted3", "3").Return("decrypted3", nil)

	result, err := wrapper.ForgeListAll()
	require.NoError(t, err)
	require.Len(t, result, 3)
	assert.Equal(t, "decrypted1", result[0].ClientSecret)
	assert.Empty(t, result[1].ClientSecret)
	assert.Equal(t, "decrypted3", result[2].ClientSecret)
}

func TestEncryptedForgeStore_SetEncryptionServiceTwice(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewForgeStore(mockStore)

	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	// Setting encryption service twice should fail
	err = wrapper.SetEncryptionService(mockEncryption)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "more than once")
}

// Test helper to verify interface compliance.
var _ encryption_types.EncryptionClient = (*EncryptedForgeStore)(nil)
