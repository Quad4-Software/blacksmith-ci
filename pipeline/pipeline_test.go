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
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	backend "codefloe.com/crowci/crow/v6/pipeline/backend/types"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/metadata"
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
	stepStarted  chan string
	stepReleases map[string]chan struct{}

	enteredOnce sync.Once
	releaseOnce sync.Once

	mu      sync.Mutex
	started []string
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

func (b *cancelBackend) StartStep(_ context.Context, step *backend.Step, _ string) error {
	b.mu.Lock()
	b.started = append(b.started, step.Name)
	b.mu.Unlock()
	if b.stepStarted != nil {
		b.stepStarted <- step.Name
	}
	return nil
}

func (b *cancelBackend) startedNames() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.started...)
}

// ResumeStep makes cancelBackend satisfy backend.WorkflowResumer so it can be
// used with RunAdopted.
func (b *cancelBackend) ResumeStep(context.Context, *backend.Step, string) error { return nil }

func (b *cancelBackend) WaitStep(_ context.Context, step *backend.Step, _ string) (*backend.State, error) {
	if release := b.stepReleases[step.Name]; release != nil {
		<-release
		return &backend.State{Exited: true}, nil
	}
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

// A canceled single-step workflow traces preparation, start, and finish, and logs once.
const (
	expectedCallbacks        = 4
	expectedAdoptedCallbacks = 3
)

// waitForCallbacks blocks until every expected tracer/logger callback has been
// invoked, no matter which goroutine invoked it.
func waitForCallbacks(t *testing.T, rec *callbackRecorder, expected int) {
	t.Helper()
	assert.Eventually(t, func() bool {
		total, _, _ := rec.snapshot()
		return total >= expected
	}, 2*time.Second, 5*time.Millisecond, "expected %d tracer/logger callbacks", expected)
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
	waitForCallbacks(t, rec, expectedCallbacks)

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

	waitForCallbacks(t, rec, expectedAdoptedCallbacks)

	total, afterRun, atSplit := rec.snapshot()
	assert.Equal(t, expectedAdoptedCallbacks, atSplit, "all tracer/logger callbacks must complete before RunAdopted returns")
	assert.Equal(t, 0, afterRun, "no tracer/logger callback may run after RunAdopted returned")
	assert.Equal(t, expectedAdoptedCallbacks, total)
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

func TestRunStartsReadyDependents(t *testing.T) {
	step := func(name string, dependencies ...string) *backend.Step {
		return &backend.Step{
			Name:        name,
			UUID:        name + "-uuid",
			Environment: map[string]string{},
			OnSuccess:   true,
			DependsOn:   dependencies,
		}
	}
	service := step("service")
	service.Type = backend.StepTypeService
	service.Detached = true
	spec := &backend.Config{Stages: []*backend.Stage{
		{Steps: []*backend.Step{service}},
		{Steps: []*backend.Step{step("build-fast"), step("build-slow")}},
		{Steps: []*backend.Step{
			step("scan-fast", "build-fast"),
			step("scan-slow", "build-slow"),
		}},
	}}

	releases := map[string]chan struct{}{}
	open := map[string]struct{}{}
	for _, name := range []string{"build-fast", "build-slow", "scan-fast", "scan-slow"} {
		releases[name] = make(chan struct{})
		open[name] = struct{}{}
	}
	started := make(chan string, len(releases)+1)
	eng := newCancelBackend()
	eng.stepStarted = started
	eng.stepReleases = releases
	r := New(spec, WithBackend(eng))
	done := make(chan error, 1)
	go func() {
		done <- r.Run(context.Background())
	}()
	t.Cleanup(func() {
		for name := range open {
			close(releases[name])
		}
		r.stepWg.Wait()
	})

	waitForStart := func() string {
		select {
		case name := <-started:
			return name
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for step to start")
			return ""
		}
	}
	release := func(name string) {
		close(releases[name])
		delete(open, name)
	}

	require.Equal(t, "service", waitForStart())
	require.ElementsMatch(t, []string{"build-fast", "build-slow"}, []string{waitForStart(), waitForStart()})
	release("build-fast")
	require.Equal(t, "scan-fast", waitForStart())
	release("scan-fast")
	release("build-slow")
	require.Equal(t, "scan-slow", waitForStart())
	release("scan-slow")
	require.NoError(t, <-done)
}

// nopBackend runs every step to completion immediately, so a test that hangs on
// it hangs because of the scheduler and not because of the backend.
type nopBackend struct {
	// block holds a step in WaitStep until the channel is closed.
	block map[string]chan struct{}
	// fail makes a step exit non-zero.
	fail map[string]bool

	mu      sync.Mutex
	started []string
}

func (b *nopBackend) Name() string                     { return "nop-test" }
func (b *nopBackend) IsAvailable(context.Context) bool { return true }
func (b *nopBackend) Flags() []cli.Flag                { return nil }

func (b *nopBackend) Load(context.Context) (*backend.BackendInfo, error) {
	return &backend.BackendInfo{Platform: "test"}, nil
}

func (b *nopBackend) SetupWorkflow(context.Context, *backend.Config, string) error { return nil }

func (b *nopBackend) StartStep(_ context.Context, step *backend.Step, _ string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.started = append(b.started, step.Name)
	return nil
}

// ResumeStep makes nopBackend satisfy backend.WorkflowResumer so it can be used
// with RunAdopted.
func (b *nopBackend) ResumeStep(_ context.Context, step *backend.Step, _ string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.started = append(b.started, step.Name)
	return nil
}

func (b *nopBackend) WaitStep(_ context.Context, step *backend.Step, _ string) (*backend.State, error) {
	if release := b.block[step.Name]; release != nil {
		<-release
	}
	state := &backend.State{Exited: true}
	if b.fail[step.Name] {
		state.ExitCode = 1
	}
	return state, nil
}

func (b *nopBackend) TailStep(context.Context, *backend.Step, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("some log line\n")), nil
}

func (b *nopBackend) DestroyStep(context.Context, *backend.Step, string) error { return nil }

func (b *nopBackend) DestroyWorkflow(context.Context, *backend.Config, string) error {
	// Release anything still blocked so a failing test tears down instead of
	// hanging on the step goroutine join.
	for _, release := range b.block {
		select {
		case <-release:
		default:
			close(release)
		}
	}
	return nil
}

func (b *nopBackend) startedSteps() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.started...)
}

