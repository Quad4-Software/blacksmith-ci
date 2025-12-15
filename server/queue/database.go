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
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/store"
)

const (
	// Maximum time to wait for a task before returning nil.
	// This prevents indefinite blocking and allows clients to retry.
	maxPollDuration = 30 * time.Second

	// Leader election lock name for queue processing.
	queueLeaderLock = "queue:processor:leader"

	// Assignment leader lock TTL - balanced for quick failover and database efficiency in HA mode.
	assignmentLeaderLockTTL = 15 * time.Second
	// Maximum canceled tasks to keep in memory before cleanup.
	maxCancelledTasksInMemory = 1000

	// Agent health timeout - if agent hasn't reported health in this duration, it's considered dead
	// This should be longer than reportHealthInterval (10s) to account for network delays.
	agentHealthTimeout = 60 * time.Second

	// Startup grace period - time to wait after server startup before resetting orphaned assignments
	// This gives agents time to reconnect after a server restart, preventing tasks from being
	// unnecessarily reset when agents are still alive and will reconnect momentarily.
	startupGracePeriod = 30 * time.Second
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
	startupTime   time.Time // Time when queue was initialized

	// Track assigned tasks to prevent double assignment
	assignedTasks map[string]int64 // task ID -> agent ID
	assignmentMu  sync.Mutex

	// Track canceled tasks to signal waiting agents
	cancelledTasks   map[string]error // task ID -> cancellation error
	cancelledTasksMu sync.RWMutex

	// Leader election state
	isLeader bool
	leaderMu sync.RWMutex

	// Track polling agents for batch assignment
	// We use a unique ID for each polling instance since one agent can have multiple workers
	pollingInstances   map[string]*pollingInstance // map[instanceID]*pollingInstance
	pollingInstancesMu sync.RWMutex
	nextInstanceID     int64
}

type pollingInstance struct {
	instanceID string
	agentID    int64
	filter     FilterFn
	response   chan *model.Task
	pollTime   time.Time
}

// NewDatabaseQueue creates a new database-backed queue for HA deployments.
func NewDatabaseQueue(ctx context.Context, store store.Store, lockService LockService, instanceID string, leaderLockTTL time.Duration) Queue {
	q := &DatabaseQueue{
		ctx:              ctx,
		store:            store,
		lockService:      lockService,
		instanceID:       instanceID,
		paused:           false,
		startupTime:      time.Now(),
		assignedTasks:    make(map[string]int64),
		cancelledTasks:   make(map[string]error),
		pollingInstances: make(map[string]*pollingInstance),
		leaderLockTTL:    leaderLockTTL,
	}

	log.Info().Str("instanceID", instanceID).Msg("Starting database queue for HA deployment")

	// Schedule orphaned assignment cleanup after grace period
	// This gives agents time to reconnect after server restart
	go func() {
		time.Sleep(startupGracePeriod)
		log.Info().Dur("gracePeriod", startupGracePeriod).Msg("Startup grace period elapsed, checking for orphaned task assignments")
		if err := q.resetOrphanedAssignments(); err != nil {
			log.Error().Err(err).Msg("Failed to reset orphaned task assignments after startup grace period")
		}
	}()

	// Start background processor with leader election for cleanup
	go q.processLoop() //nolint:contextcheck // Background process uses queue context

	// Start separate leader election process for task assignment
	go q.leaderElectionLoop()

	// Start background assignment processor for polling agents
	go q.assignmentLoop()

	return q
}

// Push pushes a task to the queue by storing it in the database.
func (q *DatabaseQueue) Push(ctx context.Context, task *model.Task) error {
	task.AgentID = 0
	err := q.store.TaskInsert(task)
	if err != nil {
		return err
	}

	// Trigger immediate assignment attempt if we're the leader and have polling instances
	q.leaderMu.RLock()
	isLeader := q.isLeader
	q.leaderMu.RUnlock()

	if isLeader {
		q.pollingInstancesMu.RLock()
		hasPolling := len(q.pollingInstances) > 0
		q.pollingInstancesMu.RUnlock()

		if hasPolling {
			// Trigger assignment in background to not block the Push operation
			go q.tryBatchAssignment()
		}
	}

	return nil
}

