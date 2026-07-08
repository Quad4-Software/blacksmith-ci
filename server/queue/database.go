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
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
)

// workflowIDFromTaskID parses a task ID as a workflow ID. Task IDs are
// fmt.Sprint(workflow.ID) when queued from the stepbuilder pipeline, so
// strconv.ParseInt is the inverse mapping.
func workflowIDFromTaskID(taskID string) (int64, error) {
	return strconv.ParseInt(taskID, 10, 64)
}

const (
	// Maximum time to wait for a task before returning nil.
	maxPollDuration = 30 * time.Second

	// Maximum canceled tasks to keep in memory before cleanup.
	maxCancelledTasksInMemory = 1000

	// Agent health timeout - if agent hasn't reported health in this duration, it's considered dead.
	agentHealthTimeout = 60 * time.Second

	// Startup grace period - time to wait after server startup before resetting orphaned assignments.
	startupGracePeriod = 30 * time.Second

	// AdoptionGracePeriodDB is the time to wait for an agent to reclaim an orphaned task
	// before resetting the assignment.
	adoptionGracePeriodDB = 5 * time.Minute

	// ProcessLeaderLock is the lock name for the orphan-cleanup leader.
	processLeaderLock = "queue:processor:leader"
)

// DatabaseQueue implements a distributed queue using the database as the backend.
// Every server instance independently assigns tasks to its locally-connected agents
// using atomic CAS operations (TaskAtomicAssign). No leader election is needed for
// task assignment — the database is the sole arbiter of ownership.
// Leader election is retained only for orphan cleanup (processLoop).
type DatabaseQueue struct {
	ctx         context.Context
	store       store.Store
	lockService LockService
	instanceID  string
	startupTime time.Time

	// Pause state
	mu     sync.RWMutex
	paused bool

	// Cancellation tracking for Wait() — in-process L1 cache.
	// The DB canceled column is the cross-instance source of truth.
	cancelledTasks   map[string]error
	cancelledTasksMu sync.RWMutex

	// Approval gate cache. Task.RequiresApproval and Task.ApprovalState are
	// bun:"-" so they are not persisted on the tasks row. We cache them in
	// memory keyed by task ID so the assignment loop can gate dispatch
	// without re-querying the workflows table for every task on every pass.
	// On a cache miss (e.g. tasks pushed by a different server instance, or
	// reloaded after a restart), the cache is lazily populated from the
	// workflow row — the workflows table is the durable source of truth.
	approvalGates   map[string]approvalGate
	approvalGatesMu sync.RWMutex

	// blockOnApproval activates the workflow approval gate when upstream deps
	// clear. Injected at server startup; nil during tests and pre-init.
	blockOnApprovalMu sync.RWMutex
	blockOnApproval   BlockOnApprovalFn

	// Polling agents connected to THIS server instance.
	pollingInstances   map[string]*pollingInstance
	pollingInstancesMu sync.RWMutex
	nextInstanceID     int64

	// Signal channel: Push/Poll triggers immediate assignment attempt.
	// Buffered at 1 so concurrent signals coalesce.
	assignTrigger chan struct{}

	processLeaderLockTTL time.Duration
}

// approvalGate captures the per-task approval cache entry.
type approvalGate struct {
	requires bool
	state    model.ApprovalState
}

type pollingInstance struct {
	instanceID string
	agentID    int64
	filter     FilterFn
	response   chan *model.Task
	pollTime   time.Time
	cancel     context.CancelFunc
}

// NewDatabaseQueue creates a new database-backed queue for HA deployments.
func NewDatabaseQueue(ctx context.Context, store store.Store, lockService LockService, instanceID string, processLeaderLockTTL time.Duration) Queue {
	q := &DatabaseQueue{
		ctx:                  ctx,
		store:                store,
		lockService:          lockService,
		instanceID:           instanceID,
		startupTime:          time.Now(),
		cancelledTasks:       make(map[string]error),
		approvalGates:        make(map[string]approvalGate),
		pollingInstances:     make(map[string]*pollingInstance),
		assignTrigger:        make(chan struct{}, 1),
		processLeaderLockTTL: processLeaderLockTTL,
	}

	log.Info().Str("instanceID", instanceID).Msg("Starting database queue for HA deployment (leaderless assignment)")

	// Startup reconciliation: clean up stale tasks from previous runs or
	// mode transitions (single-instance → HA). This handles tasks left
	// behind when Done() failed due to DB errors, or tasks written by the
	// persistentQueue wrapper in single-instance mode that were never cleaned up.
	if deleted, err := store.TaskDeleteStale(); err != nil {
		log.Error().Err(err).Msg("Failed to clean up stale tasks on startup")
	} else if deleted > 0 {
		log.Info().Int64("deleted", deleted).Msg("Cleaned up stale tasks on startup (workflows already completed)")
	}

	// Schedule orphaned assignment cleanup after grace period
	go func() {
		time.Sleep(startupGracePeriod)
		log.Info().Dur("gracePeriod", startupGracePeriod).Msg("Startup grace period elapsed, checking for orphaned task assignments")
		if err := q.resetOrphanedAssignments(); err != nil {
			log.Error().Err(err).Msg("Failed to reset orphaned task assignments after startup grace period")
		}
	}()

	// Start background process loop with leader election for cleanup only
	go q.processLoop() //nolint:contextcheck // Background process uses queue context

	// Start assignment processor — runs on every server instance
	go q.assignmentLoop()

	return q
}

