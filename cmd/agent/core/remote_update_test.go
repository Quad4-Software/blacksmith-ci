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

package core

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// fakeIdle reports busy until it has been asked busyChecks times.
type fakeIdle struct {
	busyChecks int32
	checks     atomic.Int32
}

func (f *fakeIdle) Idle() bool {
	return f.checks.Add(1) > f.busyChecks
}

func TestValidateRemoteUpdate(t *testing.T) {
	for _, tt := range []struct {
		in, want string
		wantErr  bool
	}{
		{in: "", want: ""},
		{in: "off", want: ""},
		{in: "restart", want: "restart"},
		{in: "self", wantErr: true},
	} {
		got, err := validateRemoteUpdate(tt.in)
		if tt.wantErr {
			assert.Error(t, err, tt.in)
			continue
		}
		assert.NoError(t, err, tt.in)
		assert.Equal(t, tt.want, got, tt.in)
	}
}

func TestRemoteRestartRequest(t *testing.T) {
	t.Run("ignores requests when off", func(t *testing.T) {
		started := 0
		r := &remoteRestart{state: &fakeIdle{}, start: func(func() error) { started++ }}
		r.request(t.Context())
		r.request(t.Context())
		assert.Zero(t, started)
	})

	t.Run("starts a single drain in restart mode", func(t *testing.T) {
		started := 0
		r := &remoteRestart{mode: "restart", state: &fakeIdle{}, start: func(func() error) { started++ }}
		r.request(t.Context())
		r.request(t.Context())
		assert.Equal(t, 1, started)
	})
}

func TestDrainForRestart(t *testing.T) {
	prevInterval := remoteRestartPollInterval
	prevStop := stopAgentFunc
	t.Cleanup(func() {
		remoteRestartPollInterval = prevInterval
		stopAgentFunc = prevStop
	})
	remoteRestartPollInterval = time.Millisecond

	t.Run("waits for running workflows, then stops the agent", func(t *testing.T) {
		var stoppedWith error
		stopAgentFunc = func(err error) { stoppedWith = err }
		state := &fakeIdle{busyChecks: 3}

		err := drainForRestart(t.Context(), state)

		assert.ErrorIs(t, err, ErrRemoteRestart)
		assert.ErrorIs(t, stoppedWith, ErrRemoteRestart)
		assert.EqualValues(t, 4, state.checks.Load())
	})

	t.Run("gives up quietly when the agent shuts down first", func(t *testing.T) {
		stopped := false
		stopAgentFunc = func(error) { stopped = true }
		ctx, cancel := context.WithCancelCause(t.Context())
		cancel(nil)

		err := drainForRestart(ctx, &fakeIdle{busyChecks: 1 << 30})

		assert.NoError(t, err)
		assert.False(t, stopped)
	})
}

// An agent that allows remote restarts exits with ErrRemoteRestart once the
// server asks for one, so its supervisor starts it again.
func TestAgentExitsOnRemoteRestart(t *testing.T) {
	addr := startFakeServer(t, &fakeCrowServer{restartRequested: true})

	err := runTestAgent(t, addr, "--agent-config", "", "--remote-update", "restart")

	assert.ErrorIs(t, err, ErrRemoteRestart)
}

// Without opting in, the agent keeps running through restart requests.
func TestAgentIgnoresRemoteRestartWhenOff(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)

	var once sync.Once
	srv := &fakeCrowServer{restartRequested: true}
	srv.onHealth = func() {
		// Give the agent time to act on the first report before stopping it.
		once.Do(func() { time.AfterFunc(500*time.Millisecond, func() { cancel(nil) }) })
	}
	addr := startFakeServer(t, srv)

	err := runTestAgentWithContext(ctx, t, addr, "--agent-config", "")

	assert.NoError(t, err)
}
