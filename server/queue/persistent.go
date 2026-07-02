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

package queue

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store"
)

// WithTaskStore returns a queue that is backed by the TaskStore. This
// ensures the task Queue can be restored when the system starts.
func WithTaskStore(ctx context.Context, q Queue, s store.Store) Queue {
	// Only load tasks for workflows that are still pending or running
	// This prevents completed tasks from being reprocessed on server restart
	tasks, err := s.TaskListPending()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load pending tasks from database")
	} else if len(tasks) > 0 {
		log.Info().Int("count", len(tasks)).Msg("Loading pending tasks from database on startup")
		if err := q.PushAtOnce(ctx, tasks); err != nil {
			log.Error().Err(err).Msg("PushAtOnce failed")
		}
	}
	return &persistentQueue{q, s}
}

type persistentQueue struct {
	Queue
	store store.Store
}

// Push pushes a task to the tail of this queue.
func (q *persistentQueue) Push(c context.Context, task *model.Task) error {
	if err := q.store.TaskInsert(task); err != nil {
		return err
	}
	err := q.Queue.Push(c, task)
	if err != nil {
		if err2 := q.store.TaskDelete(task.ID); err2 != nil {
			err = fmt.Errorf("delete task '%s' failed: %w: %w", task.ID, err2, err)
		}
	}
	return err
}

// PushAtOnce pushes multiple tasks to the tail of this queue.
//
// The store exposes no transaction/session primitive that spans multiple
// TaskInsert calls, so partial failures are handled with a compensating
// delete: if any insert fails midway, the rows already inserted in this call
// are removed again. Otherwise those orphaned rows would be re-pushed on the
// next restart via WithTaskStore, duplicating tasks.
func (q *persistentQueue) PushAtOnce(c context.Context, tasks []*model.Task) error {
	inserted := make([]*model.Task, 0, len(tasks))
	for _, task := range tasks {
		if err := q.store.TaskInsert(task); err != nil {
			// Roll back the rows already inserted in this batch.
			q.deleteTasks(inserted)
			return err
		}
		inserted = append(inserted, task)
	}
	if err := q.Queue.PushAtOnce(c, tasks); err != nil {
		// The in-memory push failed, so drop the persisted rows again to keep
		// the store consistent with the queue.
		q.deleteTasks(tasks)
		return err
	}
	return nil
}

// deleteTasks best-effort deletes the given tasks from the store, continuing
// past individual failures so a single missing row does not leave the rest
// orphaned.
func (q *persistentQueue) deleteTasks(tasks []*model.Task) {
	for _, task := range tasks {
		if err := q.store.TaskDelete(task.ID); err != nil {
			log.Error().Err(err).Str("taskID", task.ID).Msg("queue: push_at_once: failed to clean up task after error")
		}
	}
}

// Poll retrieves and removes a task head of this queue.
func (q *persistentQueue) Poll(c context.Context, agentID int64, f FilterFn) (*model.Task, error) {
	task, err := q.Queue.Poll(c, agentID, f)
	if task != nil {
		// Update task in database with AgentID instead of deleting it
		// This allows the task to be recovered if the server restarts while it's running
		// The AgentID should already be set by the underlying queue
		log.Debug().Str("taskID", task.ID).Int64("agentID", task.AgentID).Msg("Updating task in database with agent assignment")
		if updateErr := q.store.TaskUpdate(task); updateErr != nil {
			log.Error().Err(updateErr).Str("taskID", task.ID).Int64("agentID", task.AgentID).
				Msg("Failed to update task in database with agent assignment")
		}
	}
	return task, err
}

// Evict removes a pending task from the queue.
func (q *persistentQueue) Evict(c context.Context, id string) error {
	err := q.Queue.Evict(c, id)
	if err == nil {
		if err := q.store.TaskDelete(id); err != nil {
			// Log the error but don't fail the operation
			// This prevents missing tasks from failing the eviction
			log.Trace().Err(err).Msgf("queue: evict: failed to delete task %s from database - task may have been already deleted", id)
		}
	}
	return err
}

// EvictAtOnce removes multiple pending tasks from the queue.
func (q *persistentQueue) EvictAtOnce(c context.Context, ids []string) error {
	if err := q.Queue.EvictAtOnce(c, ids); err != nil {
		return err
	}
	for _, id := range ids {
		if err := q.store.TaskDelete(id); err != nil {
			// Log the error but continue processing other tasks
			// This prevents one missing task from failing the entire operation
			log.Trace().Err(err).Msgf("queue: evict_at_once: failed to delete task %s from database - task may have been already deleted", id)
		}
	}
	return nil
}

// Error signals the task is done with an error.
func (q *persistentQueue) Error(c context.Context, id string, err error) error {
	if err := q.Queue.Error(c, id, err); err != nil {
		return err
	}
	if err := q.store.TaskDelete(id); err != nil {
		// Log the error but don't fail the operation
		// This prevents missing tasks from failing the error operation
		log.Trace().Err(err).Msgf("queue: error: failed to delete task %s from database - task may have been already deleted", id)
	}
	return nil
}

// ErrorAtOnce signals multiple tasks are done with an error.
func (q *persistentQueue) ErrorAtOnce(c context.Context, ids []string, err error) error {
	if err := q.Queue.ErrorAtOnce(c, ids, err); err != nil {
		return err
	}
	for _, id := range ids {
		if err := q.store.TaskDelete(id); err != nil {
			// Log the error but continue processing other tasks
			// This prevents one missing task from failing the entire operation
			log.Trace().Err(err).Msgf("queue: error_at_once: failed to delete task %s from database - task may have been already deleted", id)
		}
	}
	return nil
}

// Extend extends the deadline for a task with better error handling for race conditions.
func (q *persistentQueue) Extend(c context.Context, agentID int64, taskID string) error {
	err := q.Queue.Extend(c, agentID, taskID)
	if errors.Is(err, ErrNotFound) {
		// Task not found in underlying queue - it was likely already completed and removed.
		// This is a common race condition where the agent tries to extend a task
		// that was just completed. Return success to avoid unnecessary errors.
		log.Debug().Str("taskID", taskID).Int64("agentID", agentID).Msg("Task not found for extend - likely already completed")
		return nil
	}
	return err
}

// Reclaim re-adds a task to the running state after agent restart.
func (q *persistentQueue) Reclaim(c context.Context, task *model.Task) error {
	return q.Queue.Reclaim(c, task)
}
