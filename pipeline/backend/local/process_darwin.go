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
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type processGroup struct {
	pid     int
	started unix.Timeval
}

func startWorkflowProcess(cmd *exec.Cmd, state *workflowState) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	// A very short-lived command may already have exited. Its marked detached
	// children are still discoverable independently of the original process.
	proc, err := unix.SysctlKinfoProc("kern.proc.pid", cmd.Process.Pid)
	if err == nil {
		state.stepCMDsMu.Lock()
		group := processGroup{cmd.Process.Pid, proc.Proc.P_starttime}
		state.processCleanups = append(state.processCleanups, func() error { return stopProcessGroup(group) })
		state.stepCMDsMu.Unlock()
	}
	return nil
}

func stopProcessGroup(group processGroup) error {
	ctx, cancel := context.WithTimeout(context.Background(), processCleanupTimeout)
	defer cancel()
	for {
		leaders, err := unix.SysctlKinfoProcSlice("kern.proc.pid", group.pid)
		if err != nil {
			return err
		}
		if len(leaders) > 0 && leaders[0].Proc.P_starttime != group.started {
			return nil
		}
		// An empty result means the leader exited; its group can still exist.
		if err := unix.Kill(-group.pid, unix.SIGKILL); err != nil && !errors.Is(err, unix.ESRCH) {
			return err
		}
		members, err := unix.SysctlKinfoProcSlice("kern.proc.pgrp", group.pid)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(members, func(proc unix.KinfoProc) bool { return proc.Proc.P_stat != processZombieState }) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("workflow process group did not exit: %w", ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

const (
	processMarkerKey      = "CROW_LOCAL_PROCESS_OWNER"
	processZombieState    = 5
	processCleanupTimeout = 5 * time.Second
	procArgsHeaderSize    = 4
)

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
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.uid", os.Geteuid())
	if err != nil {
		return false, fmt.Errorf("list workflow processes: %w", err)
	}
	found := false
	for _, proc := range procs {
		pid := int(proc.Proc.P_pid)
		// Zombies cannot access the workspace and have no readable environment.
		if pid <= 1 || pid == os.Getpid() || proc.Proc.P_stat == processZombieState {
			continue
		}
		raw, err := unix.SysctlRaw("kern.procargs2", pid)
		if err != nil {
			// Processes can exit during enumeration; macOS can also hide their
			// environment. Never log arguments or environments.
			if errors.Is(err, unix.ESRCH) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
				continue
			}
			return false, fmt.Errorf("read process %d ownership: %w", pid, err)
		}
		if !hasProcessMarker(raw, markers) {
			continue
		}
		// Recheck identity immediately before signaling, rather than trusting a
		// PID captured in the earlier process snapshot.
		current, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
		if err != nil || current.Proc.P_starttime != proc.Proc.P_starttime {
			continue
		}
		if err := unix.Kill(pid, unix.SIGKILL); err != nil && !errors.Is(err, unix.ESRCH) {
			return false, fmt.Errorf("kill workflow process %d: %w", pid, err)
		}
		found = true
	}
	return found, nil
}

// KERN_PROCARGS2 contains argc, the executable path, padding, argv and then
// the NUL-separated environment. Only match environment entries, not argv.
func hasProcessMarker(raw []byte, markers []string) bool {
	if len(raw) < procArgsHeaderSize {
		return false
	}
	argc := int(binary.NativeEndian.Uint32(raw[:4]))
	raw = raw[4:]
	end := bytes.IndexByte(raw, 0)
	if end < 0 {
		return false
	}
	raw = bytes.TrimLeft(raw[end+1:], "\x00")
	for range argc {
		end = bytes.IndexByte(raw, 0)
		if end < 0 {
			return false
		}
		raw = raw[end+1:]
	}
	for field := range bytes.SplitSeq(raw, []byte{0}) {
		if slices.Contains(markers, string(field)) {
			return true
		}
	}
	return false
}
