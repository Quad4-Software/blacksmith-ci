// Copyright 2025 Crow Authors
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

package jsonnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluate_BasicObject(t *testing.T) {
	source := []byte(`{
		steps: [
			{
				name: "test",
				image: "golang:1.22",
				commands: ["go test ./..."],
			},
		],
	}`)

	results, err := Evaluate(".crow.jsonnet", source, nil, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)

	output := string(results[0])
	assert.Contains(t, output, "steps:")
	assert.Contains(t, output, "name: test")
	assert.Contains(t, output, "image: golang:1.22")
}

func TestEvaluate_WithExtVars(t *testing.T) {
	source := []byte(`{
		steps: [
			{
				name: "build",
				image: if std.extVar("CI_PIPELINE_EVENT") == "tag" then "golang:release" else "golang:latest",
			},
		],
	}`)

	extVars := map[string]string{
		"CI_PIPELINE_EVENT": "tag",
	}

	results, err := Evaluate(".crow.jsonnet", source, nil, extVars)
	require.NoError(t, err)
	require.Len(t, results, 1)

	output := string(results[0])
	assert.Contains(t, output, "image: golang:release")

	// Test with non-tag event
	extVars["CI_PIPELINE_EVENT"] = "push"
	results, err = Evaluate(".crow.jsonnet", source, nil, extVars)
	require.NoError(t, err)
	require.Len(t, results, 1)

	output = string(results[0])
	assert.Contains(t, output, "image: golang:latest")
}

func TestEvaluate_WithImport(t *testing.T) {
	libSource := []byte(`{
		goImage: "golang:1.22",
		nodeImage: "node:20",
	}`)

	mainSource := []byte(`
		local images = import "images.libsonnet";
		{
			steps: [
				{
					name: "build",
					image: images.goImage,
				},
			],
		}
	`)

	siblings := []*File{
		{Name: ".crow/images.libsonnet", Data: libSource},
	}

	results, err := Evaluate(".crow/main.jsonnet", mainSource, siblings, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)

	output := string(results[0])
	assert.Contains(t, output, "image: golang:1.22")
}

func TestEvaluate_MultiDocument(t *testing.T) {
	source := []byte(`[
		{
			name: "lint",
			steps: [{name: "lint", image: "golangci-lint"}],
		},
		{
			name: "test",
			steps: [{name: "test", image: "golang:1.22"}],
		},
	]`)

	results, err := Evaluate(".crow.jsonnet", source, nil, nil)
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Contains(t, string(results[0]), "name: lint")
	assert.Contains(t, string(results[1]), "name: test")
}

func TestEvaluate_DynamicMatrix(t *testing.T) {
	source := []byte(`
		local goVersions = ["1.21", "1.22", "1.23"];
		{
			matrix: {
				GO_VERSION: goVersions,
			},
			steps: [
				{
					name: "test",
					image: "golang:${GO_VERSION}",
					commands: ["go test ./..."],
				},
			],
		}
	`)

	results, err := Evaluate(".crow.jsonnet", source, nil, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)

	output := string(results[0])
	assert.Contains(t, output, "GO_VERSION:")
	assert.Contains(t, output, "\"1.21\"")
	assert.Contains(t, output, "\"1.22\"")
	assert.Contains(t, output, "\"1.23\"")
}

func TestEvaluate_SyntaxError(t *testing.T) {
	source := []byte(`{ invalid syntax here`)

	_, err := Evaluate(".crow.jsonnet", source, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jsonnet evaluation failed")
}

func TestEvaluate_ImportNotFound(t *testing.T) {
	source := []byte(`local lib = import "nonexistent.libsonnet"; lib`)

	_, err := Evaluate(".crow.jsonnet", source, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEvaluate_UndefinedExtVar(t *testing.T) {
	source := []byte(`std.extVar("UNDEFINED_VAR")`)

	_, err := Evaluate(".crow.jsonnet", source, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jsonnet evaluation failed")
}

func TestEvaluate_EmptyArray(t *testing.T) {
	source := []byte(`[]`)

	results, err := Evaluate(".crow.jsonnet", source, nil, nil)
	require.NoError(t, err)
	// Empty array produces no workflow documents
	require.Nil(t, results)
}

func TestEvaluate_AbsoluteImportRejected(t *testing.T) {
	source := []byte(`local lib = import "/etc/passwd"; lib`)

	_, err := Evaluate(".crow.jsonnet", source, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "absolute paths are not allowed")
}

func TestIsJsonnetFile(t *testing.T) {
	assert.True(t, IsJsonnetFile(".crow.jsonnet"))
	assert.True(t, IsJsonnetFile(".crow/build.jsonnet"))
	assert.False(t, IsJsonnetFile(".crow.yaml"))
	assert.False(t, IsJsonnetFile("lib.libsonnet"))
}

func TestIsLibsonnetFile(t *testing.T) {
	assert.True(t, IsLibsonnetFile("lib.libsonnet"))
	assert.True(t, IsLibsonnetFile(".crow/helpers.libsonnet"))
	assert.False(t, IsLibsonnetFile(".crow.jsonnet"))
	assert.False(t, IsLibsonnetFile(".crow.yaml"))
}
