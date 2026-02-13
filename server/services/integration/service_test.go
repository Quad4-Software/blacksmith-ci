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

package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v5/server/integrations"
	intMocks "codefloe.com/crowci/crow/v5/server/integrations/mocks"
	"codefloe.com/crowci/crow/v5/server/model"
	modelMocks "codefloe.com/crowci/crow/v5/server/model/mocks"
)

func validVaultConfig() string {
	return `{"address":"https://vault.example.com","role_id":"test-role","secret_id":"test-secret","mount_path":"secret","auth_path":"auth/approle"}`
}

func TestNewService(t *testing.T) {
	store := modelMocks.NewMockIntegrationStore(t)
	factory := intMocks.NewMockProviderFactory(t)

	svc := NewService(store, factory)
	assert.NotNil(t, svc)
}

func TestIntegrationFind(t *testing.T) {
	t.Run("owner can find own integration", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)

		integration := &model.Integration{
			ID:     1,
			UserID: 100,
			Name:   "test-vault",
			Type:   model.IntegrationTypeVault,
		}

		store.EXPECT().IntegrationFind(int64(1)).Return(integration, nil)

		svc := NewService(store, factory)
		result, err := svc.IntegrationFind(100, 1)

		assert.NoError(t, err)
		assert.Equal(t, integration, result)
	})

	t.Run("non-owner cannot find integration", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)

		integration := &model.Integration{
			ID:     1,
			UserID: 100,
			Name:   "test-vault",
		}

		store.EXPECT().IntegrationFind(int64(1)).Return(integration, nil)

		svc := NewService(store, factory)
		_, err := svc.IntegrationFind(200, 1) // Different user

		assert.Error(t, err)
		assert.Equal(t, model.ErrIntegrationNotFound, err)
	})

	t.Run("store error propagates", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)

		store.EXPECT().IntegrationFind(int64(1)).Return(nil, fmt.Errorf("db error"))

		svc := NewService(store, factory)
		_, err := svc.IntegrationFind(100, 1)

		assert.Error(t, err)
	})
}

func TestIntegrationFindByID(t *testing.T) {
	store := modelMocks.NewMockIntegrationStore(t)
	factory := intMocks.NewMockProviderFactory(t)

	integration := &model.Integration{
		ID:   1,
		Name: "test-vault",
	}

	store.EXPECT().IntegrationFind(int64(1)).Return(integration, nil)

	svc := NewService(store, factory)
	result, err := svc.IntegrationFindByID(1)

	assert.NoError(t, err)
	assert.Equal(t, integration, result)
}

func TestIntegrationList(t *testing.T) {
	store := modelMocks.NewMockIntegrationStore(t)
	factory := intMocks.NewMockProviderFactory(t)

	integrations := []*model.Integration{
		{ID: 1, UserID: 100, Name: "vault-1"},
		{ID: 2, UserID: 100, Name: "vault-2"},
	}
	opts := &model.ListOptions{Page: 1, PerPage: 10}

	store.EXPECT().IntegrationList(int64(100), opts).Return(integrations, nil)

	svc := NewService(store, factory)
	result, err := svc.IntegrationList(100, opts)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestIntegrationListAccessible(t *testing.T) {
	store := modelMocks.NewMockIntegrationStore(t)
	factory := intMocks.NewMockProviderFactory(t)

	integrations := []*model.Integration{
		{ID: 1, Name: "vault-1"},
	}
	opts := &model.ListOptions{Page: 1, PerPage: 10}
	orgIDs := []int64{10, 20}
	repoIDs := []int64{100}

	store.EXPECT().IntegrationListAccessible(int64(1), orgIDs, repoIDs, opts).Return(integrations, nil)

	svc := NewService(store, factory)
	result, err := svc.IntegrationListAccessible(1, orgIDs, repoIDs, opts)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestIntegrationCreate(t *testing.T) {
	t.Run("valid integration", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)

		integration := &model.Integration{
			UserID: 100,
			Name:   "test-vault",
			Type:   model.IntegrationTypeVault,
			Config: validVaultConfig(),
		}

		store.EXPECT().IntegrationCreate(integration).Return(nil)

		svc := NewService(store, factory)
		err := svc.IntegrationCreate(integration)

		assert.NoError(t, err)
	})

	t.Run("invalid integration fails validation", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)

		integration := &model.Integration{
			UserID: 100,
			Name:   "", // Empty name - invalid
			Type:   model.IntegrationTypeVault,
			Config: validVaultConfig(),
		}

		svc := NewService(store, factory)
		err := svc.IntegrationCreate(integration)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})
}

func TestIntegrationUpdate(t *testing.T) {
	t.Run("valid update", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)

		integration := &model.Integration{
			ID:     1,
			UserID: 100,
			Name:   "updated-vault",
			Type:   model.IntegrationTypeVault,
			Config: validVaultConfig(),
		}

		store.EXPECT().IntegrationUpdate(integration).Return(nil)

		svc := NewService(store, factory)
		err := svc.IntegrationUpdate(integration)

		assert.NoError(t, err)
	})

	t.Run("invalid update fails validation", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)

		integration := &model.Integration{
			ID:     1,
			UserID: 100,
			Name:   "test",
			Type:   "invalid", // Invalid type
			Config: validVaultConfig(),
		}

		svc := NewService(store, factory)
		err := svc.IntegrationUpdate(integration)

		assert.Error(t, err)
	})
}

