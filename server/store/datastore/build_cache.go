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
	"context"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

const (
	mysqlErrDuplicateEntry = 1062
	pgErrUniqueViolation   = "23505"

	// How many LIKE matches are checked for a real prefix match, see
	// BuildCacheFindPrefix.
	buildCachePrefixCandidates = 50
)

// isUniqueViolation reports whether err is a unique constraint violation on any
// of the supported databases.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgErrUniqueViolation
	}
	var myErr *mysql.MySQLError
	if errors.As(err, &myErr) {
		return myErr.Number == mysqlErrDuplicateEntry
	}
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func (s storage) BuildCacheCreate(entry *model.BuildCache) error {
	_, err := s.db.NewInsert().Model(entry).Exec(context.Background())
	if isUniqueViolation(err) {
		return types.ErrConflict
	}
	return err
}

func (s storage) BuildCacheGet(id int64) (*model.BuildCache, error) {
	entry := new(model.BuildCache)
	return entry, wrapGet(s.db.NewSelect().Model(entry).Where("id = ?", id).Scan(context.Background()))
}

func (s storage) BuildCacheFind(repoID int64, scope, key string) (*model.BuildCache, error) {
	entry := new(model.BuildCache)
	return entry, wrapGet(s.db.NewSelect().Model(entry).
		Where("repo_id = ? AND scope = ? AND cache_key = ?", repoID, scope, key).
		Scan(context.Background()))
}

// BuildCacheFindPrefix narrows the candidates with LIKE and confirms the match
// in Go: SQLite's LIKE ignores ASCII case, while cache keys are case-sensitive.
func (s storage) BuildCacheFindPrefix(repoID int64, scope, prefix string) (*model.BuildCache, error) {
	var candidates []*model.BuildCache
	err := s.db.NewSelect().Model(&candidates).
		Where("repo_id = ? AND scope = ? AND committed = ?", repoID, scope, true).
		Where("cache_key LIKE ? ESCAPE '!'", escapeLike(prefix)+"%").
		Order("created DESC", "id DESC").
		Limit(buildCachePrefixCandidates).
		Scan(context.Background())
	if err != nil {
		return nil, err
	}
	for _, entry := range candidates {
		if strings.HasPrefix(entry.Key, prefix) {
			return entry, nil
		}
	}
	return nil, types.RecordNotExist
}

// escapeLike escapes the LIKE wildcards of s using '!' as the escape character,
// which, unlike a backslash, needs no extra quoting on any supported database.
func escapeLike(s string) string {
	return strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(s)
}

func (s storage) BuildCacheList(repoID int64) ([]*model.BuildCache, error) {
	var entries []*model.BuildCache
	return entries, s.db.NewSelect().Model(&entries).
		Where("repo_id = ?", repoID).
		Order("last_used DESC", "id DESC").
		Scan(context.Background())
}

// orgRepos selects the IDs of the repos currently owned by the orgs in ?.
const orgRepos = "SELECT id FROM repos WHERE org_id IN (?)"

// whereBuildCache applies filter to a query on build_caches.
func whereBuildCache(q *bun.SelectQuery, filter model.BuildCacheFilter) *bun.SelectQuery {
	if filter.RepoID != 0 {
		q = q.Where("repo_id = ?", filter.RepoID)
	}
	if filter.OrgID != 0 {
		q = q.Where("repo_id IN ("+orgRepos+")", bun.List([]int64{filter.OrgID}))
	}
	if len(filter.NotOrgIDs) > 0 {
		q = q.Where("repo_id NOT IN ("+orgRepos+")", bun.List(filter.NotOrgIDs))
	}
	if len(filter.NotBucketIDs) > 0 {
		q = q.Where("bucket_id NOT IN (?)", bun.List(filter.NotBucketIDs))
	}
	return whereBucket(q, filter.BucketID)
}

// whereBucket selects the storage of a cache or usage query.
func whereBucket(q *bun.SelectQuery, bucketID int64) *bun.SelectQuery {
	switch bucketID {
	case model.BuildCacheAnyStorage:
		return q
	case model.BuildCacheOwnBuckets:
		return q.Where("bucket_id > 0")
	default:
		return q.Where("bucket_id = ?", bucketID)
	}
}

func (s storage) BuildCacheListFilter(filter model.BuildCacheFilter) ([]*model.BuildCache, error) {
	var entries []*model.BuildCache
	q := s.db.NewSelect().Model(&entries).Order("id ASC")
	return entries, whereBuildCache(q, filter).Scan(context.Background())
}

func (s storage) BuildCacheListLRU(filter model.BuildCacheFilter, limit int) ([]*model.BuildCache, error) {
	var entries []*model.BuildCache
	q := s.db.NewSelect().Model(&entries).
		Where("committed = ?", true).
		Order("last_used ASC", "id ASC").
		Limit(int64(limit))
	return entries, whereBuildCache(q, filter).Scan(context.Background())
}

