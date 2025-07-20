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

package migration

import (
	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

type pubsubMessage030 struct {
	ID         string            `xorm:"pk 'id'"`
	Data       []byte            `xorm:"LONGBLOB 'data'"`
	Labels     map[string]string `xorm:"json 'labels'"`
	CreatedAt  int64             `xorm:"'created_at'"`
	InstanceID string            `xorm:"'instance_id'"`
}

func (pubsubMessage030) TableName() string {
	return "pubsub_messages"
}

var addPubSubMessages = xormigrate.Migration{
	ID: "add-pubsub-messages",
	MigrateSession: func(sess *xorm.Session) error {
		return sess.Sync(new(pubsubMessage030))
	},
}
