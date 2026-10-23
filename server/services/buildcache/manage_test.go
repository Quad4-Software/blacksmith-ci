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

package buildcache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

// org creates an org with one repo per name and returns their accesses.
func (f *fixture) org(t *testing.T, name string, repos ...string) (*model.Org, []Access) {
	t.Helper()
	org := &model.Org{Name: name, ForgeID: 1}
	require.NoError(t, f.store.OrgCreate(org))
	accesses := make([]Access, 0, len(repos))
	for _, repoName := range repos {
		repo := &model.Repo{
			OrgID: org.ID, ForgeID: 1, Owner: name, Name: repoName, FullName: name + "/" + repoName,
			ForgeRemoteID: model.ForgeRemoteID(name + "/" + repoName), Branch: "main",
		}
		require.NoError(t, f.store.CreateRepo(repo))
		accesses = append(accesses, Access{
			RepoID: repo.ID, OrgID: org.ID, ReadScopes: []string{"refs/heads/main"}, WriteScope: "refs/heads/main",
		})
	}
	return org, accesses
}

func (f *fixture) usage(t *testing.T, filter model.BuildCacheUsageFilter) model.BuildCacheUsageCounts {
	t.Helper()
	counts, err := f.store.BuildCacheUsageSum(filter)
	require.NoError(t, err)
	return *counts
}

func (f *fixture) exists(id int64) bool {
	_, err := f.store.BuildCacheGet(id)
	return !errors.Is(err, types.RecordNotExist)
}

func TestUsageIsRecorded(t *testing.T) {
	f := newFixture(t, Config{})
	ctx := context.Background()
	_, repos := f.org(t, "acme", "app")
	access := repos[0]

	_, err := f.service.Restore(ctx, access, "go-1", []string{"go-"}, false)
	require.ErrorIs(t, err, ErrNotFound)
	f.save(ctx, t, access, "go-1", "archive")

	_, err = f.service.Restore(ctx, access, "go-1", nil, false)
	require.NoError(t, err)
	_, err = f.service.Restore(ctx, access, "go-2", []string{"go-"}, false)
	require.NoError(t, err)
	// The existence check of a save is no restore, and cannot download.
	checked, err := f.service.Restore(ctx, access, "go-1", nil, true)
	require.NoError(t, err)
	assert.Empty(t, checked.URL)
	_, err = f.service.Restore(ctx, access, "go-3", nil, true)
	require.ErrorIs(t, err, ErrNotFound)

	size := int64(len("archive"))
	assert.Equal(t, model.BuildCacheUsageCounts{
		Hits: 1, PartialHits: 1, Misses: 1, Saves: 1, Egress: 2 * size, Ingress: size,
	}, f.usage(t, model.BuildCacheUsageFilter{RepoID: access.RepoID}))
}

func TestEgressLimit(t *testing.T) {
	f := newFixture(t, Config{MaxOrgEgress: 10})
	ctx := context.Background()
	org, repos := f.org(t, "acme", "app")
	access := repos[0]

	f.save(ctx, t, access, "k", "123456")
	_, err := f.service.Restore(ctx, access, "k", nil, false)
	require.NoError(t, err)
	_, err = f.service.Restore(ctx, access, "k", nil, false)
	assert.ErrorIs(t, err, ErrEgressExceeded, "12 bytes do not fit into 10")
	assert.EqualValues(t, 1, f.usage(t, model.BuildCacheUsageFilter{OrgID: org.ID}).Denied)

	// A save's existence check downloads nothing and is never refused.
	_, err = f.service.Restore(ctx, access, "k", nil, true)
	require.NoError(t, err)

	t.Run("lifted for the org", func(t *testing.T) {
		require.NoError(t, f.service.SetOrgSettings(ctx, &model.BuildCacheSettings{OrgID: org.ID, MaxEgress: model.BuildCacheLimitNone}))
		_, err := f.service.Restore(ctx, access, "k", nil, false)
		require.NoError(t, err)
	})

	t.Run("a new month starts over", func(t *testing.T) {
		require.NoError(t, f.service.SetOrgSettings(ctx, &model.BuildCacheSettings{OrgID: org.ID}))
		_, err := f.service.Restore(ctx, access, "k", nil, false)
		require.ErrorIs(t, err, ErrEgressExceeded)
		f.now = f.now.AddDate(0, 1, 0)
		_, err = f.service.Restore(ctx, access, "k", nil, false)
		require.NoError(t, err)
	})
}

