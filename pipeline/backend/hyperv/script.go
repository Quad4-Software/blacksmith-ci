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
	"fmt"
	"maps"
	"slices"
	"strings"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// winJoin joins Windows path components.
//
// Neither path.Join nor filepath.Join is right here: the agent may run on a
// Windows host but the tests do not, and these are guest paths either way, so
// the separator has to be a constant rather than the host's.
//
// Trimming the separator off each component leaves a drive-qualified root as
// "C:", which then rejoins as `C:\crow` rather than the `C:crow` that would
// mean "crow relative to the current directory on C:".
func winJoin(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.Trim(p, `\`); p != "" {
			cleaned = append(cleaned, p)
		}
	}
	return strings.Join(cleaned, `\`)
}

// guestPaths is a workflow's directory layout inside the guest.
type guestPaths struct {
	Base      string
	Workspace string
	Tmp       string
	Home      string
	Scripts   string
}

// newGuestPaths derives the layout from the configured base directory.
//
// Unlike the tart backend this does not probe the guest for a home directory:
// the account PowerShell Direct authenticates as is an administrator by
// necessity, so a fixed path off the system drive is always writable and one
// fewer round trip before the first step.
func newGuestPaths(base string) guestPaths {
	return guestPaths{
		Base:      base,
		Workspace: winJoin(base, "workspace"),
		Tmp:       winJoin(base, "tmp"),
		Home:      winJoin(base, "home"),
		Scripts:   winJoin(base, "scripts"),
	}
}

// notAllowedEnvVarOverwrites are the env vars a step's configuration may not
// set, because the backend derives them from the guest's own layout.
var notAllowedEnvVarOverwrites = []string{
	"CI_NETRC_MACHINE",
	"CI_NETRC_USERNAME",
	"CI_NETRC_PASSWORD",
	"CI_SCRIPT",
	"HOME",
	"USERPROFILE",
	"CI_WORKSPACE",
	"TEMP",
	"TMP",
}

// sortedKeys returns a map's keys in a stable order, so the same inputs always
// render the same script.
func sortedKeys(env map[string]string) []string {
	return slices.Sorted(maps.Keys(env))
}

// stepEnv returns the environment for a step's script.
//
// It is built from step.Environment alone. The agent process's environment is
// never inherited, because the agent is on the host and the step is in a
// guest: that separation is the whole point of this backend.
func stepEnv(step *types.Step, paths guestPaths) map[string]string {
	env := make(map[string]string, len(step.Environment)+5)
	for k, v := range step.Environment {
		if slices.Contains(notAllowedEnvVarOverwrites, k) {
			continue
		}
		env[k] = v
	}
	env["HOME"] = paths.Home
	env["USERPROFILE"] = paths.Home
	env["CI_WORKSPACE"] = paths.Workspace
	env["TEMP"] = paths.Tmp
	env["TMP"] = paths.Tmp
	return env
}

// buildScript renders the PowerShell script a step runs.
//
// PowerShell has no `set -e` covering native commands: ErrorActionPreference
// governs cmdlets, while an .exe that fails only sets $LASTEXITCODE. So each
// command is followed by an explicit check, and $LASTEXITCODE is reset first
// because a command that runs no native executable leaves the previous
// command's value in place, which would otherwise fail the step retroactively.
func buildScript(env map[string]string, workDir string, commands []string) string {
	var b strings.Builder
	b.WriteString("$ErrorActionPreference = 'Stop'\n")

	// Sorted so the same step always produces the same script, which keeps
	// the tests meaningful and any log diff readable.
	for _, k := range sortedKeys(env) {
		fmt.Fprintf(&b, "$env:%s = %s\n", k, psQuote(env[k]))
	}

	if workDir != "" {
		fmt.Fprintf(&b, "Set-Location -LiteralPath %s\n", psQuote(workDir))
	}

	for _, cmd := range commands {
		fmt.Fprintf(&b, "Write-Output %s\n", psQuote("+ "+cmd))
		b.WriteString("$global:LASTEXITCODE = 0\n")
		b.WriteString(cmd + "\n")
		b.WriteString("if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }\n")
	}

	b.WriteString("exit 0\n")
	return b.String()
}

// setupScript prepares the guest's directory layout. It runs once, after the
// VM answers its first Invoke-Command and before any step.
func setupScript(paths guestPaths) string {
	var b strings.Builder
	b.WriteString("$ErrorActionPreference = 'Stop'\n")
	for _, dir := range []string{paths.Workspace, paths.Tmp, paths.Home, paths.Scripts} {
		fmt.Fprintf(&b, "New-Item -ItemType Directory -Force -Path %s | Out-Null\n", psQuote(dir))
	}
	return b.String()
}

// guestReadyProbe is what a booting VM is asked until it answers. It is
// deliberately trivial: the question is whether PowerShell Direct can reach a
// logged-on runspace at all, not what the guest contains.
const guestReadyProbe = "Write-Output 'ready'\n"

// stepScriptPath is where a step's script is written inside the guest. The
// step UUID keeps concurrent service steps in one workflow from colliding.
func stepScriptPath(paths guestPaths, stepUUID string) string {
	return winJoin(paths.Scripts, sanitizeFileName(stepUUID)+".ps1")
}

// stepCodePath is where the step's exit code is parked for collection.
func stepCodePath(paths guestPaths, stepUUID string) string {
	return winJoin(paths.Scripts, sanitizeFileName(stepUUID)+".code")
}

// sanitizeFileName strips anything that could leave the scripts directory or
// upset the Windows filesystem. Step UUIDs never trip it, but a caller is not
// required to supply one.
func sanitizeFileName(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, s)
}
