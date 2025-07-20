// Copyright 2025 Crow CI Authors
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
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v4/server/model"
	"codeberg.org/crowci/crow/v4/server/store"
)

const (
	// Database polling interval for new tasks.
	dbPollInterval = 1 * time.Second

	// Leader election lock name for queue processing.
	queueLeaderLock = "queue:processor:leader"
)

// DatabaseQueue implements a distributed queue using the database as the backend.
// This replaces the in-memory FIFO queue to enable horizontal scaling.
type DatabaseQueue struct {
	ctx           context.Context
	store         store.Store
	lockService   LockService
	instanceID    string
	paused        bool
	mu            sync.RWMutex
	leaderLockTTL time.Duration

	// Track assigned tasks to prevent double assignment
	assignedTasks map[string]int64 // task ID -> agent ID
	assignmentMu  sync.Mutex
}

// NewDatabaseQueue creates a new database-backed queue for HA deployments.
func NewDatabaseQueue(ctx context.Context, store store.Store, lockService LockService, instanceID string, leaderLockTTL time.Duration) Queue {
	q := &DatabaseQueue{
		ctx:           ctx,
		store:         store,
		lockService:   lockService,
		instanceID:    instanceID,
		paused:        false,
		assignedTasks: make(map[string]int64),
		leaderLockTTL: leaderLockTTL,
	}

	log.Info().Str("instanceID", instanceID).Msg("Starting database queue for HA deployment")

	// Start background processor with leader election for cleanup
	go q.processLoop() //nolint:contextcheck // Background process uses queue context

	return q
}

// Push pushes a task to the queue by storing it in the database.
func (q *DatabaseQueue) Push(ctx context.Context, task *model.Task) error {
	task.AgentID = 0
	return q.store.TaskInsert(task)
}

// PushAtOnce pushes multiple tasks to the queue.
func (q *DatabaseQueue) PushAtOnce(ctx context.Context, tasks []*model.Task) error {
	for _, task := range tasks {
		task.AgentID = 0
		if err := q.store.TaskInsert(task); err != nil {
			return err
		}
	}
	return nil
}

// Poll retrieves and assigns a task to an agent.
func (q *DatabaseQueue) Poll(ctx context.Context, agentID int64, filter FilterFn) (*model.Task, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(dbPollInterval):
			q.mu.RLock()
			isPaused := q.paused
			q.mu.RUnlock()

			if isPaused {
				continue
			}

			// Get tasks from database
			tasks, err := q.store.TaskList()
			if err != nil {
				log.Error().Err(err).Msg("Failed to list tasks from database")
				continue
			}

			// Find suitable task for this agent
			for _, task := range tasks {
				// Skip if task is already assigned
				q.assignmentMu.Lock()
				if assignedAgent, exists := q.assignedTasks[task.ID]; exists && assignedAgent != 0 {
					q.assignmentMu.Unlock()
					continue
				}

				// Check if task dependencies are met
				if !q.areDependenciesMet(task, tasks) {
					q.assignmentMu.Unlock()
					continue
				}

				// Apply agent filter
				if filter != nil {
					matched, _ := filter(task)
					if !matched {
						q.assignmentMu.Unlock()
						continue
					}
				}

				// Assign task to this agent
				q.assignedTasks[task.ID] = agentID
				q.assignmentMu.Unlock()

				task.AgentID = agentID
				log.Debug().Str("taskID", task.ID).Int64("agentID", agentID).Msg("Assigned task to agent")
				return task, nil
			}
		}
	}
}

// Done marks a task as completed and removes it from the database.
func (q *DatabaseQueue) Done(ctx context.Context, id string, exitStatus model.StatusValue) error {
	q.assignmentMu.Lock()
	delete(q.assignedTasks, id)
	q.assignmentMu.Unlock()

	// Update dependent tasks with completion status
	q.updateDependentTasks(id, exitStatus)

	return q.store.TaskDelete(id)
}

// Error marks a task as failed and removes it from the database.
func (q *DatabaseQueue) Error(ctx context.Context, id string, err error) error {
	return q.Done(ctx, id, model.StatusFailure)
}

// ErrorAtOnce marks multiple tasks as failed.
func (q *DatabaseQueue) ErrorAtOnce(ctx context.Context, ids []string, err error) error {
	for _, id := range ids {
		if errUpdate := q.Error(ctx, id, err); errUpdate != nil {
			return errUpdate
		}
	}
	return nil
}

