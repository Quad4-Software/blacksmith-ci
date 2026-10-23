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
	"sort"
	"strings"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

var (
	// ErrBucketsDisabled means the server does not let orgs connect buckets.
	ErrBucketsDisabled = errors.New("connecting cache buckets is disabled on this server")
	// ErrInvalidBucket means a bucket could not be connected.
	ErrInvalidBucket = errors.New("invalid cache bucket")
	// ErrInvalidSettings means a limit is below -1.
	ErrInvalidSettings = errors.New("limits must be -1 (none), 0 (server default) or positive")
)

// ReportDays is how many days usage reports cover.
const ReportDays = 30

// Overview is the cache of the whole server.
type Overview struct {
	// Defaults apply to orgs without settings of their own.
	Defaults     Limits `json:"defaults"`
	MaxRepoSize  int64  `json:"max_repo_size"  doc:"maximum size of one repository's caches in bytes; 0 is unlimited"`
	MaxTotalSize int64  `json:"max_total_size" doc:"maximum size of all caches in the server's storage in bytes; 0 is unlimited"`
	AllowBuckets bool   `json:"allow_buckets"  doc:"whether orgs may connect buckets of their own"`
	// Size and Entries cover the server's storage.
	Size    int64 `json:"size"`
	Entries int64 `json:"entries"`
	// BucketSize and BucketEntries cover the buckets orgs connected.
	BucketSize    int64 `json:"bucket_size"`
	BucketEntries int64 `json:"bucket_entries"`
	// Usage covers the last ReportDays days of all storage.
	Usage model.BuildCacheUsageCounts `json:"usage"`
	// EgressMonth is this month's egress from the server's storage.
	EgressMonth int64         `json:"egress_month"`
	Orgs        []*OrgSummary `json:"orgs"`
} //	@name	BuildCacheOverview

// OrgSummary is the cache of one org.
type OrgSummary struct {
	OrgID  int64  `json:"org_id"`
	Name   string `json:"name"`
	IsUser bool   `json:"is_user"`
	// Size, Entries and Repos cover the server's storage.
	Size    int64 `json:"size"`
	Entries int64 `json:"entries"`
	Repos   int64 `json:"repos"`
	// BucketSize and BucketEntries cover the bucket the org connected.
	BucketSize    int64 `json:"bucket_size"`
	BucketEntries int64 `json:"bucket_entries"`
	// Usage covers the last ReportDays days of all storage.
	Usage model.BuildCacheUsageCounts `json:"usage"`
	// EgressMonth is this month's egress from the server's storage,
	// BucketEgressMonth from the bucket the org connected.
	EgressMonth       int64 `json:"egress_month"`
	BucketEgressMonth int64 `json:"bucket_egress_month"`
	// Settings are what an admin set; Limits what is in effect.
	Settings model.BuildCacheSettings `json:"settings"`
	Limits   Limits                   `json:"limits"`
	Bucket   *model.BuildCacheBucket  `json:"bucket,omitempty"`
} //	@name	BuildCacheOrgSummary

// RepoSummary is the cache of one repo of an org.
type RepoSummary struct {
	RepoID   int64  `json:"repo_id"`
	FullName string `json:"full_name"`
	// Size and Entries cover all storage.
	Size    int64                       `json:"size"`
	Entries int64                       `json:"entries"`
	Usage   model.BuildCacheUsageCounts `json:"usage"`
} //	@name	BuildCacheRepoSummary

// OrgDetail is an org's cache with a breakdown per repo and day.
type OrgDetail struct {
	OrgSummary
	RepoList []*RepoSummary `json:"repo_list"`
	// Daily is the usage of the last ReportDays days, oldest first; days
	// without traffic are left out.
	Daily []*model.BuildCacheUsageGroup `json:"daily"`
} //	@name	BuildCacheOrgDetail

func (s *Service) reportSince() int64 {
	return dayOf(s.now().Add(-(ReportDays - 1) * day))
}

