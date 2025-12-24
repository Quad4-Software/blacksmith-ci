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

func TestEncryptedRegistryStore_RegistryFind(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewRegistryStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	repo := &model.Repo{ID: 1}
	registry := &model.Registry{
		ID:       1,
		Address:  "docker.io",
		Username: "user",
		Password: "encrypted-password",
	}

	mockStore.On("RegistryFind", repo, "docker.io").Return(registry, nil)
	mockEncryption.On("Decrypt", "encrypted-password", "1").Return("decrypted-password", nil)

	result, err := wrapper.RegistryFind(repo, "docker.io")
	require.NoError(t, err)
	assert.Equal(t, "decrypted-password", result.Password)
	assert.Equal(t, "user", result.Username)
}

func TestEncryptedRegistryStore_RegistryList(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewRegistryStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	repo := &model.Repo{ID: 1}
	registries := []*model.Registry{
		{ID: 1, Address: "docker.io", Password: "encrypted1"},
		{ID: 2, Address: "gcr.io", Password: "encrypted2"},
	}

	mockStore.On("RegistryList", repo, true, mock.Anything).Return(registries, nil)
	mockEncryption.On("Decrypt", "encrypted1", "1").Return("decrypted1", nil)
	mockEncryption.On("Decrypt", "encrypted2", "2").Return("decrypted2", nil)

	result, err := wrapper.RegistryList(repo, true, nil)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "decrypted1", result[0].Password)
	assert.Equal(t, "decrypted2", result[1].Password)
}

func TestEncryptedRegistryStore_RegistryCreate(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewRegistryStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	registry := &model.Registry{
		Address:  "docker.io",
		Username: "user",
		Password: "plaintext-password",
	}

	// RegistryCreate is called with actual data to get ID
	mockStore.On("RegistryCreate", mock.AnythingOfType("*model.Registry")).Run(func(args mock.Arguments) {
		r, ok := args.Get(0).(*model.Registry)
		require.True(t, ok)
		r.ID = 1
	}).Return(nil)

	// Then encrypt and update
	mockEncryption.On("Encrypt", "plaintext-password", "1").Return("encrypted-password", nil)
	mockStore.On("RegistryUpdate", mock.MatchedBy(func(r *model.Registry) bool {
		return r.Password == "encrypted-password"
	})).Return(nil)

	// Then decrypt back for the caller
	mockEncryption.On("Decrypt", "encrypted-password", "1").Return("plaintext-password", nil)

	err = wrapper.RegistryCreate(registry)
	require.NoError(t, err)
	assert.Equal(t, int64(1), registry.ID)
	assert.Equal(t, "plaintext-password", registry.Password)
}

func TestEncryptedRegistryStore_RegistryCreateEmptyPassword(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)

	wrapper := NewRegistryStore(mockStore)

	registry := &model.Registry{
		Address:  "docker.io",
		Username: "user",
		Password: "", // Empty password
	}

	// Should just create without encryption
	mockStore.On("RegistryCreate", mock.AnythingOfType("*model.Registry")).Run(func(args mock.Arguments) {
		r, ok := args.Get(0).(*model.Registry)
		require.True(t, ok)
		r.ID = 1
	}).Return(nil)

	err := wrapper.RegistryCreate(registry)
	require.NoError(t, err)
	assert.Equal(t, int64(1), registry.ID)
}

func TestEncryptedRegistryStore_RegistryUpdate(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewRegistryStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	registry := &model.Registry{
		ID:       1,
		Address:  "docker.io",
		Password: "new-plaintext",
	}

	mockEncryption.On("Encrypt", "new-plaintext", "1").Return("new-encrypted", nil)
	mockStore.On("RegistryUpdate", mock.MatchedBy(func(r *model.Registry) bool {
		return r.Password == "new-encrypted"
	})).Return(nil)
	mockEncryption.On("Decrypt", "new-encrypted", "1").Return("new-plaintext", nil)

	err = wrapper.RegistryUpdate(registry)
	require.NoError(t, err)
	assert.Equal(t, "new-plaintext", registry.Password)
}

func TestEncryptedRegistryStore_RegistryDelete(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)

	wrapper := NewRegistryStore(mockStore)

	registry := &model.Registry{ID: 1}
	mockStore.On("RegistryDelete", registry).Return(nil)

	err := wrapper.RegistryDelete(registry)
	require.NoError(t, err)
}

func TestEncryptedRegistryStore_OrgRegistryFind(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewRegistryStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	registry := &model.Registry{
		ID:       1,
		OrgID:    10,
		Address:  "gcr.io",
		Password: "encrypted-org-password",
	}

	mockStore.On("OrgRegistryFind", int64(10), "gcr.io").Return(registry, nil)
	mockEncryption.On("Decrypt", "encrypted-org-password", "1").Return("decrypted-org-password", nil)

	result, err := wrapper.OrgRegistryFind(10, "gcr.io")
	require.NoError(t, err)
	assert.Equal(t, "decrypted-org-password", result.Password)
}

func TestEncryptedRegistryStore_GlobalRegistryFind(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewRegistryStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	registry := &model.Registry{
		ID:       1,
		Address:  "quay.io",
		Password: "encrypted-global",
	}

	mockStore.On("GlobalRegistryFind", "quay.io").Return(registry, nil)
	mockEncryption.On("Decrypt", "encrypted-global", "1").Return("decrypted-global", nil)

	result, err := wrapper.GlobalRegistryFind("quay.io")
	require.NoError(t, err)
	assert.Equal(t, "decrypted-global", result.Password)
}

func TestEncryptedRegistryStore_SetEncryptionServiceTwice(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewRegistryStore(mockStore)

	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	// Setting encryption service twice should fail
	err = wrapper.SetEncryptionService(mockEncryption)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "more than once")
}

// Test helper to verify interface compliance.
var _ encryption_types.EncryptionClient = (*EncryptedRegistryStore)(nil)
