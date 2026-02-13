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
	"regexp"
	"strings"

	goyaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"codefloe.com/crowci/crow/v5/shared/yaml"
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

	// Get the token position for this node
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
		// Record the key position
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

// jsonPointerToYAMLPath converts a JSON pointer like "/steps/0/image" to a YAML-style path like "steps[0].image".
func jsonPointerToYAMLPath(pointer string) string {
	if pointer == "" || pointer == "/" {
		return "(root)"
	}

	// Remove leading slash
	pointer = strings.TrimPrefix(pointer, "/")
	parts := strings.Split(pointer, "/")

	var result strings.Builder
	for i, part := range parts {
		// Check if this part is a numeric index
		if isNumeric(part) {
			result.WriteString("[" + part + "]")
		} else {
			if i > 0 && !isNumeric(parts[i-1]) {
				result.WriteString(".")
			}
			result.WriteString(part)
		}
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

// makeUserFriendlyMessage converts technical JSON schema error messages to user-friendly messages.
func makeUserFriendlyMessage(errMsg string) string {
	// Handle "additional properties X not allowed"
	if strings.Contains(errMsg, "additional properties") {
		re := regexp.MustCompile(`additional properties '([^']+)' not allowed`)
		if matches := re.FindStringSubmatch(errMsg); len(matches) > 1 {
			propName := matches[1]

			// Find similar property names
			suggestion := findSimilarProperty(propName)
			if suggestion != "" {
				return fmt.Sprintf("Unknown property `%s`. Did you mean `%s`?", propName, suggestion)
			}

			return fmt.Sprintf("Unknown property `%s`. This property is not recognized in the Crow configuration.", propName)
		}
	}

	// Handle "missing properties"
	if strings.Contains(errMsg, "missing properties") {
		re := regexp.MustCompile(`missing properties: '([^']+)'`)
		if matches := re.FindStringSubmatch(errMsg); len(matches) > 1 {
			return fmt.Sprintf("Required property `%s` is missing.", matches[1])
		}
	}

	// Handle type errors
	if strings.Contains(errMsg, "expected") && strings.Contains(errMsg, "but got") {
		return errMsg // Already reasonably clear
	}

	// Handle enum errors
	if strings.Contains(errMsg, "value must be one of") {
		return errMsg // Already clear
	}

	// Handle "doesn't validate with" - often too technical, simplify
	if strings.Contains(errMsg, "doesn't validate with") {
		// Extract the key information if possible
		if strings.Contains(errMsg, "required") {
			return "This section is missing required properties."
		}
		if strings.Contains(errMsg, "additionalProperties") {
			return "This section contains unrecognized properties."
		}
	}

	return errMsg
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

// extractErrors converts a ValidationError into a slice of SchemaErrors with line numbers.
func extractErrors(validationErr *jsonschema.ValidationError, locations pathLocations) []*SchemaError {
	var schemaErrors []*SchemaError

	// Get basic output which contains error details
	basic := validationErr.BasicOutput()
	if basic.Errors != nil {
		for _, e := range basic.Errors {
			errMsg := ""
			if e.Error != nil {
				errMsg = e.Error.String()
			}

			field := e.InstanceLocation
			friendlyField := jsonPointerToYAMLPath(field)
			friendlyMsg := makeUserFriendlyMessage(errMsg)

			// Look up line number
			line, column := 0, 0
			if loc, ok := locations[field]; ok {
				line = loc.line
				column = loc.column
			}

			schemaErrors = append(schemaErrors, &SchemaError{
				field:       friendlyField,
				description: friendlyMsg,
				line:        line,
				column:      column,
			})
		}
	}

	// If no nested errors, use the top-level error
	if len(schemaErrors) == 0 {
		errMsg := validationErr.Error()
		if basic.Error != nil {
			errMsg = basic.Error.String()
		}

		field := basic.InstanceLocation
		friendlyField := jsonPointerToYAMLPath(field)
		friendlyMsg := makeUserFriendlyMessage(errMsg)

		line, column := 0, 0
		if loc, ok := locations[field]; ok {
			line = loc.line
			column = loc.column
		}

		schemaErrors = append(schemaErrors, &SchemaError{
			field:       friendlyField,
			description: friendlyMsg,
			line:        line,
			column:      column,
		})
	}

	return schemaErrors
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