// Overview summarizes the cache of every org that has entries, usage,
// settings or a bucket.
func (s *Service) Overview() (*Overview, error) {
	overview := &Overview{
		Defaults:     s.DefaultLimits(),
		MaxRepoSize:  s.config.MaxRepoSize,
		MaxTotalSize: s.config.MaxTotalSize,
		AllowBuckets: s.config.AllowBuckets,
		Orgs:         []*OrgSummary{},
	}
	orgs := map[int64]*OrgSummary{}
	summary := func(orgID int64) *OrgSummary {
		if org, ok := orgs[orgID]; ok {
			return org
		}
		org := &OrgSummary{OrgID: orgID, Settings: model.BuildCacheSettings{OrgID: orgID}, Limits: overview.Defaults}
		orgs[orgID] = org
		return org
	}

	sizes, err := s.store.BuildCacheSizesByOrg()
	if err != nil {
		return nil, err
	}
	for _, size := range sizes {
		org := summary(size.OrgID)
		if size.Own {
			org.BucketSize += size.Size
			org.BucketEntries += size.Entries
			overview.BucketSize += size.Size
			overview.BucketEntries += size.Entries
		} else {
			org.Size += size.Size
			org.Entries += size.Entries
			org.Repos += size.Repos
			overview.Size += size.Size
			overview.Entries += size.Entries
		}
	}

	usage, err := s.store.BuildCacheUsageByOrg(model.BuildCacheUsageFilter{BucketID: model.BuildCacheAnyStorage, Since: s.reportSince()})
	if err != nil {
		return nil, err
	}
	for _, group := range usage {
		summary(group.OrgID).Usage.Add(group.BuildCacheUsageCounts)
		overview.Usage.Add(group.BuildCacheUsageCounts)
	}
	month, err := s.store.BuildCacheUsageByOrg(model.BuildCacheUsageFilter{BucketID: model.BuildCacheAnyStorage, Since: s.monthStart()})
	if err != nil {
		return nil, err
	}
	for _, group := range month {
		if group.Own {
			summary(group.OrgID).BucketEgressMonth += group.Egress
			continue
		}
		summary(group.OrgID).EgressMonth += group.Egress
		overview.EgressMonth += group.Egress
	}

	settings, err := s.store.BuildCacheSettingsList()
	if err != nil {
		return nil, err
	}
	for _, setting := range settings {
		org := summary(setting.OrgID)
		org.Settings = *setting
		org.Limits = s.limitsOf(setting)
	}
	buckets, err := s.store.BuildCacheBucketList()
	if err != nil {
		return nil, err
	}
	for _, bucket := range buckets {
		summary(bucket.OrgID).Bucket = bucket
	}

	for id, org := range orgs {
		if err := s.nameOrg(org); err != nil {
			return nil, err
		}
		if id != 0 || org.Size > 0 || org.BucketSize > 0 {
			overview.Orgs = append(overview.Orgs, org)
		}
	}
	sort.Slice(overview.Orgs, func(i, j int) bool {
		a, b := overview.Orgs[i], overview.Orgs[j]
		if a.Size+a.BucketSize != b.Size+b.BucketSize {
			return a.Size+a.BucketSize > b.Size+b.BucketSize
		}
		return a.OrgID < b.OrgID
	})
	return overview, nil
}

