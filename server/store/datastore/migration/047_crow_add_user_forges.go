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

package migration

import (
	"fmt"
	"time"

	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

// userForgeV047 is the migration version of UserForge model.
type userForgeV047 struct {
	ID            int64  `xorm:"pk autoincr 'id'"`
	UserID        int64  `xorm:"INDEX NOT NULL 'user_id'"`
	ForgeID       int64  `xorm:"INDEX NOT NULL UNIQUE(forge_user) 'forge_id'"`
	ForgeRemoteID string `xorm:"UNIQUE(forge_user) 'forge_remote_id'"`
	IsPrimary     bool   `xorm:"NOT NULL DEFAULT false 'is_primary'"`
	AccessToken   string `xorm:"TEXT 'access_token'"`
	RefreshToken  string `xorm:"TEXT 'refresh_token'"`
	Expiry        int64  `xorm:"'expiry'"`
	ForgeLogin    string `xorm:"VARCHAR(250) 'forge_login'"`
	ForgeEmail    string `xorm:"VARCHAR(500) 'forge_email'"`
	ForgeAvatar   string `xorm:"VARCHAR(500) 'forge_avatar'"`
	LinkedAt      int64  `xorm:"NOT NULL 'linked_at'"`
	LastUsed      int64  `xorm:"'last_used'"`
}

func (userForgeV047) TableName() string {
	return "user_forges"
}

// userV047 represents the user table structure for reading existing data.
type userV047 struct {
	ID            int64  `xorm:"pk 'id'"`
	ForgeID       int64  `xorm:"'forge_id'"`
	ForgeRemoteID string `xorm:"'forge_remote_id'"`
	Login         string `xorm:"'login'"`
	AccessToken   string `xorm:"'access_token'"`
	RefreshToken  string `xorm:"'refresh_token'"`
	Expiry        int64  `xorm:"'expiry'"`
	Email         string `xorm:"'email'"`
	Avatar        string `xorm:"'avatar'"`
}

func (userV047) TableName() string {
	return "users"
}

var crowAddUserForges = xormigrate.Migration{
	ID: "crow-add-user-forges",
	MigrateSession: func(sess *xorm.Session) error {
		// Step 1: Create the user_forges table
		if err := sess.Sync(new(userForgeV047)); err != nil {
			return err
		}

		// Step 2: Migrate existing user credentials to user_forges
		// Read all existing users with forge credentials
		var users []userV047
		if err := sess.Find(&users); err != nil {
			return err
		}

		now := time.Now().Unix()

		// Track which (forge_id, forge_remote_id) combinations we've already inserted
		// to avoid duplicate key errors (can happen with test/corrupted data)
		seen := make(map[string]bool)

		// Create a user_forge entry for each existing user
		for _, user := range users {
			// Skip users without forge credentials (shouldn't happen, but be safe)
			if user.ForgeID == 0 {
				continue
			}

			// Skip if we've already created a UserForge for this (forge_id, forge_remote_id)
			// This can happen if there's duplicate data in the users table
			forgeRemoteID := user.ForgeRemoteID
			if forgeRemoteID == "" {
				// If no remote ID, use the user's login as a fallback
				// This ensures unique values in the unique constraint
				forgeRemoteID = "legacy:" + user.Login
			}
			fullKey := fmt.Sprintf("%d:%s", user.ForgeID, forgeRemoteID)
			if seen[fullKey] {
				continue
			}
			seen[fullKey] = true

			userForge := &userForgeV047{
				UserID:        user.ID,
				ForgeID:       user.ForgeID,
				ForgeRemoteID: forgeRemoteID,
				IsPrimary:     true, // Existing forge is always the primary
				AccessToken:   user.AccessToken,
				RefreshToken:  user.RefreshToken,
				Expiry:        user.Expiry,
				ForgeLogin:    user.Login,
				ForgeEmail:    user.Email,
				ForgeAvatar:   user.Avatar,
				LinkedAt:      now,
				LastUsed:      now,
			}

			if _, err := sess.Insert(userForge); err != nil {
				return err
			}
		}

		// Step 3: Add primary_forge_id column to users table
		// This column will point to the user_forges.id of the primary forge
		if err := sess.Sync(new(userWithPrimaryForgeV047)); err != nil {
			return err
		}

		// Step 4: Update primary_forge_id for all users
		// Set it to the ID of their primary user_forge entry
		_, err := sess.Exec(`
			UPDATE users SET primary_forge_id = (
				SELECT id FROM user_forges
				WHERE user_forges.user_id = users.id
				AND user_forges.is_primary = true
				LIMIT 1
			)
		`)
		return err
	},
}

// userWithPrimaryForgeV047 adds the primary_forge_id column to users table.
type userWithPrimaryForgeV047 struct {
	ID             int64 `xorm:"pk 'id'"`
	PrimaryForgeID int64 `xorm:"'primary_forge_id'"`
}

func (userWithPrimaryForgeV047) TableName() string {
	return "users"
}
