// Copyright 2026 Crow CI Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mcp

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crowmcp "codefloe.com/crowci/crow/v6/crow-go/mcp"
)

func TestParseRemote(t *testing.T) {
	tests := []struct {
		raw, host, fullName string
	}{
		{"https://codefloe.com/a/b.git", "codefloe.com", "a/b"},
		{"https://codefloe.com/a/b", "codefloe.com", "a/b"},
		{"git@codefloe.com:up/b.git", "codefloe.com", "up/b"},
		{"ssh://git@codefloe.com:2222/a/b.git", "codefloe.com", "a/b"},
		{"https://user:pw@gitlab.com/group/sub/repo.git", "gitlab.com", "group/sub/repo"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			host, fullName, err := parseRemote(tt.raw)
			require.NoError(t, err)
			assert.Equal(t, tt.host, host)
			assert.Equal(t, tt.fullName, fullName)
		})
	}

	_, _, err := parseRemote("/local/path/repo")
	require.Error(t, err)
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(cmd.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "feat")
	git(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	return dir
}

func TestGitDiscovererReadsRemotesBranchCommit(t *testing.T) {
	dir := newRepo(t)
	git(t, dir, "remote", "add", "origin", "https://codefloe.com/a/b.git")
	git(t, dir, "remote", "add", "upstream", "git@codefloe.com:up/b.git")
	head := git(t, dir, "rev-parse", "HEAD")

	co, err := gitDiscoverer{dir: dir}.Discover(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "feat", co.Branch)
	assert.Equal(t, head, co.Commit)
	assert.ElementsMatch(t, []crowmcp.Remote{
		{Name: "origin", Host: "codefloe.com", FullName: "a/b"},
		{Name: "upstream", Host: "codefloe.com", FullName: "up/b"},
	}, co.Remotes)
}

func TestGitDiscovererNotARepo(t *testing.T) {
	_, err := gitDiscoverer{dir: t.TempDir()}.Discover(t.Context())
	require.Error(t, err)
}
