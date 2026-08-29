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
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"al.essio.dev/pkg/shellescape"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// guestShell is the shell a step runs under when it does not name one.
const guestShell = "/bin/sh"

// guestBaseName is the directory, relative to the guest account's home, that
// holds everything a workflow writes. The VM's destruction takes it with it.
const guestBaseName = "crow"

// guestPaths is a workflow's directory layout inside the guest, as absolute
// paths.
type guestPaths struct {
	Base      string
	Workspace string
	Tmp       string
	Home      string
}

// newGuestPaths derives the layout from the guest account's home directory.
func newGuestPaths(guestHome string) guestPaths {
	base := path.Join(guestHome, guestBaseName)
	return guestPaths{
		Base:      base,
		Workspace: path.Join(base, "workspace"),
		Tmp:       path.Join(base, "tmp"),
		Home:      path.Join(base, "home"),
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
	"SHELL",
	"CI_WORKSPACE",
	"TMPDIR",
}

// sortedKeys returns a map's keys in a stable order, so the same inputs always
// render the same script. Shared with the clone step's own script builder.
func sortedKeys(env map[string]string) []string {
	return slices.Sorted(maps.Keys(env))
}

// stepShell returns the shell a step runs under.
//
// As in the local backend, step.Image names the shell rather than a container
// image: steps share one VM, so the VM image cannot be a per-step choice. It
// is configured through backend_options.tart.image instead.
func stepShell(step *types.Step) string {
	if step.Image == "" {
		return guestShell
	}
	return step.Image
}

// stepEnv returns the environment for a step's script.
//
// It is built from step.Environment alone. The agent process's environment is
// never inherited, because the agent is on the host and the step is in a
// guest: that separation is the whole point of this backend.
func stepEnv(step *types.Step, paths guestPaths) map[string]string {
	env := make(map[string]string, len(step.Environment)+3)
	for k, v := range step.Environment {
		if slices.Contains(notAllowedEnvVarOverwrites, k) {
			continue
		}
		env[k] = v
	}
	env["HOME"] = paths.Home
	env["CI_WORKSPACE"] = paths.Workspace
	env["TMPDIR"] = paths.Tmp
	return env
}

// buildScript renders the POSIX script a step's shell reads from stdin.
//
// Nothing here is passed in argv, so step secrets never appear in the guest's
// process list.
func buildScript(env map[string]string, workDir string, commands []string) string {
	var b strings.Builder
	b.WriteString("set -e\n")

	// Sorted so the same step always produces the same script, which keeps
	// the tests meaningful and any log diff readable.
	for _, k := range sortedKeys(env) {
		fmt.Fprintf(&b, "export %s=%s\n", k, shellescape.Quote(env[k]))
	}

	if workDir != "" {
		fmt.Fprintf(&b, "cd %s\n", shellescape.Quote(workDir))
	}

	for _, cmd := range commands {
		fmt.Fprintf(&b, "echo %s\n%s\n", shellescape.Quote("+ "+cmd), cmd)
	}

	return b.String()
}

// setupScript prepares the guest's directory layout. It runs once, after the
// VM answers its first exec and before any step.
func setupScript(paths guestPaths) string {
	return fmt.Sprintf("set -e\nmkdir -p %s %s %s\n",
		shellescape.Quote(paths.Workspace),
		shellescape.Quote(paths.Tmp),
		shellescape.Quote(paths.Home),
	)
}
