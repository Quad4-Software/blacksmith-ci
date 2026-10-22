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
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	crowmcp "codefloe.com/crowci/crow/v6/crow-go/mcp"
)

// gitDiscoverer reads remotes, branch and HEAD of the git checkout in dir.
type gitDiscoverer struct {
	dir string
}

func (g gitDiscoverer) git(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Discover implements crowmcp.Discoverer.
func (g gitDiscoverer) Discover(ctx context.Context) (*crowmcp.Checkout, error) {
	commit, err := g.git(ctx, "rev-parse", "HEAD")
	if err != nil {
		return nil, errors.New("the working directory is not a git checkout with commits; pass a url or instance and repo")
	}
	co := &crowmcp.Checkout{Commit: commit}

	// A detached HEAD reports "HEAD"; there is no branch to fall back to then.
	if branch, err := g.git(ctx, "rev-parse", "--abbrev-ref", "HEAD"); err == nil && branch != "HEAD" {
		co.Branch = branch
	}

	names, err := g.git(ctx, "remote")
	if err != nil {
		return nil, err
	}
	for _, name := range strings.Fields(names) {
		raw, err := g.git(ctx, "remote", "get-url", name)
		if err != nil {
			continue
		}
		host, fullName, err := parseRemote(raw)
		if err != nil {
			continue
		}
		co.Remotes = append(co.Remotes, crowmcp.Remote{Name: name, Host: host, FullName: fullName})
	}
	if len(co.Remotes) == 0 {
		return nil, errors.New("the git checkout has no usable remotes")
	}
	return co, nil
}

// parseRemote extracts host and owner/name from https, ssh and scp-style remotes.
func parseRemote(raw string) (host, fullName string, err error) {
	var path string
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", "", fmt.Errorf("parse remote %q: %w", raw, err)
		}
		host, path = u.Hostname(), u.Path
	} else if at, rest, ok := strings.Cut(raw, ":"); ok && !strings.HasPrefix(raw, "/") {
		// scp-like: [user@]host:owner/name.git
		if _, h, ok := strings.Cut(at, "@"); ok {
			at = h
		}
		host, path = at, rest
	}

	fullName = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	if host == "" || !strings.Contains(fullName, "/") {
		return "", "", fmt.Errorf("unsupported git remote %q", raw)
	}
	return host, fullName, nil
}
