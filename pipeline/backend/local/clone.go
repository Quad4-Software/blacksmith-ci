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

	"codefloe.com/crowci/crow/v6/pipeline/backend/common/clone"
	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
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
	state.cloneBinary = filepath.Join(state.privateDir, "plugin-clone")
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

	netrcPath, err := clone.WriteNetRC(state.privateDir, e.os, step.Environment)
	if err != nil {
		return err
	}

	var cmd *exec.Cmd
	if netrcPath != "" {
		// Make sure the netrc is removed even if the clone fails.
		if e.os == "windows" {
			shell, err := lookupPowerShell()
			if err != nil {
				return err
			}
			script := fmt.Sprintf("& %s ; $code=$LASTEXITCODE ; Remove-Item -Force %s ; exit $code", pwshQuote(state.cloneBinary), pwshQuote(netrcPath))
			cmd = exec.CommandContext(ctx, shell, "-NoProfile", "-NonInteractive", "-Command", script)
		} else {
			rmCmd := fmt.Sprintf("rm %q", netrcPath)
			cmd = exec.CommandContext(ctx, "/bin/sh", "-c", fmt.Sprintf("%s ; export code=$? ; %s ; exit $code", state.cloneBinary, rmCmd))
		}
	} else {
		cmd = exec.CommandContext(ctx, state.cloneBinary)
	}
	cmd.Env = clone.Env(env, step.Environment)
	cmd.Dir = state.workspaceDir

	return e.startStepCmd(ctx, cmd, step, state)
}

// lookupPowerShell finds the shell the clone wrapper runs in. PowerShell 7
// (pwsh) is what the docker backend and the docs require, so it comes first;
// Windows PowerShell 5.1 ships with every Windows and is the fallback.
func lookupPowerShell() (string, error) {
	pwsh, err := exec.LookPath("pwsh.exe")
	if err == nil {
		return pwsh, nil
	}
	return exec.LookPath("powershell.exe")
}

// pwshQuote returns s as a single-quoted PowerShell string literal.
func pwshQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