// PushAtOnce pushes multiple tasks to the queue.
func (q *DatabaseQueue) PushAtOnce(ctx context.Context, tasks []*model.Task) error {
	for _, task := range tasks {
		task.AgentID = 0
		if err := q.store.TaskInsert(task); err != nil {
			return err
		}
	}

	// Trigger immediate assignment attempt if we're the leader and have polling instances
	q.leaderMu.RLock()
	isLeader := q.isLeader
	q.leaderMu.RUnlock()

	if isLeader {
		q.pollingInstancesMu.RLock()
		hasPolling := len(q.pollingInstances) > 0
		q.pollingInstancesMu.RUnlock()

		if hasPolling {
			// Trigger assignment in background to not block the Push operation
			go q.tryBatchAssignment()
		}
	}

	return nil
}

// Poll retrieves and assigns a task to an agent.
func (q *DatabaseQueue) Poll(ctx context.Context, agentID int64, filter FilterFn) (*model.Task, error) {
	// Create a timeout to prevent indefinite blocking
	pollCtx, cancel := context.WithTimeout(ctx, maxPollDuration)
	defer cancel()

	// Create response channel for this polling instance
	responseChan := make(chan *model.Task, 1)

	// Generate unique instance ID for this polling request
	q.pollingInstancesMu.Lock()
	q.nextInstanceID++
	instanceID := fmt.Sprintf("%d-%d", agentID, q.nextInstanceID)
	q.pollingInstances[instanceID] = &pollingInstance{
		instanceID: instanceID,
		agentID:    agentID,
		filter:     filter,
		response:   responseChan,
		pollTime:   time.Now(),
	}
	instanceCount := len(q.pollingInstances)
	q.pollingInstancesMu.Unlock()

	log.Debug().
		Int64("agentID", agentID).
		Str("instanceID", instanceID).
		Int("totalPolling", instanceCount).
		Msg("Agent started polling for tasks")

	// Clean up on exit
	defer func() {
		q.pollingInstancesMu.Lock()
		if _, exists := q.pollingInstances[instanceID]; exists {
			delete(q.pollingInstances, instanceID)
			close(responseChan)
		}
		q.pollingInstancesMu.Unlock()
	}()

	// If we're the leader and have instances polling, trigger batch assignment
	q.leaderMu.RLock()
	isLeader := q.isLeader
	q.leaderMu.RUnlock()

	if isLeader && instanceCount > 0 {
		// Small delay to allow more instances to register if they're coming
		time.Sleep(10 * time.Millisecond)
		go q.tryBatchAssignment()
	}

	// Wait for task assignment or timeout
	select {
	case <-pollCtx.Done():
		return nil, nil
	case task := <-responseChan:
		return task, nil
	}
}

// Done marks a task as completed and removes it from the database.
func (q *DatabaseQueue) Done(ctx context.Context, id string, exitStatus model.StatusValue) error {
	// Remove from in-memory tracking (legacy, will be removed in future refactor)
	q.assignmentMu.Lock()
	delete(q.assignedTasks, id)
	q.assignmentMu.Unlock()

	// Update dependent tasks with completion status
	q.updateDependentTasks(id, exitStatus)

	return q.store.TaskDelete(id)
}

// Error marks a task as failed and removes it from the database.
func (q *DatabaseQueue) Error(ctx context.Context, id string, err error) error {
	// Track cancellation BEFORE deleting from database
	if errors.Is(err, ErrCancel) {
		q.cancelledTasksMu.Lock()
		q.cancelledTasks[id] = err
		q.cancelledTasksMu.Unlock()

		log.Info().
			Str("taskID", id).
			Int("totalCancelled", len(q.cancelledTasks)).
			Msg("Task canceled - stored in cancellation map for Wait() detection")
	}

	return q.Done(ctx, id, model.StatusFailure)
}

// ErrorAtOnce marks multiple tasks as failed.
func (q *DatabaseQueue) ErrorAtOnce(ctx context.Context, ids []string, err error) error {
	if errors.Is(err, ErrCancel) && len(ids) > 0 {
		log.Info().
			Strs("taskIDs", ids).
			Msg("Canceling multiple tasks")
	}

	for _, id := range ids {
		if errUpdate := q.Error(ctx, id, err); errUpdate != nil {
			return errUpdate
		}
	}
	return nil
}

