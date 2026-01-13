// Copyright 2025 Crow Authors
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

// Package yaml provides YAML parsing with sequence merge support.
// It wraps github.com/goccy/go-yaml and adds support for the non-standard
// sequence merge syntax (- <<: *anchor) that was previously provided by xyaml.
package yaml

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

var registerOnce sync.Once

// registerCustomUnmarshalers registers custom unmarshalers for YAML 1.1 compatibility.
// This is called once before the first Unmarshal call.
func registerCustomUnmarshalers() {
	registerOnce.Do(func() {
		// Register a custom unmarshaler for bool that accepts YAML 1.1 booleans.
		// goccy/go-yaml follows YAML 1.2 strictly (only true/false).
		// For backward compatibility with yaml.v3 and YAML 1.1, we accept:
		// yes/no, on/off, true/false (case-insensitive)
		yaml.RegisterCustomUnmarshaler[bool](func(b *bool, data []byte) error {
			s := strings.TrimSpace(string(data))
			// Handle quoted strings (from marshal/unmarshal cycle)
			if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
				s = s[1 : len(s)-1]
			}
			switch strings.ToLower(s) {
			case "true", "yes", "on":
				*b = true
				return nil
			case "false", "no", "off":
				*b = false
				return nil
			default:
				return fmt.Errorf("cannot convert %q to bool", s)
			}
		})
	})
}

// Unmarshal parses YAML data with sequence merge support and stores the result
// in the value pointed to by v.
func Unmarshal(data []byte, v any) error {
	registerCustomUnmarshalers()

	// Handle empty data - just return without modification
	// This matches yaml.v3 behavior where empty YAML leaves the target unchanged
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}

	// Fast path: no anchors or aliases means we can use direct unmarshal
	// This avoids expensive AST parsing and double marshal/unmarshal
	hasAnchor := bytes.Contains(data, []byte("&"))
	hasAlias := bytes.Contains(data, []byte("*"))

	if !hasAnchor && !hasAlias {
		return yaml.UnmarshalWithOptions(data, v, yaml.AllowDuplicateMapKey())
	}

	// Parse into AST to handle anchors/aliases correctly
	// goccy/go-yaml has a bug where aliases to anchors defined within parent
	// sequences are not resolved correctly. We work around this by resolving
	// aliases in the AST before unmarshaling.
	// Note: Don't use parser.ParseComments - comments cause issues when nodes
	// are replaced and the AST is serialized back to string.
	file, err := parser.ParseBytes(data, 0)
	if err != nil {
		return err
	}

	// Collect anchors first
	anchors := make(map[string]ast.Node)
	for _, doc := range file.Docs {
		collectAnchors(doc.Body, anchors)
	}

	// Check if we need sequence merge processing (non-standard YAML extension)
	hasMergeKey := bytes.Contains(data, []byte("<<"))

	// Resolve sequence merges in the AST
	for _, doc := range file.Docs {
		if hasMergeKey {
			resolveSequenceMergesInNode(doc.Body, anchors)
		}
		// Resolve aliases within sequences that contain their anchor definition.
		// This works around a goccy/go-yaml bug with nested anchors.
		// Only resolves aliases within the same sequence subtree as their anchor,
		// leaving cross-document aliases for standard YAML handling.
		resolveNestedSequenceAliases(doc.Body)
	}

	// Convert back to bytes
	var buf bytes.Buffer
	for _, doc := range file.Docs {
		buf.WriteString(doc.String())
		buf.WriteString("\n")
	}
	processedData := buf.Bytes()

	// When anchors/aliases are present, we need double unmarshal to resolve
	// aliases for custom unmarshalers that re-parse without anchor context
	var intermediate any
	if err := yaml.UnmarshalWithOptions(processedData, &intermediate,
		yaml.AllowDuplicateMapKey(),
		yaml.UseOrderedMap(),
	); err != nil {
		return err
	}

	// If the entire document is null/empty, leave target unchanged
	if intermediate == nil {
		return nil
	}

	// Re-marshal the resolved data and unmarshal into target type
	resolvedData, err := yaml.Marshal(intermediate)
	if err != nil {
		return err
	}

	return yaml.UnmarshalWithOptions(resolvedData, v, yaml.AllowDuplicateMapKey())
}

// Marshal serializes the value v into YAML format.
func Marshal(v any) ([]byte, error) {
	return yaml.Marshal(v)
}

// NewDecoder returns a new decoder that reads from r.
func NewDecoder(r io.Reader) *yaml.Decoder {
	return yaml.NewDecoder(r)
}

// NewEncoder returns a new encoder that writes to w.
func NewEncoder(w io.Writer) *yaml.Encoder {
	return yaml.NewEncoder(w)
}

// resolveSequenceMergesInNode resolves sequence merges (non-standard YAML extension)
// without modifying aliases. This allows standard alias resolution to happen later.
func resolveSequenceMergesInNode(node ast.Node, anchors map[string]ast.Node) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *ast.MappingNode:
		for _, value := range n.Values {
			resolveSequenceMergesInNode(value, anchors)
		}

	case *ast.MappingValueNode:
		resolveSequenceMergesInNode(n.Value, anchors)

	case *ast.SequenceNode:
		newValues := make([]ast.Node, 0, len(n.Values))
		for _, value := range n.Values {
			// Check if this is a sequence merge (- <<: *alias)
			if merged := trySequenceMerge(value, anchors); merged != nil {
				newValues = append(newValues, merged...)
			} else {
				resolveSequenceMergesInNode(value, anchors)
				newValues = append(newValues, value)
			}
		}
		n.Values = newValues

	case *ast.AnchorNode:
		resolveSequenceMergesInNode(n.Value, anchors)

	case *ast.DocumentNode:
		resolveSequenceMergesInNode(n.Body, anchors)
	}
}

