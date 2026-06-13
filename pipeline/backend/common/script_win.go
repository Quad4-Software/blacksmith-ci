// Copyright 2022 Woodpecker Authors
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
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

func generateScriptWindows(commands []string, workDir string) string {
	var buf bytes.Buffer

	if err := setupScriptWinTmpl.Execute(&buf, map[string]string{
		"WorkDir": workDir,
	}); err != nil {
		// should never happen but well we have an error to trance
		return fmt.Sprintf("echo 'failed to generate posix script from commands: %s'; exit 1", err.Error())
	}

	for _, command := range commands {
		escaped := fmt.Sprintf("%q", command)
		escaped = strings.ReplaceAll(escaped, "$", `\$`)
		// The trace line embeds this value inside a PowerShell single-quoted
		// string (Write-Output ('+ ...')). Any single quote in the command
		// would terminate that string early and break parsing, so double them
		// per PowerShell's single-quote escaping rule.
		escaped = strings.ReplaceAll(escaped, "'", "''")
		fmt.Fprintf(&buf,
			traceScriptWin,
			escaped,
			command,
		)
	}

	return buf.String()
}

const setupScriptWinProto = `
$ErrorActionPreference = 'Stop';
if (-not (Test-Path "{{.WorkDir}}")) { New-Item -Path "{{.WorkDir}}" -ItemType Directory -Force | Out-Null };
if (-not [Environment]::GetEnvironmentVariable('HOME')) { [Environment]::SetEnvironmentVariable('HOME', 'c:\root') };
if (-not (Test-Path "$env:HOME")) { New-Item -Path "$env:HOME" -ItemType Directory -Force | Out-Null };
if ($Env:CI_NETRC_MACHINE) {
$netrc=[string]::Format("{0}\_netrc",$Env:HOME);
"machine $Env:CI_NETRC_MACHINE" >> $netrc;
"login $Env:CI_NETRC_USERNAME" >> $netrc;
"password $Env:CI_NETRC_PASSWORD" >> $netrc;
};
[Environment]::SetEnvironmentVariable("CI_NETRC_PASSWORD",$null);
[Environment]::SetEnvironmentVariable("CI_SCRIPT",$null);
cd "{{.WorkDir}}";
`

var setupScriptWinTmpl, _ = template.New("").Parse(setupScriptWinProto)

// traceScript is a helper script that is added to the step script
// to trace a command.
//
// We deliberately do NOT prefix the command with PowerShell's `&` call
// operator. `&` invokes the value of its argument as a command, which
// breaks for commands that are expressions starting with a variable
// (e.g. `$PSVersionTable | Format-List`) — `&` would try to invoke the
// hashtable value, not pipe it. Running the command directly lets pwsh
// parse it as the user typed it: native commands, cmdlets, pipelines,
// and variable references all work. Users invoking quoted paths
// (e.g. `'C:\Some Path\prog.exe' -arg`) must include their own `&`
// prefix, which is standard pwsh idiom.
const traceScriptWin = `
Write-Output ('+ %s');
%s
if ($LASTEXITCODE -ne 0) {exit $LASTEXITCODE}
`
