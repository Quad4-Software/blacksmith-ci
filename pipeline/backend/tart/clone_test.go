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
)

func TestCloneScriptWritesAndRemovesNetrc(t *testing.T) {
	got := cloneScript(
		map[string]string{"CI_REPO_CLONE_URL": "https://example.com/o/r.git"},
		testPaths,
		"/Users/admin/crow/plugin-clone",
		"machine example.com\nlogin u\npassword p\n",
	)

	assert.Contains(t, got, "export CI_REPO_CLONE_URL=https://example.com/o/r.git\n")
	assert.Contains(t, got, "cat > /Users/admin/crow/home/.netrc <<'"+netrcHeredocTag+"'")
	assert.Contains(t, got, "chmod 600")
	// The netrc must go regardless of how the clone ends, and the clone's
	// own exit code must still be what the step reports.
	assert.Contains(t, got, "rm -f /Users/admin/crow/home/.netrc")
	assert.Contains(t, got, "exit $code")
	assert.Less(t, strings.Index(got, "rm -f"), strings.Index(got, "exit $code"))
}

func TestCloneScriptWithoutNetrc(t *testing.T) {
	got := cloneScript(nil, testPaths, "/Users/admin/crow/plugin-clone", "")
	assert.NotContains(t, got, ".netrc")
	assert.Contains(t, got, "exit $code")
}

func TestCloneScriptRunsTheBinary(t *testing.T) {
	got := cloneScript(nil, testPaths, "/Users/admin/crow/plugin-clone", "")
	assert.Contains(t, got, "\n/Users/admin/crow/plugin-clone\n")
	assert.Contains(t, got, "cd /Users/admin/crow/workspace\n")
}

func TestCloneScriptQuotesAwkwardPaths(t *testing.T) {
	// shellescape only adds quotes where they are needed, so prove they do
	// appear for a home a person might plausibly have.
	paths := newGuestPaths("/Users/my admin")
	got := cloneScript(nil, paths, "/Users/my admin/crow/plugin-clone", "machine x\n")

	assert.Contains(t, got, "cd '/Users/my admin/crow/workspace'")
	assert.Contains(t, got, "'/Users/my admin/crow/plugin-clone'")
	assert.Contains(t, got, "rm -f '/Users/my admin/crow/home/.netrc'")
}

func TestPushBinaryScriptDecodesAndMarksExecutable(t *testing.T) {
	got := pushBinaryScript("/Users/admin/crow/plugin-clone", "cGF5bG9hZA==")

	assert.Contains(t, got, "mkdir -p /Users/admin/crow\n")
	assert.Contains(t, got, "base64 -d > /Users/admin/crow/plugin-clone <<'"+cloneHeredocTag+"'\n")
	assert.Contains(t, got, "\ncGF5bG9hZA==\n")
	assert.Contains(t, got, "chmod +x /Users/admin/crow/plugin-clone\n")
	// The payload must be terminated before anything else runs, or the
	// chmod would be read as more base64.
	assert.Less(t, strings.Index(got, "\n"+cloneHeredocTag+"\nchmod"), strings.Index(got, "chmod +x"))
}

func TestCloneEnvCarriesNetrcAndPromptGuards(t *testing.T) {
	env := cloneEnv(map[string]string{
		"CI_NETRC_MACHINE":  "example.com",
		"CI_NETRC_USERNAME": "u",
		"CI_NETRC_PASSWORD": "p",
		"CI_REPO_CLONE_URL": "https://example.com/o/r.git",
	}, testPaths)

	// StartStep strips the CI_NETRC_* family from every other step; the clone
	// plugin is the one thing that legitimately needs it.
	assert.Equal(t, "example.com", env["CI_NETRC_MACHINE"])
	assert.Equal(t, "p", env["CI_NETRC_PASSWORD"])
	// Without these a missing credential hangs a headless agent forever
	// instead of failing.
	assert.Equal(t, "0", env["GIT_TERMINAL_PROMPT"])
	assert.Equal(t, "Never", env["GCM_INTERACTIVE"])
	assert.Equal(t, testPaths.Home, env["HOME"])
}