// Push pushes a task to the queue by storing it in the database.
func (q *DatabaseQueue) Push(ctx context.Context, task *model.Task) error {
	task.AgentID = 0
	if err := q.store.TaskInsert(task); err != nil {
		return err
	}
	q.cacheApprovalGate(task)
	q.signalAssignment()
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
	for _, task := range tasks {
		q.cacheApprovalGate(task)
	}
	q.signalAssignment()
	return nil
}

// cacheApprovalGate records the in-memory approval gate for a task. The
// task struct carries RequiresApproval/ApprovalState (bun:"-") at push time;
// since these are not persisted on the tasks row, the queue needs an
// in-memory copy so subsequent assignment passes can gate dispatch without
// a workflow lookup on every iteration.
func (q *DatabaseQueue) cacheApprovalGate(task *model.Task) {
	q.approvalGatesMu.Lock()
	q.approvalGates[task.ID] = approvalGate{
		requires: task.RequiresApproval,
		state:    task.ApprovalState,
	}
	q.approvalGatesMu.Unlock()
}

// clearApprovalGate removes any cached approval state for a task. Called on
// terminal transitions (Done/Error/Evict) so the map does not leak entries
// across long-running server instances.
func (q *DatabaseQueue) clearApprovalGate(taskID string) {
	q.approvalGatesMu.Lock()
	delete(q.approvalGates, taskID)
	q.approvalGatesMu.Unlock()
}

// awaitingApproval reports whether a task should be held because its
// workflow approval gate has not been granted. The cache is consulted
// first; on a miss the workflow row is loaded and the result is cached
// so future passes don't re-hit the DB.
//
// On lookup error (e.g. workflow row gone — possible during a race with
// pipeline cleanup), the task is treated as not gated so it does not get
// stuck forever; the dispatch path will discover the broken state via its
// existing failure modes.
func (q *DatabaseQueue) awaitingApproval(task *model.Task) bool {
	q.approvalGatesMu.RLock()
	gate, cached := q.approvalGates[task.ID]
	q.approvalGatesMu.RUnlock()

	if !cached {
		// Cache miss: fall back to the workflow row (durable source of truth)
		// so HA peers and post-restart loads still gate correctly.
		wfID, err := workflowIDFromTaskID(task.ID)
		if err != nil {
			log.Debug().Err(err).Str("taskID", task.ID).
				Msg("Skipping approval gate lookup: task ID is not a workflow ID")
			return false
		}
		wf, err := q.store.WorkflowLoad(wfID)
		if err != nil {
			log.Warn().Err(err).Str("taskID", task.ID).
				Msg("Failed to load workflow for approval gate; treating as not gated")
			return false
		}
		gate = approvalGate{
			requires: wf.RequiresApproval,
			state:    wf.ApprovalState,
		}
		q.approvalGatesMu.Lock()
		q.approvalGates[task.ID] = gate
		q.approvalGatesMu.Unlock()
	}

	if !gate.requires {
		return false
	}
	return gate.state == model.ApprovalPending || gate.state == model.ApprovalNone
}

// shouldActivateApprovalGate reports whether the task is a gated task that
// has not yet been moved out of ApprovalNone. Called after deps are confirmed
// met, so this is exactly the activation moment.
//
// Reads via the same cache lookup awaitingApproval uses, so a cache miss
// triggers the workflow-row fallback already covered there.
func (q *DatabaseQueue) shouldActivateApprovalGate(task *model.Task) bool {
	q.approvalGatesMu.RLock()
	gate, cached := q.approvalGates[task.ID]
	q.approvalGatesMu.RUnlock()

	if !cached {
		// Same fallback path as awaitingApproval — load the workflow row
		// (durable source of truth) and cache the result. We don't dedup with
		// awaitingApproval to keep the activation check side-effect-free
		// before the hook runs.
		wfID, err := workflowIDFromTaskID(task.ID)
		if err != nil {
			return false
		}
		wf, err := q.store.WorkflowLoad(wfID)
		if err != nil {
			return false
		}
		gate = approvalGate{
			requires: wf.RequiresApproval,
			state:    wf.ApprovalState,
		}
		q.approvalGatesMu.Lock()
		q.approvalGates[task.ID] = gate
		q.approvalGatesMu.Unlock()
	}

	return gate.requires && gate.state == model.ApprovalNone
}

