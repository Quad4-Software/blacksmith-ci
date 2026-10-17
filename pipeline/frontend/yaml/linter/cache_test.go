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

package linter_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/pipeline/errors"
	errorTypes "codefloe.com/crowci/crow/v6/pipeline/errors/types"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/linter"
)

const cacheSteps = `
when:
  event: push
steps:
  build:
    image: golang
    commands: [go build]
`

func lintCache(t *testing.T, config string) []*errorTypes.PipelineError {
	t.Helper()
	conf, err := yaml.ParseString(config)
	require.NoError(t, err)
	return errors.GetPipelineErrors(linter.New().Lint([]*linter.WorkflowConfig{{
		File:      ".crow.yaml",
		RawConfig: config,
		Workflow:  conf,
	}}))
}

func TestLintCacheValid(t *testing.T) {
	config := `
cache:
  - key: 'go-{{ checksum "go.sum" "**/go.mod" }}-{{ arch }}'
    restore_keys: [go-]
    paths: [.cache/go, vendor]
    save: always
  - key: npm
    paths: node_modules
` + cacheSteps
	assert.Empty(t, lintCache(t, config))
}

func TestLintCacheErrors(t *testing.T) {
	tests := []struct {
		name  string
		cache string
		want  string
	}{
		{"missing key", "[{ paths: [a] }]", "cache entry needs a `key`"},
		{"long key", "[{ key: " + strings.Repeat("k", 256) + ", paths: [a] }]", "cache key must not be longer than 255 characters"},
		{"bad template", `[{ key: '{{ checksum', paths: [a] }]`, "invalid cache key template"},
		{"unknown function", `[{ key: '{{ secret "x" }}', paths: [a] }]`, "invalid cache key template"},
		{"duplicate key", "[{ key: k, paths: [a] }, { key: k, paths: [b] }]", "cache key is already used by cache[0]"},
		{"missing paths", "[{ key: k }]", "cache entry needs at least one path"},
		{"absolute path", "[{ key: k, paths: [/etc] }]", `path "/etc" must be relative to the workspace`},
		{"escaping path", "[{ key: k, paths: [a/../../b] }]", `path "a/../../b" must stay inside the workspace`},
		{"save policy", "[{ key: k, paths: [a], save: sometimes }]", `invalid save policy "sometimes"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lerrors := lintCache(t, "cache: "+tt.cache+"\n"+cacheSteps)
			found := false
			for _, lerr := range lerrors {
				if strings.HasPrefix(lerr.Message, tt.want) {
					found = true
				}
			}
			assert.True(t, found, "expected %q, got %v", tt.want, lerrors)
		})
	}
}
