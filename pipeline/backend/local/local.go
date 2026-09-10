// Copyright 2022 Woodpecker Authors
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

package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/pipeline/backend/common"
	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

type workflowState struct {
	processCleanups []func() error
	stepCMDs        map[string]*exec.Cmd
	stepOutputs     map[string]io.ReadCloser
	stepCMDsMu      sync.RWMutex
	baseDir         string
	homeDir         string
	workspaceDir    string
	cloneBinary     string
	// privateDir is always a directory this workflow owns, whatever homeDir
	// points at. Anything the backend writes on a step's behalf belongs here
	// rather than in an account's home: the clone step drops a .netrc and then
	// deletes it, which would destroy a real one.
	privateDir string
	// tmpDir is this workflow's own TMPDIR. Steps used to inherit the agent
	// account's, which is shared with every other workflow on the host and with
	// the rest of the account's software.
	tmpDir string
	// secretFiles are the paths the agent may have read its own token from, for
	// the sandbox profile to deny reading.
	secretFiles []string
	// hostHome records that homeDir is the agent account's own home rather than
	// a directory this workflow owns, which decides whether it may be created,
	// and how the sandbox profile has to be ordered.
	hostHome bool
}

type local struct {
	tempDir      string
	workflows    sync.Map
	cloneBinary  string
	os, arch     string
	sandboxLevel sandboxLevel
	useHostHome  bool
	secretFiles  []string
}

// New returns a new local Backend.
func New() types.Backend {
	return &local{
		os:   runtime.GOOS,
		arch: runtime.GOARCH,
	}
}

func (e *local) Name() string {
	return "local"
}

func (e *local) IsAvailable(ctx context.Context) bool {
	if c, ok := ctx.Value(types.CliCommand).(*cli.Command); ok {
		if c.String("backend-engine") == e.Name() {
			return true
		}
	}
	_, inContainer := os.LookupEnv("CROW_IN_CONTAINER")
	return !inContainer
}

func (e *local) Flags() []cli.Flag {
	return Flags
}

func (e *local) Load(ctx context.Context) (*types.BackendInfo, error) {
	c, ok := ctx.Value(types.CliCommand).(*cli.Command)
	if ok {
		e.tempDir = c.String("backend-local-temp-dir")
		e.sandboxLevel = sandboxLevel(c.String("backend-local-sandbox-level"))
		e.useHostHome = c.Bool("backend-local-use-host-home")
	}

	// Validate sandbox level
	switch e.sandboxLevel {
	case sandboxLevelNone, sandboxLevelStandard, sandboxLevelStrict:
		// Valid levels
	default:
		log.Warn().
			Str("level", string(e.sandboxLevel)).
			Msg("invalid sandbox level, defaulting to 'none'")
		e.sandboxLevel = sandboxLevelNone
	}

	e.loadClone()

	e.secretFiles = agentSecretFiles()

	if e.sandboxLevel != sandboxLevelNone {
		log.Info().
			Str("level", string(e.sandboxLevel)).
			Msg("macOS sandbox enabled for local backend")
	}

	return &types.BackendInfo{
		Platform: e.os + "/" + e.arch,
		HostCPUs: int32(runtime.NumCPU()), //nolint:gosec // core count never overflows int32
		HostMem:  hostMemTotal(),
	}, nil
}

// SetupWorkflow the pipeline environment.
func (e *local) SetupWorkflow(_ context.Context, conf *types.Config, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msg("create workflow environment")

	if len(conf.TempVolumes) > 0 {
		return fmt.Errorf("the local backend does not support `temp_volumes`")
	}

	baseDir, err := os.MkdirTemp(e.tempDir, workflowDirPrefix+"*")
	if err != nil {
		return err
	}

	state := &workflowState{
		stepCMDs:     make(map[string]*exec.Cmd),
		stepOutputs:  make(map[string]io.ReadCloser),
		baseDir:      baseDir,
		workspaceDir: filepath.Join(baseDir, "workspace"),
		homeDir:      filepath.Join(baseDir, "home"),
		privateDir:   filepath.Join(baseDir, "home"),
		tmpDir:       filepath.Join(baseDir, "tmp"),
		secretFiles:  e.secretFiles,
		hostHome:     e.useHostHome,
	}

	if err := os.Mkdir(state.privateDir, 0o700); err != nil {
		return err
	}

	// A per-workflow home keeps runs from seeing each other's state, and is the
	// default. It is wrong for tools that ask the account for its home rather
	// than reading $HOME: Xcode looks for provisioning profiles and writes
	// DerivedData under the account's real home, so with a redirected HOME it
	// never sees what fastlane just installed.
	if state.hostHome {
		hostHomeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("backend-local-use-host-home is set but the agent account has no resolvable home: %w", err)
		}
		state.homeDir = hostHomeDir
	}

	if err := os.Mkdir(state.workspaceDir, 0o700); err != nil {
		return err
	}

	if err := os.Mkdir(state.tmpDir, 0o700); err != nil {
		return err
	}

	// Create the cache directory macOS build tools expect below a home. Other
	// platforms have no equivalent and would get a meaningless Library/Caches.
	if e.os == "darwin" {
		cacheDir := filepath.Join(state.homeDir, "Library", "Caches")
		if err := os.MkdirAll(cacheDir, 0o700); err != nil {
			return err
		}
	}

	e.saveState(taskUUID, state)

	return nil
}

