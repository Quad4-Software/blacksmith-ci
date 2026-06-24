// Copyright 2021 Woodpecker Authors
// Copyright 2018 Drone.IO Inc.
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
	"errors"
	"regexp"

	"github.com/uptrace/bun"
)

// Validate a username (e.g. from github).
var reUsername = regexp.MustCompile("^[a-zA-Z0-9-_.]+$")

var errUserLoginInvalid = errors.New("invalid user login")

const maxLoginLen = 250

// SystemAdminLogin is the login of the dedicated, forge-less admin user that
// owns the bootstrap access token seeded from CROW_ADMIN_TOKEN.
const SystemAdminLogin = "crow-admin"

// User represents a registered user.
type User struct {
	bun.BaseModel `bun:"table:users"`

	// the id for this user.
	//
	// required: true
	ID int64 `json:"id" bun:"id,pk,autoincrement"`

	// ForgeID is the forge this user originally registered with.
	//
	// Deprecated: Use PrimaryForgeID and UserForge table for multi-forge support.
	ForgeID int64 `json:"forge_id,omitempty" bun:"forge_id"`

	// ForgeRemoteID is the user's ID on their original forge.
	//
	// Deprecated: Use UserForge table for multi-forge support.
	ForgeRemoteID ForgeRemoteID `json:"-" bun:"forge_remote_id"`

	// PrimaryForgeID references the user_forges.id of the user's primary forge connection.
	// The primary forge is the first forge the user registered with and cannot be unlinked.
	PrimaryForgeID int64 `json:"primary_forge_id,omitempty" bun:"primary_forge_id"`

	// Login is the username for this user.
	//
	// required: true
	Login string `json:"login" bun:"login,unique"`

	// AccessToken is the oauth2 access token.
	//
	// Deprecated: Use UserForge table for multi-forge support.
	AccessToken string `json:"-" bun:"access_token,type:text"`

	// RefreshToken is the oauth2 refresh token.
	//
	// Deprecated: Use UserForge table for multi-forge support.
	RefreshToken string `json:"-" bun:"refresh_token,type:text"`

	// Expiry is the AccessToken expiration timestamp (unix seconds).
	//
	// Deprecated: Use UserForge table for multi-forge support.
	Expiry int64 `json:"-" bun:"expiry"`

	// Email is the email address for this user.
	//
	// required: true
	Email string `json:"email" bun:"email,type:varchar(500)"`

	// the avatar url for this user.
	Avatar string `json:"avatar_url" bun:"avatar,type:varchar(500)"`

	// Admin indicates the user is a system administrator.
	//
	// NOTE: If the username is part of the WOODPECKER_ADMIN
	// environment variable, this value will be set to true on login.
	Admin bool `json:"admin,omitempty" bun:"admin"`

	// Hash is a unique token used to sign tokens.
	Hash string `json:"-" bun:"hash,unique,type:varchar(500)"`

	// OrgID is the of the user as model.Org.
	OrgID int64 `json:"org_id" bun:"org_id"`

	// Description is a short bio or description for this user.
	Description string `json:"description,omitempty" bun:"description,type:text"`

	// Timezone is the user's timezone (e.g., "America/New_York", "Europe/Berlin").
	Timezone string `json:"timezone,omitempty" bun:"timezone,type:varchar(100)"`
} //	@name	User

// TableName returns the database table name.
func (User) TableName() string {
	return "users"
}

// Validate validates the required fields and formats.
func (u *User) Validate() error {
	switch {
	case len(u.Login) == 0:
		return errUserLoginInvalid
	case len(u.Login) > maxLoginLen:
		return errUserLoginInvalid
	case !reUsername.MatchString(u.Login):
		return errUserLoginInvalid
	default:
		return nil
	}
}
