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

// cSpell:ignore ERRORLEVEL

package local

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

func TestGenCmdByShell(t *testing.T) {
	commands := []string{"go version", "go test ./..."}

	tests := []struct {
		name  string
		shell string
		want  []string
	}{
		{
			name:  "posix shell",
			shell: "sh",
			want:  []string{"-e", "-c", "echo '+ go version'\ngo version\necho '+ go test ./...'\ngo test ./..."},
		},
		{
			name:  "fish",
			shell: "fish",
			want:  []string{"-c", "echo '+ go version'\ngo version || exit $status\necho '+ go test ./...'\ngo test ./... || exit $status\n"},
		},
		{
			name:  "pwsh",
			shell: "pwsh",
			want:  []string{"-noprofile", "-noninteractive", "-c", "$ErrorActionPreference = \"Stop\"; echo '+ go version'\ngo version\necho '+ go test ./...'\ngo test ./..."},
		},
		{
			name:  "windows powershell with exe suffix and mixed case",
			shell: "PowerShell.exe",
			want:  []string{"-noprofile", "-noninteractive", "-c", "$ErrorActionPreference = \"Stop\"; echo '+ go version'\ngo version\necho '+ go test ./...'\ngo test ./..."},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args, err := genCmdByShell(tc.shell, commands, t.TempDir())
			require.NoError(t, err)
			assert.Equal(t, tc.want, args)
		})
	}
}

// cmd.exe cannot take a multi-line script as an argument, so it gets a script
// file. That file must live in the workflow's own directory so it disappears
// with the workflow instead of piling up in the agent's temp dir.
func TestGenCmdByShellCmdWritesScriptIntoWorkflowDir(t *testing.T) {
	scriptDir := t.TempDir()

	args, err := genCmdByShell("cmd.exe", []string{"go version", "go test ./..."}, scriptDir)
	require.NoError(t, err)
	require.Len(t, args, 2)
	assert.Equal(t, "/c", args[0])
	assert.Equal(t, scriptDir, filepath.Dir(args[1]))
	assert.Equal(t, ".cmd", filepath.Ext(args[1]))

	script, err := os.ReadFile(args[1])
	require.NoError(t, err)
	assert.Equal(t, strings.Join([]string{
		"@SET PROMPT=$",
		"@echo + 'go version'",
		"@go version",
		"@IF NOT %ERRORLEVEL% == 0 exit %ERRORLEVEL%",
		"@echo + 'go test ./...'",
		"@go test ./...",
		"@IF NOT %ERRORLEVEL% == 0 exit %ERRORLEVEL%",
		"",
	}, "\n"), string(script))
}

func TestPwshQuote(t *testing.T) {
	assert.Equal(t, `'C:\crow\plugin-clone.exe'`, pwshQuote(`C:\crow\plugin-clone.exe`))
	assert.Equal(t, `'C:\it''s\_netrc'`, pwshQuote(`C:\it's\_netrc`))
}

// The per-workflow temp dir only isolates a step if the step's tools read the
// variable that points at it, which on Windows is TMP/TEMP rather than TMPDIR.
// A step must not be able to point any of them elsewhere either.
func TestStepEnvTempDirPerPlatform(t *testing.T) {
	for _, name := range []string{"USERPROFILE", "TMP", "TEMP", "TMPDIR", "HOME"} {
		assert.Contains(t, notAllowedEnvVarOverwrites, name)
	}

	ctx := context.Background()
	for _, goos := range []string{"linux", "windows"} {
		t.Run(goos, func(t *testing.T) {
			engine := &local{tempDir: t.TempDir(), os: goos}
			const taskUUID = "task-uuid"
			require.NoError(t, engine.SetupWorkflow(ctx, &types.Config{}, taskUUID))
			t.Cleanup(func() { _ = engine.DestroyWorkflow(ctx, &types.Config{}, taskUUID) })

			state, err := engine.getState(taskUUID)
			require.NoError(t, err)

			// A shell that does not exist keeps StartStep from running anything
			// while still building the environment.
			err = engine.StartStep(ctx, &types.Step{
				UUID:  "step-uuid",
				Type:  types.StepTypeCommands,
				Image: filepath.Join(t.TempDir(), "no-such-shell"),
				Environment: map[string]string{
					"TEMP": "/somewhere/else",
				},
			}, taskUUID)
			require.Error(t, err)

			state.stepCMDsMu.RLock()
			cmd := state.stepCMDs["step-uuid"]
			state.stepCMDsMu.RUnlock()
			require.NotNil(t, cmd)

			env := strings.Join(cmd.Env, "\n")
			assert.Contains(t, env, "TMPDIR="+state.tmpDir)
			assert.NotContains(t, env, "TEMP=/somewhere/else")
			if goos == "windows" {
				assert.Contains(t, env, "TMP="+state.tmpDir)
				assert.Contains(t, env, "TEMP="+state.tmpDir)
			} else {
				assert.NotContains(t, env, "TMP=")
				assert.NotContains(t, env, "TEMP=")
			}
		})
	}
}

// The Library/Caches directory is a macOS convention; other platforms must
// not get it.
func TestSetupWorkflowCacheDirOnlyOnDarwin(t *testing.T) {
	ctx := context.Background()
	for _, goos := range []string{"darwin", "linux", "windows"} {
		t.Run(goos, func(t *testing.T) {
			engine := &local{tempDir: t.TempDir(), os: goos}
			const taskUUID = "task-uuid"
			require.NoError(t, engine.SetupWorkflow(ctx, &types.Config{}, taskUUID))
			t.Cleanup(func() { _ = engine.DestroyWorkflow(ctx, &types.Config{}, taskUUID) })

			state, err := engine.getState(taskUUID)
			require.NoError(t, err)

			_, err = os.Stat(filepath.Join(state.homeDir, "Library", "Caches"))
			if goos == "darwin" {
				assert.NoError(t, err)
			} else {
				assert.True(t, os.IsNotExist(err), "unexpected Library/Caches on %s", goos)
			}
		})
	}
}
