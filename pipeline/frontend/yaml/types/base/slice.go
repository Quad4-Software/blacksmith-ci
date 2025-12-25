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

package base

import (
	"errors"
	"fmt"
)

// StringOrSlice represents a string or an array of strings.
// We need to override the yaml decoder to accept both options.
type StringOrSlice []string

// UnmarshalYAML implements the Unmarshaler interface.
func (s *StringOrSlice) UnmarshalYAML(unmarshal func(any) error) error {
	// First try to detect if it's null/empty
	var raw any
	if err := unmarshal(&raw); err != nil {
		return err
	}
	if raw == nil {
		*s = nil
		return nil
	}

	// Try as string
	if stringType, ok := raw.(string); ok {
		if stringType == "" {
			*s = nil
			return nil
		}
		*s = []string{stringType}
		return nil
	}

	// Try as slice
	if sliceType, ok := raw.([]any); ok {
		parts, err := toStrings(sliceType)
		if err != nil {
			return err
		}
		*s = parts
		return nil
	}

	return errors.New("failed to unmarshal StringOrSlice")
}

func toStrings(s []any) ([]string, error) {
	if s == nil {
		return nil, nil
	}
	r := make([]string, len(s))
	for k, v := range s {
		if sv, ok := v.(string); ok {
			r[k] = sv
		} else if mv, ok := v.(map[string]any); ok {
			// Handle case where YAML parsed a string with colon as a map
			// e.g., `git commit -m "docs: add version"` becomes map["git commit -m \"docs": "add version\""]
			r[k] = mapToString(mv)
		} else {
			return nil, fmt.Errorf("cannot unmarshal '%v' of type %T into a string value", v, v)
		}
	}
	return r, nil
}

// mapToString reconstructs a string from a map that was incorrectly parsed
// due to YAML interpreting a colon as a key-value separator.
func mapToString(m map[string]any) string {
	var result string
	for key, val := range m {
		if result != "" {
			result += " "
		}
		switch v := val.(type) {
		case string:
			result += key + ": " + v
		case nil:
			result += key
		default:
			result += fmt.Sprintf("%s: %v", key, v)
		}
	}
	return result
}
