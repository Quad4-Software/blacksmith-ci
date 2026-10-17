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

// Package buildcache stores the archives that workflow cache steps restore and
// save. The server owns storage, scoping, quotas and cleanup; cache steps only
// get a token that is valid while their workflow runs.
package buildcache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store/types"
	"codefloe.com/crowci/crow/v6/shared/token"
)

var (
	// ErrNotFound means no entry matched.
	ErrNotFound = errors.New("cache entry not found")
	// ErrExists means the key is already cached, or being saved, in a scope the
	// workflow reads; keys are immutable.
	ErrExists = errors.New("cache key already exists")
	// ErrReadOnly means the workflow may restore but not save.
	ErrReadOnly = errors.New("this workflow may only restore caches")
	// ErrQuotaExceeded means the archive does not fit even after evicting the
	// least recently used entries.
	ErrQuotaExceeded = errors.New("cache quota exceeded")
	// ErrInvalidKey means the key is empty, too long or contains control characters.
	ErrInvalidKey = errors.New("invalid cache key")
	// ErrUnauthorized means the token is invalid or its workflow is not running.
	ErrUnauthorized = errors.New("cache token is not valid")
)

const (
	// MaxKeyLength matches the width of the key column.
	MaxKeyLength = 255
	// A token expires after this even if its workflow never leaves the running state.
	tokenLifetime = 7 * 24 * time.Hour
	// How many entries are loaded per eviction round.
	evictBatch = 20
)

// Store is the subset of the datastore the service needs.
type Store interface {
	BuildCacheCreate(*model.BuildCache) error
	BuildCacheGet(id int64) (*model.BuildCache, error)
	BuildCacheFind(repoID int64, scope, key string) (*model.BuildCache, error)
	BuildCacheFindPrefix(repoID int64, scope, prefix string) (*model.BuildCache, error)
	BuildCacheList(repoID int64) ([]*model.BuildCache, error)
	BuildCacheListLRU(repoID int64, limit int) ([]*model.BuildCache, error)
	BuildCacheListStale(lastUsedBefore, pendingBefore int64) ([]*model.BuildCache, error)
	BuildCacheSize(repoID int64) (int64, error)
	BuildCacheUpdate(*model.BuildCache) error
	BuildCacheDelete(id int64) error
	WorkflowLoad(int64) (*model.Workflow, error)
	GetPipeline(int64) (*model.Pipeline, error)
	GetRepo(int64) (*model.Repo, error)
}

// Config tunes the service.
type Config struct {
	// BlobURL is the base URL of the API's blob endpoint, used when the storage
	// streams archives through the server, e.g. https://ci.example.com/api/v1/cache/blobs.
	BlobURL string
	// Secret signs cache tokens.
	Secret string
	// MaxRepoSize caps the cache of one repo in bytes; 0 means unlimited.
	MaxRepoSize int64
	// MaxTotalSize caps the cache of all repos in bytes; 0 means unlimited.
	MaxTotalSize int64
	// TTL removes entries not restored for this long.
	TTL time.Duration
	// PendingTTL removes uploads that were reserved but never committed.
	PendingTTL time.Duration
	// URLExpiry is the lifetime of presigned storage URLs.
	URLExpiry time.Duration
}

// Service manages build cache entries.
type Service struct {
	store   Store
	storage Storage
	config  Config
	now     func() time.Time

	// quotaMu serializes the quota check and the reservation, so concurrent
	// saves on one server cannot both claim the last free space.
	quotaMu sync.Mutex
}

// New creates the service.
func New(store Store, storage Storage, config Config) *Service {
	return &Service{store: store, storage: storage, config: config, now: time.Now}
}

// Hit is a restored entry and where to download it.
type Hit struct {
	Entry *model.BuildCache
	// Exact is true when the requested key matched, false for a restore key.
	Exact bool
	URL   string
}

// Reservation is a pending entry and where to upload it.
type Reservation struct {
	Entry *model.BuildCache
	URL   string
}

// MintToken creates the cache token of a workflow. It is valid only while the
// workflow runs.
func (s *Service) MintToken(workflowID int64) (string, error) {
	t := token.New(token.CacheToken)
	t.Set("workflow-id", strconv.FormatInt(workflowID, 10))
	return t.SignExpires(s.config.Secret, s.now().Add(tokenLifetime).Unix())
}

