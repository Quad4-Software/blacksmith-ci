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

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegistryValidateAddress(t *testing.T) {
	valid := []string{"docker.io", "rg.fr-par.scw.cloud", "registry.example.com:5000", "localhost:5000"}
	for _, addr := range valid {
		r := &Registry{Address: addr, Username: "u", Password: "p"}
		assert.NoError(t, r.Validate(), addr)
	}

	invalid := []string{"rg.fr-par.scw.cloud/gudjam", "quay.io/org", "https://docker.io", "docker.io?x", "user@docker.io", "docker .io"}
	for _, addr := range invalid {
		r := &Registry{Address: addr, Username: "u", Password: "p"}
		assert.ErrorIs(t, r.Validate(), errRegistryAddressNotHost, addr)
	}
}