func (s storage) BuildCacheListStale(filter model.BuildCacheFilter, lastUsedBefore, pendingBefore int64) ([]*model.BuildCache, error) {
	var entries []*model.BuildCache
	q := s.db.NewSelect().Model(&entries).
		Where("(committed = ? AND last_used < ?) OR (committed = ? AND created < ?)", true, lastUsedBefore, false, pendingBefore).
		Order("id ASC")
	return entries, whereBuildCache(q, filter).Scan(context.Background())
}

func (s storage) BuildCacheSize(filter model.BuildCacheFilter) (int64, error) {
	var total int64
	q := s.db.NewSelect().Model((*model.BuildCache)(nil)).ColumnExpr("COALESCE(SUM(size), 0)")
	return total, whereBuildCache(q, filter).Scan(context.Background(), &total)
}

// ownExpr is 1 for rows kept in a bucket an org connected, 0 for the server's
// storage.
const ownExpr = "CASE WHEN bucket_id = 0 THEN 0 ELSE 1 END"

func (s storage) BuildCacheSizesByOrg() ([]*model.BuildCacheSize, error) {
	sizes := []*model.BuildCacheSize{}
	return sizes, s.db.NewSelect().
		TableExpr("build_caches AS bc").
		Join("JOIN repos AS r ON r.id = bc.repo_id").
		ColumnExpr("r.org_id AS org_id").
		ColumnExpr("CASE WHEN bc.bucket_id = 0 THEN 0 ELSE 1 END AS own").
		ColumnExpr("COALESCE(SUM(bc.size), 0) AS size").
		ColumnExpr("COUNT(*) AS entries").
		ColumnExpr("COUNT(DISTINCT bc.repo_id) AS repos").
		GroupExpr("r.org_id").
		GroupExpr("CASE WHEN bc.bucket_id = 0 THEN 0 ELSE 1 END").
		Scan(context.Background(), &sizes)
}

func (s storage) BuildCacheSizesByRepo(orgID int64) ([]*model.BuildCacheSize, error) {
	sizes := []*model.BuildCacheSize{}
	return sizes, s.db.NewSelect().
		Model((*model.BuildCache)(nil)).
		ColumnExpr("repo_id").
		ColumnExpr(ownExpr+" AS own").
		ColumnExpr("COALESCE(SUM(size), 0) AS size").
		ColumnExpr("COUNT(*) AS entries").
		Where("repo_id IN ("+orgRepos+")", bun.List([]int64{orgID})).
		GroupExpr("repo_id").
		GroupExpr(ownExpr).
		Scan(context.Background(), &sizes)
}

func (s storage) BuildCacheUpdate(entry *model.BuildCache) error {
	_, err := s.db.NewUpdate().Model(entry).WherePK().Exec(context.Background())
	return err
}

func (s storage) BuildCacheDelete(id int64) error {
	return wrapDelete(s.db.NewDelete().Model((*model.BuildCache)(nil)).Where("id = ?", id).Exec(context.Background()))
}

func (s storage) BuildCacheSettingsGet(orgID int64) (*model.BuildCacheSettings, error) {
	settings := new(model.BuildCacheSettings)
	return settings, wrapGet(s.db.NewSelect().Model(settings).Where("org_id = ?", orgID).Scan(context.Background()))
}

func (s storage) BuildCacheSettingsList() ([]*model.BuildCacheSettings, error) {
	settings := []*model.BuildCacheSettings{}
	return settings, s.db.NewSelect().Model(&settings).Order("org_id ASC").Scan(context.Background())
}

func (s storage) BuildCacheSettingsSave(settings *model.BuildCacheSettings) error {
	q := s.db.NewInsert().Model(settings)
	if s.driver == DriverMysql {
		q = q.On("DUPLICATE KEY UPDATE max_size = VALUES(max_size), max_egress = VALUES(max_egress), ttl = VALUES(ttl), updated = VALUES(updated)")
	} else {
		q = q.On("CONFLICT (org_id) DO UPDATE").
			Set("max_size = EXCLUDED.max_size").
			Set("max_egress = EXCLUDED.max_egress").
			Set("ttl = EXCLUDED.ttl").
			Set("updated = EXCLUDED.updated")
	}
	_, err := q.Exec(context.Background())
	return err
}

func (s storage) BuildCacheSettingsDelete(orgID int64) error {
	_, err := s.db.NewDelete().Model((*model.BuildCacheSettings)(nil)).Where("org_id = ?", orgID).Exec(context.Background())
	return err
}

func (s storage) BuildCacheBucketGet(id int64) (*model.BuildCacheBucket, error) {
	bucket := new(model.BuildCacheBucket)
	return bucket, wrapGet(s.db.NewSelect().Model(bucket).Where("id = ?", id).Scan(context.Background()))
}

func (s storage) BuildCacheBucketFind(orgID int64) (*model.BuildCacheBucket, error) {
	bucket := new(model.BuildCacheBucket)
	return bucket, wrapGet(s.db.NewSelect().Model(bucket).Where("org_id = ?", orgID).Scan(context.Background()))
}

