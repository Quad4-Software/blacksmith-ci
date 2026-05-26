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
	"encoding/base64"
)

// windowsInnerScript decodes the base64-encoded CI_SCRIPT and runs it via
// Invoke-Expression. The pipe-to-iex form would also work as a pure
// PowerShell expression, but writing it as a function call keeps the
// inner script free of any `|` that could confuse a hypothetical shell
// wrapper. Same syntax runs under both Windows PowerShell 5.1
// (powershell.exe) and PowerShell 7+ (pwsh.exe).
//
//nolint:gosec // base64 + Invoke-Expression by design; CI_SCRIPT is agent-controlled
const windowsInnerScript = `Invoke-Expression([System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($env:CI_SCRIPT)))`

func GenerateContainerConf(commands []string, osType, workDir string) (env map[string]string, entry []string) {
	env = make(map[string]string)
	if osType == "windows" {
		env["CI_SCRIPT"] = base64.StdEncoding.EncodeToString([]byte(generateScriptWindows(commands, workDir)))
		env["SHELL"] = "pwsh.exe"
		// Invoke pwsh directly as the container entrypoint — no cmd.exe
		// wrapper. Earlier iterations wrapped the call in `cmd /c` so we
		// could probe several PowerShell install locations, but Docker on
		// Windows escapes argv with the C-runtime convention (`\"` for
		// embedded quotes) while cmd.exe expects `""`. The mismatch
		// silently corrupted every quoted token, leaving cmd to execute a
		// garbled line that produced no output. pwsh shares the C-runtime
		// convention, so passing argv through to it round-trips cleanly.
		//
		// Side effect: the container image must expose `pwsh` on PATH.
		// The mcr.microsoft.com/powershell:* family does this; bare
		// servercore images that only ship Windows PowerShell 5.1 need
		// the pwsh-suffixed variant (e.g. powershell:windowsservercore-*)
		// or a different image. See docs/dev/configuration/windows-agent.
		entry = []string{
			"pwsh", "-NoProfile", "-NonInteractive", "-Command", windowsInnerScript,
		}
	} else {
		env["CI_SCRIPT"] = base64.StdEncoding.EncodeToString([]byte(generateScriptPosix(commands, workDir)))
		env["SHELL"] = "/bin/sh"
		entry = []string{"/bin/sh", "-c", "echo $CI_SCRIPT | base64 -d | /bin/sh -e"}
	}

	return env, entry
}
