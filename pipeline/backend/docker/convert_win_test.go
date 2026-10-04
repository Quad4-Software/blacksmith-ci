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

package docker

import (
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

func TestMustNotAddWindowsLetterPattern(t *testing.T) {
	tests := map[string]bool{
		`C:\Users`:           true,
		`D:\Data`:            true,
		`\\.\PhysicalDrive0`: true,
		`//./COM1`:           true,
		`E:`:                 true,
		`\\server\share`:     true, // UNC path
		`.\relative\path`:    true, // Relative path
		`./path`:             true, // Relative with forward slash
		`//server/share`:     true, // UNC with forward slashes
		`not/a/windows/path`: false,
		``:                   false,
		`/usr/local`:         false,
		`COM1`:               false,
		`\\.`:                false, // Incomplete device path
		`//`:                 false,
	}

	for testCase, expected := range tests {
		result := MustNotAddWindowsLetterPattern.MatchString(testCase)
		if result != expected {
			t.Errorf("Test case %q: expected %v but got %v", testCase, expected, result)
		}
	}
}

func TestAppendImageTagSuffix(t *testing.T) {
	tests := []struct {
		image string
		want  string
	}{
		{"codefloe.com/crow-plugins/clone:1.0.4", "codefloe.com/crow-plugins/clone:1.0.4-windows-ltsc2022"},
		{"reg.example.com:5000/foo/bar:2.1", "reg.example.com:5000/foo/bar:2.1-windows-ltsc2022"},
		{"alpine", "alpine:latest-windows-ltsc2022"},
		{"foo/bar@sha256:abc123", "foo/bar@sha256:abc123"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, appendImageTagSuffix(tc.image, "windows-ltsc2022"), "image %q", tc.image)
	}
}

func TestParseIsolation(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    container.Isolation
		wantErr bool
	}{
		{name: "empty", value: "", want: container.IsolationEmpty},
		{name: "default", value: "default", want: container.IsolationDefault},
		{name: "process", value: "process", want: container.IsolationProcess},
		{name: "hyperv", value: "hyperv", want: container.IsolationHyperV},
		{name: "mixed case and padding", value: "  HyperV ", want: container.IsolationHyperV},
		{name: "unknown mode", value: "hyper-v", wantErr: true},
		{name: "kubernetes-style value", value: "gvisor", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIsolation(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveIsolation(t *testing.T) {
	tests := []struct {
		name    string
		agent   container.Isolation
		step    container.Isolation
		want    container.Isolation
		wantErr bool
	}{
		{
			name: "neither set leaves the daemon to decide",
		},
		{
			name:  "step inherits the agent default",
			agent: container.IsolationHyperV,
			want:  container.IsolationHyperV,
		},
		{
			name: "step may raise an unset agent to hyperv",
			step: container.IsolationHyperV,
			want: container.IsolationHyperV,
		},
		{
			name:  "step may raise process to hyperv",
			agent: container.IsolationProcess,
			step:  container.IsolationHyperV,
			want:  container.IsolationHyperV,
		},
		{
			name:    "step may not weaken hyperv to process",
			agent:   container.IsolationHyperV,
			step:    container.IsolationProcess,
			wantErr: true,
		},
		{
			name:    "step may not weaken hyperv to default",
			agent:   container.IsolationHyperV,
			step:    container.IsolationDefault,
			wantErr: true,
		},
		{
			name:  "step may weaken when the agent did not ask for hyperv",
			agent: container.IsolationProcess,
			step:  container.IsolationDefault,
			want:  container.IsolationDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveIsolation(tt.agent, tt.step)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateIsolationForOS(t *testing.T) {
	assert.NoError(t, validateIsolationForOS("linux", container.IsolationEmpty))
	assert.NoError(t, validateIsolationForOS("linux", container.IsolationDefault))
	assert.NoError(t, validateIsolationForOS("windows", container.IsolationHyperV))
	assert.NoError(t, validateIsolationForOS("windows", container.IsolationProcess))
	assert.Error(t, validateIsolationForOS("linux", container.IsolationHyperV))
	assert.Error(t, validateIsolationForOS("linux", container.IsolationProcess))
}

func TestToHostConfigIsolation(t *testing.T) {
	t.Run("agent default applies to every step", func(t *testing.T) {
		got, err := toHostConfig(&types.Step{}, BackendOptions{}, &config{isolation: container.IsolationHyperV}, osTypeWindows)
		assert.NoError(t, err)
		assert.Equal(t, container.IsolationHyperV, got.Isolation)
	})

	t.Run("step raises itself to hyperv", func(t *testing.T) {
		got, err := toHostConfig(&types.Step{}, BackendOptions{Isolation: "hyperv"}, &config{}, osTypeWindows)
		assert.NoError(t, err)
		assert.Equal(t, container.IsolationHyperV, got.Isolation)
	})

	t.Run("step cannot escape the agent's hyperv floor", func(t *testing.T) {
		_, err := toHostConfig(&types.Step{}, BackendOptions{Isolation: "process"}, &config{isolation: container.IsolationHyperV}, osTypeWindows)
		assert.Error(t, err)
	})

	t.Run("invalid step value is rejected", func(t *testing.T) {
		_, err := toHostConfig(&types.Step{}, BackendOptions{Isolation: "nope"}, &config{}, osTypeWindows)
		assert.Error(t, err)
	})

	t.Run("hyperv is rejected on a non-windows daemon", func(t *testing.T) {
		_, err := toHostConfig(&types.Step{}, BackendOptions{Isolation: "hyperv"}, &config{}, "linux")
		assert.Error(t, err)
	})

	t.Run("unset isolation stays unset on linux", func(t *testing.T) {
		got, err := toHostConfig(&types.Step{}, BackendOptions{}, &config{}, "linux")
		assert.NoError(t, err)
		assert.Equal(t, container.IsolationEmpty, got.Isolation)
	})
}
