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

	"github.com/santhosh-tekuri/jsonschema/v6"

	"codeberg.org/crowci/crow/v4/shared/yaml"
)

//go:embed schema.json
var schemaDefinition []byte

// SchemaError represents a JSON schema validation error with field and description.
type SchemaError struct {
	field       string
	description string
}

// Field returns the JSON path to the field that caused the error.
func (e *SchemaError) Field() string {
	return e.field
}

// Description returns the error description.
func (e *SchemaError) Description() string {
	return e.description
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

// extractErrors converts a ValidationError into a slice of SchemaErrors.
func extractErrors(validationErr *jsonschema.ValidationError) []*SchemaError {
	var errors []*SchemaError

	// Get basic output which contains error details
	basic := validationErr.BasicOutput()
	if basic.Errors != nil {
		for _, e := range basic.Errors {
			errMsg := ""
			if e.Error != nil {
				errMsg = e.Error.String()
			}
			errors = append(errors, &SchemaError{
				field:       e.InstanceLocation,
				description: errMsg,
			})
		}
	}

	// If no nested errors, use the top-level error
	if len(errors) == 0 {
		errMsg := validationErr.Error()
		if basic.Error != nil {
			errMsg = basic.Error.String()
		}
		errors = append(errors, &SchemaError{
			field:       basic.InstanceLocation,
			description: errMsg,
		})
	}

	return errors
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
		return extractErrors(validationErr), fmt.Errorf("config not valid")
	}

	return nil, nil
}

// LintString lints a string against the Crow schema.
func LintString(s string) ([]*SchemaError, error) {
	return Lint(bytes.NewBufferString(s))
}