func TestOrgSizeLimit(t *testing.T) {
	f := newFixture(t, Config{MaxOrgSize: 10})
	ctx := context.Background()
	org, repos := f.org(t, "acme", "one", "two")
	_, others := f.org(t, "other", "three")

	first := f.save(ctx, t, repos[0], "a", "aaaa")
	f.now = f.now.Add(time.Minute)
	foreign := f.save(ctx, t, others[0], "a", "zzzzzzzz")
	f.now = f.now.Add(time.Minute)
	second := f.save(ctx, t, repos[1], "b", "bbbb")
	f.now = f.now.Add(time.Minute)

	// The org holds 8 bytes; 4 more evict its least recently used entry, even
	// though it belongs to another repo, and leave the other org alone.
	f.save(ctx, t, repos[1], "c", "cccc")
	assert.False(t, f.exists(first.ID), "the org's oldest entry was evicted")
	assert.True(t, f.exists(second.ID))
	assert.True(t, f.exists(foreign.ID), "other orgs are not affected")

	_, err := f.service.Reserve(ctx, repos[0], "huge", 11)
	assert.ErrorIs(t, err, ErrQuotaExceeded)

	t.Run("raised for the org", func(t *testing.T) {
		require.NoError(t, f.service.SetOrgSettings(ctx, &model.BuildCacheSettings{OrgID: org.ID, MaxSize: 100}))
		_, err := f.service.Reserve(ctx, repos[0], "huge", 11)
		require.NoError(t, err)
	})

	t.Run("lowering evicts at once", func(t *testing.T) {
		require.NoError(t, f.service.SetOrgSettings(ctx, &model.BuildCacheSettings{OrgID: org.ID, MaxSize: 4}))
		size, err := f.store.BuildCacheSize(model.BuildCacheFilter{OrgID: org.ID})
		require.NoError(t, err)
		// The 11 byte reservation is pending and cannot be evicted.
		assert.EqualValues(t, 11, size)
	})

	t.Run("invalid", func(t *testing.T) {
		assert.ErrorIs(t, f.service.SetOrgSettings(ctx, &model.BuildCacheSettings{OrgID: org.ID, MaxSize: -2}), ErrInvalidSettings)
	})
}

func TestCleanupHonorsOrgTTL(t *testing.T) {
	f := newFixture(t, Config{TTL: 48 * time.Hour})
	ctx := context.Background()
	short, shortRepos := f.org(t, "short", "app")
	keep, keepRepos := f.org(t, "keep", "app")
	_, defaultRepos := f.org(t, "default", "app")

	require.NoError(t, f.service.SetOrgSettings(ctx, &model.BuildCacheSettings{OrgID: short.ID, TTL: 3600}))
	require.NoError(t, f.service.SetOrgSettings(ctx, &model.BuildCacheSettings{OrgID: keep.ID, TTL: model.BuildCacheLimitNone}))

	shortEntry := f.save(ctx, t, shortRepos[0], "k", "x")
	keepEntry := f.save(ctx, t, keepRepos[0], "k", "x")
	defaultEntry := f.save(ctx, t, defaultRepos[0], "k", "x")

	f.now = f.now.Add(2 * time.Hour)
	result, err := f.service.Cleanup(ctx)
	require.NoError(t, err)
	assert.Equal(t, CleanupResult{Removed: 1, Freed: 1}, result)
	assert.False(t, f.exists(shortEntry.ID))
	assert.True(t, f.exists(defaultEntry.ID))

	f.now = f.now.Add(72 * time.Hour)
	_, err = f.service.Cleanup(ctx)
	require.NoError(t, err)
	assert.False(t, f.exists(defaultEntry.ID), "expired by the server default")
	assert.True(t, f.exists(keepEntry.ID), "kept without a TTL")
}

