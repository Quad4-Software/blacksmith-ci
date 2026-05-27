// Copyright 2024 Woodpecker Authors
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
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	windowsScriptBase64 = "CiRFcnJvckFjdGlvblByZWZlcmVuY2UgPSAnU3RvcCc7CmlmICgtbm90IChUZXN0LVBhdGggIi93b29kcGVja2VyL3NvbWUiKSkgeyBOZXctSXRlbSAtUGF0aCAiL3dvb2RwZWNrZXIvc29tZSIgLUl0ZW1UeXBlIERpcmVjdG9yeSAtRm9yY2UgfCBPdXQtTnVsbCB9OwppZiAoLW5vdCBbRW52aXJvbm1lbnRdOjpHZXRFbnZpcm9ubWVudFZhcmlhYmxlKCdIT01FJykpIHsgW0Vudmlyb25tZW50XTo6U2V0RW52aXJvbm1lbnRWYXJpYWJsZSgnSE9NRScsICdjOlxyb290JykgfTsKaWYgKC1ub3QgKFRlc3QtUGF0aCAiJGVudjpIT01FIikpIHsgTmV3LUl0ZW0gLVBhdGggIiRlbnY6SE9NRSIgLUl0ZW1UeXBlIERpcmVjdG9yeSAtRm9yY2UgfCBPdXQtTnVsbCB9OwppZiAoJEVudjpDSV9ORVRSQ19NQUNISU5FKSB7CiRuZXRyYz1bc3RyaW5nXTo6Rm9ybWF0KCJ7MH1cX25ldHJjIiwkRW52OkhPTUUpOwoibWFjaGluZSAkRW52OkNJX05FVFJDX01BQ0hJTkUiID4+ICRuZXRyYzsKImxvZ2luICRFbnY6Q0lfTkVUUkNfVVNFUk5BTUUiID4+ICRuZXRyYzsKInBhc3N3b3JkICRFbnY6Q0lfTkVUUkNfUEFTU1dPUkQiID4+ICRuZXRyYzsKfTsKW0Vudmlyb25tZW50XTo6U2V0RW52aXJvbm1lbnRWYXJpYWJsZSgiQ0lfTkVUUkNfUEFTU1dPUkQiLCRudWxsKTsKW0Vudmlyb25tZW50XTo6U2V0RW52aXJvbm1lbnRWYXJpYWJsZSgiQ0lfU0NSSVBUIiwkbnVsbCk7CmNkICIvd29vZHBlY2tlci9zb21lIjsKCldyaXRlLU91dHB1dCAoJysgImVjaG8gaGVsbG8gd29ybGQiJyk7CiYgZWNobyBoZWxsbyB3b3JsZDsgaWYgKCRMQVNURVhJVENPREUgLW5lIDApIHtleGl0ICRMQVNURVhJVENPREV9Cg=="
	posixScriptBase64   = "CmlmIFsgLW4gIiRDSV9ORVRSQ19NQUNISU5FIiBdOyB0aGVuCmNhdCA8PEVPRiA+ICRIT01FLy5uZXRyYwptYWNoaW5lICRDSV9ORVRSQ19NQUNISU5FCmxvZ2luICRDSV9ORVRSQ19VU0VSTkFNRQpwYXNzd29yZCAkQ0lfTkVUUkNfUEFTU1dPUkQKRU9GCmNobW9kIDA2MDAgJEhPTUUvLm5ldHJjCmZpCnVuc2V0IENJX05FVFJDX1VTRVJOQU1FCnVuc2V0IENJX05FVFJDX1BBU1NXT1JECnVuc2V0IENJX1NDUklQVApta2RpciAtcCAiL3dvb2RwZWNrZXIvc29tZSIKY2QgIi93b29kcGVja2VyL3NvbWUiCgplY2hvICsgJ2VjaG8gaGVsbG8gd29ybGQnCmVjaG8gaGVsbG8gd29ybGQK"
)

func TestGenerateContainerConf(t *testing.T) {
	gotEnv, gotEntry := GenerateContainerConf([]string{"echo hello world"}, "windows", "/woodpecker/some")
	assert.Equal(t, windowsScriptBase64, gotEnv["CI_SCRIPT"])
	assert.Equal(t, "pwsh.exe", gotEnv["SHELL"])
	assert.Equal(t, []string{
		"pwsh", "-NoProfile", "-NonInteractive", "-Command",
		`Invoke-Expression([System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($env:CI_SCRIPT)))`,
	}, gotEntry)
	gotEnv, gotEntry = GenerateContainerConf([]string{"echo hello world"}, "linux", "/woodpecker/some")
	assert.Equal(t, posixScriptBase64, gotEnv["CI_SCRIPT"])
	assert.Equal(t, "/bin/sh", gotEnv["SHELL"])
	assert.Equal(t, []string{"/bin/sh", "-c", "echo $CI_SCRIPT | base64 -d | /bin/sh -e"}, gotEntry)
}
