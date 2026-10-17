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

func (s storage) BuildCacheListLRU(repoID int64, limit int) ([]*model.BuildCache, error) {
	var entries []*model.BuildCache
	q := s.db.NewSelect().Model(&entries).
		Where("committed = ?", true).
		Order("last_used ASC", "id ASC").
		Limit(int64(limit))
	if repoID != 0 {
		q = q.Where("repo_id = ?", repoID)
	}
	return entries, q.Scan(context.Background())
}

func (s storage) BuildCacheListStale(lastUsedBefore, pendingBefore int64) ([]*model.BuildCache, error) {
	var entries []*model.BuildCache
	return entries, s.db.NewSelect().Model(&entries).
		Where("(committed = ? AND last_used < ?) OR (committed = ? AND created < ?)", true, lastUsedBefore, false, pendingBefore).
		Order("id ASC").
		Scan(context.Background())
}

func (s storage) BuildCacheSize(repoID int64) (int64, error) {
	var total int64
	q := s.db.NewSelect().Model((*model.BuildCache)(nil)).ColumnExpr("COALESCE(SUM(size), 0)")
	if repoID != 0 {
		q = q.Where("repo_id = ?", repoID)
	}
	return total, q.Scan(context.Background(), &total)
}

func (s storage) BuildCacheUpdate(entry *model.BuildCache) error {
	_, err := s.db.NewUpdate().Model(entry).WherePK().Exec(context.Background())
	return err
}

func (s storage) BuildCacheDelete(id int64) error {
	return wrapDelete(s.db.NewDelete().Model((*model.BuildCache)(nil)).Where("id = ?", id).Exec(context.Background()))
}
