// Copyright 2023 Woodpecker Authors
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
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agent_rpc "codefloe.com/crowci/crow/v6/agent/rpc"
)

func TestBackendProbeTracker(t *testing.T) {
	probeErr := errors.New("ping failed")
	start := time.Unix(1_700_000_000, 0)

	t.Run("healthy probes never produce an error", func(t *testing.T) {
		tracker := backendProbeTracker{maxUnhealthy: time.Minute}
		assert.NoError(t, tracker.record(start, nil))
		assert.NoError(t, tracker.record(start.Add(10*time.Minute), nil))
	})

	t.Run("failures under the threshold return nil and keep the start time", func(t *testing.T) {
		tracker := backendProbeTracker{maxUnhealthy: time.Minute}
		assert.NoError(t, tracker.record(start, probeErr))
		assert.NoError(t, tracker.record(start.Add(30*time.Second), probeErr))
		assert.Equal(t, start, tracker.unhealthySince, "unhealthySince must be pinned to the first failure, not the latest")
	})

	t.Run("failure past the threshold returns ErrBackendUnavailable", func(t *testing.T) {
		tracker := backendProbeTracker{maxUnhealthy: time.Minute}
		assert.NoError(t, tracker.record(start, probeErr))
		err := tracker.record(start.Add(61*time.Second), probeErr)
		assert.ErrorIs(t, err, ErrBackendUnavailable)
	})

	t.Run("a recovery resets the streak so later failures get a fresh budget", func(t *testing.T) {
		tracker := backendProbeTracker{maxUnhealthy: time.Minute}
		assert.NoError(t, tracker.record(start, probeErr))
		assert.NoError(t, tracker.record(start.Add(30*time.Second), nil), "recovery must clear unhealthySince")
		assert.True(t, tracker.unhealthySince.IsZero())
		// New failure window starts here; threshold must not be measured from the original failure.
		assert.NoError(t, tracker.record(start.Add(45*time.Second), probeErr))
		assert.NoError(t, tracker.record(start.Add(90*time.Second), probeErr))
	})
}

func TestPlanRunners(t *testing.T) {
	tests := []struct {
		name         string
		adoptedCount int
		maxWorkflows int
		want         runnerPlan
	}{
		{
			// The 6/5 bug: capacity 5, one workflow reclaimed after a restart.
			// Long-lived polling runners must stay at 5 total — the adopted one
			// occupies a slot, leaving only 4 to pull new work.
			name:         "one adopted under capacity",
			adoptedCount: 1,
			maxWorkflows: 5,
			want:         runnerPlan{pollingAdopted: 1, pollingPlain: 4, drainAdopted: 0},
		},
		{
			name:         "no adopted workflows",
			adoptedCount: 0,
			maxWorkflows: 5,
			want:         runnerPlan{pollingAdopted: 0, pollingPlain: 5, drainAdopted: 0},
		},
		{
			name:         "adopted exactly at capacity",
			adoptedCount: 5,
			maxWorkflows: 5,
			want:         runnerPlan{pollingAdopted: 5, pollingPlain: 0, drainAdopted: 0},
		},
		{
			// Capacity was lowered while workflows were in-flight. Those already
			// running on this host can't be killed, so the overflow gets
			// drain-only runners that exit instead of polling for more.
			name:         "more adopted than capacity",
			adoptedCount: 7,
			maxWorkflows: 5,
			want:         runnerPlan{pollingAdopted: 5, pollingPlain: 0, drainAdopted: 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := planRunners(tt.adoptedCount, tt.maxWorkflows)
			assert.Equal(t, tt.want, got)
			// Long-lived polling runners must never exceed capacity.
			assert.LessOrEqual(t, got.pollingAdopted+got.pollingPlain, tt.maxWorkflows)
			// Every adopted workflow is handled exactly once.
			assert.Equal(t, tt.adoptedCount, got.pollingAdopted+got.drainAdopted)
		})
	}
}

