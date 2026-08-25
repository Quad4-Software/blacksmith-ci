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

package pipeline

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	backend "codefloe.com/crowci/crow/v6/pipeline/backend/types"
	pipelineErrors "codefloe.com/crowci/crow/v6/pipeline/errors/types"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/metadata"
)

// TODO: move runtime into "runtime" subpackage

type (
	// State defines the pipeline and process state.
	State struct {
		// Global state of the pipeline.
		Pipeline struct {
			// Pipeline time started
			Started int64 `json:"time"`
			// Current pipeline step
			Step *backend.Step `json:"step"`
			// Current pipeline error state
			Error error `json:"error"`
		}

		// Current process state.
		Process *backend.State
	}
)

// Runtime is a configuration runtime.
type Runtime struct {
	errMu   sync.RWMutex
	err     error
	spec    *backend.Config
	engine  backend.Backend
	started int64

	ctx    context.Context
	tracer Tracer
	logger Logger

	taskUUID string

	// stepWg tracks the goroutines that execute the steps of a workflow. On the
	// cancel path Run returns before those steps have finished, so without this
	// group they would outlive Run and keep invoking the tracer and logger
	// callbacks. The agent uses those callbacks as producers for its upload
	// WaitGroup and calls Wait() as soon as Run returns, so a late callback
	// panicked the runner goroutine with "sync: WaitGroup is reused before
	// previous Wait has returned". Run/RunAdopted join this group during
	// teardown, after DestroyWorkflow has unblocked the steps.
	stepWg sync.WaitGroup

	// detachedLogWg tracks the logging goroutines spawned for detached steps.
	// Those goroutines outlive the step's runStep call (a detached step returns
	// immediately) and are only unblocked when their TailStep reader closes,
	// e.g. on context cancellation or workflow teardown. Run/RunAdopted wait on
	// this group before returning so the goroutines are always joined instead
	// of left dangling.
	detachedLogWg sync.WaitGroup

	Description map[string]string // The runtime descriptors.
}

// New returns a new runtime using the specified runtime
// configuration and runtime engine.
func New(spec *backend.Config, opts ...Option) *Runtime {
	r := new(Runtime)
	r.Description = map[string]string{}
	r.spec = spec
	r.ctx = context.Background()
	r.taskUUID = ulid.Make().String()
	for _, opts := range opts {
		opts(r)
	}
	return r
}

func (r *Runtime) MakeLogger() zerolog.Logger {
	logCtx := log.With()
	for key, val := range r.Description {
		logCtx = logCtx.Str(key, val)
	}
	return logCtx.Logger()
}

func (r *Runtime) currentError() error {
	r.errMu.RLock()
	defer r.errMu.RUnlock()
	return r.err
}

func (r *Runtime) recordError(err error) {
	if err == nil {
		return
	}
	r.errMu.Lock()
	r.err = err
	r.errMu.Unlock()
}

// Run starts the execution of a workflow and waits for it to complete.
func (r *Runtime) Run(runnerCtx context.Context) error {
	logger := r.MakeLogger()
	logger.Debug().Msgf("executing %d steps, each once the steps it depends on finished:", countSteps(r.spec))
	for _, stage := range r.spec.Stages {
		for _, step := range stage.Steps {
			logger.Debug().
				Str("DependsOn", strings.Join(step.DependsOn, ",")).
				Msg(step.Name)
		}
	}

	defer r.teardown(runnerCtx, logger)

	r.started = time.Now().Unix()
	if err := r.engine.SetupWorkflow(runnerCtx, r.spec, r.taskUUID); err != nil {
		var stepErr *pipelineErrors.ErrInvalidWorkflowSetup
		if errors.As(err, &stepErr) {
			state := new(State)
			state.Pipeline.Step = stepErr.Step
			state.Pipeline.Error = stepErr.Err
			state.Process = &backend.State{
				Error:    stepErr.Err,
				Exited:   true,
				ExitCode: 1,
			}

			// Trace the error if we have a tracer
			if r.tracer != nil {
				if err := r.tracer.Trace(state); err != nil {
					logger.Error().Err(err).Msg("failed to trace step error")
				}
			}
		}

		return err
	}

	return r.runSteps(r.runStep)
}

