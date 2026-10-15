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

package groupmap_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/auth/groupmap"
)

func TestMapping(t *testing.T) {
	m := groupmap.Mapping{
		AllowedGroups: []string{"staff"},
		AdminGroups:   []string{"admins"},
		RoleGroups: map[string][]string{
			"operator":        {"ops"},
			"auditor":         {"audit", "admins"},
			"secrets-manager": nil,
		},
	}
	assert.True(t, m.RestrictsGroups())
	assert.True(t, m.IsAllowed([]string{"x", "staff"}))
	assert.False(t, m.IsAllowed([]string{"Staff"}), "exact match by default")
	assert.True(t, m.ManagesAdmin())
	assert.True(t, m.IsAdmin([]string{"admins"}))
	assert.False(t, m.IsAdmin(nil))
	assert.True(t, m.ManagesGroups())
	assert.Equal(t, []string{"auditor", "operator"}, m.ManagedRoles())
	assert.Equal(t, []string{"auditor"}, m.RolesFor([]string{"admins"}))
	assert.Equal(t, []string{}, m.RolesFor(nil))
}

func TestEmptyMapping(t *testing.T) {
	var m groupmap.Mapping
	assert.False(t, m.RestrictsGroups())
	assert.True(t, m.IsAllowed(nil))
	assert.False(t, m.ManagesAdmin())
	assert.False(t, m.IsAdmin([]string{"admins"}))
	assert.False(t, m.ManagesGroups())
	assert.Equal(t, []string{}, m.ManagedRoles())
}

func TestCustomMatch(t *testing.T) {
	m := groupmap.Mapping{
		AdminGroups: []string{"ADMINS"},
		Match:       strings.EqualFold,
	}
	assert.True(t, m.IsAdmin([]string{"admins"}))
}