// resolveNestedSequenceAliases resolves aliases within sequences that contain their anchor.
// This works around a bug in goccy/go-yaml where aliases to anchors defined within
// the same parent sequence are not resolved correctly.
// It only resolves aliases that are within the same sequence subtree as their anchor,
// leaving other aliases for standard YAML handling.
func resolveNestedSequenceAliases(node ast.Node) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *ast.MappingNode:
		for _, value := range n.Values {
			resolveNestedSequenceAliases(value)
		}

	case *ast.MappingValueNode:
		resolveNestedSequenceAliases(n.Value)

	case *ast.SequenceNode:
		// Collect anchors defined within this sequence
		localAnchors := make(map[string]ast.Node)
		for _, value := range n.Values {
			collectAnchors(value, localAnchors)
		}

		// Resolve aliases within this sequence that reference local anchors
		if len(localAnchors) > 0 {
			resolveAliasesInSequence(n, localAnchors)
		}

		// Recurse into child sequences
		for _, value := range n.Values {
			resolveNestedSequenceAliases(value)
		}

	case *ast.AnchorNode:
		resolveNestedSequenceAliases(n.Value)

	case *ast.DocumentNode:
		resolveNestedSequenceAliases(n.Body)
	}
}

// resolveAliasesInSequence resolves aliases within a sequence that reference the given anchors.
func resolveAliasesInSequence(node ast.Node, anchors map[string]ast.Node) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *ast.MappingNode:
		for _, value := range n.Values {
			resolveAliasesInSequence(value, anchors)
		}

	case *ast.MappingValueNode:
		// Don't resolve aliases that are values of merge keys (<<: *alias)
		if isMergeKey(n.Key) {
			return
		}

		if alias, ok := n.Value.(*ast.AliasNode); ok {
			anchorName := alias.Value.GetToken().Value
			if anchorValue, exists := anchors[anchorName]; exists {
				n.Value = anchorValue
			}
		} else {
			resolveAliasesInSequence(n.Value, anchors)
		}

	case *ast.SequenceNode:
		for i, value := range n.Values {
			if alias, ok := value.(*ast.AliasNode); ok {
				anchorName := alias.Value.GetToken().Value
				if anchorValue, exists := anchors[anchorName]; exists {
					n.Values[i] = anchorValue
				}
			} else {
				resolveAliasesInSequence(value, anchors)
			}
		}

	case *ast.AnchorNode:
		resolveAliasesInSequence(n.Value, anchors)
	}
}

// isMergeKey checks if a node is a merge key (<<).
func isMergeKey(node ast.Node) bool {
	if node == nil {
		return false
	}
	// Check for MergeKeyNode
	if _, ok := node.(*ast.MergeKeyNode); ok {
		return true
	}
	// Check for string "<<"
	if str, ok := node.(*ast.StringNode); ok && str.Value == "<<" {
		return true
	}
	return false
}

// collectAnchors recursively collects all anchor definitions.
func collectAnchors(node ast.Node, anchors map[string]ast.Node) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *ast.AnchorNode:
		if n.Name != nil {
			anchors[n.Name.GetToken().Value] = n.Value
		}
		collectAnchors(n.Value, anchors)

	case *ast.MappingNode:
		for _, value := range n.Values {
			collectAnchors(value, anchors)
		}

	case *ast.MappingValueNode:
		collectAnchors(n.Key, anchors)
		collectAnchors(n.Value, anchors)

	case *ast.SequenceNode:
		for _, value := range n.Values {
			collectAnchors(value, anchors)
		}

	case *ast.DocumentNode:
		collectAnchors(n.Body, anchors)
	}
}

// trySequenceMerge checks if a node is a sequence merge (- <<: *alias)
// and returns the expanded values if so.
func trySequenceMerge(node ast.Node, anchors map[string]ast.Node) []ast.Node {
	// Pattern: MappingValueNode with key "<<" and alias value
	mapping, ok := node.(*ast.MappingNode)
	if !ok || len(mapping.Values) != 1 {
		return nil
	}

	mv := mapping.Values[0]
	if mv == nil {
		return nil
	}

	// Check if key is "<<"
	keyStr, ok := mv.Key.(*ast.StringNode)
	if !ok {
		// Also check for MergeKeyNode
		if _, ok := mv.Key.(*ast.MergeKeyNode); !ok {
			return nil
		}
	} else if keyStr.Value != "<<" {
		return nil
	}

	// Get the alias value
	alias, ok := mv.Value.(*ast.AliasNode)
	if !ok {
		return nil
	}

	// Look up the anchor
	anchorName := alias.Value.GetToken().Value
	anchorValue, exists := anchors[anchorName]
	if !exists {
		return nil
	}

	// If the anchor points to a sequence, expand it
	seq, ok := anchorValue.(*ast.SequenceNode)
	if !ok {
		return nil
	}

	// Return copies of the sequence values
	result := make([]ast.Node, len(seq.Values))
	copy(result, seq.Values)
	return result
}
