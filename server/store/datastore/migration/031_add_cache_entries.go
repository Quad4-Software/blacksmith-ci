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

type cacheEntry031 struct {
	Key       string `xorm:"pk 'key'"`
	Data      []byte `xorm:"LONGBLOB 'data'"`
	ExpiresAt int64  `xorm:"'expires_at'"`
	CreatedAt int64  `xorm:"'created_at'"`
}

func (cacheEntry031) TableName() string {
	return "cache_entries"
}

var addCacheEntries = xormigrate.Migration{
	ID: "add-cache-entries",
	MigrateSession: func(sess *xorm.Session) error {
		return sess.Sync(new(cacheEntry031))
	},
}
