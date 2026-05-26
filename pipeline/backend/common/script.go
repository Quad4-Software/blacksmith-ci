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
// Invoke-Expression. We deliberately avoid the more idiomatic `| iex` form
// because cmd.exe interprets `|` as a pipe even inside the quoted
// -Command argument when our launcher chains statements with `&`. The
// same PowerShell expression works under both Windows PowerShell 5.1
// (powershell.exe) and PowerShell 7+ (pwsh.exe).
//
//nolint:gosec // base64 + Invoke-Expression by design; CI_SCRIPT is agent-controlled
const windowsInnerScript = `Invoke-Expression([System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($env:CI_SCRIPT)))`

// Canonical install paths for the PowerShell interpreters in Windows
// containers. We probe these via `if exist` rather than `where`:
// nanoserver-based images can have pwsh.exe installed without it being on
// PATH, and `where.exe` itself isn't always shipped with stripped-down
// Windows base images. The literal `C:\...` form avoids relying on
// %ProgramFiles%/%SystemRoot% expansion, which not every image guarantees.
//
// Two candidate pwsh locations because the install layout shifted between
// 7.x releases:
//   - PowerShell 7.0-7.2 nanoserver images install flat: \PowerShell\pwsh.exe
//   - PowerShell 7.3+ uses the versioned subdir: \PowerShell\7\pwsh.exe
const (
	winPwshVersionedPath = `C:\Program Files\PowerShell\7\pwsh.exe`
	winPwshFlatPath      = `C:\Program Files\PowerShell\pwsh.exe`
	winPowershellPath    = `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
)

func GenerateContainerConf(commands []string, osType, workDir string) (env map[string]string, entry []string) {
	env = make(map[string]string)
	if osType == "windows" {
		env["CI_SCRIPT"] = base64.StdEncoding.EncodeToString([]byte(generateScriptWindows(commands, workDir)))
		env["SHELL"] = "powershell.exe"
		// cmd.exe ships in every Windows container (including nanoserver),
		// so we use it as a launcher that picks pwsh.exe (PowerShell 7+)
		// when present and falls back to the legacy powershell.exe
		// (Windows PowerShell 5.1) otherwise.
		//
		// Structural rules learned the hard way:
		//   - /v:on enables delayed expansion so `!PWSH!` is evaluated at
		//     run time (after `set` populates it); `%PWSH%` would expand at
		//     parse time, before the set runs, leaving it empty.
		//   - No `(...)` blocks: cmd's `)` inside a quoted string inside a
		//     `(...)` block can prematurely close the block. We use chained
		//     inline `if` statements (no parens) instead, and trigger early
		//     exit via `if not defined PWSH exit /b 1`.
		//   - The inner script uses Invoke-Expression rather than `| iex`
		//     because cmd treats `|` as a pipe even inside the quoted
		//     -Command argument once we're chaining statements with `&`.
		// cspell:disable-next-line
		entry = []string{
			"cmd", "/v:on", "/c",
			`set "PWSH=" & ` +
				`if exist "` + winPwshVersionedPath + `" set "PWSH=` + winPwshVersionedPath + `" & ` +
				`if not defined PWSH if exist "` + winPwshFlatPath + `" set "PWSH=` + winPwshFlatPath + `" & ` +
				`if not defined PWSH if exist "` + winPowershellPath + `" set "PWSH=` + winPowershellPath + `" & ` +
				`if not defined PWSH echo Neither pwsh.exe nor powershell.exe found in the container 1>&2 & ` +
				`if not defined PWSH exit /b 1 & ` +
				`"!PWSH!" -NoProfile -NonInteractive -Command "` + windowsInnerScript + `"`,
		}
	} else {
		env["CI_SCRIPT"] = base64.StdEncoding.EncodeToString([]byte(generateScriptPosix(commands, workDir)))
		env["SHELL"] = "/bin/sh"
		entry = []string{"/bin/sh", "-c", "echo $CI_SCRIPT | base64 -d | /bin/sh -e"}
	}

	return env, entry
}