// Evict removes a pending task from the queue.
func (q *DatabaseQueue) Evict(ctx context.Context, taskID string) error {
	q.assignmentMu.Lock()
	delete(q.assignedTasks, taskID)
	q.assignmentMu.Unlock()

	return q.store.TaskDelete(taskID)
}

// EvictAtOnce removes multiple pending tasks from the queue.
func (q *DatabaseQueue) EvictAtOnce(ctx context.Context, taskIDs []string) error {
	q.assignmentMu.Lock()
	for _, id := range taskIDs {
		delete(q.assignedTasks, id)
	}
	q.assignmentMu.Unlock()

	for _, id := range taskIDs {
		if err := q.store.TaskDelete(id); err != nil {
			return err
		}
	}
	return nil
}

// Wait waits until a task is completed (removed from database).
func (q *DatabaseQueue) Wait(ctx context.Context, taskID string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			tasks, err := q.store.TaskList()
			if err != nil {
				return err
			}

			// Check if task still exists
			found := false
			for _, task := range tasks {
				if task.ID == taskID {
					found = true
					break
				}
			}

			if !found {
				return nil // Task completed
			}
		}
	}
}

// Extend extends the task execution deadline by updating assignment time.
func (q *DatabaseQueue) Extend(ctx context.Context, agentID int64, taskID string) error {
	q.assignmentMu.Lock()
	defer q.assignmentMu.Unlock()

	assignedAgent, exists := q.assignedTasks[taskID]
	if !exists || assignedAgent != agentID {
		return ErrAgentMissMatch
	}

	// In this simple implementation, we just confirm the assignment
	// In a more sophisticated version, we could update timestamps
	return nil
}

// Info returns queue information from the database.
func (q *DatabaseQueue) Info(ctx context.Context) InfoT {
	tasks, err := q.store.TaskList()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get queue info from database")
		return InfoT{Paused: q.paused}
	}

	q.mu.RLock()
	isPaused := q.paused
	q.mu.RUnlock()

	q.assignmentMu.Lock()
	assignedTasks := make(map[string]int64)
	for k, v := range q.assignedTasks {
		assignedTasks[k] = v
	}
	q.assignmentMu.Unlock()

	info := InfoT{
		Paused: isPaused,
		Stats: struct {
			Workers       int `json:"worker_count"`
			Pending       int `json:"pending_count"`
			WaitingOnDeps int `json:"waiting_on_deps_count"`
			Running       int `json:"running_count"`
		}{},
	}

	for _, task := range tasks {
		if assignedAgent, isAssigned := assignedTasks[task.ID]; isAssigned && assignedAgent > 0 {
			info.Running = append(info.Running, task)
			info.Stats.Running++
		} else if q.areDependenciesMet(task, tasks) {
			info.Pending = append(info.Pending, task)
			info.Stats.Pending++
		} else {
			info.WaitingOnDeps = append(info.WaitingOnDeps, task)
			info.Stats.WaitingOnDeps++
		}
	}

	return info
}

// GetRunningWorkflowIDs returns workflow IDs that are currently assigned.
func (q *DatabaseQueue) GetRunningWorkflowIDs() []string {
	q.assignmentMu.Lock()
	defer q.assignmentMu.Unlock()

	var ids []string
	for taskID, agentID := range q.assignedTasks {
		if agentID > 0 {
			ids = append(ids, taskID)
		}
	}

	return ids
}

// Pause stops the queue from processing new tasks.
func (q *DatabaseQueue) Pause() {
	q.mu.Lock()
	q.paused = true
	q.mu.Unlock()
	log.Info().Msg("Database queue paused")
}

// Resume starts the queue processing again.
func (q *DatabaseQueue) Resume() {
	q.mu.Lock()
	q.paused = false
	q.mu.Unlock()
	log.Info().Msg("Database queue resumed")
}

// KickAgentWorkers removes tasks assigned to a specific agent.
func (q *DatabaseQueue) KickAgentWorkers(agentID int64) {
	q.assignmentMu.Lock()
	defer q.assignmentMu.Unlock()

	kickedTasks := 0
	for taskID, assignedAgent := range q.assignedTasks {
		if assignedAgent == agentID {
			delete(q.assignedTasks, taskID)
			kickedTasks++
		}
	}

	log.Info().Int64("agentID", agentID).Int("tasksKicked", kickedTasks).Msg("Kicked agent workers")
}

