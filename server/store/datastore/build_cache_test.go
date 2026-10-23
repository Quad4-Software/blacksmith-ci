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
		size, err := store.BuildCacheSize(model.BuildCacheFilter{RepoID: 1})
		require.NoError(t, err)
		assert.EqualValues(t, 1500, size)
		size, err = store.BuildCacheSize(model.BuildCacheFilter{BucketID: model.BuildCacheAnyStorage})
		require.NoError(t, err)
		assert.EqualValues(t, 2100, size)
	})

	t.Run("lru skips pending entries", func(t *testing.T) {
		entries, err := store.BuildCacheListLRU(model.BuildCacheFilter{RepoID: 1}, 2)
		require.NoError(t, err)
		require.Len(t, entries, 2)
		assert.Equal(t, newer.ID, entries[0].ID)
		assert.Equal(t, old.ID, entries[1].ID)

		entries, err = store.BuildCacheListLRU(model.BuildCacheFilter{BucketID: model.BuildCacheAnyStorage}, 1)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, other.ID, entries[0].ID)
	})

	t.Run("stale", func(t *testing.T) {
		entries, err := store.BuildCacheListStale(model.BuildCacheFilter{BucketID: model.BuildCacheAnyStorage}, 45, 35)
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

func TestBuildCacheOrgsAndBuckets(t *testing.T) {
	store, closer := newTestStore(t, new(model.BuildCache), new(model.Repo))
	defer closer()

	for _, repo := range []*model.Repo{
		{ID: 1, OrgID: 10, Owner: "a", Name: "one", FullName: "a/one", ForgeRemoteID: "1"},
		{ID: 2, OrgID: 10, Owner: "a", Name: "two", FullName: "a/two", ForgeRemoteID: "2"},
		{ID: 3, OrgID: 20, Owner: "b", Name: "three", FullName: "b/three", ForgeRemoteID: "3"},
	} {
		_, err := store.db.NewInsert().Model(repo).Exec(t.Context())
		require.NoError(t, err)
	}
	create := func(repoID, bucketID int64, key string, size, lastUsed int64) *model.BuildCache {
		entry := &model.BuildCache{RepoID: repoID, BucketID: bucketID, Scope: "refs/heads/main", Key: key, Size: size, Committed: true, LastUsed: lastUsed}
		require.NoError(t, store.BuildCacheCreate(entry))
		return entry
	}
	a := create(1, 0, "a", 100, 10)
	b := create(2, 0, "b", 200, 20)
	own := create(2, 7, "c", 400, 5)
	other := create(3, 0, "d", 800, 1)

	t.Run("size per org and storage", func(t *testing.T) {
		size, err := store.BuildCacheSize(model.BuildCacheFilter{OrgID: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 300, size, "the zero bucket selects the server's storage")

		size, err = store.BuildCacheSize(model.BuildCacheFilter{OrgID: 10, BucketID: model.BuildCacheOwnBuckets})
		require.NoError(t, err)
		assert.EqualValues(t, 400, size)

		size, err = store.BuildCacheSize(model.BuildCacheFilter{BucketID: model.BuildCacheAnyStorage, NotOrgIDs: []int64{10}})
		require.NoError(t, err)
		assert.EqualValues(t, 800, size)
	})

	t.Run("lru within an org", func(t *testing.T) {
		entries, err := store.BuildCacheListLRU(model.BuildCacheFilter{OrgID: 10}, 10)
		require.NoError(t, err)
		require.Len(t, entries, 2)
		assert.Equal(t, a.ID, entries[0].ID)
		assert.Equal(t, b.ID, entries[1].ID)
	})

	t.Run("stale excluding buckets", func(t *testing.T) {
		entries, err := store.BuildCacheListStale(model.BuildCacheFilter{BucketID: model.BuildCacheOwnBuckets, NotBucketIDs: []int64{8}}, 100, 0)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, own.ID, entries[0].ID)

		entries, err = store.BuildCacheListStale(model.BuildCacheFilter{BucketID: model.BuildCacheOwnBuckets, NotBucketIDs: []int64{7}}, 100, 0)
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("list filter", func(t *testing.T) {
		entries, err := store.BuildCacheListFilter(model.BuildCacheFilter{OrgID: 20, BucketID: model.BuildCacheAnyStorage})
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, other.ID, entries[0].ID)
	})

	t.Run("sizes by org", func(t *testing.T) {
		sizes, err := store.BuildCacheSizesByOrg()
		require.NoError(t, err)
		got := map[[2]int64]model.BuildCacheSize{}
		for _, size := range sizes {
			var own int64
			if size.Own {
				own = 1
			}
			got[[2]int64{size.OrgID, own}] = *size
		}
		assert.Equal(t, model.BuildCacheSize{OrgID: 10, Size: 300, Entries: 2, Repos: 2}, got[[2]int64{10, 0}])
		assert.Equal(t, model.BuildCacheSize{OrgID: 10, Own: true, Size: 400, Entries: 1, Repos: 1}, got[[2]int64{10, 1}])
		assert.Equal(t, model.BuildCacheSize{OrgID: 20, Size: 800, Entries: 1, Repos: 1}, got[[2]int64{20, 0}])
	})

	t.Run("sizes by repo", func(t *testing.T) {
		sizes, err := store.BuildCacheSizesByRepo(10)
		require.NoError(t, err)
		assert.Len(t, sizes, 3)
		var total int64
		for _, size := range sizes {
			total += size.Size
		}
		assert.EqualValues(t, 700, total)
	})
}

func TestBuildCacheSettings(t *testing.T) {
	store, closer := newTestStore(t, new(model.BuildCacheSettings))
	defer closer()

	_, err := store.BuildCacheSettingsGet(1)
	assert.ErrorIs(t, err, types.RecordNotExist)

	require.NoError(t, store.BuildCacheSettingsSave(&model.BuildCacheSettings{OrgID: 1, MaxSize: 10, TTL: 60}))
	require.NoError(t, store.BuildCacheSettingsSave(&model.BuildCacheSettings{OrgID: 1, MaxSize: 20, MaxEgress: -1, TTL: 0}))
	require.NoError(t, store.BuildCacheSettingsSave(&model.BuildCacheSettings{OrgID: 2, MaxSize: 5}))

	got, err := store.BuildCacheSettingsGet(1)
	require.NoError(t, err)
	assert.EqualValues(t, 20, got.MaxSize, "saving again replaces the settings")
	assert.EqualValues(t, -1, got.MaxEgress)
	assert.EqualValues(t, 0, got.TTL)

	all, err := store.BuildCacheSettingsList()
	require.NoError(t, err)
	assert.Len(t, all, 2)

	require.NoError(t, store.BuildCacheSettingsDelete(1))
	_, err = store.BuildCacheSettingsGet(1)
	assert.ErrorIs(t, err, types.RecordNotExist)
}

func TestBuildCacheBuckets(t *testing.T) {
	store, closer := newTestStore(t, new(model.BuildCacheBucket))
	defer closer()

	bucket := &model.BuildCacheBucket{OrgID: 1, Endpoint: "s3.example.com", Bucket: "cache", SecretKey: "secret", SSL: true}
	require.NoError(t, store.BuildCacheBucketCreate(bucket))
	assert.ErrorIs(t, store.BuildCacheBucketCreate(&model.BuildCacheBucket{OrgID: 1, Endpoint: "x", Bucket: "y"}), types.ErrConflict)

	got, err := store.BuildCacheBucketFind(1)
	require.NoError(t, err)
	assert.Equal(t, bucket.ID, got.ID)
	assert.Equal(t, "secret", got.SecretKey)

	plain := &model.BuildCacheBucket{OrgID: 2, Endpoint: "minio:9000", Bucket: "cache", SSL: false}
	require.NoError(t, store.BuildCacheBucketCreate(plain))
	got, err = store.BuildCacheBucketGet(plain.ID)
	require.NoError(t, err)
	assert.False(t, got.SSL, "false is stored, not replaced by a default")

	bucket.TTL = 3600
	require.NoError(t, store.BuildCacheBucketUpdate(bucket))
	got, err = store.BuildCacheBucketGet(bucket.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 3600, got.TTL)

	all, err := store.BuildCacheBucketList()
	require.NoError(t, err)
	assert.Len(t, all, 2)

	require.NoError(t, store.BuildCacheBucketDelete(bucket.ID))
	_, err = store.BuildCacheBucketFind(1)
	assert.ErrorIs(t, err, types.RecordNotExist)
}

func TestBuildCacheUsage(t *testing.T) {
	store, closer := newTestStore(t, new(model.BuildCacheUsage))
	defer closer()

	add := func(repoID, orgID, bucketID, day int64, counts model.BuildCacheUsageCounts) {
		require.NoError(t, store.BuildCacheUsageAdd(&model.BuildCacheUsage{RepoID: repoID, OrgID: orgID, BucketID: bucketID, Day: day, BuildCacheUsageCounts: counts}))
	}
	add(1, 10, 0, 100, model.BuildCacheUsageCounts{Hits: 1, Egress: 50})
	add(1, 10, 0, 100, model.BuildCacheUsageCounts{Hits: 1, Misses: 2, Egress: 25})
	add(1, 10, 0, 200, model.BuildCacheUsageCounts{PartialHits: 1, Saves: 1, Ingress: 70})
	add(2, 10, 7, 200, model.BuildCacheUsageCounts{Hits: 3, Egress: 1000})
	add(3, 20, 0, 200, model.BuildCacheUsageCounts{Misses: 4, Denied: 1})

	t.Run("adding to the same day increments", func(t *testing.T) {
		sum, err := store.BuildCacheUsageSum(model.BuildCacheUsageFilter{RepoID: 1, Since: 100})
		require.NoError(t, err)
		assert.Equal(t, model.BuildCacheUsageCounts{Hits: 2, PartialHits: 1, Misses: 2, Saves: 1, Egress: 75, Ingress: 70}, *sum)
	})

	t.Run("since and storage", func(t *testing.T) {
		sum, err := store.BuildCacheUsageSum(model.BuildCacheUsageFilter{OrgID: 10, Since: 150})
		require.NoError(t, err)
		assert.EqualValues(t, 0, sum.Egress, "own bucket traffic is not server egress")
		sum, err = store.BuildCacheUsageSum(model.BuildCacheUsageFilter{OrgID: 10, BucketID: model.BuildCacheAnyStorage})
		require.NoError(t, err)
		assert.EqualValues(t, 1075, sum.Egress)
	})

	t.Run("grouped", func(t *testing.T) {
		byOrg, err := store.BuildCacheUsageByOrg(model.BuildCacheUsageFilter{BucketID: model.BuildCacheAnyStorage})
		require.NoError(t, err)
		assert.Len(t, byOrg, 3, "org 10 server, org 10 own and org 20 server")

		byRepo, err := store.BuildCacheUsageByRepo(model.BuildCacheUsageFilter{OrgID: 10, BucketID: model.BuildCacheAnyStorage})
		require.NoError(t, err)
		assert.Len(t, byRepo, 2)

		byDay, err := store.BuildCacheUsageByDay(model.BuildCacheUsageFilter{OrgID: 10, BucketID: model.BuildCacheAnyStorage})
		require.NoError(t, err)
		require.Len(t, byDay, 2)
		assert.EqualValues(t, 100, byDay[0].Day)
		assert.EqualValues(t, 2, byDay[0].Hits)
		assert.EqualValues(t, 3, byDay[1].Hits)
	})

	t.Run("prune", func(t *testing.T) {
		require.NoError(t, store.BuildCacheUsagePrune(150))
		sum, err := store.BuildCacheUsageSum(model.BuildCacheUsageFilter{BucketID: model.BuildCacheAnyStorage})
		require.NoError(t, err)
		assert.EqualValues(t, 3, sum.Hits, "only the day 200 hits are left")
	})
}
