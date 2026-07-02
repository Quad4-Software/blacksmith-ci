// Copyright 2023 Woodpecker Authors
// Copyright 2025 Crow Authors
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
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/pipeline/backend/common/clone"
	"codefloe.com/crowci/crow/v5/pipeline/backend/types"
)

// loadClone on backend start determine if there is a global plugin-clone binary.
func (e *local) loadClone() {
	binary, err := exec.LookPath("plugin-clone")
	if err != nil || binary == "" {
		// could not found global clone plugin, just ignore it
		return
	}
	e.cloneBinary = binary
}

// setupClone prepare the clone environment before exec.
func (e *local) setupClone(ctx context.Context, state *workflowState) error {
	if e.cloneBinary != "" {
		state.cloneBinary = e.cloneBinary
		return nil
	}

	log.Info().Msg("no global 'clone' plugin installed")
	state.cloneBinary = filepath.Join(state.homeDir, "plugin-clone")
	if e.os == "windows" {
		state.cloneBinary += ".exe"
	}
	return clone.EnsureBinary(ctx, clone.EnsureBinaryConfig{
		Dest: state.cloneBinary,
		OS:   e.os,
		Arch: e.arch,
	})
}

// execClone executes a clone-step locally.
func (e *local) execClone(ctx context.Context, step *types.Step, state *workflowState, env []string) error {
	if err := clone.CheckGitAvailable(); err != nil {
		return fmt.Errorf("check for git clone capabilities failed: %w", err)
	}

	if err := e.setupClone(ctx, state); err != nil {
		return fmt.Errorf("setup clone step failed: %w", err)
	}

	if !strings.Contains(step.Image, "clone") {
		log.Warn().Msgf("the clone step image '%s' does not match the default clone image. Ignoring and using the default clone image anyway.", step.Image)
	}

	netrcPath, err := clone.WriteNetRC(state.homeDir, e.os, step.Environment)
	if err != nil {
		return err
	}

	var cmd *exec.Cmd
	if netrcPath != "" {
		// Make sure the netrc is removed even if the clone fails.
		rmCmd := fmt.Sprintf("rm %q", netrcPath)
		if e.os == "windows" {
			rmCmd = fmt.Sprintf("del %q", netrcPath)
			pwsh, err := exec.LookPath("powershell.exe")
			if err != nil {
				return err
			}
			cmd = exec.CommandContext(ctx, pwsh, "-Command", fmt.Sprintf("%s ; $code=$? ; %s ; if (!$code) {[Environment]::Exit(1)}", state.cloneBinary, rmCmd))
		} else {
			cmd = exec.CommandContext(ctx, "/bin/sh", "-c", fmt.Sprintf("%s ; export code=$? ; %s ; exit $code", state.cloneBinary, rmCmd))
		}
	} else {
		cmd = exec.CommandContext(ctx, state.cloneBinary)
	}
	cmd.Env = clone.Env(env, step.Environment)
	cmd.Dir = state.workspaceDir

	cmd, err = e.wrapCommandWithSandbox(ctx, cmd, state)
	if err != nil {
		return fmt.Errorf("could not wrap command with sandbox: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("could not create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("could not create stderr pipe: %w", err)
	}
	output := createTaggedMergedPipe(stdoutPipe, stderrPipe, e.os == "windows")

	state.stepCMDsMu.Lock()
	state.stepCMDs[step.UUID] = cmd
	state.stepOutputs[step.UUID] = output
	state.stepCMDsMu.Unlock()

	return cmd.Start()
}
