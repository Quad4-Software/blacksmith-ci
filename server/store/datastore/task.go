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
	"context"
	"database/sql"
	"errors"
	"time"

	"codefloe.com/crowci/crow/v5/server/model"
)

func (s storage) TaskList() ([]*model.Task, error) {
	tasks := make([]*model.Task, 0, perPage)
	return tasks, s.db.NewSelect().Model(&tasks).Scan(context.Background())
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
	err := s.db.NewSelect().Model(&tasks).
		Join("INNER JOIN workflows ON workflows.id = "+s.castAsInteger("task.id")).
		Where("workflows.state IN (?, ?)", model.StatusPending, model.StatusRunning).
		Scan(context.Background())
	return tasks, err
}

func (s storage) TaskInsert(task *model.Task) error {
	// only Insert set auto created ID back to object
	_, err := s.db.NewInsert().Model(task).Exec(context.Background())
	return err
}

func (s storage) TaskUpdate(task *model.Task) error {
	_, err := s.db.NewUpdate().Model(task).WherePK().Exec(context.Background())
	return err
}

func (s storage) TaskDelete(id string) error {
	return wrapDelete(s.db.NewDelete().Model((*model.Task)(nil)).Where("id = ?", id).Exec(context.Background()))
}

func (s storage) TaskAtomicAssign(taskID string, agentID int64) (bool, error) {
	// Atomically assign task to agent only if it's currently unassigned (agent_id = 0)
	res, err := s.db.NewUpdate().Model((*model.Task)(nil)).
		Set("agent_id = ?", agentID).
		Where("id = ?", taskID).
		Where("agent_id = 0").
		Exec(context.Background())
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	// Return true if exactly one row was affected (successful assignment)
	return affected == 1, nil
}

func (s storage) TaskAtomicClaim(taskID string, agentID int64) (*model.Task, error) {
	// Atomically claim a task for adoption if it was previously assigned to this agent.
	// This is HA-safe because we check that agent_id matches the requesting agent.
	task := new(model.Task)
	err := s.db.NewSelect().Model(task).
		Where("id = ?", taskID).
		Where("agent_id = ?", agentID).
		Scan(context.Background())
	if err != nil {
		if wrapGet(err) != nil {
			return nil, nil // Task doesn't exist or belongs to a different agent
		}
		return nil, err
	}
	return task, nil
}

func (s storage) TaskAtomicResetOrphan(taskID string, expectedAgentID int64) (bool, error) {
	res, err := s.db.NewUpdate().Model((*model.Task)(nil)).
		Set("agent_id = 0").
		Where("id = ?", taskID).
		Where("agent_id = ?", expectedAgentID).
		Exec(context.Background())
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected == 1, nil
}

func (s storage) TaskGetCancelled(id string) (bool, bool, error) {
	task := new(model.Task)
	err := s.db.NewSelect().Model(task).
		Column("canceled").
		Where("id = ?", id).
		Scan(context.Background())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, false, nil // not found
		}
		return false, false, err
	}
	return task.Canceled != 0, true, nil
}

func (s storage) TaskSetCancelled(id string) error {
	_, err := s.db.NewUpdate().Model((*model.Task)(nil)).
		Set("canceled = ?", 1).
		Set("canceled_at = ?", time.Now().Unix()).
		Where("id = ?", id).
		Exec(context.Background())
	return err
}

func (s storage) TaskGetAgentID(taskID string, agentID int64) (bool, error) {
	exists, err := s.db.NewSelect().Model((*model.Task)(nil)).
		Where("id = ?", taskID).
		Where("agent_id = ?", agentID).
		Exists(context.Background())
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s storage) TaskDeleteCancelledBefore(cancelledBefore int64) error {
	_, err := s.db.NewDelete().Model((*model.Task)(nil)).
		Where("canceled = ?", 1).
		Where("canceled_at > 0").
		Where("canceled_at < ?", cancelledBefore).
		Exec(context.Background())
	return err
}

// TaskDeleteStale removes task rows whose corresponding workflows are no longer
// pending or running (i.e. completed, failed, skipped) or whose workflows don't
// exist at all. This handles cleanup after DB connection failures where Done()
// couldn't delete the task row, and ensures a clean state when transitioning
// from single-instance (memory queue) to HA mode (database queue).
func (s storage) TaskDeleteStale() (int64, error) {
	// Delete tasks that have no matching workflow or whose workflow is not active.
	// Uses a subquery to find task IDs with active workflows, then deletes the rest.
	// This approach works across SQLite, PostgreSQL, and MySQL.
	res, err := s.db.NewDelete().Model((*model.Task)(nil)).
		Where(s.castAsInteger("task.id")+" NOT IN (SELECT w.id FROM workflows AS w WHERE w.state IN (?, ?))",
			model.StatusPending, model.StatusRunning).
		Exec(context.Background())
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return affected, nil
}
