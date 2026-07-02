// Copyright 2026 Crow Authors
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

package yaml

import (
	sharedYaml "codefloe.com/crowci/crow/v5/shared/yaml"
)

// Variable describes a user-settable pipeline variable declared in the
// top-level "variables:" block. The map key (the variable name) is stored
// separately by callers.
type Variable struct {
	Description string   `yaml:"description,omitempty"`
	Options     []string `yaml:"options,omitempty"`
	Default     string   `yaml:"default,omitempty"`
	Required    bool     `yaml:"required,omitempty"`
}

// ParseVariableDefs extracts the structured top-level "variables" block.
// Values that are neither scalars nor mappings (e.g. YAML anchor sequences)
// are skipped. A "variables" block written as a sequence — the documented
// anchor form (e.g. `variables:\n  - &img 'golang'`) — declares no
// user-settable variables and yields an empty result.
func ParseVariableDefs(data []byte) (map[string]Variable, error) {
	var doc struct {
		Variables variableDefs `yaml:"variables"`
	}
	if err := sharedYaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	result := make(map[string]Variable, len(doc.Variables))
	for k, v := range doc.Variables {
		if v.skip {
			continue
		}
		result[k] = v.value
	}
	return result, nil
}

// variableDefs is the top-level "variables" mapping of name to declaration.
// It accepts the anchor sequence form (`variables:\n  - &img 'golang'`) by
// treating it as an empty declaration set rather than failing the whole parse.
type variableDefs map[string]variableNode

func (d *variableDefs) UnmarshalYAML(unmarshal func(any) error) error {
	var m map[string]variableNode
	if err := unmarshal(&m); err != nil {
		// A sequence (the anchor form) carries no declared variables. Ignore it;
		// surface any other error (genuinely malformed YAML).
		var seq []any
		if seqErr := unmarshal(&seq); seqErr == nil {
			*d = variableDefs{}
			return nil
		}
		return err
	}
	*d = m
	return nil
}

// ParseVariableDefsString is the string variant of ParseVariableDefs.
func ParseVariableDefsString(data string) (map[string]Variable, error) {
	return ParseVariableDefs([]byte(data))
}

// variableNode accepts either a scalar (treated as the default value) or a
// mapping with the rich fields; anything else is skipped.
type variableNode struct {
	value Variable
	skip  bool
}

// knownVariableKeys are the YAML keys that identify a rich Variable mapping.
var knownVariableKeys = map[string]bool{
	"description": true,
	"options":     true,
	"default":     true,
	"required":    true,
}

func (n *variableNode) UnmarshalYAML(unmarshal func(any) error) error {
	// Try scalar first (e.g. FOO: "3.12").
	var scalar string
	if err := unmarshal(&scalar); err == nil {
		n.value = Variable{Default: scalar}
		return nil
	}

	// Try as a generic map to inspect which keys are present.
	// This lets us reject arbitrary YAML anchor mappings (e.g. step templates)
	// that happen to unmarshal into a zero-value Variable struct.
	var raw map[string]any
	if err := unmarshal(&raw); err != nil {
		// Non-scalar, non-mapping (e.g. anchor sequence): ignore.
		n.skip = true
		return nil
	}
	hasKnownKey := false
	for k := range raw {
		if knownVariableKeys[k] {
			hasKnownKey = true
			break
		}
	}
	if len(raw) > 0 && !hasKnownKey {
		// Non-empty mapping with none of the Variable fields — treat as an anchor template, skip.
		n.skip = true
		return nil
	}

	var v Variable
	if err := unmarshal(&v); err != nil {
		return err
	}
	n.value = v
	return nil
}

// ParseVariables extracts the top-level "variables" block as a name->default
// map, preserving backward compatibility for runtime substitution and the CLI.
func ParseVariables(data []byte) (map[string]string, error) {
	defs, err := ParseVariableDefs(data)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(defs))
	for k, v := range defs {
		result[k] = v.Default
	}
	return result, nil
}

// ParseVariablesString extracts scalar string variables from a YAML string.
func ParseVariablesString(data string) (map[string]string, error) {
	return ParseVariables([]byte(data))
}
