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
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml"
)

func TestValidateManualVariables(t *testing.T) {
	allowed := map[string]yaml.ManualVariable{
		"DEPLOY_ENV": {Name: "DEPLOY_ENV", Options: []string{"staging", "prod"}, Required: true},
		"NOTE":       {Name: "NOTE"},
	}

	assert.NoError(t, validateManualVariables(map[string]string{"DEPLOY_ENV": "prod"}, allowed))
	assert.NoError(t, validateManualVariables(map[string]string{"DEPLOY_ENV": "prod", "NOTE": "hi"}, allowed))
	assert.Error(t, validateManualVariables(map[string]string{"DEPLOY_ENV": "prod", "UNKNOWN": "x"}, allowed)) // unknown
	assert.Error(t, validateManualVariables(map[string]string{"DEPLOY_ENV": "qa"}, allowed))                   // bad option
	assert.Error(t, validateManualVariables(map[string]string{"NOTE": "hi"}, allowed))                         // missing required
	assert.Error(t, validateManualVariables(nil, allowed))                                                     // nil map, required missing
	assert.Error(t, validateManualVariables(map[string]string{}, allowed))                                     // empty map, required missing
}
