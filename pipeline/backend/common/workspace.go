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

package common

import (
	"strings"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// WorkspaceInitPrefix is the container name prefix for the workspace-init
// helper used by the Docker and Podman backends.
const WorkspaceInitPrefix = "crow-workspace-init-"

// CollectWorkspacePaths returns the shared workspace base to mount the volume
// at, plus the distinct working directories that should be pre-created. Every
// step shares the same base (DefaultWorkspaceBase / pluginWorkspaceBase), so
// the first non-empty base wins.
func CollectWorkspacePaths(conf *types.Config) (base string, workdirs []string) {
	seen := make(map[string]struct{})
	for _, stage := range conf.Stages {
		for _, step := range stage.Steps {
			if base == "" && step.WorkspaceBase != "" {
				base = step.WorkspaceBase
			}
			if step.WorkingDir == "" {
				continue
			}
			if _, ok := seen[step.WorkingDir]; ok {
				continue
			}
			seen[step.WorkingDir] = struct{}{}
			workdirs = append(workdirs, step.WorkingDir)
		}
	}
	return base, workdirs
}

// BuildWorkspaceInitScript creates the working directories and makes the whole
// workspace tree world-writable with the sticky bit set (1777, like /tmp). The
// volume is empty when this runs, so the recursive chmod only ever touches the
// directories we just created.
//
// Running this agent-controlled script before any pipeline step or plugin makes
// workspace ownership deterministic: it no longer depends on whether the first
// item to write into the fresh named volume is a (non-root) command step or a
// (root) plugin.
func BuildWorkspaceInitScript(base string, workdirs []string) string {
	var b strings.Builder
	if len(workdirs) > 0 {
		b.WriteString("mkdir -p")
		for _, dir := range workdirs {
			b.WriteString(" ")
			b.WriteString(shellSingleQuote(dir))
		}
		b.WriteString(" && ")
	}
	b.WriteString("chmod -R 1777 ")
	b.WriteString(shellSingleQuote(base))
	return b.String()
}

// shellSingleQuote wraps s in single quotes, escaping any embedded single
// quotes, so it is safe to interpolate into a `/bin/sh -c` command.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
