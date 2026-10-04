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
	"fmt"
	"strings"
)

// psPreamble is prepended to every host-side script.
//
// ProgressPreference is silenced because Hyper-V cmdlets emit progress records
// that would otherwise be interleaved with a step's output.
const psPreamble = "$ErrorActionPreference = 'Stop'\n$ProgressPreference = 'SilentlyContinue'\n"

// psQuote renders s as a PowerShell single-quoted literal.
//
// Single quotes are the only PowerShell string form with no escape sequences at
// all, so the sole thing to handle is a quote inside the value, which is
// written by doubling it. Nothing else in a value can change how it parses.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// psDecode renders an expression that evaluates to s at runtime.
//
// Anything carrying a secret or a whole script body goes through this rather
// than psQuote: base64 has no PowerShell metacharacters and no line structure,
// so a value cannot terminate the literal that holds it however it is composed.
func psDecode(s string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(s))
	return fmt.Sprintf("[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s'))", encoded)
}

// credentialScript builds the PSCredential that PowerShell Direct
// authenticates to the guest with.
//
// The password is decoded from base64 inside the script, and the script itself
// reaches PowerShell on stdin, so the password never appears in the host's
// argv or environment where another process on the host could read it.
func credentialScript(username, password string) string {
	return fmt.Sprintf(
		"$crowUser = %s\n"+
			"$crowPass = ConvertTo-SecureString (%s) -AsPlainText -Force\n"+
			"$crowCred = New-Object System.Management.Automation.PSCredential($crowUser, $crowPass)\n",
		psQuote(username), psDecode(password))
}

// execScript runs a PowerShell body inside the guest and returns when it is
// done, forwarding whatever the body wrote.
//
// Used for the backend's own setup and probe work, where the output is small
// and the exit code is the host cmdlet's own. Steps use runStepScript instead.
func execScript(vm, username, password, body string) string {
	var b strings.Builder
	b.WriteString(psPreamble)
	b.WriteString(credentialScript(username, password))
	fmt.Fprintf(&b, "$crowBody = %s\n", psDecode(body))
	fmt.Fprintf(&b, "Invoke-Command -VMName %s -Credential $crowCred -ScriptBlock { param($b) & ([ScriptBlock]::Create($b)) } -ArgumentList $crowBody\n",
		psQuote(vm))
	return b.String()
}

// runStepScript runs a step's script in the guest, streaming its output and
// exiting with the step's exit code.
//
// It takes three round trips rather than one because PowerShell remoting has
// no way to both stream a command's output and return its exit code: capturing
// the pipeline would swallow the output that has to reach the log live. So the
// script is written into the guest, run with its output left to flow back
// uncaptured, and its exit code parked in a file that a third call reads.
//
// The guest's stderr is merged into stdout by the 2>&1 redirection, and every
// record is then rendered as plain text. 2>&1 turns each stderr line of a
// native command into an error record, and PowerShell formats an error record
// with its position and category, so a step that wrote one line to stderr
// would log five lines of "NativeCommandError" noise around it. Rendering
// keeps it to the line itself. It also keeps error records from crossing back
// over PowerShell Direct, where the host would report them separately from
// the output they belong with.
func runStepScript(vm, username, password, body, scriptPath, codePath string) string {
	var b strings.Builder
	b.WriteString(psPreamble)
	b.WriteString(credentialScript(username, password))
	fmt.Fprintf(&b, "$crowBody = %s\n", psDecode(body))
	fmt.Fprintf(&b, "$crowScript = %s\n$crowCode = %s\n", psQuote(scriptPath), psQuote(codePath))
	fmt.Fprintf(&b, "$crowVM = %s\n", psQuote(vm))

	// 1. Place the script. Set-Content, not Copy-VMFile: the body carries the
	//    step's secrets and this path keeps them on the remoting channel
	//    rather than on the host's filesystem in a temporary file.
	b.WriteString("Invoke-Command -VMName $crowVM -Credential $crowCred -ScriptBlock {\n" +
		"  param($body, $path)\n" +
		"  $dir = Split-Path -Parent $path\n" +
		"  if (-not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }\n" +
		"  Set-Content -LiteralPath $path -Value $body -Encoding UTF8\n" +
		"} -ArgumentList $crowBody, $crowScript\n")

	// 2. Run it. Output is deliberately not captured so it streams to this
	//    process's stdout, which is what the agent is tailing.
	b.WriteString("Invoke-Command -VMName $crowVM -Credential $crowCred -ScriptBlock {\n" +
		"  param($path, $codePath)\n" +
		"  $ErrorActionPreference = 'Continue'\n" +
		"  & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $path 2>&1 | ForEach-Object { \"$_\" }\n" +
		"  Set-Content -LiteralPath $codePath -Value $LASTEXITCODE -Encoding ASCII\n" +
		"} -ArgumentList $crowScript, $crowCode\n")

	// 3. Collect the exit code and become it, so the host process's own exit
	//    status is the step's and WaitStep needs no special case.
	b.WriteString("$crowExit = Invoke-Command -VMName $crowVM -Credential $crowCred -ScriptBlock {\n" +
		"  param($codePath)\n" +
		"  if (Test-Path -LiteralPath $codePath) { (Get-Content -LiteralPath $codePath -Raw).Trim() } else { '1' }\n" +
		"} -ArgumentList $crowCode\n")
	b.WriteString("exit [int]$crowExit\n")

	return b.String()
}
