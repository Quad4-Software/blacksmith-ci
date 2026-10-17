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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestExecDirReportsFailedWorkflows(t *testing.T) {
	dir := t.TempDir()
	// An unparsable matrix fails before any backend is needed.
	for _, name := range []string{"a.yaml", "b.yaml"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("matrix: [unclosed\n"), 0o600))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignored"), 0o600))

	err := execDir(context.Background(), &cli.Command{}, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2 workflow(s) failed: a.yaml, b.yaml")
}
