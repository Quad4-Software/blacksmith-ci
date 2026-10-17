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
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

// memStore is an in-memory Store for the service tests.
type memStore struct {
	mu        sync.Mutex
	nextID    int64
	entries   map[int64]*model.BuildCache
	workflows map[int64]*model.Workflow
	pipelines map[int64]*model.Pipeline
	repos     map[int64]*model.Repo
}

func newMemStore() *memStore {
	return &memStore{
		entries:   map[int64]*model.BuildCache{},
		workflows: map[int64]*model.Workflow{},
		pipelines: map[int64]*model.Pipeline{},
		repos:     map[int64]*model.Repo{},
	}
}

func (m *memStore) BuildCacheCreate(entry *model.BuildCache) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		if e.RepoID == entry.RepoID && e.Scope == entry.Scope && e.Key == entry.Key {
			return types.ErrConflict
		}
	}
	m.nextID++
	entry.ID = m.nextID
	copied := *entry
	m.entries[entry.ID] = &copied
	return nil
}

func (m *memStore) BuildCacheGet(id int64) (*model.BuildCache, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.entries[id]; ok {
		copied := *e
		return &copied, nil
	}
	return nil, types.RecordNotExist
}

func (m *memStore) filter(keep func(*model.BuildCache) bool) []*model.BuildCache {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*model.BuildCache
	for _, e := range m.entries {
		if keep(e) {
			copied := *e
			out = append(out, &copied)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (m *memStore) BuildCacheFind(repoID int64, scope, key string) (*model.BuildCache, error) {
	found := m.filter(func(e *model.BuildCache) bool { return e.RepoID == repoID && e.Scope == scope && e.Key == key })
	if len(found) == 0 {
		return nil, types.RecordNotExist
	}
	return found[0], nil
}

func (m *memStore) BuildCacheFindPrefix(repoID int64, scope, prefix string) (*model.BuildCache, error) {
	found := m.filter(func(e *model.BuildCache) bool {
		return e.RepoID == repoID && e.Scope == scope && e.Committed && strings.HasPrefix(e.Key, prefix)
	})
	if len(found) == 0 {
		return nil, types.RecordNotExist
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].Created > found[j].Created })
	return found[0], nil
}

func (m *memStore) BuildCacheList(repoID int64) ([]*model.BuildCache, error) {
	return m.filter(func(e *model.BuildCache) bool { return e.RepoID == repoID }), nil
}

func (m *memStore) BuildCacheListLRU(repoID int64, limit int) ([]*model.BuildCache, error) {
	found := m.filter(func(e *model.BuildCache) bool { return e.Committed && (repoID == 0 || e.RepoID == repoID) })
	sort.SliceStable(found, func(i, j int) bool { return found[i].LastUsed < found[j].LastUsed })
	return found[:min(limit, len(found))], nil
}

func (m *memStore) BuildCacheListStale(lastUsedBefore, pendingBefore int64) ([]*model.BuildCache, error) {
	return m.filter(func(e *model.BuildCache) bool {
		return (e.Committed && e.LastUsed < lastUsedBefore) || (!e.Committed && e.Created < pendingBefore)
	}), nil
}

func (m *memStore) BuildCacheSize(repoID int64) (int64, error) {
	var total int64
	for _, e := range m.filter(func(e *model.BuildCache) bool { return repoID == 0 || e.RepoID == repoID }) {
		total += e.Size
	}
	return total, nil
}

func (m *memStore) BuildCacheUpdate(entry *model.BuildCache) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := *entry
	m.entries[entry.ID] = &copied
	return nil
}

func (m *memStore) BuildCacheDelete(id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.entries[id]; !ok {
		return types.RecordNotExist
	}
	delete(m.entries, id)
	return nil
}

func (m *memStore) WorkflowLoad(id int64) (*model.Workflow, error) {
	if w, ok := m.workflows[id]; ok {
		return w, nil
	}
	return nil, types.RecordNotExist
}

func (m *memStore) GetPipeline(id int64) (*model.Pipeline, error) {
	if p, ok := m.pipelines[id]; ok {
		return p, nil
	}
	return nil, types.RecordNotExist
}

