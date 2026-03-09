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

type notificationConfigV055 struct {
	ID        int64    `xorm:"pk autoincr 'id'"`
	RepoID    int64    `xorm:"NOT NULL UNIQUE INDEX 'repo_id'"`
	IsEnabled bool     `xorm:"DEFAULT true 'is_enabled'"`
	Events    []string `xorm:"json 'events'"`
	CreatedAt int64    `xorm:"created"`
	UpdatedAt int64    `xorm:"updated"`
}

func (notificationConfigV055) TableName() string {
	return "notification_configs"
}

var crowAddNotificationConfigs = xormigrate.Migration{
	ID: "crow-add-notification-configs",
	MigrateSession: func(sess *xorm.Session) error {
		return sess.Sync(new(notificationConfigV055))
	},
}
