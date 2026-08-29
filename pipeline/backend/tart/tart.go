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

// Package tart runs each workflow in a throwaway macOS VM managed by Tart.
//
// The model is the local backend's with a VM wrapped around it: one VM per
// workflow rather than per step, because steps share a checkout. Steps run
// over vsock through the Tart Guest Agent, so no guest networking and no SSH
// are involved, and the agent stays on the host. Pipeline code therefore never
// shares an operating system with the agent or its registration token.
package tart

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"al.essio.dev/pkg/shellescape"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/pipeline/backend/common"
	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// guestHomeProbe asks the guest for its own home directory.
//
// Every other path is derived from the answer rather than written as a literal
// "$HOME/...", because those paths are then shell-quoted when they are exported
// or changed into, and a quoted $HOME is not expanded.
const guestHomeProbe = "printf %s \"$HOME\"\n"

// tartPath is what tart itself needs to find its own helpers. It is not the
// guest's PATH, which the guest supplies.
const tartPath = "/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin:/opt/homebrew/bin"

type workflowState struct {
	vm        string
	paths     guestPaths
	runCmd    *exec.Cmd
	runOutput *ringBuffer

	mu          sync.RWMutex
	stepCMDs    map[string]*exec.Cmd
	stepOutputs map[string]io.ReadCloser
}

func newWorkflowState(vm string, paths guestPaths) *workflowState {
	return &workflowState{
		vm:          vm,
		paths:       paths,
		runOutput:   &ringBuffer{},
		stepCMDs:    make(map[string]*exec.Cmd),
		stepOutputs: make(map[string]io.ReadCloser),
	}
}

type tart struct {
	config    config
	workflows sync.Map
}

// New returns a new tart Backend.
func New() types.Backend {
	return &tart{}
}

func (e *tart) Name() string {
	return EngineName
}

// IsAvailable reports whether this host can run the backend.
//
// The tool is macOS-only, so the OS check comes first and is not
// negotiable: on any other platform a binary called "tart" on PATH is
// something else.
func (e *tart) IsAvailable(_ context.Context) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	_, err := exec.LookPath("tart")
	return err == nil
}

func (e *tart) Flags() []cli.Flag {
	return Flags
}

func (e *tart) Load(ctx context.Context) (*types.BackendInfo, error) {
	c, ok := ctx.Value(types.CliCommand).(*cli.Command)
	if !ok {
		return nil, types.ErrNoCliContextFound
	}

	cfg, err := configFromCli(c)
	if err != nil {
		return nil, err
	}
	e.config = cfg

	if _, err := exec.LookPath(e.config.Binary); err != nil {
		return nil, fmt.Errorf("tart binary %q not found: %w", e.config.Binary, err)
	}

	log.Info().
		Str("image", e.config.Image).
		Str("network", string(e.config.Network)).
		Msg("tart backend ready")

	return &types.BackendInfo{
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
		HostCPUs: int32(runtime.NumCPU()), //nolint:gosec // core count never overflows int32
		HostMem:  hostMemTotal(ctx),
	}, nil
}

// hostMemTotal reads the host's physical memory. Doing it through sysctl(8)
// rather than a syscall keeps this package free of build tags, so its tests
// still compile and run on the Linux hosts CI uses.
func hostMemTotal(ctx context.Context) int64 {
	out, err := exec.CommandContext(ctx, "sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		log.Debug().Err(err).Msg("could not read hw.memsize")
		return 0
	}
	mem, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0
	}
	return mem
}

// command builds a tart invocation.
//
// The agent's own environment is deliberately not inherited: it holds
// CROW_AGENT_SECRET, and tart needs only a PATH and TART_HOME.
func (e *tart) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, e.config.Binary, args...)
	env := []string{"PATH=" + tartPath}
	if e.config.TartHome != "" {
		env = append(env, "TART_HOME="+e.config.TartHome)
	}
	cmd.Env = env
	return cmd
}

// execGuest runs a script in the guest to completion and returns its combined
// output. Used for the backend's own setup work, not for steps.
func (e *tart) execGuest(ctx context.Context, vm, script string) ([]byte, error) {
	cmd := e.command(ctx, execArgs(vm, guestShell)...)
	cmd.Stdin = strings.NewReader(script)
	return cmd.CombinedOutput()
}

// runGuestScript starts a step's script in the guest and returns the running
// command together with its tagged stdout/stderr stream.
func (e *tart) runGuestScript(ctx context.Context, vm, shell, script string) (*exec.Cmd, io.ReadCloser, error) {
	cmd := e.command(ctx, execArgs(vm, shell)...)
	cmd.Stdin = strings.NewReader(script)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("could not create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("could not create stderr pipe: %w", err)
	}

	// tart exec keeps the two streams separate on the host side, so the
	// tagging the agent expects works exactly as it does for local steps.
	output := common.TaggedMergedPipe(stdout, stderr, false)

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return cmd, output, nil
}