// signalAssignment sends a non-blocking signal to trigger immediate assignment.
func (q *DatabaseQueue) signalAssignment() {
	select {
	case q.assignTrigger <- struct{}{}:
	default:
	}
}

// Poll retrieves and assigns a task to an agent.
func (q *DatabaseQueue) Poll(ctx context.Context, agentID int64, filter FilterFn) (*model.Task, error) {
	pollCtx, cancel := context.WithTimeout(ctx, maxPollDuration)

	responseChan := make(chan *model.Task, 1)

	q.pollingInstancesMu.Lock()
	q.nextInstanceID++
	instanceID := fmt.Sprintf("%d-%d", agentID, q.nextInstanceID)
	q.pollingInstances[instanceID] = &pollingInstance{
		instanceID: instanceID,
		agentID:    agentID,
		filter:     filter,
		response:   responseChan,
		pollTime:   time.Now(),
		cancel:     cancel,
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
		cancel()
	}()

	// Signal assignment loop to check for tasks immediately
	q.signalAssignment()

	select {
	case <-pollCtx.Done():
		return nil, nil
	case task := <-responseChan:
		return task, nil
	}
}

// Done marks a task as completed and removes it from the database.
func (q *DatabaseQueue) Done(ctx context.Context, id string, exitStatus model.StatusValue) error {
	q.updateDependentTasks(id, exitStatus)
	q.clearApprovalGate(id)
	return q.store.TaskDelete(id)
}

// Error marks a task as failed.
// For cancellations, the task is marked canceled in the DB (not deleted) so
// cross-instance Wait() calls can detect the cancel. A background cleaner
// removes canceled rows after a TTL.
func (q *DatabaseQueue) Error(ctx context.Context, id string, err error) error {
	if errors.Is(err, ErrCancel) {
		// Mark canceled in DB for cross-instance visibility
		if setErr := q.store.TaskSetCancelled(id); setErr != nil {
			log.Error().Err(setErr).Str("taskID", id).Msg("Failed to set task canceled in DB")
		}

		// L1 cache for local Wait() calls
		q.cancelledTasksMu.Lock()
		q.cancelledTasks[id] = err
		q.cancelledTasksMu.Unlock()

		log.Info().Str("taskID", id).Msg("Task canceled — marked in DB, row kept for cross-instance Wait() detection")

		// Update dependent tasks but do NOT delete the row.
		// The row stays with canceled=true so Wait() on other server instances
		// can detect the cancellation. A background cleaner removes it later.
		q.updateDependentTasks(id, model.StatusFailure)
		q.clearApprovalGate(id)
		return nil
	}

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
	// Mark canceled in DB for cross-instance visibility
	if err := q.store.TaskSetCancelled(taskID); err != nil {
		log.Error().Err(err).Str("taskID", taskID).Msg("Failed to set task canceled in DB on evict")
	}

	// L1 cache
	q.cancelledTasksMu.Lock()
	q.cancelledTasks[taskID] = ErrCancel
	q.cancelledTasksMu.Unlock()

	q.clearApprovalGate(taskID)

	return q.store.TaskDelete(taskID)
}

// EvictAtOnce removes multiple pending tasks from the queue.
func (q *DatabaseQueue) EvictAtOnce(ctx context.Context, taskIDs []string) error {
	// Mark all as canceled in DB
	for _, id := range taskIDs {
		if err := q.store.TaskSetCancelled(id); err != nil {
			log.Error().Err(err).Str("taskID", id).Msg("Failed to set task canceled in DB on evict")
		}
	}

	// L1 cache
	q.cancelledTasksMu.Lock()
	for _, id := range taskIDs {
		q.cancelledTasks[id] = ErrCancel
	}
	q.cancelledTasksMu.Unlock()

	if len(taskIDs) > 0 {
		log.Info().Strs("taskIDs", taskIDs).Int("count", len(taskIDs)).Msg("Evicting tasks and marking as canceled")
	}

	for _, id := range taskIDs {
		q.clearApprovalGate(id)
		if err := q.store.TaskDelete(id); err != nil {
			return err
		}
	}
	return nil
}

