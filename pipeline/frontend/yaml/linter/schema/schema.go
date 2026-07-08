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

package schema

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	goyaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"

	"codefloe.com/crowci/crow/v6/shared/yaml"
)

//go:embed schema.json
var schemaDefinition []byte

// SchemaError represents a JSON schema validation error with field and description.
type SchemaError struct {
	field       string
	description string
	line        int
	column      int
}

// Field returns the YAML path to the field that caused the error.
func (e *SchemaError) Field() string {
	return e.field
}

// Description returns a user-friendly error description.
func (e *SchemaError) Description() string {
	return e.description
}

// Line returns the line number (1-based) where the error occurred, or 0 if unknown.
func (e *SchemaError) Line() int {
	return e.line
}

// Column returns the column number (1-based) where the error occurred, or 0 if unknown.
func (e *SchemaError) Column() int {
	return e.column
}

// compileSchema compiles the embedded schema definition.
func compileSchema() (*jsonschema.Schema, error) {
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaDefinition))
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", schemaDoc); err != nil {
		return nil, fmt.Errorf("failed to add schema resource: %w", err)
	}

	sch, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}

	return sch, nil
}

// pathLocations maps JSON pointer paths to YAML line/column positions.
type pathLocations map[string]struct {
	line   int
	column int
}

// buildPathLocations walks the YAML AST and builds a map from JSON paths to line numbers.
func buildPathLocations(data []byte) pathLocations {
	locations := make(pathLocations)

	file, err := parser.ParseBytes(data, 0)
	if err != nil {
		return locations
	}

	for _, doc := range file.Docs {
		walkNode(doc.Body, "", locations)
	}

	return locations
}

// walkNode recursively walks the AST and records path locations.
func walkNode(node ast.Node, path string, locations pathLocations) {
	if node == nil {
		return
	}

	// Record the position for this path on first visit only.
	// Children walked with the same path (e.g. MappingNode → MappingValueNode → value)
	// would otherwise overwrite it and the last child would win, pointing the error
	// at the wrong line.
	if _, ok := locations[path]; !ok {
		tok := node.GetToken()
		if tok != nil && tok.Position != nil {
			locations[path] = struct {
				line   int
				column int
			}{
				line:   tok.Position.Line,
				column: tok.Position.Column,
			}
		}
	}

	switch n := node.(type) {
	case *ast.MappingNode:
		for _, value := range n.Values {
			walkNode(value, path, locations)
		}

	case *ast.MappingValueNode:
		keyName := getKeyName(n.Key)
		childPath := path
		if keyName != "" {
			if path == "" {
				childPath = "/" + keyName
			} else {
				childPath = path + "/" + keyName
			}
		}
		// Record the key position (overrides any earlier set from walkNode top,
		// because the key line/column is more useful than the value's first token).
		if tok := n.Key.GetToken(); tok != nil && tok.Position != nil {
			locations[childPath] = struct {
				line   int
				column int
			}{
				line:   tok.Position.Line,
				column: tok.Position.Column,
			}
		}
		walkNode(n.Value, childPath, locations)

	case *ast.SequenceNode:
		for i, value := range n.Values {
			childPath := fmt.Sprintf("%s/%d", path, i)
			walkNode(value, childPath, locations)
		}

	case *ast.AnchorNode:
		walkNode(n.Value, path, locations)

	case *ast.AliasNode:
		// Aliases don't contribute to path locations

	case *ast.DocumentNode:
		walkNode(n.Body, path, locations)
	}
}

// getKeyName extracts the key name from a mapping key node.
func getKeyName(node ast.Node) string {
	switch n := node.(type) {
	case *ast.StringNode:
		return n.Value
	default:
		tok := node.GetToken()
		if tok != nil {
			return tok.Value
		}
	}
	return ""
}

