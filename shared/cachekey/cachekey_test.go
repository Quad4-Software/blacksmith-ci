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

package cachekey

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
}

func TestRender(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.sum", "one")
	writeFile(t, dir, "a/go.mod", "two")
	writeFile(t, dir, "b/go.mod", "three")

	key, err := Render(`go-{{ checksum "go.sum" }}-{{ os }}-{{ arch }}`, dir)
	require.NoError(t, err)
	sum, err := Checksum(dir, "go.sum")
	require.NoError(t, err)
	assert.Equal(t, "go-"+sum+"-"+runtime.GOOS+"-"+runtime.GOARCH, key)
	assert.Len(t, sum, 64)

	plain, err := Render("static", dir)
	require.NoError(t, err)
	assert.Equal(t, "static", plain)

	_, err = Render(`{{ checksum "missing.lock" }}`, dir)
	assert.ErrorIs(t, err, ErrNoFiles)
}

func TestChecksumTracksFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a/go.mod", "two")
	writeFile(t, dir, "b/go.mod", "three")

	base, err := Checksum(dir, "**/go.mod")
	require.NoError(t, err)

	same, err := Checksum(dir, "b/go.mod", "a/go.mod", "**/go.mod")
	require.NoError(t, err)
	assert.Equal(t, base, same, "order and duplicates do not matter")

	writeFile(t, dir, "b/go.mod", "changed")
	edited, err := Checksum(dir, "**/go.mod")
	require.NoError(t, err)
	assert.NotEqual(t, base, edited)

	require.NoError(t, os.Rename(filepath.Join(dir, "b"), filepath.Join(dir, "c")))
	renamed, err := Checksum(dir, "**/go.mod")
	require.NoError(t, err)
	assert.NotEqual(t, edited, renamed, "renaming a file changes the checksum")

	_, err = Checksum(dir, "../outside")
	assert.Error(t, err)
}

func TestParse(t *testing.T) {
	assert.NoError(t, Parse(`k-{{ checksum "a" "b" }}-{{ arch }}`))
	assert.Error(t, Parse(`{{ checksum`))
	assert.Error(t, Parse(`{{ env "HOME" }}`), "only cache functions are available")
}

func TestValidatePath(t *testing.T) {
	for _, ok := range []string{".cache/go", "node_modules", ".", "a/../b"} {
		assert.NoError(t, ValidatePath(ok), ok)
	}
	for _, bad := range []string{"", "/etc", "..", "../x", "a/../../x", `\windows`} {
		assert.Error(t, ValidatePath(bad), bad)
	}
}
