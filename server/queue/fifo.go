// Copyright 2022 Woodpecker Authors
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
	"container/list"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store"
	"codefloe.com/crowci/crow/v5/shared/constant"
)

type entry struct {
	item       *model.Task
	done       chan bool
	error      error
	deadline   time.Time
	orphanedAt time.Time // When the task became orphaned (deadline expired without extension)
}

type worker struct {
	agentID int64
	filter  FilterFn
	channel chan *model.Task
	stop    context.CancelCauseFunc
}

type fifo struct {
	sync.Mutex

	ctx           context.Context
	store         store.Store // Database store for task persistence
	workers       map[*worker]struct{}
	running       map[string]*entry
	pending       *list.List
	waitingOnDeps *list.List
	extension     time.Duration
	paused        bool
}

// processTimeInterval is the time till the queue rearranges things,
// as the agent pull in 10 milliseconds we should also give them work asap.
const processTimeInterval = 100 * time.Millisecond

// minCapacityScore is used as the initial value for capacity scoring during task assignment.
const minCapacityScore = -1000

// adoptionGracePeriod is the time to wait for an agent to reclaim an orphaned task
// before canceling it as a zombie. This should be long enough for an agent to restart
// and re-register, but short enough to not leave zombie tasks running indefinitely.
const adoptionGracePeriod = 5 * time.Minute

var ErrWorkerKicked = fmt.Errorf("worker was kicked")

// NewMemoryQueue returns a new fifo queue.
func NewMemoryQueue(ctx context.Context, store store.Store) Queue {
	q := &fifo{
		ctx:           ctx,
		store:         store,
		workers:       map[*worker]struct{}{},
		running:       map[string]*entry{},
		pending:       list.New(),
		waitingOnDeps: list.New(),
		extension:     constant.TaskTimeout,
		paused:        false,
	}

	// Task loading is handled by WithTaskStore wrapper which filters for pending/running workflows
	// This prevents double-loading and ensures only active tasks are restored on restart

	go q.process()
	return q
}

// Push pushes a task to the tail of this queue.
func (q *fifo) Push(_ context.Context, task *model.Task) error {
	q.Lock()
	q.pending.PushBack(task)
	q.Unlock()
	return nil
}

// PushAtOnce pushes multiple tasks to the tail of this queue.
func (q *fifo) PushAtOnce(_ context.Context, tasks []*model.Task) error {
	q.Lock()
	for _, task := range tasks {
		q.pending.PushBack(task)
	}
	q.Unlock()
	return nil
}

// Poll retrieves and removes a task head of this queue.
func (q *fifo) Poll(c context.Context, agentID int64, filter FilterFn) (*model.Task, error) {
	q.Lock()
	ctx, stop := context.WithCancelCause(c)

	_worker := &worker{
		agentID: agentID,
		channel: make(chan *model.Task, 1),
		filter:  filter,
		stop:    stop,
	}
	q.workers[_worker] = struct{}{}
	workerCount := 0
	for w := range q.workers {
		if w.agentID == agentID {
			workerCount++
		}
	}
	log.Debug().Msgf("queue: agent %d created worker, now has %d workers waiting", agentID, workerCount)
	q.Unlock()

	// Add a timeout to prevent indefinite blocking, which causes issues with
	// reverse proxies (nginx, traefik, etc.) that have connection timeouts.
	// 30 seconds is well under typical proxy timeouts (60s) and still provides
	// good responsiveness for task pickup.
	pollTimeout := time.NewTimer(30 * time.Second)
	defer pollTimeout.Stop()

	for {
		select {
		case <-ctx.Done():
			q.Lock()
			delete(q.workers, _worker)
			q.Unlock()
			return nil, ctx.Err()
		case <-pollTimeout.C:
			// Timeout reached with no task available
			q.Lock()
			delete(q.workers, _worker)
			q.Unlock()
			// Return nil task (not an error) - agent will immediately poll again
			return nil, nil
		case t := <-_worker.channel:
			return t, nil
		}
	}
}

// Done signals the task is complete.
func (q *fifo) Done(_ context.Context, id string, exitStatus model.StatusValue) error {
	return q.finished([]string{id}, exitStatus, nil)
}

// Error signals the task is done with an error.
func (q *fifo) Error(_ context.Context, id string, err error) error {
	return q.finished([]string{id}, model.StatusFailure, err)
}

