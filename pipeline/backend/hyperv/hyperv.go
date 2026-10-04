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

// Package hyperv runs each workflow in a throwaway Windows VM on a Hyper-V
// host.
//
// The model is the tart backend's, with Hyper-V in place of
// Virtualization.framework: one VM per workflow rather than per step, because
// steps share a checkout. The VM is a differencing child of a golden VHDX, so
// creating it is instant and it inherits nothing from the workflow before it.
//
// Steps run over PowerShell Direct, which reaches the guest across the VMBus
// rather than the network, so a guest needs no network adapter for the agent
// to drive it and the agent never has a route into the VM to be borrowed. The
// agent itself stays on the host, so pipeline code never shares an operating
// system with it or with its registration token.
//
// This is the isolation the docker backend cannot offer on Windows: there,
// backend_options.docker.isolation gives a step its own kernel but leaves the
// daemon, its image cache and its network shared and long-lived across every
// job on the host.
package hyperv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/pipeline/backend/common"
	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

type workflowState struct {
	vm    string
	disk  string
	paths guestPaths

	mu          sync.RWMutex
	stepCMDs    map[string]*exec.Cmd
	stepOutputs map[string]io.ReadCloser
}

func newWorkflowState(vm, disk string, paths guestPaths) *workflowState {
	return &workflowState{
		vm:          vm,
		disk:        disk,
		paths:       paths,
		stepCMDs:    make(map[string]*exec.Cmd),
		stepOutputs: make(map[string]io.ReadCloser),
	}
}

type hyperv struct {
	config    config
	workflows sync.Map

	// mu guards leased, the count of VM slots currently taken.
	mu     sync.Mutex
	leased int
}

// New returns a new Hyper-V Backend.
func New() types.Backend {
	return &hyperv{}
}

func (e *hyperv) Name() string {
	return EngineName
}

// IsAvailable reports whether this host can run the backend.
//
// The OS check comes first and is not negotiable: Hyper-V exists only on
// Windows, and PowerShell on any other platform is not driving a hypervisor.
// Whether the Hyper-V role is actually installed is left to Load, which can
// say so properly instead of silently disqualifying the backend.
func (e *hyperv) IsAvailable(_ context.Context) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	_, err := exec.LookPath("powershell.exe")
	return err == nil
}

func (e *hyperv) Flags() []cli.Flag {
	return Flags
}

func (e *hyperv) Load(ctx context.Context) (*types.BackendInfo, error) {
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
		return nil, fmt.Errorf("PowerShell %q not found: %w", e.config.Binary, err)
	}

	if err := e.checkHost(ctx); err != nil {
		return nil, err
	}

	log.Info().
		Str("base", e.config.BaseVHDX).
		Int("maxVMs", e.config.MaxVMs).
		Str("switch", e.config.SwitchName).
		Msg("hyperv backend ready")

	return &types.BackendInfo{
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
		HostCPUs: int32(runtime.NumCPU()), //nolint:gosec // core count never overflows int32
		HostMem:  e.hostMemTotal(ctx),
	}, nil
}

// checkHost verifies the two things that make the rest of the backend possible
// and that are worth failing at startup over rather than on the first build:
// the Hyper-V cmdlets are present, and the golden disk exists.
func (e *hyperv) checkHost(ctx context.Context) error {
	script := psPreamble +
		"if (-not (Get-Command Get-VM -ErrorAction SilentlyContinue)) { throw 'the Hyper-V PowerShell module is not installed' }\n" +
		fmt.Sprintf("if (-not (Test-Path -LiteralPath %s)) { throw 'base VHDX not found' }\n", psQuote(e.config.BaseVHDX))
	if out, err := e.run(ctx, script); err != nil {
		return fmt.Errorf("hyperv host check: %w: %s", err, bytes.TrimSpace(out))
	}
	return nil
}

// hostMemTotal reads the host's physical memory. A failure is not fatal: the
// figure is reported to the server for scheduling, not used here.
func (e *hyperv) hostMemTotal(ctx context.Context) int64 {
	out, err := e.run(ctx, psPreamble+"(Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory\n")
	if err != nil {
		log.Debug().Err(err).Msg("could not read host memory")
		return 0
	}
	var mem int64
	if _, err := fmt.Sscan(strings.TrimSpace(string(out)), &mem); err != nil {
		return 0
	}
	return mem
}

