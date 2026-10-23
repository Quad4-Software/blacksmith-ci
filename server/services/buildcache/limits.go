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
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

const (
	day = 24 * time.Hour
	// Usage is kept this long, so a year can be compared with the last.
	usageRetention = 400 * day
)

// Limits are the limits in effect for an org's caches in the server's storage.
// Zero means unlimited, for TTL that entries are kept until evicted by size.
type Limits struct {
	MaxSize   int64 `json:"max_size"   doc:"maximum size of the org's caches in bytes; 0 is unlimited"`
	MaxEgress int64 `json:"max_egress" doc:"maximum bytes restored per calendar month (UTC); 0 is unlimited"`
	TTL       int64 `json:"ttl"        doc:"seconds an entry is kept without being restored; 0 keeps entries until evicted by size"`
}

// resolveLimit applies an org's setting to the server default.
func resolveLimit(setting, fallback int64) int64 {
	switch {
	case setting == model.BuildCacheLimitDefault:
		return fallback
	case setting < 0:
		return 0
	default:
		return setting
	}
}

// DefaultLimits are the limits of an org without settings of its own.
func (s *Service) DefaultLimits() Limits {
	return Limits{
		MaxSize:   s.config.MaxOrgSize,
		MaxEgress: s.config.MaxOrgEgress,
		TTL:       int64(s.config.TTL / time.Second),
	}
}

// limitsOf resolves the settings of an org, which may be nil.
func (s *Service) limitsOf(settings *model.BuildCacheSettings) Limits {
	limits := s.DefaultLimits()
	if settings == nil {
		return limits
	}
	return Limits{
		MaxSize:   resolveLimit(settings.MaxSize, limits.MaxSize),
		MaxEgress: resolveLimit(settings.MaxEgress, limits.MaxEgress),
		TTL:       resolveLimit(settings.TTL, limits.TTL),
	}
}

// settingsOf loads the settings of an org, nil when it has none.
func (s *Service) settingsOf(orgID int64) (*model.BuildCacheSettings, error) {
	if orgID == 0 {
		return nil, nil
	}
	settings, err := s.store.BuildCacheSettingsGet(orgID)
	if errors.Is(err, types.RecordNotExist) {
		return nil, nil
	}
	return settings, err
}

// OrgLimits are the limits in effect for an org.
func (s *Service) OrgLimits(orgID int64) (Limits, error) {
	settings, err := s.settingsOf(orgID)
	if err != nil {
		return Limits{}, err
	}
	return s.limitsOf(settings), nil
}

func (s *Service) orgSizeLimit(orgID int64) (int64, error) {
	if orgID == 0 {
		return 0, nil
	}
	limits, err := s.OrgLimits(orgID)
	return limits.MaxSize, err
}

// monthStart is the start of the calendar month (UTC) egress is counted in.
func (s *Service) monthStart() int64 {
	now := s.now().UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Unix()
}

// dayOf is the start of the UTC day of t.
func dayOf(t time.Time) int64 {
	return t.Unix() / int64(day/time.Second) * int64(day/time.Second)
}

// checkEgress refuses a restore of size bytes that does not fit into this
// month's egress limit: the one an admin set for the org in the server's
// storage, or the one the org set for its own bucket.
func (s *Service) checkEgress(access Access, size int64) error {
	limit, filter, err := s.egressLimit(access)
	if err != nil || limit <= 0 {
		return err
	}
	filter.Since = s.monthStart()
	used, err := s.store.BuildCacheUsageSum(filter)
	if err != nil {
		return err
	}
	if used.Egress+size > limit {
		return ErrEgressExceeded
	}
	return nil
}

// egressLimit is the monthly egress limit of the storage access uses and the
// usage it counts; 0 is unlimited.
func (s *Service) egressLimit(access Access) (int64, model.BuildCacheUsageFilter, error) {
	if access.BucketID != model.BuildCacheGlobalStorage {
		bucket, err := s.store.BuildCacheBucketGet(access.BucketID)
		if err != nil {
			return 0, model.BuildCacheUsageFilter{}, err
		}
		return bucket.MaxEgress, model.BuildCacheUsageFilter{BucketID: bucket.ID}, nil
	}
	if access.OrgID == 0 {
		return 0, model.BuildCacheUsageFilter{}, nil
	}
	limits, err := s.OrgLimits(access.OrgID)
	if err != nil {
		return 0, model.BuildCacheUsageFilter{}, err
	}
	return limits.MaxEgress, model.BuildCacheUsageFilter{OrgID: access.OrgID}, nil
}

// record adds counts to the usage of the access's repo today. Usage is
// reporting only, so a failure is logged rather than failing the cache step.
func (s *Service) record(access Access, counts model.BuildCacheUsageCounts) {
	err := s.store.BuildCacheUsageAdd(&model.BuildCacheUsage{
		RepoID:                access.RepoID,
		OrgID:                 access.OrgID,
		BucketID:              access.BucketID,
		Day:                   dayOf(s.now()),
		BuildCacheUsageCounts: counts,
	})
	if err != nil {
		log.Warn().Err(err).Int64("repo_id", access.RepoID).Msg("could not record build cache usage")
	}
}

// CleanupResult is what a cleanup removed.
type CleanupResult struct {
	Removed int64 `json:"removed" doc:"number of entries removed"`
	Freed   int64 `json:"freed"   doc:"bytes freed"`
}

func (r *CleanupResult) add(entry *model.BuildCache) {
	r.Removed++
	r.Freed += entry.Size
}