// ErrorAtOnce signals multiple done are complete with an error.
func (q *fifo) ErrorAtOnce(_ context.Context, ids []string, err error) error {
	return q.finished(ids, model.StatusFailure, err)
}

func (q *fifo) finished(ids []string, exitStatus model.StatusValue, err error) error {
	q.Lock()

	for _, id := range ids {
		taskEntry, ok := q.running[id]
		if ok {
			taskEntry.error = err
			close(taskEntry.done)
			delete(q.running, id)
			// Task deletion from database is handled by persistentQueue wrapper
		} else {
			q.removeFromPending(id)
		}
		q.updateDepStatusInQueue(id, exitStatus)
	}

	q.Unlock()
	return nil
}

// Evict removes a pending task from the queue.
func (q *fifo) Evict(ctx context.Context, taskID string) error {
	return q.EvictAtOnce(ctx, []string{taskID})
}

// EvictAtOnce removes multiple pending tasks from the queue.
func (q *fifo) EvictAtOnce(_ context.Context, taskIDs []string) error {
	q.Lock()
	defer q.Unlock()

	for _, id := range taskIDs {
		var next *list.Element
		for element := q.pending.Front(); element != nil; element = next {
			next = element.Next()
			task, ok := element.Value.(*model.Task)
			if ok && task.ID == id {
				q.pending.Remove(element)
				return nil
			}
		}
	}
	return ErrNotFound
}

// Wait waits until the item is done executing.
func (q *fifo) Wait(ctx context.Context, taskID string) error {
	q.Lock()
	state := q.running[taskID]
	q.Unlock()
	if state != nil {
		select {
		case <-ctx.Done():
		case <-state.done:
			return state.error
		}
	}
	return nil
}

// Extend extends the task execution deadline.
func (q *fifo) Extend(_ context.Context, agentID int64, taskID string) error {
	q.Lock()
	defer q.Unlock()

	state, ok := q.running[taskID]
	if ok {
		if state.item.AgentID != agentID {
			return ErrAgentMissMatch
		}

		state.deadline = time.Now().Add(q.extension)
		return nil
	}
	return ErrNotFound
}

// Reclaim re-adds a task to the running state after agent restart.
// This is used when an agent reclaims an orphaned task that it was previously running.
func (q *fifo) Reclaim(_ context.Context, task *model.Task) error {
	q.Lock()
	defer q.Unlock()

	// Remove from pending if it was resubmitted there by resubmitExpiredPipelines
	var next *list.Element
	for e := q.pending.Front(); e != nil; e = next {
		next = e.Next()
		if t, ok := e.Value.(*model.Task); ok && t.ID == task.ID {
			q.pending.Remove(e)
			break
		}
	}

	// Add to running state with fresh deadline and done channel
	q.running[task.ID] = &entry{
		item:     task,
		done:     make(chan bool),
		deadline: time.Now().Add(q.extension),
	}

	log.Debug().
		Str("task_id", task.ID).
		Int64("agent_id", task.AgentID).
		Msg("task reclaimed and re-added to running state")

	return nil
}

// Info returns internal queue information.
func (q *fifo) Info(_ context.Context) InfoT {
	q.Lock()
	defer q.Unlock()

	info := InfoT{
		Stats: struct {
			Workers       int `json:"worker_count"`
			Pending       int `json:"pending_count"`
			WaitingOnDeps int `json:"waiting_on_deps_count"`
			Running       int `json:"running_count"`
		}{
			Workers:       len(q.workers),
			Pending:       q.pending.Len(),
			Running:       len(q.running),
			WaitingOnDeps: q.waitingOnDeps.Len(),
		},
		Paused: q.paused,
	}

	for _, entry := range q.running {
		info.Running = append(info.Running, entry.item)
	}

	for element := q.pending.Front(); element != nil; element = element.Next() {
		if task, ok := element.Value.(*model.Task); ok {
			info.Pending = append(info.Pending, task)
		}
	}

	for element := q.waitingOnDeps.Front(); element != nil; element = element.Next() {
		if task, ok := element.Value.(*model.Task); ok {
			info.WaitingOnDeps = append(info.WaitingOnDeps, task)
		}
	}

	return info
}

// GetRunningWorkflowIDs returns a list of workflow IDs that are currently running
// This is useful for cleanup operations.
func (q *fifo) GetRunningWorkflowIDs() []string {
	q.Lock()
	defer q.Unlock()

	var ids []string
	for id := range q.running {
		ids = append(ids, id)
	}
	return ids
}