// SetupWorkflow clones, boots and prepares this workflow's VM.
func (e *tart) SetupWorkflow(ctx context.Context, conf *types.Config, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msg("create workflow environment")

	if len(conf.TempVolumes) > 0 {
		return fmt.Errorf("the tart backend does not support `temp_volumes`")
	}

	spec, err := resolveVMOptions(e.config, conf)
	if err != nil {
		return err
	}

	state := newWorkflowState(vmName(taskUUID), guestPaths{})

	if err := e.cloneVM(ctx, spec.Image, state.vm); err != nil {
		return err
	}

	// From here on any failure has to take the VM with it, or a failed setup
	// leaves a clone of a multi-gigabyte image behind for every attempt.
	setupFailed := func(err error) error {
		if destroyErr := e.destroyVM(ctx, state); destroyErr != nil {
			log.Warn().Err(destroyErr).Str("vm", state.vm).Msg("could not clean up after a failed setup")
		}
		return err
	}

	if err := e.configureVM(ctx, state.vm, spec.CPU, spec.MemoryMB); err != nil {
		return setupFailed(err)
	}

	runCmd, err := e.startVM(ctx, state.vm, state.runOutput)
	if err != nil {
		return setupFailed(err)
	}
	state.runCmd = runCmd

	guestHome, err := e.waitForGuestAgent(ctx, state.vm, state.runOutput)
	if err != nil {
		return setupFailed(err)
	}
	state.paths = newGuestPaths(guestHome)

	if out, err := e.execGuest(ctx, state.vm, setupScript(state.paths)); err != nil {
		return setupFailed(fmt.Errorf("prepare guest directories: %w: %s", err, out))
	}

	e.saveState(taskUUID, state)
	return nil
}

// StartStep runs a step's script in the workflow's VM.
func (e *tart) StartStep(ctx context.Context, step *types.Step, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msgf("start step %s", step.Name)

	state, err := e.getState(taskUUID)
	if err != nil {
		return err
	}

	// The shell is step.Image only for step types where step.Image names one.
	// A clone or plugin step's image names a binary to run, so those run under
	// the default shell instead.
	var (
		script string
		shell  = guestShell
	)
	switch step.Type {
	case types.StepTypeClone:
		script, err = e.cloneStepScript(ctx, step, state)
		if err != nil {
			return err
		}
	case types.StepTypeCommands, types.StepTypeService:
		shell = stepShell(step)
		script = buildScript(stepEnv(step, state.paths), state.paths.Workspace, step.Commands)
	case types.StepTypePlugin:
		// A plugin is a binary on the guest's PATH, as in the local backend.
		script = buildScript(stepEnv(step, state.paths), state.paths.Workspace, []string{shellescape.Quote(step.Image)})
	default:
		return ErrUnsupportedStepType
	}

	cmd, output, err := e.runGuestScript(ctx, state.vm, shell, script)
	if err != nil {
		return err
	}

	state.mu.Lock()
	state.stepCMDs[step.UUID] = cmd
	state.stepOutputs[step.UUID] = output
	state.mu.Unlock()

	return nil
}

// WaitStep waits for a step to finish and reports its exit code.
func (e *tart) WaitStep(_ context.Context, step *types.Step, taskUUID string) (*types.State, error) {
	log.Trace().Str("taskUUID", taskUUID).Msgf("wait for step %s", step.Name)

	state, err := e.getState(taskUUID)
	if err != nil {
		return nil, err
	}

	state.mu.RLock()
	cmd, ok := state.stepCMDs[step.UUID]
	state.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("step cmd for %s not found", step.UUID)
	}

	err = cmd.Wait()
	exitCode := 0

	var execExitError *exec.ExitError
	if errors.As(err, &execExitError) {
		exitCode = execExitError.ExitCode()
		// A non-zero exit code is a pipeline failure, not an agent error.
		err = nil
	}

	return &types.State{Exited: true, ExitCode: exitCode}, err
}

// TailStep returns a step's merged, stream-tagged output.
func (e *tart) TailStep(_ context.Context, step *types.Step, taskUUID string) (io.ReadCloser, error) {
	log.Trace().Str("taskUUID", taskUUID).Msgf("tail logs of step %s", step.Name)

	state, err := e.getState(taskUUID)
	if err != nil {
		return nil, err
	}

	state.mu.RLock()
	output, ok := state.stepOutputs[step.UUID]
	state.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("step output for %s not found", step.UUID)
	}
	return output, nil
}

// DestroyStep does nothing: WaitStep already waited for the command, and the
// VM's destruction takes any process the guest is still running with it.
func (e *tart) DestroyStep(_ context.Context, _ *types.Step, _ string) error {
	return nil
}

// DestroyWorkflow stops and deletes this workflow's VM.
//
// A workflow whose setup failed never stored any state, and SetupWorkflow has
// already torn down whatever it had created. Reporting "workflow state not
// found" here would bury the real error under a second, meaningless one, so
// having nothing to destroy is not a failure.
func (e *tart) DestroyWorkflow(ctx context.Context, _ *types.Config, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msg("delete workflow environment")

	state, err := e.getState(taskUUID)
	if errors.Is(err, ErrWorkflowStateNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	defer e.deleteState(taskUUID)
	return e.destroyVM(ctx, state)
}

func (e *tart) getState(taskUUID string) (*workflowState, error) {
	value, ok := e.workflows.Load(taskUUID)
	if !ok {
		return nil, ErrWorkflowStateNotFound
	}
	state, ok := value.(*workflowState)
	if !ok {
		return nil, fmt.Errorf("could not parse state: %v", value)
	}
	return state, nil
}

func (e *tart) saveState(taskUUID string, state *workflowState) {
	e.workflows.Store(taskUUID, state)
}

func (e *tart) deleteState(taskUUID string) {
	e.workflows.Delete(taskUUID)
}
