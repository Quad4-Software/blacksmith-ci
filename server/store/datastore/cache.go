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

package datastore

import (
	"codeberg.org/crowci/crow/v4/server/model"
)

// CacheGet retrieves a cache entry by key.
func (s *storage) CacheGet(key string) (*model.CacheEntry, error) {
	entry := &model.CacheEntry{}
	has, err := s.engine.Where("key = ?", key).Get(entry)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrCacheNotFound
	}
	return entry, nil
}

// CacheSet stores a cache entry.
func (s *storage) CacheSet(entry *model.CacheEntry) error {
	// Use INSERT OR REPLACE to handle both insert and update
	_, err := s.engine.Insert(entry)
	if err != nil {
		// If insert fails due to duplicate key, update instead
		_, err = s.engine.Where("key = ?", entry.Key).Update(entry)
	}
	return err
}

// CacheDelete removes a cache entry.
func (s *storage) CacheDelete(key string) error {
	_, err := s.engine.Where("key = ?", key).Delete(&model.CacheEntry{})
	return err
}

// CacheCleanupExpired removes expired cache entries.
func (s *storage) CacheCleanupExpired(cutoff int64) error {
	_, err := s.engine.Where("expires_at < ?", cutoff).Delete(&model.CacheEntry{})
	return err
}
