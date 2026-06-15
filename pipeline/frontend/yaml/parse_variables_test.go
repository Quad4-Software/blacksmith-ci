// Copyright 2026 Crow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVariableDefs(t *testing.T) {
	t.Run("parses rich mapping form", func(t *testing.T) {
		defs, err := ParseVariableDefsString(`
variables:
  DEPLOY_ENV:
    description: Target env
    options: [staging, production]
    default: staging
    required: true
steps:
  test:
    image: golang
`)
		require.NoError(t, err)
		assert.Equal(t, Variable{
			Description: "Target env",
			Options:     []string{"staging", "production"},
			Default:     "staging",
			Required:    true,
		}, defs["DEPLOY_ENV"])
	})

	t.Run("parses scalar shorthand as default", func(t *testing.T) {
		defs, err := ParseVariableDefsString(`
variables:
  FOO_VERSION: "3.12"
`)
		require.NoError(t, err)
		assert.Equal(t, Variable{Default: "3.12"}, defs["FOO_VERSION"])
	})

	t.Run("ignores non-scalar non-mapping values", func(t *testing.T) {
		defs, err := ParseVariableDefsString(`
variables:
  base_cmds: &base
    - go version
`)
		require.NoError(t, err)
		_, ok := defs["base_cmds"]
		assert.False(t, ok)
	})

	t.Run("parses sparse mapping (required only)", func(t *testing.T) {
		defs, err := ParseVariableDefsString(`
variables:
  DEPLOY_ENV:
    required: true
`)
		require.NoError(t, err)
		assert.Equal(t, Variable{Required: true}, defs["DEPLOY_ENV"])
	})

	t.Run("treats empty mapping as zero-value variable", func(t *testing.T) {
		defs, err := ParseVariableDefsString(`
variables:
  DEPLOY_ENV: {}
`)
		require.NoError(t, err)
		v, ok := defs["DEPLOY_ENV"]
		require.True(t, ok)
		assert.Equal(t, Variable{}, v)
	})
}

func TestParseVariables(t *testing.T) {
	t.Run("ParseVariables still returns name to default map", func(t *testing.T) {
		vars, err := ParseVariablesString(`
variables:
  DEPLOY_ENV:
    options: [staging, production]
    default: staging
  FOO_VERSION: "3.12"
`)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"DEPLOY_ENV":  "staging",
			"FOO_VERSION": "3.12",
		}, vars)
	})

	t.Run("extracts scalar string variables", func(t *testing.T) {
		vars, err := ParseVariablesString(`
variables:
  FOO_VERSION: "3.12"
  BAR_VERSION: "0.3.1"

steps:
  test:
    image: golang
`)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"FOO_VERSION": "3.12",
			"BAR_VERSION": "0.3.1",
		}, vars)
	})

	t.Run("ignores non-scalar values", func(t *testing.T) {
		vars, err := ParseVariablesString(`
variables:
  GREETING: hello
  base_commands: &base-cmds
    - go version
    - go mod download
  step_template: &base-step
    image: golang:1.19

steps:
  test:
    image: golang
`)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"GREETING": "hello",
		}, vars)
	})

	t.Run("returns empty map when no variables block", func(t *testing.T) {
		vars, err := ParseVariablesString(`
steps:
  test:
    image: golang
`)
		require.NoError(t, err)
		assert.Empty(t, vars)
	})

	t.Run("returns empty map for empty variables block", func(t *testing.T) {
		vars, err := ParseVariablesString(`
variables: {}

steps:
  test:
    image: golang
`)
		require.NoError(t, err)
		assert.Empty(t, vars)
	})

	t.Run("handles variables with special characters", func(t *testing.T) {
		vars, err := ParseVariablesString(`
variables:
  MY_TAG: "v1.2.3-beta+build.123"
  MY_REGISTRY: "docker.io/myorg"
`)
		require.NoError(t, err)
		assert.Equal(t, "v1.2.3-beta+build.123", vars["MY_TAG"])
		assert.Equal(t, "docker.io/myorg", vars["MY_REGISTRY"])
	})

	t.Run("preserves envsubst references in values", func(t *testing.T) {
		vars, err := ParseVariablesString(`
variables:
  MY_IMAGE: "docker.io/myorg/myapp:${CI_COMMIT_SHA}"
`)
		require.NoError(t, err)
		assert.Equal(t, "docker.io/myorg/myapp:${CI_COMMIT_SHA}", vars["MY_IMAGE"])
	})
}
