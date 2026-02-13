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
	"github.com/rs/zerolog/log"
	"xorm.io/xorm"

	"codefloe.com/crowci/crow/v5/server/model"
)

// Maximum number of records to store in one PostgreSQL statement.
// Too large a value results in an error because PostgreSQL only supports 65535 parameters.
const pgBatchSize = 1000

func (s storage) LogFind(step *model.Step) ([]*model.LogEntry, error) {
	var logEntries []*model.LogEntry
	return logEntries, s.engine.Asc("id").Where("step_id = ?", step.ID).Find(&logEntries)
}

func (s storage) LogFindAfter(stepID int64, afterLogID int64) ([]*model.LogEntry, error) {
	var logEntries []*model.LogEntry
	return logEntries, s.engine.Asc("id").Where("step_id = ? AND id > ?", stepID, afterLogID).Find(&logEntries)
}

func (s storage) LogAppend(_ *model.Step, logEntries []*model.LogEntry) error {
	if len(logEntries) == 0 {
		return nil
	}

	// Group entries by step_id for efficient duplicate checking
	stepGroups := make(map[int64][]*model.LogEntry)
	for _, entry := range logEntries {
		stepGroups[entry.StepID] = append(stepGroups[entry.StepID], entry)
	}

	var filteredEntries []*model.LogEntry
	for stepID, entries := range stepGroups {
		// Get existing line numbers for this step
		var existingLines []int
		err := s.engine.Table(&model.LogEntry{}).
			Where("step_id = ?", stepID).
			Cols("line").
			Find(&existingLines)
		if err != nil {
			log.Error().Err(err).Int64("stepID", stepID).Msg("failed to query existing log lines")
			// Fall back to inserting all entries if query fails
			filteredEntries = append(filteredEntries, entries...)
			continue
		}

		// Create a set of existing lines for fast lookup
		existingLineSet := make(map[int]bool)
		for _, line := range existingLines {
			existingLineSet[line] = true
		}

		// Filter out entries that already exist
		for _, entry := range entries {
			if !existingLineSet[entry.Line] {
				filteredEntries = append(filteredEntries, entry)
			}
		}
	}

	if len(filteredEntries) == 0 {
		// All entries already exist
		return nil
	}

	var err error
	// TODO: adapted from slices.Chunk(); switch to it in Go 1.23+
	for i := 0; i < len(filteredEntries); i += pgBatchSize {
		end := min(pgBatchSize, len(filteredEntries[i:]))
		chunk := filteredEntries[i : i+end]

		if _, err = s.engine.Insert(chunk); err != nil {
			log.Error().Err(err).Msg("could not store log entries to db")
		}
	}

	return err
}

func (s storage) LogDelete(step *model.Step) error {
	sess := s.engine.NewSession()
	defer sess.Close()
	return logDelete(sess, step.ID)
}

func logDelete(sess *xorm.Session, stepID int64) error {
	_, err := sess.Where("step_id = ?", stepID).Delete(new(model.LogEntry))
	return err
}
