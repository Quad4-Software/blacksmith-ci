// Copyright 2026 Crow Authors
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

package watch

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	nmocks "codefloe.com/crowci/crow/v6/server/services/notification/mocks"
	"codefloe.com/crowci/crow/v6/server/store"
)

// raceWait bounds how long the fake store waits for the interleaving it tries
// to provoke. With per-key locking the interleaving cannot happen, so the wait
// times out and the calls proceed one after the other.
const raceWait = 200 * time.Millisecond

// raceStore is an in-memory store holding the state of one key. It tries to
// make two PipelineFinished calls read the state before either writes, and
// makes the older run write last.
type raceStore struct {
	store.Store
	mu      sync.Mutex
	watches []*model.PipelineWatch
	states  []*model.WatchWorkflowState
	users   map[int64]*model.User

	readCount    int
	bothRead     chan struct{}
	newerWritten chan struct{}
	newerOnce    sync.Once
	// olderNumber enables the interleaving; it is the pipeline number of the older run.
	olderNumber int64
}

func newRaceStore(watches ...*model.PipelineWatch) *raceStore {
	return &raceStore{
		watches:      watches,
		users:        map[int64]*model.User{1: {ID: 1, Email: "a@x.io"}},
		bothRead:     make(chan struct{}),
		newerWritten: make(chan struct{}),
	}
}

func (f *raceStore) WatchListByKey(int64, model.WebhookEvent, string) ([]*model.PipelineWatch, error) {
	return f.watches, nil
}

func (f *raceStore) GetUser(id int64) (*model.User, error) {
	return f.users[id], nil
}

func (f *raceStore) WatchStateList(int64, model.WebhookEvent, string) ([]*model.WatchWorkflowState, error) {
	f.mu.Lock()
	out := make([]*model.WatchWorkflowState, 0, len(f.states))
	for _, s := range f.states {
		cp := *s
		out = append(out, &cp)
	}
	f.readCount++
	if f.readCount == 2 {
		close(f.bothRead)
	}
	f.mu.Unlock()
	if f.olderNumber > 0 {
		// wait for the other call to read as well if it can get that far
		select {
		case <-f.bothRead:
		case <-time.After(raceWait):
		}
	}
	return out, nil
}

func (f *raceStore) WatchStateReplace(_ int64, _ model.WebhookEvent, _ string, states []*model.WatchWorkflowState) error {
	if f.olderNumber > 0 {
		older := len(states) > 0 && states[0].PipelineNumber == f.olderNumber
		if older {
			// let the newer run write first if it can get that far
			select {
			case <-f.newerWritten:
			case <-time.After(raceWait):
			}
		}
		defer func() {
			if !older {
				f.newerOnce.Do(func() { close(f.newerWritten) })
			}
		}()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states = states
	return nil
}

func TestPipelineFinished_ConcurrentRunsKeepNewestState(t *testing.T) {
	st := newRaceStore(emailWatch(1, 1))
	st.olderNumber = 1
	svc := NewService(st, nil, "https://ci", "secret")

	older := finishedPipeline(1, model.StatusFailure, wf("a", model.StatusFailure))
	newer := finishedPipeline(2, model.StatusSuccess, wf("a", model.StatusSuccess))

	var wg sync.WaitGroup
	for _, p := range []*model.Pipeline{older, newer} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, svc.PipelineFinished(t.Context(), svcRepo, p))
		}()
	}
	wg.Wait()

	require.Len(t, st.states, 1)
	assert.Equal(t, int64(2), st.states[0].PipelineNumber, "the older run must not overwrite the newer state")
	assert.False(t, st.states[0].Failing)
}

func TestPipelineFinished_DuplicateSendsOneEmail(t *testing.T) {
	st := newRaceStore(emailWatch(1, 1, model.NotificationEventFailure))
	mailer := nmocks.NewMockService(t)
	p := finishedPipeline(3, model.StatusFailure, wf("a", model.StatusFailure))
	mailer.EXPECT().SendPipelineEmail(mock.Anything, "a@x.io", svcRepo, mock.Anything, "broken", mock.Anything).Return(nil).Once()
	svc := NewService(st, mailer, "https://ci", "secret")

	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cp := *p
			assert.NoError(t, svc.PipelineFinished(t.Context(), svcRepo, &cp))
		}()
	}
	wg.Wait()

	require.Len(t, st.states, 1)
	assert.True(t, st.states[0].Failing)
	assert.Equal(t, int64(3), st.states[0].PipelineNumber)
}