func TestIntegrationDelete(t *testing.T) {
	t.Run("owner can delete", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)

		integration := &model.Integration{
			ID:     1,
			UserID: 100,
			Name:   "test-vault",
		}

		store.EXPECT().IntegrationFind(int64(1)).Return(integration, nil)
		store.EXPECT().IntegrationDelete(integration).Return(nil)

		svc := NewService(store, factory)
		err := svc.IntegrationDelete(100, 1)

		assert.NoError(t, err)
	})

	t.Run("non-owner cannot delete", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)

		integration := &model.Integration{
			ID:     1,
			UserID: 100,
			Name:   "test-vault",
		}

		store.EXPECT().IntegrationFind(int64(1)).Return(integration, nil)

		svc := NewService(store, factory)
		err := svc.IntegrationDelete(200, 1) // Different user

		assert.Error(t, err)
		assert.Equal(t, model.ErrIntegrationNotFound, err)
	})
}

func TestCheckAccess(t *testing.T) {
	store := modelMocks.NewMockIntegrationStore(t)
	factory := intMocks.NewMockProviderFactory(t)
	svc := NewService(store, factory)

	t.Run("inactive integration denied", func(t *testing.T) {
		integration := &model.Integration{
			ID:       1,
			Name:     "test-vault",
			IsActive: false,
		}

		err := svc.CheckAccess(integration, 1, 0, 0)
		assert.Error(t, err)
		assert.ErrorIs(t, err, model.ErrIntegrationInactive)
		assert.Contains(t, err.Error(), "test-vault")
		assert.Contains(t, err.Error(), "disabled")
	})

	t.Run("owner in allowlist has access", func(t *testing.T) {
		integration := &model.Integration{
			ID:             1,
			UserID:         100,
			IsActive:       true,
			AllowedUserIDs: []int64{100}, // owner must be explicitly allowlisted
		}

		err := svc.CheckAccess(integration, 100, 0, 0)
		assert.NoError(t, err)
	})

	t.Run("owner without allowlist has no access", func(t *testing.T) {
		integration := &model.Integration{
			ID:       1,
			Name:     "test-vault",
			UserID:   100,
			IsActive: true,
			// no allowlists - owner cannot use for secrets
		}

		err := svc.CheckAccess(integration, 100, 0, 0)
		assert.Error(t, err)
		assert.ErrorIs(t, err, model.ErrIntegrationAccessDenied)
	})

	t.Run("user in allowlist has access", func(t *testing.T) {
		integration := &model.Integration{
			ID:             1,
			UserID:         100,
			IsActive:       true,
			AllowedUserIDs: []int64{200, 300},
		}

		err := svc.CheckAccess(integration, 200, 0, 0)
		assert.NoError(t, err)
	})

	t.Run("org in allowlist has access", func(t *testing.T) {
		integration := &model.Integration{
			ID:            1,
			UserID:        100,
			IsActive:      true,
			AllowedOrgIDs: []int64{10, 20},
		}

		err := svc.CheckAccess(integration, 200, 10, 0)
		assert.NoError(t, err)
	})

	t.Run("repo in allowlist has access", func(t *testing.T) {
		integration := &model.Integration{
			ID:             1,
			UserID:         100,
			IsActive:       true,
			AllowedRepoIDs: []int64{1000, 2000},
		}

		err := svc.CheckAccess(integration, 200, 0, 1000)
		assert.NoError(t, err)
	})

	t.Run("no access", func(t *testing.T) {
		integration := &model.Integration{
			ID:             1,
			Name:           "test-vault",
			UserID:         100,
			IsActive:       true,
			AllowedUserIDs: []int64{300},
			AllowedOrgIDs:  []int64{30},
			AllowedRepoIDs: []int64{3000},
		}

		err := svc.CheckAccess(integration, 200, 10, 1000)
		assert.Error(t, err)
		assert.ErrorIs(t, err, model.ErrIntegrationAccessDenied)
		assert.Contains(t, err.Error(), "test-vault")
		assert.Contains(t, err.Error(), "allowlist")
	})
}

func TestGetProvider(t *testing.T) {
	t.Run("inactive integration returns error", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)

		integration := &model.Integration{
			ID:       1,
			IsActive: false,
		}

		store.EXPECT().IntegrationFind(int64(1)).Return(integration, nil)

		svc := NewService(store, factory)
		_, err := svc.GetProvider(context.Background(), 1, 100)

		assert.Error(t, err)
		assert.ErrorIs(t, err, model.ErrIntegrationInactive)
	})

	t.Run("store error propagates", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)

		store.EXPECT().IntegrationFind(int64(1)).Return(nil, fmt.Errorf("not found"))

		svc := NewService(store, factory)
		_, err := svc.GetProvider(context.Background(), 1, 100)

		assert.Error(t, err)
	})
}