func (m *memStore) GetRepo(id int64) (*model.Repo, error) {
	if r, ok := m.repos[id]; ok {
		return r, nil
	}
	return nil, types.RecordNotExist
}

type fixture struct {
	store   *memStore
	service *Service
	now     time.Time
}

func newFixture(t *testing.T, config Config) *fixture {
	t.Helper()
	storage, err := NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	config.BlobURL = "https://ci.example.com/api/v1/cache/blobs"
	config.Secret = "secret"
	if config.PendingTTL == 0 {
		config.PendingTTL = time.Hour
	}
	f := &fixture{store: newMemStore(), now: time.Now().Truncate(time.Second)}
	f.service = New(f.store, storage, config)
	f.service.now = func() time.Time { return f.now }
	return f
}

var (
	mainAccess    = Access{RepoID: 1, ReadScopes: []string{"refs/heads/main"}, WriteScope: "refs/heads/main"}
	featureAccess = Access{RepoID: 1, ReadScopes: []string{"refs/heads/feature", "refs/heads/main"}, WriteScope: "refs/heads/feature"}
)

// save stores data under key the way the cache step does.
func (f *fixture) save(ctx context.Context, t *testing.T, access Access, key, data string) *model.BuildCache {
	t.Helper()
	reservation, err := f.service.Reserve(ctx, access, key, int64(len(data)))
	require.NoError(t, err)
	assert.Equal(t, "https://ci.example.com/api/v1/cache/blobs/"+strconv.FormatInt(reservation.Entry.ID, 10), reservation.URL)
	require.NoError(t, f.service.WriteBlob(ctx, access, reservation.Entry.ID, strings.NewReader(data)))
	entry, err := f.service.Commit(ctx, access, reservation.Entry.ID)
	require.NoError(t, err)
	return entry
}

func (f *fixture) read(t *testing.T, access Access, id int64) string {
	t.Helper()
	r, _, err := f.service.OpenBlob(context.Background(), access, id)
	require.NoError(t, err)
	defer r.Close()
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(data)
}

func TestRestoreRoundTrip(t *testing.T) {
	f := newFixture(t, Config{})
	ctx := context.Background()

	_, err := f.service.Restore(ctx, mainAccess, "go-1", []string{"go-"})
	assert.ErrorIs(t, err, ErrNotFound)

	saved := f.save(ctx, t, mainAccess, "go-1", "archive")
	assert.True(t, saved.Committed)
	assert.EqualValues(t, len("archive"), saved.Size)

	f.now = f.now.Add(time.Minute)
	hit, err := f.service.Restore(ctx, mainAccess, "go-1", nil)
	require.NoError(t, err)
	assert.True(t, hit.Exact)
	assert.Equal(t, f.now.Unix(), hit.Entry.LastUsed)
	assert.Equal(t, "archive", f.read(t, mainAccess, hit.Entry.ID))
}

func TestRestorePrefersExactThenOwnScope(t *testing.T) {
	f := newFixture(t, Config{})
	ctx := context.Background()

	f.save(ctx, t, mainAccess, "go-1", "main-1")
	f.now = f.now.Add(time.Minute)
	f.save(ctx, t, mainAccess, "go-2", "main-2")
	f.now = f.now.Add(time.Minute)
	f.save(ctx, t, featureAccess, "go-3", "feature-3")

	// An exact match in the default branch wins over a prefix match in the own scope.
	hit, err := f.service.Restore(ctx, featureAccess, "go-1", []string{"go-"})
	require.NoError(t, err)
	assert.True(t, hit.Exact)
	assert.Equal(t, "refs/heads/main", hit.Entry.Scope)

	// Without an exact match the own scope is searched first.
	hit, err = f.service.Restore(ctx, featureAccess, "go-9", []string{"go-"})
	require.NoError(t, err)
	assert.False(t, hit.Exact)
	assert.Equal(t, "go-3", hit.Entry.Key)

	// The default branch never sees feature branch caches.
	hit, err = f.service.Restore(ctx, mainAccess, "go-9", []string{"go-"})
	require.NoError(t, err)
	assert.Equal(t, "go-2", hit.Entry.Key, "newest main entry")
	_, _, err = f.service.OpenBlob(ctx, mainAccess, f.mustFind(t, "refs/heads/feature", "go-3").ID)
	assert.ErrorIs(t, err, ErrNotFound)
}