// Wait waits until a task is completed (removed from database) or canceled.
// Uses targeted DB queries instead of full table scans.
func (q *DatabaseQueue) Wait(ctx context.Context, taskID string) error {
	log.Info().Str("taskID", taskID).Msg("Wait() called — monitoring for task completion or cancellation")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
			// L1 cache check (same-instance cancel)
			q.cancelledTasksMu.RLock()
			if cancelErr, wasCancelled := q.cancelledTasks[taskID]; wasCancelled {
				q.cancelledTasksMu.RUnlock()
				log.Info().Str("taskID", taskID).Msg("Wait() detected cancel via L1 cache")
				return cancelErr
			}
			q.cancelledTasksMu.RUnlock()

			// DB check — cross-instance source of truth
			canceled, exists, err := q.store.TaskGetCancelled(taskID)
			if err != nil {
				return err
			}

			if !exists {
				// Task row gone — check L1 cache one more time
				q.cancelledTasksMu.RLock()
				if cancelErr, wasCancelled := q.cancelledTasks[taskID]; wasCancelled {
					q.cancelledTasksMu.RUnlock()
					return cancelErr
				}
				q.cancelledTasksMu.RUnlock()
				// Task completed normally
				log.Info().Str("taskID", taskID).Msg("Wait() detected task completed normally")
				return nil
			}

			if canceled {
				log.Info().Str("taskID", taskID).Msg("Wait() detected cancel via DB")
				return ErrCancel
			}
		}
	}
}

// Extend verifies the task is still assigned to the requesting agent.
// Uses a targeted query instead of a full table scan.
func (q *DatabaseQueue) Extend(ctx context.Context, agentID int64, taskID string) error {
	assigned, err := q.store.TaskGetAgentID(taskID, agentID)
	if err != nil {
		return err
	}

	if !assigned {
		// Task not found or belongs to different agent — likely already completed
		log.Debug().Str("taskID", taskID).Int64("agentID", agentID).Msg("Task not found for extend — likely already completed")
		return nil
	}

	log.Debug().Str("taskID", taskID).Int64("agentID", agentID).Msg("Extended task deadline")
	return nil
}

// Reclaim re-adds a task to the running state after agent restart.
// For the database queue, this is a no-op since the database already has the task.
func (q *DatabaseQueue) Reclaim(_ context.Context, task *model.Task) error {
	log.Debug().Str("task_id", task.ID).Int64("agent_id", task.AgentID).Msg("task reclaimed (database queue — no-op)")
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

	q.pollingInstancesMu.RLock()
	workerCount := len(q.pollingInstances)
	q.pollingInstancesMu.RUnlock()

	info := InfoT{
		Paused: isPaused,
		Stats: struct {
			Workers       int `json:"worker_count"`
			Pending       int `json:"pending_count"`
			WaitingOnDeps int `json:"waiting_on_deps_count"`
			Running       int `json:"running_count"`
		}{
			Workers: workerCount,
		},
	}

	for _, task := range tasks {
		switch {
		case task.AgentID > 0:
			info.Running = append(info.Running, task)
			info.Stats.Running++
		case !q.areDependenciesMet(task, tasks) || q.awaitingApproval(task):
			info.WaitingOnDeps = append(info.WaitingOnDeps, task)
			info.Stats.WaitingOnDeps++
		default:
			info.Pending = append(info.Pending, task)
			info.Stats.Pending++
		}
	}

	return info
}

