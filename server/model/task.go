// Copyright 2021 Woodpecker Authors
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

import (
	"fmt"

	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v5/pipeline"
)

// Task defines scheduled pipeline Task.
type Task struct {
	bun.BaseModel `bun:"table:tasks"`

	ID           string                   `json:"id"           bun:"id,pk"`
	PID          int                      `json:"pid"          bun:"pid"`
	Name         string                   `json:"name"         bun:"name"`
	Data         []byte                   `json:"-"            bun:"data"`
	Labels       JSONMapStringString      `json:"labels"       bun:"labels,type:text"`
	Dependencies JSONSliceString          `json:"dependencies" bun:"dependencies,type:text"`
	RunOn        JSONSliceString          `json:"run_on"       bun:"run_on,type:text"`
	DepStatus    JSONMapStringStatusValue `json:"dep_status"   bun:"dependencies_status,type:text"`
	AgentID      int64                    `json:"agent_id"     bun:"agent_id"`
	PipelineID   int64                    `json:"pipeline_id"  bun:"pipeline_id"`
	RepoID       int64                    `json:"repo_id"      bun:"repo_id"`
	Canceled     int64                    `json:"canceled"    bun:"canceled"`
	CancelledAt  int64                    `json:"canceled_at" bun:"canceled_at"`
} //	@name	Task

// TableName returns the database table name.
func (Task) TableName() string {
	return "tasks"
}

func (t *Task) String() string {
	return fmt.Sprintf("%s (%s) - %s", t.ID, t.Dependencies, t.DepStatus)
}

func (t *Task) ApplyLabelsFromRepo(r *Repo) error {
	if r == nil {
		return fmt.Errorf("repo is nil but needed to get task labels")
	}
	if t.Labels == nil {
		t.Labels = make(JSONMapStringString)
	}
	t.Labels[pipeline.LabelRepoFullName] = r.FullName
	t.Labels[pipeline.LabelOrgID] = fmt.Sprintf("%d", r.OrgID)
	return nil
}

// ShouldRun tells if a task should be run or skipped, based on dependencies.
func (t *Task) ShouldRun() bool {
	// First, ensure all dependencies have completed (have a status recorded)
	for _, dep := range t.Dependencies {
		if _, hasStatus := t.DepStatus[dep]; !hasStatus {
			return false // Dependency hasn't completed yet
		}
	}

	if t.runsOnFailure() && t.runsOnSuccess() {
		return true
	}

	if !t.runsOnFailure() && t.runsOnSuccess() {
		for _, status := range t.DepStatus {
			if status != StatusSuccess {
				return false
			}
		}
		return true
	}

	if t.runsOnFailure() && !t.runsOnSuccess() {
		for _, status := range t.DepStatus {
			if status == StatusSuccess {
				return false
			}
		}
		return true
	}

	return false
}

func (t *Task) runsOnFailure() bool {
	for _, status := range t.RunOn {
		if status == string(StatusFailure) {
			return true
		}
	}
	return false
}

func (t *Task) runsOnSuccess() bool {
	if len(t.RunOn) == 0 {
		return true
	}

	for _, status := range t.RunOn {
		if status == string(StatusSuccess) {
			return true
		}
	}
	return false
}
