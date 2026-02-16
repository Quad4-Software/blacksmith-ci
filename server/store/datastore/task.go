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

package datastore

import (
	"codefloe.com/crowci/crow/v5/server/model"
)

func (s storage) TaskList() ([]*model.Task, error) {
	tasks := make([]*model.Task, 0, perPage)
	return tasks, s.engine.Find(&tasks)
}

// TaskListPending returns only tasks whose corresponding workflows are still pending or running.
// This prevents completed tasks from being reloaded into the queue on server restart.
func (s storage) TaskListPending() ([]*model.Task, error) {
	tasks := make([]*model.Task, 0, perPage)
	// Join tasks with workflows to filter by workflow state.
	// Task.ID is a string representation of Workflow.ID, so we cast
	// tasks.id to integer rather than workflows.id to text. This avoids
	// MySQL "Illegal mix of collations" errors that occur when CAST(...
	// AS CHAR) produces a string with the connection default collation
	// while the tasks.id column may use a different collation.
	err := s.engine.
		Join("INNER", "workflows", "workflows.id = "+s.castAsInteger("tasks.id")).
		Where("workflows.state IN (?, ?)", model.StatusPending, model.StatusRunning).
		Find(&tasks)
	return tasks, err
}

func (s storage) TaskInsert(task *model.Task) error {
	// only Insert set auto created ID back to object
	_, err := s.engine.Insert(task)
	return err
}

func (s storage) TaskUpdate(task *model.Task) error {
	_, err := s.engine.ID(task.ID).Update(task)
	return err
}

func (s storage) TaskDelete(id string) error {
	return wrapDelete(s.engine.Where("id = ?", id).Delete(new(model.Task)))
}

func (s storage) TaskAtomicAssign(taskID string, agentID int64) (bool, error) {
	// Atomically assign task to agent only if it's currently unassigned (agent_id = 0)
	affected, err := s.engine.Where("id = ? AND agent_id = 0", taskID).
		Cols("agent_id").
		Update(&model.Task{AgentID: agentID})
	if err != nil {
		return false, err
	}
	// Return true if exactly one row was affected (successful assignment)
	return affected == 1, nil
}

func (s storage) TaskAtomicClaim(taskID string, agentID int64) (*model.Task, error) {
	// Atomically claim a task for adoption if it was previously assigned to this agent.
	// This is HA-safe because we check that agent_id matches the requesting agent.
	task := new(model.Task)
	found, err := s.engine.Where("id = ? AND agent_id = ?", taskID, agentID).Get(task)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil // Task doesn't exist or belongs to a different agent
	}
	return task, nil
}