func graphStep(name string, dependencies ...string) *backend.Step {
	return &backend.Step{
		Name:        name,
		UUID:        name + "-uuid",
		Type:        backend.StepTypeCommands,
		Environment: map[string]string{},
		OnSuccess:   true,
		Failure:     metadata.FailureFail,
		DependsOn:   dependencies,
	}
}

// runWithin runs the workflow and fails the test if it does not finish, so that
// a scheduling deadlock surfaces as a failure instead of a hung test binary.
func runWithin(t *testing.T, r *Runtime) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- r.Run(context.Background()) }()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not finish, the step scheduler deadlocked")
		return nil
	}
}

// A step rerun keeps the target step and its dependents but drops the steps it
// depends on, so the compiled config it hands to the runtime contains steps
// whose DependsOn names steps that are not there. Those dependencies have to be
// ignored, otherwise the workflow never starts anything.
func TestRunIgnoresDependenciesMissingFromConfig(t *testing.T) {
	clone := graphStep("clone")
	clone.Type = backend.StepTypeClone
	spec := &backend.Config{Stages: []*backend.Stage{
		{Steps: []*backend.Step{clone}},
		// "build" was filtered out by the rerun, "test" still depends on it.
		{Steps: []*backend.Step{graphStep("test", "build")}},
		{Steps: []*backend.Step{graphStep("deploy", "test")}},
	}}

	eng := &nopBackend{}
	require.NoError(t, runWithin(t, New(spec, WithBackend(eng), WithTracer(noopTracer()))))
	assert.ElementsMatch(t, []string{"clone", "test", "deploy"}, eng.startedSteps())
}

// A dependency graph that cannot be scheduled must be reported instead of
// leaving the workflow blocked on results that will never arrive.
func TestRunReportsUnschedulableSteps(t *testing.T) {
	spec := &backend.Config{Stages: []*backend.Stage{
		{Steps: []*backend.Step{graphStep("clone")}},
		{Steps: []*backend.Step{graphStep("a", "b"), graphStep("b", "a")}},
	}}

	err := runWithin(t, New(spec, WithBackend(&nopBackend{}), WithTracer(noopTracer())))
	var stuck *StuckStepGraphError
	require.ErrorAs(t, err, &stuck)
	assert.ElementsMatch(t, []string{"a", "b"}, stuck.Steps)
}

