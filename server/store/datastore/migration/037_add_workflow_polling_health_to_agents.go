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

var addWorkflowPollingHealthToAgents = xormigrate.Migration{
	ID: "add-workflow-polling-health-to-agents",
	MigrateSession: func(sess *xorm.Session) error {
		type agent struct {
			WorkflowPollingHealthy bool `xorm:"workflow_polling_healthy"`
		}

		if err := sess.Sync(new(agent)); err != nil {
			return err
		}

		// Default false is appropriate - agents will update this on next health report
		return nil
	},
}
