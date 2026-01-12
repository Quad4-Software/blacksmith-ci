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

// UserForge represents a user's OAuth credentials for a specific forge.
// A user can have multiple forge connections (e.g., GitHub + GitLab),
// allowing them to manage repositories from different forges under one account.
type UserForge struct {
	// ID is the unique identifier for this user-forge connection.
	ID int64 `json:"id" xorm:"pk autoincr 'id'"`

	// UserID references the user who owns this forge connection.
	UserID int64 `json:"user_id" xorm:"INDEX NOT NULL 'user_id'"`

	// ForgeID references the forge this connection is for.
	ForgeID int64 `json:"forge_id" xorm:"INDEX NOT NULL UNIQUE(forge_user) 'forge_id'"`

	// ForgeRemoteID is the user's unique identifier on the forge (e.g., GitHub user ID).
	// Combined with ForgeID, this must be unique across all users.
	ForgeRemoteID ForgeRemoteID `json:"-" xorm:"UNIQUE(forge_user) 'forge_remote_id'"`

	// IsPrimary indicates if this is the user's primary forge (first registered).
	// Each user has exactly one primary forge which cannot be unlinked.
	IsPrimary bool `json:"is_primary" xorm:"NOT NULL DEFAULT false 'is_primary'"`

	// OAuth tokens for this forge connection
	AccessToken  string `json:"-" xorm:"TEXT 'access_token'"`
	RefreshToken string `json:"-" xorm:"TEXT 'refresh_token'"`
	Expiry       int64  `json:"-" xorm:"'expiry'"`

	// ForgeLogin is the username on this forge (may differ from User.Login).
	ForgeLogin string `json:"forge_login" xorm:"VARCHAR(250) 'forge_login'"`

	// ForgeEmail is the email address on this forge.
	ForgeEmail string `json:"forge_email,omitempty" xorm:"VARCHAR(500) 'forge_email'"`

	// ForgeAvatar is the avatar URL on this forge.
	ForgeAvatar string `json:"forge_avatar,omitempty" xorm:"VARCHAR(500) 'forge_avatar'"`

	// LinkedAt is the unix timestamp when this forge was linked.
	LinkedAt int64 `json:"linked_at" xorm:"NOT NULL 'linked_at'"`

	// LastUsed is the unix timestamp when this forge was last used for an operation.
	LastUsed int64 `json:"last_used,omitempty" xorm:"'last_used'"`
} //	@name	UserForge

// TableName returns the database table name for xorm.
func (UserForge) TableName() string {
	return "user_forges"
}
