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

package tart

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

func confWithOptions(opts map[string]any) *types.Config {
	return &types.Config{Stages: []*types.Stage{{
		Steps: []*types.Step{{
			Name:           "first",
			BackendOptions: map[string]any{EngineName: opts},
		}},
	}}}
}

func TestParseBackendOptions(t *testing.T) {
	opts, err := parseBackendOptions(&types.Step{
		BackendOptions: map[string]any{EngineName: map[string]any{
			"image":  "ghcr.io/example/xcode:16",
			"cpu":    8,
			"memory": "16Gi",
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/example/xcode:16", opts.Image)
	assert.Equal(t, 8, opts.CPU)
	assert.Equal(t, "16Gi", opts.Memory)
}

func TestParseBackendOptionsEmpty(t *testing.T) {
	opts, err := parseBackendOptions(&types.Step{})
	require.NoError(t, err)
	assert.Equal(t, BackendOptions{}, opts)
}

func TestResolveIgnoresStepImageByDefault(t *testing.T) {
	cfg := config{Image: "agent-image"}
	spec, err := resolveVMOptions(cfg, confWithOptions(map[string]any{"image": "repo-chosen-image"}))
	require.NoError(t, err)
	assert.Equal(t, "agent-image", spec.Image, "a repo must not pick the VM image unless the agent allows it")
}

func TestResolveHonorsStepImageWhenAllowed(t *testing.T) {
	cfg := config{Image: "agent-image", AllowImageFromStep: true}
	spec, err := resolveVMOptions(cfg, confWithOptions(map[string]any{"image": "repo-chosen-image"}))
	require.NoError(t, err)
	assert.Equal(t, "repo-chosen-image", spec.Image)
}

func TestResolveClampsCPUAndMemoryToAgentMaxima(t *testing.T) {
	cfg := config{Image: "img", CPU: 4, MemoryMB: 8192}
	spec, err := resolveVMOptions(cfg, confWithOptions(map[string]any{"cpu": 32, "memory": "64Gi"}))
	require.NoError(t, err)
	assert.Equal(t, 4, spec.CPU)
	assert.Equal(t, 8192, spec.MemoryMB)
}

func TestResolveAllowsSmallerRequests(t *testing.T) {
	cfg := config{Image: "img", CPU: 8, MemoryMB: 16384}
	spec, err := resolveVMOptions(cfg, confWithOptions(map[string]any{"cpu": 2, "memory": "4Gi"}))
	require.NoError(t, err)
	assert.Equal(t, 2, spec.CPU)
	assert.Equal(t, 4096, spec.MemoryMB)
}

func TestResolveUsesAgentValuesWhenStepIsSilent(t *testing.T) {
	cfg := config{Image: "img", CPU: 4, MemoryMB: 8192}
	spec, err := resolveVMOptions(cfg, &types.Config{})
	require.NoError(t, err)
	assert.Equal(t, "img", spec.Image)
	assert.Equal(t, 4, spec.CPU)
	assert.Equal(t, 8192, spec.MemoryMB)
}

func TestResolveFirstStepWins(t *testing.T) {
	cfg := config{Image: "img", AllowImageFromStep: true}
	conf := &types.Config{Stages: []*types.Stage{{Steps: []*types.Step{
		{Name: "a", BackendOptions: map[string]any{EngineName: map[string]any{"image": "first"}}},
		{Name: "b", BackendOptions: map[string]any{EngineName: map[string]any{"image": "second"}}},
	}}}}
	spec, err := resolveVMOptions(cfg, conf)
	require.NoError(t, err)
	assert.Equal(t, "first", spec.Image, "one VM per workflow means the first step to ask decides")
}

func TestResolveRejectsBadMemory(t *testing.T) {
	_, err := resolveVMOptions(config{Image: "img"}, confWithOptions(map[string]any{"memory": "many"}))
	assert.ErrorContains(t, err, "memory")
}
