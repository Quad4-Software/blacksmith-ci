// Copyright 2025 Crow CI Authors
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
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v5/server/forge"
	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/store"
)

const (
	// Cache TTL for membership entries.
	membershipCacheTTL = 10 * time.Minute

	// Cleanup interval for expired cache entries.
	cacheCleanupInterval = 5 * time.Minute
)

// distributedMembershipCache implements a distributed membership cache using database storage.
type distributedMembershipCache struct {
	store  store.Store
	ttl    time.Duration
	ctx    context.Context
	cancel context.CancelCauseFunc
}

// NewDistributedMembershipService creates a new distributed membership service.
func NewDistributedMembershipService(_store store.Store) MembershipService {
	ctx, cancel := context.WithCancelCause(context.Background())

	d := &distributedMembershipCache{
		store:  _store,
		ttl:    membershipCacheTTL,
		ctx:    ctx,
		cancel: cancel,
	}

	// Start background cleanup process
	go d.cleanupExpired()

	return d
}

// Get returns if the user is a member of the organization.
func (d *distributedMembershipCache) Get(ctx context.Context, _forge forge.Forge, u *model.User, org string) (*model.OrgPerm, error) {
	key := fmt.Sprintf("%s-%s", u.ForgeRemoteID, org)

	// Try to get from cache first
	if cachedPerm, err := d.getCached(key); err == nil && cachedPerm != nil {
		return cachedPerm, nil
	}

	// Cache miss - fetch from forge
	perm, err := _forge.OrgMembership(ctx, u, org)
	if err != nil {
		return nil, err
	}

	// Store in cache
	if err := d.setCached(key, perm); err != nil {
		log.Error().Err(err).Str("key", key).Msg("Failed to cache membership")
	}

	return perm, nil
}

// Close shuts down the distributed membership cache.
func (d *distributedMembershipCache) Close() error {
	d.cancel(nil)
	return nil
}

// getCached retrieves a cached membership entry.
func (d *distributedMembershipCache) getCached(key string) (*model.OrgPerm, error) {
	entry, err := d.store.CacheGet(key)
	if err != nil {
		return nil, err
	}

	// Check if expired
	if time.Unix(entry.ExpiresAt, 0).Before(time.Now()) {
		// Entry is expired, delete it
		_ = d.store.CacheDelete(key)
		return nil, fmt.Errorf("cache entry expired")
	}

	// Deserialize the cached permission
	perm, err := deserializeOrgPerm(entry.Data)
	if err != nil {
		// If we can't deserialize, delete the corrupt entry
		_ = d.store.CacheDelete(key)
		return nil, err
	}

	return perm, nil
}

// setCached stores a membership entry in the cache.
func (d *distributedMembershipCache) setCached(key string, perm *model.OrgPerm) error {
	data := serializeOrgPerm(perm)

	entry := &model.CacheEntry{
		Key:       key,
		Data:      data,
		ExpiresAt: time.Now().Add(d.ttl).Unix(),
		CreatedAt: time.Now().Unix(),
	}

	return d.store.CacheSet(entry)
}

// cleanupExpired removes expired cache entries.
func (d *distributedMembershipCache) cleanupExpired() {
	ticker := time.NewTicker(cacheCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Unix()
			if err := d.store.CacheCleanupExpired(cutoff); err != nil {
				log.Error().Err(err).Msg("Failed to cleanup expired cache entries")
			}
		}
	}
}

// serializeOrgPerm converts an OrgPerm to bytes for storage.
func serializeOrgPerm(perm *model.OrgPerm) []byte {
	// Simple serialization: "member:admin" format
	result := fmt.Sprintf("%t:%t", perm.Member, perm.Admin)
	return []byte(result)
}

// deserializeOrgPerm converts bytes back to an OrgPerm.
func deserializeOrgPerm(data []byte) (*model.OrgPerm, error) {
	str := string(data)
	var member, admin bool

	_, err := fmt.Sscanf(str, "%t:%t", &member, &admin)
	if err != nil {
		return nil, err
	}

	return &model.OrgPerm{
		Member: member,
		Admin:  admin,
	}, nil
}
