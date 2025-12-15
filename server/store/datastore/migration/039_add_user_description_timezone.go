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

type userV039 struct {
	ID          int64  `xorm:"pk autoincr 'id'"`
	Description string `xorm:"TEXT 'description'"`
	Timezone    string `xorm:"varchar(100) 'timezone'"`
}

func (userV039) TableName() string {
	return "users"
}

var addUserDescriptionTimezone = xormigrate.Migration{
	ID: "add-user-description-timezone",
	MigrateSession: func(sess *xorm.Session) error {
		return sess.Sync(new(userV039))
	},
}
