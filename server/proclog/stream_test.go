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
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	storeMocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func TestSubscribeInitialFlushThenLive(t *testing.T) {
	storeMock := storeMocks.NewMockStore(t)

	// Initial flush: return two rows.
	storeMock.On("ProcLogTailRecent", mock.Anything, SourceTypeAgent, int64(1), mock.Anything).
		Return([]*model.ProcLogEntry{
			{ID: 5, Time: 100, Level: "info", Line: "history-a"},
			{ID: 6, Time: 200, Level: "info", Line: "history-b"},
		}, nil).Once()

	// Live polls past sinceID=6: first poll returns one new row.
	storeMock.On("ProcLogTail", mock.Anything, SourceTypeAgent, int64(1), int64(6), mock.Anything).
		Return([]*model.ProcLogEntry{
			{ID: 7, Time: 300, Level: "info", Line: "live-a"},
		}, nil).Once()

	// All subsequent polls return nothing.
	storeMock.On("ProcLogTail", mock.Anything, SourceTypeAgent, int64(1), int64(7), mock.Anything).
		Return([]*model.ProcLogEntry{}, nil).Maybe()

	s := NewStream(storeMock, StreamOpts{
		PollInterval:  10 * time.Millisecond,
		MaxFlushRows:  100,
		ChannelBuffer: 16,
	})

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	ch, err := s.Subscribe(ctx, SourceTypeAgent, 1)
	require.NoError(t, err)

	var got []string
	timeout := time.After(2 * time.Second)
	for len(got) < 3 {
		select {
		case e := <-ch:
			got = append(got, e.Line)
		case <-timeout:
			t.Fatalf("only got %d entries: %v", len(got), got)
		}
	}
	require.Equal(t, []string{"history-a", "history-b", "live-a"}, got)
}

func TestSubscribeInitialFlushExceedsBuffer(t *testing.T) {
	storeMock := storeMocks.NewMockStore(t)

	const flushed = 50
	historical := make([]*model.ProcLogEntry, 0, flushed)
	for i := range flushed {
		historical = append(historical, &model.ProcLogEntry{
			ID: int64(i + 1), Time: int64(i), Level: "info", Line: "x",
		})
	}

	storeMock.On("ProcLogTailRecent", mock.Anything, SourceTypeAgent, int64(1), mock.Anything).
		Return(historical, nil).Once()
	storeMock.On("ProcLogTail", mock.Anything, SourceTypeAgent, int64(1), int64(flushed), mock.Anything).
		Return([]*model.ProcLogEntry{}, nil).Maybe()

	s := NewStream(storeMock, StreamOpts{
		PollInterval:  10 * time.Millisecond,
		MaxFlushRows:  flushed,
		ChannelBuffer: 4, // intentionally smaller than the flush count
	})

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	ch, err := s.Subscribe(ctx, SourceTypeAgent, 1)
	require.NoError(t, err)

	var got int
	timeout := time.After(2 * time.Second)
	for got < flushed {
		select {
		case <-ch:
			got++
		case <-timeout:
			t.Fatalf("only received %d of %d entries", got, flushed)
		}
	}
}

func TestSubscribeCancelClosesChannel(t *testing.T) {
	storeMock := storeMocks.NewMockStore(t)
	storeMock.On("ProcLogTailRecent", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]*model.ProcLogEntry{}, nil).Maybe()
	storeMock.On("ProcLogTail", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]*model.ProcLogEntry{}, nil).Maybe()

	s := NewStream(storeMock, StreamOpts{
		PollInterval:  10 * time.Millisecond,
		MaxFlushRows:  100,
		ChannelBuffer: 16,
	})

	ctx, cancel := context.WithCancelCause(context.Background())
	ch, err := s.Subscribe(ctx, SourceTypeAgent, 1)
	require.NoError(t, err)

	cancel(nil)
	select {
	case _, ok := <-ch:
		if !ok {
			return // expected: channel closed.
		}
		// May receive a tail entry first, but eventually channel closes.
		for range ch {
		}
	case <-time.After(time.Second):
		t.Fatal("channel did not close after context cancel")
	}
}
