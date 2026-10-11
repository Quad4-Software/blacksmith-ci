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
	"errors"
	"fmt"
	"slices"
)

// Role is an instance role a user holds besides admin.
type Role string

const (
	RoleOperator       Role = "operator"
	RoleSecretsManager Role = "secrets-manager"
	RoleAuditor        Role = "auditor"
)

// ErrRoleInvalid is returned for a role name Crow does not know.
var ErrRoleInvalid = errors.New("invalid role")

// AllRoles returns every role in display order.
func AllRoles() []Role { return []Role{RoleOperator, RoleSecretsManager, RoleAuditor} }

// IsValid reports whether r is a known role.
func (r Role) IsValid() bool { return slices.Contains(AllRoles(), r) }

// Permission is an instance-level action guarded by a role.
type Permission string

const (
	PermUsersRead        Permission = "users:read"
	PermUsersWrite       Permission = "users:write"
	PermOrgsRead         Permission = "orgs:read"
	PermOrgsDelete       Permission = "orgs:delete"
	PermReposRead        Permission = "repos:read"
	PermReposRepair      Permission = "repos:repair"
	PermQueueRead        Permission = "queue:read"
	PermQueueWrite       Permission = "queue:write"
	PermAgentsRead       Permission = "agents:read"
	PermAgentsRegister   Permission = "agents:register"
	PermAgentsWrite      Permission = "agents:write"
	PermSecretsWrite     Permission = "secrets:write"
	PermMaintenanceRead  Permission = "maintenance:read"
	PermMaintenanceWrite Permission = "maintenance:write"
	PermMetricsAll       Permission = "metrics:all"
	PermServerAdmin      Permission = "server:admin"
)

// AllPermissions returns every permission, sorted.
func AllPermissions() []Permission {
	return []Permission{
		PermAgentsRead, PermAgentsRegister, PermAgentsWrite, PermMaintenanceRead, PermMaintenanceWrite,
		PermMetricsAll, PermOrgsDelete, PermOrgsRead, PermQueueRead, PermQueueWrite,
		PermReposRead, PermReposRepair, PermSecretsWrite, PermServerAdmin,
		PermUsersRead, PermUsersWrite,
	}
}

// readPermissions grant visibility only; every other permission changes state.
var readPermissions = []Permission{
	PermUsersRead, PermOrgsRead, PermReposRead, PermQueueRead,
	PermAgentsRead, PermMaintenanceRead, PermMetricsAll,
}

// rolePermissions is the single source of truth for what each role may do.
// Admins hold every permission and are not listed.
var rolePermissions = map[Role][]Permission{
	RoleOperator: {
		PermQueueRead, PermQueueWrite, PermAgentsRead, PermAgentsWrite,
		PermMaintenanceRead, PermMetricsAll,
	},
	RoleSecretsManager: {PermSecretsWrite},
	RoleAuditor: {
		PermUsersRead, PermOrgsRead, PermReposRead, PermQueueRead,
		PermAgentsRead, PermMaintenanceRead, PermMetricsAll,
	},
}

// ParseRoles validates role names, and returns them deduplicated and sorted.
func ParseRoles(in []string) ([]Role, error) {
	roles := make([]Role, 0, len(in))
	for _, s := range in {
		r := Role(s)
		if !r.IsValid() {
			return nil, fmt.Errorf("%w: %s", ErrRoleInvalid, s)
		}
		if !slices.Contains(roles, r) {
			roles = append(roles, r)
		}
	}
	slices.Sort(roles)
	return roles, nil
}

// RolesToStrings converts roles for storage in User.Roles.
func RolesToStrings(roles []Role) JSONSliceString {
	out := make(JSONSliceString, len(roles))
	for i, r := range roles {
		out[i] = string(r)
	}
	return out
}

// HasRole reports whether the user holds role r. Unknown stored names never match.
func (u *User) HasRole(r Role) bool {
	return u != nil && r.IsValid() && slices.Contains(u.Roles, string(r))
}

// Can reports whether the user holds permission p.
func (u *User) Can(p Permission) bool {
	if u == nil {
		return false
	}
	if u.Admin {
		return true
	}
	for _, r := range AllRoles() {
		if u.HasRole(r) && slices.Contains(rolePermissions[r], p) {
			return true
		}
	}
	return false
}

// Permissions returns the user's effective permissions, sorted.
func (u *User) Permissions() []Permission {
	perms := []Permission{}
	for _, p := range AllPermissions() {
		if u.Can(p) {
			perms = append(perms, p)
		}
	}
	return perms
}

// HasAnyReadPermission reports whether the user may read any admin area.
func (u *User) HasAnyReadPermission() bool {
	return slices.ContainsFunc(readPermissions, u.Can)
}

// HasAnyWritePermission reports whether the user may change any admin area.
func (u *User) HasAnyWritePermission() bool {
	return slices.ContainsFunc(AllPermissions(), func(p Permission) bool {
		return !slices.Contains(readPermissions, p) && u.Can(p)
	})
}

// The known roles are the stored role names Crow recognizes; unknown names are dropped.
func knownRoles(stored []string) []string {
	roles := []string{}
	for _, r := range stored {
		if Role(r).IsValid() {
			roles = append(roles, r)
		}
	}
	return roles
}

// MarshalJSON adds the effective permissions, and always emits roles as an array.
func (u User) MarshalJSON() ([]byte, error) {
	type plain User
	return json.Marshal(struct {
		plain
		Roles       []string     `json:"roles"`
		Permissions []Permission `json:"permissions"`
	}{plain(u), knownRoles(u.Roles), u.Permissions()})
}