func (f *fixture) mustFind(t *testing.T, scope, key string) *model.BuildCache {
	t.Helper()
	entry, err := f.store.BuildCacheFind(1, scope, key)
	require.NoError(t, err)
	return entry
}

func TestReserveRules(t *testing.T) {
	f := newFixture(t, Config{PendingTTL: time.Hour})
	ctx := context.Background()

	t.Run("read-only", func(t *testing.T) {
		_, err := f.service.Reserve(ctx, Access{RepoID: 1, ReadScopes: []string{"refs/heads/main"}}, "k", 1)
		assert.ErrorIs(t, err, ErrReadOnly)
	})

	t.Run("invalid key", func(t *testing.T) {
		_, err := f.service.Reserve(ctx, mainAccess, "", 1)
		assert.ErrorIs(t, err, ErrInvalidKey)
		_, err = f.service.Reserve(ctx, mainAccess, "a\nb", 1)
		assert.ErrorIs(t, err, ErrInvalidKey)
		_, err = f.service.Reserve(ctx, mainAccess, strings.Repeat("k", MaxKeyLength+1), 1)
		assert.ErrorIs(t, err, ErrInvalidKey)
	})

	t.Run("key cached in a readable scope", func(t *testing.T) {
		f.save(ctx, t, mainAccess, "shared", "x")
		_, err := f.service.Reserve(ctx, featureAccess, "shared", 1)
		assert.ErrorIs(t, err, ErrExists)
	})

	t.Run("concurrent save of the same key", func(t *testing.T) {
		_, err := f.service.Reserve(ctx, mainAccess, "busy", 1)
		require.NoError(t, err)
		_, err = f.service.Reserve(ctx, mainAccess, "busy", 1)
		assert.ErrorIs(t, err, ErrExists)

		// Once the first upload is abandoned the key is free again.
		f.now = f.now.Add(2 * time.Hour)
		_, err = f.service.Reserve(ctx, mainAccess, "busy", 1)
		assert.NoError(t, err)
	})

	t.Run("upload beyond reservation", func(t *testing.T) {
		reservation, err := f.service.Reserve(ctx, mainAccess, "small", 2)
		require.NoError(t, err)
		err = f.service.WriteBlob(ctx, mainAccess, reservation.Entry.ID, strings.NewReader("too long"))
		assert.ErrorIs(t, err, ErrObjectTooLarge)
		_, err = f.service.Commit(ctx, mainAccess, reservation.Entry.ID)
		assert.Error(t, err, "nothing was stored")
	})

	t.Run("other workflows cannot upload or commit", func(t *testing.T) {
		reservation, err := f.service.Reserve(ctx, mainAccess, "mine", 1)
		require.NoError(t, err)
		assert.ErrorIs(t, f.service.WriteBlob(ctx, featureAccess, reservation.Entry.ID, strings.NewReader("x")), ErrNotFound)
		_, err = f.service.Commit(ctx, featureAccess, reservation.Entry.ID)
		assert.ErrorIs(t, err, ErrNotFound)
		otherRepo := Access{RepoID: 2, ReadScopes: mainAccess.ReadScopes, WriteScope: mainAccess.WriteScope}
		assert.ErrorIs(t, f.service.WriteBlob(ctx, otherRepo, reservation.Entry.ID, strings.NewReader("x")), ErrNotFound)
	})
}

