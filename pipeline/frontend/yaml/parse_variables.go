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

// ParseVariables extracts scalar string key-value pairs from the top-level
// "variables" block. Non-scalar values (maps, sequences) are ignored since
// those are typically used only for YAML anchors/aliases.
func ParseVariables(data []byte) (map[string]string, error) {
	var doc struct {
		Variables map[string]any `yaml:"variables"`
	}
	if err := sharedYaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for k, v := range doc.Variables {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result, nil
}

// ParseVariablesString extracts scalar string variables from a YAML string.
func ParseVariablesString(data string) (map[string]string, error) {
	return ParseVariables([]byte(data))
}