func TestCleanupPrunesUsage(t *testing.T) {
	f := newFixture(t, Config{})
	ctx := context.Background()
	_, repos := f.org(t, "acme", "app")

	_, err := f.service.Restore(ctx, repos[0], "k", nil, false)
	require.ErrorIs(t, err, ErrNotFound)
	f.now = f.now.Add(usageRetention + day)
	_, err = f.service.Cleanup(ctx)
	require.NoError(t, err)
	assert.Zero(t, f.usage(t, model.BuildCacheUsageFilter{BucketID: model.BuildCacheAnyStorage}).Misses)
}

// bucketFixture lets orgs connect buckets that are directories.
func bucketFixture(t *testing.T, config Config) (*fixture, map[string]Storage) {
	t.Helper()
	config.AllowBuckets = true
	f := newFixture(t, config)
	opened := map[string]Storage{}
	f.service.openBucket = func(_ context.Context, bucket *model.BuildCacheBucket) (Storage, error) {
		if bucket.SecretKey != "secret" {
			return nil, errors.New("access denied")
		}
		name := bucket.Endpoint + "/" + bucket.Bucket + "/" + bucket.Prefix
		if storage, ok := opened[name]; ok {
			return storage, nil
		}
		storage, err := NewLocalStorage(t.TempDir())
		if err != nil {
			return nil, err
		}
		opened[name] = storage
		return storage, nil
	}
	return f, opened
}

