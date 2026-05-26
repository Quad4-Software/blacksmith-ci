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

package clone

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnv_DisablesInteractivePrompts(t *testing.T) {
	base := []string{"HOME=/tmp/wf", "PATH=/usr/bin"}

	out := Env(base, nil)

	assert.Contains(t, out, "GIT_TERMINAL_PROMPT=0")
	assert.Contains(t, out, "GCM_INTERACTIVE=Never")
	for _, e := range base {
		assert.Contains(t, out, e, "must preserve caller-supplied env")
	}
}

func TestEnv_PassesNetrcThrough(t *testing.T) {
	stepEnv := map[string]string{
		"CI_NETRC_MACHINE":  "codefloe.com",
		"CI_NETRC_USERNAME": "alice",
		"CI_NETRC_PASSWORD": "s3cret",
		"USER_DEFINED_VAR":  "should-not-appear",
	}

	out := Env([]string{"HOME=/tmp/wf"}, stepEnv)

	assert.Contains(t, out, "CI_NETRC_MACHINE=codefloe.com")
	assert.Contains(t, out, "CI_NETRC_USERNAME=alice")
	assert.Contains(t, out, "CI_NETRC_PASSWORD=s3cret")
	for _, e := range out {
		assert.NotContains(t, e, "USER_DEFINED_VAR", "must only pass CI_NETRC_* through")
	}
}

func TestEnv_SkipsAbsentNetrcVars(t *testing.T) {
	out := Env([]string{"HOME=/tmp/wf"}, map[string]string{})

	for _, e := range out {
		assert.NotEqual(t, "CI_NETRC_MACHINE=", e)
		assert.NotEqual(t, "CI_NETRC_USERNAME=", e)
		assert.NotEqual(t, "CI_NETRC_PASSWORD=", e)
	}
}

func TestGenNetRC_FormatsMachineLoginPassword(t *testing.T) {
	got := GenNetRC(map[string]string{
		"CI_NETRC_MACHINE":  "git.example.com",
		"CI_NETRC_USERNAME": "alice",
		"CI_NETRC_PASSWORD": "s3cret",
	})

	assert.Contains(t, got, "machine git.example.com")
	assert.Contains(t, got, "login alice")
	assert.Contains(t, got, "password s3cret")
}

func TestWriteNetRC_NoCreds_ReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()

	path, err := WriteNetRC(tmp, "linux", map[string]string{})

	require.NoError(t, err)
	assert.Empty(t, path, "no netrc written when machine is missing")
	entries, err := os.ReadDir(tmp)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestWriteNetRC_Linux_WritesDotNetrc(t *testing.T) {
	tmp := t.TempDir()
	stepEnv := map[string]string{
		"CI_NETRC_MACHINE":  "git.example.com",
		"CI_NETRC_USERNAME": "alice",
		"CI_NETRC_PASSWORD": "s3cret",
	}

	path, err := WriteNetRC(tmp, "linux", stepEnv)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmp, ".netrc"), path, "Linux uses .netrc")
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "machine git.example.com")
}

func TestWriteNetRC_Windows_WritesUnderscoreNetrc(t *testing.T) {
	tmp := t.TempDir()
	stepEnv := map[string]string{
		"CI_NETRC_MACHINE":  "git.example.com",
		"CI_NETRC_USERNAME": "alice",
		"CI_NETRC_PASSWORD": "s3cret",
	}

	path, err := WriteNetRC(tmp, "windows", stepEnv)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmp, "_netrc"), path,
		"Windows uses _netrc because that's what curl/libcurl looks for first")
}

func TestWriteNetRC_RestrictivePerms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file perms work differently; assertion below is POSIX-specific")
	}
	tmp := t.TempDir()
	stepEnv := map[string]string{
		"CI_NETRC_MACHINE":  "git.example.com",
		"CI_NETRC_USERNAME": "alice",
		"CI_NETRC_PASSWORD": "s3cret",
	}

	path, err := WriteNetRC(tmp, "linux", stepEnv)

	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"git refuses to read netrc with looser permissions on POSIX")
}

func TestCheckGitAvailable_OnSystemsWithGit(t *testing.T) {
	// Test machines always have git on PATH (otherwise the whole repo
	// can't be built). Just verify the function returns nil here.
	err := CheckGitAvailable()
	assert.NoError(t, err)
}

func TestEnsureBinary_DownloadsWhenMissing(t *testing.T) {
	// Mock the releases API and the asset download with two paired servers.
	expectedBody := []byte("fake-binary-bytes")
	assetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(expectedBody)
	}))
	defer assetSrv.Close()

	releasesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"assets":[{"name":"plugin-clone-test-os-test-arch","browser_download_url":"` + assetSrv.URL + `"}]}`))
	}))
	defer releasesSrv.Close()

	tmp := t.TempDir()
	dest := filepath.Join(tmp, "plugin-clone")

	err := EnsureBinary(context.Background(), EnsureBinaryConfig{
		Dest:        dest,
		OS:          "test-os",
		Arch:        "test-arch",
		ReleasesURL: releasesSrv.URL,
	})

	require.NoError(t, err)
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, expectedBody, got)
}

func TestEnsureBinary_NoMatchingAsset(t *testing.T) {
	releasesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"assets":[{"name":"plugin-clone-other-os-other-arch","browser_download_url":"http://example.invalid"}]}`))
	}))
	defer releasesSrv.Close()

	err := EnsureBinary(context.Background(), EnsureBinaryConfig{
		Dest:        filepath.Join(t.TempDir(), "plugin-clone"),
		OS:          "test-os",
		Arch:        "test-arch",
		ReleasesURL: releasesSrv.URL,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "test-os")
	assert.Contains(t, err.Error(), "test-arch")
}
