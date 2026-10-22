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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crowmcp "codefloe.com/crowci/crow/v6/crow-go/mcp"
)

func TestFallbackSingleInstanceFromServerAndToken(t *testing.T) {
	cfg, err := loadConfig(filepath.Join(t.TempDir(), "missing.yaml"), "https://ci.example.com", "x")
	require.NoError(t, err)
	require.Len(t, cfg.Instances, 1)
	assert.Equal(t, "ci.example.com", cfg.Instances[0].Name)
}

func TestConfigFileWinsOverServerAndToken(t *testing.T) {
	p := filepath.Join(t.TempDir(), "mcp.yaml")
	require.NoError(t, os.WriteFile(p, []byte("instances:\n  - name: a\n    url: https://a\n    token_env: X\n"), 0o600))
	cfg, err := loadConfig(p, "https://ci.example.com", "x")
	require.NoError(t, err)
	assert.Equal(t, "a", cfg.Instances[0].Name)
}

func TestNoConfigAndNoServerIsError(t *testing.T) {
	_, err := loadConfig(filepath.Join(t.TempDir(), "missing.yaml"), "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing.yaml")
}

func TestRemoteHostWithoutInstanceListsForgeHosts(t *testing.T) {
	dir := newRepo(t)
	git(t, dir, "remote", "add", "origin", "https://github.com/a/b.git")

	reg, err := crowmcp.NewRegistry(&crowmcp.Config{Instances: []crowmcp.Instance{
		{Name: "codefloe", URL: "https://ci.codefloe.com", ForgeHosts: []string{"codefloe.com"}, TokenEnv: "X"},
	}}, crowmcp.DefaultHTTPClientFactory(false))
	require.NoError(t, err)

	_, err = reg.Resolve(t.Context(), crowmcp.TargetInput{}, gitDiscoverer{dir: dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github.com")
	assert.Contains(t, err.Error(), "codefloe.com")
}

func TestLinterConfinedToRoot(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".crow"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".crow", "a.yaml"), []byte("when:\n  - event: push\nsteps:\n  - name: x\n    image: alpine\n"), 0o600))
	secret := filepath.Join(t.TempDir(), "secret.yaml")
	require.NoError(t, os.WriteFile(secret, []byte("x: [unclosed\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.txt"), []byte("x"), 0o600))

	lint := confinedLinter(root)

	report, err := lint(t.Context(), []string{".crow/"})
	require.NoError(t, err, report)
	assert.Contains(t, report, "valid")

	for _, bad := range []string{secret, "../" + filepath.Base(filepath.Dir(secret)) + "/secret.yaml", "notes.txt"} {
		_, err := lint(t.Context(), []string{bad})
		require.Error(t, err, bad)
	}
}