// Evict removes a pending task from the queue.
func (q *DatabaseQueue) Evict(ctx context.Context, taskID string) error {
	// Track as canceled when evicting (eviction usually means cancellation)
	q.cancelledTasksMu.Lock()
	q.cancelledTasks[taskID] = ErrCancel
	q.cancelledTasksMu.Unlock()

	// Remove from in-memory tracking (legacy, will be removed in future refactor)
	q.assignmentMu.Lock()
	delete(q.assignedTasks, taskID)
	q.assignmentMu.Unlock()

	return q.store.TaskDelete(taskID)
}

// EvictAtOnce removes multiple pending tasks from the queue.
func (q *DatabaseQueue) EvictAtOnce(ctx context.Context, taskIDs []string) error {
	// Track all as canceled when evicting (eviction usually means cancellation)
	q.cancelledTasksMu.Lock()
	for _, id := range taskIDs {
		q.cancelledTasks[id] = ErrCancel
	}
	q.cancelledTasksMu.Unlock()

	if len(taskIDs) > 0 {
		log.Info().
			Strs("taskIDs", taskIDs).
			Int("count", len(taskIDs)).
			Msg("Evicting tasks and marking as canceled")
	}

	// Remove from in-memory tracking (legacy, will be removed in future refactor)
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
	log.Info().
		Str("taskID", taskID).
		Msg("Wait() called - agent monitoring for task completion or cancellation")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
			// First check if task was canceled
			q.cancelledTasksMu.RLock()
			if cancelErr, wasCancelled := q.cancelledTasks[taskID]; wasCancelled {
				q.cancelledTasksMu.RUnlock()
				log.Info().
					Str("taskID", taskID).
					Msg("Wait() detected task was canceled - returning ErrCancel to agent")
				return cancelErr
			}
			q.cancelledTasksMu.RUnlock()

			// Then check if task still exists in database
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
				// Task no longer in database - check once more for cancellation
				q.cancelledTasksMu.RLock()
				cancelledCount := len(q.cancelledTasks)
				if cancelErr, wasCancelled := q.cancelledTasks[taskID]; wasCancelled {
					q.cancelledTasksMu.RUnlock()
					log.Info().
						Str("taskID", taskID).
						Msg("Wait() detected task was canceled (after DB deletion) - returning ErrCancel")
					return cancelErr
				}
				q.cancelledTasksMu.RUnlock()

				// Task completed normally
				log.Info().
					Str("taskID", taskID).
					Int("cancelledTasksInMemory", cancelledCount).
					Msg("Wait() detected task completed normally (not in DB, not in canceled list)")
				return nil
			}
		}
	}
}

