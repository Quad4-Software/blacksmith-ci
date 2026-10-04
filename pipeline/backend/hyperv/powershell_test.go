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
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPSQuote(t *testing.T) {
	assert.Equal(t, "'plain'", psQuote("plain"))
	assert.Equal(t, "'it''s'", psQuote("it's"))
	// Nothing else is special inside a PowerShell single-quoted string, so a
	// value full of metacharacters comes through untouched.
	assert.Equal(t, `'$(Get-Date) "x" `+"`"+`n'`, psQuote(`$(Get-Date) "x" `+"`"+`n`))
}

func TestPSDecodeRoundTrips(t *testing.T) {
	const secret = "line1\nline2 '@ \x00 $(bad)"
	got := psDecode(secret)

	start := strings.Index(got, "('") + 2
	end := strings.Index(got[start:], "')") + start
	decoded, err := base64.StdEncoding.DecodeString(got[start:end])
	assert.NoError(t, err)
	assert.Equal(t, secret, string(decoded))
}

func TestCredentialScriptKeepsPasswordOutOfTheLiteral(t *testing.T) {
	got := credentialScript("crow", "p@ss'word")
	assert.Contains(t, got, "$crowUser = 'crow'\n")
	// The password is only ever present base64-encoded, so no value can break
	// out of the script however it is composed.
	assert.NotContains(t, got, "p@ss")
	assert.Contains(t, got, "FromBase64String")
	assert.Contains(t, got, "New-Object System.Management.Automation.PSCredential")
}

func TestExecScriptTargetsTheVM(t *testing.T) {
	got := execScript("crow-abc", "crow", "pw", "Write-Output 'hi'")
	assert.Contains(t, got, "Invoke-Command -VMName 'crow-abc' -Credential $crowCred")
	assert.Contains(t, got, psPreamble)
	// The body travels as a base64 argument rather than inline, so a script
	// containing a here-string terminator cannot end the wrapper.
	assert.NotContains(t, got, "Write-Output 'hi'")
}

func TestRunStepScriptStreamsThenCollectsExitCode(t *testing.T) {
	got := runStepScript("crow-abc", "crow", "pw", "exit 3", `C:\crow\scripts\s.ps1`, `C:\crow\scripts\s.code`)

	// Three round trips: place the script, run it, read the code back.
	assert.Equal(t, 3, strings.Count(got, "Invoke-Command -VMName $crowVM"))

	// The run must not be captured into a variable, or its output would never
	// reach the log while the step is still running.
	runIdx := strings.Index(got, "& powershell.exe")
	assert.Positive(t, runIdx)
	assert.NotContains(t, got[:runIdx], "$crowRun = Invoke-Command")

	// Native stderr is merged so it cannot surface as a terminating error.
	assert.Contains(t, got, "-File $path 2>&1")

	// The exit code is read back and becomes this process's own, which is
	// what lets WaitStep treat the step like any other child process.
	assert.Contains(t, got, "exit [int]$crowExit\n")
	assert.Contains(t, got, "Set-Content -LiteralPath $codePath -Value $LASTEXITCODE")
}

func TestRunStepScriptDefaultsToFailureWhenNoCodeWasWritten(t *testing.T) {
	got := runStepScript("crow-abc", "crow", "pw", "", "s.ps1", "s.code")
	// A guest that died before writing the file must not read as success.
	assert.Contains(t, got, "else { '1' }")
}
