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
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const (
	processMarkerKey      = "CROW_LOCAL_PROCESS_OWNER"
	processCleanupTimeout = 5 * time.Second
)

// procStat holds the /proc/<pid>/stat fields used to identify a process.
type procStat struct {
	pid     int
	state   byte
	pgrp    int
	started uint64
}

func (p procStat) zombie() bool { return p.state == 'Z' || p.state == 'X' }

type processGroup struct {
	pid     int
	started uint64
}

func startWorkflowProcess(cmd *exec.Cmd, state *workflowState) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	// A very short-lived command may already have exited. Its marked detached
	// children are still discoverable independently of the original process.
	if stat, err := readProcStat(cmd.Process.Pid); err == nil {
		state.stepCMDsMu.Lock()
		group := processGroup{cmd.Process.Pid, stat.started}
		state.processCleanups = append(state.processCleanups, func() error { return stopProcessGroup(group) })
		state.stepCMDsMu.Unlock()
	}
	return nil
}

func stopProcessGroup(group processGroup) error {
	ctx, cancel := context.WithTimeout(context.Background(), processCleanupTimeout)
	defer cancel()
	for {
		leader, err := readProcStat(group.pid)
		if err == nil && leader.started != group.started {
			// The PID now belongs to an unrelated process.
			return nil
		}
		// A missing leader means it exited; its group can still exist, and the
		// kernel does not reuse a PID while it is still in use as a group ID.
		if err := unix.Kill(-group.pid, unix.SIGKILL); err != nil && !errors.Is(err, unix.ESRCH) {
			return err
		}
		procs, err := listProcStats()
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(procs, func(p procStat) bool { return p.pgrp == group.pid && !p.zombie() }) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("workflow process group did not exit: %w", ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// A marker survives reparenting and setsid, unlike a parent PID or process
// group. This is cooperative cleanup, not containment of hostile processes.
func configureWorkflowProcess(cmd *exec.Cmd, state *workflowState) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	marker := processMarkerKey + "=" + rand.Text()
	env := cmd.Environ()
	cmd.Env = slices.DeleteFunc(env, func(value string) bool {
		return strings.HasPrefix(value, processMarkerKey+"=")
	})
	cmd.Env = append(cmd.Env, marker)
	state.stepCMDsMu.Lock()
	state.processCleanups = append(state.processCleanups, func() error { return stopMarkedProcesses([]string{marker}) })
	state.stepCMDsMu.Unlock()
	cmd.Cancel = func() error {
		// Cmd.Wait waits for this cancellation callback, keeping the command's
		// process group available while we signal its ordinary children.
		groupErr := unix.Kill(-cmd.Process.Pid, unix.SIGKILL)
		if groupErr != nil && !errors.Is(groupErr, unix.ESRCH) {
			return groupErr
		}
		err := cmd.Process.Kill()
		if err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		if cleanupErr := stopMarkedProcesses([]string{marker}); cleanupErr != nil {
			return cleanupErr
		}
		return err
	}
}

// Do not use the step's canceled context: teardown still has to complete.
// Rescanning catches children forked while the preceding snapshot was killed.
func stopMarkedProcesses(markers []string) error {
	if len(markers) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), processCleanupTimeout)
	defer cancel()
	for {
		found, err := killMarkedProcesses(markers)
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("workflow processes did not exit: %w", ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func killMarkedProcesses(markers []string) (bool, error) {
	procs, err := listProcStats()
	if err != nil {
		return false, fmt.Errorf("list workflow processes: %w", err)
	}
	found := false
	for _, proc := range procs {
		// Zombies cannot access the workspace and have no readable environment.
		if proc.pid <= 1 || proc.pid == os.Getpid() || proc.zombie() {
			continue
		}
		// Reading another user's environment fails with EACCES, and processes
		// can exit during enumeration. Never log arguments or environments.
		environ, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", proc.pid))
		if err != nil || !hasEnvironMarker(environ, markers) {
			continue
		}
		// Recheck identity immediately before signaling, rather than trusting a
		// PID captured in the earlier process snapshot.
		current, err := readProcStat(proc.pid)
		if err != nil || current.started != proc.started {
			continue
		}
		if err := unix.Kill(proc.pid, unix.SIGKILL); err != nil && !errors.Is(err, unix.ESRCH) {
			return false, fmt.Errorf("kill workflow process %d: %w", proc.pid, err)
		}
		found = true
	}
	return found, nil
}

// hasEnvironMarker matches the NUL-separated contents of /proc/<pid>/environ.
func hasEnvironMarker(environ []byte, markers []string) bool {
	for field := range bytes.SplitSeq(environ, []byte{0}) {
		if slices.Contains(markers, string(field)) {
			return true
		}
	}
	return false
}

func listProcStats() ([]procStat, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	procs := make([]procStat, 0, len(entries))
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		stat, err := readProcStat(pid)
		if err != nil {
			// The process exited after the directory was listed.
			continue
		}
		procs = append(procs, stat)
	}
	return procs, nil
}

// procStatStartField is the index of starttime (field 22 of proc(5)) counted
// from the state field, which is the first field after the command name.
const procStatStartField = 19

func readProcStat(pid int) (procStat, error) {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return procStat{}, unix.ESRCH
		}
		return procStat{}, err
	}
	return parseProcStat(pid, raw)
}

func parseProcStat(pid int, raw []byte) (procStat, error) {
	// The command name is in parentheses and may itself contain spaces or ")".
	end := bytes.LastIndexByte(raw, ')')
	if end < 0 {
		return procStat{}, fmt.Errorf("malformed stat for process %d", pid)
	}
	fields := strings.Fields(string(raw[end+1:]))
	if len(fields) <= procStatStartField || len(fields[0]) != 1 {
		return procStat{}, fmt.Errorf("malformed stat for process %d", pid)
	}
	pgrp, err := strconv.Atoi(fields[2])
	if err != nil {
		return procStat{}, fmt.Errorf("malformed stat for process %d: %w", pid, err)
	}
	started, err := strconv.ParseUint(fields[procStatStartField], 10, 64)
	if err != nil {
		return procStat{}, fmt.Errorf("malformed stat for process %d: %w", pid, err)
	}
	return procStat{pid: pid, state: fields[0][0], pgrp: pgrp, started: started}, nil
}