func TestConnectBucket(t *testing.T) {
	f, opened := bucketFixture(t, Config{MaxOrgSize: 4, MaxOrgEgress: 1})
	ctx := context.Background()
	org, repos := f.org(t, "acme", "app")
	access := repos[0]

	global := f.save(ctx, t, access, "k", "x")

	_, err := f.service.ConnectBucket(ctx, org.ID, BucketInput{Endpoint: "s3.example.com", Bucket: "cache", SecretKey: "wrong"})
	require.ErrorIs(t, err, ErrInvalidBucket)
	assert.True(t, f.exists(global.ID), "a bucket that cannot be reached changes nothing")

	_, err = f.service.ConnectBucket(ctx, org.ID, BucketInput{Endpoint: "https://s3.example.com", Bucket: "cache", SecretKey: "secret"})
	require.ErrorIs(t, err, ErrInvalidBucket, "the endpoint has no scheme")

	bucket, err := f.service.ConnectBucket(ctx, org.ID, BucketInput{Endpoint: "s3.example.com", Bucket: "cache", Prefix: "/ci/", SecretKey: "secret"})
	require.NoError(t, err)
	assert.Equal(t, "ci", bucket.Prefix)
	assert.False(t, f.exists(global.ID), "entries in the server's storage are no longer found and removed")

	access.BucketID = bucket.ID
	t.Run("not limited by the server", func(t *testing.T) {
		entry := f.save(ctx, t, access, "big", "0123456789")
		assert.Equal(t, bucket.ID, entry.BucketID)
		size, err := opened["s3.example.com/cache/ci"].Size(ctx, objectName(entry))
		require.NoError(t, err)
		assert.EqualValues(t, 10, size)

		hit, err := f.service.Restore(ctx, access, "big", nil, false)
		require.NoError(t, err, "the egress limit applies to the server's storage only")
		assert.Equal(t, entry.ID, hit.Entry.ID)
		assert.Equal(t, "0123456789", f.read(ctx, t, access, entry.ID))
	})

	t.Run("changing credentials keeps the cache", func(t *testing.T) {
		updated, err := f.service.ConnectBucket(ctx, org.ID, BucketInput{Endpoint: "s3.example.com", Bucket: "cache", Prefix: "ci", AccessKey: "new", TTL: 60})
		require.NoError(t, err, "an empty secret keeps the current one")
		assert.Equal(t, bucket.ID, updated.ID)
		assert.Equal(t, "secret", updated.SecretKey)
		assert.Greater(t, updated.Updated, bucket.Updated)
		_, err = f.service.Restore(ctx, access, "big", nil, false)
		require.NoError(t, err)
	})

	t.Run("disconnect", func(t *testing.T) {
		entries, err := f.store.BuildCacheListFilter(model.BuildCacheFilter{OrgID: org.ID, BucketID: bucket.ID})
		require.NoError(t, err)
		require.NotEmpty(t, entries)

		require.NoError(t, f.service.DisconnectBucket(ctx, org.ID))
		assert.ErrorIs(t, f.service.DisconnectBucket(ctx, org.ID), ErrNotFound)
		entries, err = f.store.BuildCacheListFilter(model.BuildCacheFilter{OrgID: org.ID, BucketID: model.BuildCacheAnyStorage})
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("a row left in a disconnected bucket is dropped", func(t *testing.T) {
		left := &model.BuildCache{RepoID: access.RepoID, BucketID: bucket.ID, Scope: "refs/heads/main", Key: "left", Size: 1, Committed: true}
		require.NoError(t, f.store.BuildCacheCreate(left))
		access.BucketID = model.BuildCacheGlobalStorage
		_, err := f.service.Restore(ctx, access, "left", nil, false)
		require.ErrorIs(t, err, ErrNotFound)
		assert.False(t, f.exists(left.ID))
		f.save(ctx, t, access, "left", "x")
	})
}

func TestBucketsDisabled(t *testing.T) {
	f := newFixture(t, Config{})
	org, _ := f.org(t, "acme", "app")
	_, err := f.service.ConnectBucket(context.Background(), org.ID, BucketInput{Endpoint: "s3.example.com", Bucket: "cache"})
	assert.ErrorIs(t, err, ErrBucketsDisabled)
}

func TestAuthorizeUsesConnectedBucket(t *testing.T) {
	f, _ := bucketFixture(t, Config{})
	org, repos := f.org(t, "acme", "app")
	bucket, err := f.service.ConnectBucket(context.Background(), org.ID, BucketInput{Endpoint: "s3.example.com", Bucket: "cache", SecretKey: "secret"})
	require.NoError(t, err)

	repo, err := f.store.GetRepo(repos[0].RepoID)
	require.NoError(t, err)
	f.store.pipelines[10] = &model.Pipeline{ID: 10, RepoID: repo.ID, Event: model.EventPush, Branch: "main", Ref: "refs/heads/main"}
	f.store.workflows[100] = &model.Workflow{ID: 100, PipelineID: 10, State: model.StatusRunning}
	raw, err := f.service.MintToken(100)
	require.NoError(t, err)

	access, err := f.service.Authorize(raw)
	require.NoError(t, err)
	assert.Equal(t, org.ID, access.OrgID)
	assert.Equal(t, bucket.ID, access.BucketID)
}

func TestOverviewAndOrg(t *testing.T) {
	f, _ := bucketFixture(t, Config{MaxOrgSize: 1000, TTL: time.Hour})
	ctx := context.Background()
	acme, acmeRepos := f.org(t, "acme", "one", "two")
	other, otherRepos := f.org(t, "other", "app")

	f.save(ctx, t, acmeRepos[0], "a", "aaaa")
	f.save(ctx, t, acmeRepos[1], "b", "bb")
	_, err := f.service.Restore(ctx, acmeRepos[0], "a", nil, false)
	require.NoError(t, err)
	_, err = f.service.Restore(ctx, acmeRepos[1], "missing", nil, false)
	require.ErrorIs(t, err, ErrNotFound)

	bucket, err := f.service.ConnectBucket(ctx, other.ID, BucketInput{Endpoint: "s3.example.com", Bucket: "other", SecretKey: "secret"})
	require.NoError(t, err)
	own := otherRepos[0]
	own.BucketID = bucket.ID
	f.save(ctx, t, own, "c", "cccccccc")
	require.NoError(t, f.service.SetOrgSettings(ctx, &model.BuildCacheSettings{OrgID: acme.ID, MaxSize: 500}))

	overview, err := f.service.Overview()
	require.NoError(t, err)
	assert.EqualValues(t, 6, overview.Size)
	assert.EqualValues(t, 2, overview.Entries)
	assert.EqualValues(t, 8, overview.BucketSize)
	assert.EqualValues(t, 4, overview.EgressMonth)
	assert.Equal(t, Limits{MaxSize: 1000, TTL: 3600}, overview.Defaults)
	require.Len(t, overview.Orgs, 2)

	byName := map[string]*OrgSummary{}
	for _, org := range overview.Orgs {
		byName[org.Name] = org
	}
	assert.EqualValues(t, 6, byName["acme"].Size)
	assert.EqualValues(t, 2, byName["acme"].Repos)
	assert.EqualValues(t, 500, byName["acme"].Limits.MaxSize)
	assert.Equal(t, model.BuildCacheUsageCounts{Hits: 1, Misses: 1, Saves: 2, Egress: 4, Ingress: 6}, byName["acme"].Usage)
	assert.EqualValues(t, 8, byName["other"].BucketSize)
	require.NotNil(t, byName["other"].Bucket)
	assert.Equal(t, "other", byName["other"].Bucket.Bucket)

	detail, err := f.service.Org(acme.ID)
	require.NoError(t, err)
	require.Len(t, detail.RepoList, 2)
	assert.Equal(t, "acme/one", detail.RepoList[0].FullName, "largest first")
	assert.EqualValues(t, 1, detail.RepoList[0].Usage.Hits)
	require.Len(t, detail.Daily, 1)
	assert.EqualValues(t, 1, detail.Daily[0].Hits)

	usage, err := f.service.RepoUsage(acmeRepos[1].RepoID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, usage.Misses)

	t.Run("purge", func(t *testing.T) {
		result, err := f.service.PurgeOrg(ctx, acme.ID)
		require.NoError(t, err)
		assert.Equal(t, CleanupResult{Removed: 2, Freed: 6}, result)
	})

	t.Run("deleted org", func(t *testing.T) {
		require.NoError(t, f.service.OrgDeleted(ctx, other.ID))
		_, err := f.store.BuildCacheBucketFind(other.ID)
		assert.ErrorIs(t, err, types.RecordNotExist)
		_, err = f.store.BuildCacheSettingsGet(acme.ID)
		require.NoError(t, err, "other orgs keep their settings")
	})
}

func TestUsageKeyIsPerDay(t *testing.T) {
	f := newFixture(t, Config{})
	ctx := context.Background()
	org, repos := f.org(t, "acme", "app")
	for i := range 3 {
		_, err := f.service.Restore(ctx, repos[0], fmt.Sprintf("k-%d", i), nil, false)
		require.ErrorIs(t, err, ErrNotFound)
		f.now = f.now.Add(day)
	}
	detail, err := f.service.Org(org.ID)
	require.NoError(t, err)
	assert.Len(t, detail.Daily, 3)
	assert.True(t, strings.HasPrefix(detail.Name, "acme"))
}

func TestUnreachableBucketDoesNotBlockTheKey(t *testing.T) {
	f, _ := bucketFixture(t, Config{})
	ctx := context.Background()
	org, repos := f.org(t, "acme", "app")
	bucket, err := f.service.ConnectBucket(ctx, org.ID, BucketInput{Endpoint: "s3.example.com", Bucket: "cache", SecretKey: "secret"})
	require.NoError(t, err)
	access := repos[0]
	access.BucketID = bucket.ID

	reachable := f.service.openBucket
	f.service.openBucket = func(context.Context, *model.BuildCacheBucket) (Storage, error) {
		return nil, errors.New("connection refused")
	}
	f.service.closeBucket(bucket.ID)
	_, err = f.service.Reserve(ctx, access, "k", 1)
	require.Error(t, err)

	f.service.openBucket = reachable
	f.save(ctx, t, access, "k", "x")
}

func TestPurgeOrgRefusesOrgZero(t *testing.T) {
	f := newFixture(t, Config{})
	ctx := context.Background()
	_, repos := f.org(t, "acme", "app")
	entry := f.save(ctx, t, repos[0], "k", "x")

	_, err := f.service.PurgeOrg(ctx, 0)
	require.ErrorIs(t, err, ErrNotFound)
	assert.True(t, f.exists(entry.ID), "org 0 must not match every org")
	assert.ErrorIs(t, f.service.SetOrgSettings(ctx, &model.BuildCacheSettings{MaxSize: 1}), ErrNotFound)
}

func TestDisabledBucketsKeepTheirEntries(t *testing.T) {
	f, _ := bucketFixture(t, Config{TTL: time.Hour})
	ctx := context.Background()
	org, repos := f.org(t, "acme", "app")
	bucket, err := f.service.ConnectBucket(ctx, org.ID, BucketInput{Endpoint: "s3.example.com", Bucket: "cache", SecretKey: "secret"})
	require.NoError(t, err)
	own := repos[0]
	own.BucketID = bucket.ID
	entry := f.save(ctx, t, own, "k", "x")

	// An admin turns buckets off: workflows authorize to the server's storage.
	f.service.config.AllowBuckets = false
	f.service.openBucket = func(context.Context, *model.BuildCacheBucket) (Storage, error) {
		t.Fatal("the server must not contact a bucket while buckets are disabled")
		return nil, nil
	}
	_, err = f.service.Restore(ctx, repos[0], "k", []string{"k"}, false)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.service.Reserve(ctx, repos[0], "k", 1)
	require.ErrorIs(t, err, ErrExists)
	f.now = f.now.Add(2 * time.Hour)
	_, err = f.service.Cleanup(ctx)
	require.NoError(t, err)
	assert.True(t, f.exists(entry.ID), "the entry survives until buckets are enabled again")
}

func TestOwnersLimitTheirBucket(t *testing.T) {
	f, _ := bucketFixture(t, Config{MaxOrgSize: 1, MaxOrgEgress: 1})
	ctx := context.Background()
	org, repos := f.org(t, "acme", "one", "two")
	input := BucketInput{Endpoint: "s3.example.com", Bucket: "cache", SecretKey: "secret", MaxSize: 10, MaxEgress: 10}
	bucket, err := f.service.ConnectBucket(ctx, org.ID, input)
	require.NoError(t, err)
	one, two := repos[0], repos[1]
	one.BucketID, two.BucketID = bucket.ID, bucket.ID

	t.Run("size limit evicts across the org's repos, not the server's limit", func(t *testing.T) {
		first := f.save(ctx, t, one, "a", "aaaa")
		f.now = f.now.Add(time.Minute)
		second := f.save(ctx, t, two, "b", "bbbb")
		f.now = f.now.Add(time.Minute)
		f.save(ctx, t, one, "c", "cccc")
		assert.False(t, f.exists(first.ID), "the bucket's least recently used entry was evicted")
		assert.True(t, f.exists(second.ID))

		_, err := f.service.Reserve(ctx, one, "huge", 11)
		assert.ErrorIs(t, err, ErrQuotaExceeded)
	})

	t.Run("egress limit counts the bucket's restores", func(t *testing.T) {
		_, err := f.service.Restore(ctx, one, "c", nil, false)
		require.NoError(t, err)
		_, err = f.service.Restore(ctx, two, "b", nil, false)
		require.NoError(t, err)
		_, err = f.service.Restore(ctx, one, "c", nil, false)
		assert.ErrorIs(t, err, ErrEgressExceeded, "8 bytes restored, 4 more do not fit into 10")

		input.MaxEgress = model.BuildCacheLimitNone
		updated, err := f.service.ConnectBucket(ctx, org.ID, input)
		require.NoError(t, err)
		assert.Zero(t, updated.MaxEgress, "-1 is stored as unlimited")
		_, err = f.service.Restore(ctx, one, "c", nil, false)
		require.NoError(t, err)
	})

	t.Run("lowering the size limit evicts at once", func(t *testing.T) {
		input.MaxSize = 4
		_, err := f.service.ConnectBucket(ctx, org.ID, input)
		require.NoError(t, err)
		size, err := f.store.BuildCacheSize(model.BuildCacheFilter{BucketID: bucket.ID})
		require.NoError(t, err)
		assert.LessOrEqual(t, size, int64(4))
	})

	t.Run("the org page reports the bucket's egress", func(t *testing.T) {
		detail, err := f.service.Org(org.ID)
		require.NoError(t, err)
		assert.EqualValues(t, 12, detail.BucketEgressMonth)
		assert.Zero(t, detail.EgressMonth)
	})

	t.Run("invalid", func(t *testing.T) {
		input.MaxSize = -2
		_, err := f.service.ConnectBucket(ctx, org.ID, input)
		assert.ErrorIs(t, err, ErrInvalidSettings)
	})
}
