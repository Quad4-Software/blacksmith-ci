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

package datastore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

func TestBuildCaches(t *testing.T) {
	store, closer := newTestStore(t, new(model.BuildCache))
	defer closer()

	const main = "refs/heads/main"
	create := func(repoID int64, scope, key string, size int64, committed bool, created, lastUsed int64) *model.BuildCache {
		entry := &model.BuildCache{RepoID: repoID, Scope: scope, Key: key, Size: size, Committed: committed, Created: created, LastUsed: lastUsed}
		require.NoError(t, store.BuildCacheCreate(entry))
		return entry
	}

	old := create(1, main, "go-aaa", 100, true, 10, 50)
	newer := create(1, main, "go-bbb", 200, true, 20, 40)
	pending := create(1, main, "go-ccc", 300, false, 30, 30)
	create(1, main, "GO-ddd", 400, true, 40, 60)
	create(1, main, "go_eee", 500, true, 50, 70)
	other := create(2, main, "go-aaa", 600, true, 10, 10)

	var devPending *model.BuildCache
	t.Run("unique per repo, scope and key", func(t *testing.T) {
		err := store.BuildCacheCreate(&model.BuildCache{RepoID: 1, Scope: main, Key: "go-aaa"})
		assert.ErrorIs(t, err, types.ErrConflict)
		devPending = create(1, "refs/heads/dev", "go-aaa", 0, false, 0, 0)
	})

	t.Run("find", func(t *testing.T) {
		got, err := store.BuildCacheFind(1, main, "go-aaa")
		require.NoError(t, err)
		assert.Equal(t, old.ID, got.ID)
		_, err = store.BuildCacheFind(1, main, "go-zzz")
		assert.ErrorIs(t, err, types.RecordNotExist)
	})

	t.Run("prefix picks newest committed and respects case and wildcards", func(t *testing.T) {
		got, err := store.BuildCacheFindPrefix(1, main, "go-")
		require.NoError(t, err)
		assert.Equal(t, newer.ID, got.ID, "the pending go-ccc must be skipped")

		got, err = store.BuildCacheFindPrefix(1, main, "go_")
		require.NoError(t, err)
		assert.Equal(t, "go_eee", got.Key, "_ must not act as a wildcard")

		_, err = store.BuildCacheFindPrefix(1, main, "Go-")
		assert.ErrorIs(t, err, types.RecordNotExist, "matching is case-sensitive")
	})

	t.Run("sizes", func(t *testing.T) {
		size, err := store.BuildCacheSize(1)
		require.NoError(t, err)
		assert.EqualValues(t, 1500, size)
		size, err = store.BuildCacheSize(0)
		require.NoError(t, err)
		assert.EqualValues(t, 2100, size)
	})

	t.Run("lru skips pending entries", func(t *testing.T) {
		entries, err := store.BuildCacheListLRU(1, 2)
		require.NoError(t, err)
		require.Len(t, entries, 2)
		assert.Equal(t, newer.ID, entries[0].ID)
		assert.Equal(t, old.ID, entries[1].ID)

		entries, err = store.BuildCacheListLRU(0, 1)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, other.ID, entries[0].ID)
	})

	t.Run("stale", func(t *testing.T) {
		entries, err := store.BuildCacheListStale(45, 35)
		require.NoError(t, err)
		ids := make([]int64, 0, len(entries))
		for _, entry := range entries {
			ids = append(ids, entry.ID)
		}
		assert.ElementsMatch(t, []int64{newer.ID, pending.ID, other.ID, devPending.ID}, ids)
	})

	t.Run("update, list and delete", func(t *testing.T) {
		pending.Committed = true
		pending.LastUsed = 99
		require.NoError(t, store.BuildCacheUpdate(pending))

		entries, err := store.BuildCacheList(1)
		require.NoError(t, err)
		require.NotEmpty(t, entries)
		assert.Equal(t, pending.ID, entries[0].ID, "most recently used first")

		require.NoError(t, store.BuildCacheDelete(pending.ID))
		assert.ErrorIs(t, store.BuildCacheDelete(pending.ID), types.RecordNotExist)
		_, err = store.BuildCacheGet(pending.ID)
		assert.ErrorIs(t, err, types.RecordNotExist)
	})
}
