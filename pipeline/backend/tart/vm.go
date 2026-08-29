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

package tart

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// bootPollInterval is how often SetupWorkflow asks a booting VM whether its
// guest agent is answering yet. A cold boot takes roughly 26 seconds on an M4,
// so a one-second poll costs a handful of cheap failed connections.
const bootPollInterval = time.Second

// runLogLimit bounds how much of a 'tart run' child's output is kept. It exists
// so a VM that fails to boot can say why; it is not a log stream.
const runLogLimit = 64 * 1024

// ringBuffer keeps the last runLogLimit bytes written to it.
type ringBuffer struct {
	mu   sync.Mutex
	data []byte
}

func (b *ringBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if len(b.data) > runLogLimit {
		b.data = b.data[len(b.data)-runLogLimit:]
	}
	return len(p), nil
}

func (b *ringBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}

// cloneVM creates this workflow's VM from the configured image.
func (e *tart) cloneVM(ctx context.Context, image, name string) error {
	out, err := e.command(ctx, cloneArgs(image, name)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tart clone %s: %w: %s", image, err, bytes.TrimSpace(out))
	}
	return nil
}

// configureVM applies CPU and memory. It is a no-op when neither is set, in
// which case the image's own configuration is kept.
func (e *tart) configureVM(ctx context.Context, name string, cpu, memoryMB int) error {
	args := setArgs(name, cpu, memoryMB)
	if args == nil {
		return nil
	}
	out, err := e.command(ctx, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tart set: %w: %s", err, bytes.TrimSpace(out))
	}
	return nil
}

// startVM boots the VM as a child process. The VM lives exactly as long as
// this process does: killing it stops the VM.
func (e *tart) startVM(ctx context.Context, name string, out *ringBuffer) (*exec.Cmd, error) {
	cmd := e.command(ctx, runArgs(name, e.config.Network)...)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("tart run %s: %w", name, err)
	}
	return cmd, nil
}

// waitForGuestAgent polls until the VM answers, and returns the guest account's
// home directory, which every other guest path is derived from.
//
// Note that the probe runs a shell rather than a bare binary: macOS has no
// /bin/true, and tart reports a missing binary as exit 125, which would look
// like a VM that never becomes ready.
func (e *tart) waitForGuestAgent(ctx context.Context, name string, runOutput *ringBuffer) (string, error) {
	deadline := time.Now().Add(e.config.StartupTimeout)
	ticker := time.NewTicker(bootPollInterval)
	defer ticker.Stop()

	for {
		out, err := e.execGuest(ctx, name, guestHomeProbe)
		if err == nil {
			if home := strings.TrimSpace(string(out)); home != "" {
				return home, nil
			}
			return "", fmt.Errorf("VM %s answered but reported no home directory", name)
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}

		if time.Now().After(deadline) {
			return "", fmt.Errorf("VM %s did not answer 'tart exec' within %s; tart run said: %s",
				name, e.config.StartupTimeout, runOutput.String())
		}
	}
}

// destroyVM stops and deletes the VM, and waits for the run child to exit.
//
// Delete is attempted even when stop fails: a VM left registered with tart
// occupies disk until something removes it.
func (e *tart) destroyVM(ctx context.Context, state *workflowState) error {
	var errs []error

	if out, err := e.command(ctx, stopArgs(state.vm, e.config.StopTimeout)...).CombinedOutput(); err != nil {
		// A VM that was never started, or already stopped, is the normal
		// case on a failed setup, so this is not worth reporting loudly.
		log.Debug().Err(err).Str("vm", state.vm).Msgf("tart stop: %s", bytes.TrimSpace(out))
	}

	if state.runCmd != nil {
		_ = state.runCmd.Wait()
	}

	if out, err := e.command(ctx, deleteArgs(state.vm)...).CombinedOutput(); err != nil {
		errs = append(errs, fmt.Errorf("tart delete: %w: %s", err, bytes.TrimSpace(out)))
	}

	return errors.Join(errs...)
}
