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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	storeMocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func TestProcLogEndToEnd_IngestThenSubscribe(t *testing.T) {
	storeMock := storeMocks.NewMockStore(t)

	// Shared state across mock calls.
	var (
		mu     sync.Mutex
		stored []*model.ProcLogEntry
	)

	storeMock.On("ProcLogInsert", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			entries, ok := args.Get(1).([]*model.ProcLogEntry)
			require.True(t, ok)
			mu.Lock()
			defer mu.Unlock()
			for _, e := range entries {
				e.ID = int64(len(stored) + 1)
				stored = append(stored, e)
			}
		}).Return(nil)

	storeMock.On("ProcLogTailRecent", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(func(_ context.Context, st string, sid int64, limit int) []*model.ProcLogEntry {
			mu.Lock()
			defer mu.Unlock()
			out := make([]*model.ProcLogEntry, 0)
			for _, e := range stored {
				if e.SourceType == st && e.SourceID == sid {
					out = append(out, e)
				}
			}
			if len(out) > limit {
				out = out[len(out)-limit:]
			}
			return out
		}, nil)

	storeMock.On("ProcLogTail", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(func(_ context.Context, st string, sid int64, since int64, _ int) []*model.ProcLogEntry {
			mu.Lock()
			defer mu.Unlock()
			out := make([]*model.ProcLogEntry, 0)
			for _, e := range stored {
				if e.SourceType == st && e.SourceID == sid && e.ID > since {
					out = append(out, e)
				}
			}
			return out
		}, nil)

	recv := NewReceiver(storeMock)
	stream := NewStream(storeMock, StreamOpts{
		PollInterval:  10 * time.Millisecond,
		MaxFlushRows:  100,
		ChannelBuffer: 16,
	})

	// 1. Subscribe.
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	ch, err := stream.Subscribe(ctx, SourceTypeAgent, 42)
	require.NoError(t, err)

	// 2. Ingest two entries.
	require.NoError(t, recv.Ingest(ctx, SourceTypeAgent, 42, []*model.ProcLogEntry{
		{Time: 1, Level: "info", Line: "alpha"},
		{Time: 2, Level: "warn", Line: "beta"},
	}))

	// 3. Subscriber observes both lines within a few poll intervals.
	got := make([]string, 0, 2)
	timeout := time.After(2 * time.Second)
	for len(got) < 2 {
		select {
		case e := <-ch:
			got = append(got, e.Line)
		case <-timeout:
			t.Fatalf("only saw %d entries: %v", len(got), got)
		}
	}
	require.Equal(t, []string{"alpha", "beta"}, got)
}
