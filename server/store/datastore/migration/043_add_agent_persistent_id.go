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
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

// addAgentPersistentID adds persistent identity tracking for agents.
// This allows metrics to be aggregated across agent ID changes (e.g., when
// agents using master token reconnect and get new IDs after cleanup).
var addAgentPersistentID = xormigrate.Migration{
	ID: "crow-add-agent-persistent-id",
	MigrateSession: func(sess *xorm.Session) error {
		// Struct for syncing new column only (without pk autoincr to avoid SQLite issues)
		type agentSync struct {
			PersistentID string `xorm:"VARCHAR(64) INDEX 'persistent_id'"`
		}

		// Rename table for xorm sync
		if err := sess.Table("agents").Sync(new(agentSync)); err != nil {
			return err
		}

		// Add AgentPersistentID and AgentName to workflows table
		type workflowSync struct {
			AgentPersistentID string `xorm:"VARCHAR(64) INDEX 'agent_persistent_id'"`
			AgentName         string `xorm:"VARCHAR(255) 'agent_name'"`
		}

		if err := sess.Table("workflows").Sync(new(workflowSync)); err != nil {
			return err
		}

		// Struct for querying existing agents (includes existing columns for reading)
		type agentQuery struct {
			ID           int64  `xorm:"pk 'id'"`
			Token        string `xorm:"'token'"`
			Name         string `xorm:"'name'"`
			Platform     string `xorm:"'platform'"`
			Backend      string `xorm:"'backend'"`
			PersistentID string `xorm:"'persistent_id'"`
		}

		// Backfill PersistentID for existing agents
		var agents []agentQuery
		if err := sess.Table("agents").Find(&agents); err != nil {
			return err
		}

		for _, a := range agents {
			if a.PersistentID == "" {
				// Generate persistent ID from token + name + platform + backend
				data := fmt.Sprintf("%s|%s|%s|%s", a.Token, a.Name, a.Platform, a.Backend)
				hash := sha256.Sum256([]byte(data))
				persistentID := hex.EncodeToString(hash[:])

				if _, err := sess.Table("agents").ID(a.ID).Cols("persistent_id").Update(&agentQuery{PersistentID: persistentID}); err != nil {
					return err
				}
			}
		}

		return nil
	},
}