// Cleanup removes abandoned uploads and entries unused for longer than their
// TTL, evicts entries until every org and the total fit into their size limit
// again, and drops old usage.
func (s *Service) Cleanup(ctx context.Context) (CleanupResult, error) {
	var (
		result CleanupResult
		errs   []error
	)
	remove := func(entries []*model.BuildCache, err error) {
		if err != nil {
			errs = append(errs, err)
			return
		}
		for _, entry := range entries {
			if err := s.delete(ctx, entry); err != nil {
				errs = append(errs, err)
				continue
			}
			result.add(entry)
		}
	}
	now := s.now()
	cutoff := func(ttl int64) int64 {
		if ttl <= 0 {
			return 0
		}
		return now.Add(-time.Duration(ttl) * time.Second).Unix()
	}

	// Abandoned uploads of any storage the server may contact.
	pendingIn := model.BuildCacheAnyStorage
	if !s.config.AllowBuckets {
		pendingIn = model.BuildCacheGlobalStorage
	}
	remove(s.store.BuildCacheListStale(model.BuildCacheFilter{BucketID: pendingIn}, 0, now.Add(-s.config.PendingTTL).Unix()))

	defaultTTL := int64(s.config.TTL / time.Second)

	// The server's storage, by the TTL an admin set for an org or the default.
	settings, err := s.store.BuildCacheSettingsList()
	if err != nil {
		errs = append(errs, err)
	}
	var customOrgs []int64
	for _, setting := range settings {
		if setting.TTL == model.BuildCacheLimitDefault {
			continue
		}
		customOrgs = append(customOrgs, setting.OrgID)
		if ttl := resolveLimit(setting.TTL, defaultTTL); ttl > 0 {
			remove(s.store.BuildCacheListStale(model.BuildCacheFilter{OrgID: setting.OrgID}, cutoff(ttl), 0))
		}
	}
	if defaultTTL > 0 {
		remove(s.store.BuildCacheListStale(model.BuildCacheFilter{NotOrgIDs: customOrgs}, cutoff(defaultTTL), 0))
	}

	// Connected buckets, by the TTL their org set or the default. While
	// buckets are disabled the server does not contact them.
	var buckets []*model.BuildCacheBucket
	if s.config.AllowBuckets {
		buckets, err = s.store.BuildCacheBucketList()
		if err != nil {
			errs = append(errs, err)
		}
	}
	var customBuckets []int64
	for _, bucket := range buckets {
		if bucket.TTL == model.BuildCacheLimitDefault {
			continue
		}
		customBuckets = append(customBuckets, bucket.ID)
		if ttl := resolveLimit(bucket.TTL, defaultTTL); ttl > 0 {
			remove(s.store.BuildCacheListStale(model.BuildCacheFilter{BucketID: bucket.ID}, cutoff(ttl), 0))
		}
	}
	if defaultTTL > 0 && s.config.AllowBuckets {
		remove(s.store.BuildCacheListStale(model.BuildCacheFilter{BucketID: model.BuildCacheOwnBuckets, NotBucketIDs: customBuckets}, cutoff(defaultTTL), 0))
	}

	if result.Removed > 0 {
		log.Info().Int64("count", result.Removed).Int64("bytes", result.Freed).Msg("removed expired build cache entries")
	}

	// Size limits may have been lowered since the entries were saved.
	sizes, err := s.store.BuildCacheSizesByOrg()
	if err != nil {
		errs = append(errs, err)
	}
	s.quotaMu.Lock()
	for _, size := range sizes {
		if size.Own || size.OrgID == 0 {
			continue
		}
		freed, err := s.enforceOrgSize(ctx, size.OrgID)
		result.Removed += freed.Removed
		result.Freed += freed.Freed
		if err != nil {
			errs = append(errs, err)
		}
	}
	for _, bucket := range buckets {
		freed, err := s.evictCounted(ctx, model.BuildCacheFilter{BucketID: bucket.ID}, bucket.MaxSize)
		result.Removed += freed.Removed
		result.Freed += freed.Freed
		if err != nil {
			errs = append(errs, err)
		}
	}
	evicted, err := s.evictCounted(ctx, model.BuildCacheFilter{}, s.config.MaxTotalSize)
	result.Removed += evicted.Removed
	result.Freed += evicted.Freed
	if err != nil {
		errs = append(errs, err)
	}
	s.quotaMu.Unlock()

	if err := s.store.BuildCacheUsagePrune(dayOf(now.Add(-usageRetention))); err != nil {
		errs = append(errs, err)
	}
	return result, errors.Join(errs...)
}

// enforceOrgSize evicts an org's entries until they fit into its size limit.
// The caller holds quotaMu.
func (s *Service) enforceOrgSize(ctx context.Context, orgID int64) (CleanupResult, error) {
	limit, err := s.orgSizeLimit(orgID)
	if err != nil {
		return CleanupResult{}, err
	}
	return s.evictCounted(ctx, model.BuildCacheFilter{OrgID: orgID}, limit)
}

// evictCounted is evictUntil without extra room. Pending uploads that cannot
// be evicted are not an error here.
func (s *Service) evictCounted(ctx context.Context, filter model.BuildCacheFilter, limit int64) (CleanupResult, error) {
	result, err := s.evictUntil(ctx, filter, limit, 0)
	if errors.Is(err, ErrQuotaExceeded) {
		err = nil
	}
	return result, err
}
