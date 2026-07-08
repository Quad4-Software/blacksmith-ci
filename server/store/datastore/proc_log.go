// Copyright 2021 Woodpecker Authors
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
	"database/sql"
	"errors"

	"codefloe.com/crowci/crow/v6/server/model"
)

func (s storage) ProcLogInsert(ctx context.Context, entries []*model.ProcLogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	_, err := s.db.NewInsert().Model(&entries).Exec(ctx)
	return err
}

func (s storage) ProcLogTail(
	ctx context.Context, sourceType string, sourceID int64, sinceID int64, limit int,
) ([]*model.ProcLogEntry, error) {
	var out []*model.ProcLogEntry
	err := s.db.NewSelect().
		Model(&out).
		Where("source_type = ?", sourceType).
		Where("source_id = ?", sourceID).
		Where("id > ?", sinceID).
		OrderExpr("id ASC").
		Limit(limit).
		Scan(ctx)
	return out, err
}

func (s storage) ProcLogTrim(
	ctx context.Context, sourceType string, sourceID int64, keepLastN int,
) error {
	// Find the cutoff id: the id of the (keepLastN+1)-th most recent entry.
	// Anything with id <= cutoff is trimmed.
	var cutoff int64
	err := s.db.NewSelect().
		ColumnExpr("id").
		TableExpr("proc_log_entries").
		Where("source_type = ?", sourceType).
		Where("source_id = ?", sourceID).
		OrderExpr("id DESC").
		Offset(keepLastN).
		Limit(1).
		Scan(ctx, &cutoff)
	if errors.Is(err, sql.ErrNoRows) {
		// No row past the offset: nothing to trim.
		return nil
	}
	if err != nil {
		return err
	}
	_, err = s.db.NewDelete().
		Model((*model.ProcLogEntry)(nil)).
		Where("source_type = ?", sourceType).
		Where("source_id = ?", sourceID).
		Where("id <= ?", cutoff).
		Exec(ctx)
	return err
}

func (s storage) ProcLogListSources(ctx context.Context) ([]model.ProcLogSourceKey, error) {
	var out []model.ProcLogSourceKey
	err := s.db.NewSelect().
		ColumnExpr("DISTINCT source_type, source_id").
		TableExpr("proc_log_entries").
		Scan(ctx, &out)
	return out, err
}