func noopTracer() Tracer {
	return TraceFunc(func(*State) error { return nil })
}

// `depends_on` on a service passes the linter and the runtime used to ignore
// it. Honoring it deadlocks the workflow, because every dependency-less step
// waits on the services while the service waits on a work step.
func TestRunIgnoresServiceDependencies(t *testing.T) {
	clone := graphStep("clone")
	clone.Type = backend.StepTypeClone
	service := graphStep("db", "build")
	service.Type = backend.StepTypeService
	service.Detached = true
	spec := &backend.Config{Stages: []*backend.Stage{
		{Steps: []*backend.Step{service}},
		{Steps: []*backend.Step{clone}},
		{Steps: []*backend.Step{graphStep("build", "clone")}},
	}}

	eng := &nopBackend{}
	require.NoError(t, runWithin(t, New(spec, WithBackend(eng), WithTracer(noopTracer()))))
	assert.Equal(t, "db", eng.startedSteps()[0], "the service has to start first")
	assert.ElementsMatch(t, []string{"db", "clone", "build"}, eng.startedSteps())
}

// A step whose own dependencies succeeded runs even while an unrelated lane is
// failing, and it does so regardless of which lane finishes first. Deciding
// this from the workflow-wide error instead made the outcome depend on that
// ordering, so the same workflow skipped the step on some runs and not others.
func TestRunKeepsUnrelatedLanesGoing(t *testing.T) {
	tests := []struct {
		name     string
		blocked  string
		unblocks []string
	}{
		{name: "failing lane finishes last", blocked: "fail", unblocks: []string{"fail"}},
		{name: "failing lane finishes first", blocked: "ok", unblocks: []string{"ok"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clone := graphStep("clone")
			clone.Type = backend.StepTypeClone
			spec := &backend.Config{Stages: []*backend.Stage{
				{Steps: []*backend.Step{clone}},
				{Steps: []*backend.Step{graphStep("fail", "clone"), graphStep("ok", "clone")}},
				{Steps: []*backend.Step{graphStep("deploy", "ok")}},
			}}

			release := make(chan struct{})
			eng := &nopBackend{
				block: map[string]chan struct{}{test.blocked: release},
				fail:  map[string]bool{"fail": true},
			}
			r := New(spec, WithBackend(eng), WithTracer(noopTracer()))
			done := make(chan error, 1)
			go func() { done <- r.Run(context.Background()) }()

			// Whichever lane is held back, the other one runs to its end first.
			assert.Eventually(t, func() bool {
				want := "deploy"
				if test.blocked == "ok" {
					want = "fail"
				}
				return slices.Contains(eng.startedSteps(), want)
			}, 5*time.Second, 10*time.Millisecond)

			close(release)
			select {
			case err := <-done:
				assert.Error(t, err, "the workflow still fails as a whole")
			case <-time.After(10 * time.Second):
				t.Fatal("Run did not finish")
			}
			assert.ElementsMatch(t, []string{"clone", "fail", "ok", "deploy"}, eng.startedSteps())
		})
	}
}

// A failure is carried down the lane it happened in: the steps that depend on
// the failed step are skipped, and so are the steps that depend on those.
func TestRunSkipsLaneOfFailedStep(t *testing.T) {
	clone := graphStep("clone")
	clone.Type = backend.StepTypeClone
	spec := &backend.Config{Stages: []*backend.Stage{
		{Steps: []*backend.Step{clone}},
		{Steps: []*backend.Step{graphStep("build", "clone")}},
		{Steps: []*backend.Step{graphStep("test", "build")}},
		{Steps: []*backend.Step{graphStep("deploy", "test")}},
	}}

	eng := &nopBackend{fail: map[string]bool{"build": true}}
	err := runWithin(t, New(spec, WithBackend(eng), WithTracer(noopTracer())))
	require.Error(t, err)
	assert.ElementsMatch(t, []string{"clone", "build"}, eng.startedSteps(),
		"test depends on the failed step and deploy depends on test")
}

