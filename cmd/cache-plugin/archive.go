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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

const (
	dirPerm = 0o755
	// Only the permission bits of an archived mode are restored; setuid, setgid
	// and sticky bits are not something an archive should be able to set.
	permBits = 0o777
)

// writeArchive writes the paths below workspace as a zstd-compressed tar to w.
// Entry names are slash-separated and relative to the workspace. Symlinks are
// stored as links and never followed. It returns the paths that do not exist.
func writeArchive(w io.Writer, workspace string, paths []string) (missing []string, err error) {
	zw, err := zstd.NewWriter(w)
	if err != nil {
		return nil, err
	}
	tw := tar.NewWriter(zw)

	for _, p := range paths {
		root := filepath.Join(workspace, filepath.FromSlash(path.Clean(p)))
		if _, err := os.Lstat(root); errors.Is(err, fs.ErrNotExist) {
			missing = append(missing, p)
			continue
		}
		err := filepath.WalkDir(root, func(file string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			return addToArchive(tw, workspace, file, d)
		})
		if err != nil {
			return missing, err
		}
	}

	if err := tw.Close(); err != nil {
		return missing, err
	}
	return missing, zw.Close()
}

func addToArchive(tw *tar.Writer, workspace, file string, d fs.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(workspace, file)
	if err != nil {
		return err
	}

	var link string
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		if link, err = os.Readlink(file); err != nil {
			return err
		}
	case info.IsDir(), info.Mode().IsRegular():
	default:
		// Sockets, devices and pipes have no place in a cache.
		return nil
	}

	header, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(rel)
	if info.IsDir() {
		header.Name += "/"
	}
	// Ownership of the saving container means nothing where the cache is restored.
	header.Uid, header.Gid, header.Uname, header.Gname = 0, 0, "", ""
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}

	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(tw, f)
	return err
}

// extraction is what one extractArchive call wrote, so that a restore that
// fails halfway can be undone.
type extraction struct {
	files int
	// created lists the files, links and directories written, parents first.
	created []string
}

// undo removes what the extraction wrote, children before their parents. A
// directory that also holds files from elsewhere is left in place.
func (e *extraction) undo() {
	for i := len(e.created) - 1; i >= 0; i-- {
		_ = os.Remove(e.created[i])
	}
	e.created, e.files = nil, 0
}

// extractArchive restores a zstd-compressed tar from r into workspace. Only
// entries inside one of paths are written, and never through a symlink, so a
// cache cannot write anywhere else, even one saved by a malicious step. On an
// error the returned extraction holds what was written up to then.
func extractArchive(r io.Reader, workspace string, paths []string) (*extraction, error) {
	e := new(extraction)
	zr, err := zstd.NewReader(r)
	if err != nil {
		return e, err
	}
	defer zr.Close()
	tr := tar.NewReader(zr)

	allowed := make([]string, len(paths))
	for i, p := range paths {
		allowed[i] = path.Clean(p)
	}

	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return e, nil
		}
		if err != nil {
			return e, err
		}

		name := path.Clean(header.Name)
		if !insideAny(name, allowed) {
			continue
		}
		target := filepath.Join(workspace, filepath.FromSlash(name))
		if err := checkNoSymlinkParents(workspace, name); err != nil {
			return e, err
		}

		if err := e.extractEntry(tr, header, target); err != nil {
			return e, fmt.Errorf("extract %s: %w", name, err)
		}
		if header.Typeflag == tar.TypeReg {
			e.files++
		}
	}
}

func insideAny(name string, paths []string) bool {
	if name == ".." || strings.HasPrefix(name, "../") || path.IsAbs(name) {
		return false
	}
	for _, p := range paths {
		if p == "." || name == p || strings.HasPrefix(name, p+"/") {
			return true
		}
	}
	return false
}

// checkNoSymlinkParents fails when a directory between workspace and the entry
// is a symlink, which would let the entry escape the workspace.
func checkNoSymlinkParents(workspace, name string) error {
	dir := workspace
	parts := strings.Split(name, "/")
	for _, part := range parts[:len(parts)-1] {
		dir = filepath.Join(dir, part)
		info, err := os.Lstat(dir)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("refusing to extract %s through symlink %s", name, dir)
		}
	}
	return nil
}

// mkdirAll creates dir and its missing parents and records them as created.
func (e *extraction) mkdirAll(dir string) error {
	var missing []string
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Lstat(d); !errors.Is(err, fs.ErrNotExist) || d == filepath.Dir(d) {
			break
		}
		missing = append(missing, d)
	}
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return err
	}
	for i := len(missing) - 1; i >= 0; i-- {
		e.created = append(e.created, missing[i])
	}
	return nil
}

func (e *extraction) extractEntry(tr *tar.Reader, header *tar.Header, target string) error {
	mode := fs.FileMode(header.Mode) & permBits

	switch header.Typeflag {
	case tar.TypeDir:
		// A symlink in place of the directory would redirect what follows.
		if info, err := os.Lstat(target); err == nil && info.Mode()&fs.ModeSymlink != 0 {
			if err := os.Remove(target); err != nil {
				return err
			}
		}
		if err := e.mkdirAll(target); err != nil {
			return err
		}
		return os.Chmod(target, mode|0o700)
	case tar.TypeReg:
		if err := e.mkdirAll(filepath.Dir(target)); err != nil {
			return err
		}
		// Remove first so an existing symlink at the target is replaced rather
		// than written through.
		if err := removeIfNotDir(target); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			return err
		}
		e.created = append(e.created, target)
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		return os.Chtimes(target, time.Now(), header.ModTime)
	case tar.TypeSymlink:
		if err := e.mkdirAll(filepath.Dir(target)); err != nil {
			return err
		}
		if err := removeIfNotDir(target); err != nil {
			return err
		}
		if err := os.Symlink(header.Linkname, target); err != nil {
			return err
		}
		e.created = append(e.created, target)
		return nil
	default:
		return nil
	}
}

func removeIfNotDir(target string) error {
	info, err := os.Lstat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", target)
	}
	return os.Remove(target)
}
