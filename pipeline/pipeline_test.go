// Copyright 2026 Crow Authors
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

package pipeline

import (
	"context"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"

	backend "codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// cancelBackend simulates a backend whose step keeps running until the workflow
// environment is torn down — the situation an agent hits when a pipeline is
// canceled while a step is in flight. WaitStep blocks until DestroyWorkflow
// releases it and then lingers for a moment, which reliably places the step
// goroutine's trailing tracer call after the point where Run would return if it
// did not join its step goroutines.
type cancelBackend struct {
	// waitEntered is closed once WaitStep is running, so the test knows when it
	// is safe to cancel the workflow context.
	waitEntered chan struct{}
	// release is closed by DestroyWorkflow and unblocks WaitStep.
	release chan struct{}
	// releaseDelay is how long WaitStep lingers after being released.
	releaseDelay time.Duration

	enteredOnce sync.Once
	releaseOnce sync.Once
}

func newCancelBackend() *cancelBackend {
	return &cancelBackend{
		waitEntered:  make(chan struct{}),
		release:      make(chan struct{}),
		releaseDelay: 50 * time.Millisecond,
	}
}

func (b *cancelBackend) Name() string                     { return "cancel-test" }
func (b *cancelBackend) IsAvailable(context.Context) bool { return true }
func (b *cancelBackend) Flags() []cli.Flag                { return nil }

func (b *cancelBackend) Load(context.Context) (*backend.BackendInfo, error) {
	return &backend.BackendInfo{Platform: "test"}, nil
}

func (b *cancelBackend) SetupWorkflow(context.Context, *backend.Config, string) error { return nil }

func (b *cancelBackend) StartStep(context.Context, *backend.Step, string) error { return nil }

// ResumeStep makes cancelBackend satisfy backend.WorkflowResumer so it can be
// used with RunAdopted.
func (b *cancelBackend) ResumeStep(context.Context, *backend.Step, string) error { return nil }

func (b *cancelBackend) WaitStep(context.Context, *backend.Step, string) (*backend.State, error) {
	b.enteredOnce.Do(func() { close(b.waitEntered) })
	<-b.release
	// Linger so that, without the join in Run's teardown, the step goroutine
	// traces its result only after Run has already returned to its caller.
	time.Sleep(b.releaseDelay)
	return &backend.State{Exited: true}, nil
}

func (b *cancelBackend) TailStep(context.Context, *backend.Step, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("some log line\n")), nil
}

func (b *cancelBackend) DestroyStep(context.Context, *backend.Step, string) error { return nil }

func (b *cancelBackend) DestroyWorkflow(context.Context, *backend.Config, string) error {
	b.releaseOnce.Do(func() { close(b.release) })
	return nil
}

func singleStepConfig() *backend.Config {
	return &backend.Config{
		Stages: []*backend.Stage{{
			Steps: []*backend.Step{{
				Name:        "step",
				UUID:        "step-uuid",
				Type:        backend.StepTypeCommands,
				Environment: map[string]string{},
				OnSuccess:   true,
			}},
		}},
	}
}

// callbackRecorder counts tracer/logger invocations and remembers how many had
// happened by the time Run returned.
type callbackRecorder struct {
	mu           sync.Mutex
	total        int
	afterRun     int
	runReturned  bool
	totalAtSplit int
}

func (c *callbackRecorder) record() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.total++
	if c.runReturned {
		c.afterRun++
	}
}

func (c *callbackRecorder) runDone() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runReturned = true
	c.totalAtSplit = c.total
}

func (c *callbackRecorder) snapshot() (total, afterRun, atSplit int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.total, c.afterRun, c.totalAtSplit
}

// newRecordingRuntime builds a runtime wired to eng whose tracer and logger
// mimic the agent's upload bookkeeping: a WaitGroup owned by the caller of Run,
// with Add/Done inside the callbacks.
func newRecordingRuntime(ctx context.Context, eng backend.Backend, rec *callbackRecorder, uploads *sync.WaitGroup) *Runtime {
	return New(singleStepConfig(),
		WithContext(ctx),
		WithBackend(eng),
		WithTracer(TraceFunc(func(*State) error {
			uploads.Add(1)
			defer uploads.Done()
			rec.record()
			return nil
		})),
		WithLogger(Logger(func(_ *backend.Step, rc io.ReadCloser) error {
			uploads.Add(1)
			defer uploads.Done()
			defer rc.Close()
			_, _ = io.Copy(io.Discard, rc)
			rec.record()
			return nil
		})),
	)
}

