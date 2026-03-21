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

func TestParseVariables(t *testing.T) {
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
