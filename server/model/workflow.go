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

import "github.com/uptrace/bun"

// Workflow represents a workflow in the pipeline.
type Workflow struct {
	bun.BaseModel `bun:"table:workflows"`

	ID                          int64               `json:"id"                            bun:"id,pk,autoincrement"`
	PipelineID                  int64               `json:"pipeline_id"                   bun:"pipeline_id,unique:s"`
	PID                         int                 `json:"pid"                           bun:"pid,unique:s"`
	Name                        string              `json:"name"                          bun:"name"`
	PipelineName                string              `json:"pipeline_name"                 bun:"pipeline_name"`
	State                       StatusValue         `json:"state"                         bun:"state"`
	Error                       string              `json:"error,omitempty"               bun:"error,type:text"`
	Started                     int64               `json:"started,omitempty"             bun:"started"`
	Finished                    int64               `json:"finished,omitempty"            bun:"finished"`
	AgentID                     int64               `json:"agent_id,omitempty"            bun:"agent_id"`
	AgentPersistentID           string              `json:"agent_persistent_id,omitempty" bun:"agent_persistent_id,type:varchar(64)"`
	AgentName                   string              `json:"agent_name,omitempty"          bun:"agent_name,type:varchar(255)"`
	Platform                    string              `json:"platform,omitempty"            bun:"platform"`
	Environ                     JSONMapStringString `json:"environ,omitempty"             bun:"environ,type:text"`
	DependsOn                   JSONSliceString     `json:"depends_on,omitempty"          bun:"depends_on,type:text"`
	RequiresApproval            bool                `json:"requires_approval,omitempty"        bun:"requires_approval,default:false"`
	ApprovalUsers               JSONSliceString     `json:"approval_users,omitempty"           bun:"approval_users,type:text"`
	ApprovalTeams               JSONSliceString     `json:"approval_teams,omitempty"           bun:"approval_teams,type:text"`
	ApprovalState               ApprovalState       `json:"approval_state,omitempty"           bun:"approval_state"`
	ApprovalDeadline            int64               `json:"approval_deadline,omitempty"        bun:"approval_deadline,default:0"`
	ApprovalTimeoutSeconds      int64               `json:"approval_timeout_seconds,omitempty" bun:"approval_timeout_seconds,default:0"`
	AxisID                      int                 `json:"-"                             bun:"axis_id"`
	OriginalWorkflowID          int64               `json:"original_workflow_id"           bun:"original_workflow_id,default:0"`
	Attempt                     int                 `json:"attempt"                       bun:"attempt,default:1"`
	Children                    []*Step             `json:"children,omitempty"                     bun:"-"`
	AutoscalerProvisioning      bool                `json:"autoscaler_provisioning,omitempty"       bun:"-"`
	AutoscalerProvisioningStale bool                `json:"autoscaler_provisioning_stale,omitempty" bun:"-"`
}

// TableName returns the database table name.
func (Workflow) TableName() string {
	return "workflows"
}

// AgentWorkflow is a historic workflow processed by an agent, enriched with the
// owning pipeline's repo and number so the frontend can deep-link to it.
type AgentWorkflow struct {
	Workflow
	RepoID         int64 `json:"repo_id"         bun:"repo_id,scanonly"`
	PipelineNumber int64 `json:"pipeline_number" bun:"pipeline_number,scanonly"`
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
	// Map from original workflow ID -> latest workflow attempt
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

// PipelineStatus determines pipeline status based on the corresponding
// workflow list. Only the latest attempt of each workflow is considered.
//
// Precedence (highest wins):
//  1. StatusKilled — any workflow killed by the user.
//  2. StatusDeclined — any workflow declined or expired at an approval gate.
//  3. StatusFailure / StatusError — any workflow failed (terminal outcome).
//  4. StatusBlocked — any workflow still waiting for an approval gate. This
//     is more actionable than "running" and matches what the pipeline-level
//     approval gate uses, so the UI shows the same icon and affordances.
//  5. StatusRunning — any workflow still pending or running.
//  6. StatusSuccess — every workflow reached a non-failing terminal state.
//
// Failure overrides blocked / running because it is terminal: the pipeline is
// doomed regardless of any other gates or pending work. Blocked overrides
// running because a blocked gate requires human action, which is more useful
// to surface to the user than a generic "wait".
func PipelineStatus(workflows []*Workflow) StatusValue {
	status := StatusSuccess
	declinedSeen := false
	blockedSeen := false
	runningSeen := false

	for _, p := range LatestAttempts(workflows) {
		if p.State == StatusKilled {
			return StatusKilled
		}
		if p.State == StatusDeclined {
			declinedSeen = true
			continue
		}
		if p.Failing() {
			status = p.State
			continue
		}
		if p.State == StatusBlocked {
			blockedSeen = true
			continue
		}
		if p.State == StatusPending || p.State == StatusRunning {
			runningSeen = true
			continue
		}
	}

	if declinedSeen {
		return StatusDeclined
	}
	// A failure (status != StatusSuccess at this point means a Failing() hit)
	// outranks blocked / running because it is terminal.
	if status != StatusSuccess {
		return status
	}
	if blockedSeen {
		return StatusBlocked
	}
	if runningSeen {
		return StatusRunning
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