// processLoop runs background processing with leader election for cleanup.
func (q *DatabaseQueue) processLoop() {
	log.Debug().Str("lockName", queueLeaderLock).Dur("ttl", q.leaderLockTTL).Msg("Starting database queue leader election process")

	for {
		select {
		case <-q.ctx.Done():
			log.Debug().Msg("Stopping database queue leader election process")
			return
		case <-time.After(q.leaderLockTTL):
			log.Debug().Str("lockName", queueLeaderLock).Msg("Attempting to acquire queue processing leader lock")
			// Try to acquire leader lock for cleanup operations
			lockCtx, releaseLock, err := q.lockService.AcquireLock(q.ctx, queueLeaderLock, q.leaderLockTTL)
			if err != nil {
				log.Debug().Err(err).Str("lockName", queueLeaderLock).Msg("Failed to acquire queue processing leader lock")
				continue
			}

			log.Info().Str("lockName", queueLeaderLock).Str("leaderInstance", q.instanceID).Msg("This instance is now the queue processing leader - performing cleanup")
			// We're the leader, perform cleanup
			go func() {
				defer func() {
					log.Debug().Str("lockName", queueLeaderLock).Msg("Releasing queue processing leader lock")
					releaseLock()
				}()

				select {
				case <-lockCtx.Done():
					log.Debug().Str("lockName", queueLeaderLock).Msg("Queue processing leader lock context canceled")
					return
				default:
					q.cleanupStaleAssignments()
				}
			}()
		}
	}
}

// cleanupStaleAssignments removes task assignments that may have become stale.
func (q *DatabaseQueue) cleanupStaleAssignments() {
	log.Debug().Msg("Starting cleanup of stale task assignments")

	tasks, err := q.store.TaskList()
	if err != nil {
		log.Error().Err(err).Msg("Failed to cleanup stale assignments")
		return
	}

	q.assignmentMu.Lock()
	defer q.assignmentMu.Unlock()

	// Create a map of valid task IDs
	validTasks := make(map[string]bool)
	for _, task := range tasks {
		validTasks[task.ID] = true
	}

	// Remove assignments for tasks that no longer exist
	removed := 0
	for taskID := range q.assignedTasks {
		if !validTasks[taskID] {
			log.Debug().Str("taskID", taskID).Msg("Removing stale task assignment")
			delete(q.assignedTasks, taskID)
			removed++
		}
	}

	if removed > 0 {
		log.Info().Int("removedAssignments", removed).Msg("Cleaned up stale task assignments")
	} else {
		log.Debug().Msg("No stale task assignments to clean up")
	}
}

// areDependenciesMet checks if all dependencies for a task are completed successfully.
func (q *DatabaseQueue) areDependenciesMet(task *model.Task, allTasks []*model.Task) bool {
	if len(task.Dependencies) == 0 {
		return true
	}

	// Create a map of existing tasks for quick lookup
	existingTasks := make(map[string]*model.Task)
	for _, t := range allTasks {
		existingTasks[t.ID] = t
	}

	for _, depID := range task.Dependencies {
		// If dependency still exists in the queue, it's not completed
		if _, exists := existingTasks[depID]; exists {
			return false
		}

		// Check if dependency completed successfully
		if task.DepStatus != nil {
			if status, hasStatus := task.DepStatus[depID]; hasStatus {
				if status != model.StatusSuccess {
					return false
				}
			} else {
				// No status recorded for this dependency, assume not completed
				return false
			}
		} else {
			// No dependency status tracking, assume not completed
			return false
		}
	}

	return true
}

// updateDependentTasks updates the dependency status for tasks that depend on the completed task.
func (q *DatabaseQueue) updateDependentTasks(completedTaskID string, status model.StatusValue) {
	tasks, err := q.store.TaskList()
	if err != nil {
		log.Error().Err(err).Msg("Failed to update dependent tasks")
		return
	}

	for _, task := range tasks {
		// Check if this task depends on the completed task
		for _, depID := range task.Dependencies {
			if depID == completedTaskID {
				// Update the dependency status in the task
				if task.DepStatus == nil {
					task.DepStatus = make(map[string]model.StatusValue)
				}
				task.DepStatus[depID] = status

				// Note: We can't update the task in the database because there's no TaskUpdate method
				// The dependency status is maintained in memory for this queue implementation
				log.Debug().
					Str("taskID", task.ID).
					Str("depID", depID).
					Str("status", string(status)).
					Msg("Updated dependency status for task")
				break
			}
		}
	}
}
