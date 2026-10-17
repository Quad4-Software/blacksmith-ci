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
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func processAlive(pid int) bool {
	stat, err := readProcStat(pid)
	return err == nil && !stat.zombie()
}

func TestParseProcStat(t *testing.T) {
	raw := []byte("4242 (we ird) cmd)) S 1 4240 4240 0 -1 4194560 100 0 0 0 1 2 0 0 20 0 1 0 987654 1000 10 0 0 0\n")
	stat, err := parseProcStat(4242, raw)
	require.NoError(t, err)
	assert.Equal(t, procStat{pid: 4242, state: 'S', pgrp: 4240, started: 987654}, stat)
	assert.False(t, stat.zombie())

	_, err = parseProcStat(1, []byte("1 (init"))
	assert.Error(t, err)
	_, err = parseProcStat(1, []byte("1 (init) S 0 1"))
	assert.Error(t, err)
}

func TestReadProcStatSelf(t *testing.T) {
	stat, err := readProcStat(os.Getpid())
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), stat.pid)
	assert.Equal(t, unix.Getpgrp(), stat.pgrp)
	assert.NotZero(t, stat.started)
}

func TestHasEnvironMarker(t *testing.T) {
	marker := processMarkerKey + "=abc"
	assert.True(t, hasEnvironMarker([]byte("PATH=/bin\x00"+marker+"\x00"), []string{marker}))
	assert.False(t, hasEnvironMarker([]byte("PATH=/bin\x00"+marker+"d\x00"), []string{marker}))
	assert.False(t, hasEnvironMarker(nil, []string{marker}))
}

func TestCancelKillsChildProcesses(t *testing.T) {
	state := &workflowState{}
	// The shell backgrounds a child that would outlive a kill of the shell alone.
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "sleep 120 & echo $! ; wait")
	configureWorkflowProcess(cmd, state)
	out, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, startWorkflowProcess(cmd, state))
	var childPID int
	_, err = fmt.Fscan(out, &childPID)
	require.NoError(t, err)
	require.True(t, processAlive(childPID))

	cancel(nil)
	require.Error(t, cmd.Wait())
	require.Eventually(t, func() bool { return !processAlive(childPID) }, 2*time.Second, 20*time.Millisecond)
	require.NoError(t, cleanupWorkflowProcesses(state))
}
