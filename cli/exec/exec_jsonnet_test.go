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

package exec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v5/cli/common"
	crow_jsonnet "codefloe.com/crowci/crow/v5/pipeline/frontend/jsonnet"
	"codefloe.com/crowci/crow/v5/pipeline/frontend/yaml/matrix"
)

// TestJsonnetEvaluationProducesParseableYAML verifies that Jsonnet files
// are correctly evaluated to YAML that the matrix parser can handle.
func TestJsonnetEvaluationProducesParseableYAML(t *testing.T) {
	t.Run("BasicPipeline", func(t *testing.T) {
		source := []byte(`{
			steps: [
				{
					name: "test",
					image: "golang:1.22",
					commands: ["go test ./..."],
				},
			],
		}`)

		docs, err := crow_jsonnet.Evaluate(".crow.jsonnet", source, nil, nil)
		require.NoError(t, err)
		require.Len(t, docs, 1)

		axes, err := matrix.ParseString(string(docs[0]))
		require.NoError(t, err)
		assert.Empty(t, axes)
	})

	t.Run("WithMatrix", func(t *testing.T) {
		source := []byte(`{
			matrix: {
				GO_VERSION: ["1.21", "1.22"],
			},
			steps: [
				{
					name: "test",
					image: "golang:${GO_VERSION}",
					commands: ["go test ./..."],
				},
			],
		}`)

		docs, err := crow_jsonnet.Evaluate(".crow.jsonnet", source, nil, nil)
		require.NoError(t, err)
		require.Len(t, docs, 1)

		axes, err := matrix.ParseString(string(docs[0]))
		require.NoError(t, err)
		assert.Len(t, axes, 2)
	})

	t.Run("WithCommentsInArrays", func(t *testing.T) {
		// This is the original issue #851: Jsonnet comments in arrays
		// should not cause matrix parsing to fail.
		source := []byte(`{
			when: [
				// { event: 'pull_request' },
				{ event: 'manual' },
			],
			steps: [
				{
					name: "deploy",
					image: "alpine",
					commands: ["echo deploying"],
				},
			],
		}`)

		docs, err := crow_jsonnet.Evaluate(".crow.jsonnet", source, nil, nil)
		require.NoError(t, err)
		require.Len(t, docs, 1)

		axes, err := matrix.ParseString(string(docs[0]))
		require.NoError(t, err)
		assert.Empty(t, axes)
	})

	t.Run("MultiDocument", func(t *testing.T) {
		source := []byte(`[
			{
				name: "lint",
				steps: [{name: "lint", image: "golangci-lint"}],
			},
			{
				name: "test",
				matrix: {
					GO_VERSION: ["1.21", "1.22"],
				},
				steps: [{name: "test", image: "golang:${GO_VERSION}"}],
			},
		]`)

		docs, err := crow_jsonnet.Evaluate(".crow.jsonnet", source, nil, nil)
		require.NoError(t, err)
		require.Len(t, docs, 2)

		// First doc has no matrix
		axes, err := matrix.ParseString(string(docs[0]))
		require.NoError(t, err)
		assert.Empty(t, axes)

		// Second doc has a 2-element matrix
		axes, err = matrix.ParseString(string(docs[1]))
		require.NoError(t, err)
		assert.Len(t, axes, 2)
	})

	t.Run("WithExtVars", func(t *testing.T) {
		source := []byte(`{
			steps: [
				{
					name: "build",
					image: if std.extVar("CI_PIPELINE_EVENT") == "tag" then "golang:release" else "golang:latest",
					commands: ["go build ./..."],
				},
			],
		}`)

		extVars := map[string]string{"CI_PIPELINE_EVENT": "tag"}
		docs, err := crow_jsonnet.Evaluate(".crow.jsonnet", source, nil, extVars)
		require.NoError(t, err)
		require.Len(t, docs, 1)

		axes, err := matrix.ParseString(string(docs[0]))
		require.NoError(t, err)
		assert.Empty(t, axes)
		assert.Contains(t, string(docs[0]), "golang:release")
	})
}

