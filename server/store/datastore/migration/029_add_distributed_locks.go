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

type distributedLock029 struct {
	ID         int64  `xorm:"pk autoincr 'id'"`
	Name       string `xorm:"UNIQUE NOT NULL 'name'"`
	InstanceID string `xorm:"NOT NULL 'instance_id'"`
	AcquiredAt int64  `xorm:"'acquired_at'"`
	ExpiresAt  int64  `xorm:"'expires_at'"`
}

func (distributedLock029) TableName() string {
	return "distributed_locks"
}

var addDistributedLocks = xormigrate.Migration{
	ID: "add-distributed-locks",
	MigrateSession: func(sess *xorm.Session) error {
		return sess.Sync(new(distributedLock029))
	},
}