// StartStep the pipeline step.
func (e *local) StartStep(ctx context.Context, step *types.Step, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msgf("start step %s", step.Name)

	state, err := e.getState(taskUUID)
	if err != nil {
		return err
	}

	// Get environment variables, minus the agent's own configuration.
	env := stepEnviron()
	for a, b := range step.Environment {
		// append allowed env vars to command env
		if !slices.Contains(notAllowedEnvVarOverwrites, a) {
			env = append(env, a+"="+b)
		}
	}

	// Set HOME and CI_WORKSPACE.
	//
	// The clone step keeps the workflow's own directory even when the rest of
	// the workflow uses the account's home. It writes a .netrc there and removes
	// it afterwards, so pointing it at a real home would overwrite and then
	// delete the account's own.
	stepHome := state.homeDir
	if step.Type == types.StepTypeClone {
		stepHome = state.privateDir
	}
	env = append(env, "HOME="+stepHome)
	env = append(env, "USERPROFILE="+stepHome)
	env = append(env, "CI_WORKSPACE="+state.workspaceDir)
	env = append(env, "TMPDIR="+state.tmpDir)
	if e.os == "windows" {
		// Nothing on Windows reads TMPDIR; Go's os.TempDir and the Windows
		// toolchains resolve TMP, then TEMP.
		env = append(env, "TMP="+state.tmpDir)
		env = append(env, "TEMP="+state.tmpDir)
	}

	switch step.Type {
	case types.StepTypeClone:
		types.ReportStepProgress(ctx, "Starting clone process")
		return e.execClone(ctx, step, state, env)
	case types.StepTypeCommands:
		types.ReportStepProgress(ctx, "Starting command process")
		return e.execCommands(ctx, step, state, env)
	case types.StepTypePlugin:
		types.ReportStepProgress(ctx, "Starting plugin process")
		return e.execPlugin(ctx, step, state, env)
	case types.StepTypeService:
		types.ReportStepProgress(ctx, "Starting service process")
		return e.execService(ctx, step, state, env)
	default:
		return ErrUnsupportedStepType
	}
}

// execCommands use step.Image as shell and run the commands in it.
func (e *local) execCommands(ctx context.Context, step *types.Step, state *workflowState, env []string) error {
	// Prepare commands
	// TODO: support `entrypoint` from pipeline config
	args, err := genCmdByShell(step.Image, step.Commands, state.tmpDir)
	if err != nil {
		return fmt.Errorf("could not convert commands into args: %w", err)
	}

	// Use "image name" as run command (indicate shell)
	cmd := exec.CommandContext(ctx, step.Image, args...)
	cmd.Env = env
	cmd.Dir = state.workspaceDir

	return e.startStepCmd(ctx, cmd, step, state)
}

// execService use step.Image as shell and run the commands in it as a background service.
func (e *local) execService(ctx context.Context, step *types.Step, state *workflowState, env []string) error {
	// Services are treated like commands but run in background
	// They should start and keep running until the context is canceled

	// Prepare commands
	args, err := genCmdByShell(step.Image, step.Commands, state.tmpDir)
	if err != nil {
		return fmt.Errorf("could not convert commands into args: %w", err)
	}

	// Use "image name" as run command (indicate shell)
	cmd := exec.CommandContext(ctx, step.Image, args...)
	cmd.Env = env
	cmd.Dir = state.workspaceDir

	return e.startStepCmd(ctx, cmd, step, state)
}

// execPlugin use step.Image as exec binary.
func (e *local) execPlugin(ctx context.Context, step *types.Step, state *workflowState, env []string) error {
	binary, err := exec.LookPath(step.Image)
	if err != nil {
		return fmt.Errorf("lookup plugin binary: %w", err)
	}

	cmd := exec.CommandContext(ctx, binary)
	cmd.Env = env
	cmd.Dir = state.workspaceDir

	return e.startStepCmd(ctx, cmd, step, state)
}

