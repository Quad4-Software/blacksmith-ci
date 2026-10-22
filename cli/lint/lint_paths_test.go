// Copyright 2026 Crow CI Authors
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

package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}

func TestLintPathsReportsErrors(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".crow")
	writeFile(t, filepath.Join(dir, "broken.yaml"), "steps:\n  - name: x\n    image: [unclosed\n")

	report, err := LintPaths(t.Context(), []string{dir}, Options{})
	require.Error(t, err)
	assert.Contains(t, report, "broken.yaml")
}

func TestLintPathsValid(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".crow")
	writeFile(t, filepath.Join(dir, "ok.yaml"), "when:\n  - event: push\nsteps:\n  - name: test\n    image: alpine\n    commands:\n      - echo hi\n")

	report, err := LintPaths(t.Context(), []string{dir}, Options{})
	require.NoError(t, err, report)
	assert.Contains(t, report, "ok.yaml")
	assert.Contains(t, report, "valid")
}

func TestLintPathsMissingPath(t *testing.T) {
	_, err := LintPaths(t.Context(), []string{filepath.Join(t.TempDir(), "nope")}, Options{})
	require.Error(t, err)
}
