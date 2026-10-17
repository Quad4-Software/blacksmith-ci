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

package local

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

func processAlive(pid int) bool {
	proc, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	return err == nil && proc.Proc.P_stat != 5
}

func TestHasProcessMarker(t *testing.T) {
	marker := processMarkerKey + "=unique"
	for _, tc := range []struct {
		name, arg, env string
		want           bool
	}{
		{"environment", "arg", marker, true},
		{"argument is not ownership", marker, "OTHER=value", false},
		{"substring is not ownership", "arg", "OTHER=" + marker, false},
		{"other workflow", "arg", processMarkerKey + "=other", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := binary.NativeEndian.AppendUint32(nil, 2)
			raw = append(raw, []byte(fmt.Sprintf("/bin/test%c%c/bin/test%c%s%c%s%c", 0, 0, 0, tc.arg, 0, tc.env, 0))...)
			require.Equal(t, tc.want, hasProcessMarker(raw, []string{marker}))
		})
	}
	require.False(t, hasProcessMarker(nil, []string{marker}))
	require.False(t, hasProcessMarker([]byte(strings.Repeat("x", 20)), []string{marker}))
}

func TestWorkflowCleansSccache(t *testing.T) {
	binary, err := exec.LookPath("sccache")
	if err != nil {
		t.Skip("sccache is not installed")
	}
	// Use a short path: Darwin's socket name limit includes the full path.
	base, err := os.MkdirTemp("/tmp", "crow-local-sc-") //nolint:usetesting // t.TempDir exceeds Darwin's Unix socket path limit.
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(base)) })
	state := &workflowState{baseDir: base, stepCMDs: make(map[string]*exec.Cmd), stepOutputs: make(map[string]io.ReadCloser)}
	engine := &local{os: "darwin", sandboxLevel: sandboxLevelNone}
	engine.saveState("sccache", state)
	cmd := exec.CommandContext(context.Background(), binary, "--start-server")
	// No inherited storage credentials or host-wide daemon endpoint.
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + base, "TMPDIR=" + base, "SCCACHE_DIR=" + filepath.Join(base, "cache"), "SCCACHE_SERVER_UDS=" + filepath.Join(base, "s")}
	require.NoError(t, engine.startStepCmd(context.Background(), cmd, &types.Step{UUID: "sccache"}, state))
	t.Cleanup(func() { require.NoError(t, cleanupWorkflowProcesses(state)) })
	status, err := engine.WaitStep(context.Background(), &types.Step{UUID: "sccache"}, "sccache")
	require.NoError(t, err)
	require.Zero(t, status.ExitCode)
	// The launcher has exited; only the detached daemon retains this marker.
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.uid", os.Geteuid())
	require.NoError(t, err)
	var daemonPID int
	for _, proc := range procs {
		raw, err := unix.SysctlRaw("kern.procargs2", int(proc.Proc.P_pid))
		if err == nil && hasProcessMarker(raw, []string{processMarkerFromCommand(cmd)}) {
			daemonPID = int(proc.Proc.P_pid)
			break
		}
	}
	require.NotZero(t, daemonPID, "detached sccache daemon was not discoverable")
	require.NoError(t, engine.DestroyWorkflow(context.Background(), nil, "sccache"))
	require.False(t, processAlive(daemonPID))
	require.NoDirExists(t, base)
}

func processMarkerFromCommand(cmd *exec.Cmd) string {
	for _, entry := range cmd.Env {
		if strings.HasPrefix(entry, processMarkerKey+"=") {
			return entry
		}
	}
	return ""
}