// teardown destroys the workflow environment and then joins every goroutine the
// runtime spawned. Run and RunAdopted defer it so that once they return, no
// tracer or logger callback can still be invoked — callers such as the agent
// rely on that to know all uploads are accounted for.
//
// The order matters: a canceled workflow is abandoned with steps still waiting on
// running containers, and destroying the workflow is exactly what unblocks them,
// so DestroyWorkflow has to happen before the joins or they would block until the
// step timeout.
func (r *Runtime) teardown(runnerCtx context.Context, logger zerolog.Logger) {
	// Check if workflows should be preserved for adoption by a restarted agent.
	// This is set when the agent shuts down due to server disconnect. The
	// containers deliberately keep running, so joining the step goroutines here
	// would block until the workflow times out; the agent is on its way out and
	// reports nothing for this workflow, so they are left to die with the process.
	if ShouldPreserveWorkflows() {
		logger.Info().
			Str("task_uuid", r.taskUUID).
			Msg("preserving workflow containers for adoption (server disconnect)")
		return
	}

	ctx := runnerCtx //nolint:contextcheck
	if ctx.Err() != nil {
		ctx = GetShutdownCtx()
	}
	if err := r.engine.DestroyWorkflow(ctx, r.spec, r.taskUUID); err != nil {
		logger.Error().Err(err).Msg("could not destroy engine")
	}

	// Join the step goroutines of an abandoned workflow. They are also what
	// registers detached logging goroutines, so they have to be joined first.
	r.stepWg.Wait()

	// Join detached steps' logging goroutines. Their TailStep readers close
	// once the workflow is torn down above (or once r.ctx is canceled), so
	// this wait cannot deadlock and ensures no goroutine is left dangling.
	r.detachedLogWg.Wait()
}

// Updates the current status of a step.
func (r *Runtime) traceStep(processState *backend.State, err error, step *backend.Step) error {
	if r.tracer == nil {
		// no tracer nothing to trace :)
		return nil
	}

	if processState == nil {
		processState = new(backend.State)
		if err != nil {
			processState.Error = err
			processState.Exited = true
			processState.OOMKilled = false
			processState.ExitCode = 126 // command invoked cannot be executed.
		}
	}

	state := new(State)
	state.Pipeline.Started = r.started
	state.Pipeline.Step = step
	state.Process = processState // empty
	state.Pipeline.Error = r.currentError()

	if traceErr := r.tracer.Trace(state); traceErr != nil {
		return traceErr
	}
	return err
}

type stepResult struct {
	step *backend.Step
	err  error
}

// runSteps schedules the steps of the workflow, starting each one as soon as
// the steps it depends on have finished.
//
// Order comes from Step.DependsOn only. The stage grouping the compiler emits
// is a by-product of flattening the graph and is not consulted: the compiler
// expresses order in DependsOn for sequential workflows as well as for DAG ones
// (see setDependenciesForSequence in the compiler's dag.go), so the graph is
// always complete on its own.
func (r *Runtime) runSteps(runStep func(*backend.Step) (*backend.State, error)) error {
	steps := make([]*backend.Step, 0)
	services := make([]string, 0)
	present := make(map[string]struct{})
	for _, stage := range r.spec.Stages {
		for _, step := range stage.Steps {
			steps = append(steps, step)
			present[step.Name] = struct{}{}
			if step.Type == backend.StepTypeService {
				services = append(services, step.Name)
			}
		}
	}

	pending := make(map[string]int, len(steps))
	dependents := make(map[string][]*backend.Step, len(steps))
	for _, step := range steps {
		// Services are not part of the dependency graph: the compiler puts them
		// in their own stage ahead of everything else and never runs them
		// through the DAG compiler, so a `depends_on` on a service is neither
		// linted (lintDependsOn only covers `steps`) nor validated. Honoring
		// it would let a service wait on a work step while every
		// dependency-less step waits on the services, deadlocking the workflow.
		// Services always start first instead.
		var dependencies []string
		if step.Type != backend.StepTypeService {
			dependencies = presentDependencies(step, present)
			if len(dependencies) == 0 {
				dependencies = services
			}
		}
		pending[step.Name] = len(dependencies)
		for _, dependency := range dependencies {
			dependents[dependency] = append(dependents[dependency], step)
		}
	}

	// The channel is buffered so that a step goroutine can always publish its
	// result and exit: on the cancel path the loop below returns without
	// draining, and teardown then joins the goroutines via r.stepWg. One slot
	// per step is enough because every step publishes exactly one result.
	results := make(chan stepResult, len(steps))
	logger := r.MakeLogger()

	// upstreamFailed records, per step, whether anything it transitively
	// depends on failed. Whether a step runs is decided from this and never
	// from the workflow-wide error: with lanes of different lengths running
	// concurrently, a workflow-wide error is observed at a point that depends
	// on which lane happens to finish first, so the same workflow would skip
	// different steps from run to run.
	upstreamFailed := make(map[string]bool, len(steps))
	ready := make([]*backend.Step, 0, len(steps))
	for _, step := range steps {
		if pending[step.Name] == 0 {
			ready = append(ready, step)
		}
	}

	// release marks a step as finished and queues the dependents it unblocks,
	// carrying the failure of its lane down to them.
	release := func(step *backend.Step, failed bool) {
		failed = failed || upstreamFailed[step.Name]
		for _, dependent := range dependents[step.Name] {
			if failed {
				upstreamFailed[dependent.Name] = true
			}
			pending[dependent.Name]--
			if pending[dependent.Name] == 0 {
				ready = append(ready, dependent)
			}
		}
	}

	inFlight := 0
	completed := 0
	for completed < len(steps) {
		// Start everything that became runnable, resolving the steps that are
		// skipped without spending a goroutine on them. Both the decision and
		// the bookkeeping happen here on the scheduler goroutine, so they do
		// not race with running steps.
		for len(ready) > 0 {
			step := ready[0]
			ready = ready[1:]

			if reason, skip := skipReason(step, upstreamFailed[step.Name]); skip {
				logger.Debug().Str("step", step.Name).Msg(reason)
				completed++
				release(step, false)
				continue
			}

			inFlight++
			r.stepWg.Add(1)
			go func() {
				defer r.stepWg.Done()
				results <- stepResult{step: step, err: r.executeStep(step, runStep)}
			}()
		}

		if completed >= len(steps) {
			break
		}

		// Nothing is running and nothing can be started, but steps remain: the
		// loop would block on results that will never arrive. Fail loudly
		// instead of hanging until the workflow timeout.
		if inFlight == 0 {
			return &StuckStepGraphError{Steps: unstartedSteps(steps, pending)}
		}

		select {
		case <-r.ctx.Done():
			return ErrCancel
		case result := <-results:
			inFlight--
			completed++
			r.recordError(result.err)
			release(result.step, result.err != nil)
		}
	}

	return r.currentError()
}

