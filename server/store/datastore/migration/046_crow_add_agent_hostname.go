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

// crowAddAgentHostname adds a hostname column to the agents table.
// This allows pre-flight checks to verify if a workflow's hostname label
// can be matched by an available agent.
var crowAddAgentHostname = xormigrate.Migration{
	ID: "crow-add-agent-hostname",
	MigrateSession: func(sess *xorm.Session) error {
		type agentSync struct {
			Hostname string `xorm:"VARCHAR(255) 'hostname'"`
		}

		return sess.Table("agents").Sync(new(agentSync))
	},
}