// GetRunningWorkflowIDs returns workflow IDs that are currently assigned (from DB, not in-memory).
func (q *DatabaseQueue) GetRunningWorkflowIDs() []string {
	tasks, err := q.store.TaskList()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get running workflow IDs from database")
		return nil
	}

	var ids []string
	for _, task := range tasks {
		if task.AgentID > 0 {
			ids = append(ids, task.ID)
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

// SetBlockOnApprovalHook injects the callback used to activate the workflow
// approval gate (persist State=Blocked, ApprovalState=Pending, deadline) when
// upstream deps clear. Pass nil to disable.
func (q *DatabaseQueue) SetBlockOnApprovalHook(fn BlockOnApprovalFn) {
	q.blockOnApprovalMu.Lock()
	defer q.blockOnApprovalMu.Unlock()
	q.blockOnApproval = fn
}

// activateApprovalGate fires the injected hook (if any) asynchronously and
// flips the cached gate state to Pending so subsequent passes through
// tryBatchAssignment treat the task as gate-blocked rather than
// gate-not-yet-activated. The hook runs in a goroutine because it does DB IO
// (workflow update) we don't want to block the assignment loop on, and
// because it calls back into the queue via SetApprovalState.
func (q *DatabaseQueue) activateApprovalGate(taskID string) {
	q.blockOnApprovalMu.RLock()
	fn := q.blockOnApproval
	q.blockOnApprovalMu.RUnlock()
	if fn != nil {
		log.Debug().Str("taskID", taskID).Msg("queue: deps cleared on gated task, activating approval gate")
		go fn(q.ctx, taskID)
	}
	q.approvalGatesMu.Lock()
	gate, ok := q.approvalGates[taskID]
	if !ok {
		gate = approvalGate{requires: true}
	}
	gate.state = model.ApprovalPending
	q.approvalGates[taskID] = gate
	q.approvalGatesMu.Unlock()
}

// SetApprovalState updates the cached approval gate for a task and signals
// the assignment loop to re-evaluate eligibility. The durable source of truth
// for the gate lives on the workflow row in the DB (updated by the caller);
// this cache update lets the next assignment pass dispatch the task without
// having to re-query the workflow row.
//
// The cache write must precede the signal so the re-evaluation observes the
// new state instead of racing against it.
func (q *DatabaseQueue) SetApprovalState(_ context.Context, taskID string, state model.ApprovalState) error {
	q.approvalGatesMu.Lock()
	gate, exists := q.approvalGates[taskID]
	if !exists {
		// No prior cache entry — the task may have been pushed by another
		// server instance, or this server restarted after the original Push.
		// Mark it as approval-required so the next assignment pass treats the
		// supplied state authoritatively (rather than skipping the task as
		// "not gated" because the cache is empty).
		gate = approvalGate{requires: true}
	}
	gate.state = state
	q.approvalGates[taskID] = gate
	q.approvalGatesMu.Unlock()

	log.Debug().Str("taskID", taskID).Str("state", string(state)).
		Msg("Database queue approval gate updated")

	// Signal AFTER the cache write so the assignment loop sees the new state.
	q.signalAssignment()
	return nil
}

// KickAgentWorkers cancels all polling instances for a given agent.
func (q *DatabaseQueue) KickAgentWorkers(agentID int64) {
	q.pollingInstancesMu.Lock()
	defer q.pollingInstancesMu.Unlock()

	kickedCount := 0
	for id, inst := range q.pollingInstances {
		if inst.agentID == agentID {
			inst.cancel()
			delete(q.pollingInstances, id)
			kickedCount++
		}
	}

	if kickedCount > 0 {
		log.Info().Int64("agentID", agentID).Int("kicked", kickedCount).Msg("Kicked agent workers")
	}
}

// assignmentLoop checks for waiting instances and assigns tasks.
// Runs on EVERY server instance — no leader election needed.
// Assignment is primarily signal-driven (Push/Poll trigger immediately).
// The fallback ticker is slow (2s) to avoid hammering the database.
func (q *DatabaseQueue) assignmentLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	cleanupTicker := time.NewTicker(10 * time.Second)
	defer cleanupTicker.Stop()

	cancelledCleanupTicker := time.NewTicker(5 * time.Minute)
	defer cancelledCleanupTicker.Stop()

	for {
		select {
		case <-q.ctx.Done():
			return

		case <-ticker.C:
			// Slow fallback — catches any tasks missed by signal-driven assignment
			if q.hasPollers() && !q.isPaused() {
				q.tryBatchAssignment()
			}

		case <-q.assignTrigger:
			// Primary path: immediate assignment on Push/Poll signal.
			// Small delay to batch concurrent signals (e.g. PushAtOnce).
			time.Sleep(10 * time.Millisecond)
			// Drain any extra signals that arrived during the delay
			select {
			case <-q.assignTrigger:
			default:
			}
			if q.hasPollers() && !q.isPaused() {
				q.tryBatchAssignment()
			}

		case <-cleanupTicker.C:
			q.cleanupStalePollingInstances()

		case <-cancelledCleanupTicker.C:
			q.cleanupCancelledTasks()
		}
	}
}

func (q *DatabaseQueue) hasPollers() bool {
	q.pollingInstancesMu.RLock()
	n := len(q.pollingInstances)
	q.pollingInstancesMu.RUnlock()
	return n > 0
}

func (q *DatabaseQueue) isPaused() bool {
	q.mu.RLock()
	p := q.paused
	q.mu.RUnlock()
	return p
}

// cleanupStalePollingInstances removes instances that have been polling for too long.
func (q *DatabaseQueue) cleanupStalePollingInstances() {
	q.pollingInstancesMu.Lock()
	defer q.pollingInstancesMu.Unlock()

	now := time.Now()
	for instanceID, instance := range q.pollingInstances {
		if now.Sub(instance.pollTime) > maxPollDuration {
			instance.cancel() // unblock the Poll goroutine and release context resources
			delete(q.pollingInstances, instanceID)
			log.Debug().Str("instanceID", instanceID).Int64("agentID", instance.agentID).
				Msg("Cleaned up stale polling instance")
		}
	}
}

// cleanupCancelledTasks cleans up canceled task entries from both L1 cache and DB.
func (q *DatabaseQueue) cleanupCancelledTasks() {
	// Clean L1 cache if too large
	q.cancelledTasksMu.RLock()
	count := len(q.cancelledTasks)
	q.cancelledTasksMu.RUnlock()

	if count > maxCancelledTasksInMemory {
		q.cancelledTasksMu.Lock()
		oldCount := len(q.cancelledTasks)
		q.cancelledTasks = make(map[string]error)
		q.cancelledTasksMu.Unlock()
		log.Warn().Int("count", oldCount).Msg("Cleared canceled tasks L1 cache due to excessive memory usage")
	}

	// Purge canceled rows from DB that are older than 5 minutes.
	// This gives cross-instance Wait() calls time to observe the canceled flag.
	cutoff := time.Now().Add(-5 * time.Minute).Unix()
	if err := q.store.TaskDeleteCancelledBefore(cutoff); err != nil {
		log.Error().Err(err).Msg("Failed to purge old canceled tasks from DB")
	}
}

// processLoop runs background cleanup with leader election.
// Only one server instance runs orphan cleanup at a time.
func (q *DatabaseQueue) processLoop() {
	log.Debug().Str("lockName", processLeaderLock).Dur("ttl", q.processLeaderLockTTL).Msg("Starting database queue process loop (orphan cleanup)")

	orphanTicker := time.NewTicker(30 * time.Second)
	defer orphanTicker.Stop()

	for {
		select {
		case <-q.ctx.Done():
			log.Debug().Msg("Stopping database queue process loop")
			return
		case <-orphanTicker.C:
			lockCtx, releaseLock, err := q.lockService.AcquireLock(q.ctx, processLeaderLock, q.processLeaderLockTTL)
			if err != nil {
				log.Debug().Err(err).Str("lockName", processLeaderLock).Msg("Failed to acquire process leader lock")
				continue
			}

			log.Info().Str("lockName", processLeaderLock).Str("instanceID", q.instanceID).Msg("Acquired process leader lock — performing orphan cleanup")

			func() {
				defer releaseLock()

				select {
				case <-lockCtx.Done():
					return
				default:
					if err := q.resetOrphanedAssignments(); err != nil {
						log.Error().Err(err).Msg("Failed to reset orphaned task assignments")
					}
					// Reconcile stale tasks — remove task rows for workflows that are
					// no longer pending/running. This catches tasks that Done() failed
					// to delete (e.g. during DB connection errors).
					if deleted, err := q.store.TaskDeleteStale(); err != nil {
						log.Error().Err(err).Msg("Failed to reconcile stale tasks")
					} else if deleted > 0 {
						log.Info().Int64("deleted", deleted).Msg("Reconciled stale tasks (workflows already completed)")
					}
				}
			}()
		}
	}
}

// areDependenciesMet checks if all dependencies for a task are completed successfully.
//
// A dependency can be in one of three states:
//   - still present as a task row → not finished yet, keep waiting.
//   - gone, with a status recorded in DepStatus → resolved; met only if success.
//   - gone, with no status recorded → the dependency task disappeared before its
//     completion status was persisted into this task's DepStatus. This is a race
//     between updateDependentTasks (which records DepStatus) and task deletion
//     (Done/Error → TaskDelete, or the TaskDeleteStale reconciler). If we simply
//     returned false here the downstream would wait forever: the dependency is
//     gone, so no future updateDependentTasks call will ever populate DepStatus.
//     Treat such a phantom dependency as failed — record it so the state is
//     observable and stable — which unblocks the downstream (it will not be
//     assigned, and is reaped by the pipeline-level cascade-skip that operates
//     on the workflow tree, not on task rows).
func (q *DatabaseQueue) areDependenciesMet(task *model.Task, allTasks []*model.Task) bool {
	if len(task.Dependencies) == 0 {
		return true
	}

	existingTasks := make(map[string]*model.Task)
	for _, t := range allTasks {
		existingTasks[t.ID] = t
	}

	met := true
	for _, depID := range task.Dependencies {
		if _, exists := existingTasks[depID]; exists {
			// Dependency still queued/running — not finished yet.
			return false
		}

		if task.DepStatus != nil {
			if status, hasStatus := task.DepStatus[depID]; hasStatus {
				if status != model.StatusSuccess {
					met = false
				}
				continue
			}
		}

		// Dependency is gone and left no status: it terminated in a way we can
		// no longer observe. Assume failure so the downstream unblocks instead
		// of waiting on a phantom dependency forever.
		log.Warn().Str("taskID", task.ID).Str("depID", depID).
			Msg("dependency vanished without a recorded status — treating as failed to avoid a permanent block")
		if task.DepStatus == nil {
			task.DepStatus = make(map[string]model.StatusValue)
		}
		task.DepStatus[depID] = model.StatusFailure
		if err := q.store.TaskUpdate(task); err != nil {
			log.Error().Err(err).Str("taskID", task.ID).Str("depID", depID).
				Msg("failed to persist synthesized failed dependency status")
		}
		met = false
	}

	return met
}

// tryBatchAssignment attempts to assign tasks to all polling instances.
// Every server instance runs this independently. The DB CAS (TaskAtomicAssign)
// ensures only one server wins each task assignment.
func (q *DatabaseQueue) tryBatchAssignment() {
	tasks, err := q.store.TaskList()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list tasks for batch assignment")
		return
	}

	// TaskList has no defined order (the tasks.id column is TEXT, and the row
	// order drifts as agent_id is updated/reset). Assign oldest-first so older
	// pipelines are served before newer ones — workflow IDs are monotonic, so
	// taskLess yields enqueue order (issue #1208).
	sort.SliceStable(tasks, func(i, j int) bool { return taskLess(tasks[i], tasks[j]) })

	// Get snapshot of polling instances
	q.pollingInstancesMu.RLock()
	instances := make([]*pollingInstance, 0, len(q.pollingInstances))
	for _, instance := range q.pollingInstances {
		instances = append(instances, instance)
	}
	q.pollingInstancesMu.RUnlock()

	if len(tasks) == 0 || len(instances) == 0 {
		return
	}

	// Build available instances list
	availableInstances := make([]*pollingInstance, 0, len(instances))
	for _, inst := range instances {
		if inst != nil {
			availableInstances = append(availableInstances, inst)
		}
	}

	assignmentsMade := 0

	for _, task := range tasks {
		if len(availableInstances) == 0 {
			break
		}

		// Skip already assigned tasks
		if task.AgentID != 0 {
			continue
		}

		if !q.areDependenciesMet(task, tasks) {
			continue
		}

		// Gate activation: deps just cleared on a task that requires approval
		// but has not yet been blocked. Fire the injected hook (which persists
		// the workflow row) and mark the cache Pending so this and subsequent
		// passes hold the task until the gate is decided.
		if q.shouldActivateApprovalGate(task) {
			q.activateApprovalGate(task.ID)
			continue
		}

		// Skip tasks whose workflow is still gated on approval.
		if q.awaitingApproval(task) {
			continue
		}

		for i := 0; i < len(availableInstances); i++ {
			instance := availableInstances[i]

			if instance.filter != nil {
				matched, _, _ := instance.filter(task)
				if !matched {
					continue
				}
			}

			// Atomic CAS — only one server wins
			atomicAssigned, err := q.store.TaskAtomicAssign(task.ID, instance.agentID)
			if err != nil {
				log.Error().Err(err).Str("taskID", task.ID).Int64("agentID", instance.agentID).
					Msg("DB error during atomic task assignment")
				break
			}

			if atomicAssigned {
				task.AgentID = instance.agentID
				assignmentsMade++

				// Atomically check the instance is still registered and send.
				// Poll's deferred cleanup closes the channel under pollingInstancesMu,
				// so holding the lock while sending prevents a send-on-closed-channel panic.
				delivered := false
				q.pollingInstancesMu.Lock()
				if _, stillRegistered := q.pollingInstances[instance.instanceID]; stillRegistered {
					select {
					case instance.response <- task:
						log.Debug().Str("taskID", task.ID).Int64("agentID", instance.agentID).
							Str("instanceID", instance.instanceID).
							Msg("Task assigned to polling instance")
						delivered = true
					default:
						log.Warn().Str("taskID", task.ID).Int64("agentID", instance.agentID).
							Msg("Instance response channel full")
					}
					delete(q.pollingInstances, instance.instanceID)
				} else {
					log.Debug().Str("taskID", task.ID).Int64("agentID", instance.agentID).
						Msg("Polling instance gone before delivery")
				}
				q.pollingInstancesMu.Unlock()

				// If delivery failed, reset the DB assignment so the task goes
				// back to the unassigned pool immediately instead of waiting for
				// orphan cleanup (which could be 30s+ away).
				if !delivered {
					if _, resetErr := q.store.TaskAtomicResetOrphan(task.ID, instance.agentID); resetErr != nil {
						log.Error().Err(resetErr).Str("taskID", task.ID).
							Msg("Failed to reset task assignment after delivery failure")
					} else {
						log.Info().Str("taskID", task.ID).Int64("agentID", instance.agentID).
							Msg("Reset task assignment after delivery failure — task back in pool")
						assignmentsMade-- // doesn't count as a successful assignment
					}
				}

				// Remove this instance from available list
				availableInstances[i] = availableInstances[len(availableInstances)-1]
				availableInstances = availableInstances[:len(availableInstances)-1]
				break
			}

			// CAS failed — another server assigned this task; skip to next task
			log.Debug().Str("taskID", task.ID).Msg("CAS race on task assignment — another server assigned it")
			break
		}
	}

	if assignmentsMade > 0 {
		log.Info().Int("assignmentsMade", assignmentsMade).Msg("Completed batch task assignment")
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
		for _, depID := range task.Dependencies {
			if depID == completedTaskID {
				if task.DepStatus == nil {
					task.DepStatus = make(map[string]model.StatusValue)
				}
				task.DepStatus[depID] = status

				if err := q.store.TaskUpdate(task); err != nil {
					log.Error().Err(err).Str("taskID", task.ID).Str("depID", depID).
						Msg("Failed to persist dependency status update")
				} else {
					log.Debug().Str("taskID", task.ID).Str("depID", depID).Str("status", string(status)).
						Msg("Updated dependency status for task")
				}
				break
			}
		}
	}
}

