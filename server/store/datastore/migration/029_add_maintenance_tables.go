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

	"codeberg.org/crowci/crow/v3/server/model"
)

var addMaintenanceTables = xormigrate.Migration{
	ID: "add-maintenance-tables",
	MigrateSession: func(sess *xorm.Session) error {
		// Create maintenance_config table
		if err := sess.Sync(new(model.MaintenanceConfig)); err != nil {
			return err
		}

		// Create maintenance_stats table
		if err := sess.Sync(new(model.MaintenanceStats)); err != nil {
			return err
		}

		// Create maintenance_logs table
		if err := sess.Sync(new(model.MaintenanceLog)); err != nil {
			return err
		}

		// Insert default maintenance configuration
		defaultConfig := &model.MaintenanceConfig{
			Enabled:  true,
			Schedule: "0 5 * * *", // 5 AM daily
		}
		_, err := sess.Insert(defaultConfig)
		return err
	},
}