// Extend extends the task execution deadline by updating assignment time.
func (q *DatabaseQueue) Extend(ctx context.Context, agentID int64, taskID string) error {
	// Check task assignment from database instead of in-memory map
	// to support HA scenarios where the server may have restarted
	tasks, err := q.store.TaskList()
	if err != nil {
		return err
	}

	// Find the specific task
	var task *model.Task
	for _, t := range tasks {
		if t.ID == taskID {
			task = t
			break
		}
	}

	if task == nil {
		// Task not found in database - it was likely already completed and deleted.
		// This is a common race condition where the agent tries to extend a task
		// that was just completed. Return success to avoid unnecessary errors.
		log.Debug().Str("taskID", taskID).Int64("agentID", agentID).Msg("Task not found for extend - likely already completed")
		return nil
	}

	// Verify the task is assigned to the requesting agent
	if task.AgentID != agentID {
		return ErrAgentMissMatch
	}

	// For now, we just confirm the assignment exists
	// In a more sophisticated version, we could update timestamps in the database
	log.Debug().Str("taskID", taskID).Int64("agentID", agentID).Msg("Extended task deadline")
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
		// In HA mode, check the database state (task.AgentID) as the source of truth
		// The local assignedTasks map only contains tasks assigned by this server instance
		switch {
		case task.AgentID > 0:
			// Task is assigned to an agent (either by this server or another server in HA)
			info.Running = append(info.Running, task)
			info.Stats.Running++
		case q.areDependenciesMet(task, tasks):
			info.Pending = append(info.Pending, task)
			info.Stats.Pending++
		default:
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

// assignmentLoop periodically checks for waiting instances and assigns tasks.
func (q *DatabaseQueue) assignmentLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	cleanupTicker := time.NewTicker(10 * time.Second)
	defer cleanupTicker.Stop()

	cancelledCleanupTicker := time.NewTicker(5 * time.Minute)
	defer cancelledCleanupTicker.Stop()

	orphanedAssignmentTicker := time.NewTicker(30 * time.Second)
	defer orphanedAssignmentTicker.Stop()

	for {
		select {
		case <-q.ctx.Done():
			return
		case <-ticker.C:
			// Only process if we're the leader
			q.leaderMu.RLock()
			isLeader := q.isLeader
			q.leaderMu.RUnlock()

			if !isLeader {
				continue
			}

			// Check if we have polling instances
			q.pollingInstancesMu.RLock()
			instanceCount := len(q.pollingInstances)
			q.pollingInstancesMu.RUnlock()

			if instanceCount > 0 {
				q.tryBatchAssignment()
			}
		case <-cleanupTicker.C:
			// Clean up stale polling instances
			q.cleanupStalePollingInstances()
		case <-cancelledCleanupTicker.C:
			// Clean up old canceled tasks
			q.cleanupCancelledTasks()
		case <-orphanedAssignmentTicker.C:
			// Only the leader should reset orphaned assignments
			q.leaderMu.RLock()
			isLeader := q.isLeader
			q.leaderMu.RUnlock()

			if isLeader {
				// Check for and reset orphaned task assignments
				// This handles agents that die during runtime
				if err := q.resetOrphanedAssignments(); err != nil {
					log.Error().Err(err).Msg("Failed to reset orphaned task assignments")
				}
			}
		}
	}
}

// cleanupStalePollingInstances removes instances that have been polling for too long.
func (q *DatabaseQueue) cleanupStalePollingInstances() {
	q.pollingInstancesMu.Lock()
	defer q.pollingInstancesMu.Unlock()

	now := time.Now()
	for instanceID, instance := range q.pollingInstances {
		if now.Sub(instance.pollTime) > maxPollDuration {
			// Don't close the channel here - it's closed by Poll's defer
			delete(q.pollingInstances, instanceID)
			log.Debug().Str("instanceID", instanceID).Int64("agentID", instance.agentID).
				Msg("Cleaned up stale polling instance")
		}
	}
}

// cleanupCancelledTasks removes old canceled task entries to prevent memory leak.
func (q *DatabaseQueue) cleanupCancelledTasks() {
	q.cancelledTasksMu.RLock()
	count := len(q.cancelledTasks)
	q.cancelledTasksMu.RUnlock()

	// Only clear if we have excessive entries (memory concern)
	// We need to keep canceled tasks for Wait() to detect them
	if count > maxCancelledTasksInMemory {
		q.cancelledTasksMu.Lock()
		oldCount := len(q.cancelledTasks)
		q.cancelledTasks = make(map[string]error)
		q.cancelledTasksMu.Unlock()
		log.Warn().Int("count", oldCount).Msg("Cleared canceled tasks due to excessive memory usage")
	} else if count > 0 {
		log.Debug().Int("count", count).Msg("Keeping canceled tasks for Wait() detection")
	}
}

// leaderElectionLoop runs the leader election process for task assignment.
func (q *DatabaseQueue) leaderElectionLoop() {
	assignmentLockName := "queue:assignment:leader"
	log.Info().Str("lockName", assignmentLockName).Str("instanceID", q.instanceID).Msg("Starting task assignment leader election")

	for {
		select {
		case <-q.ctx.Done():
			log.Debug().Msg("Stopping task assignment leader election")
			return
		default:
			// Try to acquire assignment leader lock
			assignmentCtx, releaseLock, err := q.lockService.AcquireLock(q.ctx, assignmentLockName, assignmentLeaderLockTTL)
			if err != nil {
				// Failed to acquire lock - not the leader
				q.leaderMu.Lock()
				q.isLeader = false
				q.leaderMu.Unlock()

				// Wait before trying again
				time.Sleep(assignmentLeaderLockTTL)
				continue
			}

			// Successfully acquired lock - we are the leader
			q.leaderMu.Lock()
			q.isLeader = true
			q.leaderMu.Unlock()

			log.Info().Str("lockName", assignmentLockName).Str("instanceID", q.instanceID).Msg("Became task assignment leader")

			// Hold the lock until context is canceled or lock is lost
			<-assignmentCtx.Done()

			// Release the lock
			releaseLock()

			// Check if we should immediately retry (lock was lost due to transient error)
			// vs shutting down (context was canceled)
			if q.ctx.Err() != nil {
				// Context canceled, we're shutting down - don't retry
				q.leaderMu.Lock()
				q.isLeader = false
				q.leaderMu.Unlock()
				log.Info().Str("lockName", assignmentLockName).Str("instanceID", q.instanceID).Msg("Lost task assignment leadership - shutting down")
				return
			}

			// Lock was lost but we're not shutting down - immediately retry to minimize downtime
			// Keep isLeader=true during the retry to maintain task assignment
			log.Warn().Str("lockName", assignmentLockName).Str("instanceID", q.instanceID).Msg("Lost task assignment leadership - attempting immediate reacquisition")
		}
	}
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

// atomicTaskAssignment atomically assigns a task to an agent if it's still unassigned.
// Returns true if the assignment was successful, false if the task was already assigned to another agent.
func (q *DatabaseQueue) atomicTaskAssignment(taskID string, agentID int64) (bool, error) {
	return q.store.TaskAtomicAssign(taskID, agentID)
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

// tryBatchAssignment attempts to assign tasks to all polling instances.
func (q *DatabaseQueue) tryBatchAssignment() {
	q.mu.RLock()
	isPaused := q.paused
	q.mu.RUnlock()

	if isPaused {
		return
	}

	// Get tasks from database
	tasks, err := q.store.TaskList()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list tasks for batch assignment")
		return
	}

	// Get snapshot of polling instances
	q.pollingInstancesMu.RLock()
	instances := make([]*pollingInstance, 0, len(q.pollingInstances))
	for _, instance := range q.pollingInstances {
		instances = append(instances, instance)
	}
	q.pollingInstancesMu.RUnlock()

	if len(tasks) == 0 {
		return
	}

	if len(instances) == 0 {
		// Log when we have tasks but no agents polling - helps diagnose assignment issues
		unassignedCount := 0
		for _, task := range tasks {
			if task.AgentID == 0 {
				unassignedCount++
			}
		}
		if unassignedCount > 0 {
			log.Debug().
				Int("unassignedTasks", unassignedCount).
				Msg("Tasks waiting but no agents polling yet")
		}
		return
	}

	// Count ready tasks for logging
	readyTasks := 0
	for _, task := range tasks {
		if task.AgentID == 0 && q.areDependenciesMet(task, tasks) {
			readyTasks++
		}
	}

	if readyTasks > 0 {
		log.Debug().
			Int("readyTasks", readyTasks).
			Int("pollingInstances", len(instances)).
			Msg("Starting batch assignment")
	}

	// Track assignments made
	assignmentsMade := 0

	// Build a list of available instances (not nil)
	availableInstances := make([]*pollingInstance, 0, len(instances))
	for _, inst := range instances {
		if inst != nil {
			availableInstances = append(availableInstances, inst)
		}
	}

	// Try to assign tasks to waiting instances
	for _, task := range tasks {
		if len(availableInstances) == 0 {
			break // No more instances waiting
		}

		// Skip already assigned tasks (check DB state - single source of truth)
		if task.AgentID != 0 {
			continue
		}

		// Check dependencies
		if !q.areDependenciesMet(task, tasks) {
			continue
		}

		// Find an instance that can handle this task
		for i := 0; i < len(availableInstances); i++ {
			instance := availableInstances[i]
			if instance == nil {
				continue
			}

			if instance.filter != nil {
				matched, _, _ := instance.filter(task)
				if !matched {
					continue
				}
			}

			// Try to atomically assign the task in the database
			// This is the ONLY place where assignment happens - DB is source of truth
			atomicAssigned, err := q.atomicTaskAssignment(task.ID, instance.agentID)
			if err != nil {
				log.Error().Err(err).Str("taskID", task.ID).Int64("agentID", instance.agentID).
					Msg("Failed to atomically assign task in batch assignment")
				continue
			}

			if atomicAssigned {
				// Success! DB now has the assignment
				// Update in-memory tracking for legacy compatibility only
				q.assignmentMu.Lock()
				q.assignedTasks[task.ID] = instance.agentID
				q.assignmentMu.Unlock()

				task.AgentID = instance.agentID
				assignmentsMade++

				// Send task to the instance's channel
				select {
				case instance.response <- task:
					log.Debug().Str("taskID", task.ID).Int64("agentID", instance.agentID).
						Str("instanceID", instance.instanceID).
						Msg("Task assigned to polling instance in batch assignment")
				default:
					log.Warn().Str("taskID", task.ID).Int64("agentID", instance.agentID).
						Msg("Instance response channel full - task assigned in DB but agent may not receive it")
					// Note: Task is still assigned in DB, agent will eventually get it via polling
				}

				// Remove this instance from available instances
				// Swap with last element and shrink slice
				availableInstances[i] = availableInstances[len(availableInstances)-1]
				availableInstances = availableInstances[:len(availableInstances)-1]

				// Also remove from the map so it doesn't get picked up again
				q.pollingInstancesMu.Lock()
				delete(q.pollingInstances, instance.instanceID)
				q.pollingInstancesMu.Unlock()
				break
			}
		}
	}

	if assignmentsMade > 0 {
		log.Info().
			Int("assignmentsMade", assignmentsMade).
			Int("pollingInstances", len(instances)).
			Msg("Completed batch task assignment")
	}
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

				// Persist the updated dependency status to the database
				if err := q.store.TaskUpdate(task); err != nil {
					log.Error().
						Err(err).
						Str("taskID", task.ID).
						Str("depID", depID).
						Msg("Failed to persist dependency status update to database")
				} else {
					log.Debug().
						Str("taskID", task.ID).
						Str("depID", depID).
						Str("status", string(status)).
						Msg("Updated and persisted dependency status for task")
				}
				break
			}
		}
	}
}

