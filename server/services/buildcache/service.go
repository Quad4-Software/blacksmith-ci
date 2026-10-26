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
	"sync/atomic"
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
	// ErrEgressExceeded means restoring the entry would exceed the org's
	// monthly egress limit.
	ErrEgressExceeded = errors.New("cache egress limit of this month reached")
	// ErrBucketGone means an entry's bucket was disconnected.
	ErrBucketGone = errors.New("cache bucket is not connected")
	// ErrStorageUnavailable means the server's storage failed its last check,
	// so cache steps skip caching rather than wait for it.
	ErrStorageUnavailable = errors.New("the build cache storage is unavailable")
)

const (
	// MaxKeyLength matches the width of the key column.
	MaxKeyLength = 255
	// A token expires after this even if its workflow never leaves the running state.
	tokenLifetime = 7 * 24 * time.Hour
	// How many entries are loaded per eviction round.
	evictBatch = 20
	// Archives larger than this are uploaded in parts of this size, if the
	// storage supports it.
	partSize = 16 << 20
	// S3 takes at most this many parts per upload.
	maxParts = 10000
)

// Store is the subset of the datastore the service needs.
type Store interface {
	BuildCacheCreate(*model.BuildCache) error
	BuildCacheGet(id int64) (*model.BuildCache, error)
	BuildCacheFind(repoID int64, scope, key string) (*model.BuildCache, error)
	BuildCacheFindPrefix(repoID int64, scope, prefix string) (*model.BuildCache, error)
	BuildCacheList(repoID int64) ([]*model.BuildCache, error)
	BuildCacheListFilter(filter model.BuildCacheFilter) ([]*model.BuildCache, error)
	BuildCacheListLRU(filter model.BuildCacheFilter, limit int) ([]*model.BuildCache, error)
	BuildCacheListStale(filter model.BuildCacheFilter, lastUsedBefore, pendingBefore int64) ([]*model.BuildCache, error)
	BuildCacheSize(filter model.BuildCacheFilter) (int64, error)
	BuildCacheSizesByOrg() ([]*model.BuildCacheSize, error)
	BuildCacheSizesByRepo(orgID int64) ([]*model.BuildCacheSize, error)
	BuildCacheUpdate(*model.BuildCache) error
	BuildCacheDelete(id int64) error
	BuildCacheSettingsGet(orgID int64) (*model.BuildCacheSettings, error)
	BuildCacheSettingsList() ([]*model.BuildCacheSettings, error)
	BuildCacheSettingsSave(*model.BuildCacheSettings) error
	BuildCacheSettingsDelete(orgID int64) error
	BuildCacheBucketGet(id int64) (*model.BuildCacheBucket, error)
	BuildCacheBucketFind(orgID int64) (*model.BuildCacheBucket, error)
	BuildCacheBucketList() ([]*model.BuildCacheBucket, error)
	BuildCacheBucketCreate(*model.BuildCacheBucket) error
	BuildCacheBucketUpdate(*model.BuildCacheBucket) error
	BuildCacheBucketDelete(id int64) error
	BuildCacheUsageAdd(*model.BuildCacheUsage) error
	BuildCacheUsageSum(filter model.BuildCacheUsageFilter) (*model.BuildCacheUsageCounts, error)
	BuildCacheUsageByOrg(filter model.BuildCacheUsageFilter) ([]*model.BuildCacheUsageGroup, error)
	BuildCacheUsageByRepo(filter model.BuildCacheUsageFilter) ([]*model.BuildCacheUsageGroup, error)
	BuildCacheUsageByDay(filter model.BuildCacheUsageFilter) ([]*model.BuildCacheUsageGroup, error)
	BuildCacheUsagePrune(before int64) error
	WorkflowLoad(int64) (*model.Workflow, error)
	GetPipeline(int64) (*model.Pipeline, error)
	GetRepo(int64) (*model.Repo, error)
	OrgGet(int64) (*model.Org, error)
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
	// MaxOrgSize caps the cache of one org in bytes unless an admin set
	// another limit for it; 0 means unlimited.
	MaxOrgSize int64
	// MaxTotalSize caps the cache of all repos in bytes; 0 means unlimited.
	MaxTotalSize int64
	// MaxOrgEgress caps the bytes one org restores per calendar month unless
	// an admin set another limit for it; 0 means unlimited.
	MaxOrgEgress int64
	// TTL removes entries not restored for this long.
	TTL time.Duration
	// AllowBuckets lets orgs keep their caches in a bucket of their own.
	AllowBuckets bool
	// AllowPrivateBuckets lets those buckets be on loopback, private and
	// link-local addresses, which the server refuses otherwise.
	AllowPrivateBuckets bool
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

	// storageDown is set while the server's storage fails its checks.
	storageDown atomic.Bool

	// quotaMu serializes the quota check and the reservation, so concurrent
	// saves on one server cannot both claim the last free space.
	quotaMu sync.Mutex

	// openBucket connects to a bucket an org connected.
	openBucket func(context.Context, *model.BuildCacheBucket) (Storage, error)
	bucketsMu  sync.Mutex
	// buckets are the open buckets by ID, reopened when the bucket changes.
	buckets map[int64]openedBucket
}

