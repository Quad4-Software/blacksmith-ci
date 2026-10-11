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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserToEntityRoles(t *testing.T) {
	e := UserToEntity(&User{ID: 2, Login: "bob", Roles: JSONSliceString{"auditor", "operator"}})
	assert.Equal(t, []string{"auditor", "operator"}, e.Roles)

	out, err := json.Marshal(e)
	require.NoError(t, err)
	assert.Contains(t, string(out), `"roles":["auditor","operator"]`)
}

func TestOrgToEntityHasNoRoles(t *testing.T) {
	out, err := json.Marshal(OrgToEntity(&Org{ID: 3, Name: "acme"}))
	require.NoError(t, err)
	assert.NotContains(t, string(out), `"roles"`)
}

func TestUserToEntityDropsUnknownRoles(t *testing.T) {
	e := UserToEntity(&User{ID: 2, Login: "bob", Roles: JSONSliceString{"root", "auditor"}})
	assert.Equal(t, []string{"auditor"}, e.Roles)
}
