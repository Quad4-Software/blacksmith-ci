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

func TestExtractEvaluateVariables(t *testing.T) {
	t.Run("equality literal", func(t *testing.T) {
		names, opts := extractEvaluateVariables(`DEPLOY_ENV == "prod"`)
		assert.Equal(t, []string{"DEPLOY_ENV"}, names)
		assert.Equal(t, []string{"prod"}, opts["DEPLOY_ENV"])
	})

	t.Run("in list literal", func(t *testing.T) {
		names, opts := extractEvaluateVariables(`DEPLOY_ENV in ["staging", "prod"]`)
		assert.Equal(t, []string{"DEPLOY_ENV"}, names)
		assert.Equal(t, []string{"staging", "prod"}, opts["DEPLOY_ENV"])
	})

	t.Run("or chain accumulates options", func(t *testing.T) {
		names, opts := extractEvaluateVariables(`DEPLOY_ENV == "a" || DEPLOY_ENV == "b"`)
		assert.Equal(t, []string{"DEPLOY_ENV"}, names)
		assert.Equal(t, []string{"a", "b"}, opts["DEPLOY_ENV"])
	})

	t.Run("equality literal reversed order", func(t *testing.T) {
		names, opts := extractEvaluateVariables(`"prod" == DEPLOY_ENV`)
		assert.Equal(t, []string{"DEPLOY_ENV"}, names)
		assert.Equal(t, []string{"prod"}, opts["DEPLOY_ENV"])
	})

	t.Run("deduplicates repeated options", func(t *testing.T) {
		_, opts := extractEvaluateVariables(`DEPLOY_ENV == "prod" || DEPLOY_ENV == "prod"`)
		assert.Equal(t, []string{"prod"}, opts["DEPLOY_ENV"])
	})

	t.Run("filters built-in CI identifiers", func(t *testing.T) {
		names, _ := extractEvaluateVariables(`CI_COMMIT_BRANCH == "main" && DEPLOY_ENV == "prod"`)
		assert.Equal(t, []string{"DEPLOY_ENV"}, names)
	})

	t.Run("filters task selector", func(t *testing.T) {
		names, _ := extractEvaluateVariables(`task == "deploy"`)
		assert.Empty(t, names)
	})

	t.Run("complex expression yields name but no options", func(t *testing.T) {
		names, opts := extractEvaluateVariables(`len(VERSION) > 3`)
		assert.Equal(t, []string{"VERSION"}, names)
		assert.Empty(t, opts["VERSION"])
	})

	t.Run("invalid expression yields nothing", func(t *testing.T) {
		names, _ := extractEvaluateVariables(`this is not valid !!`)
		assert.Empty(t, names)
	})
}

func TestIsBuiltinIdent(t *testing.T) {
	require.True(t, isBuiltinIdent("CI"))
	require.True(t, isBuiltinIdent("CI_COMMIT_BRANCH"))
	require.True(t, isBuiltinIdent("task"))
	require.False(t, isBuiltinIdent("DEPLOY_ENV"))
}

func TestExtractManualVariables(t *testing.T) {
	t.Run("declared overrides extracted", func(t *testing.T) {
		vars, err := ExtractManualVariables([]byte(`
variables:
  DEPLOY_ENV:
    description: Target env
    options: [staging, production]
    default: staging
    required: true
when:
  - event: manual
    evaluate: 'DEPLOY_ENV == "other"'
steps:
  deploy:
    image: alpine
`))
		require.NoError(t, err)
		require.Len(t, vars, 1)
		assert.Equal(t, ManualVariable{
			Name:        "DEPLOY_ENV",
			Description: "Target env",
			Options:     []string{"staging", "production"},
			Default:     "staging",
			Required:    true,
		}, vars[0])
		assert.NotContains(t, vars[0].Options, "other")
	})

	t.Run("extracted-only variable with inferred options", func(t *testing.T) {
		vars, err := ExtractManualVariables([]byte(`
when:
  - event: manual
    evaluate: 'TIER in ["free", "pro"]'
steps:
  build:
    image: alpine
`))
		require.NoError(t, err)
		require.Len(t, vars, 1)
		assert.Equal(t, ManualVariable{Name: "TIER", Options: []string{"free", "pro"}}, vars[0])
	})

	t.Run("collects step-level evaluate", func(t *testing.T) {
		vars, err := ExtractManualVariables([]byte(`
steps:
  build:
    image: alpine
    when:
      - evaluate: 'RUN_BUILD == "yes"'
`))
		require.NoError(t, err)
		require.Len(t, vars, 1)
		assert.Equal(t, "RUN_BUILD", vars[0].Name)
	})

	t.Run("result is sorted by name", func(t *testing.T) {
		vars, err := ExtractManualVariables([]byte(`
when:
  - event: manual
    evaluate: 'ZED == "1" && ABLE == "2"'
steps:
  s:
    image: alpine
`))
		require.NoError(t, err)
		require.Len(t, vars, 2)
		assert.Equal(t, "ABLE", vars[0].Name)
		assert.Equal(t, "ZED", vars[1].Name)
	})
}
