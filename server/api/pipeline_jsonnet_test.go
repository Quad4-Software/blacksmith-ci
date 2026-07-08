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

package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml"
	yamlBaseTypes "codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/types/base"
	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	forge_types "codefloe.com/crowci/crow/v6/server/forge/types"
	"codefloe.com/crowci/crow/v6/server/model"
)

func TestPreprocessJsonnetConfigs(t *testing.T) {
	mockForge := new(forge_mocks.MockForge)
	mockForge.On("Name").Return("mock")
	mockForge.On("URL").Return("https://codefloe.com")

	repo := &model.Repo{FullName: "owner/repo"}
	pl := &model.Pipeline{Event: model.EventManual, Branch: "main"}

	t.Run("PassThroughYAMLFiles", func(t *testing.T) {
		files := []*forge_types.FileMeta{
			{Name: ".crow.yaml", Data: []byte("steps:\n  - name: test\n    image: alpine")},
		}

		result, err := preprocessJsonnetConfigs(files, repo, pl, mockForge)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, ".crow.yaml", result[0].Name)
		assert.Equal(t, files[0].Data, result[0].Data)
	})

	t.Run("EvaluateJsonnetToYAML", func(t *testing.T) {
		files := []*forge_types.FileMeta{
			{Name: ".crow.jsonnet", Data: []byte(`{
				steps: [
					{name: "test", image: "alpine", commands: ["echo hi"]},
				],
			}`)},
		}

		result, err := preprocessJsonnetConfigs(files, repo, pl, mockForge)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, ".crow.jsonnet", result[0].Name)
		assert.Contains(t, string(result[0].Data), "name: test")
		assert.Contains(t, string(result[0].Data), "image: alpine")
	})

	t.Run("JsonnetWithManualEvent", func(t *testing.T) {
		// This is the exact scenario from issue #852
		files := []*forge_types.FileMeta{
			{Name: ".crow.jsonnet", Data: []byte(`{
				when: [
					{ event: 'manual' },
				],
				steps: [
					{
						name: 'Flake Check',
						image: 'docker.io/nixos/nix',
						commands: [
							'echo "experimental-features = nix-command flakes" > /etc/nix/nix.conf',
							'nix flake check',
						],
					},
				],
			}`)},
		}

		result, err := preprocessJsonnetConfigs(files, repo, pl, mockForge)
		require.NoError(t, err)
		require.Len(t, result, 1)

		// Parse the resulting YAML and verify manual trigger is detectable
		parsed, err := yaml.ParseBytes(result[0].Data)
		require.NoError(t, err)

		hasManual := workflowHasManualTrigger(parsed.When)
		assert.True(t, hasManual, "manual trigger should be detected in Jsonnet-produced YAML")
	})

	t.Run("JsonnetWithoutManualEvent", func(t *testing.T) {
		files := []*forge_types.FileMeta{
			{Name: ".crow.jsonnet", Data: []byte(`{
				when: [
					{ event: 'push' },
				],
				steps: [
					{name: "test", image: "alpine"},
				],
			}`)},
		}

		result, err := preprocessJsonnetConfigs(files, repo, pl, mockForge)
		require.NoError(t, err)
		require.Len(t, result, 1)

		parsed, err := yaml.ParseBytes(result[0].Data)
		require.NoError(t, err)

		hasManual := workflowHasManualTrigger(parsed.When)
		assert.False(t, hasManual, "push-only workflow should not have manual trigger")
	})

	t.Run("MultiDocumentJsonnet", func(t *testing.T) {
		files := []*forge_types.FileMeta{
			{Name: ".crow.jsonnet", Data: []byte(`[
				{
					when: [{event: 'manual'}],
					steps: [{name: "deploy", image: "alpine"}],
				},
				{
					when: [{event: 'push'}],
					steps: [{name: "test", image: "golang"}],
				},
			]`)},
		}

		result, err := preprocessJsonnetConfigs(files, repo, pl, mockForge)
		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, ".crow-0.yaml", result[0].Name)
		assert.Equal(t, ".crow-1.yaml", result[1].Name)

		// First doc should have manual trigger
		parsed0, err := yaml.ParseBytes(result[0].Data)
		require.NoError(t, err)
		assert.True(t, workflowHasManualTrigger(parsed0.When))

		// Second doc should not
		parsed1, err := yaml.ParseBytes(result[1].Data)
		require.NoError(t, err)
		assert.False(t, workflowHasManualTrigger(parsed1.When))
	})

	t.Run("SkipsLibsonnetFiles", func(t *testing.T) {
		files := []*forge_types.FileMeta{
			{Name: ".crow/helpers.libsonnet", Data: []byte(`{helper: true}`)},
			{Name: ".crow.yaml", Data: []byte("steps:\n  - name: test\n    image: alpine")},
		}

		result, err := preprocessJsonnetConfigs(files, repo, pl, mockForge)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, ".crow.yaml", result[0].Name)
	})

	t.Run("JsonnetWithImports", func(t *testing.T) {
		files := []*forge_types.FileMeta{
			{Name: ".crow/images.libsonnet", Data: []byte(`{nixImage: "docker.io/nixos/nix"}`)},
			{Name: ".crow/build.jsonnet", Data: []byte(`
				local images = import "images.libsonnet";
				{
					when: [{event: 'manual'}],
					steps: [{name: "build", image: images.nixImage}],
				}
			`)},
		}

		result, err := preprocessJsonnetConfigs(files, repo, pl, mockForge)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Contains(t, string(result[0].Data), "docker.io/nixos/nix")

		parsed, err := yaml.ParseBytes(result[0].Data)
		require.NoError(t, err)
		assert.True(t, workflowHasManualTrigger(parsed.When))
	})

	t.Run("MixedYAMLAndJsonnet", func(t *testing.T) {
		files := []*forge_types.FileMeta{
			{Name: ".crow/test.yaml", Data: []byte("when:\n  - event: push\nsteps:\n  - name: test\n    image: alpine")},
			{Name: ".crow/deploy.jsonnet", Data: []byte(`{
				when: [{event: 'manual'}],
				steps: [{name: "deploy", image: "alpine"}],
			}`)},
		}

		result, err := preprocessJsonnetConfigs(files, repo, pl, mockForge)
		require.NoError(t, err)
		require.Len(t, result, 2)

		// YAML file passes through unchanged
		assert.Equal(t, ".crow/test.yaml", result[0].Name)

		// Jsonnet file is evaluated
		assert.Equal(t, ".crow/deploy.jsonnet", result[1].Name)
		assert.Contains(t, string(result[1].Data), "name: deploy")
	})

	t.Run("JsonnetSyntaxError", func(t *testing.T) {
		files := []*forge_types.FileMeta{
			{Name: ".crow.jsonnet", Data: []byte(`{ invalid syntax here`)},
		}

		_, err := preprocessJsonnetConfigs(files, repo, pl, mockForge)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), ".crow.jsonnet")
	})
}

