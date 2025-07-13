// Copyright 2025 Crow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package migration

import (
	"fmt"
	"strings"

	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

var addDependsOnToSteps = xormigrate.Migration{
	ID: "add-depends-on-to-steps",
	MigrateSession: func(sess *xorm.Session) (err error) {
		// Check if the column already exists
		hasColumn, err := sess.IsTableExist("steps")
		if err != nil {
			return fmt.Errorf("check if steps table exists failed: %w", err)
		}
		if !hasColumn {
			return nil // Table doesn't exist, will be created by sync
		}

		// Add the depends_on column
		_, err = sess.Exec("ALTER TABLE steps ADD COLUMN depends_on JSON")
		if err != nil {
			// Column might already exist, which is fine
			// Check if the error is about column already existing
			if strings.Contains(err.Error(), "already exists") ||
				strings.Contains(err.Error(), "duplicate column") ||
				strings.Contains(err.Error(), "column") && strings.Contains(err.Error(), "exists") {
				return nil // Column already exists, which is fine
			}
			return fmt.Errorf("add depends_on column failed: %w", err)
		}

		return nil
	},
}