func TestRunWithRecover(t *testing.T) {
	t.Run("returns nil for non-panicking function", func(t *testing.T) {
		fn := runWithRecover("test", func() error { return nil })
		assert.NoError(t, fn())
	})

	t.Run("returns error for non-panicking function that errors", func(t *testing.T) {
		want := errors.New("boom")
		fn := runWithRecover("test", func() error { return want })
		assert.ErrorIs(t, fn(), want)
	})

	t.Run("recovers from panic and returns it as error", func(t *testing.T) {
		fn := runWithRecover("test", func() error {
			panic("oh no") //nolint:forbidigo // testing recover()
		})
		err := fn()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "panic in test")
		assert.Contains(t, err.Error(), "oh no")
	})

	t.Run("recovers from panic with non-string value", func(t *testing.T) {
		fn := runWithRecover("test", func() error {
			panic(errors.New("typed panic")) //nolint:forbidigo // testing recover()
		})
		err := fn()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "panic in test")
		assert.Contains(t, err.Error(), "typed panic")
	})

	t.Run("recovers from runtime panic (out-of-range index)", func(t *testing.T) {
		fn := runWithRecover("test", func() error {
			s := []int{}
			i := len(s) // computed at runtime so nilness can't prove the panic
			_ = s[i]
			return nil
		})
		err := fn()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "panic in test")
	})

	// A recovered panic must take the whole agent down: the goroutine is gone and
	// the workflow-runner loop would otherwise treat the error as transient,
	// leaving an agent that reports healthy but never runs a pipeline again.
	t.Run("triggers agent shutdown on panic", func(t *testing.T) {
		prev := stopAgentFunc
		t.Cleanup(func() { stopAgentFunc = prev })

		var stopped error
		var calls int
		stopAgentFunc = func(err error) {
			calls++
			stopped = err
		}

		err := runWithRecover("workflow-runner-0", func() error {
			panic("oh no") //nolint:forbidigo // testing recover()
		})()

		assert.Error(t, err)
		assert.Equal(t, 1, calls, "panic must trigger exactly one agent shutdown")
		assert.Equal(t, err, stopped, "agent shutdown must be given the panic error")
	})

	t.Run("does not trigger agent shutdown for a plain error", func(t *testing.T) {
		prev := stopAgentFunc
		t.Cleanup(func() { stopAgentFunc = prev })

		var calls int
		stopAgentFunc = func(error) { calls++ }

		assert.Error(t, runWithRecover("test", func() error { return errors.New("boom") })())
		assert.Zero(t, calls, "transient errors must not shut the agent down")
	})
}

func TestStringSliceAddToMap(t *testing.T) {
	tests := []struct {
		name     string
		sl       []string
		m        map[string]string
		expected map[string]string
		err      bool
	}{
		{
			name: "add values to map",
			sl:   []string{"foo=bar", "baz=qux=nux"},
			m:    make(map[string]string),
			expected: map[string]string{
				"foo": "bar",
				"baz": "qux=nux",
			},
			err: false,
		},
		{
			name:     "empty slice",
			sl:       []string{},
			m:        make(map[string]string),
			expected: map[string]string{},
			err:      false,
		},
		{
			name:     "missing value",
			sl:       []string{"foo", "baz=qux"},
			m:        make(map[string]string),
			expected: map[string]string{},
			err:      true,
		},
		{
			name:     "empty string in slice",
			sl:       []string{"foo=bar", "", "baz=qux"},
			m:        make(map[string]string),
			expected: map[string]string{"foo": "bar", "baz": "qux"},
			err:      false,
		},
		{
			name: "multiple values for same key in different entries",
			sl:   []string{"org=value1", "org=value2"},
			m:    make(map[string]string),
			expected: map[string]string{
				"org": "value1|value2",
			},
			err: false,
		},
		{
			name: "multiple values for same key in same entry",
			sl:   []string{"org=value1,org=value2"},
			m:    make(map[string]string),
			expected: map[string]string{
				"org": "value1|value2",
			},
			err: false,
		},
		{
			name: "multiple key-value pairs in one entry",
			sl:   []string{"org=value1,repo=value2"},
			m:    make(map[string]string),
			expected: map[string]string{
				"org":  "value1",
				"repo": "value2",
			},
			err: false,
		},
		{
			name: "complex example with multiple keys and values",
			sl:   []string{"org=value1,org=value2,repo=test"},
			m:    make(map[string]string),
			expected: map[string]string{
				"org":  "value1|value2",
				"repo": "test",
			},
			err: false,
		},
		{
			name: "value containing equals sign",
			sl:   []string{"key=value=with=equals"},
			m:    make(map[string]string),
			expected: map[string]string{
				"key": "value=with=equals",
			},
			err: false,
		},
		{
			name: "appending to existing map",
			sl:   []string{"new=value"},
			m: map[string]string{
				"existing": "data",
			},
			expected: map[string]string{
				"existing": "data",
				"new":      "value",
			},
			err: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := stringSliceAddToMap(tt.sl, tt.m)

			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.EqualValues(t, tt.expected, tt.m)
			}
		})
	}
}