// Pause stops the queue from handing out new work items in Poll.
func (q *fifo) Pause() {
	q.Lock()
	q.paused = true
	q.Unlock()
}

// Resume starts the queue again.
func (q *fifo) Resume() {
	q.Lock()
	q.paused = false
	q.Unlock()
}

// KickAgentWorkers kicks all workers for a given agent.
func (q *fifo) KickAgentWorkers(agentID int64) {
	q.Lock()
	defer q.Unlock()

	for worker := range q.workers {
		if worker.agentID == agentID {
			worker.stop(ErrWorkerKicked)
			delete(q.workers, worker)
		}
	}
}

// helper function that loops through the queue and attempts to
// match the item to a single subscriber until context got cancel.
func (q *fifo) process() {
	for {
		select {
		case <-time.After(processTimeInterval):
		case <-q.ctx.Done():
			return
		}

		q.Lock()
		if q.paused {
			q.Unlock()
			continue
		}

		q.resubmitExpiredPipelines()
		q.filterWaiting()
		for pending, worker := q.assignToWorker(); pending != nil && worker != nil; pending, worker = q.assignToWorker() {
			task, _ := pending.Value.(*model.Task)
			task.AgentID = worker.agentID
			delete(q.workers, worker)
			q.pending.Remove(pending)
			q.running[task.ID] = &entry{
				item:     task,
				done:     make(chan bool),
				deadline: time.Now().Add(q.extension),
			}
			worker.channel <- task
		}
		q.Unlock()
	}
}

func (q *fifo) filterWaiting() {
	// resubmits all waiting tasks to pending, deps may have cleared
	var nextWaiting *list.Element
	for e := q.waitingOnDeps.Front(); e != nil; e = nextWaiting {
		nextWaiting = e.Next()
		task, _ := e.Value.(*model.Task)
		q.pending.PushBack(task)
	}

	// rebuild waitingDeps
	q.waitingOnDeps = list.New()
	var filtered []*list.Element
	var nextPending *list.Element
	for element := q.pending.Front(); element != nil; element = nextPending {
		nextPending = element.Next()
		task, _ := element.Value.(*model.Task)
		if q.depsInQueue(task) {
			log.Debug().Msgf("queue: waiting due to unmet dependencies %v", task.ID)
			q.waitingOnDeps.PushBack(task)
			filtered = append(filtered, element)
		}
	}

	// filter waiting tasks
	for _, f := range filtered {
		q.pending.Remove(f)
	}
}

func (q *fifo) assignToWorker() (*list.Element, *worker) {
	var next *list.Element

	for element := q.pending.Front(); element != nil; element = next {
		next = element.Next()
		task, _ := element.Value.(*model.Task)
		log.Debug().Msgf("queue: trying to assign task: %v with deps %v", task.ID, task.Dependencies)

		// Collect all matching workers with their scores and priorities
		type workerMatch struct {
			worker   *worker
			score    int
			priority int32
		}
		var matches []workerMatch

		for worker := range q.workers {
			matched, score, priority := worker.filter(task)
			if matched {
				matches = append(matches, workerMatch{
					worker:   worker,
					score:    score,
					priority: priority,
				})
			}
		}

		if len(matches) == 0 {
			continue
		}

		// Find the highest priority among all matched agents
		var maxPriority int32
		for _, match := range matches {
			if match.priority > maxPriority {
				maxPriority = match.priority
			}
		}

		// Collect all agents with the highest priority
		var candidates []workerMatch
		for _, match := range matches {
			if match.priority == maxPriority {
				candidates = append(candidates, match)
			}
		}

		// Pick a candidate from those with the highest priority
		var selectedWorker *worker
		var selectedPriority int32

		if len(candidates) == 1 {
			selectedWorker = candidates[0].worker
			selectedPriority = candidates[0].priority
		} else {
			// Multiple candidates with same priority - prefer agent with more available capacity
			// Count running tasks and waiting workers per agent
			agentTaskCount := make(map[int64]int)
			agentWorkerCount := make(map[int64]int)

			// Count running tasks per agent
			for _, entry := range q.running {
				agentTaskCount[entry.item.AgentID]++
			}

			// Count waiting workers per agent
			for worker := range q.workers {
				agentWorkerCount[worker.agentID]++
			}

			// Find the candidate with the most available capacity
			// Available capacity = waiting workers - running tasks
			// This ensures we prefer agents that have more workers ready to take tasks
			maxAvailableCapacity := minCapacityScore
			for _, candidate := range candidates {
				runningTasks := agentTaskCount[candidate.worker.agentID]
				waitingWorkers := agentWorkerCount[candidate.worker.agentID]
				availableCapacity := waitingWorkers - runningTasks

				log.Debug().Msgf("queue: agent %d has %d waiting workers, %d running tasks, capacity score: %d",
					candidate.worker.agentID, waitingWorkers, runningTasks, availableCapacity)

				if availableCapacity > maxAvailableCapacity {
					maxAvailableCapacity = availableCapacity
					selectedWorker = candidate.worker
					selectedPriority = candidate.priority
				}
			}
		}

		if selectedWorker != nil {
			log.Debug().Msgf("queue: assigned task: %v with deps %v to worker with priority %d", task.ID, task.Dependencies, selectedPriority)
			return element, selectedWorker
		}
	}

	return nil, nil
}