// A canceled single-step workflow traces twice (started + finished) and logs once.
const expectedCallbacks = 3

// waitForCallbacks blocks until every expected tracer/logger callback has been
// invoked, no matter which goroutine invoked it.
func waitForCallbacks(t *testing.T, rec *callbackRecorder) {
	t.Helper()
	assert.Eventually(t, func() bool {
		total, _, _ := rec.snapshot()
		return total >= expectedCallbacks
	}, 2*time.Second, 5*time.Millisecond, "expected %d tracer/logger callbacks", expectedCallbacks)
}

// TestRunJoinsStepGoroutinesOnCancel is the regression test for
// https://codefloe.com/crowci/crow/issues/1396: on the cancel path Run used to
// return while its step goroutines were still running, so the tracer and logger
// callbacks — which the agent uses as upload producers — outlived Run. The agent
// had already moved on to uploads.Wait(), and a late uploads.Add(1) panicked with
// "sync: WaitGroup is reused before previous Wait has returned", killing the
// runner goroutine and leaving behind an agent that still reported healthy.
func TestRunJoinsStepGoroutinesOnCancel(t *testing.T) {
	eng := newCancelBackend()
	rec := &callbackRecorder{}

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	var uploads sync.WaitGroup
	r := newRecordingRuntime(ctx, eng, rec, &uploads)

	go func() {
		<-eng.waitEntered
		cancel(nil)
	}()

	err := r.Run(context.Background())
	rec.runDone()
	uploads.Wait()

	assert.ErrorIs(t, err, ErrCancel)

	// Give any goroutine that outlived Run the chance to invoke a callback, so
	// afterRun reports a real violation rather than passing vacuously.
	waitForCallbacks(t, rec)

	total, afterRun, atSplit := rec.snapshot()
	assert.Equal(t, expectedCallbacks, atSplit, "all tracer/logger callbacks must complete before Run returns")
	assert.Equal(t, 0, afterRun, "no tracer/logger callback may run after Run returned")
	assert.Equal(t, expectedCallbacks, total)
}

// TestRunAdoptedJoinsStepGoroutinesOnCancel covers the same invariant for the
// adopted-workflow code path.
func TestRunAdoptedJoinsStepGoroutinesOnCancel(t *testing.T) {
	eng := newCancelBackend()
	rec := &callbackRecorder{}

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	var uploads sync.WaitGroup
	r := newRecordingRuntime(ctx, eng, rec, &uploads)

	go func() {
		<-eng.waitEntered
		cancel(nil)
	}()

	err := r.RunAdopted(context.Background())
	rec.runDone()
	uploads.Wait()

	assert.ErrorIs(t, err, ErrCancel)

	waitForCallbacks(t, rec)

	total, afterRun, atSplit := rec.snapshot()
	assert.Equal(t, expectedCallbacks, atSplit, "all tracer/logger callbacks must complete before RunAdopted returns")
	assert.Equal(t, 0, afterRun, "no tracer/logger callback may run after RunAdopted returned")
	assert.Equal(t, expectedCallbacks, total)
}

// TestRunLeavesNoGoroutineBehind asserts that abandoning a stage on cancel does
// not leave anything running — neither a step goroutine nor the goroutine that
// publishes the stage result on a channel nobody reads.
func TestRunLeavesNoGoroutineBehind(t *testing.T) {
	before := runtime.NumGoroutine()

	eng := newCancelBackend()

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	r := New(singleStepConfig(), WithContext(ctx), WithBackend(eng))

	go func() {
		<-eng.waitEntered
		cancel(nil)
	}()

	err := r.Run(context.Background())
	assert.ErrorIs(t, err, ErrCancel)

	// Poll inline rather than via assert.Eventually, which would spawn
	// goroutines of its own and skew the count.
	after := runtime.NumGoroutine()
	for range 200 {
		if after <= before {
			break
		}
		time.Sleep(10 * time.Millisecond)
		after = runtime.NumGoroutine()
	}
	assert.LessOrEqual(t, after, before, "a goroutine outlived Run after cancel")
}