// resetOrphanedAssignments resets task assignments for agents that are dead or non-existent.
func (q *DatabaseQueue) resetOrphanedAssignments() error {
	tasks, err := q.store.TaskList()
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}

	agents, err := q.store.AgentList(&model.AgentListOptions{ListOptions: model.ListOptions{All: true}})
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	now := time.Now()
	healthyAgents := make(map[int64]bool)
	agentLastContact := make(map[int64]time.Time)

	for _, agent := range agents {
		lastContact := time.Unix(agent.LastContact, 0)
		agentLastContact[agent.ID] = lastContact
		if now.Sub(lastContact) < agentHealthTimeout {
			healthyAgents[agent.ID] = true
		} else {
			log.Debug().Int64("agentID", agent.ID).Dur("timeSinceLastContact", now.Sub(lastContact)).
				Msg("Agent considered unhealthy")
		}
	}

	resetCount := 0
	skippedForAdoption := 0
	for _, task := range tasks {
		if task.AgentID == 0 {
			continue
		}

		if healthyAgents[task.AgentID] {
			continue
		}

		lastContact, hasLastContact := agentLastContact[task.AgentID]
		if hasLastContact {
			timeSinceLastContact := now.Sub(lastContact)
			if timeSinceLastContact < adoptionGracePeriodDB {
				log.Debug().Str("taskID", task.ID).Int64("agentID", task.AgentID).
					Dur("timeSinceLastContact", timeSinceLastContact).Dur("gracePeriod", adoptionGracePeriodDB).
					Msg("Task orphaned but within adoption grace period")
				skippedForAdoption++
				continue
			}
		}

		// Atomic reset — only succeeds if agent_id still matches
		oldAgentID := task.AgentID
		reset, err := q.store.TaskAtomicResetOrphan(task.ID, oldAgentID)
		if err != nil {
			log.Error().Err(err).Str("taskID", task.ID).Int64("deadAgentID", oldAgentID).
				Msg("Failed to atomically reset orphaned task assignment")
			continue
		}
		if !reset {
			log.Debug().Str("taskID", task.ID).Int64("deadAgentID", oldAgentID).
				Msg("Orphan reset skipped — agent reclaimed task before reset")
			continue
		}
		resetCount++
		log.Warn().Str("taskID", task.ID).Int64("deadAgentID", oldAgentID).
			Msg("Reset orphaned task assignment — agent dead and adoption grace period elapsed")
	}

	if resetCount > 0 || skippedForAdoption > 0 {
		log.Info().Int("reset", resetCount).Int("waitingForAdoption", skippedForAdoption).
			Msg("Orphaned task assignment check completed")
	}

	return nil
}
