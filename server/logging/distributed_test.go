// Copyright 2023 Woodpecker Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logging

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

// fakeLogStore serves LogFindAfter from a growing list, like the logs table.
type fakeLogStore struct {
	mu   sync.Mutex
	logs []*model.LogEntry
}

func (f *fakeLogStore) add(line int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, &model.LogEntry{ID: int64(len(f.logs) + 1), StepID: 1, Line: line})
}

func (f *fakeLogStore) after(id int64) []*model.LogEntry {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*model.LogEntry
	for _, entry := range f.logs {
		if entry.ID > id {
			out = append(out, entry)
		}
	}
	return out
}

func newDistributedForTest(t *testing.T, logs *fakeLogStore) *distributedLog {
	t.Helper()
	store := store_mocks.NewMockStore(t)
	store.On("LogFind", mock.Anything).Return(func(*model.Step) []*model.LogEntry { return logs.after(0) }, nil).Maybe()
	store.On("LogFindAfter", int64(1), mock.Anything).Return(func(_, id int64) []*model.LogEntry { return logs.after(id) }, nil).Maybe()
	d, ok := NewDistributed(nil, store, "test").(*distributedLog)
	require.True(t, ok)
	return d
}

type tailResult struct {
	lines     []int
	completed bool
	err       error
}

// tail runs Tail until it returns and collects the log lines it delivered.
func tail(ctx context.Context, d *distributedLog, started *sync.WaitGroup) <-chan tailResult {
	out := make(chan tailResult, 1)
	go func() {
		handler := make(LogChan, 100)
		var res tailResult
		done := make(chan struct{})
		go func() {
			defer close(done)
			for batch := range handler {
				for _, entry := range batch {
					if entry.ID == -1 {
						res.completed = res.completed || strings.Contains(string(entry.Data), "workflow_completed")
						continue
					}
					res.lines = append(res.lines, entry.Line)
				}
			}
		}()
		started.Done()
		res.err = d.Tail(ctx, 1, handler)
		close(handler)
		<-done
		out <- res
	}()
	return out
}

func sessionCount(d *distributedLog) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.sessions[1])
}

// A second client watching the same step must not cancel the first one, and
// Close must deliver the logs written before it to every client.
func TestDistributedLogConcurrentViewers(t *testing.T) {
	logs := &fakeLogStore{}
	logs.add(1)
	d := newDistributedForTest(t, logs)

	var started sync.WaitGroup
	started.Add(2)
	first := tail(t.Context(), d, &started)
	second := tail(t.Context(), d, &started)
	started.Wait()
	require.Eventually(t, func() bool { return sessionCount(d) == 2 }, 2*time.Second, 10*time.Millisecond)

	logs.add(2)
	require.NoError(t, d.Write(t.Context(), 1, []*model.LogEntry{{StepID: 1, Line: 2}}))
	logs.add(3)

	closeStart := time.Now()
	require.NoError(t, d.Close(t.Context(), 1))
	assert.Less(t, time.Since(closeStart), 50*time.Millisecond, "Close must not block on the sessions")

	for _, ch := range []<-chan tailResult{first, second} {
		select {
		case res := <-ch:
			require.NoError(t, res.err)
			assert.Equal(t, []int{1, 2, 3}, res.lines)
			assert.True(t, res.completed, "missing completion reload signal")
		case <-time.After(5 * time.Second):
			t.Fatal("Tail did not return after Close")
		}
	}
	assert.Zero(t, sessionCount(d))
}

// A client leaving must only end its own session.
func TestDistributedLogViewerLeaves(t *testing.T) {
	logs := &fakeLogStore{}
	d := newDistributedForTest(t, logs)

	leaving, leave := context.WithCancelCause(t.Context())
	var started sync.WaitGroup
	started.Add(2)
	gone := tail(leaving, d, &started)
	staying := tail(t.Context(), d, &started)
	started.Wait()
	require.Eventually(t, func() bool { return sessionCount(d) == 2 }, 2*time.Second, 10*time.Millisecond)

	leave(nil)
	select {
	case res := <-gone:
		assert.ErrorIs(t, res.err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("Tail did not return after its client left")
	}
	assert.Equal(t, 1, sessionCount(d))

	logs.add(7)
	require.NoError(t, d.Close(t.Context(), 1))
	res := <-staying
	assert.Equal(t, []int{7}, res.lines)
}