// command builds a PowerShell invocation that reads its script from stdin.
//
// Nothing is passed in argv, because these scripts carry the guest password
// and a step's environment: argv is readable by other processes on the host,
// stdin is not. The agent's own environment is deliberately not inherited
// either, since it holds CROW_AGENT_SECRET.
func (e *hyperv) command(ctx context.Context) *exec.Cmd {
	cmd := exec.CommandContext(ctx, e.config.Binary,
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", "-")
	cmd.Env = []string{}
	return cmd
}

// run executes a host-side script to completion and returns its combined
// output.
func (e *hyperv) run(ctx context.Context, script string) ([]byte, error) {
	cmd := e.command(ctx)
	cmd.Stdin = strings.NewReader(script)
	return cmd.CombinedOutput()
}

// execGuest runs a script inside the guest and returns its combined output.
// Used for the backend's own setup and probe work, not for steps.
func (e *hyperv) execGuest(ctx context.Context, vm, body string) ([]byte, error) {
	return e.run(ctx, execScript(vm, e.config.GuestUsername, e.config.GuestPassword, body))
}

// runGuestScript starts a step's script in the guest and returns the running
// command together with its tagged output stream.
//
// PowerShell Direct carries the guest's stdout and stderr back interleaved on
// one channel, so unlike the tart backend there are not two streams to tag
// apart; everything is reported as stdout.
func (e *hyperv) runGuestScript(ctx context.Context, state *workflowState, stepUUID, body string) (*exec.Cmd, io.ReadCloser, error) {
	script := runStepScript(
		state.vm, e.config.GuestUsername, e.config.GuestPassword, body,
		stepScriptPath(state.paths, stepUUID), stepCodePath(state.paths, stepUUID),
	)

	cmd := e.command(ctx)
	cmd.Stdin = strings.NewReader(script)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("could not create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("could not create stderr pipe: %w", err)
	}
	output := common.TaggedMergedPipe(stdout, stderr, false)

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return cmd, output, nil
}

// SetupWorkflow creates, boots and prepares this workflow's VM.
func (e *hyperv) SetupWorkflow(ctx context.Context, conf *types.Config, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msg("create workflow environment")

	if len(conf.TempVolumes) > 0 {
		return fmt.Errorf("the hyperv backend does not support `temp_volumes`")
	}

	if err := e.leaseVM(); err != nil {
		return err
	}

	vm := vmName(taskUUID)
	state := newWorkflowState(vm, diskPath(e.config.VMPath, vm), newGuestPaths(e.config.GuestBase))

	// From here on any failure has to take the VM and its slot with it, or a
	// failed setup leaks a disk and permanently burns one of the licensed
	// slots this host is allowed to run.
	setupFailed := func(err error) error {
		if destroyErr := e.destroyVM(ctx, state.vm, state.disk); destroyErr != nil {
			logDestroyFailure(state.vm, destroyErr)
		}
		e.releaseVM()
		return err
	}

	if err := e.createVM(ctx, state.vm, state.disk); err != nil {
		return setupFailed(err)
	}
	if err := e.startVM(ctx, state.vm); err != nil {
		return setupFailed(err)
	}
	if err := e.waitForGuest(ctx, state.vm); err != nil {
		return setupFailed(err)
	}
	if out, err := e.execGuest(ctx, state.vm, setupScript(state.paths)); err != nil {
		return setupFailed(fmt.Errorf("prepare guest directories: %w: %s", err, bytes.TrimSpace(out)))
	}

	e.saveState(taskUUID, state)
	return nil
}

// StartStep runs a step's script in the workflow's VM.
func (e *hyperv) StartStep(ctx context.Context, step *types.Step, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msgf("start step %s", step.Name)

	state, err := e.getState(taskUUID)
	if err != nil {
		return err
	}

	var body string
	switch step.Type {
	case types.StepTypeClone:
		types.ReportStepProgress(ctx, "Preparing clone process")
		body, err = e.cloneStepScript(ctx, step, state)
		if err != nil {
			return err
		}
	case types.StepTypeCommands, types.StepTypeService:
		types.ReportStepProgress(ctx, "Preparing command process")
		body = buildScript(stepEnv(step, state.paths), state.paths.Workspace, step.Commands)
	case types.StepTypePlugin:
		types.ReportStepProgress(ctx, "Preparing plugin process")
		// A plugin is a binary on the guest's PATH, as in the local backend.
		body = buildScript(stepEnv(step, state.paths), state.paths.Workspace, []string{"& " + psQuote(step.Image)})
	default:
		return ErrUnsupportedStepType
	}

	types.ReportStepProgress(ctx, "Starting guest process")
	cmd, output, err := e.runGuestScript(ctx, state, step.UUID, body)
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
func (e *hyperv) WaitStep(_ context.Context, step *types.Step, taskUUID string) (*types.State, error) {
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

// TailStep returns a step's output.
func (e *hyperv) TailStep(_ context.Context, step *types.Step, taskUUID string) (io.ReadCloser, error) {
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
func (e *hyperv) DestroyStep(_ context.Context, _ *types.Step, _ string) error {
	return nil
}

// DestroyWorkflow stops and deletes this workflow's VM and frees its slot.
//
// A workflow whose setup failed never stored any state, and SetupWorkflow has
// already torn down whatever it had created. Reporting "workflow state not
// found" here would bury the real error under a second, meaningless one, so
// having nothing to destroy is not a failure.
func (e *hyperv) DestroyWorkflow(ctx context.Context, _ *types.Config, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msg("delete workflow environment")

	state, err := e.getState(taskUUID)
	if errors.Is(err, ErrWorkflowStateNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	defer func() {
		e.deleteState(taskUUID)
		e.releaseVM()
	}()
	return e.destroyVM(ctx, state.vm, state.disk)
}

func (e *hyperv) getState(taskUUID string) (*workflowState, error) {
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

func (e *hyperv) saveState(taskUUID string, state *workflowState) {
	e.workflows.Store(taskUUID, state)
}

func (e *hyperv) deleteState(taskUUID string) {
	e.workflows.Delete(taskUUID)
}