func TestFetchSecret(t *testing.T) {
	t.Run("access denied for inactive integration", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)

		integration := &model.Integration{
			ID:       1,
			Name:     "test-vault",
			UserID:   100,
			IsActive: false,
		}

		store.EXPECT().IntegrationFind(int64(1)).Return(integration, nil)

		svc := NewService(store, factory)
		ref := &model.ExternalSecretRef{
			IntegrationID: 1,
			Path:          "secret/test",
			Key:           "api_key",
		}

		_, err := svc.FetchSecret(context.Background(), ref, 100, 0, 0, 1000)
		assert.Error(t, err)
		assert.ErrorIs(t, err, model.ErrIntegrationInactive)
	})

	t.Run("access denied for non-permitted user", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)

		integration := &model.Integration{
			ID:             1,
			Name:           "test-vault",
			UserID:         100,
			IsActive:       true,
			AllowedUserIDs: []int64{300}, // User 200 not in list
		}

		store.EXPECT().IntegrationFind(int64(1)).Return(integration, nil)

		svc := NewService(store, factory)
		ref := &model.ExternalSecretRef{
			IntegrationID: 1,
			Path:          "secret/test",
			Key:           "api_key",
		}

		_, err := svc.FetchSecret(context.Background(), ref, 200, 0, 0, 1000)
		assert.Error(t, err)
		assert.ErrorIs(t, err, model.ErrIntegrationAccessDenied)
	})
}

func TestFetchSecretsBatch(t *testing.T) {
	t.Run("empty refs returns empty map", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)

		svc := NewService(store, factory)

		result, err := svc.FetchSecretsBatch(context.Background(), nil, 100, 0, 0, 1000)
		assert.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("access denied for non-permitted user in batch", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)

		integration := &model.Integration{
			ID:             1,
			Name:           "test-vault",
			UserID:         100,
			IsActive:       true,
			AllowedUserIDs: []int64{300},
		}

		store.EXPECT().IntegrationFind(int64(1)).Return(integration, nil)

		svc := NewService(store, factory)
		refs := []*model.ExternalSecretRef{
			{IntegrationID: 1, Path: "secret/test", Key: "key1"},
		}

		_, err := svc.FetchSecretsBatch(context.Background(), refs, 200, 0, 0, 1000)
		assert.Error(t, err)
		assert.ErrorIs(t, err, model.ErrIntegrationAccessDenied)
	})
}

func TestClose(t *testing.T) {
	store := modelMocks.NewMockIntegrationStore(t)
	factory := intMocks.NewMockProviderFactory(t)

	svc := NewService(store, factory)

	// Close should not panic even when pool is empty
	svc.Close()
}

func TestReleasePipeline(t *testing.T) {
	store := modelMocks.NewMockIntegrationStore(t)
	factory := intMocks.NewMockProviderFactory(t)

	svc := NewService(store, factory)

	// ReleasePipeline should not panic even when no providers exist
	svc.ReleasePipeline(1000)
}

func TestTestConnection(t *testing.T) {
	t.Run("successful connection test", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)
		provider := intMocks.NewMockProvider(t)

		integration := &model.Integration{
			ID:       1,
			Name:     "test-vault",
			Type:     model.IntegrationTypeVault,
			Config:   validVaultConfig(),
			IsActive: true,
		}

		factory.EXPECT().Create(integration).Return(provider, nil)
		provider.EXPECT().TestConnection(mock.Anything).Return(nil)
		provider.EXPECT().Close().Return(nil)

		svc := NewService(store, factory)
		err := svc.TestConnection(integration)

		assert.NoError(t, err)
	})

	t.Run("connection test failure", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)
		provider := intMocks.NewMockProvider(t)

		integration := &model.Integration{
			ID:       1,
			Name:     "test-vault",
			Type:     model.IntegrationTypeVault,
			Config:   validVaultConfig(),
			IsActive: true,
		}

		factory.EXPECT().Create(integration).Return(provider, nil)
		provider.EXPECT().TestConnection(mock.Anything).Return(integrations.ErrAuthenticationFailed)
		provider.EXPECT().Close().Return(nil) // Called when pool is closed

		svc := NewService(store, factory)
		err := svc.TestConnection(integration)

		assert.Error(t, err)
		assert.Equal(t, integrations.ErrAuthenticationFailed, err)

		// Clean up the pool - this will call Close() on cached providers
		svc.Close()
	})

	t.Run("provider creation failure", func(t *testing.T) {
		store := modelMocks.NewMockIntegrationStore(t)
		factory := intMocks.NewMockProviderFactory(t)

		integration := &model.Integration{
			ID:       1,
			Name:     "test-vault",
			Type:     model.IntegrationTypeVault,
			Config:   validVaultConfig(),
			IsActive: true,
		}

		factory.EXPECT().Create(integration).Return(nil, integrations.ErrProviderNotConfigured)

		svc := NewService(store, factory)
		err := svc.TestConnection(integration)

		assert.Error(t, err)
	})
}