func (q *fifo) resubmitExpiredPipelines() {
	now := time.Now()
	for taskID, taskState := range q.running {
		if now.After(taskState.deadline) {
			// Task deadline expired - mark as orphaned if not already
			if taskState.orphanedAt.IsZero() {
				taskState.orphanedAt = now
				log.Info().
					Str("task_id", taskID).
					Int64("agent_id", taskState.item.AgentID).
					Msg("task orphaned, waiting for agent to reclaim")
				continue // Give agent time to reclaim
			}

			// Check if grace period has passed
			if now.Sub(taskState.orphanedAt) < adoptionGracePeriod {
				continue // Still within grace period
			}

			// Grace period expired - cancel as zombie task
			log.Warn().
				Str("task_id", taskID).
				Int64("agent_id", taskState.item.AgentID).
				Dur("orphaned_for", now.Sub(taskState.orphanedAt)).
				Msg("orphaned task not reclaimed within grace period, canceling as zombie")

			taskState.error = fmt.Errorf("task orphaned and not reclaimed within grace period")
			delete(q.running, taskID)
			close(taskState.done)
		}
	}
}

func (q *fifo) depsInQueue(task *model.Task) bool {
	var next *list.Element
	for element := q.pending.Front(); element != nil; element = next {
		next = element.Next()
		possibleDep, ok := element.Value.(*model.Task)
		log.Debug().Msgf("queue: pending right now: %v", possibleDep.ID)
		for _, dep := range task.Dependencies {
			if ok && possibleDep.ID == dep {
				return true
			}
		}
	}
	for possibleDepID := range q.running {
		log.Debug().Msgf("queue: running right now: %v", possibleDepID)
		for _, dep := range task.Dependencies {
			if possibleDepID == dep {
				return true
			}
		}
	}
	return false
}

func (q *fifo) updateDepStatusInQueue(taskID string, status model.StatusValue) {
	var next *list.Element

	for element := q.pending.Front(); element != nil; element = next {
		next = element.Next()
		pending, ok := element.Value.(*model.Task)
		for _, dep := range pending.Dependencies {
			if ok && taskID == dep {
				pending.DepStatus[dep] = status
			}
		}
	}

	for _, running := range q.running {
		for _, dep := range running.item.Dependencies {
			if taskID == dep {
				running.item.DepStatus[dep] = status
			}
		}
	}

	for element := q.waitingOnDeps.Front(); element != nil; element = next {
		next = element.Next()
		waiting, ok := element.Value.(*model.Task)
		for _, dep := range waiting.Dependencies {
			if ok && taskID == dep {
				waiting.DepStatus[dep] = status
			}
		}
	}

	// Only trigger immediate re-evaluation for successful completions
	if status == model.StatusSuccess {
		q.filterWaiting()
	}
}

func (q *fifo) removeFromPending(taskID string) {
	log.Debug().Msgf("queue: trying to remove %s", taskID)
	var next *list.Element
	for element := q.pending.Front(); element != nil; element = next {
		next = element.Next()
		task, _ := element.Value.(*model.Task)
		if task.ID == taskID {
			log.Debug().Msgf("queue: %s is removed from pending", taskID)
			q.pending.Remove(element)
			return
		}
	}
}