// skipReason reports whether a step is skipped because of the outcome of the
// lane it belongs to, and why. A step runs on the failure of its own
// dependencies rather than on the failure of the workflow as a whole, so an
// unrelated lane failing no longer takes it down with it.
func skipReason(step *backend.Step, upstreamFailed bool) (string, bool) {
	switch {
	case upstreamFailed && !step.OnFailure:
		return "skipped, a step it depends on failed", true
	case !upstreamFailed && !step.OnSuccess:
		return "skipped, the steps it depends on succeeded", true
	default:
		return "", false
	}
}

// presentDependencies returns the dependencies of a step that are actually part
// of the workflow being run. A compiled config can name steps that are not in
// it: a step rerun keeps the target step and its dependents but drops the steps
// it depends on (see filterConfigInPlace in server/pipeline/rerun.go), leaving
// DependsOn pointing at steps that were filtered out. Waiting for those would
// stall the workflow forever, so they are ignored and the step is scheduled on
// whatever remains.
func presentDependencies(step *backend.Step, present map[string]struct{}) []string {
	dependencies := make([]string, 0, len(step.DependsOn))
	for _, dependency := range step.DependsOn {
		if _, ok := present[dependency]; ok {
			dependencies = append(dependencies, dependency)
		}
	}
	return dependencies
}

// countSteps returns the number of steps in a workflow across all stages.
func countSteps(spec *backend.Config) int {
	count := 0
	for _, stage := range spec.Stages {
		count += len(stage.Steps)
	}
	return count
}

// unstartedSteps lists the steps that still have unmet dependencies, for the
// diagnostics of StuckStepGraphError.
func unstartedSteps(steps []*backend.Step, pending map[string]int) []string {
	names := make([]string, 0)
	for _, step := range steps {
		if pending[step.Name] > 0 {
			names = append(names, step.Name)
		}
	}
	return names
}

// executeStep traces and runs a single step. Whether the step should run at all
// is decided by the scheduler in runSteps before this is called. The runStep
// argument starts the step and waits for it, which differs between a fresh
// workflow and an adopted one.
func (r *Runtime) executeStep(step *backend.Step, runStep func(*backend.Step) (*backend.State, error)) error {
	logger := r.MakeLogger()
	logger.Debug().Str("step", step.Name).Msg("prepare")

	if err := r.traceStep(nil, nil, step); err != nil {
		return err
	}

	// add compatibility for drone-ci plugins
	metadata.SetDroneEnviron(step.Environment)
	logger.Debug().Str("step", step.Name).Msg("executing")
	processState, err := runStep(step)
	logger.Debug().Str("step", step.Name).Msg("complete")

	err = r.traceStep(processState, err, step)
	if err != nil && step.Failure == metadata.FailureIgnore {
		return nil
	}
	return err
}

