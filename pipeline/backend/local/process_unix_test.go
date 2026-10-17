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

//go:build darwin || linux

package local

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

func TestLocalDaemonHelper(t *testing.T) {
	mode := os.Getenv("CROW_TEST_DAEMON")
	if mode == "" {
		return
	}
	if mode == "daemon" {
		time.Sleep(120 * time.Second)
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestLocalDaemonHelper$")
	cmd.Env = append(os.Environ(), "CROW_TEST_DAEMON=daemon")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if mode == "ordinary" {
		cmd = exec.Command("/bin/sleep", "120")
	}
	if err := cmd.Start(); err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(os.Getenv("CROW_TEST_PIDFILE"), []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		os.Exit(3)
	}
	if mode == "cancel" {
		time.Sleep(120 * time.Second)
	}
	if mode == "failure" {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestWorkflowProcessCleanup(t *testing.T) {
	for _, mode := range []string{"success", "failure", "cancel", "ordinary"} {
		t.Run(mode, func(t *testing.T) {
			start := func(mode string) (*local, *workflowState, context.CancelFunc, int) {
				t.Helper()
				engine := &local{os: runtime.GOOS, sandboxLevel: sandboxLevelNone}
				state := &workflowState{baseDir: t.TempDir(), stepCMDs: make(map[string]*exec.Cmd), stepOutputs: make(map[string]io.ReadCloser)}
				engine.saveState("test", state)
				pidfile := filepath.Join(state.baseDir, "pid")
				ctx, cancel := context.WithCancelCause(context.Background())
				t.Cleanup(func() { cancel(nil) })
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLocalDaemonHelper$")
				cmd.Env = append(os.Environ(), "CROW_TEST_DAEMON="+mode, "CROW_TEST_PIDFILE="+pidfile)
				require.NoError(t, engine.startStepCmd(ctx, cmd, &types.Step{UUID: "step"}, state))
				t.Cleanup(func() { require.NoError(t, cleanupWorkflowProcesses(state)) })
				var pid int
				require.Eventually(t, func() bool {
					data, err := os.ReadFile(pidfile)
					if err != nil {
						return false
					}
					pid, err = strconv.Atoi(string(data))
					return err == nil
				}, 5*time.Second, 20*time.Millisecond)
				if mode != "cancel" {
					status, err := engine.WaitStep(ctx, &types.Step{UUID: "step"}, "test")
					require.NoError(t, err)
					if mode == "failure" {
						require.Equal(t, 1, status.ExitCode)
					} else {
						require.Zero(t, status.ExitCode)
					}
				}
				return engine, state, func() { cancel(nil) }, pid
			}
			sibling, siblingState, _, siblingPID := start("success")
			engine, state, cancel, pid := start(mode)
			require.NoError(t, unix.Kill(pid, 0))

			if mode == "cancel" {
				cancel()
				_, err := engine.WaitStep(context.Background(), &types.Step{UUID: "step"}, "test")
				require.NoError(t, err)
				require.Eventually(t, func() bool { return !processAlive(pid) }, time.Second, 20*time.Millisecond)
			}
			require.NoError(t, engine.DestroyWorkflow(context.Background(), nil, "test"))
			require.NoDirExists(t, state.baseDir)
			require.False(t, processAlive(pid), "detached daemon survived teardown")
			require.True(t, processAlive(siblingPID), "sibling workflow was killed")
			require.DirExists(t, siblingState.baseDir)
			require.NoError(t, sibling.DestroyWorkflow(context.Background(), nil, "test"))
		})
	}
}