// Authorize resolves a cache token to the access of its workflow.
func (s *Service) Authorize(raw string) (Access, error) {
	parsed, err := token.Parse([]token.Type{token.CacheToken}, raw, func(*token.Token) (string, error) {
		return s.config.Secret, nil
	})
	if err != nil {
		return Access{}, ErrUnauthorized
	}
	workflowID, err := strconv.ParseInt(parsed.Get("workflow-id"), 10, 64)
	if err != nil {
		return Access{}, ErrUnauthorized
	}
	workflow, err := s.store.WorkflowLoad(workflowID)
	if err != nil {
		return Access{}, ErrUnauthorized
	}
	if workflow.State != model.StatusRunning {
		return Access{}, ErrUnauthorized
	}
	pipeline, err := s.store.GetPipeline(workflow.PipelineID)
	if err != nil {
		return Access{}, ErrUnauthorized
	}
	repo, err := s.store.GetRepo(pipeline.RepoID)
	if err != nil {
		return Access{}, ErrUnauthorized
	}
	return AccessFor(repo, pipeline), nil
}

// ValidateKey checks a cache key or restore key prefix.
func ValidateKey(key string) error {
	if key == "" || len(key) > MaxKeyLength {
		return ErrInvalidKey
	}
	if strings.ContainsFunc(key, unicode.IsControl) {
		return ErrInvalidKey
	}
	return nil
}

func objectName(entry *model.BuildCache) string {
	return fmt.Sprintf("%d/%d.tar.zst", entry.RepoID, entry.ID)
}

// Restore finds the entry for key, falling back to the newest entry whose key
// starts with one of restoreKeys. An exact match in any readable scope wins
// over a prefix match; otherwise scopes are searched in order of preference.
func (s *Service) Restore(ctx context.Context, access Access, key string, restoreKeys []string) (*Hit, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	for _, prefix := range restoreKeys {
		if err := ValidateKey(prefix); err != nil {
			return nil, err
		}
	}

	hit, err := s.find(access, key, restoreKeys)
	if err != nil {
		return nil, err
	}

	hit.Entry.LastUsed = s.now().Unix()
	if err := s.store.BuildCacheUpdate(hit.Entry); err != nil {
		return nil, err
	}
	hit.URL, err = s.downloadURL(ctx, hit.Entry)
	if err != nil {
		return nil, err
	}
	return hit, nil
}

func (s *Service) find(access Access, key string, restoreKeys []string) (*Hit, error) {
	for _, scope := range access.ReadScopes {
		entry, err := s.store.BuildCacheFind(access.RepoID, scope, key)
		if err == nil && entry.Committed {
			return &Hit{Entry: entry, Exact: true}, nil
		}
		if err != nil && !errors.Is(err, types.RecordNotExist) {
			return nil, err
		}
	}
	for _, scope := range access.ReadScopes {
		for _, prefix := range restoreKeys {
			entry, err := s.store.BuildCacheFindPrefix(access.RepoID, scope, prefix)
			if err == nil {
				return &Hit{Entry: entry}, nil
			}
			if !errors.Is(err, types.RecordNotExist) {
				return nil, err
			}
		}
	}
	return nil, ErrNotFound
}