// nameOrg fills in the name of an org; an org deleted since leaves it empty.
func (s *Service) nameOrg(summary *OrgSummary) error {
	if summary.OrgID == 0 {
		return nil
	}
	org, err := s.store.OrgGet(summary.OrgID)
	if errors.Is(err, types.RecordNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	summary.Name = org.Name
	summary.IsUser = org.IsUser
	return nil
}

// Org details the cache of one org.
func (s *Service) Org(orgID int64) (*OrgDetail, error) {
	detail := &OrgDetail{
		OrgSummary: OrgSummary{OrgID: orgID, Settings: model.BuildCacheSettings{OrgID: orgID}},
		RepoList:   []*RepoSummary{},
	}
	if err := s.nameOrg(&detail.OrgSummary); err != nil {
		return nil, err
	}
	settings, err := s.settingsOf(orgID)
	if err != nil {
		return nil, err
	}
	if settings != nil {
		detail.Settings = *settings
	}
	detail.Limits = s.limitsOf(settings)
	bucket, err := s.store.BuildCacheBucketFind(orgID)
	switch {
	case err == nil:
		detail.Bucket = bucket
	case !errors.Is(err, types.RecordNotExist):
		return nil, err
	}

	repos := map[int64]*RepoSummary{}
	repo := func(repoID int64) *RepoSummary {
		if summary, ok := repos[repoID]; ok {
			return summary
		}
		summary := &RepoSummary{RepoID: repoID}
		repos[repoID] = summary
		return summary
	}
	sizes, err := s.store.BuildCacheSizesByRepo(orgID)
	if err != nil {
		return nil, err
	}
	for _, size := range sizes {
		summary := repo(size.RepoID)
		summary.Size += size.Size
		summary.Entries += size.Entries
		if size.Own {
			detail.BucketSize += size.Size
			detail.BucketEntries += size.Entries
		} else {
			detail.Size += size.Size
			detail.Entries += size.Entries
			detail.Repos++
		}
	}

	since := s.reportSince()
	usage, err := s.store.BuildCacheUsageByRepo(model.BuildCacheUsageFilter{OrgID: orgID, BucketID: model.BuildCacheAnyStorage, Since: since})
	if err != nil {
		return nil, err
	}
	for _, group := range usage {
		repo(group.RepoID).Usage.Add(group.BuildCacheUsageCounts)
		detail.Usage.Add(group.BuildCacheUsageCounts)
	}
	month, err := s.store.BuildCacheUsageSum(model.BuildCacheUsageFilter{OrgID: orgID, Since: s.monthStart()})
	if err != nil {
		return nil, err
	}
	detail.EgressMonth = month.Egress
	if detail.Bucket != nil {
		bucketMonth, err := s.store.BuildCacheUsageSum(model.BuildCacheUsageFilter{BucketID: detail.Bucket.ID, Since: s.monthStart()})
		if err != nil {
			return nil, err
		}
		detail.BucketEgressMonth = bucketMonth.Egress
	}
	detail.Daily, err = s.store.BuildCacheUsageByDay(model.BuildCacheUsageFilter{OrgID: orgID, BucketID: model.BuildCacheAnyStorage, Since: since})
	if err != nil {
		return nil, err
	}

	for _, summary := range repos {
		if r, err := s.store.GetRepo(summary.RepoID); err == nil {
			summary.FullName = r.FullName
		}
		detail.RepoList = append(detail.RepoList, summary)
	}
	sort.Slice(detail.RepoList, func(i, j int) bool {
		a, b := detail.RepoList[i], detail.RepoList[j]
		if a.Size != b.Size {
			return a.Size > b.Size
		}
		return a.RepoID < b.RepoID
	})
	return detail, nil
}

// RepoUsage is a repo's usage over the last ReportDays days.
func (s *Service) RepoUsage(repoID int64) (*model.BuildCacheUsageCounts, error) {
	return s.store.BuildCacheUsageSum(model.BuildCacheUsageFilter{RepoID: repoID, BucketID: model.BuildCacheAnyStorage, Since: s.reportSince()})
}

// SetOrgSettings stores the limits an admin set for an org and evicts its
// entries until they fit into a lowered size limit.
func (s *Service) SetOrgSettings(ctx context.Context, settings *model.BuildCacheSettings) error {
	if settings.OrgID <= 0 {
		return ErrNotFound
	}
	for _, v := range []int64{settings.MaxSize, settings.MaxEgress, settings.TTL} {
		if v < model.BuildCacheLimitNone {
			return ErrInvalidSettings
		}
	}
	settings.Updated = s.now().Unix()
	var err error
	if settings.MaxSize == 0 && settings.MaxEgress == 0 && settings.TTL == 0 {
		err = s.store.BuildCacheSettingsDelete(settings.OrgID)
	} else {
		err = s.store.BuildCacheSettingsSave(settings)
	}
	if err != nil {
		return err
	}
	s.quotaMu.Lock()
	defer s.quotaMu.Unlock()
	_, err = s.enforceOrgSize(ctx, settings.OrgID)
	return err
}

// PurgeOrg removes every entry of an org's repos, in any storage.
func (s *Service) PurgeOrg(ctx context.Context, orgID int64) (CleanupResult, error) {
	var result CleanupResult
	if orgID <= 0 {
		// An org ID of 0 would not filter at all and purge every org.
		return result, ErrNotFound
	}
	entries, err := s.store.BuildCacheListFilter(model.BuildCacheFilter{OrgID: orgID, BucketID: model.BuildCacheAnyStorage})
	if err != nil {
		return result, err
	}
	var errs []error
	for _, entry := range entries {
		if err := s.delete(ctx, entry); err != nil {
			errs = append(errs, err)
			continue
		}
		result.add(entry)
	}
	return result, errors.Join(errs...)
}

// BucketInput connects or changes the bucket of an org. An empty SecretKey
// keeps the secret key of the bucket already connected.
type BucketInput struct {
	Endpoint  string `json:"endpoint"   doc:"S3-compatible endpoint, e.g. s3.amazonaws.com or minio.example.com:9000; must be reachable from pipeline steps"`
	Bucket    string `json:"bucket"     doc:"bucket name"`
	Region    string `json:"region,omitempty"`
	Prefix    string `json:"prefix,omitempty"     doc:"key prefix of the cache archives"`
	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty" doc:"secret key; leave empty to keep the current one"`
	SSL       bool   `json:"ssl"`
	PathStyle bool   `json:"path_style" doc:"use path-style addressing, required by MinIO and some S3-compatible providers"`
	TTL       int64  `json:"ttl"        doc:"seconds an entry is kept without being restored; 0 is the server default, -1 keeps entries until removed by hand"`
	MaxSize   int64  `json:"max_size"   doc:"maximum size of the caches in the bucket in bytes; 0 or -1 is unlimited"`
	MaxEgress int64  `json:"max_egress" doc:"maximum bytes restored from the bucket per calendar month (UTC); 0 or -1 is unlimited"`
}

// sameLocation compares the fields that decide where archives are; changing
// one moves the cache to a new location.
func sameLocation(a, b *model.BuildCacheBucket) bool {
	return a.Endpoint == b.Endpoint && a.Bucket == b.Bucket && a.Prefix == b.Prefix
}

// ConnectBucket connects a bucket to an org, or changes the one it has, after
// checking that the server can reach it. When the location of the cache
// changes, the entries kept in the previous location are removed: they would
// no longer be found.
func (s *Service) ConnectBucket(ctx context.Context, orgID int64, input BucketInput) (*model.BuildCacheBucket, error) {
	if !s.config.AllowBuckets {
		return nil, ErrBucketsDisabled
	}
	input.Endpoint = strings.TrimSpace(input.Endpoint)
	input.Bucket = strings.TrimSpace(input.Bucket)
	input.Prefix = strings.Trim(strings.TrimSpace(input.Prefix), "/")
	if input.Endpoint == "" || input.Bucket == "" {
		return nil, fmt.Errorf("%w: endpoint and bucket are required", ErrInvalidBucket)
	}
	if strings.Contains(input.Endpoint, "/") {
		return nil, fmt.Errorf("%w: the endpoint is a host with an optional port, without scheme or path", ErrInvalidBucket)
	}
	for _, v := range []int64{input.TTL, input.MaxSize, input.MaxEgress} {
		if v < model.BuildCacheLimitNone {
			return nil, ErrInvalidSettings
		}
	}

	current, err := s.store.BuildCacheBucketFind(orgID)
	if err != nil && !errors.Is(err, types.RecordNotExist) {
		return nil, err
	}
	if errors.Is(err, types.RecordNotExist) {
		current = nil
	}

	now := s.now().Unix()
	bucket := &model.BuildCacheBucket{
		OrgID:     orgID,
		Endpoint:  input.Endpoint,
		Bucket:    input.Bucket,
		Region:    strings.TrimSpace(input.Region),
		Prefix:    input.Prefix,
		AccessKey: strings.TrimSpace(input.AccessKey),
		SecretKey: input.SecretKey,
		SSL:       input.SSL,
		PathStyle: input.PathStyle,
		TTL:       input.TTL,
		// A bucket has no server default to fall back to: -1 and 0 are both
		// unlimited.
		MaxSize:   max(input.MaxSize, 0),
		MaxEgress: max(input.MaxEgress, 0),
		Created:   now,
		Updated:   now,
	}
	if current != nil {
		bucket.ID = current.ID
		bucket.Created = current.Created
		// Open buckets are reopened when Updated changes, so it must change
		// even when the bucket is changed twice within a second.
		bucket.Updated = max(now, current.Updated+1)
		if bucket.SecretKey == "" {
			bucket.SecretKey = current.SecretKey
		}
	}

	if _, err := s.openBucket(ctx, bucket); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidBucket, err)
	}

	moved := current == nil || !sameLocation(current, bucket)
	if moved {
		// Entries in the previous location; with no bucket yet those are in
		// the server's storage.
		previous := model.BuildCacheGlobalStorage
		if current != nil {
			previous = current.ID
		}
		s.purgeStorage(ctx, orgID, previous)
	}

	if current == nil {
		err = s.store.BuildCacheBucketCreate(bucket)
	} else {
		err = s.store.BuildCacheBucketUpdate(bucket)
	}
	if err != nil {
		return nil, err
	}
	s.closeBucket(bucket.ID)

	// A lowered size limit applies at once, as for an admin's org limit.
	s.quotaMu.Lock()
	_, err = s.evictCounted(ctx, model.BuildCacheFilter{BucketID: bucket.ID}, bucket.MaxSize)
	s.quotaMu.Unlock()
	if err != nil {
		return nil, err
	}
	return bucket, nil
}

