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

package local

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitCloneEnv_DisablesInteractivePrompts(t *testing.T) {
	base := []string{"HOME=/tmp/wf", "USERPROFILE=/tmp/wf", "PATH=/usr/bin"}

	out := gitCloneEnv(base, nil)

	assert.Contains(t, out, "GIT_TERMINAL_PROMPT=0",
		"GIT_TERMINAL_PROMPT=0 must be set so git fails fast on a missing credential helper instead of waiting on stdin (causes infinite hang on a non-interactive Windows host)")
	assert.Contains(t, out, "GCM_INTERACTIVE=Never",
		"GCM_INTERACTIVE=Never must be set so the Windows-bundled Git Credential Manager doesn't try to open a UI prompt on a headless service")

	// base entries preserved
	for _, want := range base {
		assert.Contains(t, out, want, "must preserve caller-supplied env")
	}
}

func TestGitCloneEnv_PassesNetrcThrough(t *testing.T) {
	stepEnv := map[string]string{
		"CI_NETRC_MACHINE":  "codefloe.com",
		"CI_NETRC_USERNAME": "alice",
		"CI_NETRC_PASSWORD": "s3cret",
		"USER_DEFINED_VAR":  "should-not-appear",
	}

	out := gitCloneEnv([]string{"HOME=/tmp/wf"}, stepEnv)

	assert.Contains(t, out, "CI_NETRC_MACHINE=codefloe.com",
		"the clone plugin needs CI_NETRC_MACHINE in its env to write a usable netrc; StartStep strips it via notAllowedEnvVarOverwrites, so we have to add it back here")
	assert.Contains(t, out, "CI_NETRC_USERNAME=alice")
	assert.Contains(t, out, "CI_NETRC_PASSWORD=s3cret")

	for _, e := range out {
		assert.NotContains(t, e, "USER_DEFINED_VAR",
			"gitCloneEnv must not be a back-door for arbitrary step env; only the CI_NETRC_* family passes through")
	}
}

func TestGitCloneEnv_SkipsAbsentNetrcVars(t *testing.T) {
	// No netrc in the step env (e.g. public-repo clone with
	// AUTHENTICATE_PUBLIC_REPOS=false). We must not append empty
	// CI_NETRC_MACHINE= entries — that's worse than absence.
	out := gitCloneEnv([]string{"HOME=/tmp/wf"}, map[string]string{})

	for _, e := range out {
		assert.NotEqual(t, "CI_NETRC_MACHINE=", e)
		assert.NotEqual(t, "CI_NETRC_USERNAME=", e)
		assert.NotEqual(t, "CI_NETRC_PASSWORD=", e)
	}
}