// jsonPointerToYAMLPath converts a JSON pointer like "/steps/0/image" to a
// YAML-style path like "steps[0].image". A dot is written before every named
// segment except the first; a numeric segment is rendered as `[N]` with no
// dot in front of it.
func jsonPointerToYAMLPath(pointer string) string {
	if pointer == "" || pointer == "/" {
		return "(root)"
	}

	pointer = strings.TrimPrefix(pointer, "/")
	parts := strings.Split(pointer, "/")

	var result strings.Builder
	for i, part := range parts {
		if isNumeric(part) {
			result.WriteString("[" + part + "]")
			continue
		}
		if i > 0 {
			result.WriteString(".")
		}
		result.WriteString(part)
	}

	return result.String()
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// validProperties lists the valid top-level properties in the schema.
var validProperties = []string{
	"steps", "services", "clone", "skip_clone", "when", "workspace",
	"matrix", "labels", "depends_on", "runs_on", "name", "variables", "$schema",
}

// describeKind turns a concrete ErrorKind into a user-friendly message.
// Returns "" when the kind is a container (OneOf, AnyOf, AllOf, etc.) that
// should not be surfaced on its own — callers should recurse into its causes.
func describeKind(k jsonschema.ErrorKind) string {
	switch k := k.(type) {
	case *kind.Type:
		want := strings.Join(k.Want, " or ")
		return fmt.Sprintf("Expected %s, got %s.", want, k.Got)

	case *kind.AdditionalProperties:
		if len(k.Properties) == 1 {
			name := k.Properties[0]
			if suggestion := findSimilarProperty(name); suggestion != "" {
				return fmt.Sprintf("Unknown property `%s`. Did you mean `%s`?", name, suggestion)
			}
			return fmt.Sprintf("Unknown property `%s`.", name)
		}
		quoted := make([]string, len(k.Properties))
		for i, p := range k.Properties {
			quoted[i] = "`" + p + "`"
		}
		return fmt.Sprintf("Unknown properties: %s.", strings.Join(quoted, ", "))

	case *kind.Required:
		if len(k.Missing) == 1 {
			return fmt.Sprintf("Required property `%s` is missing.", k.Missing[0])
		}
		quoted := make([]string, len(k.Missing))
		for i, p := range k.Missing {
			quoted[i] = "`" + p + "`"
		}
		return fmt.Sprintf("Required properties missing: %s.", strings.Join(quoted, ", "))

	case *kind.Enum:
		quoted := make([]string, len(k.Want))
		for i, v := range k.Want {
			quoted[i] = fmt.Sprintf("`%v`", v)
		}
		return fmt.Sprintf("Value `%v` is not allowed. Must be one of: %s.", k.Got, strings.Join(quoted, ", "))

	case *kind.Const:
		return fmt.Sprintf("Value `%v` is not allowed. Must be `%v`.", k.Got, k.Want)

	case *kind.Pattern:
		return fmt.Sprintf("Value `%s` does not match the required pattern `%s`.", k.Got, k.Want)

	case *kind.MinLength:
		return fmt.Sprintf("Value is too short (got length %d, minimum %d).", k.Got, k.Want)
	case *kind.MaxLength:
		return fmt.Sprintf("Value is too long (got length %d, maximum %d).", k.Got, k.Want)
	case *kind.MinItems:
		return fmt.Sprintf("List is too short (got %d items, minimum %d).", k.Got, k.Want)
	case *kind.MaxItems:
		return fmt.Sprintf("List is too long (got %d items, maximum %d).", k.Got, k.Want)
	case *kind.MinProperties:
		return fmt.Sprintf("Object has too few properties (got %d, minimum %d).", k.Got, k.Want)
	case *kind.MaxProperties:
		return fmt.Sprintf("Object has too many properties (got %d, maximum %d).", k.Got, k.Want)

	case *kind.UniqueItems:
		return fmt.Sprintf("List items must be unique (duplicate at positions %d and %d).", k.Duplicates[0], k.Duplicates[1])

	case *kind.Format:
		return fmt.Sprintf("Value does not match the `%s` format.", k.Want)

	case *kind.FalseSchema:
		return "This value is not allowed here."

	// Container kinds — let the caller recurse into causes.
	case *kind.OneOf, *kind.AnyOf, *kind.AllOf, *kind.Not,
		*kind.Reference, *kind.Schema, *kind.Group:
		return ""
	}

	// Fallback: use the library's own localized rendering.
	return k.LocalizedString(nil)
}

// findSimilarProperty finds a similar valid property name using simple string matching.
func findSimilarProperty(input string) string {
	input = strings.ToLower(input)

	// Common typos and their corrections
	typoMap := map[string]string{
		"service":    "services",
		"step":       "steps",
		"pipeline":   "steps",
		"pipelines":  "steps",
		"label":      "labels",
		"depend_on":  "depends_on",
		"dependson":  "depends_on",
		"depends":    "depends_on",
		"run_on":     "runs_on",
		"runson":     "runs_on",
		"run":        "runs_on",
		"skip":       "skip_clone",
		"skipclone":  "skip_clone",
		"workspace":  "workspace",
		"workspaces": "workspace",
		"var":        "variables",
		"vars":       "variables",
		"variable":   "variables",
		"env":        "environment (in steps)",
		"image":      "image (inside steps)",
		"command":    "commands (inside steps)",
		"commands":   "commands (inside steps)",
	}

	if suggestion, ok := typoMap[input]; ok {
		return suggestion
	}

	// Check for prefix matches
	for _, prop := range validProperties {
		if strings.HasPrefix(prop, input) || strings.HasPrefix(input, prop) {
			return prop
		}
	}

	return ""
}

// extractErrors converts a ValidationError tree into a slice of SchemaErrors.
//
// It walks the Causes tree and surfaces leaf errors only. For OneOf/AnyOf
// containers it picks the single branch the user got closest to satisfying
// (the deepest InstanceLocation under that branch) — otherwise the failing
// alternatives of every disjunction would all surface as noise.
//
// Results are deduplicated by (InstanceLocation, message) so the same leaf
// reached through multiple AllOf/$ref paths does not appear twice.
func extractErrors(validationErr *jsonschema.ValidationError, locations pathLocations) []*SchemaError {
	leaves := collectLeaves(validationErr)
	seen := make(map[string]struct{}, len(leaves))
	var out []*SchemaError

	for _, leaf := range leaves {
		msg := describeKind(leaf.ErrorKind)
		if msg == "" {
			// describeKind returned empty (container kind that slipped through)
			// — fall back to the lib's localized text rather than dropping the error.
			msg = leaf.ErrorKind.LocalizedString(nil)
		}

		// AdditionalProperties reports failure on the parent object, but the
		// useful location is the unknown child key itself. Reach down so the
		// 'jump to line' link in the UI lands on the offending key.
		field := instancePointer(leaf.InstanceLocation)
		if ap, ok := leaf.ErrorKind.(*kind.AdditionalProperties); ok && len(ap.Properties) == 1 {
			child := field + "/" + escapePointerSegment(ap.Properties[0])
			if _, hasLoc := locations[child]; hasLoc {
				field = child
			}
		}

		key := field + "\x00" + msg
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}

		line, column := 0, 0
		if loc, ok := locations[field]; ok {
			line = loc.line
			column = loc.column
		}

		out = append(out, &SchemaError{
			field:       jsonPointerToYAMLPath(field),
			description: msg,
			line:        line,
			column:      column,
		})
	}

	// Defensive: if for some reason the tree produced no leaves, surface the
	// top-level error rather than returning an empty slice with err != nil.
	if len(out) == 0 {
		field := instancePointer(validationErr.InstanceLocation)
		line, column := 0, 0
		if loc, ok := locations[field]; ok {
			line = loc.line
			column = loc.column
		}
		out = append(out, &SchemaError{
			field:       jsonPointerToYAMLPath(field),
			description: validationErr.ErrorKind.LocalizedString(nil),
			line:        line,
			column:      column,
		})
	}

	return out
}