type openedBucket struct {
	storage Storage
	updated int64
}

// New creates the service.
func New(store Store, storage Storage, config Config) *Service {
	return &Service{
		store:      store,
		storage:    storage,
		config:     config,
		now:        time.Now,
		openBucket: openS3Bucket(!config.AllowPrivateBuckets),
		buckets:    map[int64]openedBucket{},
	}
}

// openS3Bucket connects to the bucket of an org. With publicOnly, the
// server refuses endpoints on internal addresses, so a user cannot make it
// probe services that only it can reach.
func openS3Bucket(publicOnly bool) func(context.Context, *model.BuildCacheBucket) (Storage, error) {
	return func(ctx context.Context, bucket *model.BuildCacheBucket) (Storage, error) {
		return NewS3Storage(ctx, S3Options{
			Endpoint:   bucket.Endpoint,
			Bucket:     bucket.Bucket,
			Region:     bucket.Region,
			AccessKey:  bucket.AccessKey,
			SecretKey:  bucket.SecretKey,
			SSL:        bucket.SSL,
			PathStyle:  bucket.PathStyle,
			Prefix:     bucket.Prefix,
			PublicOnly: publicOnly,
		})
	}
}

// storageFor returns the storage of a bucket ID, 0 being the server's.
func (s *Service) storageFor(ctx context.Context, bucketID int64) (Storage, error) {
	if bucketID == model.BuildCacheGlobalStorage {
		if s.storageDown.Load() {
			return nil, ErrStorageUnavailable
		}
		return s.storage, nil
	}
	bucket, err := s.store.BuildCacheBucketGet(bucketID)
	if errors.Is(err, types.RecordNotExist) {
		return nil, ErrBucketGone
	}
	if err != nil {
		return nil, err
	}

	s.bucketsMu.Lock()
	opened, ok := s.buckets[bucketID]
	s.bucketsMu.Unlock()
	if ok && opened.updated == bucket.Updated {
		return opened.storage, nil
	}

	storage, err := s.openBucket(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to the cache bucket of org %d: %w", bucket.OrgID, err)
	}
	s.bucketsMu.Lock()
	s.buckets[bucketID] = openedBucket{storage: storage, updated: bucket.Updated}
	s.bucketsMu.Unlock()
	return storage, nil
}

// visible reports whether an entry may be restored. While buckets are
// disabled, entries in an org's bucket are neither restored nor deleted: the
// server does not contact the bucket, and the cache is back once buckets are
// enabled again.
func (s *Service) visible(entry *model.BuildCache) bool {
	return entry.BucketID == model.BuildCacheGlobalStorage || s.config.AllowBuckets
}

