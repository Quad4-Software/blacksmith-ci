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

package docker

import (
	"encoding/base64"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/stretchr/testify/assert"

	backend "codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

func TestSplitVolumeParts(t *testing.T) {
	testdata := []struct {
		from    string
		to      []string
		success bool
	}{
		{
			from:    `Z::Z::rw`,
			to:      []string{`Z:`, `Z:`, `rw`},
			success: true,
		},
		{
			from:    `Z:\:Z:\:rw`,
			to:      []string{`Z:\`, `Z:\`, `rw`},
			success: true,
		},
		{
			from:    `Z:\git\refs:Z:\git\refs:rw`,
			to:      []string{`Z:\git\refs`, `Z:\git\refs`, `rw`},
			success: true,
		},
		{
			from:    `Z:\git\refs:Z:\git\refs`,
			to:      []string{`Z:\git\refs`, `Z:\git\refs`},
			success: true,
		},
		{
			from:    `Z:/:Z:/:rw`,
			to:      []string{`Z:/`, `Z:/`, `rw`},
			success: true,
		},
		{
			from:    `Z:/git/refs:Z:/git/refs:rw`,
			to:      []string{`Z:/git/refs`, `Z:/git/refs`, `rw`},
			success: true,
		},
		{
			from:    `Z:/git/refs:Z:/git/refs`,
			to:      []string{`Z:/git/refs`, `Z:/git/refs`},
			success: true,
		},
		{
			from:    `/test:/test`,
			to:      []string{`/test`, `/test`},
			success: true,
		},
		{
			from:    `test:/test`,
			to:      []string{`test`, `/test`},
			success: true,
		},
		{
			from:    `test:test`,
			to:      []string{`test`, `test`},
			success: true,
		},
	}
	for _, test := range testdata {
		results, err := splitVolumeParts(test.from)
		if test.success != (err == nil) {
			assert.Equal(t, test.success, reflect.DeepEqual(results, test.to))
		}
	}
}

// dummy vars to test against.
var (
	testCmdStep = &backend.Step{
		Name:        "hello",
		UUID:        "f51821af-4cb8-435e-a3c2-3a684185d828",
		Type:        backend.StepTypeCommands,
		Commands:    []string{"echo \"hello world\"", "ls"},
		Image:       "alpine",
		Environment: map[string]string{"SHELL": "/bin/zsh"},
	}

	testPluginStep = &backend.Step{
		Name:        "lint",
		UUID:        "d841ee40-e66e-4275-bb3f-55bf89744b21",
		Type:        backend.StepTypePlugin,
		Image:       "mstruebing/editorconfig-checker",
		Environment: make(map[string]string),
	}

	testEngine = &docker{
		info: system.Info{
			Architecture:    "x86_64",
			OSType:          "linux",
			DefaultRuntime:  "runc",
			DockerRootDir:   "/var/lib/docker",
			OperatingSystem: "Archlinux",
			Name:            "SOME_HOSTNAME",
		},
	}
)

func TestToContainerName(t *testing.T) {
	assert.EqualValues(t, "crow_f51821af-4cb8-435e-a3c2-3a684185d828", toContainerName(testCmdStep))
	assert.EqualValues(t, "crow_d841ee40-e66e-4275-bb3f-55bf89744b21", toContainerName(testPluginStep))

	descriptiveStep := &backend.Step{
		Name:                "build",
		UUID:                "irrelevant",
		ContainerNameScheme: "descriptive",
		Environment: map[string]string{
			"CI_REPO_OWNER":      "myowner",
			"CI_REPO_NAME":       "myrepo",
			"CI_PIPELINE_NUMBER": "123",
			"CI_WORKFLOW_NUMBER": "1",
		},
	}
	assert.EqualValues(t, "myowner-myrepo-123-1-build", toContainerName(descriptiveStep))

	// Test fallback to hash when environment variables are incomplete
	stepIncomplete := &backend.Step{
		Name:                "build",
		UUID:                "abcdefghijk",
		ContainerNameScheme: "descriptive",
		Environment: map[string]string{
			"CI_REPO_OWNER": "myowner",
			"CI_REPO_NAME":  "myrepo",
			// Missing CI_PIPELINE_NUMBER
			"CI_WORKFLOW_NUMBER": "1",
		},
	}
	assert.EqualValues(t, "crow_abcdefghijk", toContainerName(stepIncomplete))

	// Test truncation
	longRepo := &backend.Step{
		Name:                "build",
		UUID:                "irrelevant",
		ContainerNameScheme: "descriptive",
		Environment: map[string]string{
			"CI_REPO_OWNER":      strings.Repeat("a", 35),
			"CI_REPO_NAME":       strings.Repeat("a", 35),
			"CI_PIPELINE_NUMBER": "123",
			"CI_WORKFLOW_NUMBER": "456",
		},
	}
	name := toContainerName(longRepo)
	if len(name) > 64 {
		t.Errorf("container name too long: %d chars", len(name))
	}
}

func TestStepToConfig(t *testing.T) {
	taskUUID := "test-task-uuid"

	// StepTypeCommands
	conf := testEngine.toConfig(testCmdStep, BackendOptions{}, taskUUID)
	if assert.NotNil(t, conf) {
		assert.EqualValues(t, []string{"/bin/sh", "-c", "echo $CI_SCRIPT | base64 -d | /bin/sh -e"}, conf.Entrypoint)
		assert.Nil(t, conf.Cmd)
		assert.EqualValues(t, testCmdStep.UUID, conf.Labels["wp_uuid"])
		assert.EqualValues(t, testCmdStep.UUID, conf.Labels[LabelStepUUID])
		assert.EqualValues(t, taskUUID, conf.Labels[LabelTaskUUID])
	}

	// StepTypePlugin
	conf = testEngine.toConfig(testPluginStep, BackendOptions{}, taskUUID)
	if assert.NotNil(t, conf) {
		assert.Nil(t, conf.Cmd)
		assert.EqualValues(t, testPluginStep.UUID, conf.Labels["wp_uuid"])
		assert.EqualValues(t, testPluginStep.UUID, conf.Labels[LabelStepUUID])
		assert.EqualValues(t, taskUUID, conf.Labels[LabelTaskUUID])
	}
}

func TestToEnv(t *testing.T) {
	assert.Nil(t, toEnv(nil))
	assert.EqualValues(t, []string{"A=B"}, toEnv(map[string]string{"A": "B"}))
	assert.ElementsMatch(t, []string{"A=B=C", "T=T"}, toEnv(map[string]string{"A": "B=C", "": "Z", "T": "T"}))
}

func TestToVol(t *testing.T) {
	assert.Nil(t, toVol(nil))
	assert.EqualValues(t, map[string]struct{}{"/test": {}}, toVol([]string{"test:/test"}))
}

func TestToVolSkipsWindowsNamedPipe(t *testing.T) {
	// Windows named-pipe binds (docker-out-of-docker, e.g. \\.\pipe\docker_engine)
	// must NOT be declared as anonymous volumes: the Windows daemon validates
	// anonymous-volume mount points as directories and rejects a pipe with
	// "Only directories can be mapped on this platform". The pipe is still
	// forwarded via HostConfig.Binds, so it remains usable as a bind mount.
	_, exists := toVol([]string{`\\.\pipe\docker_engine:\\.\pipe\docker_engine`})[`\\.\pipe\docker_engine`]
	assert.False(t, exists, "named pipe destination must not become an anonymous volume")

	// real binds alongside a pipe keep their destinations; only the pipe is dropped
	assert.EqualValues(t, map[string]struct{}{"/test": {}},
		toVol([]string{`\\.\pipe\docker_engine:\\.\pipe\docker_engine`, "test:/test"}))
}

func TestEncodeAuthToBase64(t *testing.T) {
	res, err := encodeAuthToBase64(backend.Auth{})
	assert.NoError(t, err)
	assert.EqualValues(t, "e30=", res)

	res, err = encodeAuthToBase64(backend.Auth{Username: "user", Password: "pwd"})
	assert.NoError(t, err)
	assert.EqualValues(t, "eyJ1c2VybmFtZSI6InVzZXIiLCJwYXNzd29yZCI6InB3ZCJ9", res)
}

func TestToConfigSmall(t *testing.T) {
	engine := docker{info: system.Info{OSType: "linux", Architecture: "riscv64"}}

	conf := engine.toConfig(&backend.Step{
		Name:     "test",
		UUID:     "09238932",
		Commands: []string{"go test"},
	}, BackendOptions{}, "task-123")

	assert.NotNil(t, conf)
	sort.Strings(conf.Env)
	assert.EqualValues(t, &container.Config{
		AttachStdout: true,
		AttachStderr: true,
		Entrypoint:   []string{"/bin/sh", "-c", "echo $CI_SCRIPT | base64 -d | /bin/sh -e"},
		Labels: map[string]string{
			"wp_step":     "test",
			"wp_uuid":     "09238932",
			LabelTaskUUID: "task-123",
			LabelStepUUID: "09238932",
			LabelStepName: "test",
		},
		Env: []string{
			"CI_SCRIPT=CmlmIFsgLW4gIiRDSV9ORVRSQ19NQUNISU5FIiBdOyB0aGVuCmNhdCA8PEVPRiA+ICRIT01FLy5uZXRyYwptYWNoaW5lICRDSV9ORVRSQ19NQUNISU5FCmxvZ2luICRDSV9ORVRSQ19VU0VSTkFNRQpwYXNzd29yZCAkQ0lfTkVUUkNfUEFTU1dPUkQKRU9GCmNobW9kIDA2MDAgJEhPTUUvLm5ldHJjCmZpCnVuc2V0IENJX05FVFJDX1VTRVJOQU1FCnVuc2V0IENJX05FVFJDX1BBU1NXT1JECnVuc2V0IENJX1NDUklQVApta2RpciAtcCAiIgpjZCAiIgoKZWNobyArICdnbyB0ZXN0JwpnbyB0ZXN0Cg==",
			"SHELL=/bin/sh",
		},
	}, conf)
}

func TestToConfigFull(t *testing.T) {
	engine := docker{
		info: system.Info{OSType: "linux", Architecture: "riscv64"},
		config: config{
			enableIPv6: true,
			resourceLimit: resourceLimit{
				MemSwapLimit: 12,
				MemLimit:     13,
				ShmSize:      14,
				CPUQuota:     15,
				CPUShares:    16,
			},
		},
	}

	conf := engine.toConfig(&backend.Step{
		Name:          "test",
		UUID:          "09238932",
		Type:          backend.StepTypeCommands,
		Image:         "golang:1.2.3",
		Pull:          true,
		Detached:      true,
		Privileged:    true,
		WorkingDir:    "/src/abc",
		WorkspaceBase: "/src",
		Environment:   map[string]string{"TAGS": "sqlite"},
		Commands:      []string{"go test", "go vet ./..."},
		ExtraHosts:    []backend.HostAlias{{Name: "t", IP: "1.2.3.4"}},
		Volumes:       []string{"/cache:/cache"},
		Tmpfs:         []string{"/tmp"},
		Devices:       []string{"/dev/sdc"},
		Networks:      []backend.Conn{{Name: "extra-net", Aliases: []string{"extra.net"}}},
		DNS:           []string{"9.9.9.9", "8.8.8.8"},
		DNSSearch:     nil,
		OnFailure:     true,
		OnSuccess:     true,
		Failure:       "fail",
		AuthConfig:    backend.Auth{Username: "user", Password: "123456"},
		NetworkMode:   "bridge",
		Ports:         []backend.Port{{Number: 21}, {Number: 22}},
	}, BackendOptions{}, "task-456")

	assert.NotNil(t, conf)
	sort.Strings(conf.Env)
	assert.EqualValues(t, &container.Config{
		Image:        "golang:1.2.3",
		WorkingDir:   "/src",
		AttachStdout: true,
		AttachStderr: true,
		Entrypoint:   []string{"/bin/sh", "-c", "echo $CI_SCRIPT | base64 -d | /bin/sh -e"},
		Labels: map[string]string{
			"wp_step":     "test",
			"wp_uuid":     "09238932",
			LabelTaskUUID: "task-456",
			LabelStepUUID: "09238932",
			LabelStepName: "test",
		},
		Env: []string{
			"CI_SCRIPT=CmlmIFsgLW4gIiRDSV9ORVRSQ19NQUNISU5FIiBdOyB0aGVuCmNhdCA8PEVPRiA+ICRIT01FLy5uZXRyYwptYWNoaW5lICRDSV9ORVRSQ19NQUNISU5FCmxvZ2luICRDSV9ORVRSQ19VU0VSTkFNRQpwYXNzd29yZCAkQ0lfTkVUUkNfUEFTU1dPUkQKRU9GCmNobW9kIDA2MDAgJEhPTUUvLm5ldHJjCmZpCnVuc2V0IENJX05FVFJDX1VTRVJOQU1FCnVuc2V0IENJX05FVFJDX1BBU1NXT1JECnVuc2V0IENJX1NDUklQVApta2RpciAtcCAiL3NyYy9hYmMiCmNkICIvc3JjL2FiYyIKCmVjaG8gKyAnZ28gdGVzdCcKZ28gdGVzdAoKZWNobyArICdnbyB2ZXQgLi8uLi4nCmdvIHZldCAuLy4uLgo=",
			"SHELL=/bin/sh",
			"TAGS=sqlite",
		},
		Volumes: map[string]struct{}{
			"/cache": {},
		},
	}, conf)
}

func TestParseTmpfs(t *testing.T) {
	tests := []struct {
		name     string
		entries  []string
		memLimit int64
		expected map[string]string
	}{
		{
			name:     "simple path no limit",
			entries:  []string{"/tmp"},
			memLimit: 0,
			expected: map[string]string{"/tmp": ""},
		},
		{
			name:     "path with size no limit",
			entries:  []string{"/tmp:size=2G"},
			memLimit: 0,
			expected: map[string]string{"/tmp": "size=2G"},
		},
		{
			name:     "simple path with limit injects size",
			entries:  []string{"/tmp"},
			memLimit: 512 * 1024 * 1024, // 512M
			expected: map[string]string{"/tmp": "size=512M"},
		},
		{
			name:     "size within limit passes through",
			entries:  []string{"/tmp:size=256M"},
			memLimit: 512 * 1024 * 1024,
			expected: map[string]string{"/tmp": "size=256M"},
		},
		{
			name:     "size exceeding limit gets capped",
			entries:  []string{"/tmp:size=2G"},
			memLimit: 512 * 1024 * 1024,
			expected: map[string]string{"/tmp": "size=512M"},
		},
		{
			name:     "size with other options preserved",
			entries:  []string{"/var/lib/data:size=4G,mode=1770"},
			memLimit: 1024 * 1024 * 1024, // 1G
			expected: map[string]string{"/var/lib/data": "size=1G,mode=1770"},
		},
		{
			name:     "no size with other options adds size",
			entries:  []string{"/tmp:mode=1770"},
			memLimit: 256 * 1024 * 1024,
			expected: map[string]string{"/tmp": "mode=1770,size=256M"},
		},
		{
			name:     "multiple entries",
			entries:  []string{"/tmp", "/var/lib/data:size=100M"},
			memLimit: 512 * 1024 * 1024,
			expected: map[string]string{"/tmp": "size=512M", "/var/lib/data": "size=100M"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTmpfs(tt.entries, tt.memLimit)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatBytes(t *testing.T) {
	assert.Equal(t, "1G", formatBytes(1024*1024*1024))
	assert.Equal(t, "512M", formatBytes(512*1024*1024))
	assert.Equal(t, "2G", formatBytes(2*1024*1024*1024))
	assert.Equal(t, "1536M", formatBytes(1536*1024*1024))
	assert.Equal(t, "1000000", formatBytes(1000000)) // not aligned to MiB
}

func TestToWindowsConfig(t *testing.T) {
	engine := docker{
		info: system.Info{OSType: "windows", Architecture: "x86_64"},
		config: config{
			enableIPv6: true,
		},
	}

	conf := engine.toConfig(&backend.Step{
		Name:          "test",
		UUID:          "23434553",
		Type:          backend.StepTypeCommands,
		Image:         "golang:1.2.3",
		WorkingDir:    "/src/abc",
		WorkspaceBase: "/src",
		Environment: map[string]string{
			"TAGS":         "sqlite",
			"CI_WORKSPACE": "/src",
		},
		Commands:    []string{"go test", "go vet ./..."},
		ExtraHosts:  []backend.HostAlias{{Name: "t", IP: "1.2.3.4"}},
		Volumes:     []string{"wp_default_abc:/src", "/cache:/cache/some/more", "test:/test"},
		Networks:    []backend.Conn{{Name: "extra-net", Aliases: []string{"extra.net"}}},
		DNS:         []string{"9.9.9.9", "8.8.8.8"},
		Failure:     "fail",
		AuthConfig:  backend.Auth{Username: "user", Password: "123456"},
		NetworkMode: "nat",
		Ports:       []backend.Port{{Number: 21}, {Number: 22}},
	}, BackendOptions{}, "task-789")

	assert.NotNil(t, conf)
	sort.Strings(conf.Env)
	assert.EqualValues(t, &container.Config{
		Image:        "golang:1.2.3",
		WorkingDir:   "C:/src",
		AttachStdout: true,
		AttachStderr: true,
		Entrypoint: []string{
			"pwsh", "-NoProfile", "-NonInteractive", "-Command",
			`Invoke-Expression([System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($env:CI_SCRIPT)))`,
		},
		Labels: map[string]string{
			"wp_step":     "test",
			"wp_uuid":     "23434553",
			LabelTaskUUID: "task-789",
			LabelStepUUID: "23434553",
			LabelStepName: "test",
		},
		Env: []string{
			"CI_SCRIPT=CiRFcnJvckFjdGlvblByZWZlcmVuY2UgPSAnU3RvcCc7CmlmICgtbm90IChUZXN0LVBhdGggIkM6L3NyYy9hYmMiKSkgeyBOZXctSXRlbSAtUGF0aCAiQzovc3JjL2FiYyIgLUl0ZW1UeXBlIERpcmVjdG9yeSAtRm9yY2UgfCBPdXQtTnVsbCB9OwppZiAoLW5vdCBbRW52aXJvbm1lbnRdOjpHZXRFbnZpcm9ubWVudFZhcmlhYmxlKCdIT01FJykpIHsgW0Vudmlyb25tZW50XTo6U2V0RW52aXJvbm1lbnRWYXJpYWJsZSgnSE9NRScsICdjOlxyb290JykgfTsKaWYgKC1ub3QgKFRlc3QtUGF0aCAiJGVudjpIT01FIikpIHsgTmV3LUl0ZW0gLVBhdGggIiRlbnY6SE9NRSIgLUl0ZW1UeXBlIERpcmVjdG9yeSAtRm9yY2UgfCBPdXQtTnVsbCB9OwppZiAoJEVudjpDSV9ORVRSQ19NQUNISU5FKSB7CiRuZXRyYz1bc3RyaW5nXTo6Rm9ybWF0KCJ7MH1cX25ldHJjIiwkRW52OkhPTUUpOwoibWFjaGluZSAkRW52OkNJX05FVFJDX01BQ0hJTkUiID4+ICRuZXRyYzsKImxvZ2luICRFbnY6Q0lfTkVUUkNfVVNFUk5BTUUiID4+ICRuZXRyYzsKInBhc3N3b3JkICRFbnY6Q0lfTkVUUkNfUEFTU1dPUkQiID4+ICRuZXRyYzsKfTsKW0Vudmlyb25tZW50XTo6U2V0RW52aXJvbm1lbnRWYXJpYWJsZSgiQ0lfTkVUUkNfUEFTU1dPUkQiLCRudWxsKTsKW0Vudmlyb25tZW50XTo6U2V0RW52aXJvbm1lbnRWYXJpYWJsZSgiQ0lfU0NSSVBUIiwkbnVsbCk7CmNkICJDOi9zcmMvYWJjIjsKCldyaXRlLU91dHB1dCAoJysgImdvIHRlc3QiJyk7CmdvIHRlc3QKaWYgKCRMQVNURVhJVENPREUgLW5lIDApIHtleGl0ICRMQVNURVhJVENPREV9CgpXcml0ZS1PdXRwdXQgKCcrICJnbyB2ZXQgLi8uLi4iJyk7CmdvIHZldCAuLy4uLgppZiAoJExBU1RFWElUQ09ERSAtbmUgMCkge2V4aXQgJExBU1RFWElUQ09ERX0K",
			"CI_WORKSPACE=C:/src",
			"SHELL=pwsh.exe",
			"TAGS=sqlite",
		},
		Volumes: map[string]struct{}{
			"C:/cache/some/more": {},
			"C:/src":             {},
			"C:/test":            {},
		},
	}, conf)

	ciScript, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(conf.Env[0], "CI_SCRIPT="))
	if assert.NoError(t, err) {
		assert.EqualValues(t, `
$ErrorActionPreference = 'Stop';
if (-not (Test-Path "C:/src/abc")) { New-Item -Path "C:/src/abc" -ItemType Directory -Force | Out-Null };
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
cd "C:/src/abc";

Write-Output ('+ "go test"');
go test
if ($LASTEXITCODE -ne 0) {exit $LASTEXITCODE}

Write-Output ('+ "go vet ./..."');
go vet ./...
if ($LASTEXITCODE -ne 0) {exit $LASTEXITCODE}
`, string(ciScript))
	}
}
