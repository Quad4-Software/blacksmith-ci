// Copyright 2023 Woodpecker Authors
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

package schema_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/linter/schema"
)

func TestSchema(t *testing.T) {
	t.Parallel()

	testTable := []struct {
		name     string
		testFile string
		fail     bool
	}{
		{
			name:     "Clone",
			testFile: ".crow/test-clone.yaml",
		},
		{
			name:     "Clone skip",
			testFile: ".crow/test-clone-skip.yaml",
		},
		{
			name:     "Matrix",
			testFile: ".crow/test-matrix.yaml",
		},
		{
			name:     "Multi Pipeline",
			testFile: ".crow/test-multi.yaml",
		},
		{
			name:     "Plugin",
			testFile: ".crow/test-plugin.yaml",
		},
		{
			name:     "Run on",
			testFile: ".crow/test-run-on.yaml",
		},
		{
			name:     "Service",
			testFile: ".crow/test-service.yaml",
		},
		{
			name:     "Step",
			testFile: ".crow/test-step.yaml",
		},
		{
			name:     "When",
			testFile: ".crow/test-when.yaml",
		},
		{
			name:     "Workspace",
			testFile: ".crow/test-workspace.yaml",
		},
		{
			name:     "Labels",
			testFile: ".crow/test-labels.yaml",
		},
		{
			name:     "Map and Sequence Merge",
			testFile: ".crow/test-merge-map-and-sequence.yaml",
		},
		{
			name:     "Notify",
			testFile: ".crow/test-notify.yaml",
		},
		{
			name:     "Notify disabled",
			testFile: ".crow/test-notify-disabled.yaml",
		},
		{
			name:     "Backend options",
			testFile: ".crow/test-backend-options.yaml",
		},
		{
			name:     "Broken Config",
			testFile: ".crow/test-broken.yaml",
			fail:     true,
		},
		{
			name:     "Array syntax",
			testFile: ".crow/test-array-syntax.yaml",
			fail:     false,
		},
		{
			name:     "Step DAG syntax",
			testFile: ".crow/test-dag.yaml",
			fail:     false,
		},
		{
			name:     "Custom backend",
			testFile: ".crow/test-custom-backend.yaml",
			fail:     false,
		},
		{
			name:     "Broken Plugin by environment",
			testFile: ".crow/test-broken-plugin.yaml",
			fail:     true,
		},
		{
			name:     "Broken Plugin by commands",
			testFile: ".crow/test-broken-plugin2.yaml",
			fail:     true,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			fi, err := os.Open(tt.testFile)
			assert.NoError(t, err, "could not open test file")
			defer fi.Close()
			configErrors, err := schema.Lint(fi)
			if tt.fail {
				if len(configErrors) == 0 {
					assert.Error(t, err, "Expected config errors but got none")
				}
			} else {
				assert.NoError(t, err, fmt.Sprintf("Validation failed: %v", configErrors))
			}
		})
	}
}

// TestSchemaErrorMessages pins the user-facing shape of schema errors:
// the field path, line number, and message text. Together they make the error
// actionable in the UI — a regression in any one of them turns the report back
// into the "lint-and-test:steps@ line 62 / validation failed" noise that
// motivated this work.
func TestSchemaErrorMessages(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		yaml        string
		wantField   string
		wantLine    int
		wantMessage string
	}{
		{
			name: "empty environment under a step (real-world regression case)",
			yaml: `steps:
  install:
    image: alpine
    commands:
      - echo hi

  typecheck:
    image: alpine
    environment:
    commands:
      - echo typecheck

  tail:
    image: alpine
    commands:
      - echo tail
`,
			// Path points at the actually-broken field, not at the parent
			// 'steps' map and not at the last step in that map.
			wantField:   "steps.typecheck.environment",
			wantLine:    9,
			wantMessage: "Expected object, got null.",
		},
		{
			name: "unknown top-level property",
			yaml: `pipelines:
  - name: foo
steps:
  test:
    image: alpine
    commands: echo hi
`,
			// Field and line point at the offending key, not the parent object.
			// Typo map kicks in -> 'pipelines' suggests 'steps'.
			wantField:   "pipelines",
			wantLine:    1,
			wantMessage: "Unknown property `pipelines`. Did you mean `steps`?",
		},
		{
			name: "unknown property without a typo match",
			yaml: `steps:
  test:
    image: alpine
    commands: echo hi
totallyUnknown: 1
`,
			wantField:   "totallyUnknown",
			wantLine:    5,
			wantMessage: "Unknown property `totallyUnknown`.",
		},
		{
			name: "top-level property used where it is not allowed",
			yaml: `steps:
  test:
    image: alpine
    commands: echo hi
services:
  db:
    image: postgres
    depends_on: [test]
`,
			// depends_on is a valid top-level key, so it must not suggest itself.
			wantField:   "services.db.depends_on",
			wantLine:    8,
			wantMessage: "Unknown property `depends_on`.",
		},
		{
			name: "missing required steps key",
			yaml: `when:
  - event: push
`,
			wantField:   "(root)",
			wantLine:    1,
			wantMessage: "Required property `steps` is missing.",
		},
		{
			name: "unknown property inside notify",
			yaml: `steps:
  test:
    image: alpine
    commands: echo hi
notify:
  disable: true
`,
			wantField:   "notify.disable",
			wantLine:    6,
			wantMessage: "Unknown property `disable`.",
		},
		{
			name: "invalid notify event",
			yaml: `steps:
  test:
    image: alpine
    commands: echo hi
notify:
  events:
    - failure
    - bogus
`,
			wantField:   "notify.events[1]",
			wantLine:    8,
			wantMessage: "Value `bogus` is not allowed. Must be one of:",
		},
		{
			name: "enum violation on when.event",
			yaml: `steps:
  test:
    image: alpine
    commands: echo hi
when:
  - event: not_a_real_event
`,
			wantField: "when[0].event",
			wantLine:  6,
			// Just assert a prefix — the enum list is long and may grow.
			wantMessage: "Value `not_a_real_event` is not allowed. Must be one of:",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			errs, err := schema.LintString(tc.yaml)
			assert.Error(t, err, "expected validation to fail")
			if !assert.NotEmpty(t, errs, "expected at least one schema error") {
				return
			}
			got := errs[0]
			assert.Equal(t, tc.wantField, got.Field(), "field path")
			assert.Equal(t, tc.wantLine, got.Line(), "line number")
			assert.True(t,
				strings.HasPrefix(got.Description(), tc.wantMessage),
				"message %q should start with %q", got.Description(), tc.wantMessage,
			)
		})
	}
}
