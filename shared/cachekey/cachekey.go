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

// Package cachekey renders build cache keys. Keys are Go templates evaluated in
// the cache step after clone, because only then do the files exist whose
// checksums the key is built from:
//
//	go-{{ arch }}-{{ checksum "go.sum" }}
package cachekey

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"text/template"

	"github.com/bmatcuk/doublestar/v4"
)

// ErrNoFiles is returned when checksum patterns match no file.
var ErrNoFiles = errors.New("checksum patterns match no file")

// Funcs returns the template functions available in cache keys. The checksum
// function hashes the files matching glob patterns relative to workspace.
func Funcs(workspace string) template.FuncMap {
	return template.FuncMap{
		"checksum": func(patterns ...string) (string, error) {
			return Checksum(workspace, patterns...)
		},
		"os":   func() string { return runtime.GOOS },
		"arch": func() string { return runtime.GOARCH },
	}
}

// Parse checks that key is a valid template without evaluating it.
func Parse(key string) error {
	_, err := template.New("key").Funcs(Funcs("")).Option("missingkey=error").Parse(key)
	return err
}

// Render evaluates key in workspace.
func Render(key, workspace string) (string, error) {
	tmpl, err := template.New("key").Funcs(Funcs(workspace)).Option("missingkey=error").Parse(key)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, nil); err != nil {
		return "", err
	}
	return b.String(), nil
}

// Checksum returns the SHA-256 over the paths and contents of all regular files
// matching patterns, in lexical order, so the result changes when a file is
// added, removed, renamed or edited.
func Checksum(workspace string, patterns ...string) (string, error) {
	if len(patterns) == 0 {
		return "", errors.New("checksum needs at least one file pattern")
	}
	root := os.DirFS(workspace)

	var files []string
	for _, pattern := range patterns {
		if err := ValidatePath(pattern); err != nil {
			return "", err
		}
		matches, err := doublestar.Glob(root, path.Clean(pattern), doublestar.WithFilesOnly(), doublestar.WithFailOnIOErrors())
		if err != nil {
			return "", fmt.Errorf("checksum %q: %w", pattern, err)
		}
		files = append(files, matches...)
	}
	slices.Sort(files)
	files = slices.Compact(files)
	if len(files) == 0 {
		return "", fmt.Errorf("%w: %s", ErrNoFiles, strings.Join(patterns, ", "))
	}

	hash := sha256.New()
	for _, name := range files {
		if err := hashFile(hash, root, name); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func hashFile(w io.Writer, root fs.FS, name string) error {
	f, err := root.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	// The name and a separator keep "a"+"bc" apart from "ab"+"c".
	fmt.Fprintf(w, "%s\x00", name)
	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("checksum %q: %w", name, err)
	}
	_, err = w.Write([]byte{0})
	return err
}

// ValidatePath checks a cache path or checksum pattern: it must be relative to
// the workspace and stay inside it.
func ValidatePath(p string) error {
	if p == "" {
		return errors.New("path must not be empty")
	}
	if path.IsAbs(p) || filepath.IsAbs(p) || strings.HasPrefix(p, "\\") {
		return fmt.Errorf("path %q must be relative to the workspace", p)
	}
	clean := path.Clean(filepath.ToSlash(p))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("path %q must stay inside the workspace", p)
	}
	return nil
}
