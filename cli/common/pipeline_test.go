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

package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSiblingFiles(t *testing.T) {
	t.Run("LoadsJsonnetAndLibsonnetFiles", func(t *testing.T) {
		dir := t.TempDir()
		mainFile := filepath.Join(dir, "main.jsonnet")
		require.NoError(t, os.WriteFile(mainFile, []byte(`{}`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "helpers.libsonnet"), []byte(`{helper: true}`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "other.jsonnet"), []byte(`{other: true}`), 0o644))

		siblings, err := LoadSiblingFiles(dir, mainFile)
		require.NoError(t, err)
		assert.Len(t, siblings, 2)

		names := make(map[string]bool)
		for _, s := range siblings {
			names[filepath.Base(s.Name)] = true
		}
		assert.True(t, names["helpers.libsonnet"])
		assert.True(t, names["other.jsonnet"])
	})

	t.Run("ExcludesSpecifiedFile", func(t *testing.T) {
		dir := t.TempDir()
		mainFile := filepath.Join(dir, "pipeline.jsonnet")
		require.NoError(t, os.WriteFile(mainFile, []byte(`{}`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "lib.libsonnet"), []byte(`{}`), 0o644))

		siblings, err := LoadSiblingFiles(dir, mainFile)
		require.NoError(t, err)
		assert.Len(t, siblings, 1)
		assert.Equal(t, "lib.libsonnet", filepath.Base(siblings[0].Name))
	})

	t.Run("IgnoresYAMLAndOtherFiles", func(t *testing.T) {
		dir := t.TempDir()
		mainFile := filepath.Join(dir, "main.jsonnet")
		require.NoError(t, os.WriteFile(mainFile, []byte(`{}`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "pipeline.yaml"), []byte(`steps: []`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.md"), []byte(`# readme`), 0o644))

		siblings, err := LoadSiblingFiles(dir, mainFile)
		require.NoError(t, err)
		assert.Empty(t, siblings)
	})

	t.Run("IgnoresDirectories", func(t *testing.T) {
		dir := t.TempDir()
		mainFile := filepath.Join(dir, "main.jsonnet")
		require.NoError(t, os.WriteFile(mainFile, []byte(`{}`), 0o644))
		require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir.jsonnet"), 0o755))

		siblings, err := LoadSiblingFiles(dir, mainFile)
		require.NoError(t, err)
		assert.Empty(t, siblings)
	})

	t.Run("EmptyDirectory", func(t *testing.T) {
		dir := t.TempDir()
		mainFile := filepath.Join(dir, "main.jsonnet")

		siblings, err := LoadSiblingFiles(dir, mainFile)
		require.NoError(t, err)
		assert.Empty(t, siblings)
	})

	t.Run("NonexistentDirectory", func(t *testing.T) {
		_, err := LoadSiblingFiles("/nonexistent/path", "/nonexistent/path/main.jsonnet")
		assert.Error(t, err)
	})

	t.Run("ReadsFileContents", func(t *testing.T) {
		dir := t.TempDir()
		mainFile := filepath.Join(dir, "main.jsonnet")
		require.NoError(t, os.WriteFile(mainFile, []byte(`{}`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "images.libsonnet"), []byte(`{goImage: "golang:1.22"}`), 0o644))

		siblings, err := LoadSiblingFiles(dir, mainFile)
		require.NoError(t, err)
		require.Len(t, siblings, 1)
		assert.Equal(t, `{goImage: "golang:1.22"}`, string(siblings[0].Data))
	})
}
