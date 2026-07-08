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

// Package cache provides a per-pipeline caching decorator for secret providers.
package cache

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/integrations"
	"codefloe.com/crowci/crow/v6/server/model"
)

// CachedProvider wraps a Provider with per-pipeline caching.
// This cache is scoped to a single pipeline execution and is cleared when Close() is called.
type CachedProvider struct {
	provider   integrations.Provider
	pipelineID int64
	cache      map[string]string
	mu         sync.RWMutex
	hits       int64
	misses     int64
}

// NewCachedProvider creates a new cached provider wrapper.
func NewCachedProvider(provider integrations.Provider, pipelineID int64) *CachedProvider {
	return &CachedProvider{
		provider:   provider,
		pipelineID: pipelineID,
		cache:      make(map[string]string),
	}
}

// Type returns the underlying provider's type.
func (c *CachedProvider) Type() model.IntegrationType {
	return c.provider.Type()
}

// GetSecret retrieves a secret, using the cache if available.
func (c *CachedProvider) GetSecret(ctx context.Context, path, key string, version int) (string, error) {
	cacheKey := makeCacheKey(path, key, version)

	// Check cache first
	c.mu.RLock()
	if value, found := c.cache[cacheKey]; found {
		c.mu.RUnlock()
		c.mu.Lock()
		c.hits++
		c.mu.Unlock()
		return value, nil
	}
	c.mu.RUnlock()

	// Cache miss - fetch from provider
	value, err := c.provider.GetSecret(ctx, path, key, version)
	if err != nil {
		return "", err
	}

	// Store in cache
	c.mu.Lock()
	c.cache[cacheKey] = value
	c.misses++
	c.mu.Unlock()

	return value, nil
}

// GetSecretsBatch retrieves multiple secrets, using the cache where available.
func (c *CachedProvider) GetSecretsBatch(ctx context.Context, requests []integrations.SecretRequest) (map[string]string, error) {
	results := make(map[string]string, len(requests))
	var misses []integrations.SecretRequest

	// Check cache for each request
	c.mu.RLock()
	for _, req := range requests {
		cacheKey := makeCacheKey(req.Path, req.Key, req.Version)
		if value, found := c.cache[cacheKey]; found {
			results[req.ID] = value
		} else {
			misses = append(misses, req)
		}
	}
	c.mu.RUnlock()

	// Update hit/miss counters
	c.mu.Lock()
	c.hits += int64(len(results))
	c.mu.Unlock()

	// If there are cache misses, fetch from provider
	if len(misses) > 0 {
		fetched, err := c.provider.GetSecretsBatch(ctx, misses)
		if err != nil {
			return nil, err
		}

		// Store fetched secrets in cache and results
		c.mu.Lock()
		for _, req := range misses {
			if value, ok := fetched[req.ID]; ok {
				cacheKey := makeCacheKey(req.Path, req.Key, req.Version)
				c.cache[cacheKey] = value
				results[req.ID] = value
			}
		}
		c.misses += int64(len(misses))
		c.mu.Unlock()
	}

	return results, nil
}

// TestConnection delegates to the underlying provider.
func (c *CachedProvider) TestConnection(ctx context.Context) error {
	return c.provider.TestConnection(ctx)
}

// Close clears the cache and closes the underlying provider.
func (c *CachedProvider) Close() error {
	c.mu.Lock()
	cacheSize := len(c.cache)
	hits := c.hits
	misses := c.misses
	c.cache = make(map[string]string) // Clear cache
	c.mu.Unlock()

	log.Debug().
		Int64("pipeline_id", c.pipelineID).
		Int("cache_size", cacheSize).
		Int64("cache_hits", hits).
		Int64("cache_misses", misses).
		Msg("Closing cached provider")

	return c.provider.Close()
}

// Stats returns cache statistics.
func (c *CachedProvider) Stats() (hits, misses int64, size int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses, len(c.cache)
}

// makeCacheKey creates a unique cache key from path, key, and version.
func makeCacheKey(path, key string, version int) string {
	if version > 0 {
		return path + ":" + key + ":v" + itoa(version)
	}
	return path + ":" + key
}

// itoa converts an int to string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	result := ""
	negative := i < 0
	if negative {
		i = -i
	}

	for i > 0 {
		result = string(rune('0'+i%10)) + result
		i /= 10
	}

	if negative {
		result = "-" + result
	}

	return result
}

// ProviderPool manages cached providers per pipeline.
type ProviderPool struct {
	factory   integrations.ProviderFactory
	providers map[poolKey]*CachedProvider
	mu        sync.RWMutex
}

type poolKey struct {
	integrationID int64
	pipelineID    int64
}

// NewProviderPool creates a new provider pool.
func NewProviderPool(factory integrations.ProviderFactory) *ProviderPool {
	return &ProviderPool{
		factory:   factory,
		providers: make(map[poolKey]*CachedProvider),
	}
}

// GetProvider returns a cached provider for the given integration and pipeline.
// Creates a new provider if one doesn't exist.
func (p *ProviderPool) GetProvider(ctx context.Context, integration *model.Integration, pipelineID int64) (*CachedProvider, error) {
	key := poolKey{
		integrationID: integration.ID,
		pipelineID:    pipelineID,
	}

	// Check if we already have a provider
	p.mu.RLock()
	if cached, ok := p.providers[key]; ok {
		p.mu.RUnlock()
		return cached, nil
	}
	p.mu.RUnlock()

	// Create new provider
	provider, err := p.factory.Create(integration)
	if err != nil {
		return nil, err
	}

	// Wrap in cache
	cached := NewCachedProvider(provider, pipelineID)

	// Store in pool
	p.mu.Lock()
	// Double-check someone else didn't create it while we were creating
	if existing, ok := p.providers[key]; ok {
		p.mu.Unlock()
		// Close the one we just created
		_ = cached.Close()
		return existing, nil
	}
	p.providers[key] = cached
	p.mu.Unlock()

	log.Debug().
		Int64("integration_id", integration.ID).
		Str("integration_name", integration.Name).
		Int64("pipeline_id", pipelineID).
		Msg("Created new cached provider")

	return cached, nil
}

// ReleasePipeline closes and removes all providers for a pipeline.
func (p *ProviderPool) ReleasePipeline(pipelineID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for key, cached := range p.providers {
		if key.pipelineID == pipelineID {
			if err := cached.Close(); err != nil {
				log.Warn().Err(err).
					Int64("integration_id", key.integrationID).
					Int64("pipeline_id", pipelineID).
					Msg("Error closing provider during pipeline release")
			}
			delete(p.providers, key)
		}
	}
}

// Close closes all providers in the pool.
func (p *ProviderPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for key, cached := range p.providers {
		if err := cached.Close(); err != nil {
			log.Warn().Err(err).
				Int64("integration_id", key.integrationID).
				Int64("pipeline_id", key.pipelineID).
				Msg("Error closing provider during pool close")
		}
	}

	p.providers = make(map[poolKey]*CachedProvider)
}