// Reserve creates a pending entry for an archive of size bytes in the
// workflow's own scope, evicting the least recently used entries to make room.
func (s *Service) Reserve(ctx context.Context, access Access, key string, size int64) (*Reservation, error) {
	if access.WriteScope == "" {
		return nil, ErrReadOnly
	}
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	if size < 0 {
		return nil, ErrQuotaExceeded
	}

	// A key restored exactly from any readable scope is not saved again.
	for _, scope := range access.ReadScopes {
		entry, err := s.store.BuildCacheFind(access.RepoID, scope, key)
		switch {
		case errors.Is(err, types.RecordNotExist):
			continue
		case err != nil:
			return nil, err
		case entry.Committed:
			return nil, ErrExists
		case scope == access.WriteScope && s.abandoned(entry):
			// An upload that never finished must not block the key forever.
			if err := s.delete(ctx, entry); err != nil {
				return nil, err
			}
		case scope == access.WriteScope:
			// Another workflow is saving the same key right now.
			return nil, ErrExists
		}
	}

	s.quotaMu.Lock()
	defer s.quotaMu.Unlock()

	if err := s.makeRoom(ctx, access.RepoID, size); err != nil {
		return nil, err
	}

	now := s.now().Unix()
	entry := &model.BuildCache{
		RepoID:   access.RepoID,
		Scope:    access.WriteScope,
		Key:      key,
		Size:     size,
		Created:  now,
		LastUsed: now,
	}
	if err := s.store.BuildCacheCreate(entry); err != nil {
		if errors.Is(err, types.ErrConflict) {
			return nil, ErrExists
		}
		return nil, err
	}

	url, err := s.uploadURL(ctx, entry)
	if err != nil {
		return nil, err
	}
	return &Reservation{Entry: entry, URL: url}, nil
}

func (s *Service) abandoned(entry *model.BuildCache) bool {
	return !entry.Committed && s.now().Sub(time.Unix(entry.Created, 0)) > s.config.PendingTTL
}

// makeRoom evicts least recently used committed entries until size more bytes
// fit into both the repo and the total quota.
func (s *Service) makeRoom(ctx context.Context, repoID, size int64) error {
	if s.config.MaxRepoSize > 0 && size > s.config.MaxRepoSize {
		return ErrQuotaExceeded
	}
	if s.config.MaxTotalSize > 0 && size > s.config.MaxTotalSize {
		return ErrQuotaExceeded
	}
	if err := s.evictUntil(ctx, repoID, s.config.MaxRepoSize, size); err != nil {
		return err
	}
	return s.evictUntil(ctx, 0, s.config.MaxTotalSize, size)
}

// evictUntil evicts entries of repoID (0 for all repos) until used+extra fits
// into limit. A limit of 0 means unlimited.
func (s *Service) evictUntil(ctx context.Context, repoID, limit, extra int64) error {
	if limit <= 0 {
		return nil
	}
	for {
		used, err := s.store.BuildCacheSize(repoID)
		if err != nil {
			return err
		}
		if used+extra <= limit {
			return nil
		}
		victims, err := s.store.BuildCacheListLRU(repoID, evictBatch)
		if err != nil {
			return err
		}
		if len(victims) == 0 {
			// Only pending uploads are left, nothing can be evicted.
			return ErrQuotaExceeded
		}
		for _, victim := range victims {
			if err := s.delete(ctx, victim); err != nil {
				return err
			}
			used -= victim.Size
			log.Debug().Int64("repo_id", victim.RepoID).Str("key", victim.Key).Msg("evicted build cache entry to make room")
			if used+extra <= limit {
				return nil
			}
		}
	}
}

// Commit marks an uploaded entry as restorable, recording its stored size.
func (s *Service) Commit(ctx context.Context, access Access, id int64) (*model.BuildCache, error) {
	entry, err := s.writable(access, id)
	if err != nil {
		return nil, err
	}

	size, err := s.storage.Size(ctx, objectName(entry))
	if err != nil {
		return nil, fmt.Errorf("cache archive was not uploaded: %w", err)
	}

	if size > entry.Size {
		// The upload is larger than reserved, so claim the difference too.
		s.quotaMu.Lock()
		err := s.makeRoom(ctx, entry.RepoID, size-entry.Size)
		s.quotaMu.Unlock()
		if err != nil {
			if delErr := s.delete(ctx, entry); delErr != nil {
				log.Error().Err(delErr).Int64("id", entry.ID).Msg("could not delete oversized build cache entry")
			}
			return nil, err
		}
	}

	entry.Size = size
	entry.Committed = true
	entry.LastUsed = s.now().Unix()
	return entry, s.store.BuildCacheUpdate(entry)
}

