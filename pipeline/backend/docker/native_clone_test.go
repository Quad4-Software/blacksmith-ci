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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/docker/docker/api/types/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backend "codefloe.com/crowci/crow/v5/pipeline/backend/types"
)

func TestNativeCloneTargetPath_RelToWorkspaceBase(t *testing.T) {
	got := nativeCloneTargetPath(
		`C:\ProgramData\Docker\volumes\foo\_data`,
		`C:\workspace`,
		`C:\workspace\owner\repo`,
	)
	assert.Equal(t, filepath.Join(`C:\ProgramData\Docker\volumes\foo\_data`, "owner", "repo"), got)
}

func TestNativeCloneTargetPath_WorkingDirEqualsBase(t *testing.T) {
	got := nativeCloneTargetPath(
		`C:\ProgramData\Docker\volumes\foo\_data`,
		`C:\workspace`,
		`C:\workspace`,
	)
	assert.Equal(t, `C:\ProgramData\Docker\volumes\foo\_data`, got,
		"when WorkingDir == WorkspaceBase, target is the mountpoint itself")
}

func TestNativeCloneState_WaitReturnsCompletedState(t *testing.T) {
	s := newNativeCloneState()

	go func() {
		time.Sleep(20 * time.Millisecond)
		s.finish(&backend.State{ExitCode: 0, Exited: true})
	}()

	state, err := s.wait()
	require.NoError(t, err)
	assert.Equal(t, 0, state.ExitCode)
	assert.True(t, state.Exited)
}

func TestNativeCloneState_FinishedBeforeWait(t *testing.T) {
	s := newNativeCloneState()
	s.finish(&backend.State{ExitCode: 42, Exited: true})

	state, err := s.wait()
	require.NoError(t, err)
	assert.Equal(t, 42, state.ExitCode)
}

func TestNativeCloneState_TailReadsBufferedOutput(t *testing.T) {
	s := newNativeCloneState()
	_, _ = s.output.Write([]byte("clone log line"))
	s.finish(&backend.State{ExitCode: 0, Exited: true})

	r := s.tail()
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, []byte("clone log line"), got)
}

func TestNativeCloneState_MultipleWaitsReturnSameState(t *testing.T) {
	s := newNativeCloneState()
	s.finish(&backend.State{ExitCode: 7, Exited: true})

	state1, _ := s.wait()
	state2, _ := s.wait()
	assert.Equal(t, state1, state2)
}

// Ensure backend import is used.
var _ = filepath.Join

func TestRunNativeClone_CapturesStdoutAndExitCode(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil && runtime.GOOS != "windows" {
		t.Skip("sh not available")
	}

	tmp := t.TempDir()
	// Fake "clone binary" — a tiny shell script that writes a marker and
	// exits 0. On Windows skip; the function under test only runs there in
	// production but the runner contract is OS-agnostic for tests.
	if runtime.GOOS == "windows" {
		t.Skip("test exercises POSIX exec path; production uses Windows path")
	}
	binary := filepath.Join(tmp, "fake-clone")
	require.NoError(t, os.WriteFile(binary, []byte("#!/bin/sh\necho 'cloned!'\nexit 0\n"), 0o755))

	s := newNativeCloneState()
	runNativeClone(context.Background(), runNativeCloneRequest{
		Binary: binary,
		Dir:    tmp,
		Env:    os.Environ(),
		State:  s,
	})

	state, err := s.wait()
	require.NoError(t, err)
	assert.Equal(t, 0, state.ExitCode)
	assert.True(t, state.Exited)

	r := s.tail()
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Contains(t, string(got), "cloned!")
}

func TestRunNativeClone_NonZeroExitCaptured(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test exercises POSIX exec path; production uses Windows path")
	}
	tmp := t.TempDir()
	binary := filepath.Join(tmp, "fake-clone")
	require.NoError(t, os.WriteFile(binary, []byte("#!/bin/sh\necho 'oops' >&2\nexit 17\n"), 0o755))

	s := newNativeCloneState()
	runNativeClone(context.Background(), runNativeCloneRequest{
		Binary: binary,
		Dir:    tmp,
		Env:    os.Environ(),
		State:  s,
	})

	state, err := s.wait()
	require.NoError(t, err)
	assert.Equal(t, 17, state.ExitCode)
	assert.True(t, state.Exited)
}

// TestStartStep_WindowsCloneWithoutRegisteredVolumeFails exercises the
// Windows-only routing branch added in Task 10: when a clone step arrives
// for a task that never had SetupWorkflow called (or the volume entry was
// evicted), StartStep should error out with the "no workflow volume
// registered" sentinel instead of falling through to the container path.
//
// This narrows test focus to the routing logic itself — we avoid mocking
// client.APIClient since the error path returns BEFORE any client call.
func TestStartStep_WindowsCloneWithoutRegisteredVolumeFails(t *testing.T) {
	e := &docker{
		info: system.Info{OSType: "windows", Architecture: "x86_64"},
	}
	step := &backend.Step{
		Name: "clone",
		UUID: "step-uuid",
		Type: backend.StepTypeClone,
	}

	err := e.StartStep(context.Background(), step, "task-uuid-not-registered")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no workflow volume registered for task task-uuid-not-registered")
}