// RunAdopted resumes monitoring of an adopted workflow (from a previous agent instance).
// Unlike Run, this skips SetupWorkflow (environment already exists) and uses ResumeStep
// instead of StartStep (containers already running).
func (r *Runtime) RunAdopted(runnerCtx context.Context) error {
	logger := r.MakeLogger()
	logger.Info().Msgf("resuming adopted workflow with %d steps", countSteps(r.spec))

	// Check if engine supports resuming steps
	resumer, ok := r.engine.(backend.WorkflowResumer)
	if !ok {
		return errors.New("backend does not support workflow resumption")
	}

	// For adopted workflows we still clean up when done (unless preservation is
	// enabled for another restart).
	defer r.teardown(runnerCtx, logger)

	r.started = time.Now().Unix()
	// Skip SetupWorkflow - the workflow environment (network, volume) already exists

	return r.runSteps(func(step *backend.Step) (*backend.State, error) {
		return r.runAdoptedStep(step, resumer)
	})
}

// runAdoptedStep runs an adopted step (already running container).
func (r *Runtime) runAdoptedStep(step *backend.Step, resumer backend.WorkflowResumer) (*backend.State, error) {
	logger := r.MakeLogger()
	logger.Debug().Str("step", step.Name).Msg("resuming adopted step")

	// Resume step - verifies container exists and is running
	if err := resumer.ResumeStep(r.ctx, step, r.taskUUID); err != nil {
		return nil, err
	}

	// For detached steps the logging goroutine outlives this call, so it is
	// tracked on the runtime-wide WaitGroup and joined by Run/RunAdopted. For
	// regular steps a local WaitGroup is used and waited on below.
	wg := &r.detachedLogWg
	if !step.Detached {
		wg = new(sync.WaitGroup)
	}
	if r.logger != nil {
		rc, err := r.engine.TailStep(r.ctx, step, r.taskUUID)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil, ErrCancel
			}
			return nil, err
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			logger := r.MakeLogger()

			if err := r.logger(step, rc); err != nil {
				logger.Error().Err(err).Msg("process logging failed")
			}
			_ = rc.Close()
		}()
	}

	// nothing else to do, this is a detached process
	if step.Detached {
		return nil, nil
	}

	// We wait until all data was logged
	wg.Wait()

	waitState, err := r.engine.WaitStep(r.ctx, step, r.taskUUID)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return waitState, ErrCancel
		}
		return nil, err
	}

	if err := r.engine.DestroyStep(r.ctx, step, r.taskUUID); err != nil {
		return nil, err
	}

	if waitState.OOMKilled {
		return waitState, &OomError{
			UUID: step.UUID,
			Code: waitState.ExitCode,
		}
	} else if waitState.ExitCode != 0 {
		return waitState, &ExitError{
			UUID: step.UUID,
			Code: waitState.ExitCode,
		}
	}

	return waitState, nil
}

// Runs the step and returns the state and error.
func (r *Runtime) runStep(step *backend.Step) (*backend.State, error) {
	if err := r.engine.StartStep(r.ctx, step, r.taskUUID); err != nil {
		return nil, err
	}

	// For detached steps the logging goroutine outlives this call, so it is
	// tracked on the runtime-wide WaitGroup and joined by Run/RunAdopted. For
	// regular steps a local WaitGroup is used and waited on below.
	wg := &r.detachedLogWg
	if !step.Detached {
		wg = new(sync.WaitGroup)
	}
	if r.logger != nil {
		rc, err := r.engine.TailStep(r.ctx, step, r.taskUUID)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil, ErrCancel
			}
			return nil, err
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			logger := r.MakeLogger()

			if err := r.logger(step, rc); err != nil {
				logger.Error().Err(err).Msg("process logging failed")
			}
			_ = rc.Close()
		}()
	}

	// nothing else to do, this is a detached process.
	if step.Detached {
		return nil, nil
	}

	// We wait until all data was logged. (Needed for some backends like local as WaitStep kills the log stream)
	wg.Wait()

	waitState, err := r.engine.WaitStep(r.ctx, step, r.taskUUID)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return waitState, ErrCancel
		}
		return nil, err
	}

	if err := r.engine.DestroyStep(r.ctx, step, r.taskUUID); err != nil {
		return nil, err
	}

	if waitState.OOMKilled {
		return waitState, &OomError{
			UUID: step.UUID,
			Code: waitState.ExitCode,
		}
	} else if waitState.ExitCode != 0 {
		return waitState, &ExitError{
			UUID: step.UUID,
			Code: waitState.ExitCode,
		}
	}

	return waitState, nil
}