// startStepCmd wraps a step's command in the sandbox, captures its output,
// records it in the workflow state and starts it.
func (e *local) startStepCmd(ctx context.Context, cmd *exec.Cmd, step *types.Step, state *workflowState) error {
	// Wrap with sandbox if enabled (BEFORE setting up pipes)
	cmd, err := e.wrapCommandWithSandbox(ctx, cmd, state)
	if err != nil {
		return fmt.Errorf("could not wrap command with sandbox: %w", err)
	}

	configureCancel(cmd)
	configureWorkflowProcess(cmd, state)

	// Capture stdout and stderr separately with stream tags (AFTER sandbox wrapping)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("could not create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("could not create stderr pipe: %w", err)
	}

	// Create merged output with stream tags
	output := common.TaggedMergedPipe(stdoutPipe, stderrPipe, e.os == "windows")

	state.stepCMDsMu.Lock()
	state.stepCMDs[step.UUID] = cmd
	state.stepOutputs[step.UUID] = output
	state.stepCMDsMu.Unlock()

	return startWorkflowProcess(cmd, state)
}

// WaitStep for the pipeline step to complete and returns
// the completion results.
func (e *local) WaitStep(_ context.Context, step *types.Step, taskUUID string) (*types.State, error) {
	log.Trace().Str("taskUUID", taskUUID).Msgf("wait for step %s", step.Name)

	state, err := e.getState(taskUUID)
	if err != nil {
		return nil, err
	}

	state.stepCMDsMu.RLock()
	cmd, ok := state.stepCMDs[step.UUID]
	state.stepCMDsMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("step cmd for %s not found", step.UUID)
	}

	err = cmd.Wait()
	ExitCode := 0

	var execExitError *exec.ExitError
	if errors.As(err, &execExitError) {
		ExitCode = execExitError.ExitCode()
		// Non-zero exit code is a pipeline failure, but not an agent error.
		err = nil
	}

	return &types.State{
		Exited:   true,
		ExitCode: ExitCode,
	}, err
}

// TailStep the pipeline step logs.
func (e *local) TailStep(_ context.Context, step *types.Step, taskUUID string) (io.ReadCloser, error) {
	log.Trace().Str("taskUUID", taskUUID).Msgf("tail logs of step %s", step.Name)

	state, err := e.getState(taskUUID)
	if err != nil {
		return nil, err
	}

	state.stepCMDsMu.RLock()
	output, ok := state.stepOutputs[step.UUID]
	state.stepCMDsMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("step output for %s not found", step.UUID)
	}

	return output, nil
}

func (e *local) DestroyStep(_ context.Context, _ *types.Step, _ string) error {
	// WaitStep already waits for the command to finish, so there is nothing to do here.
	return nil
}

// DestroyWorkflow the pipeline environment.
func (e *local) DestroyWorkflow(_ context.Context, _ *types.Config, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msg("delete workflow environment")

	state, err := e.getState(taskUUID)
	if err != nil {
		return err
	}

	if err := cleanupWorkflowProcesses(state); err != nil {
		return fmt.Errorf("stop workflow processes: %w", err)
	}

	if err := removeAllRetry(state.baseDir); err != nil {
		return err
	}

	e.deleteState(taskUUID)

	return nil
}

// removeAllRetry deletes a workflow directory, retrying for a short while.
//
// On Windows a file stays undeletable while any handle to it is open, and
// right after a step exits there often still is one: an indexer or antivirus
// scanning what the step just wrote, or a child process that outlived it by a
// moment. The first attempt then fails with a sharing violation that clears
// itself within a second.
func removeAllRetry(dir string) error {
	const attempts = 5
	var err error
	for i := range attempts {
		if err = os.RemoveAll(dir); err == nil {
			return nil
		}
		time.Sleep(time.Duration(i+1) * 200 * time.Millisecond)
	}
	return err
}

func (e *local) getState(taskUUID string) (*workflowState, error) {
	state, ok := e.workflows.Load(taskUUID)
	if !ok {
		return nil, ErrWorkflowStateNotFound
	}

	s, ok := state.(*workflowState)
	if !ok {
		return nil, fmt.Errorf("could not parse state: %v", state)
	}

	return s, nil
}

func (e *local) saveState(taskUUID string, state *workflowState) {
	e.workflows.Store(taskUUID, state)
}

func (e *local) deleteState(taskUUID string) {
	e.workflows.Delete(taskUUID)
}
