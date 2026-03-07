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

var crowAddStaleThreshold = xormigrate.Migration{
	ID: "crow-add-stale-threshold",
	MigrateSession: func(sess *xorm.Session) error {
		type maintenance_config struct { //nolint:revive,stylecheck
			ID             int64 `xorm:"pk autoincr 'id'"`
			StaleThreshold int64 `xorm:"'stale_threshold' default(3600)"`
		}

		return sess.Sync(new(maintenance_config))
	},
}
