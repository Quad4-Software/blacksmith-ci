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

// Package groupmap maps identity provider groups to sign-in access, admin status and instance roles.
package groupmap

import "slices"

// Mapping decides sign-in access, admin status and instance roles from a
// user's identity provider groups.
type Mapping struct {
	AllowedGroups []string
	AdminGroups   []string
	// RoleGroups maps an instance role name to the groups that grant it.
	RoleGroups map[string][]string
	// Match reports whether a configured group matches one of the user's
	// groups. nil means exact comparison.
	Match func(configured, actual string) bool
}

// IsAllowed reports whether a user with these groups may sign in.
func (m Mapping) IsAllowed(groups []string) bool {
	return !m.RestrictsGroups() || m.intersects(m.AllowedGroups, groups)
}

// RestrictsGroups reports whether sign-in is limited to the allowed groups.
func (m Mapping) RestrictsGroups() bool { return len(m.AllowedGroups) > 0 }

// ManagesAdmin reports whether admin status is derived from the admin groups.
func (m Mapping) ManagesAdmin() bool { return len(m.AdminGroups) > 0 }

// IsAdmin reports whether these groups grant instance admin.
func (m Mapping) IsAdmin(groups []string) bool { return m.intersects(m.AdminGroups, groups) }

// ManagesGroups reports whether any check needs the user's groups.
func (m Mapping) ManagesGroups() bool {
	return m.RestrictsGroups() || m.ManagesAdmin() || len(m.ManagedRoles()) > 0
}

// ManagedRoles returns the roles that have at least one mapped group, sorted.
func (m Mapping) ManagedRoles() []string {
	roles := []string{}
	for role, groups := range m.RoleGroups {
		if len(groups) > 0 {
			roles = append(roles, role)
		}
	}
	slices.Sort(roles)
	return roles
}

// RolesFor returns the managed roles these groups grant, sorted.
func (m Mapping) RolesFor(groups []string) []string {
	roles := []string{}
	for _, role := range m.ManagedRoles() {
		if m.intersects(m.RoleGroups[role], groups) {
			roles = append(roles, role)
		}
	}
	return roles
}

func (m Mapping) intersects(want, have []string) bool {
	match := m.Match
	if match == nil {
		match = func(configured, actual string) bool { return configured == actual }
	}
	return slices.ContainsFunc(want, func(w string) bool {
		return slices.ContainsFunc(have, func(h string) bool { return match(w, h) })
	})
}
