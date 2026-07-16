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
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

func TestCollectWorkspacePaths(t *testing.T) {
	conf := &types.Config{
		Stages: []*types.Stage{
			{Steps: []*types.Step{
				{WorkspaceBase: "/crow", WorkingDir: "/crow/src/example.com/owner/repo"},
			}},
			{Steps: []*types.Step{
				// duplicate working dir must be collapsed
				{WorkspaceBase: "/crow", WorkingDir: "/crow/src/example.com/owner/repo"},
				// plugin without a working dir contributes nothing extra
				{WorkspaceBase: "/crow"},
			}},
		},
	}

	base, workdirs := CollectWorkspacePaths(conf)
	assert.Equal(t, "/crow", base)
	assert.Equal(t, []string{"/crow/src/example.com/owner/repo"}, workdirs)
}

func TestCollectWorkspacePathsEmpty(t *testing.T) {
	base, workdirs := CollectWorkspacePaths(&types.Config{})
	assert.Empty(t, base)
	assert.Empty(t, workdirs)
}

func TestBuildWorkspaceInitScript(t *testing.T) {
	script := BuildWorkspaceInitScript("/crow", []string{"/crow/src/example.com/owner/repo"})
	assert.Equal(t, "mkdir -p '/crow/src/example.com/owner/repo' && chmod -R 1777 '/crow'", script)
}

func TestBuildWorkspaceInitScriptNoWorkdirs(t *testing.T) {
	script := BuildWorkspaceInitScript("/crow", nil)
	assert.Equal(t, "chmod -R 1777 '/crow'", script)
}

func TestBuildWorkspaceInitScriptQuotesPaths(t *testing.T) {
	// a path containing a single quote must not break out of the shell string
	script := BuildWorkspaceInitScript("/crow", []string{"/crow/src/o'wner/repo"})
	assert.Equal(t, `mkdir -p '/crow/src/o'\''wner/repo' && chmod -R 1777 '/crow'`, script)
}