func TestValidateGRPCConfig(t *testing.T) {
	t.Run("CROW_GRPC_ADDR set returns error", func(t *testing.T) {
		t.Setenv("CROW_GRPC_ADDR", "grpc.example.com:443")
		t.Setenv("WOODPECKER_GRPC_ADDR", "")

		err := validateGRPCConfig("server:9000", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CROW_GRPC_ADDR is a server/autoscaler-only variable")
		assert.Contains(t, err.Error(), "CROW_SERVER")
	})

	t.Run("WOODPECKER_GRPC_ADDR set returns error naming WOODPECKER_GRPC_ADDR", func(t *testing.T) {
		t.Setenv("CROW_GRPC_ADDR", "")
		t.Setenv("WOODPECKER_GRPC_ADDR", "grpc.example.com:443")

		err := validateGRPCConfig("server:9000", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "WOODPECKER_GRPC_ADDR is a server/autoscaler-only variable")
	})

	t.Run("port 443 without grpc-secure logs warning", func(t *testing.T) {
		t.Setenv("CROW_GRPC_ADDR", "")
		t.Setenv("WOODPECKER_GRPC_ADDR", "")

		var buf bytes.Buffer
		originalLogger := log.Logger
		log.Logger = zerolog.New(&buf)
		t.Cleanup(func() { log.Logger = originalLogger })

		err := validateGRPCConfig("grpc.example.com:443", false)
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "TLS-typical port")
		assert.Contains(t, buf.String(), "CROW_GRPC_SECURE")
	})

	t.Run("port 8443 without grpc-secure logs warning", func(t *testing.T) {
		t.Setenv("CROW_GRPC_ADDR", "")
		t.Setenv("WOODPECKER_GRPC_ADDR", "")

		var buf bytes.Buffer
		originalLogger := log.Logger
		log.Logger = zerolog.New(&buf)
		t.Cleanup(func() { log.Logger = originalLogger })

		err := validateGRPCConfig("grpc.example.com:8443", false)
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "TLS-typical port")
	})

	t.Run("port 443 with grpc-secure does not warn", func(t *testing.T) {
		t.Setenv("CROW_GRPC_ADDR", "")
		t.Setenv("WOODPECKER_GRPC_ADDR", "")

		var buf bytes.Buffer
		originalLogger := log.Logger
		log.Logger = zerolog.New(&buf)
		t.Cleanup(func() { log.Logger = originalLogger })

		err := validateGRPCConfig("grpc.example.com:443", true)
		require.NoError(t, err)
		assert.Empty(t, buf.String())
	})

	t.Run("port 9000 without grpc-secure does not warn", func(t *testing.T) {
		t.Setenv("CROW_GRPC_ADDR", "")
		t.Setenv("WOODPECKER_GRPC_ADDR", "")

		var buf bytes.Buffer
		originalLogger := log.Logger
		log.Logger = zerolog.New(&buf)
		t.Cleanup(func() { log.Logger = originalLogger })

		err := validateGRPCConfig("server:9000", false)
		require.NoError(t, err)
		assert.Empty(t, buf.String())
	})
}

// fakeWorkflowRunner stands in for agent.Runner so the polling loop can be
// driven without a gRPC client or a backend engine. Each Run consults the
// behave callback for whether the poll picked up a workflow, and for the error
// it should return.
type fakeWorkflowRunner struct {
	runs   int
	behave func(run int) (ranWorkflow bool, err error)
}

func (f *fakeWorkflowRunner) Run(_, _ context.Context) (bool, error) {
	f.runs++
	return f.behave(f.runs)
}

func TestPollWorkflowsStopsAfterLifetimeBudget(t *testing.T) {
	counter.SetRetiring(false)
	t.Cleanup(func() { counter.SetRetiring(false) })

	lifetime := newWorkflowLifetime(2)
	runner := &fakeWorkflowRunner{behave: func(int) (bool, error) { return true, nil }}

	require.NoError(t, pollWorkflows(t.Context(), t.Context(), 0, runner, lifetime))
	assert.Equal(t, 2, runner.runs, "runner must poll exactly as often as the budget allows")
	assert.Equal(t, 2, lifetime.Spent())
	assert.True(t, lifetime.Exhausted())
}

func TestPollWorkflowsIdlePollsKeepTheBudget(t *testing.T) {
	counter.SetRetiring(false)
	t.Cleanup(func() { counter.SetRetiring(false) })

	lifetime := newWorkflowLifetime(1)
	// The queue is empty for the first three polls; only the fourth hands out
	// a workflow. A single-use agent must still be around to take it.
	runner := &fakeWorkflowRunner{behave: func(run int) (bool, error) {
		return run == 4, nil
	}}

	require.NoError(t, pollWorkflows(t.Context(), t.Context(), 0, runner, lifetime))
	assert.Equal(t, 4, runner.runs)
	assert.Equal(t, 1, lifetime.Spent())
}

