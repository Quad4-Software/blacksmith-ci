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
	encryption_mocks "codeberg.org/crowci/crow/v5/server/services/encryption/types/mocks"
	"codeberg.org/crowci/crow/v5/server/store"
	store_mocks "codeberg.org/crowci/crow/v5/server/store/mocks"
)

func setupCompositeStore(t *testing.T) (*CompositeEncryptedStore, *store_mocks.MockStore, *encryption_mocks.MockEncryptionService) {
	t.Helper()

	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	secretWrapper := NewSecretStore(mockStore)
	registryWrapper := NewRegistryStore(mockStore)
	userWrapper := NewUserStore(mockStore)
	forgeWrapper := NewForgeStore(mockStore)
	userForgeWrapper := NewUserForgeStore(mockStore)

	composite := NewCompositeStore(mockStore, secretWrapper, registryWrapper, userWrapper, forgeWrapper, userForgeWrapper)

	err := composite.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	return composite, mockStore, mockEncryption
}

func TestCompositeStore_ImplementsStoreInterface(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)

	secretWrapper := NewSecretStore(mockStore)
	registryWrapper := NewRegistryStore(mockStore)
	userWrapper := NewUserStore(mockStore)
	forgeWrapper := NewForgeStore(mockStore)
	userForgeWrapper := NewUserForgeStore(mockStore)

	composite := NewCompositeStore(mockStore, secretWrapper, registryWrapper, userWrapper, forgeWrapper, userForgeWrapper)

	// Verify it implements store.Store
	var _ store.Store = composite
}

func TestCompositeStore_SetEncryptionServiceTwice(t *testing.T) {
	mockStore := store_mocks.NewMockStore(t)
	mockEncryption := encryption_mocks.NewMockEncryptionService(t)

	secretWrapper := NewSecretStore(mockStore)
	registryWrapper := NewRegistryStore(mockStore)
	userWrapper := NewUserStore(mockStore)
	forgeWrapper := NewForgeStore(mockStore)
	userForgeWrapper := NewUserForgeStore(mockStore)

	composite := NewCompositeStore(mockStore, secretWrapper, registryWrapper, userWrapper, forgeWrapper, userForgeWrapper)

	err := composite.SetEncryptionService(mockEncryption)
	require.NoError(t, err)

	// Setting encryption service twice should fail
	err = composite.SetEncryptionService(mockEncryption)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "more than once")
}

func TestCompositeStore_SecretOperations(t *testing.T) {
	composite, mockStore, mockEncryption := setupCompositeStore(t)

	t.Run("SecretFind", func(t *testing.T) {
		repo := &model.Repo{ID: 1}
		secret := &model.Secret{ID: 1, Name: "test", Value: "encrypted"}

		mockStore.On("SecretFind", repo, "test").Return(secret, nil).Once()
		mockEncryption.On("Decrypt", "encrypted", "1").Return("decrypted", nil).Once()

		result, err := composite.SecretFind(repo, "test")
		require.NoError(t, err)
		assert.Equal(t, "decrypted", result.Value)
	})

	t.Run("SecretCreate", func(t *testing.T) {
		secret := &model.Secret{Name: "new", Value: "plain"}

		mockStore.On("SecretCreate", mock.AnythingOfType("*model.Secret")).Run(func(args mock.Arguments) {
			s, ok := args.Get(0).(*model.Secret)
			require.True(t, ok)
			s.ID = 2
		}).Return(nil).Once()
		mockEncryption.On("Encrypt", "plain", "2").Return("encrypted", nil).Once()
		mockStore.On("SecretUpdate", mock.Anything).Return(nil).Once()
		mockEncryption.On("Decrypt", "encrypted", "2").Return("plain", nil).Once()

		err := composite.SecretCreate(secret)
		require.NoError(t, err)
		assert.Equal(t, int64(2), secret.ID)
	})
}

func TestCompositeStore_RegistryOperations(t *testing.T) {
	composite, mockStore, mockEncryption := setupCompositeStore(t)

	t.Run("RegistryFind", func(t *testing.T) {
		repo := &model.Repo{ID: 1}
		registry := &model.Registry{ID: 1, Address: "docker.io", Password: "encrypted"}

		mockStore.On("RegistryFind", repo, "docker.io").Return(registry, nil).Once()
		mockEncryption.On("Decrypt", "encrypted", "1").Return("decrypted", nil).Once()

		result, err := composite.RegistryFind(repo, "docker.io")
		require.NoError(t, err)
		assert.Equal(t, "decrypted", result.Password)
	})

	t.Run("RegistryCreate", func(t *testing.T) {
		registry := &model.Registry{Address: "gcr.io", Password: "plain"}

		mockStore.On("RegistryCreate", mock.AnythingOfType("*model.Registry")).Run(func(args mock.Arguments) {
			r, ok := args.Get(0).(*model.Registry)
			require.True(t, ok)
			r.ID = 2
		}).Return(nil).Once()
		mockEncryption.On("Encrypt", "plain", "2").Return("encrypted", nil).Once()
		mockStore.On("RegistryUpdate", mock.Anything).Return(nil).Once()
		mockEncryption.On("Decrypt", "encrypted", "2").Return("plain", nil).Once()

		err := composite.RegistryCreate(registry)
		require.NoError(t, err)
		assert.Equal(t, int64(2), registry.ID)
	})
}

