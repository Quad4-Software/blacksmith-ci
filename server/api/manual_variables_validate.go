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

package api

import (
	"fmt"
	"slices"

	"codefloe.com/crowci/crow/v5/pipeline/frontend/yaml"
)

// validateManualVariables checks submitted values against the allowed set:
// unknown variables, values outside declared options, and missing required
// variables are rejected.
func validateManualVariables(submitted map[string]string, allowed map[string]yaml.ManualVariable) error {
	for name, value := range submitted {
		a, ok := allowed[name]
		if !ok {
			return fmt.Errorf("variable %q is not accepted by the selected workflows", name)
		}
		if len(a.Options) > 0 && !slices.Contains(a.Options, value) {
			return fmt.Errorf("variable %q value %q is not one of the allowed options", name, value)
		}
	}
	for name, a := range allowed {
		if a.Required {
			if v, ok := submitted[name]; !ok || v == "" {
				return fmt.Errorf("variable %q is required", name)
			}
		}
	}
	return nil
}
