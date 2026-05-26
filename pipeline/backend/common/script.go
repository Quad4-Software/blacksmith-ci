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

// windowsInnerScript decodes the base64-encoded CI_SCRIPT and pipes it to
// iex. The same syntax works under both Windows PowerShell 5.1 (powershell.exe)
// and PowerShell 7+ (pwsh.exe), so we can choose either interpreter at runtime.
//
//nolint:gosec // base64 + iex by design; CI_SCRIPT is agent-controlled
const windowsInnerScript = `[System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($Env:CI_SCRIPT)) | iex`

// Canonical install paths for the two PowerShell interpreters. We probe these
// via `if exist` rather than `where`: nanoserver-based images can have pwsh.exe
// installed without it being on PATH, and `where.exe` itself isn't always
// shipped with stripped-down Windows base images.
const (
	winPwshPath       = `%ProgramFiles%\PowerShell\7\pwsh.exe`
	winPowershellPath = `%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe`
)

func GenerateContainerConf(commands []string, osType, workDir string) (env map[string]string, entry []string) {
	env = make(map[string]string)
	if osType == "windows" {
		env["CI_SCRIPT"] = base64.StdEncoding.EncodeToString([]byte(generateScriptWindows(commands, workDir)))
		env["SHELL"] = "powershell.exe"
		// cmd.exe ships in every Windows container (including nanoserver),
		// so we use it as a launcher that picks pwsh.exe (PowerShell 7+)
		// when present and falls back to the legacy powershell.exe
		// (Windows PowerShell 5.1) otherwise. We use absolute paths via
		// `if exist` because nanoserver-based PowerShell images install
		// pwsh at the canonical location but don't always add it to PATH,
		// and `where.exe` isn't reliably present in nanoserver either.
		// cspell:disable-next-line
		entry = []string{
			"cmd", "/c",
			`if exist "` + winPwshPath + `" ("` + winPwshPath + `" -noprofile -noninteractive -command "` + windowsInnerScript + `") else if exist "` + winPowershellPath + `" ("` + winPowershellPath + `" -noprofile -noninteractive -command "` + windowsInnerScript + `") else (echo Neither pwsh.exe nor powershell.exe found in the container 1>&2 & exit /b 1)`,
		}
	} else {
		env["CI_SCRIPT"] = base64.StdEncoding.EncodeToString([]byte(generateScriptPosix(commands, workDir)))
		env["SHELL"] = "/bin/sh"
		entry = []string{"/bin/sh", "-c", "echo $CI_SCRIPT | base64 -d | /bin/sh -e"}
	}

	return env, entry
}
