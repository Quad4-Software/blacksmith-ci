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

type secretV040 struct {
	ID     int64  `xorm:"pk autoincr 'id'"`
	Source string `xorm:"varchar(50) DEFAULT 'internal' 'source'"`
}

func (secretV040) TableName() string {
	return "secrets"
}

var addSecretSource = xormigrate.Migration{
	ID: "add-secret-source",
	MigrateSession: func(sess *xorm.Session) (err error) {
		if err := sess.Sync(new(secretV040)); err != nil {
			return err
		}

		// Update existing secrets to have "internal" as default source
		_, err = sess.Exec("UPDATE secrets SET source = 'internal' WHERE source IS NULL OR source = ''")
		return err
	},
}
