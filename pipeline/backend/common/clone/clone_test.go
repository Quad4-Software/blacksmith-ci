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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

const testAssetName = "plugin-clone-test-os-test-arch"

// releaseServers spins up a paired asset server and releases-metadata server.
// The metadata advertises both the platform binary and a checksums.txt whose
// body is produced by checksumsBody(binaryDigest).
func releaseServers(t *testing.T, binaryBody []byte, checksumsBody func(binaryDigest string) string) (releasesURL string) {
	t.Helper()

	binaryDigest := sha256Hex(binaryBody)
	checksums := checksumsBody(binaryDigest)

	assetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(binaryBody)
	}))
	t.Cleanup(assetSrv.Close)

	checksumsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(checksums))
	}))
	t.Cleanup(checksumsSrv.Close)

	releasesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := fmt.Sprintf(
			`{"assets":[{"name":%q,"browser_download_url":%q},{"name":"checksums.txt","browser_download_url":%q}]}`,
			testAssetName, assetSrv.URL, checksumsSrv.URL,
		)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(releasesSrv.Close)

	return releasesSrv.URL
}

func TestEnsureBinary_DownloadsWhenMissing(t *testing.T) {
	expectedBody := []byte("fake-binary-bytes")
	releasesURL := releaseServers(t, expectedBody, func(digest string) string {
		return digest + "  " + testAssetName + "\n"
	})

	tmp := t.TempDir()
	dest := filepath.Join(tmp, "plugin-clone")

	err := EnsureBinary(context.Background(), EnsureBinaryConfig{
		Dest:        dest,
		OS:          "test-os",
		Arch:        "test-arch",
		ReleasesURL: releasesURL,
	})

	require.NoError(t, err)
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, expectedBody, got)
}

func TestEnsureBinary_NoMatchingAsset(t *testing.T) {
	releasesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"assets":[{"name":"plugin-clone-other-os-other-arch","browser_download_url":"https://codefloe.com/x"}]}`))
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

func TestEnsureBinary_RejectsChecksumMismatch(t *testing.T) {
	// checksums.txt advertises a digest for a different payload, so the
	// downloaded binary must be rejected and never written to dest.
	releasesURL := releaseServers(t, []byte("tampered-binary-bytes"), func(_ string) string {
		return sha256Hex([]byte("the-expected-binary")) + "  " + testAssetName + "\n"
	})

	dest := filepath.Join(t.TempDir(), "plugin-clone")

	err := EnsureBinary(context.Background(), EnsureBinaryConfig{
		Dest:        dest,
		OS:          "test-os",
		Arch:        "test-arch",
		ReleasesURL: releasesURL,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
	_, statErr := os.Stat(dest)
	assert.True(t, os.IsNotExist(statErr), "tampered binary must not be left at dest")
}

func TestEnsureBinary_RejectsMissingChecksumEntry(t *testing.T) {
	// checksums.txt exists but lists no line for our asset.
	releasesURL := releaseServers(t, []byte("some-binary"), func(_ string) string {
		return sha256Hex([]byte("unrelated")) + "  some-other-asset\n"
	})

	dest := filepath.Join(t.TempDir(), "plugin-clone")

	err := EnsureBinary(context.Background(), EnsureBinaryConfig{
		Dest:        dest,
		OS:          "test-os",
		Arch:        "test-arch",
		ReleasesURL: releasesURL,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no checksum listed")
	_, statErr := os.Stat(dest)
	assert.True(t, os.IsNotExist(statErr))
}

func TestEnsureBinary_RejectsMissingChecksumsAsset(t *testing.T) {
	// Release advertises the binary but no checksums.txt at all: refuse to
	// install an unverifiable binary.
	assetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("some-binary"))
	}))
	defer assetSrv.Close()
	releasesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{"assets":[{"name":%q,"browser_download_url":%q}]}`, testAssetName, assetSrv.URL)
	}))
	defer releasesSrv.Close()

	dest := filepath.Join(t.TempDir(), "plugin-clone")

	err := EnsureBinary(context.Background(), EnsureBinaryConfig{
		Dest:        dest,
		OS:          "test-os",
		Arch:        "test-arch",
		ReleasesURL: releasesSrv.URL,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksums.txt")
	_, statErr := os.Stat(dest)
	assert.True(t, os.IsNotExist(statErr))
}

func TestValidateAssetURL(t *testing.T) {
	require.NoError(t, validateAssetURL("https://codefloe.com/crow-plugins/clone/releases/download/v1/linux-amd64_plugin-clone"))
	require.NoError(t, validateAssetURL("https://sub.codefloe.com/asset"))
	assert.Error(t, validateAssetURL("http://codefloe.com/asset"), "plaintext http must be rejected")
	assert.Error(t, validateAssetURL("https://evil.example.com/asset"), "foreign host must be rejected")
	assert.Error(t, validateAssetURL("https://codefloe.com.evil.com/asset"), "suffix-spoofed host must be rejected")
}
