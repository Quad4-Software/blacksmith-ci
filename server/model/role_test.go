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
	"os"
	"regexp"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserCan(t *testing.T) {
	tests := []struct {
		name  string
		user  User
		allow []Permission
		deny  []Permission
	}{
		{
			name: "plain user",
			user: User{},
			deny: AllPermissions(),
		},
		{
			name:  "admin",
			user:  User{Admin: true},
			allow: AllPermissions(),
		},
		{
			name:  "operator",
			user:  User{Roles: JSONSliceString{"operator"}},
			allow: []Permission{PermQueueRead, PermQueueWrite, PermAgentsRead, PermAgentsWrite, PermMaintenanceRead, PermMetricsAll},
			deny:  []Permission{PermUsersRead, PermUsersWrite, PermSecretsWrite, PermMaintenanceWrite, PermServerAdmin, PermReposRead, PermAgentsRegister},
		},
		{
			name:  "secrets manager",
			user:  User{Roles: JSONSliceString{"secrets-manager"}},
			allow: []Permission{PermSecretsWrite},
			deny:  []Permission{PermQueueRead, PermAgentsWrite, PermUsersRead, PermServerAdmin, PermAgentsRegister},
		},
		{
			name:  "auditor",
			user:  User{Roles: JSONSliceString{"auditor"}},
			allow: []Permission{PermUsersRead, PermOrgsRead, PermReposRead, PermQueueRead, PermAgentsRead, PermMaintenanceRead, PermMetricsAll},
			deny:  []Permission{PermUsersWrite, PermQueueWrite, PermAgentsWrite, PermSecretsWrite, PermMaintenanceWrite, PermServerAdmin, PermOrgsDelete, PermReposRepair, PermAgentsRegister},
		},
		{
			name: "unknown role grants nothing",
			user: User{Roles: JSONSliceString{"superuser", "ADMIN"}},
			deny: AllPermissions(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, p := range tt.allow {
				assert.True(t, tt.user.Can(p), "expected %s", p)
			}
			for _, p := range tt.deny {
				assert.False(t, tt.user.Can(p), "unexpected %s", p)
			}
		})
	}
}

func TestUserCanNil(t *testing.T) {
	var u *User
	assert.False(t, u.Can(PermUsersRead))
}

func TestParseRoles(t *testing.T) {
	roles, err := ParseRoles([]string{"auditor", "operator", "auditor"})
	require.NoError(t, err)
	assert.Equal(t, []Role{RoleAuditor, RoleOperator}, roles)

	_, err = ParseRoles([]string{"operator", "root"})
	assert.ErrorIs(t, err, ErrRoleInvalid)
	assert.ErrorContains(t, err, "root")

	roles, err = ParseRoles(nil)
	require.NoError(t, err)
	assert.Empty(t, roles)
}

func TestUserPermissionsSortedAndDistinct(t *testing.T) {
	u := User{Roles: JSONSliceString{"operator", "auditor"}}
	perms := u.Permissions()
	assert.IsIncreasing(t, perms)
	assert.Contains(t, perms, PermQueueWrite)
	assert.Contains(t, perms, PermUsersRead)
}

func TestUserReadWriteHelpers(t *testing.T) {
	assert.False(t, (&User{}).HasAnyReadPermission())
	assert.True(t, (&User{Roles: JSONSliceString{"auditor"}}).HasAnyReadPermission())
	assert.False(t, (&User{Roles: JSONSliceString{"auditor"}}).HasAnyWritePermission())
	assert.True(t, (&User{Roles: JSONSliceString{"secrets-manager"}}).HasAnyWritePermission())
	assert.True(t, (&User{Admin: true}).HasAnyWritePermission())
}

func TestUserMarshalJSONAddsRolesAndPermissions(t *testing.T) {
	b, err := json.Marshal(&User{Login: "alice"})
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(b, &got))
	assert.Equal(t, []any{}, got["roles"])
	assert.Equal(t, []any{}, got["permissions"])
	assert.Equal(t, "alice", got["login"])

	b, err = json.Marshal(User{Roles: JSONSliceString{"secrets-manager"}})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &got))
	assert.Equal(t, []any{"secrets-manager"}, got["roles"])
	assert.Equal(t, []any{"secrets:write"}, got["permissions"])
}

func TestPermissionTablesUseKnownPermissions(t *testing.T) {
	all := AllPermissions()
	assert.True(t, slices.IsSorted(all), "AllPermissions must be sorted")
	assert.Len(t, slices.Compact(slices.Clone(all)), len(all), "AllPermissions must be unique")
	for _, p := range readPermissions {
		assert.Contains(t, all, p, "read permission %s", p)
	}
	for role, perms := range rolePermissions {
		assert.True(t, role.IsValid(), "role %s", role)
		for _, p := range perms {
			assert.Contains(t, all, p, "permission %s of role %s", p, role)
		}
	}
}

// The web UI mirrors the permission lists in web/src/compositions/usePermissions.ts.
var tsPermissionList = regexp.MustCompile(`(?s)const (readPermissions|writePermissions): Permission\[\] = \[(.*?)\];`)

var tsPermission = regexp.MustCompile(`'([a-z]+:[a-z]+)'`)

func TestWebPermissionListsMatch(t *testing.T) {
	src, err := os.ReadFile("../../web/src/compositions/usePermissions.ts")
	require.NoError(t, err)

	lists := map[string][]Permission{}
	for _, m := range tsPermissionList.FindAllStringSubmatch(string(src), -1) {
		for _, p := range tsPermission.FindAllStringSubmatch(m[2], -1) {
			lists[m[1]] = append(lists[m[1]], Permission(p[1]))
		}
	}
	require.Contains(t, lists, "readPermissions")
	require.Contains(t, lists, "writePermissions")

	assert.ElementsMatch(t, readPermissions, lists["readPermissions"], "readPermissions differ from usePermissions.ts")
	union := append(slices.Clone(lists["readPermissions"]), lists["writePermissions"]...)
	assert.ElementsMatch(t, AllPermissions(), union, "AllPermissions differ from the union in usePermissions.ts")
}