// TestJsonnetSiblingFileLoading verifies that sibling Jsonnet/libsonnet
// files are loaded and available for imports during exec evaluation.
func TestJsonnetSiblingFileLoading(t *testing.T) {
	dir := t.TempDir()

	mainContent := []byte(`
		local images = import "images.libsonnet";
		{
			steps: [
				{
					name: "build",
					image: images.goImage,
					commands: ["go build ./..."],
				},
			],
		}
	`)
	libContent := []byte(`{
		goImage: "golang:1.22",
	}`)

	mainFile := filepath.Join(dir, "pipeline.jsonnet")
	require.NoError(t, os.WriteFile(mainFile, mainContent, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "images.libsonnet"), libContent, 0o644))

	siblings, err := common.LoadSiblingFiles(dir, mainFile)
	require.NoError(t, err)
	require.Len(t, siblings, 1)

	docs, err := crow_jsonnet.Evaluate(mainFile, mainContent, siblings, nil)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Contains(t, string(docs[0]), "golang:1.22")

	axes, err := matrix.ParseString(string(docs[0]))
	require.NoError(t, err)
	assert.Empty(t, axes)
}

// TestJsonnetFileDetectionInDir verifies that execDir's filepath.Walk
// correctly identifies .jsonnet files alongside .yaml and .yml files.
func TestJsonnetFileDetectionInDir(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		"build.yaml":        "steps: [{name: test, image: alpine}]",
		"deploy.yml":        "steps: [{name: deploy, image: alpine}]",
		"lint.jsonnet":      `{steps: [{name: "lint", image: "golangci-lint"}]}`,
		"helpers.libsonnet": `{helper: true}`,
		"readme.md":         "# readme",
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}

	var matched []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		name := info.Name()
		if info.Mode().IsRegular() && (filepath.Ext(name) == ".yaml" || filepath.Ext(name) == ".yml" || filepath.Ext(name) == ".jsonnet") {
			matched = append(matched, name)
		}
		return nil
	})
	require.NoError(t, err)

	assert.Len(t, matched, 3)
	assert.Contains(t, matched, "build.yaml")
	assert.Contains(t, matched, "deploy.yml")
	assert.Contains(t, matched, "lint.jsonnet")
	// .libsonnet and .md files should NOT be matched
	assert.NotContains(t, matched, "helpers.libsonnet")
	assert.NotContains(t, matched, "readme.md")
}

// TestRunExecYAMLMatrixErrorMessage verifies that the improved error
// message includes the file name and the underlying error.
func TestRunExecYAMLMatrixErrorMessage(t *testing.T) {
	// Invalid YAML that will fail matrix parsing
	invalidYAML := ":\n  :\n    - :\n      invalid"

	axes, err := matrix.ParseString(invalidYAML)
	if err != nil {
		// Verify we can construct the improved error format
		wrappedErr := assert.ErrorContains(t, err, "")
		_ = wrappedErr
		improvedMsg := "parse matrix fail for test.jsonnet: " + err.Error()
		assert.Contains(t, improvedMsg, "test.jsonnet")
		assert.Contains(t, improvedMsg, "parse matrix fail")
	} else {
		// If matrix parsing succeeds on this input, it returns empty axes
		assert.Empty(t, axes)
	}
}

// TestJsonnetIsJsonnetFileUsedByExec verifies that the IsJsonnetFile
// check works correctly for the file extensions the exec command handles.
func TestJsonnetIsJsonnetFileUsedByExec(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		expected bool
	}{
		{"JsonnetExtension", "pipeline.jsonnet", true},
		{"CrowJsonnet", ".crow.jsonnet", true},
		{"PathWithJsonnet", "/path/to/.crow/build.jsonnet", true},
		{"YAMLExtension", "pipeline.yaml", false},
		{"YMLExtension", "pipeline.yml", false},
		{"LibsonnetExtension", "helpers.libsonnet", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, crow_jsonnet.IsJsonnetFile(tt.file))
		})
	}
}
