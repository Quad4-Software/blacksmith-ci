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

// Different ways to handle failure states.
const (
	FailureIgnore = "ignore"
	FailureFail   = "fail"
	//nolint:godot
	// TODO: Not implemented yet.
	// FailureCancel = "cancel"
)

// Step represents a process in the pipeline.
type Step struct {
	bun.BaseModel `bun:"table:steps"`

	ID             int64           `json:"id"                   bun:"id,pk,autoincrement"`
	UUID           string          `json:"uuid"                 bun:"uuid"`
	PipelineID     int64           `json:"pipeline_id"          bun:"pipeline_id,unique:s"`
	PID            int             `json:"pid"                  bun:"pid,unique:s"`
	PPID           int             `json:"ppid"                 bun:"ppid"`
	Name           string          `json:"name"                 bun:"name"`
	State          StatusValue     `json:"state"                bun:"state"`
	Error          string          `json:"error,omitempty"      bun:"error,type:text"`
	Failure        string          `json:"-"                    bun:"failure"`
	ExitCode       int             `json:"exit_code"            bun:"exit_code"`
	Started        int64           `json:"started,omitempty"    bun:"started"`
	Finished       int64           `json:"finished,omitempty"   bun:"finished"`
	Type           StepType        `json:"type,omitempty"        bun:"type"`
	DependsOn      JSONSliceString `json:"depends_on,omitempty"  bun:"depends_on,type:text"`
	OriginalStepID int64           `json:"original_step_id"      bun:"original_step_id,default:0"`
	Attempt        int             `json:"attempt"               bun:"attempt,default:1"`
} //	@name	Step

// TableName returns the database table name.
func (Step) TableName() string {
	return "steps"
}

// Running returns true if the process state is pending or running.
func (p *Step) Running() bool {
	return p.State == StatusPending || p.State == StatusRunning
}

// Failing returns true if the process state is failed, killed or error.
func (p *Step) Failing() bool {
	return p.Failure == FailureFail && (p.State == StatusError || p.State == StatusKilled || p.State == StatusFailure)
}

// StepType identifies the type of step.
type StepType string //	@name	StepType

const (
	StepTypeClone    StepType = "clone"
	StepTypeService  StepType = "service"
	StepTypePlugin   StepType = "plugin"
	StepTypeCommands StepType = "commands"
	StepTypeCache    StepType = "cache"
)
