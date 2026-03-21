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

package model

import "github.com/uptrace/bun"

// LogEntryType identifies the type of line in the logs.
type LogEntryType int //	@name	LogEntryType

const (
	LogEntryStdout LogEntryType = iota
	LogEntryStderr
	LogEntryExitCode
	LogEntryMetadata
	LogEntryProgress
)

type LogEntry struct {
	bun.BaseModel `bun:"table:log_entries"`

	ID      int64        `json:"id"       bun:"id,pk,autoincrement"`
	StepID  int64        `json:"step_id"  bun:"step_id"`
	Time    int64        `json:"time"     bun:"time"`
	Line    int          `json:"line"     bun:"line"`
	Data    []byte       `json:"data"     bun:"data,type:bytea"`
	Created int64        `json:"-"        bun:",nullzero"`
	Type    LogEntryType `json:"type"     bun:"type"`
} //	@name	LogEntry

// TODO: store info what specific command the line belongs to (must be optional and impl. by backend)

func (LogEntry) TableName() string {
	return "log_entries"
}
