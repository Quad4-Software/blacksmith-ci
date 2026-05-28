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

const (
	ProcLogSourceTypeAgent      = "agent"
	ProcLogSourceTypeAutoscaler = "autoscaler"
)

// ProcLogEntry is one process-log line shipped by an agent or autoscaler.
// Indexed by (source_type, source_id, id) for tail queries.
type ProcLogEntry struct {
	bun.BaseModel `bun:"table:proc_log_entries"`

	ID         int64  `json:"id"          bun:"id,pk,autoincrement"`
	SourceType string `json:"source_type" bun:"source_type,notnull"`
	SourceID   int64  `json:"source_id"   bun:"source_id,notnull"`
	// Time is Unix nanoseconds; set by the producer.
	Time  int64  `json:"time"        bun:"time,notnull"`
	Level string `json:"level"       bun:"level,notnull"`
	Line  string `json:"line"        bun:"line,notnull"`
}

func (ProcLogEntry) TableName() string {
	return "proc_log_entries"
}

// ProcLogSourceKey identifies a unique log source.
// The bun column tags allow scanning DISTINCT source_type, source_id results
// from ProcLogListSources directly into this struct.
type ProcLogSourceKey struct {
	SourceType string `bun:"source_type"`
	SourceID   int64  `bun:"source_id"`
}