func TestPollWorkflowsRetiringIsLatchedForHealthChecks(t *testing.T) {
	counter.SetRetiring(false)
	t.Cleanup(func() { counter.SetRetiring(false) })
	counter.SetHealthy(true)
	counter.RunnerStarted()

	lifetime := newWorkflowLifetime(1)
	runner := &fakeWorkflowRunner{behave: func(int) (bool, error) { return true, nil }}
	require.NoError(t, pollWorkflows(t.Context(), t.Context(), 0, runner, lifetime))

	// The last runner of a retiring agent is about to exit; an empty runner
	// pool is expected here and must not be reported as a zombie agent.
	counter.RunnerStopped()
	assert.True(t, counter.Healthy(), "a retiring agent must stay healthy while it exits")
}

func TestPollWorkflowsWithoutLifetimeLimitRunsUntilShutdown(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)

	lifetime := newWorkflowLifetime(0)
	runner := &fakeWorkflowRunner{behave: func(run int) (bool, error) {
		if run == 25 {
			cancel(nil)
		}
		return true, nil
	}}

	require.NoError(t, pollWorkflows(ctx, t.Context(), 0, runner, lifetime))
	assert.Equal(t, 25, runner.runs, "an unlimited agent polls until it is shut down")
	assert.Zero(t, lifetime.Spent())
}

func TestPollWorkflowsReturnsFatalRunnerError(t *testing.T) {
	lifetime := newWorkflowLifetime(0)
	runner := &fakeWorkflowRunner{behave: func(int) (bool, error) {
		return false, fmt.Errorf("connection lost: %w", agent_rpc.ErrAgentRestartRequired)
	}}

	err := pollWorkflows(t.Context(), t.Context(), 0, runner, lifetime)
	require.ErrorIs(t, err, agent_rpc.ErrAgentRestartRequired)
	assert.Equal(t, 1, runner.runs)
}

// Runners now block on a slot held by a sibling instead of exiting, so the
// whole pool has to come back cleanly: it must share the budget, run exactly
// what it is allowed to, and every runner must return rather than sit waiting
// on a budget that will never be released.
func TestPollWorkflowsSharesTheBudgetBetweenRunners(t *testing.T) {
	counter.SetRetiring(false)
	t.Cleanup(func() { counter.SetRetiring(false) })

	const (
		poolSize = 2
		budget   = 4
	)
	lifetime := newWorkflowLifetime(budget)

	// Runners take turns: each poll blocks until the other runner has had one,
	// which is the interleaving that used to make the loser exit for good.
	var (
		mu      sync.Mutex
		running int
		peak    int
	)
	newRunner := func() *fakeWorkflowRunner {
		return &fakeWorkflowRunner{behave: func(int) (bool, error) {
			mu.Lock()
			running++
			peak = max(peak, running)
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			running--
			mu.Unlock()
			return true, nil
		}}
	}

	runners := make([]*fakeWorkflowRunner, poolSize)
	var wg sync.WaitGroup
	for i := range runners {
		runners[i] = newRunner()
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, pollWorkflows(t.Context(), t.Context(), i, runners[i], lifetime))
		}()
	}
	wg.Wait()

	total := 0
	for _, runner := range runners {
		total += runner.runs
		assert.NotZero(t, runner.runs, "every runner must get work while the budget lasts")
	}
	assert.Equal(t, budget, total, "the pool must run exactly the budgeted workflows")
	assert.Equal(t, poolSize, peak, "the pool must keep its width until the budget is gone")
}

func TestShouldRetire(t *testing.T) {
	canceled, cancel := context.WithCancelCause(t.Context())
	cancel(nil)

	spent := newWorkflowLifetime(1)
	spent.Charge()

	tests := []struct {
		name     string
		ctx      context.Context
		lifetime *workflowLifetime
		want     bool
	}{
		{
			name:     "budget used up",
			ctx:      t.Context(),
			lifetime: spent,
			want:     true,
		},
		{
			name: "runner gone with budget left, a panic rather than a retirement",
			ctx:  t.Context(),
			// The runner that would have spent this slot died instead.
			lifetime: newWorkflowLifetime(1),
			want:     false,
		},
		{
			name:     "already shutting down for another reason",
			ctx:      canceled,
			lifetime: spent,
			want:     false,
		},
		{
			name:     "no lifetime limit configured",
			ctx:      t.Context(),
			lifetime: newWorkflowLifetime(0),
			want:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, shouldRetire(test.ctx, test.lifetime))
		})
	}
}