// BucketsAllowed reports whether orgs may connect buckets of their own.
func (s *Service) BucketsAllowed() bool {
	return s.config.AllowBuckets
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
	// UploadID, PartSize and PartURLs are set by UploadInParts. Part i covers
	// the bytes from i*PartSize, the last part may be shorter.
	UploadID string
	PartSize int64
	PartURLs []string
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
	access := AccessFor(repo, pipeline)
	if s.config.AllowBuckets && access.OrgID != 0 {
		bucket, err := s.store.BuildCacheBucketFind(access.OrgID)
		switch {
		case err == nil:
			access.BucketID = bucket.ID
		case !errors.Is(err, types.RecordNotExist):
			return Access{}, err
		}
	}
	return access, nil
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
//
// A check only asks whether key is cached, as a save does before archiving:
// it is neither counted as a hit or miss nor held against the egress limit,
// and its hit carries no download URL.
func (s *Service) Restore(ctx context.Context, access Access, key string, restoreKeys []string, check bool) (*Hit, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	for _, prefix := range restoreKeys {
		if err := ValidateKey(prefix); err != nil {
			return nil, err
		}
	}

	// Answered before the lookup: a miss would make a saving step archive
	// for nothing, and would count as a miss.
	if access.BucketID == model.BuildCacheGlobalStorage && s.storageDown.Load() {
		return nil, ErrStorageUnavailable
	}

	hit, err := s.findStored(ctx, access, key, restoreKeys)
	if errors.Is(err, ErrNotFound) && !check {
		s.record(access, model.BuildCacheUsageCounts{Misses: 1})
	}
	if err != nil {
		return nil, err
	}

	if !check {
		if err := s.checkEgress(access, hit.Entry.Size); err != nil {
			s.record(access, model.BuildCacheUsageCounts{Denied: 1})
			return nil, err
		}
	}

	hit.Entry.LastUsed = s.now().Unix()
	if err := s.store.BuildCacheUpdate(hit.Entry); err != nil {
		return nil, err
	}
	if check {
		// A check gets no download URL, or it could restore past the
		// egress limit without being counted.
		return hit, nil
	}
	hit.URL, err = s.downloadURL(ctx, hit.Entry)
	if err != nil {
		return nil, err
	}

	counts := model.BuildCacheUsageCounts{Egress: hit.Entry.Size}
	if hit.Exact {
		counts.Hits = 1
	} else {
		counts.PartialHits = 1
	}
	s.record(access, counts)
	return hit, nil
}

// findStored is find, skipping entries whose archive is gone from storage. Such
// an entry is deleted, so the key can be saved again; otherwise it would never
// restore and also block its key until it expires.
//
// An entry kept in another storage than the org uses now, left over from
// connecting or disconnecting a bucket, is deleted the same way.
func (s *Service) findStored(ctx context.Context, access Access, key string, restoreKeys []string) (*Hit, error) {
	for {
		hit, err := s.find(access, key, restoreKeys)
		if err != nil {
			return nil, err
		}
		if hit.Entry.BucketID != access.BucketID {
			if err := s.forget(ctx, hit.Entry); err != nil {
				return nil, err
			}
			continue
		}
		storage, err := s.storageFor(ctx, hit.Entry.BucketID)
		if err != nil {
			return nil, err
		}
		_, err = storage.Size(ctx, objectName(hit.Entry))
		if err == nil {
			return hit, nil
		}
		if !errors.Is(err, ErrObjectNotFound) {
			return nil, err
		}
		log.Warn().Int64("repo_id", hit.Entry.RepoID).Str("key", hit.Entry.Key).Str("scope", hit.Entry.Scope).
			Msg("build cache archive is missing from storage, deleting its entry")
		if err := s.delete(ctx, hit.Entry); err != nil {
			return nil, err
		}
	}
}

func (s *Service) find(access Access, key string, restoreKeys []string) (*Hit, error) {
	for _, scope := range access.ReadScopes {
		entry, err := s.store.BuildCacheFind(access.RepoID, scope, key)
		if err == nil && entry.Committed && s.visible(entry) {
			return &Hit{Entry: entry, Exact: true}, nil
		}
		if err != nil && !errors.Is(err, types.RecordNotExist) {
			return nil, err
		}
	}
	for _, scope := range access.ReadScopes {
		for _, prefix := range restoreKeys {
			entry, err := s.store.BuildCacheFindPrefix(access.RepoID, scope, prefix)
			if err == nil && s.visible(entry) {
				return &Hit{Entry: entry}, nil
			}
			if err == nil {
				continue
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
		case !s.visible(entry):
			// Kept in a bucket while buckets are disabled; it must survive
			// until they are enabled again.
			return nil, ErrExists
		case entry.BucketID != access.BucketID:
			// Left over from the storage the org used before.
			if err := s.forget(ctx, entry); err != nil {
				return nil, err
			}
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

	// A storage that cannot be reached must fail before the key is reserved,
	// or the pending entry would block the key until it is abandoned.
	if _, err := s.storageFor(ctx, access.BucketID); err != nil {
		return nil, err
	}

	s.quotaMu.Lock()
	defer s.quotaMu.Unlock()

	if err := s.makeRoom(ctx, access, size); err != nil {
		return nil, err
	}

	now := s.now().Unix()
	entry := &model.BuildCache{
		RepoID:   access.RepoID,
		BucketID: access.BucketID,
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
		return nil, errors.Join(err, s.deleteRow(entry))
	}
	return &Reservation{Entry: entry, URL: url}, nil
}

// UploadInParts switches a reservation to an upload in parts, which the cache
// step sends in parallel. It leaves the reservation alone when the storage
// cannot take parts or the archive fits into one.
func (s *Service) UploadInParts(ctx context.Context, r *Reservation) error {
	if r.Entry.Size <= partSize {
		return nil
	}
	plain, err := s.storageFor(ctx, r.Entry.BucketID)
	if err != nil {
		return err
	}
	storage, ok := plain.(MultipartStorage)
	if !ok {
		return nil
	}
	size := max(partSize, (r.Entry.Size+maxParts-1)/maxParts)
	parts := int((r.Entry.Size + size - 1) / size)
	uploadID, urls, err := storage.StartMultipart(ctx, objectName(r.Entry), parts, s.config.URLExpiry)
	if err != nil {
		return err
	}
	r.UploadID, r.PartSize, r.PartURLs = uploadID, size, urls
	return nil
}

func (s *Service) abandoned(entry *model.BuildCache) bool {
	return !entry.Committed && s.now().Sub(time.Unix(entry.Created, 0)) > s.config.PendingTTL
}

// makeRoom evicts least recently used committed entries until size more bytes
// fit into the repo, org and total quota. A bucket the org connected is
// limited only by the size limit its owner set.
func (s *Service) makeRoom(ctx context.Context, access Access, size int64) error {
	if access.BucketID != model.BuildCacheGlobalStorage {
		return s.makeBucketRoom(ctx, access.BucketID, size)
	}
	orgLimit, err := s.orgSizeLimit(access.OrgID)
	if err != nil {
		return err
	}
	for _, limit := range []int64{s.config.MaxRepoSize, orgLimit, s.config.MaxTotalSize} {
		if limit > 0 && size > limit {
			return ErrQuotaExceeded
		}
	}
	if _, err := s.evictUntil(ctx, model.BuildCacheFilter{RepoID: access.RepoID}, s.config.MaxRepoSize, size); err != nil {
		return err
	}
	if access.OrgID != 0 {
		if _, err := s.evictUntil(ctx, model.BuildCacheFilter{OrgID: access.OrgID}, orgLimit, size); err != nil {
			return err
		}
	}
	_, err = s.evictUntil(ctx, model.BuildCacheFilter{}, s.config.MaxTotalSize, size)
	return err
}

// makeBucketRoom evicts entries of a bucket until size more bytes fit into
// the size limit its owner set.
func (s *Service) makeBucketRoom(ctx context.Context, bucketID, size int64) error {
	bucket, err := s.store.BuildCacheBucketGet(bucketID)
	if errors.Is(err, types.RecordNotExist) {
		return ErrBucketGone
	}
	if err != nil {
		return err
	}
	if bucket.MaxSize > 0 && size > bucket.MaxSize {
		return ErrQuotaExceeded
	}
	_, err = s.evictUntil(ctx, model.BuildCacheFilter{BucketID: bucketID}, bucket.MaxSize, size)
	return err
}

// evictUntil evicts entries matching filter until their size plus extra fits
// into limit, reporting what it removed. A limit of 0 means unlimited.
func (s *Service) evictUntil(ctx context.Context, filter model.BuildCacheFilter, limit, extra int64) (CleanupResult, error) {
	var result CleanupResult
	if limit <= 0 {
		return result, nil
	}
	for {
		used, err := s.store.BuildCacheSize(filter)
		if err != nil {
			return result, err
		}
		if used+extra <= limit {
			return result, nil
		}
		victims, err := s.store.BuildCacheListLRU(filter, evictBatch)
		if err != nil {
			return result, err
		}
		if len(victims) == 0 {
			// Only pending uploads are left, nothing can be evicted.
			return result, ErrQuotaExceeded
		}
		for _, victim := range victims {
			if err := s.delete(ctx, victim); err != nil {
				return result, err
			}
			result.add(victim)
			used -= victim.Size
			log.Debug().Int64("repo_id", victim.RepoID).Str("key", victim.Key).Msg("evicted build cache entry to make room")
			if used+extra <= limit {
				return result, nil
			}
		}
	}
}

// Commit marks an uploaded entry as restorable, recording its stored size.
// UploadID is set when the archive was uploaded in parts, which are assembled
// first.
func (s *Service) Commit(ctx context.Context, access Access, id int64, uploadID string) (*model.BuildCache, error) {
	entry, err := s.writable(access, id)
	if err != nil {
		return nil, err
	}

	storage, err := s.storageFor(ctx, entry.BucketID)
	if err != nil {
		return nil, err
	}
	if uploadID != "" {
		multipart, ok := storage.(MultipartStorage)
		if !ok {
			return nil, ErrNotFound
		}
		if err := multipart.CompleteMultipart(ctx, objectName(entry), uploadID); err != nil {
			return nil, fmt.Errorf("cache archive was not uploaded: %w", err)
		}
	}

	size, err := storage.Size(ctx, objectName(entry))
	if err != nil {
		return nil, fmt.Errorf("cache archive was not uploaded: %w", err)
	}

	if size > entry.Size {
		// The upload is larger than reserved, so claim the difference too.
		s.quotaMu.Lock()
		err := s.makeRoom(ctx, access, size-entry.Size)
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
	if err := s.store.BuildCacheUpdate(entry); err != nil {
		return nil, err
	}
	s.record(access, model.BuildCacheUsageCounts{Saves: 1, Ingress: size})
	return entry, nil
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
	if entry.RepoID != access.RepoID || access.WriteScope == "" || entry.Scope != access.WriteScope || entry.Committed ||
		entry.BucketID != access.BucketID {
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
	if entry.RepoID != access.RepoID || !access.CanRead(entry.Scope) || !entry.Committed || entry.BucketID != access.BucketID {
		return nil, 0, ErrNotFound
	}
	storage, err := s.storageFor(ctx, entry.BucketID)
	if err != nil {
		return nil, 0, err
	}
	r, err := storage.Open(ctx, objectName(entry))
	return r, entry.Size, err
}

// WriteBlob stores the upload of a pending entry, accepting at most the
// reserved size.
func (s *Service) WriteBlob(ctx context.Context, access Access, id int64, r io.Reader) error {
	entry, err := s.writable(access, id)
	if err != nil {
		return err
	}
	storage, err := s.storageFor(ctx, entry.BucketID)
	if err != nil {
		return err
	}
	_, err = storage.Write(ctx, objectName(entry), r, entry.Size)
	return err
}

func (s *Service) downloadURL(ctx context.Context, entry *model.BuildCache) (string, error) {
	storage, err := s.storageFor(ctx, entry.BucketID)
	if err != nil {
		return "", err
	}
	url, err := storage.DownloadURL(ctx, objectName(entry), s.config.URLExpiry)
	if err != nil || url != "" {
		return url, err
	}
	return fmt.Sprintf("%s/%d", s.config.BlobURL, entry.ID), nil
}

func (s *Service) uploadURL(ctx context.Context, entry *model.BuildCache) (string, error) {
	storage, err := s.storageFor(ctx, entry.BucketID)
	if err != nil {
		return "", err
	}
	url, err := storage.UploadURL(ctx, objectName(entry), s.config.URLExpiry)
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
// next cleanup retries rather than an archive nothing refers to. The row of an
// entry whose bucket was disconnected is just dropped.
func (s *Service) delete(ctx context.Context, entry *model.BuildCache) error {
	storage, err := s.storageFor(ctx, entry.BucketID)
	if errors.Is(err, ErrBucketGone) {
		return s.deleteRow(entry)
	}
	if err != nil {
		return err
	}
	if multipart, ok := storage.(MultipartStorage); ok && !entry.Committed {
		// The parts of an upload that never finished are stored, and billed,
		// until they are aborted.
		if err := multipart.AbortMultipart(ctx, objectName(entry)); err != nil {
			log.Warn().Err(err).Int64("id", entry.ID).Msg("could not abort the unfinished upload of a build cache entry")
		}
	}
	if err := storage.Delete(ctx, objectName(entry)); err != nil {
		return fmt.Errorf("delete cache archive %d: %w", entry.ID, err)
	}
	return s.deleteRow(entry)
}

// forget deletes an entry, dropping its row even when its archive cannot be
// deleted: the archive is in a bucket the org owns, or will be in one it
// disconnects, and the row must not shadow the key.
func (s *Service) forget(ctx context.Context, entry *model.BuildCache) error {
	if err := s.delete(ctx, entry); err != nil {
		log.Warn().Err(err).Int64("id", entry.ID).Int64("bucket_id", entry.BucketID).
			Msg("could not delete the archive of a build cache entry, dropping the entry anyway")
		return s.deleteRow(entry)
	}
	return nil
}

func (s *Service) deleteRow(entry *model.BuildCache) error {
	if err := s.store.BuildCacheDelete(entry.ID); err != nil && !errors.Is(err, types.RecordNotExist) {
		return err
	}
	return nil
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
			// Every deletion would fail; the next run after recovery catches up.
			if s.storageDown.Load() {
				continue
			}
			if _, err := s.Cleanup(ctx); err != nil {
				log.Error().Err(err).Msg("build cache cleanup failed")
			}
		}
	}
}

// CheckStorage probes the server's storage and records whether cache requests
// may use it, logging when that changes. A storage that cannot be probed is
// taken to be usable.
func (s *Service) CheckStorage(ctx context.Context) error {
	checker, ok := s.storage.(Checker)
	if !ok {
		return nil
	}
	err := checker.Check(ctx)
	wasDown := s.storageDown.Swap(err != nil)
	switch {
	case err != nil && !wasDown:
		log.Error().Err(err).Msg("build cache storage is unavailable, cache steps skip caching until it is back")
	case err == nil && wasDown:
		log.Info().Msg("build cache storage is available again")
	}
	return err
}

// RunStorageCheck runs CheckStorage every interval until ctx is done.
func (s *Service) RunStorageCheck(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.CheckStorage(ctx)
		}
	}
}
