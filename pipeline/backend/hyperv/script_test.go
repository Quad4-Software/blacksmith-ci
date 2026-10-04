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

package hyperv

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

func TestWinJoin(t *testing.T) {
	assert.Equal(t, `C:\crow\workspace`, winJoin(`C:\crow`, "workspace"))
	assert.Equal(t, `C:\crow`, winJoin(`C:`, "crow"))
	assert.Equal(t, `C:\crow`, winJoin(`C:\`, "crow"))
	assert.Equal(t, `C:\crow\a\b`, winJoin(`C:\crow\`, `\a\`, "b"))
	// An empty first component is how diskPath renders when no VM path is
	// configured, and must not produce a leading separator.
	assert.Equal(t, "crow-x.vhdx", winJoin("", "crow-x.vhdx"))
}

func TestNewGuestPaths(t *testing.T) {
	paths := newGuestPaths(`C:\crow`)
	assert.Equal(t, `C:\crow`, paths.Base)
	assert.Equal(t, `C:\crow\workspace`, paths.Workspace)
	assert.Equal(t, `C:\crow\tmp`, paths.Tmp)
	assert.Equal(t, `C:\crow\home`, paths.Home)
	assert.Equal(t, `C:\crow\scripts`, paths.Scripts)
}

func TestStepEnvDerivesLayoutAndDropsOverrides(t *testing.T) {
	paths := newGuestPaths(`C:\crow`)
	env := stepEnv(&types.Step{Environment: map[string]string{
		"FOO":               "bar",
		"HOME":              "C:\\attacker",
		"CI_WORKSPACE":      "C:\\attacker",
		"TEMP":              "C:\\attacker",
		"CI_NETRC_PASSWORD": "hunter2",
	}}, paths)

	assert.Equal(t, "bar", env["FOO"])
	assert.Equal(t, paths.Home, env["HOME"])
	assert.Equal(t, paths.Home, env["USERPROFILE"])
	assert.Equal(t, paths.Workspace, env["CI_WORKSPACE"])
	assert.Equal(t, paths.Tmp, env["TEMP"])
	// Every other step has its netrc credentials stripped; only the clone
	// step puts them back, via cloneEnv.
	assert.NotContains(t, env, "CI_NETRC_PASSWORD")
}

func TestBuildScriptChecksEveryCommand(t *testing.T) {
	got := buildScript(map[string]string{"FOO": "bar"}, `C:\crow\workspace`, []string{"go build ./...", "go test ./..."})

	assert.Contains(t, got, "$env:FOO = 'bar'\n")
	assert.Contains(t, got, "Set-Location -LiteralPath 'C:\\crow\\workspace'\n")
	// $LASTEXITCODE is reset before each command, because a command that runs
	// no native executable would otherwise inherit the previous one's value.
	assert.Equal(t, 2, strings.Count(got, "$global:LASTEXITCODE = 0\n"))
	assert.Equal(t, 2, strings.Count(got, "if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }\n"))
	assert.True(t, strings.HasSuffix(got, "exit 0\n"))
}

func TestBuildScriptIsDeterministic(t *testing.T) {
	env := map[string]string{"B": "2", "A": "1", "C": "3"}
	first := buildScript(env, "", nil)
	for range 5 {
		assert.Equal(t, first, buildScript(env, "", nil))
	}
	assert.Less(t, strings.Index(first, "$env:A"), strings.Index(first, "$env:B"))
}

func TestBuildScriptQuotesHostileValues(t *testing.T) {
	got := buildScript(map[string]string{"EVIL": "it's '; Remove-Item C:\\ #"}, "", nil)
	// The quote inside the value is doubled, so it cannot end the literal and
	// the rest of the value can never be parsed as code.
	assert.Contains(t, got, "$env:EVIL = 'it''s ''; Remove-Item C:\\ #'\n")
}

func TestSetupScriptCreatesEveryDirectory(t *testing.T) {
	paths := newGuestPaths(`C:\crow`)
	got := setupScript(paths)
	for _, dir := range []string{paths.Workspace, paths.Tmp, paths.Home, paths.Scripts} {
		assert.Contains(t, got, psQuote(dir))
	}
}

func TestStepPathsAreSanitized(t *testing.T) {
	paths := newGuestPaths(`C:\crow`)
	assert.Equal(t, `C:\crow\scripts\01JABC.ps1`, stepScriptPath(paths, "01JABC"))
	assert.Equal(t, `C:\crow\scripts\01JABC.code`, stepCodePath(paths, "01JABC"))
	// A UUID is never hostile, but nothing here relies on that.
	assert.Equal(t, `C:\crow\scripts\_________evil.ps1`, stepScriptPath(paths, `..\..\..\evil`))
}