// TestWorkflowHasManualTriggerWithJsonnetOutput verifies that the
// workflowHasManualTrigger function correctly detects manual events
// in YAML produced by Jsonnet evaluation.
func TestWorkflowHasManualTriggerWithJsonnetOutput(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected bool
	}{
		{
			name:     "ManualEventAsString",
			yaml:     "when:\n  - event: manual\nsteps:\n  - name: test\n    image: alpine",
			expected: true,
		},
		{
			name:     "ManualEventAsList",
			yaml:     "when:\n  - event:\n      - manual\n      - push\nsteps:\n  - name: test\n    image: alpine",
			expected: true,
		},
		{
			name:     "NoManualEvent",
			yaml:     "when:\n  - event: push\nsteps:\n  - name: test\n    image: alpine",
			expected: false,
		},
		{
			name:     "EmptyWhen",
			yaml:     "steps:\n  - name: test\n    image: alpine",
			expected: false,
		},
		{
			name:     "NoEventFilter",
			yaml:     "when:\n  - branch: main\nsteps:\n  - name: test\n    image: alpine",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := yaml.ParseString(tt.yaml)
			require.NoError(t, err)

			result := workflowHasManualTrigger(parsed.When)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConstraintMatchManualFromJsonnet verifies that constraint.When.Match
// works correctly with YAML produced by Jsonnet evaluation (matching the
// scenario where Jsonnet outputs {event: 'manual'} as a string, not an array).
func TestConstraintMatchManualFromJsonnet(t *testing.T) {
	// This YAML mimics what Jsonnet produces: event as a string value
	yamlStr := "when:\n  - event: manual\nsteps:\n  - name: test\n    image: alpine"

	parsed, err := yaml.ParseString(yamlStr)
	require.NoError(t, err)

	require.Len(t, parsed.When.Constraints, 1)
	assert.Equal(t, yamlBaseTypes.StringOrSlice{"manual"}, parsed.When.Constraints[0].Event)
}
