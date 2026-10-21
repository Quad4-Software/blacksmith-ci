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

package main

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tarEntry struct {
	name, body, link string
	typeflag         byte
}

// craftArchive builds an archive by hand, as a malicious step could upload it.
func craftArchive(t *testing.T, entries ...tarEntry) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	zw, err := zstd.NewWriter(&buf)
	require.NoError(t, err)
	tw := tar.NewWriter(zw)
	for _, e := range entries {
		header := &tar.Header{Name: e.name, Typeflag: e.typeflag, Mode: 0o644, Linkname: e.link, Size: int64(len(e.body))}
		require.NoError(t, tw.WriteHeader(header))
		if e.body != "" {
			_, err := tw.Write([]byte(e.body))
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, zw.Close())
	return &buf
}

func TestExtractStaysInsideDeclaredPaths(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "workspace")
	outside := filepath.Join(parent, "outside")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(outside, 0o755))

	archive := craftArchive(t,
		tarEntry{name: "cache/ok.txt", body: "ok", typeflag: tar.TypeReg},
		tarEntry{name: "../outside/traversal.txt", body: "x", typeflag: tar.TypeReg},
		tarEntry{name: "src/undeclared.txt", body: "x", typeflag: tar.TypeReg},
		tarEntry{name: "/abs.txt", body: "x", typeflag: tar.TypeReg},
		tarEntry{name: "cache/escape", link: outside, typeflag: tar.TypeSymlink},
		tarEntry{name: "cache/escape/through-link.txt", body: "x", typeflag: tar.TypeReg},
	)

	_, err := extractArchive(archive, workspace, []string{"cache"})
	require.Error(t, err, "writing through a symlink must abort the restore")

	assert.Equal(t, "ok", readFile(t, workspace, "cache/ok.txt"))
	assert.NoFileExists(t, filepath.Join(outside, "traversal.txt"))
	assert.NoFileExists(t, filepath.Join(outside, "through-link.txt"))
	assert.NoFileExists(t, filepath.Join(workspace, "src/undeclared.txt"))
}

func TestExtractReplacesSymlinkInsteadOfWritingThroughIt(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "workspace")
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, "cache"), 0o755))
	victim := filepath.Join(parent, "victim.txt")
	require.NoError(t, os.WriteFile(victim, []byte("original"), 0o644))
	require.NoError(t, os.Symlink(victim, filepath.Join(workspace, "cache/file.txt")))

	archive := craftArchive(t, tarEntry{name: "cache/file.txt", body: "restored", typeflag: tar.TypeReg})
	_, err := extractArchive(archive, workspace, []string{"cache"})
	require.NoError(t, err)

	assert.Equal(t, "original", readFile(t, parent, "victim.txt"))
	assert.Equal(t, "restored", readFile(t, workspace, "cache/file.txt"))
}

func TestArchiveRoundTrip(t *testing.T) {
	src := t.TempDir()
	writeFile(t, src, "a/one.txt", "1")
	writeFile(t, src, "a/nested/two.txt", "2")
	writeFile(t, src, "b.txt", "b")

	var buf bytes.Buffer
	missing, err := writeArchive(&buf, src, []string{"a", "b.txt", "nope"}, new(counter))
	require.NoError(t, err)
	assert.Equal(t, []string{"nope"}, missing)

	dst := t.TempDir()
	extracted, err := extractArchive(&buf, dst, []string{"a", "b.txt"})
	require.NoError(t, err)
	assert.Equal(t, 3, extracted.files)
	assert.Equal(t, "2", readFile(t, dst, "a/nested/two.txt"))
	assert.Equal(t, "b", readFile(t, dst, "b.txt"))
}

func TestUndoRemovesWhatAFailedExtractionWrote(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "workspace")
	writeFile(t, workspace, "cache/kept.txt", "from the clone")

	archive := craftArchive(t,
		tarEntry{name: "cache/new/", typeflag: tar.TypeDir},
		tarEntry{name: "cache/new/deep/file.txt", body: "x", typeflag: tar.TypeReg},
		tarEntry{name: "cache/link", link: "kept.txt", typeflag: tar.TypeSymlink},
		tarEntry{name: "cache/escape", link: parent, typeflag: tar.TypeSymlink},
		tarEntry{name: "cache/escape/through-link.txt", body: "x", typeflag: tar.TypeReg},
	)
	extracted, err := extractArchive(archive, workspace, []string{"cache"})
	require.Error(t, err)
	assert.FileExists(t, filepath.Join(workspace, "cache/new/deep/file.txt"))

	extracted.undo()
	assert.NoDirExists(t, filepath.Join(workspace, "cache/new"))
	assert.NoFileExists(t, filepath.Join(workspace, "cache/link"))
	assert.NoFileExists(t, filepath.Join(workspace, "cache/escape"))
	assert.Equal(t, "from the clone", readFile(t, workspace, "cache/kept.txt"))
}