// collectLeaves walks the ValidationError tree and returns the concrete
// (non-container) failures that should be shown to the user.
func collectLeaves(verr *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if verr == nil {
		return nil
	}
	if len(verr.Causes) == 0 {
		return []*jsonschema.ValidationError{verr}
	}

	switch verr.ErrorKind.(type) {
	case *kind.OneOf, *kind.AnyOf:
		// All branches failed. Surface only the branch the user got closest to
		// satisfying — i.e. the one whose deepest leaf reaches furthest into
		// the instance. The others are alternatives the user clearly did not
		// intend (e.g. "got object, want array" when they wrote a map).
		best := verr.Causes[0]
		bestDepth := maxLeafDepth(best)
		for _, c := range verr.Causes[1:] {
			if d := maxLeafDepth(c); d > bestDepth {
				best = c
				bestDepth = d
			}
		}
		return collectLeaves(best)

	default:
		// AllOf, Reference, Schema, Group and other container kinds — every
		// cause is independently failing and worth surfacing.
		var leaves []*jsonschema.ValidationError
		for _, c := range verr.Causes {
			leaves = append(leaves, collectLeaves(c)...)
		}
		return leaves
	}
}

// maxLeafDepth returns the deepest InstanceLocation length found among the
// leaves of this subtree. Used as a "closeness to valid" score for picking
// among OneOf/AnyOf branches.
func maxLeafDepth(verr *jsonschema.ValidationError) int {
	if verr == nil {
		return 0
	}
	if len(verr.Causes) == 0 {
		return len(verr.InstanceLocation)
	}
	max := len(verr.InstanceLocation)
	for _, c := range verr.Causes {
		if d := maxLeafDepth(c); d > max {
			max = d
		}
	}
	return max
}

