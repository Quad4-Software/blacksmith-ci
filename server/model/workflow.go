// Copyright 2021 Woodpecker Authors
// Copyright 2018 Drone.IO Inc.
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

package model

// Workflow represents a workflow in the pipeline.
type Workflow struct {
	ID                     int64             `json:"id"                            xorm:"pk autoincr 'id'"`
	PipelineID             int64             `json:"pipeline_id"                   xorm:"UNIQUE(s) INDEX 'pipeline_id'"`
	PID                    int               `json:"pid"                           xorm:"UNIQUE(s) 'pid'"`
	Name                   string            `json:"name"                          xorm:"name"`
	PipelineName           string            `json:"pipeline_name"                 xorm:"pipeline_name"`
	State                  StatusValue       `json:"state"                         xorm:"state"`
	Error                  string            `json:"error,omitempty"               xorm:"TEXT 'error'"`
	Started                int64             `json:"started,omitempty"             xorm:"started"`
	Finished               int64             `json:"finished,omitempty"            xorm:"finished"`
	AgentID                int64             `json:"agent_id,omitempty"            xorm:"agent_id"`
	AgentPersistentID      string            `json:"agent_persistent_id,omitempty" xorm:"VARCHAR(64) INDEX 'agent_persistent_id'"`
	AgentName              string            `json:"agent_name,omitempty"          xorm:"VARCHAR(255) 'agent_name'"`
	Platform               string            `json:"platform,omitempty"            xorm:"platform"`
	Environ                map[string]string `json:"environ,omitempty"             xorm:"json 'environ'"`
	DependsOn              []string          `json:"depends_on,omitempty"          xorm:"json 'depends_on'"`
	AxisID                 int               `json:"-"                             xorm:"axis_id"`
	OriginalWorkflowID     int64             `json:"original_workflow_id"           xorm:"'original_workflow_id' DEFAULT 0"`
	Attempt                int               `json:"attempt"                       xorm:"'attempt' DEFAULT 1"`
	Children               []*Step           `json:"children,omitempty"                xorm:"-"`
	AutoscalerProvisioning bool              `json:"autoscaler_provisioning,omitempty" xorm:"-"`
}

// TableName return database table name for xorm.
func (Workflow) TableName() string {
	return "workflows"
}

// Running returns true if the process state is pending or running.
func (p *Workflow) Running() bool {
	return p.State == StatusPending || p.State == StatusRunning
}

// Failing returns true if the process state is failed, killed or error.
func (p *Workflow) Failing() bool {
	return p.State == StatusError || p.State == StatusKilled || p.State == StatusFailure
}

// LatestAttempts returns only the latest attempt of each workflow,
// grouping by OriginalWorkflowID (or by the workflow's own ID if it is the original).
func LatestAttempts(workflows []*Workflow) []*Workflow {
	// Map from original workflow ID → latest workflow attempt
	latest := make(map[int64]*Workflow)
	for _, wf := range workflows {
		key := wf.OriginalWorkflowID
		if key == 0 {
			key = wf.ID
		}
		if prev, exists := latest[key]; !exists || wf.Attempt > prev.Attempt {
			latest[key] = wf
		}
	}

	result := make([]*Workflow, 0, len(latest))
	for _, wf := range latest {
		result = append(result, wf)
	}
	return result
}

// IsThereRunningStage determine if it contains workflows running or pending to run.
// Only considers the latest attempt of each workflow.
func IsThereRunningStage(workflows []*Workflow) bool {
	for _, p := range LatestAttempts(workflows) {
		if p.Running() {
			return true
		}
	}
	return false
}

// PipelineStatus determine pipeline status based on corresponding workflow list.
// Only considers the latest attempt of each workflow.
func PipelineStatus(workflows []*Workflow) StatusValue {
	status := StatusSuccess

	for _, p := range LatestAttempts(workflows) {
		if p.State == StatusKilled {
			return StatusKilled
		}
		if p.Failing() {
			status = p.State
		}
	}

	return status
}

// WorkflowStatus determine workflow status based on corresponding step list.
func WorkflowStatus(steps []*Step) StatusValue {
	status := StatusSuccess

	for _, p := range steps {
		if p.State == StatusKilled {
			return StatusKilled
		}
		if p.Failing() {
			status = p.State
		}
	}

	return status
}
