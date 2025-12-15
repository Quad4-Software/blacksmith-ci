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
	"bufio"
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

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"codeberg.org/crowci/crow/v5/pipeline/backend/types"
)

type workflowState struct {
	stepCMDs     map[string]*exec.Cmd
	baseDir      string
	homeDir      string
	workspaceDir string
	cloneBinary  string
}

type local struct {
	tempDir      string
	workflows    sync.Map
	output       io.ReadCloser
	cloneBinary  string
	os, arch     string
	sandboxLevel sandboxLevel
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

	if e.sandboxLevel != sandboxLevelNone {
		log.Info().
			Str("level", string(e.sandboxLevel)).
			Msg("macOS sandbox enabled for local backend")
	}

	return &types.BackendInfo{
		Platform: e.os + "/" + e.arch,
	}, nil
}

// SetupWorkflow the pipeline environment.
func (e *local) SetupWorkflow(_ context.Context, _ *types.Config, taskUUID string) error {
	log.Trace().Str("taskUUID", taskUUID).Msg("create workflow environment")

	baseDir, err := os.MkdirTemp(e.tempDir, "crow-local-*")
	if err != nil {
		return err
	}

	state := &workflowState{
		stepCMDs:     make(map[string]*exec.Cmd),
		baseDir:      baseDir,
		workspaceDir: filepath.Join(baseDir, "workspace"),
		homeDir:      filepath.Join(baseDir, "home"),
	}

	if err := os.Mkdir(state.homeDir, 0o700); err != nil {
		return err
	}

	if err := os.Mkdir(state.workspaceDir, 0o700); err != nil {
		return err
	}

	// Create common cache directories for build tools
	cacheDir := filepath.Join(state.homeDir, "Library", "Caches")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return err
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

	// Get environment variables
	env := os.Environ()
	for a, b := range step.Environment {
		// append allowed env vars to command env
		if !slices.Contains(notAllowedEnvVarOverwrites, a) {
			env = append(env, a+"="+b)
		}
	}

	// Set HOME and CI_WORKSPACE
	env = append(env, "HOME="+state.homeDir)
	env = append(env, "USERPROFILE="+state.homeDir)
	env = append(env, "CI_WORKSPACE="+state.workspaceDir)

	switch step.Type {
	case types.StepTypeClone:
		return e.execClone(ctx, step, state, env)
	case types.StepTypeCommands:
		return e.execCommands(ctx, step, state, env)
	case types.StepTypePlugin:
		return e.execPlugin(ctx, step, state, env)
	case types.StepTypeService:
		return e.execService(ctx, step, state, env)
	default:
		return ErrUnsupportedStepType
	}
}

// execCommands use step.Image as shell and run the commands in it.
func (e *local) execCommands(ctx context.Context, step *types.Step, state *workflowState, env []string) error {
	// Prepare commands
	// TODO: support `entrypoint` from pipeline config
	args, err := e.genCmdByShell(step.Image, step.Commands)
	if err != nil {
		return fmt.Errorf("could not convert commands into args: %w", err)
	}

	// Use "image name" as run command (indicate shell)
	cmd := exec.CommandContext(ctx, step.Image, args...)
	cmd.Env = env
	cmd.Dir = state.workspaceDir

	// Wrap with sandbox if enabled (BEFORE setting up pipes)
	cmd, err = e.wrapCommandWithSandbox(ctx, cmd, state)
	if err != nil {
		return fmt.Errorf("could not wrap command with sandbox: %w", err)
	}

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
	e.output = createTaggedMergedPipe(stdoutPipe, stderrPipe, e.os == "windows")

	state.stepCMDs[step.UUID] = cmd

	return cmd.Start()
}

// execService use step.Image as shell and run the commands in it as a background service.
func (e *local) execService(ctx context.Context, step *types.Step, state *workflowState, env []string) error {
	// Services are treated like commands but run in background
	// They should start and keep running until the context is canceled

	// Prepare commands
	args, err := e.genCmdByShell(step.Image, step.Commands)
	if err != nil {
		return fmt.Errorf("could not convert commands into args: %w", err)
	}

	// Use "image name" as run command (indicate shell)
	cmd := exec.CommandContext(ctx, step.Image, args...)
	cmd.Env = env
	cmd.Dir = state.workspaceDir

	// Wrap with sandbox if enabled
	cmd, err = e.wrapCommandWithSandbox(ctx, cmd, state)
	if err != nil {
		return fmt.Errorf("could not wrap command with sandbox: %w", err)
	}

	// Capture stdout and stderr separately with stream tags
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	// Create merged output with stream tags
	e.output = createTaggedMergedPipe(stdoutPipe, stderrPipe, e.os == "windows")

	state.stepCMDs[step.UUID] = cmd

	return cmd.Start()
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

	// Wrap with sandbox if enabled
	cmd, err = e.wrapCommandWithSandbox(ctx, cmd, state)
	if err != nil {
		return fmt.Errorf("could not wrap command with sandbox: %w", err)
	}

	// Capture stdout and stderr separately with stream tags
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	// Create merged output with stream tags
	e.output = createTaggedMergedPipe(stdoutPipe, stderrPipe, e.os == "windows")

	state.stepCMDs[step.UUID] = cmd

	return cmd.Start()
}

// WaitStep for the pipeline step to complete and returns
// the completion results.
func (e *local) WaitStep(_ context.Context, step *types.Step, taskUUID string) (*types.State, error) {
	log.Trace().Str("taskUUID", taskUUID).Msgf("wait for step %s", step.Name)

	state, err := e.getState(taskUUID)
	if err != nil {
		return nil, err
	}

	cmd, ok := state.stepCMDs[step.UUID]
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
	return e.output, nil
}

// createTaggedMergedPipe creates a pipe that merges stdout and stderr with stream tags.
// Tags: 0x01 = stdout, 0x02 = stderr.
func createTaggedMergedPipe(stdout, stderr io.ReadCloser, isWindows bool) io.ReadCloser {
	pr, pw := io.Pipe()

	go func() {
		var wg sync.WaitGroup
		const numStreams = 2 // stdout and stderr
		wg.Add(numStreams)

		// Copy stdout with tag 0x01
		go func() {
			defer wg.Done()
			reader := stdout
			if isWindows {
				// Sanitize non-UTF8 output on Windows
				reader = io.NopCloser(transform.NewReader(stdout, unicode.UTF8.NewDecoder().Transformer))
			}
			copyWithTag(pw, reader, '\x01')
			stdout.Close()
		}()

		// Copy stderr with tag 0x02
		go func() {
			defer wg.Done()
			reader := stderr
			if isWindows {
				// Sanitize non-UTF8 output on Windows
				reader = io.NopCloser(transform.NewReader(stderr, unicode.UTF8.NewDecoder().Transformer))
			}
			copyWithTag(pw, reader, '\x02')
			stderr.Close()
		}()

		wg.Wait()
		pw.Close()
	}()

	return pr
}

// copyWithTag reads from src line-by-line and writes to dst, prepending each line with a tag byte.
func copyWithTag(dst io.Writer, src io.Reader, tag byte) {
	scanner := bufio.NewScanner(src)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Prepend tag byte to each line
		tagged := make([]byte, len(line)+2) // +1 for tag, +1 for newline
		tagged[0] = tag
		copy(tagged[1:], line)
		tagged[len(tagged)-1] = '\n'
		if _, err := dst.Write(tagged); err != nil {
			log.Error().Err(err).Msg("failed to write tagged log line")
			return
		}
	}
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

	err = os.RemoveAll(state.baseDir)
	if err != nil {
		return err
	}

	e.deleteState(taskUUID)

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