// DisconnectBucket disconnects the bucket of an org, which then caches in the
// server's storage again. The archives in the bucket are deleted if possible.
func (s *Service) DisconnectBucket(ctx context.Context, orgID int64) error {
	bucket, err := s.store.BuildCacheBucketFind(orgID)
	if errors.Is(err, types.RecordNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	s.purgeStorage(ctx, orgID, bucket.ID)
	if err := s.store.BuildCacheBucketDelete(bucket.ID); err != nil && !errors.Is(err, types.RecordNotExist) {
		return err
	}
	s.closeBucket(bucket.ID)
	return nil
}

// purgeStorage removes the entries of an org kept in one storage. Rows are
// dropped even when an archive cannot be deleted, as the org is leaving that
// storage. A bucket holds only its org's entries, so it is purged as a whole,
// including entries of repos deleted since.
func (s *Service) purgeStorage(ctx context.Context, orgID, bucketID int64) {
	filter := model.BuildCacheFilter{BucketID: bucketID}
	if bucketID == model.BuildCacheGlobalStorage {
		filter.OrgID = orgID
	}
	entries, err := s.store.BuildCacheListFilter(filter)
	if err != nil {
		log.Error().Err(err).Int64("org_id", orgID).Msg("could not list the build caches to move")
		return
	}
	for _, entry := range entries {
		if err := s.forget(ctx, entry); err != nil {
			log.Error().Err(err).Int64("id", entry.ID).Msg("could not remove a build cache entry")
		}
	}
}

func (s *Service) closeBucket(id int64) {
	s.bucketsMu.Lock()
	delete(s.buckets, id)
	s.bucketsMu.Unlock()
}

// OrgDeleted removes what is left of the cache of a deleted org: its settings
// and bucket. Entries went with the org's repos.
func (s *Service) OrgDeleted(ctx context.Context, orgID int64) error {
	var errs []error
	if bucket, err := s.store.BuildCacheBucketFind(orgID); err == nil {
		s.purgeStorage(ctx, orgID, bucket.ID)
		errs = append(errs, s.store.BuildCacheBucketDelete(bucket.ID))
		s.closeBucket(bucket.ID)
	} else if !errors.Is(err, types.RecordNotExist) {
		errs = append(errs, err)
	}
	errs = append(errs, s.store.BuildCacheSettingsDelete(orgID))
	return errors.Join(errs...)
}
