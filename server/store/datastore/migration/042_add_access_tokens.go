// Copyright 2025 Crow Authors
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
	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

type accessTokenV042 struct {
	ID        int64    `xorm:"pk autoincr 'id'"`
	UserID    int64    `xorm:"NOT NULL INDEX 'user_id'"`
	OrgID     int64    `xorm:"NOT NULL DEFAULT 0 INDEX 'org_id'"`
	RepoID    int64    `xorm:"NOT NULL DEFAULT 0 INDEX 'repo_id'"`
	Name      string   `xorm:"NOT NULL 'name'"`
	TokenHash string   `xorm:"UNIQUE TEXT 'token_hash'"`
	Scopes    []string `xorm:"json 'scopes'"`
	ExpiresAt int64    `xorm:"'expires_at'"`
	LastUsed  int64    `xorm:"'last_used'"`
	CreatedAt int64    `xorm:"'created_at' created"`
	UpdatedAt int64    `xorm:"'updated_at' updated"`
}

func (accessTokenV042) TableName() string {
	return "access_tokens"
}

var addAccessTokens = xormigrate.Migration{
	ID: "add-access-tokens",
	MigrateSession: func(sess *xorm.Session) error {
		return sess.Sync(new(accessTokenV042))
	},
}