// writable loads a pending entry the workflow may upload to.
func (s *Service) writable(access Access, id int64) (*model.BuildCache, error) {
	entry, err := s.store.BuildCacheGet(id)
	if errors.Is(err, types.RecordNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if entry.RepoID != access.RepoID || access.WriteScope == "" || entry.Scope != access.WriteScope || entry.Committed {
		return nil, ErrNotFound
	}
	return entry, nil
}

// OpenBlob streams a committed entry the workflow may restore.
func (s *Service) OpenBlob(ctx context.Context, access Access, id int64) (io.ReadCloser, int64, error) {
	entry, err := s.store.BuildCacheGet(id)
	if errors.Is(err, types.RecordNotExist) {
		return nil, 0, ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	if entry.RepoID != access.RepoID || !access.CanRead(entry.Scope) || !entry.Committed {
		return nil, 0, ErrNotFound
	}
	r, err := s.storage.Open(ctx, objectName(entry))
	return r, entry.Size, err
}

// WriteBlob stores the upload of a pending entry, accepting at most the
// reserved size.
func (s *Service) WriteBlob(ctx context.Context, access Access, id int64, r io.Reader) error {
	entry, err := s.writable(access, id)
	if err != nil {
		return err
	}
	_, err = s.storage.Write(ctx, objectName(entry), r, entry.Size)
	return err
}

func (s *Service) downloadURL(ctx context.Context, entry *model.BuildCache) (string, error) {
	url, err := s.storage.DownloadURL(ctx, objectName(entry), s.config.URLExpiry)
	if err != nil || url != "" {
		return url, err
	}
	return fmt.Sprintf("%s/%d", s.config.BlobURL, entry.ID), nil
}

func (s *Service) uploadURL(ctx context.Context, entry *model.BuildCache) (string, error) {
	url, err := s.storage.UploadURL(ctx, objectName(entry), s.config.URLExpiry)
	if err != nil || url != "" {
		return url, err
	}
	return fmt.Sprintf("%s/%d", s.config.BlobURL, entry.ID), nil
}

// List lists the cache entries of a repo.
func (s *Service) List(repoID int64) ([]*model.BuildCache, error) {
	return s.store.BuildCacheList(repoID)
}

// Delete removes one entry of a repo.
func (s *Service) Delete(ctx context.Context, repoID, id int64) error {
	entry, err := s.store.BuildCacheGet(id)
	if errors.Is(err, types.RecordNotExist) || (err == nil && entry.RepoID != repoID) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return s.delete(ctx, entry)
}

// Purge removes every entry of a repo.
func (s *Service) Purge(ctx context.Context, repoID int64) error {
	entries, err := s.store.BuildCacheList(repoID)
	if err != nil {
		return err
	}
	var errs []error
	for _, entry := range entries {
		errs = append(errs, s.delete(ctx, entry))
	}
	return errors.Join(errs...)
}

// delete removes the archive before the row, so a failure leaves a row the
// next cleanup retries rather than an archive nothing refers to.
func (s *Service) delete(ctx context.Context, entry *model.BuildCache) error {
	if err := s.storage.Delete(ctx, objectName(entry)); err != nil {
		return fmt.Errorf("delete cache archive %d: %w", entry.ID, err)
	}
	if err := s.store.BuildCacheDelete(entry.ID); err != nil && !errors.Is(err, types.RecordNotExist) {
		return err
	}
	return nil
}

// Cleanup removes entries unused for longer than the TTL and abandoned
// uploads, then evicts entries until the total quota holds again.
func (s *Service) Cleanup(ctx context.Context) error {
	now := s.now()
	var lastUsedBefore int64
	if s.config.TTL > 0 {
		lastUsedBefore = now.Add(-s.config.TTL).Unix()
	}
	stale, err := s.store.BuildCacheListStale(lastUsedBefore, now.Add(-s.config.PendingTTL).Unix())
	if err != nil {
		return err
	}
	var errs []error
	for _, entry := range stale {
		errs = append(errs, s.delete(ctx, entry))
	}
	if len(stale) > 0 {
		log.Info().Int("count", len(stale)).Msg("removed expired build cache entries")
	}

	s.quotaMu.Lock()
	defer s.quotaMu.Unlock()
	if err := s.evictUntil(ctx, 0, s.config.MaxTotalSize, 0); err != nil && !errors.Is(err, ErrQuotaExceeded) {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// RunCleanup runs Cleanup every interval until ctx is done.
func (s *Service) RunCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Cleanup(ctx); err != nil {
				log.Error().Err(err).Msg("build cache cleanup failed")
			}
		}
	}
}
