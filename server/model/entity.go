// Copyright 2025 Crow CI Authors
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

// EntityType represents the type of entity (user or organization).
type EntityType string

const (
	EntityTypeUser EntityType = "user"
	EntityTypeOrg  EntityType = "org"
)

// Entity represents a unified view of users and organizations for admin management.
type Entity struct {
	ID         int64      `json:"id"`
	EntityType EntityType `json:"entity_type"`
	Name       string     `json:"name"`
	ForgeID    int64      `json:"forge_id,omitempty"`

	// User-specific fields (nil for orgs)
	Email       *string `json:"email,omitempty"`
	Avatar      *string `json:"avatar,omitempty"`
	Admin       *bool   `json:"admin,omitempty"`
	Description *string `json:"description,omitempty"`
	Timezone    *string `json:"timezone,omitempty"`
	// Instance roles besides admin, so the admin user list can show and edit them.
	Roles []string `json:"roles,omitempty"`

	// Org-specific fields (nil for users)
	IsUser  *bool `json:"is_user,omitempty"`
	Private *bool `json:"private,omitempty"`
} //	@name	Entity

// UserToEntity converts a User model to an Entity.
func UserToEntity(user *User) *Entity {
	return &Entity{
		ID:          user.ID,
		EntityType:  EntityTypeUser,
		Name:        user.Login,
		ForgeID:     user.ForgeID,
		Email:       &user.Email,
		Avatar:      &user.Avatar,
		Admin:       &user.Admin,
		Description: &user.Description,
		Timezone:    &user.Timezone,
		Roles:       knownRoles(user.Roles),
	}
}

// OrgToEntity converts an Org model to an Entity.
func OrgToEntity(org *Org) *Entity {
	return &Entity{
		ID:         org.ID,
		EntityType: EntityTypeOrg,
		Name:       org.Name,
		ForgeID:    org.ForgeID,
		IsUser:     &org.IsUser,
		Private:    &org.Private,
	}
}
