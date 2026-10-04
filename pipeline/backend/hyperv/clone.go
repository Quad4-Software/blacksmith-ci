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

package hyperv

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/pipeline/backend/common/clone"
	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// cloneBinaryName is what the clone plugin is called, on the host cache and in
// the guest alike.
const cloneBinaryName = "plugin-clone.exe"

// netrcFileName is _netrc rather than .netrc: that is what curl and libcurl
// look for first on Windows, and git's own fallback to .netrc varies by build.
const netrcFileName = "_netrc"

// cloneStepScript prepares the clone step and returns the script to run.
//
// The binary is fetched and cached on the host, then copied into the guest,
// rather than downloaded inside each guest. A throwaway VM caches nothing, so
// a guest-side download would cost a fresh ~50 MB transfer every workflow and
// would make the clone step depend on the guest reaching the release host.
func (e *hyperv) cloneStepScript(ctx context.Context, step *types.Step, state *workflowState) (string, error) {
	if !strings.Contains(step.Image, "clone") {
		log.Warn().Msgf("the clone step image '%s' does not match the default clone image. Ignoring and using the default clone image anyway.", step.Image)
	}

	clonePath, err := e.ensureGuestCloneBinary(ctx, state)
	if err != nil {
		return "", err
	}

	var netrc string
	if step.Environment["CI_NETRC_MACHINE"] != "" {
		netrc = clone.GenNetRC(step.Environment)
	}

	return cloneScript(cloneEnv(step.Environment, state.paths), state.paths, clonePath, netrc), nil
}

// cloneEnv is the clone step's environment.
//
// It is the usual step environment plus the two things only a clone needs: the
// CI_NETRC_* family, which every other step has stripped, and the prompt guards
// that stop git blocking forever on a headless agent when a credential is wrong.
func cloneEnv(stepEnvironment map[string]string, paths guestPaths) map[string]string {
	env := stepEnv(&types.Step{Environment: stepEnvironment}, paths)
	env["GIT_TERMINAL_PROMPT"] = "0"
	env["GCM_INTERACTIVE"] = "Never"
	for _, k := range []string{"CI_NETRC_MACHINE", "CI_NETRC_USERNAME", "CI_NETRC_PASSWORD"} {
		if v := stepEnvironment[k]; v != "" {
			env[k] = v
		}
	}
	return env
}

// ensureGuestCloneBinary makes the clone plugin available in the guest and
// returns its guest path. An image that already ships one is used as-is.
func (e *hyperv) ensureGuestCloneBinary(ctx context.Context, state *workflowState) (string, error) {
	guestPath := winJoin(state.paths.Base, cloneBinaryName)

	probe := fmt.Sprintf("if (Get-Command %s -ErrorAction SilentlyContinue) { Write-Output 'yes' }\n",
		psQuote(cloneBinaryName))
	if out, err := e.execGuest(ctx, state.vm, probe); err == nil {
		if strings.TrimSpace(string(out)) == "yes" {
			log.Debug().Msg("using the clone plugin shipped with the VM image")
			return cloneBinaryName, nil
		}
	}

	hostPath, err := e.hostCloneBinary(ctx)
	if err != nil {
		return "", err
	}

	// Copy-VMFile rather than the remoting channel the step scripts use: the
	// binary is tens of megabytes, which is what this integration service is
	// for and what PowerShell remoting's object serialization is not.
	script := psPreamble + fmt.Sprintf(
		"Copy-VMFile -Name %s -SourcePath %s -DestinationPath %s -CreateFullPath -FileSource Host -Force\n",
		psQuote(state.vm), psQuote(hostPath), psQuote(guestPath))
	if out, err := e.run(ctx, script); err != nil {
		return "", fmt.Errorf("copy the clone plugin into the guest: %w: %s", err, bytes.TrimSpace(out))
	}

	return guestPath, nil
}

// hostCloneBinary returns the path of the host's cached clone plugin,
// downloading it once if it is not there yet.
//
// The cache is what makes the host-side fetch worth doing: it is shared by
// every workflow this agent runs, while each guest is discarded.
func (e *hyperv) hostCloneBinary(ctx context.Context) (string, error) {
	cacheDir := filepath.Join(os.TempDir(), "crow-hyperv-clone")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return "", err
	}
	dest := filepath.Join(cacheDir, cloneBinaryName)
	return dest, clone.EnsureBinary(ctx, clone.EnsureBinaryConfig{
		Dest: dest,
		// The guest is a Windows VM on a Windows host, so the host's own
		// architecture is also the guest's.
		OS:   "windows",
		Arch: hostArch(),
	})
}

// cloneScript runs the clone plugin with a netrc that is removed afterwards,
// whatever the clone's outcome, while still reporting the clone's exit code.
func cloneScript(env map[string]string, paths guestPaths, clonePath, netrc string) string {
	var b strings.Builder
	b.WriteString("$ErrorActionPreference = 'Stop'\n")

	for _, k := range sortedKeys(env) {
		fmt.Fprintf(&b, "$env:%s = %s\n", k, psQuote(env[k]))
	}

	fmt.Fprintf(&b, "New-Item -ItemType Directory -Force -Path %s | Out-Null\n", psQuote(paths.Workspace))
	fmt.Fprintf(&b, "Set-Location -LiteralPath %s\n", psQuote(paths.Workspace))

	netrcPath := winJoin(paths.Home, netrcFileName)
	if netrc != "" {
		fmt.Fprintf(&b, "Set-Content -LiteralPath %s -Value %s -Encoding ASCII\n",
			psQuote(netrcPath), psDecode(netrc))
	}

	// The netrc has to be removed whether the clone succeeded or not, so the
	// exit code is captured rather than allowed to end the script.
	b.WriteString("$global:LASTEXITCODE = 0\n")
	fmt.Fprintf(&b, "& %s\n$crowCloneCode = $LASTEXITCODE\n", psQuote(clonePath))
	if netrc != "" {
		fmt.Fprintf(&b, "Remove-Item -LiteralPath %s -Force -ErrorAction SilentlyContinue\n", psQuote(netrcPath))
	}
	b.WriteString("exit $crowCloneCode\n")

	return b.String()
}
