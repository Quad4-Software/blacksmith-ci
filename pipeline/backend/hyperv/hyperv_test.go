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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostEnvKeepsWhatHyperVNeeds(t *testing.T) {
	env := hostEnv([]string{
		"COMPUTERNAME=HOST1",
		`PSModulePath=C:\Windows\system32\WindowsPowerShell\v1.0\Modules`,
		`SystemRoot=C:\Windows`,
		`TEMP=C:\Windows\TEMP`,
	})
	assert.Equal(t, []string{
		"COMPUTERNAME=HOST1",
		`PSModulePath=C:\Windows\system32\WindowsPowerShell\v1.0\Modules`,
		`SystemRoot=C:\Windows`,
		`TEMP=C:\Windows\TEMP`,
	}, env)
}

func TestHostEnvDropsTheAgentsConfiguration(t *testing.T) {
	env := hostEnv([]string{
		"CROW_AGENT_SECRET=hunter2",
		"CROW_SERVER=crow.example.com:443",
		"CROW_BACKEND_HYPERV_GUEST_PASSWORD=pw",
		"HTTPS_PROXY=http://proxy:3128",
		"COMPUTERNAME=HOST1",
	})
	assert.Equal(t, []string{"COMPUTERNAME=HOST1"}, env)
}

func TestHostEnvMatchesNamesCaseInsensitively(t *testing.T) {
	// Windows reports some of these in mixed case depending on how the
	// process was started, for example "Path" rather than "PATH".
	env := hostEnv([]string{`Path=C:\Windows`, `SYSTEMROOT=C:\Windows`})
	assert.Equal(t, []string{`Path=C:\Windows`, `SYSTEMROOT=C:\Windows`}, env)
}

func TestHostEnvSkipsDriveWorkingDirectories(t *testing.T) {
	env := hostEnv([]string{`=C:=C:\agent`, "=ExitCode=00000000", "COMPUTERNAME=HOST1"})
	assert.Equal(t, []string{"COMPUTERNAME=HOST1"}, env)
}
