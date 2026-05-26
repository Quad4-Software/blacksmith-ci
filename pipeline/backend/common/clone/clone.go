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

// Package clone exposes the bits the local and docker backends both need to
// run the crow-plugins/clone binary natively on the agent host: env-var
// preparation, netrc generation, and downloading the plugin binary if it
// isn't already on PATH.
package clone

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Env returns base with:
//
//  1. GIT_TERMINAL_PROMPT=0 and GCM_INTERACTIVE=Never appended — git and the
//     Windows-bundled Git Credential Manager refuse to interactively prompt
//     for credentials. Without these, missing or wrong credentials cause an
//     indefinite hang on a headless agent.
//
//  2. The CI_NETRC_* family from stepEnv appended — these are added by the
//     server's pipeline compiler so the clone plugin can authenticate. The
//     local backend's StartStep strips them from the regular subprocess env
//     via notAllowedEnvVarOverwrites; this re-adds them for the clone step.
func Env(base []string, stepEnv map[string]string) []string {
	out := slices.Clone(base)
	out = append(out,
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=Never",
	)
	for _, k := range []string{
		"CI_NETRC_MACHINE",
		"CI_NETRC_USERNAME",
		"CI_NETRC_PASSWORD",
	} {
		if v, ok := stepEnv[k]; ok && v != "" {
			out = append(out, k+"="+v)
		}
	}
	return out
}

const netrcFile = `
machine %s
login %s
password %s
`

// GenNetRC renders the netrc body from CI_NETRC_* values in stepEnv.
// Missing keys render as empty strings — the caller is expected to gate
// on at least CI_NETRC_MACHINE being set before writing the file.
func GenNetRC(stepEnv map[string]string) string {
	return fmt.Sprintf(
		netrcFile,
		stepEnv["CI_NETRC_MACHINE"],
		stepEnv["CI_NETRC_USERNAME"],
		stepEnv["CI_NETRC_PASSWORD"],
	)
}

// WriteNetRC writes a netrc file into homeDir using the CI_NETRC_* values
// from stepEnv. Returns the absolute path of the written file (or empty
// string when no netrc was written because CI_NETRC_MACHINE is missing).
//
// Filename is .netrc on POSIX and _netrc on Windows. The latter matches
// what curl/libcurl looks for first on Windows; using .netrc would mean
// git on Windows might not find it depending on the version's fallback.
func WriteNetRC(homeDir, os string, stepEnv map[string]string) (string, error) {
	if stepEnv["CI_NETRC_MACHINE"] == "" {
		return "", nil
	}
	name := ".netrc"
	if os == "windows" {
		name = "_netrc"
	}
	path := filepath.Join(homeDir, name)
	return path, osWriteFile(path, []byte(GenNetRC(stepEnv)), 0o600)
}

// osWriteFile is a thin indirection to allow a future swap to a sandboxed
// writer if needed. Today it is the std library.
var osWriteFile = os.WriteFile

// DefaultReleasesURL is where the latest crow-plugins/clone release metadata
// lives. Tests override it via EnsureBinaryConfig.ReleasesURL.
const DefaultReleasesURL = "https://codeberg.org/api/v1/repos/crow-plugins/clone/releases/latest"

// CheckGitAvailable verifies that the git binary is on PATH. Returns an
// error if it isn't — the clone plugin shells out to git, so this is a
// prerequisite. Called at agent startup, not per clone, so the cost is
// negligible.
func CheckGitAvailable() error {
	_, err := exec.LookPath("git")
	return err
}

// EnsureBinaryConfig holds the inputs to EnsureBinary. Kept as a struct so
// future options (auth token for private releases, signature verification,
// etc.) can be added without breaking the signature.
type EnsureBinaryConfig struct {
	// Dest is the absolute path where the binary should live.
	Dest string

	// OS and Arch select the right asset from the release. Matches Go's
	// runtime.GOOS / runtime.GOARCH naming.
	OS, Arch string

	// ReleasesURL overrides DefaultReleasesURL for tests.
	ReleasesURL string
}

// httpTimeout bounds how long a single releases-metadata or download
// request may take. The binary is ~50MB; 10 minutes is generous but still
// puts an upper bound on a hung endpoint pinning a clone step.
const httpTimeout = 10 * time.Minute

// httpClient is the shared HTTP client used for releases-metadata and binary
// downloads.
var httpClient = &http.Client{Timeout: httpTimeout}

// EnsureBinary downloads the plugin-clone binary to cfg.Dest if it isn't
// already present. The release is selected by matching cfg.OS and cfg.Arch
// against asset names from the releases API.
//
// The download writes through a temp file in the destination dir and is
// renamed into place atomically so concurrent callers or crash-resumed
// downloads never observe a partial binary at cfg.Dest.
func EnsureBinary(ctx context.Context, cfg EnsureBinaryConfig) error {
	if cfg.ReleasesURL == "" {
		cfg.ReleasesURL = DefaultReleasesURL
	}
	if _, err := os.Stat(cfg.Dest); err == nil {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.ReleasesURL, nil)
	if err != nil {
		return fmt.Errorf("build releases request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("get latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: HTTP %d", cfg.ReleasesURL, resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read releases body: %w", err)
	}

	var rel struct {
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(raw, &rel); err != nil {
		return fmt.Errorf("decode release metadata: %w", err)
	}

	for _, asset := range rel.Assets {
		if strings.Contains(asset.Name, cfg.OS) && strings.Contains(asset.Name, cfg.Arch) {
			return downloadAndChmod(ctx, asset.URL, cfg.Dest)
		}
	}
	return fmt.Errorf("no plugin-clone asset found for %s/%s", cfg.OS, cfg.Arch)
}

func downloadAndChmod(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download binary: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}

	// Write to a temp file in the same directory as dest so the final
	// rename happens on a single filesystem (and is therefore atomic).
	tmp, err := os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	// Cleanup on any error path; no-op once Rename succeeds (file no
	// longer exists at tmpPath) or once another writer beats us (we
	// remove tmpPath explicitly below before returning nil).
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write binary: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// Another writer may have finished while we were downloading. In
	// that case keep their copy and discard ours rather than racing
	// rename(), which on Windows errors when the destination exists.
	if _, err := os.Stat(dest); err == nil {
		return nil
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return fmt.Errorf("rename into place: %w", err)
	}
	return nil
}

// Ensure errors is used (the import will be needed in later tasks).
var _ = errors.New