func TestCompositeStore_UserOperations(t *testing.T) {
	composite, mockStore, mockEncryption := setupCompositeStore(t)

	t.Run("GetUser", func(t *testing.T) {
		user := &model.User{ID: 1, Login: "test", AccessToken: "encrypted-access", RefreshToken: "encrypted-refresh"}

		mockStore.On("GetUser", int64(1)).Return(user, nil).Once()
		mockEncryption.On("Decrypt", "encrypted-access", "1:access").Return("decrypted-access", nil).Once()
		mockEncryption.On("Decrypt", "encrypted-refresh", "1:refresh").Return("decrypted-refresh", nil).Once()

		result, err := composite.GetUser(1)
		require.NoError(t, err)
		assert.Equal(t, "decrypted-access", result.AccessToken)
		assert.Equal(t, "decrypted-refresh", result.RefreshToken)
	})

	t.Run("CreateUser", func(t *testing.T) {
		user := &model.User{Login: "newuser", AccessToken: "plain-access", RefreshToken: "plain-refresh"}

		mockStore.On("CreateUser", mock.AnythingOfType("*model.User")).Run(func(args mock.Arguments) {
			u, ok := args.Get(0).(*model.User)
			require.True(t, ok)
			u.ID = 2
		}).Return(nil).Once()
		mockEncryption.On("Encrypt", "plain-access", "2:access").Return("encrypted-access", nil).Once()
		mockEncryption.On("Encrypt", "plain-refresh", "2:refresh").Return("encrypted-refresh", nil).Once()
		mockStore.On("UpdateUser", mock.Anything).Return(nil).Once()
		mockEncryption.On("Decrypt", "encrypted-access", "2:access").Return("plain-access", nil).Once()
		mockEncryption.On("Decrypt", "encrypted-refresh", "2:refresh").Return("plain-refresh", nil).Once()

		err := composite.CreateUser(user)
		require.NoError(t, err)
		assert.Equal(t, int64(2), user.ID)
	})
}

func TestCompositeStore_ForgeOperations(t *testing.T) {
	composite, mockStore, mockEncryption := setupCompositeStore(t)

	t.Run("ForgeGet", func(t *testing.T) {
		forge := &model.Forge{ID: 1, Type: "github", ClientSecret: "encrypted"}

		mockStore.On("ForgeGet", int64(1)).Return(forge, nil).Once()
		mockEncryption.On("Decrypt", "encrypted", "1").Return("decrypted", nil).Once()

		result, err := composite.ForgeGet(1)
		require.NoError(t, err)
		assert.Equal(t, "decrypted", result.ClientSecret)
	})

	t.Run("ForgeCreate", func(t *testing.T) {
		forge := &model.Forge{Type: "gitlab", ClientSecret: "plain"}

		mockStore.On("ForgeCreate", mock.AnythingOfType("*model.Forge")).Run(func(args mock.Arguments) {
			f, ok := args.Get(0).(*model.Forge)
			require.True(t, ok)
			f.ID = 2
		}).Return(nil).Once()
		mockEncryption.On("Encrypt", "plain", "2").Return("encrypted", nil).Once()
		mockStore.On("ForgeUpdate", mock.Anything).Return(nil).Once()
		mockEncryption.On("Decrypt", "encrypted", "2").Return("plain", nil).Once()

		err := composite.ForgeCreate(forge)
		require.NoError(t, err)
		assert.Equal(t, int64(2), forge.ID)
	})
}

func TestCompositeStore_UserSecretList(t *testing.T) {
	composite, mockStore, mockEncryption := setupCompositeStore(t)

	secrets := []*model.Secret{
		{ID: 1, Name: "secret1", Value: "encrypted1"},
		{ID: 2, Name: "secret2", Value: "encrypted2"},
	}

	mockStore.On("UserSecretList", []int64{1, 2}, []int64{3, 4}, true, "search", mock.Anything).Return(secrets, nil)
	mockEncryption.On("Decrypt", "encrypted1", "1").Return("decrypted1", nil)
	mockEncryption.On("Decrypt", "encrypted2", "2").Return("decrypted2", nil)

	result, err := composite.UserSecretList([]int64{1, 2}, []int64{3, 4}, true, "search", nil)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "decrypted1", result[0].Value)
	assert.Equal(t, "decrypted2", result[1].Value)
}

func TestCompositeStore_PassthroughOperations(t *testing.T) {
	composite, mockStore, _ := setupCompositeStore(t)

	t.Run("GetRepo", func(t *testing.T) {
		repo := &model.Repo{ID: 1, Name: "test-repo"}
		mockStore.On("GetRepo", int64(1)).Return(repo, nil).Once()

		result, err := composite.GetRepo(1)
		require.NoError(t, err)
		assert.Equal(t, "test-repo", result.Name)
	})

	t.Run("Ping", func(t *testing.T) {
		mockStore.On("Ping").Return(nil).Once()

		err := composite.Ping()
		require.NoError(t, err)
	})

	t.Run("GetRepoCount", func(t *testing.T) {
		mockStore.On("GetRepoCount").Return(int64(42), nil).Once()

		count, err := composite.GetRepoCount()
		require.NoError(t, err)
		assert.Equal(t, int64(42), count)
	})
}
