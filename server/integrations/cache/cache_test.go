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

package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v5/server/integrations"
	intMocks "codefloe.com/crowci/crow/v5/server/integrations/mocks"
	"codefloe.com/crowci/crow/v5/server/model"
)

func TestMakeCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		key      string
		version  int
		expected string
	}{
		{
			name:     "without version",
			path:     "secret/test",
			key:      "api_key",
			version:  0,
			expected: "secret/test:api_key",
		},
		{
			name:     "with version",
			path:     "secret/test",
			key:      "api_key",
			version:  5,
			expected: "secret/test:api_key:v5",
		},
		{
			name:     "complex path",
			path:     "secret/data/prod/database",
			key:      "password",
			version:  10,
			expected: "secret/data/prod/database:password:v10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := makeCacheKey(tt.path, tt.key, tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{123, "123"},
		{-1, "-1"},
		{-123, "-123"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := itoa(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCachedProvider_Type(t *testing.T) {
	provider := intMocks.NewMockProvider(t)
	provider.EXPECT().Type().Return(model.IntegrationTypeVault)

	cached := NewCachedProvider(provider, 1000)
	assert.Equal(t, model.IntegrationTypeVault, cached.Type())
}

func TestCachedProvider_GetSecret(t *testing.T) {
	t.Run("cache miss then hit", func(t *testing.T) {
		provider := intMocks.NewMockProvider(t)

		// First call should hit the provider
		provider.EXPECT().GetSecret(mock.Anything, "secret/test", "api_key", 0).
			Return("secret_value", nil).Once()

		cached := NewCachedProvider(provider, 1000)

		// First call - cache miss
		value1, err := cached.GetSecret(context.Background(), "secret/test", "api_key", 0)
		assert.NoError(t, err)
		assert.Equal(t, "secret_value", value1)

		// Second call - cache hit (provider not called again)
		value2, err := cached.GetSecret(context.Background(), "secret/test", "api_key", 0)
		assert.NoError(t, err)
		assert.Equal(t, "secret_value", value2)

		// Verify stats
		hits, misses, size := cached.Stats()
		assert.Equal(t, int64(1), hits)
		assert.Equal(t, int64(1), misses)
		assert.Equal(t, 1, size)
	})

	t.Run("different versions are cached separately", func(t *testing.T) {
		provider := intMocks.NewMockProvider(t)

		provider.EXPECT().GetSecret(mock.Anything, "secret/test", "key", 1).
			Return("v1_value", nil).Once()
		provider.EXPECT().GetSecret(mock.Anything, "secret/test", "key", 2).
			Return("v2_value", nil).Once()

		cached := NewCachedProvider(provider, 1000)

		// Get version 1
		v1, err := cached.GetSecret(context.Background(), "secret/test", "key", 1)
		assert.NoError(t, err)
		assert.Equal(t, "v1_value", v1)

		// Get version 2
		v2, err := cached.GetSecret(context.Background(), "secret/test", "key", 2)
		assert.NoError(t, err)
		assert.Equal(t, "v2_value", v2)

		_, _, size := cached.Stats()
		assert.Equal(t, 2, size)
	})

	t.Run("error from provider", func(t *testing.T) {
		provider := intMocks.NewMockProvider(t)

		provider.EXPECT().GetSecret(mock.Anything, "secret/test", "key", 0).
			Return("", integrations.ErrSecretNotFound)

		cached := NewCachedProvider(provider, 1000)

		_, err := cached.GetSecret(context.Background(), "secret/test", "key", 0)
		assert.Error(t, err)
		assert.Equal(t, integrations.ErrSecretNotFound, err)
	})
}

func TestCachedProvider_GetSecretsBatch(t *testing.T) {
	t.Run("batch with partial cache hits", func(t *testing.T) {
		provider := intMocks.NewMockProvider(t)

		// First populate the cache with one secret
		provider.EXPECT().GetSecret(mock.Anything, "secret/test", "key1", 0).
			Return("value1", nil).Once()

		cached := NewCachedProvider(provider, 1000)

		// Prime the cache
		_, _ = cached.GetSecret(context.Background(), "secret/test", "key1", 0)

		// Now request a batch that includes the cached key and a new one
		requests := []integrations.SecretRequest{
			{ID: "1:secret/test:key1:0", Path: "secret/test", Key: "key1", Version: 0},
			{ID: "1:secret/test:key2:0", Path: "secret/test", Key: "key2", Version: 0},
		}

		// Only key2 should hit the provider
		provider.EXPECT().GetSecretsBatch(mock.Anything, []integrations.SecretRequest{
			{ID: "1:secret/test:key2:0", Path: "secret/test", Key: "key2", Version: 0},
		}).Return(map[string]string{
			"1:secret/test:key2:0": "value2",
		}, nil).Once()

		results, err := cached.GetSecretsBatch(context.Background(), requests)
		assert.NoError(t, err)
		assert.Equal(t, "value1", results["1:secret/test:key1:0"])
		assert.Equal(t, "value2", results["1:secret/test:key2:0"])
	})

	t.Run("all cache hits", func(t *testing.T) {
		provider := intMocks.NewMockProvider(t)

		// Prime cache
		provider.EXPECT().GetSecret(mock.Anything, "secret/test", "key1", 0).
			Return("value1", nil).Once()
		provider.EXPECT().GetSecret(mock.Anything, "secret/test", "key2", 0).
			Return("value2", nil).Once()

		cached := NewCachedProvider(provider, 1000)

		_, _ = cached.GetSecret(context.Background(), "secret/test", "key1", 0)
		_, _ = cached.GetSecret(context.Background(), "secret/test", "key2", 0)

		// Batch request - all from cache
		requests := []integrations.SecretRequest{
			{ID: "1:secret/test:key1:0", Path: "secret/test", Key: "key1", Version: 0},
			{ID: "1:secret/test:key2:0", Path: "secret/test", Key: "key2", Version: 0},
		}

		results, err := cached.GetSecretsBatch(context.Background(), requests)
		assert.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("all cache misses", func(t *testing.T) {
		provider := intMocks.NewMockProvider(t)

		requests := []integrations.SecretRequest{
			{ID: "req1", Path: "secret/test", Key: "key1", Version: 0},
			{ID: "req2", Path: "secret/test", Key: "key2", Version: 0},
		}

		provider.EXPECT().GetSecretsBatch(mock.Anything, requests).
			Return(map[string]string{
				"req1": "value1",
				"req2": "value2",
			}, nil).Once()

		cached := NewCachedProvider(provider, 1000)

		results, err := cached.GetSecretsBatch(context.Background(), requests)
		assert.NoError(t, err)
		assert.Len(t, results, 2)
	})
}

func TestCachedProvider_TestConnection(t *testing.T) {
	provider := intMocks.NewMockProvider(t)
	provider.EXPECT().TestConnection(mock.Anything).Return(nil)

	cached := NewCachedProvider(provider, 1000)
	err := cached.TestConnection(context.Background())
	assert.NoError(t, err)
}

func TestCachedProvider_Close(t *testing.T) {
	provider := intMocks.NewMockProvider(t)

	// Prime cache
	provider.EXPECT().GetSecret(mock.Anything, "test", "key", 0).
		Return("value", nil).Once()
	provider.EXPECT().Close().Return(nil)

	cached := NewCachedProvider(provider, 1000)

	_, _ = cached.GetSecret(context.Background(), "test", "key", 0)

	_, _, size := cached.Stats()
	assert.Equal(t, 1, size)

	err := cached.Close()
	assert.NoError(t, err)

	// Cache should be cleared
	_, _, size = cached.Stats()
	assert.Equal(t, 0, size)
}

func TestProviderPool_GetProvider(t *testing.T) {
	t.Run("creates and caches provider", func(t *testing.T) {
		factory := intMocks.NewMockProviderFactory(t)
		provider := intMocks.NewMockProvider(t)

		integration := &model.Integration{
			ID:   1,
			Name: "test",
		}

		// Should only be called once
		factory.EXPECT().Create(integration).Return(provider, nil).Once()

		pool := NewProviderPool(factory)

		// First call creates provider
		p1, err := pool.GetProvider(context.Background(), integration, 1000)
		assert.NoError(t, err)
		assert.NotNil(t, p1)

		// Second call returns cached provider
		p2, err := pool.GetProvider(context.Background(), integration, 1000)
		assert.NoError(t, err)
		assert.Equal(t, p1, p2)

		// Clean up
		provider.EXPECT().Close().Return(nil)
		pool.Close()
	})

	t.Run("different pipelines get different providers", func(t *testing.T) {
		factory := intMocks.NewMockProviderFactory(t)
		provider1 := intMocks.NewMockProvider(t)
		provider2 := intMocks.NewMockProvider(t)

		integration := &model.Integration{
			ID:   1,
			Name: "test",
		}

		factory.EXPECT().Create(integration).Return(provider1, nil).Once()
		factory.EXPECT().Create(integration).Return(provider2, nil).Once()

		pool := NewProviderPool(factory)

		p1, _ := pool.GetProvider(context.Background(), integration, 1000)
		p2, _ := pool.GetProvider(context.Background(), integration, 2000)

		assert.NotEqual(t, p1, p2)

		// Clean up
		provider1.EXPECT().Close().Return(nil)
		provider2.EXPECT().Close().Return(nil)
		pool.Close()
	})
}

func TestProviderPool_ReleasePipeline(t *testing.T) {
	factory := intMocks.NewMockProviderFactory(t)
	provider1 := intMocks.NewMockProvider(t)
	provider2 := intMocks.NewMockProvider(t)

	integration := &model.Integration{
		ID:   1,
		Name: "test",
	}

	factory.EXPECT().Create(integration).Return(provider1, nil).Once()
	factory.EXPECT().Create(integration).Return(provider2, nil).Once()

	pool := NewProviderPool(factory)

	// Create providers for two pipelines
	_, _ = pool.GetProvider(context.Background(), integration, 1000)
	_, _ = pool.GetProvider(context.Background(), integration, 2000)

	// Release pipeline 1000
	provider1.EXPECT().Close().Return(nil)
	pool.ReleasePipeline(1000)

	// Pipeline 2000 should still have its provider
	provider2.EXPECT().Close().Return(nil)
	pool.Close()
}

func TestProviderPool_Close(t *testing.T) {
	factory := intMocks.NewMockProviderFactory(t)
	provider := intMocks.NewMockProvider(t)

	integration := &model.Integration{
		ID:   1,
		Name: "test",
	}

	factory.EXPECT().Create(integration).Return(provider, nil)
	provider.EXPECT().Close().Return(nil)

	pool := NewProviderPool(factory)
	_, _ = pool.GetProvider(context.Background(), integration, 1000)

	pool.Close()
}
