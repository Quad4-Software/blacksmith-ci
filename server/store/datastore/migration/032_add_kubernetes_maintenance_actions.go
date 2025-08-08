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
	"time"

	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

type maintenanceConfigK8s struct {
	ID              int64     `xorm:"pk autoincr 'id'"`
	ActionType      string    `xorm:"'action_type' unique"`
	Enabled         bool      `xorm:"'enabled'"`
	Schedule        string    `xorm:"'schedule'"`
	TotalRuns       int64     `xorm:"'total_runs' default(0)"`
	SuccessfulRuns  int64     `xorm:"'successful_runs' default(0)"`
	LastRun         time.Time `xorm:"'last_run'"`
	LastRunSuccess  bool      `xorm:"'last_run_success' default(false)"`
	LastRunDuration int64     `xorm:"'last_run_duration' default(0)"`
	LastRunError    string    `xorm:"'last_run_error'"`
	Created         int64     `xorm:"created"`
	Updated         int64     `xorm:"updated"`
}

func (maintenanceConfigK8s) TableName() string {
	return "maintenance_config"
}

var addKubernetesMaintenanceActions = xormigrate.Migration{
	ID: "add-kubernetes-maintenance-actions",
	MigrateSession: func(sess *xorm.Session) error {
		if err := sess.Sync(new(maintenanceConfigK8s)); err != nil {
			return err
		}

		// Convert existing data: set the first row as "vacuum" if action_type is empty
		_, _ = sess.Exec("UPDATE maintenance_config SET action_type = 'vacuum' WHERE action_type IS NULL OR action_type = ''")

		var count int64
		count, _ = sess.Where("action_type = 'kubernetes_cleanup'").Count(new(maintenanceConfigK8s))
		if count == 0 {
			now := time.Now().Unix()
			newConfig := &maintenanceConfigK8s{
				ActionType: "kubernetes_cleanup",
				Enabled:    false,
				Schedule:   "0 5 * * *",
				Created:    now,
				Updated:    now,
			}
			_, _ = sess.Insert(newConfig)
			// Continue even if this fails - the store layer handles missing action types
		}

		return nil
	},
}
