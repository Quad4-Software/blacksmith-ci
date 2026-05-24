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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/pipeline/backend/types"
)

// checkGitCloneCap check if we have the git binary on hand.
func checkGitCloneCap() error {
	_, err := exec.LookPath("git")
	return err
}

// gitCloneEnv augments the clone command's environment with:
//
//  1. GIT_TERMINAL_PROMPT=0 and GCM_INTERACTIVE=Never — git and the
//     Windows-bundled Git Credential Manager refuse to interactively prompt
//     for credentials. Without these, missing or wrong credentials cause an
//     indefinite hang on a headless agent instead of a fast failure.
//
//  2. The CI_NETRC_* family from the step environment — these are added by
//     the server's pipeline compiler so the clone plugin can authenticate.
//     They get stripped from the regular subprocess env by StartStep's
//     notAllowedEnvVarOverwrites filter (which is meant to stop user pipeline
//     YAML from overriding them but accidentally also blocks the legitimate
//     server-set values). Re-add them here for the clone step specifically.
func gitCloneEnv(base []string, stepEnv map[string]string) []string {
	out := slices.Clone(base)
	out = append(out,
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=Never",
	)
	for _, k := range []string{
		"CI_NETRC_MACHINE",
		"CI_NETRC_USERNAME",
		"CI_NETRC_PASSWORD",
	} {
		if v, ok := stepEnv[k]; ok && v != "" {
			out = append(out, k+"="+v)
		}
	}
	return out
}

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
func (e *local) setupClone(state *workflowState) error {
	if e.cloneBinary != "" {
		state.cloneBinary = e.cloneBinary
		return nil
	}

	log.Info().Msg("no global 'clone' plugin installed")
	state.cloneBinary = filepath.Join(state.homeDir, "plugin-clone")
	if e.os == "windows" {
		state.cloneBinary += ".exe"
	}
	return e.downloadLatestCloneBinary(state.cloneBinary)
}

// execClone executes a clone-step locally.
func (e *local) execClone(ctx context.Context, step *types.Step, state *workflowState, env []string) error {
	if err := checkGitCloneCap(); err != nil {
		return fmt.Errorf("check for git clone capabilities failed: %w", err)
	}

	if err := e.setupClone(state); err != nil {
		return fmt.Errorf("setup clone step failed: %w", err)
	}

	if !strings.Contains(step.Image, "clone") {
		log.Warn().Msgf("the clone step image '%s' does not match the default clone image. Ignoring and using the default clone image anyway.", step.Image)
	}

	rmCmd, err := e.writeNetRC(step, state)
	if err != nil {
		return err
	}

	// Prepare command
	var cmd *exec.Cmd
	if rmCmd != "" {
		// if we have a netrc injected we have to make sure it's deleted in any case after clone was attempted
		if e.os == "windows" {
			pwsh, err := exec.LookPath("powershell.exe")
			if err != nil {
				return err
			}
			cmd = exec.CommandContext(ctx, pwsh, "-Command", fmt.Sprintf("%s ; $code=$? ; %s ; if (!$code) {[Environment]::Exit(1)}", state.cloneBinary, rmCmd))
		} else {
			cmd = exec.CommandContext(ctx, "/bin/sh", "-c", fmt.Sprintf("%s ; export code=$? ; %s ; exit $code", state.cloneBinary, rmCmd))
		}
	} else {
		// if we have NO netrc, we can just exec the clone directly
		cmd = exec.CommandContext(ctx, state.cloneBinary)
	}
	cmd.Env = gitCloneEnv(env, step.Environment)
	cmd.Dir = state.workspaceDir

	// Wrap with sandbox if enabled
	cmd, err = e.wrapCommandWithSandbox(ctx, cmd, state)
	if err != nil {
		return fmt.Errorf("could not wrap command with sandbox: %w", err)
	}

	// Capture stdout and stderr separately with stream tags
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	// Create merged output with stream tags
	e.output = createTaggedMergedPipe(stdoutPipe, stderrPipe, e.os == "windows")

	state.stepCMDsMu.Lock()
	state.stepCMDs[step.UUID] = cmd
	state.stepCMDsMu.Unlock()

	return cmd.Start()
}

// writeNetRC write a netrc file into the home dir of a given workflow state.
func (e *local) writeNetRC(step *types.Step, state *workflowState) (string, error) {
	if step.Environment["CI_NETRC_MACHINE"] == "" {
		log.Trace().Msg("no netrc to write")
		return "", nil
	}

	file := filepath.Join(state.homeDir, ".netrc")
	rmCmd := fmt.Sprintf("rm \"%s\"", file)
	if e.os == "windows" {
		file = filepath.Join(state.homeDir, "_netrc")
		rmCmd = fmt.Sprintf("del \"%s\"", file)
	}

	log.Trace().Msgf("try to write netrc to '%s'", file)
	return rmCmd, os.WriteFile(file, []byte(genNetRC(step.Environment)), 0o600)
}

// downloadLatestCloneBinary download the latest clone binary based on runtime OS and Arch
// and saves it to dest.
func (e *local) downloadLatestCloneBinary(dest string) error {
	type asset struct {
		Name               string
		BrowserDownloadURL string `json:"browser_download_url"`
	}

	type release struct {
		Assets []asset
	}

	// get latest release
	req, _ := http.NewRequest(http.MethodGet, "https://codeberg.org/api/v1/repos/crow-plugins/clone/releases/latest", nil)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not get latest release: %w", err)
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	var rel release
	if err := json.Unmarshal(raw, &rel); err != nil {
		return fmt.Errorf("could not unmarshal github response: %w", err)
	}

	for _, at := range rel.Assets {
		if strings.Contains(at.Name, e.os) && strings.Contains(at.Name, e.arch) {
			resp2, err := http.Get(at.BrowserDownloadURL)
			if err != nil {
				return fmt.Errorf("could not download clone plugin: %w", err)
			}
			defer resp2.Body.Close()

			file, err := os.Create(dest)
			if err != nil {
				return fmt.Errorf("could not create clone plugin: %w", err)
			}
			defer file.Close()

			if _, err := io.Copy(file, resp2.Body); err != nil {
				return fmt.Errorf("could not download clone plugin: %w", err)
			}
			if err := os.Chmod(dest, 0o755); err != nil {
				return err
			}

			// download successful
			log.Trace().Msgf("download of 'clone' plugin to '%s' successful", dest)
			return nil
		}
	}

	return fmt.Errorf("could not download clone plugin, binary for this os/arch not found")
}