// resetOrphanedAssignments resets task assignments for agents that are dead or non-existent.
// This is called on startup and periodically during runtime to handle:
// - Server restarts where agents may have disappeared
// - Agents that crashed without graceful shutdown
// - Agents that lost network connectivity.
func (q *DatabaseQueue) resetOrphanedAssignments() error {
	tasks, err := q.store.TaskList()
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}

	// Get list of all agents (including potentially dead ones)
	agents, err := q.store.AgentList(&model.AgentListOptions{ListOptions: model.ListOptions{All: true}})
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	// Build a map of agent IDs to their health status
	// An agent is considered healthy if it reported within agentHealthTimeout
	now := time.Now().Unix()
	healthyAgents := make(map[int64]bool)
	for _, agent := range agents {
		timeSinceLastContact := time.Duration(now-agent.LastContact) * time.Second
		if timeSinceLastContact < agentHealthTimeout {
			healthyAgents[agent.ID] = true
		} else {
			log.Debug().
				Int64("agentID", agent.ID).
				Dur("timeSinceLastContact", timeSinceLastContact).
				Msg("Agent considered unhealthy - hasn't reported recently")
		}
	}

	resetCount := 0
	for _, task := range tasks {
		// Skip unassigned tasks
		if task.AgentID == 0 {
			continue
		}

		// Check if the assigned agent is healthy
		if !healthyAgents[task.AgentID] {
			// Agent is dead or unhealthy - reset the assignment
			oldAgentID := task.AgentID
			task.AgentID = 0
			if err := q.store.TaskUpdate(task); err != nil {
				log.Error().
					Err(err).
					Str("taskID", task.ID).
					Int64("deadAgentID", oldAgentID).
					Msg("Failed to reset orphaned task assignment")
				continue
			}
			resetCount++
			log.Warn().
				Str("taskID", task.ID).
				Int64("deadAgentID", oldAgentID).
				Msg("Reset orphaned task assignment - agent is dead or unhealthy")
		}
	}

	if resetCount > 0 {
		log.Warn().Int("count", resetCount).Msg("Reset orphaned task assignments")
	}

	return nil
}
