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

// TeamMembership represents the caller's membership in a team within an
// organization on a forge. Returned by Forge.TeamMemberships.
type TeamMembership struct {
	// Org is the org/owner login this team belongs to.
	// On GitHub: the org slug. On GitLab: the root group path.
	Org string `json:"org"`

	// Team is the team slug or — on GitLab — the path of the subgroup relative
	// to Org (e.g. "sub/sub2/team" for a 3-deep subgroup under Org).
	Team string `json:"team"`
} //	@name	TeamMembership

// Path returns the canonical "org/team" form used for matching against YAML.
func (m TeamMembership) Path() string { return m.Org + "/" + m.Team }
