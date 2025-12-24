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

func TestEncryptedSecretStore_SecretFind(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewSecretStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	repo := &model.Repo{ID: 1}
	secret := &model.Secret{
		ID:    1,
		Name:  "test-secret",
		Value: "encrypted-value",
	}

	mockStore.On("SecretFind", repo, "test-secret").Return(secret, nil)
	mockEncryption.On("Decrypt", "encrypted-value", "1").Return("decrypted-value", nil)

	result, err := wrapper.SecretFind(repo, "test-secret")
	require.NoError(t, err)
	assert.Equal(t, "decrypted-value", result.Value)
}

func TestEncryptedSecretStore_SecretList(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewSecretStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	repo := &model.Repo{ID: 1}
	secrets := []*model.Secret{
		{ID: 1, Name: "secret1", Value: "encrypted1"},
		{ID: 2, Name: "secret2", Value: "encrypted2"},
	}

	mockStore.On("SecretList", repo, true, mock.Anything).Return(secrets, nil)
	mockEncryption.On("Decrypt", "encrypted1", "1").Return("decrypted1", nil)
	mockEncryption.On("Decrypt", "encrypted2", "2").Return("decrypted2", nil)

	result, err := wrapper.SecretList(repo, true, nil)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "decrypted1", result[0].Value)
	assert.Equal(t, "decrypted2", result[1].Value)
}

func TestEncryptedSecretStore_SecretCreate(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewSecretStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	secret := &model.Secret{
		Name:  "new-secret",
		Value: "plaintext-value",
	}

	// SecretCreate is called first with an empty secret to get ID
	mockStore.On("SecretCreate", mock.AnythingOfType("*model.Secret")).Run(func(args mock.Arguments) {
		s, ok := args.Get(0).(*model.Secret)
		require.True(t, ok)
		s.ID = 1
	}).Return(nil)

	// Then encrypt and update
	mockEncryption.On("Encrypt", "plaintext-value", "1").Return("encrypted-value", nil)
	mockStore.On("SecretUpdate", mock.MatchedBy(func(s *model.Secret) bool {
		return s.Value == "encrypted-value"
	})).Return(nil)

	// Then decrypt back for the caller
	mockEncryption.On("Decrypt", "encrypted-value", "1").Return("plaintext-value", nil)

	err = wrapper.SecretCreate(secret)
	require.NoError(t, err)
	assert.Equal(t, int64(1), secret.ID)
	assert.Equal(t, "plaintext-value", secret.Value)
}

func TestEncryptedSecretStore_SecretUpdate(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewSecretStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	secret := &model.Secret{
		ID:    1,
		Name:  "test-secret",
		Value: "new-plaintext",
	}

	mockEncryption.On("Encrypt", "new-plaintext", "1").Return("new-encrypted", nil)
	mockStore.On("SecretUpdate", mock.MatchedBy(func(s *model.Secret) bool {
		return s.Value == "new-encrypted"
	})).Return(nil)
	mockEncryption.On("Decrypt", "new-encrypted", "1").Return("new-plaintext", nil)

	err = wrapper.SecretUpdate(secret)
	require.NoError(t, err)
	assert.Equal(t, "new-plaintext", secret.Value)
}

func TestEncryptedSecretStore_SecretDelete(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)

	wrapper := NewSecretStore(mockStore)

	secret := &model.Secret{ID: 1}
	mockStore.On("SecretDelete", secret).Return(nil)

	err := wrapper.SecretDelete(secret)
	require.NoError(t, err)
}

func TestEncryptedSecretStore_OrgSecretFind(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewSecretStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	secret := &model.Secret{
		ID:    1,
		OrgID: 10,
		Name:  "org-secret",
		Value: "encrypted-org-value",
	}

	mockStore.On("OrgSecretFind", int64(10), "org-secret").Return(secret, nil)
	mockEncryption.On("Decrypt", "encrypted-org-value", "1").Return("decrypted-org-value", nil)

	result, err := wrapper.OrgSecretFind(10, "org-secret")
	require.NoError(t, err)
	assert.Equal(t, "decrypted-org-value", result.Value)
}

func TestEncryptedSecretStore_GlobalSecretFind(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewSecretStore(mockStore)
	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	secret := &model.Secret{
		ID:    1,
		Name:  "global-secret",
		Value: "encrypted-global",
	}

	mockStore.On("GlobalSecretFind", "global-secret").Return(secret, nil)
	mockEncryption.On("Decrypt", "encrypted-global", "1").Return("decrypted-global", nil)

	result, err := wrapper.GlobalSecretFind("global-secret")
	require.NoError(t, err)
	assert.Equal(t, "decrypted-global", result.Value)
}

func TestEncryptedSecretStore_SetEncryptionServiceTwice(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	wrapper := NewSecretStore(mockStore)

	err := wrapper.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	// Setting encryption service twice should fail
	err = wrapper.SetEncryptionService(mockEncryption)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "more than once")
}

// Test helper to verify interface compliance.
var _ encryption_types.EncryptionClient = (*EncryptedSecretStore)(nil)
