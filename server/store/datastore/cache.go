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
	"context"
	"database/sql"
	"errors"

	"codefloe.com/crowci/crow/v5/server/model"
)

// CacheGet retrieves a cache entry by key.
func (s *storage) CacheGet(key string) (*model.CacheEntry, error) {
	entry := &model.CacheEntry{}
	err := s.db.NewSelect().Model(entry).Where("\"key\" = ?", key).Scan(context.Background())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCacheNotFound
		}
		return nil, err
	}
	return entry, nil
}

// CacheSet stores a cache entry.
func (s *storage) CacheSet(entry *model.CacheEntry) error {
	// Try insert first; if duplicate key, update instead
	_, err := s.db.NewInsert().Model(entry).Exec(context.Background())
	if err != nil {
		_, err = s.db.NewUpdate().Model(entry).Where("\"key\" = ?", entry.Key).Exec(context.Background())
	}
	return err
}

// CacheDelete removes a cache entry.
func (s *storage) CacheDelete(key string) error {
	_, err := s.db.NewDelete().Model((*model.CacheEntry)(nil)).Where("\"key\" = ?", key).Exec(context.Background())
	return err
}

// CacheCleanupExpired removes expired cache entries.
func (s *storage) CacheCleanupExpired(cutoff int64) error {
	_, err := s.db.NewDelete().Model((*model.CacheEntry)(nil)).Where("expires_at < ?", cutoff).Exec(context.Background())
	return err
}
