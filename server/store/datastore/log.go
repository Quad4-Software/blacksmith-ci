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

	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v6/server/model"
)

// Maximum number of records to store in one PostgreSQL statement.
// Too large a value results in an error because PostgreSQL only supports 65535 parameters.
const pgBatchSize = 1000

func (s storage) LogFind(step *model.Step) ([]*model.LogEntry, error) {
	var logEntries []*model.LogEntry
	return logEntries, s.db.NewSelect().Model(&logEntries).
		Where("step_id = ?", step.ID).
		Order("line ASC", "id ASC").
		Scan(context.Background())
}

func (s storage) LogFindAfter(stepID int64, afterLogID int64) ([]*model.LogEntry, error) {
	var logEntries []*model.LogEntry
	return logEntries, s.db.NewSelect().Model(&logEntries).
		Where("step_id = ? AND id > ?", stepID, afterLogID).
		Order("id ASC").
		Scan(context.Background())
}

func (s storage) LogAppend(_ *model.Step, logEntries []*model.LogEntry) error {
	if len(logEntries) == 0 {
		return nil
	}

	ctx := context.Background()

	var err error
	// TODO: adapted from slices.Chunk(); switch to it in Go 1.23+
	for i := 0; i < len(logEntries); i += pgBatchSize {
		end := min(pgBatchSize, len(logEntries[i:]))
		chunk := logEntries[i : i+end]

		// Rely on the UNIQUE(step_id, line) index to dedup atomically instead of
		// a racy SELECT-then-INSERT that scanned every existing line per step.
		q := s.db.NewInsert().Model(&chunk)
		if s.driver == DriverMysql {
			q = q.On("DUPLICATE KEY UPDATE step_id = step_id") // no-op = DO NOTHING
		} else {
			q = q.On("CONFLICT (step_id, line) DO NOTHING")
		}
		if _, err = q.Exec(ctx); err != nil {
			log.Error().Err(err).Msg("could not store log entries to db")
		}
	}

	return err
}

func (s storage) LogDelete(step *model.Step) error {
	return logDelete(context.Background(), s.db, step.ID)
}

func logDelete(ctx context.Context, db bun.IDB, stepID int64) error {
	_, err := db.NewDelete().Model((*model.LogEntry)(nil)).Where("step_id = ?", stepID).Exec(ctx)
	return err
}