// A `when: status: failure` step runs when its own lane failed, and is skipped
// when it did not.
func TestRunStartsOnFailureStepInFailedLane(t *testing.T) {
	newSpec := func(failing bool) (*backend.Config, *nopBackend) {
		clone := graphStep("clone")
		clone.Type = backend.StepTypeClone
		notify := graphStep("notify", "build")
		notify.OnSuccess = false
		notify.OnFailure = true
		spec := &backend.Config{Stages: []*backend.Stage{
			{Steps: []*backend.Step{clone}},
			{Steps: []*backend.Step{graphStep("build", "clone")}},
			{Steps: []*backend.Step{notify}},
		}}
		return spec, &nopBackend{fail: map[string]bool{"build": failing}}
	}

	spec, eng := newSpec(true)
	require.Error(t, runWithin(t, New(spec, WithBackend(eng), WithTracer(noopTracer()))))
	assert.Contains(t, eng.startedSteps(), "notify")

	spec, eng = newSpec(false)
	require.NoError(t, runWithin(t, New(spec, WithBackend(eng), WithTracer(noopTracer()))))
	assert.NotContains(t, eng.startedSteps(), "notify")
}

// An adopted workflow goes through the same scheduler as a fresh one, so it
// resumes each step once the steps it depends on have finished rather than one
// stage at a time, and a failure only takes down its own lane.
func TestRunAdoptedSchedulesByDependencies(t *testing.T) {
	clone := graphStep("clone")
	clone.Type = backend.StepTypeClone
	spec := &backend.Config{Stages: []*backend.Stage{
		{Steps: []*backend.Step{clone}},
		{Steps: []*backend.Step{graphStep("fail", "clone"), graphStep("ok", "clone")}},
		{Steps: []*backend.Step{graphStep("after-fail", "fail"), graphStep("after-ok", "ok")}},
	}}

	eng := &nopBackend{fail: map[string]bool{"fail": true}}
	r := New(spec, WithBackend(eng), WithTracer(noopTracer()))
	done := make(chan error, 1)
	go func() { done <- r.RunAdopted(context.Background()) }()

	select {
	case err := <-done:
		require.Error(t, err, "the workflow still fails as a whole")
	case <-time.After(10 * time.Second):
		t.Fatal("RunAdopted did not finish, the step scheduler deadlocked")
	}

	assert.ElementsMatch(t, []string{"clone", "fail", "ok", "after-ok"}, eng.startedSteps(),
		"after-fail depends on the failed step, after-ok does not")
}

// A step that fails under `failure: ignore` does not count as a failure, so the
// steps depending on it still run and the workflow still succeeds.
func TestRunKeepsLaneOfIgnoredFailure(t *testing.T) {
	clone := graphStep("clone")
	clone.Type = backend.StepTypeClone
	flaky := graphStep("flaky", "clone")
	flaky.Failure = metadata.FailureIgnore
	spec := &backend.Config{Stages: []*backend.Stage{
		{Steps: []*backend.Step{clone}},
		{Steps: []*backend.Step{flaky}},
		{Steps: []*backend.Step{graphStep("after-flaky", "flaky")}},
	}}

	eng := &nopBackend{fail: map[string]bool{"flaky": true}}
	assert.NoError(t, runWithin(t, New(spec, WithBackend(eng), WithTracer(noopTracer()))))
	assert.ElementsMatch(t, []string{"clone", "flaky", "after-flaky"}, eng.startedSteps())
}

// Canceling a workflow while part of the graph is still queued has to leave no
// step goroutine behind: the queued steps are dropped and the running ones are
// joined during teardown, which is what keeps the agent's upload bookkeeping
// from being used after it was waited on (see
// TestRunJoinsStepGoroutinesOnCancel).
func TestRunJoinsStepGoroutinesOnCancelMidGraph(t *testing.T) {
	clone := graphStep("clone")
	clone.Type = backend.StepTypeClone
	spec := &backend.Config{Stages: []*backend.Stage{
		{Steps: []*backend.Step{clone}},
		{Steps: []*backend.Step{graphStep("slow", "clone")}},
		// queued behind "slow" and never started
		{Steps: []*backend.Step{graphStep("never", "slow")}},
	}}

	eng := newCancelBackend()
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	before := runtime.NumGoroutine()
	r := New(spec, WithContext(ctx), WithBackend(eng), WithTracer(noopTracer()))
	go func() {
		<-eng.waitEntered
		cancel(nil)
	}()

	assert.ErrorIs(t, r.Run(context.Background()), ErrCancel)
	assert.NotContains(t, eng.startedNames(), "never", "a queued step must not start after cancel")

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
