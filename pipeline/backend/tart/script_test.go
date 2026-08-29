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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// testPaths is the layout of the stock cirruslabs image, whose 'tart exec'
// account is admin.
var testPaths = newGuestPaths("/Users/admin")

func TestNewGuestPathsIsAbsolute(t *testing.T) {
	// Paths must never be written as a literal "$HOME/...": they are
	// shell-quoted when exported or changed into, and a quoted $HOME is not
	// expanded by the guest.
	for _, p := range []string{testPaths.Base, testPaths.Workspace, testPaths.Tmp, testPaths.Home} {
		assert.True(t, strings.HasPrefix(p, "/"), "%q must be absolute", p)
		assert.NotContains(t, p, "$")
	}
	assert.Equal(t, "/Users/admin/crow/workspace", testPaths.Workspace)
}

func TestStepShellDefaultsToSh(t *testing.T) {
	assert.Equal(t, "/bin/sh", stepShell(&types.Step{}))
	assert.Equal(t, "/bin/bash", stepShell(&types.Step{Image: "/bin/bash"}))
}

func TestStepEnvDropsProtectedOverrides(t *testing.T) {
	env := stepEnv(&types.Step{
		Environment: map[string]string{
			"CI_COMMIT_SHA": "abc",
			"HOME":          "/tmp/evil",
			"CI_WORKSPACE":  "/tmp/evil",
			"TMPDIR":        "/tmp/evil",
		},
	}, testPaths)

	assert.Equal(t, "abc", env["CI_COMMIT_SHA"])
	assert.Equal(t, testPaths.Workspace, env["CI_WORKSPACE"])
	assert.Equal(t, testPaths.Home, env["HOME"])
	assert.Equal(t, testPaths.Tmp, env["TMPDIR"])
}

func TestBuildScriptExportsQuotesAndTraces(t *testing.T) {
	got := buildScript(
		map[string]string{"MSG": "hello 'world'"},
		testPaths.Workspace,
		[]string{"echo one", "echo two"},
	)

	assert.True(t, strings.HasPrefix(got, "set -e\n"), "script must abort on the first failing command")
	assert.Contains(t, got, `export MSG='hello '"'"'world'"'"''`)
	assert.Contains(t, got, "cd /Users/admin/crow/workspace\n")
	assert.Contains(t, got, "echo '+ echo one'")
	assert.Contains(t, got, "\necho one\n")
	// Commands keep their pipeline order.
	assert.Less(t, strings.Index(got, "echo '+ echo one'"), strings.Index(got, "echo '+ echo two'"))
}

func TestBuildScriptQuotesAwkwardWorkDir(t *testing.T) {
	got := buildScript(nil, "/Users/admin/my builds", nil)
	assert.Contains(t, got, `cd '/Users/admin/my builds'`)
}

func TestBuildScriptSortsEnvDeterministically(t *testing.T) {
	env := map[string]string{"B": "2", "A": "1", "C": "3"}
	first := buildScript(env, "/w", nil)
	for range 5 {
		assert.Equal(t, first, buildScript(env, "/w", nil))
	}
	assert.Less(t, strings.Index(first, "export A="), strings.Index(first, "export B="))
}

func TestSetupScriptCreatesGuestDirs(t *testing.T) {
	got := setupScript(testPaths)
	assert.Contains(t, got, "mkdir -p")
	assert.Contains(t, got, testPaths.Workspace)
	assert.Contains(t, got, testPaths.Tmp)
	assert.Contains(t, got, testPaths.Home)
}
