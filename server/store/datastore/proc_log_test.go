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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
)

func TestProcLogInsertAndTail(t *testing.T) {
	store, closer := newTestStore(t, new(model.ProcLogEntry))
	defer closer()
	ctx := context.Background()

	entries := []*model.ProcLogEntry{
		{SourceType: model.ProcLogSourceTypeAgent, SourceID: 1, Time: 100, Level: "info", Line: "a"},
		{SourceType: model.ProcLogSourceTypeAgent, SourceID: 1, Time: 200, Level: "info", Line: "b"},
		{SourceType: model.ProcLogSourceTypeAgent, SourceID: 2, Time: 150, Level: "info", Line: "other"},
	}
	require.NoError(t, store.ProcLogInsert(ctx, entries))

	got, err := store.ProcLogTail(ctx, model.ProcLogSourceTypeAgent, 1, 0, 100)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].Line)
	assert.Equal(t, "b", got[1].Line)

	got2, err := store.ProcLogTail(ctx, model.ProcLogSourceTypeAgent, 1, got[0].ID, 100)
	require.NoError(t, err)
	require.Len(t, got2, 1)
	assert.Equal(t, "b", got2[0].Line)
}

func TestProcLogTailRecent(t *testing.T) {
	store, closer := newTestStore(t, new(model.ProcLogEntry))
	defer closer()
	ctx := context.Background()

	entries := make([]*model.ProcLogEntry, 0, 10)
	for i := range 10 {
		entries = append(entries, &model.ProcLogEntry{
			SourceType: model.ProcLogSourceTypeAgent, SourceID: 1,
			Time: int64(i), Level: "info", Line: fmt.Sprintf("line-%d", i),
		})
	}
	entries = append(entries, &model.ProcLogEntry{
		SourceType: model.ProcLogSourceTypeAgent, SourceID: 2, Time: 0, Level: "info", Line: "other",
	})
	require.NoError(t, store.ProcLogInsert(ctx, entries))

	got, err := store.ProcLogTailRecent(ctx, model.ProcLogSourceTypeAgent, 1, 3)
	require.NoError(t, err)
	require.Len(t, got, 3)
	// newest rows, but handed back oldest-first so the SSE replay stays in order
	assert.Equal(t, "line-7", got[0].Line)
	assert.Equal(t, "line-8", got[1].Line)
	assert.Equal(t, "line-9", got[2].Line)
}

func TestProcLogTrim(t *testing.T) {
	store, closer := newTestStore(t, new(model.ProcLogEntry))
	defer closer()
	ctx := context.Background()

	entries := make([]*model.ProcLogEntry, 0, 10)
	for i := 0; i < 10; i++ {
		entries = append(entries, &model.ProcLogEntry{
			SourceType: model.ProcLogSourceTypeAgent, SourceID: 1,
			Time: int64(i), Level: "info", Line: "x",
		})
	}
	require.NoError(t, store.ProcLogInsert(ctx, entries))

	require.NoError(t, store.ProcLogTrim(ctx, model.ProcLogSourceTypeAgent, 1, 3))

	got, err := store.ProcLogTail(ctx, model.ProcLogSourceTypeAgent, 1, 0, 100)
	require.NoError(t, err)
	assert.Len(t, got, 3, "should keep only the most recent 3 entries")
}

func TestProcLogListSources(t *testing.T) {
	store, closer := newTestStore(t, new(model.ProcLogEntry))
	defer closer()
	ctx := context.Background()

	require.NoError(t, store.ProcLogInsert(ctx, []*model.ProcLogEntry{
		{SourceType: model.ProcLogSourceTypeAgent, SourceID: 1, Time: 1, Level: "info", Line: "x"},
		{SourceType: model.ProcLogSourceTypeAgent, SourceID: 2, Time: 2, Level: "info", Line: "x"},
		{SourceType: model.ProcLogSourceTypeAutoscaler, SourceID: 7, Time: 3, Level: "info", Line: "x"},
	}))

	keys, err := store.ProcLogListSources(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []model.ProcLogSourceKey{
		{SourceType: model.ProcLogSourceTypeAgent, SourceID: 1},
		{SourceType: model.ProcLogSourceTypeAgent, SourceID: 2},
		{SourceType: model.ProcLogSourceTypeAutoscaler, SourceID: 7},
	}, keys)
}
