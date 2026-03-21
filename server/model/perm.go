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

import "github.com/uptrace/bun"

// Perm defines a repository permission for an individual user.
type Perm struct {
	bun.BaseModel `bun:"table:perms"`

	UserID  int64 `json:"-"       bun:"user_id,notnull,unique:s"`
	RepoID  int64 `json:"-"       bun:"repo_id,notnull,unique:s"`
	Repo    *Repo `json:"-"       bun:"-"`
	Pull    bool  `json:"pull"    bun:"pull"`
	Push    bool  `json:"push"    bun:"push"`
	Admin   bool  `json:"admin"   bun:"admin"`
	Synced  int64 `json:"synced"  bun:"synced"`
	Created int64 `json:"created" bun:",nullzero"`
	Updated int64 `json:"updated" bun:",nullzero"`
} //	@name	Perm

// TableName returns the database table name.
func (Perm) TableName() string {
	return "perms"
}

// OrgPerm defines an organization permission for an individual user.
type OrgPerm struct {
	Member bool `json:"member"`
	Admin  bool `json:"admin"`
} //	@name	OrgPerm