func (s storage) BuildCacheBucketList() ([]*model.BuildCacheBucket, error) {
	buckets := []*model.BuildCacheBucket{}
	return buckets, s.db.NewSelect().Model(&buckets).Order("id ASC").Scan(context.Background())
}

func (s storage) BuildCacheBucketCreate(bucket *model.BuildCacheBucket) error {
	_, err := s.db.NewInsert().Model(bucket).Exec(context.Background())
	if isUniqueViolation(err) {
		return types.ErrConflict
	}
	return err
}

func (s storage) BuildCacheBucketUpdate(bucket *model.BuildCacheBucket) error {
	_, err := s.db.NewUpdate().Model(bucket).WherePK().Exec(context.Background())
	return err
}

func (s storage) BuildCacheBucketDelete(id int64) error {
	return wrapDelete(s.db.NewDelete().Model((*model.BuildCacheBucket)(nil)).Where("id = ?", id).Exec(context.Background()))
}

// buildCacheUsageCounters are the columns BuildCacheUsageAdd increments.
var buildCacheUsageCounters = []string{"hits", "partial_hits", "misses", "denied", "saves", "egress", "ingress"}

// BuildCacheUsageAdd increments the counters in one statement, so concurrent
// cache steps on any number of servers never lose an update.
func (s storage) BuildCacheUsageAdd(u *model.BuildCacheUsage) error {
	updates := make([]string, len(buildCacheUsageCounters))
	for i, col := range buildCacheUsageCounters {
		if s.driver == DriverMysql {
			updates[i] = col + " = " + col + " + VALUES(" + col + ")"
		} else {
			updates[i] = col + " = build_cache_usage." + col + " + EXCLUDED." + col
		}
	}
	conflict := "ON CONFLICT (repo_id, bucket_id, day) DO UPDATE SET "
	if s.driver == DriverMysql {
		conflict = "ON DUPLICATE KEY UPDATE "
	}
	query := "INSERT INTO build_cache_usage (repo_id, org_id, bucket_id, day, " + strings.Join(buildCacheUsageCounters, ", ") +
		") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) " + conflict + strings.Join(updates, ", ")
	_, err := s.db.ExecContext(context.Background(), query,
		u.RepoID, u.OrgID, u.BucketID, u.Day,
		u.Hits, u.PartialHits, u.Misses, u.Denied, u.Saves, u.Egress, u.Ingress)
	return err
}

// usageSums selects the sum of every usage counter under its own name.
func usageSums(q *bun.SelectQuery) *bun.SelectQuery {
	for _, col := range buildCacheUsageCounters {
		q = q.ColumnExpr("COALESCE(SUM(" + col + "), 0) AS " + col)
	}
	return q
}

func (s storage) usageQuery(filter model.BuildCacheUsageFilter) *bun.SelectQuery {
	q := usageSums(s.db.NewSelect().Model((*model.BuildCacheUsage)(nil)))
	if filter.OrgID != 0 {
		q = q.Where("org_id = ?", filter.OrgID)
	}
	if filter.RepoID != 0 {
		q = q.Where("repo_id = ?", filter.RepoID)
	}
	if filter.Since != 0 {
		q = q.Where("day >= ?", filter.Since)
	}
	return whereBucket(q, filter.BucketID)
}

func (s storage) BuildCacheUsageSum(filter model.BuildCacheUsageFilter) (*model.BuildCacheUsageCounts, error) {
	counts := new(model.BuildCacheUsageCounts)
	return counts, s.usageQuery(filter).Scan(context.Background(), counts)
}

func (s storage) BuildCacheUsageByOrg(filter model.BuildCacheUsageFilter) ([]*model.BuildCacheUsageGroup, error) {
	groups := []*model.BuildCacheUsageGroup{}
	return groups, s.usageQuery(filter).
		ColumnExpr("org_id").ColumnExpr(ownExpr+" AS own").
		GroupExpr("org_id").GroupExpr(ownExpr).
		Scan(context.Background(), &groups)
}

func (s storage) BuildCacheUsageByRepo(filter model.BuildCacheUsageFilter) ([]*model.BuildCacheUsageGroup, error) {
	groups := []*model.BuildCacheUsageGroup{}
	return groups, s.usageQuery(filter).
		ColumnExpr("repo_id").ColumnExpr(ownExpr+" AS own").
		GroupExpr("repo_id").GroupExpr(ownExpr).
		Scan(context.Background(), &groups)
}

func (s storage) BuildCacheUsageByDay(filter model.BuildCacheUsageFilter) ([]*model.BuildCacheUsageGroup, error) {
	groups := []*model.BuildCacheUsageGroup{}
	return groups, s.usageQuery(filter).
		ColumnExpr("day").
		GroupExpr("day").
		OrderExpr("day ASC").
		Scan(context.Background(), &groups)
}

func (s storage) BuildCacheUsagePrune(before int64) error {
	_, err := s.db.NewDelete().Model((*model.BuildCacheUsage)(nil)).Where("day < ?", before).Exec(context.Background())
	return err
}
