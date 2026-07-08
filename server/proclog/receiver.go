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

package proclog

import (
	"context"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
)

// Receiver ingests batches of process log entries into the store.
// Caller (gRPC handler / HTTP endpoint) is responsible for authenticating the source.
type Receiver struct {
	store store.Store
}

func NewReceiver(s store.Store) *Receiver {
	return &Receiver{store: s}
}

// Ingest stamps every entry with the given source and persists in one batch insert.
// Empty input is a no-op (no store call).
func (r *Receiver) Ingest(
	ctx context.Context, sourceType string, sourceID int64, entries []*model.ProcLogEntry,
) error {
	if len(entries) == 0 {
		return nil
	}
	for _, e := range entries {
		e.SourceType = sourceType
		e.SourceID = sourceID
	}
	return r.store.ProcLogInsert(ctx, entries)
}