// instancePointer converts the library's []string instance location back into
// a JSON pointer string (e.g. ["steps","typecheck","environment"] → "/steps/typecheck/environment").
func instancePointer(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	var b strings.Builder
	for _, p := range parts {
		b.WriteByte('/')
		b.WriteString(escapePointerSegment(p))
	}
	return b.String()
}

// escapePointerSegment escapes a single JSON pointer segment per RFC 6901
// (`~` → `~0`, `/` → `~1`).
func escapePointerSegment(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}

// Lint lints an io.Reader against the Crow `schema.json`.
func Lint(r io.Reader) ([]*SchemaError, error) {
	// Compile schema
	sch, err := compileSchema()
	if err != nil {
		return nil, err
	}

	// Read YAML config
	rBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to load yml file: %w", err)
	}

	// Build path-to-line mapping from YAML AST
	locations := buildPathLocations(rBytes)

	// parse yaml (with sequence merge support)
	var yamlDoc any
	if err := yaml.Unmarshal(rBytes, &yamlDoc); err != nil {
		return nil, fmt.Errorf("failed to parse yml file: %w", err)
	}

	// convert to json
	jsonDoc, err := json.Marshal(yamlDoc)
	if err != nil {
		return nil, fmt.Errorf("failed to convert yaml to json: %w", err)
	}

	// Parse JSON document
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(jsonDoc))
	if err != nil {
		return nil, fmt.Errorf("failed to parse json document: %w", err)
	}

	// Validate
	err = sch.Validate(doc)
	if err != nil {
		validationErr := &jsonschema.ValidationError{}
		ok := errors.As(err, &validationErr)
		if !ok {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
		return extractErrors(validationErr, locations), fmt.Errorf("config not valid")
	}

	return nil, nil
}

// LintString lints a string against the Crow schema.
func LintString(s string) ([]*SchemaError, error) {
	return Lint(bytes.NewBufferString(s))
}

// Ensure goyaml import is used (for AST types compatibility check).
var _ = goyaml.Marshal