func TestQuotaEvictsLeastRecentlyUsed(t *testing.T) {
	f := newFixture(t, Config{MaxRepoSize: 10})
	ctx := context.Background()

	first := f.save(ctx, t, mainAccess, "a", "aaaa")
	f.now = f.now.Add(time.Minute)
	second := f.save(ctx, t, mainAccess, "b", "bbbb")
	f.now = f.now.Add(time.Minute)

	// Restoring a makes b the least recently used entry.
	_, err := f.service.Restore(ctx, mainAccess, "a", nil)
	require.NoError(t, err)
	f.now = f.now.Add(time.Minute)

	f.save(ctx, t, mainAccess, "c", "cccc")
	_, err = f.store.BuildCacheGet(second.ID)
	assert.ErrorIs(t, err, types.RecordNotExist, "b was evicted")
	_, err = f.store.BuildCacheGet(first.ID)
	assert.NoError(t, err, "a was kept")

	_, err = f.service.Reserve(ctx, mainAccess, "huge", 11)
	assert.ErrorIs(t, err, ErrQuotaExceeded)
}

func TestQuotaFullOfPendingUploads(t *testing.T) {
	f := newFixture(t, Config{MaxTotalSize: 5})
	ctx := context.Background()

	_, err := f.service.Reserve(ctx, mainAccess, "a", 5)
	require.NoError(t, err)
	_, err = f.service.Reserve(ctx, mainAccess, "b", 1)
	assert.ErrorIs(t, err, ErrQuotaExceeded)
}

func TestCleanup(t *testing.T) {
	f := newFixture(t, Config{TTL: 24 * time.Hour, PendingTTL: time.Hour, MaxTotalSize: 100})
	ctx := context.Background()

	old := f.save(ctx, t, mainAccess, "old", "x")
	f.now = f.now.Add(23 * time.Hour)
	fresh := f.save(ctx, t, mainAccess, "fresh", "y")
	abandoned, err := f.service.Reserve(ctx, mainAccess, "abandoned", 1)
	require.NoError(t, err)

	f.now = f.now.Add(2 * time.Hour)
	require.NoError(t, f.service.Cleanup(ctx))

	remaining, err := f.service.List(1)
	require.NoError(t, err)
	ids := make([]int64, 0, len(remaining))
	for _, e := range remaining {
		ids = append(ids, e.ID)
	}
	assert.Equal(t, []int64{fresh.ID}, ids)
	assert.False(t, slices.Contains(ids, old.ID))
	assert.False(t, slices.Contains(ids, abandoned.Entry.ID))
}

func TestDeleteAndPurge(t *testing.T) {
	f := newFixture(t, Config{})
	ctx := context.Background()

	a := f.save(ctx, t, mainAccess, "a", "x")
	f.save(ctx, t, mainAccess, "b", "y")

	assert.ErrorIs(t, f.service.Delete(ctx, 2, a.ID), ErrNotFound, "entry of another repo")
	require.NoError(t, f.service.Delete(ctx, 1, a.ID))
	assert.ErrorIs(t, f.service.Delete(ctx, 1, a.ID), ErrNotFound)

	require.NoError(t, f.service.Purge(ctx, 1))
	remaining, err := f.service.List(1)
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

func TestAuthorize(t *testing.T) {
	f := newFixture(t, Config{})
	f.store.repos[1] = &model.Repo{ID: 1, Branch: "main"}
	f.store.pipelines[10] = &model.Pipeline{ID: 10, RepoID: 1, Event: model.EventPush, Branch: "feature", Ref: "refs/heads/feature"}
	f.store.workflows[100] = &model.Workflow{ID: 100, PipelineID: 10, State: model.StatusRunning}

	raw, err := f.service.MintToken(100)
	require.NoError(t, err)

	access, err := f.service.Authorize(raw)
	require.NoError(t, err)
	assert.Equal(t, featureAccess, access)

	f.store.workflows[100].State = model.StatusSuccess
	_, err = f.service.Authorize(raw)
	assert.ErrorIs(t, err, ErrUnauthorized, "token dies with its workflow")

	_, err = f.service.Authorize(raw + "x")
	assert.ErrorIs(t, err, ErrUnauthorized)

	other := New(f.store, nil, Config{Secret: "other"})
	f.store.workflows[100].State = model.StatusRunning
	_, err = other.Authorize(raw)
	assert.ErrorIs(t, err, ErrUnauthorized, "signed with another secret")
}
