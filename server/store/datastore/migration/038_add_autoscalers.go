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

type autoscalerV038 struct {
	ID                int64             `xorm:"pk autoincr 'id'"`
	Created           int64             `xorm:"created"`
	Updated           int64             `xorm:"updated"`
	Name              string            `xorm:"UNIQUE NOT NULL 'name'"`
	Token             string            `xorm:"token"`
	OwnerID           int64             `xorm:"'owner_id'"`
	LastContact       int64             `xorm:"last_contact"`
	MinAgents         int32             `xorm:"'min_agents'"`
	MaxAgents         int32             `xorm:"'max_agents'"`
	WorkflowsPerAgent int32             `xorm:"'workflows_per_agent'"`
	Provider          string            `xorm:"VARCHAR(50) 'provider'"`
	InstanceType      string            `xorm:"VARCHAR(100) 'instance_type'"`
	Region            string            `xorm:"VARCHAR(100) 'region'"`
	AgentLabels       map[string]string `xorm:"JSON 'agent_labels'"`
	Enabled           bool              `xorm:"DEFAULT true 'enabled'"`
	Version           string            `xorm:"'version'"`
	ActiveAgents      int32             `xorm:"'active_agents'"`
	PendingAgents     int32             `xorm:"'pending_agents'"`
	OrgID             int64             `xorm:"INDEX 'org_id'"`
}

func (autoscalerV038) TableName() string {
	return "autoscalers"
}

var addAutoscalers = xormigrate.Migration{
	ID: "add-autoscalers",
	MigrateSession: func(sess *xorm.Session) error {
		return sess.Sync(new(autoscalerV038))
	},
}
