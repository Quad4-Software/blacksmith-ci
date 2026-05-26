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

package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"codefloe.com/crowci/crow/v5/pipeline/backend/common/clone"
	backend "codefloe.com/crowci/crow/v5/pipeline/backend/types"
)

// nativeCloneTargetPath computes the host filesystem path to clone INTO,
// given a docker volume's mountpoint and the step's container-side paths.
//
// The volume is bind-mounted into container steps at workspaceBase. Files
// written to mountpoint/<x> are seen at workspaceBase/<x> inside containers.
// The clone step needs files to land at workingDir inside the container, so
// on the host we write to mountpoint + (workingDir relative to workspaceBase).
func nativeCloneTargetPath(mountpoint, workspaceBase, workingDir string) string {
	rel := strings.TrimPrefix(workingDir, workspaceBase)
	rel = strings.TrimPrefix(rel, `\`)
	rel = strings.TrimPrefix(rel, `/`)
	if rel == "" {
		return mountpoint
	}
	// Split on either separator so each segment is joined with the host's
	// native separator. Input workingDir may use `\` (Windows) while the
	// test/host may evaluate filepath.Join with `/` (POSIX).
	parts := strings.FieldsFunc(rel, func(r rune) bool {
		return r == '\\' || r == '/'
	})
	return filepath.Join(append([]string{mountpoint}, parts...)...)
}

// nativeCloneState tracks an in-flight native clone on Windows. The docker
// backend stores one per clone step keyed by step.UUID.
type nativeCloneState struct {
	output *clone.StreamBuffer

	mu       sync.Mutex
	finished bool
	state    *backend.State
	done     chan struct{}
}

func newNativeCloneState() *nativeCloneState {
	return &nativeCloneState{
		output: clone.NewStreamBuffer(),
		done:   make(chan struct{}),
	}
}

// finish marks the clone complete with the given exit state and unblocks
// any waiters. Safe to call exactly once; subsequent calls are no-ops.
func (s *nativeCloneState) finish(state *backend.State) {
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return
	}
	s.finished = true
	s.state = state
	s.mu.Unlock()
	_ = s.output.Close()
	close(s.done)
}

// wait blocks until the clone completes and returns the final state.
// Returns the same state on repeated calls.
func (s *nativeCloneState) wait() (*backend.State, error) {
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state, nil
}

// tail returns a reader streaming the clone's captured stdout/stderr.
// Reads block until either new output is written or finish() is called.
func (s *nativeCloneState) tail() io.ReadCloser {
	return s.output.Reader()
}

// runNativeCloneRequest captures everything needed to start a native
// clone subprocess.
type runNativeCloneRequest struct {
	Binary  string   // absolute path to plugin-clone.exe
	Dir     string   // working dir for the process — the volume-mounted workspace
	Env     []string // process environment (already augmented by clone.Env)
	HomeDir string   // per-workflow netrc home dir; removed after the process exits
	State   *nativeCloneState
}

// runNativeClone runs the clone binary synchronously and stores the result
// on req.State. Stdout and stderr are merged into State.output via
// StreamBuffer so TailStep can stream them.
//
// The caller is responsible for kicking this off in a goroutine if async
// behavior is desired; runNativeClone itself blocks until the process exits.
//
// If req.HomeDir is set, it is removed once the process exits (regardless of
// exit code) so the netrc file containing credentials does not persist on
// disk between workflows.
func runNativeClone(ctx context.Context, req runNativeCloneRequest) {
	// Defer credential cleanup so a panic in cmd.Run or anywhere below
	// still wipes the per-workflow netrc dir from disk.
	defer func() {
		if req.HomeDir != "" {
			_ = os.RemoveAll(req.HomeDir)
		}
	}()

	cmd := exec.CommandContext(ctx, req.Binary)
	cmd.Dir = req.Dir
	cmd.Env = req.Env
	cmd.Stdout = req.State.output
	cmd.Stderr = req.State.output

	runErr := cmd.Run()
	if runErr != nil {
		exitCode := 1
		ee := &exec.ExitError{}
		if errors.As(runErr, &ee) {
			exitCode = ee.ExitCode()
		}
		req.State.finish(&backend.State{
			ExitCode: exitCode,
			Exited:   true,
			Error:    runErr,
		})
		return
	}
	req.State.finish(&backend.State{
		ExitCode: cmd.ProcessState.ExitCode(),
		Exited:   true,
	})
}

// nativeCloneConfig captures the immutable per-workflow inputs we need
// outside the *Step. The docker backend passes its config + OS/Arch here.
type nativeCloneConfig struct {
	VolumeName string
	OS, Arch   string
}

// startNativeClone prepares the host environment for a native clone and
// kicks it off in a goroutine. The clone runs against the workflow volume's
// host mountpoint so subsequent container-based steps see the cloned files.
//
// Returns immediately after starting the goroutine; callers track completion
// via the returned nativeCloneState (which the docker backend also stores in
// its nativeClones sync.Map).
func (e *docker) startNativeClone(ctx context.Context, step *backend.Step, conf nativeCloneConfig) (*nativeCloneState, error) {
	if err := clone.CheckGitAvailable(); err != nil {
		return nil, fmt.Errorf("git not available on host: %w", err)
	}
	vol, err := e.client.VolumeInspect(ctx, conf.VolumeName)
	if err != nil {
		return nil, fmt.Errorf("inspect workflow volume %q: %w", conf.VolumeName, err)
	}
	cloneDir := nativeCloneTargetPath(vol.Mountpoint, step.WorkspaceBase, step.WorkingDir)
	if err := os.MkdirAll(cloneDir, 0o755); err != nil {
		return nil, fmt.Errorf("create clone dir %q: %w", cloneDir, err)
	}

	cacheDir := filepath.Dir(vol.Mountpoint)
	binaryPath := filepath.Join(cacheDir, "plugin-clone.exe")
	if err := clone.EnsureBinary(ctx, clone.EnsureBinaryConfig{
		Dest: binaryPath,
		OS:   conf.OS,
		Arch: conf.Arch,
	}); err != nil {
		return nil, fmt.Errorf("ensure plugin-clone binary: %w", err)
	}

	// Per-workflow home dir for the netrc so concurrent workflows do not
	// race on a shared _netrc file and so credentials don't outlive the
	// clone (runNativeClone removes this dir on exit).
	homeDir, err := os.MkdirTemp(cacheDir, "clone-home-")
	if err != nil {
		return nil, fmt.Errorf("create per-workflow netrc home dir: %w", err)
	}
	if _, err := clone.WriteNetRC(homeDir, conf.OS, step.Environment); err != nil {
		_ = os.RemoveAll(homeDir)
		return nil, fmt.Errorf("write netrc: %w", err)
	}

	env := append(os.Environ(),
		"HOME="+homeDir,
		"USERPROFILE="+homeDir,
		"CI_WORKSPACE="+cloneDir,
	)
	for k, v := range step.Environment {
		env = append(env, k+"="+v)
	}
	env = clone.Env(env, step.Environment)

	state := newNativeCloneState()
	e.nativeClones.Store(step.UUID, state)

	go runNativeClone(ctx, runNativeCloneRequest{
		Binary:  binaryPath,
		Dir:     cloneDir,
		Env:     env,
		HomeDir: homeDir,
		State:   state,
	})

	return state, nil
}
