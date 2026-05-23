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

//go:build !windows

package podman

import (
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

// TestToResourceLimits tests conversion to OCI spec.LinuxResources.
// This is Podman-specific as Docker uses different types.
func TestToResourceLimits(t *testing.T) {
	tests := []struct {
		name     string
		config   config
		expected func(t *testing.T, result *spec.LinuxResources)
	}{
		{
			name:   "no limits returns nil",
			config: config{},
			expected: func(t *testing.T, result *spec.LinuxResources) {
				assert.Nil(t, result)
			},
		},
		{
			name: "memory limit",
			config: config{
				resourceLimit: resourceLimit{
					MemLimit: 512 * 1024 * 1024,
				},
			},
			expected: func(t *testing.T, result *spec.LinuxResources) {
				assert.NotNil(t, result)
				assert.NotNil(t, result.Memory)
				assert.Equal(t, int64(512*1024*1024), *result.Memory.Limit)
				assert.Nil(t, result.CPU)
			},
		},
		{
			name: "memory swap limit",
			config: config{
				resourceLimit: resourceLimit{
					MemSwapLimit: 1024 * 1024 * 1024,
				},
			},
			expected: func(t *testing.T, result *spec.LinuxResources) {
				assert.NotNil(t, result)
				assert.NotNil(t, result.Memory)
				assert.Equal(t, int64(1024*1024*1024), *result.Memory.Swap)
			},
		},
		{
			name: "cpu quota",
			config: config{
				resourceLimit: resourceLimit{
					CPUQuota: 100000,
				},
			},
			expected: func(t *testing.T, result *spec.LinuxResources) {
				assert.NotNil(t, result)
				assert.NotNil(t, result.CPU)
				assert.Equal(t, int64(100000), *result.CPU.Quota)
			},
		},
		{
			name: "cpu shares",
			config: config{
				resourceLimit: resourceLimit{
					CPUShares: 512,
				},
			},
			expected: func(t *testing.T, result *spec.LinuxResources) {
				assert.NotNil(t, result)
				assert.NotNil(t, result.CPU)
				assert.Equal(t, uint64(512), *result.CPU.Shares)
			},
		},
		{
			name: "cpu set",
			config: config{
				resourceLimit: resourceLimit{
					CPUSet: "0,1",
				},
			},
			expected: func(t *testing.T, result *spec.LinuxResources) {
				assert.NotNil(t, result)
				assert.NotNil(t, result.CPU)
				assert.Equal(t, "0,1", result.CPU.Cpus)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toResourceLimits(&tt.config)
			tt.expected(t, result)
		})
	}
}

// TestParseIPAddresses tests IP string to net.IP conversion.
// This is Podman-specific as it requires net.IP types for DNS servers.
func TestParseIPAddresses(t *testing.T) {
	tests := []struct {
		name     string
		ips      []string
		expected int
	}{
		{"nil", nil, 0},
		{"empty", []string{}, 0},
		{"single ipv4", []string{"8.8.8.8"}, 1},
		{"multiple ipv4", []string{"8.8.8.8", "1.1.1.1"}, 2},
		{"ipv6", []string{"2001:4860:4860::8888"}, 1},
		{"invalid filtered", []string{"invalid", "8.8.8.8"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseIPAddresses(tt.ips)
			assert.Len(t, result, tt.expected)
		})
	}
}
