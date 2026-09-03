// Copyright 2023 Woodpecker Authors
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
	"fmt"
	"os"
	"strings"

	"al.essio.dev/pkg/shellescape"
)

// genCmdByShell turns the step commands into arguments for the shell named by
// the step image. A shell that needs a script file gets it in scriptDir, which
// belongs to the workflow so the file goes away with the workflow.
func genCmdByShell(shell string, cmdList []string, scriptDir string) (args []string, err error) {
	script := ""
	for _, cmd := range cmdList {
		script += fmt.Sprintf("echo %s\n%s\n", strings.TrimSpace(shellescape.Quote("+ "+cmd)), cmd)
	}
	script = strings.TrimSpace(script)

	switch strings.TrimSuffix(strings.ToLower(shell), ".exe") {
	case "cmd":
		script := "@SET PROMPT=$\n"
		for _, cmd := range cmdList {
			script += fmt.Sprintf("@echo + %s\n", strings.TrimSpace(shellescape.Quote(cmd)))
			script += fmt.Sprintf("@%s\n", cmd)
			script += "@IF NOT %ERRORLEVEL% == 0 exit %ERRORLEVEL%\n"
		}
		cmd, err := os.CreateTemp(scriptDir, "*.cmd")
		if err != nil {
			return nil, err
		}
		defer cmd.Close()
		if _, err := cmd.WriteString(script); err != nil {
			return nil, err
		}
		return []string{"/c", cmd.Name()}, nil
	case "fish":
		script := ""
		for _, cmd := range cmdList {
			script += fmt.Sprintf("echo %s\n%s || exit $status\n", strings.TrimSpace(shellescape.Quote("+ "+cmd)), cmd)
		}
		return []string{"-c", script}, nil
	case "powershell", "pwsh":
		// cspell:disable-next-line
		return []string{"-noprofile", "-noninteractive", "-c", "$ErrorActionPreference = \"Stop\"; " + script}, nil
	default:
		// normal posix shells
		return []string{"-e", "-c", script}, nil
	}
}
