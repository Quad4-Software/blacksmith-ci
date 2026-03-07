// Copyright 2025 Crow Authors
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

package jsonnet

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/google/go-jsonnet"
)

// File represents a named file with its contents, used for Jsonnet imports.
type File struct {
	Name string
	Data []byte
}

// Evaluate takes Jsonnet source, evaluates it, and returns one or more YAML documents.
// The siblings parameter provides files available for Jsonnet import statements.
// The extVars parameter provides external variables accessible via std.extVar().
// If the Jsonnet evaluates to a JSON array, each element becomes a separate YAML document.
func Evaluate(filename string, source []byte, siblings []*File, extVars map[string]string) ([][]byte, error) {
	vm := jsonnet.MakeVM()

	// Register external variables
	for k, v := range extVars {
		vm.ExtVar(k, v)
	}

	// Register custom importer for sibling files
	vm.Importer(&siblingImporter{
		files:   siblings,
		baseDir: filepath.Dir(filename),
	})

	// Evaluate the Jsonnet source
	output, err := vm.EvaluateAnonymousSnippet(filename, string(source))
	if err != nil {
		return nil, fmt.Errorf("jsonnet evaluation failed: %w", err)
	}

	// Parse the JSON output to determine if it's an array (multi-document) or single object
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(output), &raw); err != nil {
		return nil, fmt.Errorf("jsonnet output is not valid JSON: %w", err)
	}

	// Check if the output is an array
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		if len(arr) == 0 {
			// Empty array = no workflows
			return nil, nil
		}
		// Multi-document: each array element becomes a separate YAML document
		results := make([][]byte, 0, len(arr))
		for i, item := range arr {
			yamlBytes, err := jsonToYAML(item)
			if err != nil {
				return nil, fmt.Errorf("jsonnet: failed to convert array element %d to YAML: %w", i, err)
			}
			results = append(results, yamlBytes)
		}
		return results, nil
	}

	// Single document
	yamlBytes, err := jsonToYAML(raw)
	if err != nil {
		return nil, fmt.Errorf("jsonnet: failed to convert output to YAML: %w", err)
	}
	return [][]byte{yamlBytes}, nil
}

// jsonToYAML converts a JSON byte slice to YAML.
func jsonToYAML(jsonData []byte) ([]byte, error) {
	var obj any
	if err := json.Unmarshal(jsonData, &obj); err != nil {
		return nil, err
	}
	return yaml.Marshal(obj)
}

// siblingImporter resolves Jsonnet imports from in-memory sibling files.
type siblingImporter struct {
	files   []*File
	baseDir string
	cache   map[string]jsonnet.Contents
}

func (i *siblingImporter) Import(importedFrom, importedPath string) (contents jsonnet.Contents, foundAt string, err error) {
	if i.cache == nil {
		i.cache = make(map[string]jsonnet.Contents)
	}

	// Reject absolute paths
	if filepath.IsAbs(importedPath) {
		return jsonnet.Contents{}, "", fmt.Errorf("import %q: absolute paths are not allowed", importedPath)
	}

	// Resolve the import path relative to the importing file
	fromDir := filepath.Dir(importedFrom)
	if fromDir == "" || fromDir == "." {
		fromDir = i.baseDir
	}
	resolvedPath := filepath.Clean(filepath.Join(fromDir, importedPath))

	// Check cache
	if cached, ok := i.cache[resolvedPath]; ok {
		return cached, resolvedPath, nil
	}

	// Search in sibling files by matching the resolved path against each file's path
	for _, f := range i.files {
		candidates := []string{
			f.Name,
			filepath.Join(i.baseDir, filepath.Base(f.Name)),
		}
		for _, candidate := range candidates {
			if filepath.Clean(candidate) == resolvedPath {
				c := jsonnet.MakeContents(string(f.Data))
				i.cache[resolvedPath] = c
				return c, resolvedPath, nil
			}
		}
	}

	return jsonnet.Contents{}, "", fmt.Errorf("import %q not found (imported from %q)", importedPath, importedFrom)
}

// IsJsonnetFile returns true if the filename has a .jsonnet extension.
func IsJsonnetFile(filename string) bool {
	return strings.HasSuffix(filename, ".jsonnet")
}

// IsLibsonnetFile returns true if the filename has a .libsonnet extension.
func IsLibsonnetFile(filename string) bool {
	return strings.HasSuffix(filename, ".libsonnet")
}
