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
	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

type integrationV045 struct {
	ID             int64   `xorm:"pk autoincr 'id'"`
	UserID         int64   `xorm:"NOT NULL INDEX 'user_id'"`
	Name           string  `xorm:"NOT NULL VARCHAR(255) 'name'"`
	Type           string  `xorm:"NOT NULL VARCHAR(50) 'type'"`
	Description    string  `xorm:"TEXT 'description'"`
	Config         string  `xorm:"TEXT 'config'"`
	AllowedUserIDs []int64 `xorm:"json 'allowed_user_ids'"`
	AllowedOrgIDs  []int64 `xorm:"json 'allowed_org_ids'"`
	AllowedRepoIDs []int64 `xorm:"json 'allowed_repo_ids'"`
	IsActive       bool    `xorm:"DEFAULT true 'is_active'"`
	CreatedAt      int64   `xorm:"'created_at' created"`
	UpdatedAt      int64   `xorm:"'updated_at' updated"`
}

func (integrationV045) TableName() string {
	return "integrations"
}

var crowAddIntegrations = xormigrate.Migration{
	ID: "crow-add-integrations",
	MigrateSession: func(sess *xorm.Session) error {
		return sess.Sync(new(integrationV045))
	},
}
