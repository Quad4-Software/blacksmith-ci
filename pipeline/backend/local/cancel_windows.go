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

//go:build windows

package local

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// cancelWaitDelay bounds how long Wait blocks on the output pipes after the
// step was canceled. A grandchild that survived the kill holds copies of
// them; without a delay Wait would block until it exits on its own.
const cancelWaitDelay = 10 * time.Second

// configureCancel makes canceling a step kill everything it started.
//
// The default cancel only terminates the shell the step runs in. Windows has
// no process groups, so a build tool that shell spawned keeps running after
// the step was canceled, and keeps the workspace files open the teardown
// then cannot delete. The /T flag of taskkill walks the tree the shell started.
func configureCancel(cmd *exec.Cmd) {
	cmd.WaitDelay = cancelWaitDelay
	cmd.Cancel = func() error {
		pid := strconv.Itoa(cmd.Process.Pid)
		if err := exec.Command("taskkill", "/T", "/F", "/PID", pid).Run(); err == nil {
			return nil
		}
		// taskkill also fails when the process is already gone; fall back to
		// the plain kill, which reports that case as os.ErrProcessDone.
		err := cmd.Process.Kill()
		if errors.Is(err, os.ErrProcessDone) {
			return os.ErrProcessDone
		}
		return err
	}
}