// TestWaitStep_RoutesToNativeCloneState verifies WaitStep returns the
// state from a tracked nativeCloneState entry without ever touching the
// docker client (e.client is nil here — a client call would panic).
func TestWaitStep_RoutesToNativeCloneState(t *testing.T) {
	e := &docker{}
	state := newNativeCloneState()
	state.finish(&backend.State{ExitCode: 0, Exited: true})
	e.nativeClones.Store("step-uuid", state)

	step := &backend.Step{Name: "clone", UUID: "step-uuid", Type: backend.StepTypeClone}
	got, err := e.WaitStep(context.Background(), step, "task-uuid")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 0, got.ExitCode)
	assert.True(t, got.Exited)
}

// TestTailStep_RoutesToNativeCloneState verifies TailStep returns the
// tail reader from a tracked nativeCloneState entry.
func TestTailStep_RoutesToNativeCloneState(t *testing.T) {
	e := &docker{}
	state := newNativeCloneState()
	_, _ = state.output.Write([]byte("hello from native"))
	state.finish(&backend.State{ExitCode: 0, Exited: true})
	e.nativeClones.Store("step-uuid", state)

	step := &backend.Step{Name: "clone", UUID: "step-uuid", Type: backend.StepTypeClone}
	rc, err := e.TailStep(context.Background(), step, "task-uuid")
	require.NoError(t, err)
	require.NotNil(t, rc)
	defer rc.Close()

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, []byte("hello from native"), got)
}

// TestDestroyStep_RoutesToNativeCloneStateAndDeletes verifies DestroyStep
// removes the tracked entry from nativeClones and returns nil without
// touching the docker client.
func TestDestroyStep_RoutesToNativeCloneStateAndDeletes(t *testing.T) {
	e := &docker{}
	state := newNativeCloneState()
	state.finish(&backend.State{ExitCode: 0, Exited: true})
	e.nativeClones.Store("step-uuid", state)

	step := &backend.Step{Name: "clone", UUID: "step-uuid", Type: backend.StepTypeClone}
	err := e.DestroyStep(context.Background(), step, "task-uuid")
	require.NoError(t, err)

	_, ok := e.nativeClones.Load("step-uuid")
	assert.False(t, ok, "DestroyStep must remove the entry from nativeClones")
}

// TestResumeStep_NativeCloneStillTracked verifies ResumeStep returns
// nil when a Windows clone step's nativeCloneState is still in memory —
// no docker client interaction is needed; WaitStep will surface the
// state. The docker client (e.client) is nil here as a tripwire: if
// routing fails, the fall-through ContainerInspect call would panic.
func TestResumeStep_NativeCloneStillTracked(t *testing.T) {
	e := &docker{
		info: system.Info{OSType: "windows", Architecture: "x86_64"},
	}
	state := newNativeCloneState()
	state.finish(&backend.State{ExitCode: 0, Exited: true})
	e.nativeClones.Store("step-uuid", state)

	step := &backend.Step{Name: "clone", UUID: "step-uuid", Type: backend.StepTypeClone}
	err := e.ResumeStep(context.Background(), step, "task-uuid")
	require.NoError(t, err)
}

// TestResumeStep_NativeCloneStateLost verifies ResumeStep returns a
// "not resumable" error when the in-memory nativeCloneState is gone
// (e.g. agent restart). The caller must re-execute the workflow.
func TestResumeStep_NativeCloneStateLost(t *testing.T) {
	e := &docker{
		info: system.Info{OSType: "windows", Architecture: "x86_64"},
	}

	step := &backend.Step{Name: "clone", UUID: "step-uuid", Type: backend.StepTypeClone}
	err := e.ResumeStep(context.Background(), step, "task-uuid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not resumable")
}

// TestStartStep_NonWindowsCloneSkipsNativeRouting verifies the routing
// guard: a clone step on a Linux daemon must NOT enter the native path,
// even if the docker client is nil. If the guard misfires, the test would
// panic on nil-pointer deref inside startNativeClone (VolumeInspect on a
// nil client). Instead we expect the test to reach parseBackendOptions and
// downstream code, which will fail differently (nil client) — we only
// assert the native-path sentinel is NOT what we see.
func TestStartStep_NonWindowsCloneSkipsNativeRouting(t *testing.T) {
	e := &docker{
		info: system.Info{OSType: "linux", Architecture: "x86_64"},
	}
	step := &backend.Step{
		Name: "clone",
		UUID: "step-uuid",
		Type: backend.StepTypeClone,
	}

	// We expect this to panic or fail with a nil-client error, NOT with
	// the "no workflow volume registered" sentinel.
	defer func() {
		if r := recover(); r != nil {
			// Panic on nil client is expected — confirms we got past the
			// Windows guard into container code paths.
			return
		}
	}()
	err := e.StartStep(context.Background(), step, "task-uuid")
	if err != nil {
		assert.NotContains(t, err.Error(), "no workflow volume registered",
			"non-Windows clone steps must not enter the native plugin-clone routing")
	}
}
