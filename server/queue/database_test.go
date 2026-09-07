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
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v6/server/model"
	mocks_store "codefloe.com/crowci/crow/v6/server/store/mocks"
)

// mockLockService is a simple mock for the LockService interface.
type mockLockService struct{}

func (m *mockLockService) AcquireLock(_ context.Context, _ string, _ time.Duration) (context.Context, context.CancelFunc, error) {
	return nil, nil, ErrNotFound // always fail to acquire — tests don't need process leader
}

// testCancelCtx creates a cancellable context and returns a CancelFunc compatible
// with pollingInstance.cancel. Uses WithCancelCause to satisfy the forbidigo linter.
func testCancelCtx(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(parent)
	return ctx, func() { cancel(nil) }
}

func newTestDatabaseQueue(t *testing.T) (*DatabaseQueue, *mocks_store.MockStore) {
	t.Helper()
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	mockStore := mocks_store.NewMockStore(t)

	q := &DatabaseQueue{
		ctx:                  ctx,
		store:                mockStore,
		lockService:          &mockLockService{},
		instanceID:           "test-instance",
		startupTime:          time.Now(),
		cancelledTasks:       make(map[string]error),
		approvalGates:        make(map[string]approvalGate),
		pollingInstances:     make(map[string]*pollingInstance),
		assignTrigger:        make(chan struct{}, 1),
		processLeaderLockTTL: 30 * time.Second,
	}

	return q, mockStore
}

func TestAreDependenciesMet(t *testing.T) {
	t.Run("no dependencies are always met", func(t *testing.T) {
		q, _ := newTestDatabaseQueue(t)
		task := &model.Task{ID: "task-1"}
		assert.True(t, q.areDependenciesMet(task, []*model.Task{task}))
	})

	t.Run("dependency still present is not met", func(t *testing.T) {
		q, _ := newTestDatabaseQueue(t)
		dep := &model.Task{ID: "dep"}
		task := &model.Task{ID: "task-1", Dependencies: []string{"dep"}}
		assert.False(t, q.areDependenciesMet(task, []*model.Task{task, dep}))
	})

	t.Run("gone dependency with success status is met", func(t *testing.T) {
		q, _ := newTestDatabaseQueue(t)
		task := &model.Task{
			ID:           "task-1",
			Dependencies: []string{"dep"},
			DepStatus:    map[string]model.StatusValue{"dep": model.StatusSuccess},
		}
		assert.True(t, q.areDependenciesMet(task, []*model.Task{task}))
	})

	t.Run("gone dependency with failure status is not met", func(t *testing.T) {
		q, _ := newTestDatabaseQueue(t)
		task := &model.Task{
			ID:           "task-1",
			Dependencies: []string{"dep"},
			DepStatus:    map[string]model.StatusValue{"dep": model.StatusFailure},
		}
		assert.False(t, q.areDependenciesMet(task, []*model.Task{task}))
	})

	// Regression: a dependency that vanished before its status was recorded must
	// not block the downstream forever. It is synthesized as failed and persisted.
	t.Run("gone dependency without status is treated as failed and persisted", func(t *testing.T) {
		q, mockStore := newTestDatabaseQueue(t)
		task := &model.Task{ID: "task-1", Dependencies: []string{"dep"}}
		mockStore.EXPECT().TaskUpdate(task).Return(nil).Once()

		assert.False(t, q.areDependenciesMet(task, []*model.Task{task}))
		assert.Equal(t, model.StatusFailure, task.DepStatus["dep"])
	})
}

func TestDatabaseQueuePushTriggersAssignment(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	task := &model.Task{ID: "task-1", Data: []byte("test")}
	mockStore.EXPECT().TaskInsert(mock.AnythingOfType("*model.Task")).Return(nil)

	assert.NoError(t, q.Push(t.Context(), task))

	// Verify the assign trigger was signaled
	select {
	case <-q.assignTrigger:
		// expected
	default:
		t.Fatal("Expected assignTrigger to be signaled after Push")
	}
}

func TestDatabaseQueuePollRegistersInstance(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	// Mock the TaskList call from tryBatchAssignment (triggered by Poll's signal)
	mockStore.EXPECT().TaskList().Return([]*model.Task{}, nil).Maybe()

	// Start Poll in a goroutine (it blocks)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer cancel()
		_, _ = q.Poll(ctx, 42, filterFnTrue)
	}()

	// Give Poll time to register
	time.Sleep(50 * time.Millisecond)

	q.pollingInstancesMu.RLock()
	count := len(q.pollingInstances)
	var foundAgentID int64
	for _, inst := range q.pollingInstances {
		foundAgentID = inst.agentID
	}
	q.pollingInstancesMu.RUnlock()

	assert.Equal(t, 1, count, "Expected one polling instance registered")
	assert.Equal(t, int64(42), foundAgentID, "Expected agent ID 42")

	wg.Wait()
}

func TestDatabaseQueueBatchAssignmentCAS(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	task := &model.Task{ID: "task-1", Data: []byte("test"), AgentID: 0}

	// Register a polling instance manually
	responseChan := make(chan *model.Task, 1)
	_, instCancel := testCancelCtx(t.Context())
	defer instCancel()
	q.pollingInstancesMu.Lock()
	q.pollingInstances["test-inst"] = &pollingInstance{
		instanceID: "test-inst",
		agentID:    10,
		filter:     filterFnTrue,
		response:   responseChan,
		pollTime:   time.Now(),
		cancel:     instCancel,
	}
	q.pollingInstancesMu.Unlock()

	// Mock: TaskList returns one unassigned task
	mockStore.EXPECT().TaskList().Return([]*model.Task{task}, nil)
	// Mock: TaskAtomicAssign succeeds
	mockStore.EXPECT().TaskAtomicAssign("task-1", int64(10)).Return(true, nil)

	q.tryBatchAssignment()

	// Verify the task was delivered to the response channel
	select {
	case got := <-responseChan:
		assert.Equal(t, "task-1", got.ID)
		assert.Equal(t, int64(10), got.AgentID)
	default:
		t.Fatal("Expected task to be delivered to response channel")
	}

	// Verify the instance was removed from the map
	q.pollingInstancesMu.RLock()
	_, exists := q.pollingInstances["test-inst"]
	q.pollingInstancesMu.RUnlock()
	assert.False(t, exists, "Polling instance should be removed after assignment")
}

// TestDatabaseQueueBatchAssignmentFifoOrder verifies that, regardless of the
// order TaskList returns rows in, the oldest queued task (smallest workflow ID)
// is assigned first (issue #1208).
func TestDatabaseQueueBatchAssignmentFifoOrder(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	// Pre-cache approval gates so the assignment loop does not hit the store's
	// WorkflowLoad fallback for these numeric task IDs.
	q.cacheApprovalGate(&model.Task{ID: "2"})
	q.cacheApprovalGate(&model.Task{ID: "3"})

	// Older task is "2", newer is "3" — but TaskList returns them newest-first.
	older := &model.Task{ID: "2", Data: []byte("{}"), AgentID: 0}
	newer := &model.Task{ID: "3", Data: []byte("{}"), AgentID: 0}

	responseChan := make(chan *model.Task, 1)
	_, instCancel := testCancelCtx(t.Context())
	defer instCancel()
	q.pollingInstancesMu.Lock()
	q.pollingInstances["test-inst"] = &pollingInstance{
		instanceID: "test-inst",
		agentID:    10,
		filter:     filterFnTrue,
		response:   responseChan,
		pollTime:   time.Now(),
		cancel:     instCancel,
	}
	q.pollingInstancesMu.Unlock()

	mockStore.EXPECT().TaskList().Return([]*model.Task{newer, older}, nil)
	// Only the older task should be assigned; a mock for "3" is intentionally
	// absent so an out-of-order assignment would fail the test.
	mockStore.EXPECT().TaskAtomicAssign("2", int64(10)).Return(true, nil)

	q.tryBatchAssignment()

	select {
	case got := <-responseChan:
		assert.Equal(t, "2", got.ID, "older task must be assigned before newer one")
	default:
		t.Fatal("Expected the older task to be delivered to the response channel")
	}
}

func TestDatabaseQueueBatchAssignmentCASRace(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	task := &model.Task{ID: "task-1", Data: []byte("test"), AgentID: 0}

	responseChan := make(chan *model.Task, 1)
	_, instCancel := testCancelCtx(t.Context())
	defer instCancel()
	q.pollingInstancesMu.Lock()
	q.pollingInstances["test-inst"] = &pollingInstance{
		instanceID: "test-inst",
		agentID:    10,
		filter:     filterFnTrue,
		response:   responseChan,
		pollTime:   time.Now(),
		cancel:     instCancel,
	}
	q.pollingInstancesMu.Unlock()

	// Mock: TaskList returns one unassigned task
	mockStore.EXPECT().TaskList().Return([]*model.Task{task}, nil)
	// Mock: CAS fails (another server assigned it first)
	mockStore.EXPECT().TaskAtomicAssign("task-1", int64(10)).Return(false, nil)

	q.tryBatchAssignment()

	// Response channel should be empty — no task delivered
	select {
	case got := <-responseChan:
		t.Fatalf("Did not expect a task, got %v", got)
	default:
		// expected
	}
}

// TestDatabaseQueueBatchAssignmentHonorsPriority verifies that the HA queue
// assigns to the highest-priority eligible agent. The old first-match loop
// discarded the priority returned by the filter, so a lower-priority agent
// could win purely on map-iteration order.
func TestDatabaseQueueBatchAssignmentHonorsPriority(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)
	q.cacheApprovalGate(&model.Task{ID: "1"})

	task := &model.Task{ID: "1", Data: []byte("{}"), AgentID: 0}

	lowPriority := func(*model.Task) (bool, int, int32) { return true, 0, 1 }
	highPriority := func(*model.Task) (bool, int, int32) { return true, 0, 5 }

	lowChan := make(chan *model.Task, 1)
	highChan := make(chan *model.Task, 1)
	_, lowCancel := testCancelCtx(t.Context())
	defer lowCancel()
	_, highCancel := testCancelCtx(t.Context())
	defer highCancel()

	q.pollingInstancesMu.Lock()
	q.pollingInstances["low"] = &pollingInstance{
		instanceID: "low", agentID: 10, filter: lowPriority,
		response: lowChan, pollTime: time.Now(), cancel: lowCancel,
	}
	q.pollingInstances["high"] = &pollingInstance{
		instanceID: "high", agentID: 20, filter: highPriority,
		response: highChan, pollTime: time.Now(), cancel: highCancel,
	}
	q.pollingInstancesMu.Unlock()

	mockStore.EXPECT().TaskList().Return([]*model.Task{task}, nil)
	// Only the high-priority agent (20) may be assigned; a mock for agent 10 is
	// intentionally absent so a wrong assignment fails the test.
	mockStore.EXPECT().TaskAtomicAssign("1", int64(20)).Return(true, nil)

	q.tryBatchAssignment()

	select {
	case got := <-highChan:
		assert.Equal(t, "1", got.ID)
		assert.Equal(t, int64(20), got.AgentID, "task must go to the higher-priority agent")
	default:
		t.Fatal("Expected the task to be delivered to the high-priority agent")
	}
}

// TestDatabaseQueueBatchAssignmentBalancesLoad verifies that, among equal-
// priority agents, the least-loaded strategy spreads work by load fraction:
// agent 10 has two free slots and agent 20 has one, so the first task goes to
// agent 10 (tie broken by smallest ID) and the second to the now-less-loaded
// agent 20 rather than filling agent 10's remaining slot.
func TestDatabaseQueueBatchAssignmentBalancesLoad(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)
	q.cacheApprovalGate(&model.Task{ID: "1"})
	q.cacheApprovalGate(&model.Task{ID: "2"})

	task1 := &model.Task{ID: "1", Data: []byte("{}"), AgentID: 0}
	task2 := &model.Task{ID: "2", Data: []byte("{}"), AgentID: 0}

	samePriority := func(*model.Task) (bool, int, int32) { return true, 0, 0 }

	// Agent 10: two polling instances (capacity 2). Agent 20: one (capacity 1).
	a1 := make(chan *model.Task, 1)
	a2 := make(chan *model.Task, 1)
	b := make(chan *model.Task, 1)
	_, c1 := testCancelCtx(t.Context())
	defer c1()
	_, c2 := testCancelCtx(t.Context())
	defer c2()
	_, c3 := testCancelCtx(t.Context())
	defer c3()

	q.pollingInstancesMu.Lock()
	q.pollingInstances["a1"] = &pollingInstance{instanceID: "a1", agentID: 10, filter: samePriority, response: a1, pollTime: time.Now(), cancel: c1}
	q.pollingInstances["a2"] = &pollingInstance{instanceID: "a2", agentID: 10, filter: samePriority, response: a2, pollTime: time.Now(), cancel: c2}
	q.pollingInstances["b"] = &pollingInstance{instanceID: "b", agentID: 20, filter: samePriority, response: b, pollTime: time.Now(), cancel: c3}
	q.pollingInstancesMu.Unlock()

	mockStore.EXPECT().TaskList().Return([]*model.Task{task1, task2}, nil)
	mockStore.EXPECT().TaskAtomicAssign("1", int64(10)).Return(true, nil)
	mockStore.EXPECT().TaskAtomicAssign("2", int64(20)).Return(true, nil)

	q.tryBatchAssignment()

	// Task 2 lands on agent 20 (deterministic single instance).
	select {
	case got := <-b:
		assert.Equal(t, "2", got.ID)
		assert.Equal(t, int64(20), got.AgentID)
	default:
		t.Fatal("Expected task 2 to be delivered to agent 20")
	}
	// Task 1 lands on one of agent 10's two instances.
	select {
	case got := <-a1:
		assert.Equal(t, "1", got.ID)
	case got := <-a2:
		assert.Equal(t, "1", got.ID)
	default:
		t.Fatal("Expected task 1 to be delivered to agent 10")
	}
}

func TestDatabaseQueueErrorCancelKeepsRow(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	// Mock: TaskSetCancelled succeeds
	mockStore.EXPECT().TaskSetCancelled("task-1").Return(nil)
	// Mock: TaskList for updateDependentTasks (called from Error → updateDependentTasks)
	mockStore.EXPECT().TaskList().Return([]*model.Task{}, nil)

	// Error with ErrCancel should NOT call TaskDelete
	// (no TaskDelete mock set up — if called, mockery will fail the test)
	err := q.Error(t.Context(), "task-1", ErrCancel)
	assert.NoError(t, err)

	// Verify L1 cache has the cancel
	q.cancelledTasksMu.RLock()
	cancelErr, exists := q.cancelledTasks["task-1"]
	q.cancelledTasksMu.RUnlock()
	assert.True(t, exists, "L1 cache should have cancel entry")
	assert.ErrorIs(t, cancelErr, ErrCancel)
}

func TestDatabaseQueueErrorNonCancelDeletesRow(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	// Mock: for Done path (non-cancel error)
	mockStore.EXPECT().TaskList().Return([]*model.Task{}, nil)
	mockStore.EXPECT().TaskDelete("task-1").Return(nil)

	err := q.Error(t.Context(), "task-1", ErrNotFound)
	assert.NoError(t, err)

	// L1 cache should NOT have an entry
	q.cancelledTasksMu.RLock()
	_, exists := q.cancelledTasks["task-1"]
	q.cancelledTasksMu.RUnlock()
	assert.False(t, exists, "L1 cache should not have entry for non-cancel error")
}

func TestDatabaseQueueWaitDetectsCancelViaL1Cache(t *testing.T) {
	q, _ := newTestDatabaseQueue(t)

	// Pre-populate L1 cache
	q.cancelledTasksMu.Lock()
	q.cancelledTasks["task-1"] = ErrCancel
	q.cancelledTasksMu.Unlock()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	err := q.Wait(ctx, "task-1")
	assert.ErrorIs(t, err, ErrCancel)
}

func TestDatabaseQueueWaitDetectsCancelViaDB(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	// First poll: task exists and is canceled in DB
	mockStore.EXPECT().TaskGetCancelled("task-1").Return(true, true, nil)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	err := q.Wait(ctx, "task-1")
	assert.ErrorIs(t, err, ErrCancel)
}

func TestDatabaseQueueWaitDetectsNormalCompletion(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	// Task doesn't exist in DB (was deleted by Done)
	mockStore.EXPECT().TaskGetCancelled("task-1").Return(false, false, nil)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	err := q.Wait(ctx, "task-1")
	assert.NoError(t, err)
}

func TestDatabaseQueueExtendUsesTargetedQuery(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	// Mock: task exists and is assigned to agent 42
	mockStore.EXPECT().TaskGetAgentID("task-1", int64(42)).Return(true, nil)

	err := q.Extend(t.Context(), 42, "task-1")
	assert.NoError(t, err)
}

func TestDatabaseQueueExtendWrongAgent(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	// Mock: task exists but is NOT assigned to agent 99
	mockStore.EXPECT().TaskGetAgentID("task-1", int64(99)).Return(false, nil)

	err := q.Extend(t.Context(), 99, "task-1")
	assert.NoError(t, err) // returns nil, not error — task likely completed
}

func TestDatabaseQueueGetRunningWorkflowIDsFromDB(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	mockStore.EXPECT().TaskList().Return([]*model.Task{
		{ID: "1", AgentID: 10},
		{ID: "2", AgentID: 0},
		{ID: "3", AgentID: 20},
	}, nil)

	ids := q.GetRunningWorkflowIDs()
	assert.Len(t, ids, 2)
	assert.Contains(t, ids, "1")
	assert.Contains(t, ids, "3")
}

func TestDatabaseQueueKickAgentWorkers(t *testing.T) {
	q, _ := newTestDatabaseQueue(t)

	// Register two instances for agent 42 and one for agent 99
	ctx1, cancel1 := testCancelCtx(t.Context())
	ctx2, cancel2 := testCancelCtx(t.Context())
	ctx3, cancel3 := testCancelCtx(t.Context())
	defer cancel1()
	defer cancel2()
	defer cancel3()

	q.pollingInstancesMu.Lock()
	q.pollingInstances["a"] = &pollingInstance{instanceID: "a", agentID: 42, response: make(chan *model.Task, 1), cancel: cancel1}
	q.pollingInstances["b"] = &pollingInstance{instanceID: "b", agentID: 42, response: make(chan *model.Task, 1), cancel: cancel2}
	q.pollingInstances["c"] = &pollingInstance{instanceID: "c", agentID: 99, response: make(chan *model.Task, 1), cancel: cancel3}
	q.pollingInstancesMu.Unlock()

	q.KickAgentWorkers(42)

	q.pollingInstancesMu.RLock()
	remaining := len(q.pollingInstances)
	q.pollingInstancesMu.RUnlock()
	assert.Equal(t, 1, remaining, "Should have 1 instance remaining (agent 99)")

	// Verify contexts were canceled for agent 42
	assert.Error(t, ctx1.Err(), "Context for agent 42 instance a should be canceled")
	assert.Error(t, ctx2.Err(), "Context for agent 42 instance b should be canceled")
	assert.NoError(t, ctx3.Err(), "Context for agent 99 should not be canceled")
}

func TestDatabaseQueueEvictMarksCancelledInDB(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	mockStore.EXPECT().TaskSetCancelled("task-1").Return(nil)
	mockStore.EXPECT().TaskDelete("task-1").Return(nil)

	err := q.Evict(t.Context(), "task-1")
	assert.NoError(t, err)

	// Verify L1 cache
	q.cancelledTasksMu.RLock()
	_, exists := q.cancelledTasks["task-1"]
	q.cancelledTasksMu.RUnlock()
	assert.True(t, exists, "Evict should mark task canceled in L1 cache")
}

func TestDatabaseQueueInfoShowsWorkerCount(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	mockStore.EXPECT().TaskList().Return([]*model.Task{}, nil)

	// Register 3 polling instances
	q.pollingInstancesMu.Lock()
	for i := range 3 {
		_, cancelFn := testCancelCtx(t.Context())
		defer cancelFn()
		id := string(rune('a' + i))
		q.pollingInstances[id] = &pollingInstance{instanceID: id, agentID: int64(i), cancel: cancelFn}
	}
	q.pollingInstancesMu.Unlock()

	info := q.Info(t.Context())
	assert.Equal(t, 3, info.Stats.Workers, "Worker count should reflect polling instances")
}

// TestDatabaseQueueAwaitingApproval verifies that a task gated on a workflow
// approval is NOT dispatched even when a matching polling instance is
// waiting. Mirrors TestFifoAwaitingApproval for the DB-backed queue.
func TestDatabaseQueueAwaitingApproval(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	gated := &model.Task{
		ID:               "gated",
		Data:             []byte("{}"),
		RequiresApproval: true,
		ApprovalState:    model.ApprovalPending,
	}
	free := &model.Task{ID: "free", Data: []byte("{}")}

	// Push both tasks so the approval gate cache is populated for "gated"
	// and an empty (non-gated) entry exists for "free".
	mockStore.EXPECT().TaskInsert(mock.AnythingOfType("*model.Task")).Return(nil).Times(2)
	assert.NoError(t, q.PushAtOnce(t.Context(), []*model.Task{gated, free}))

	// Register a polling instance for the assignment loop to match.
	responseChan := make(chan *model.Task, 1)
	_, instCancel := testCancelCtx(t.Context())
	defer instCancel()
	q.pollingInstancesMu.Lock()
	q.pollingInstances["test-inst"] = &pollingInstance{
		instanceID: "test-inst",
		agentID:    10,
		filter:     filterFnTrue,
		response:   responseChan,
		pollTime:   time.Now(),
		cancel:     instCancel,
	}
	q.pollingInstancesMu.Unlock()

	// tryBatchAssignment lists tasks and assigns only ungated ones.
	mockStore.EXPECT().TaskList().Return([]*model.Task{gated, free}, nil)
	mockStore.EXPECT().TaskAtomicAssign("free", int64(10)).Return(true, nil)

	q.tryBatchAssignment()

	// Only "free" should be delivered to the polling instance. The mock
	// for TaskAtomicAssign is only registered for "free" — if the gated
	// task were dispatched, mockery would fail on the unexpected call.
	select {
	case got := <-responseChan:
		assert.Equal(t, "free", got.ID, "expect free task to be dispatched")
	default:
		t.Fatal("Expected free task to be delivered to response channel")
	}
}

// TestDatabaseQueueAwaitingApprovalInfoStats verifies that gated tasks appear
// in the WaitingOnDeps bucket of Info, not Pending.
func TestDatabaseQueueAwaitingApprovalInfoStats(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	gated := &model.Task{
		ID:               "gated",
		Data:             []byte("{}"),
		RequiresApproval: true,
		ApprovalState:    model.ApprovalPending,
	}

	// Push to seed the gate cache.
	mockStore.EXPECT().TaskInsert(mock.AnythingOfType("*model.Task")).Return(nil)
	assert.NoError(t, q.Push(t.Context(), gated))

	mockStore.EXPECT().TaskList().Return([]*model.Task{gated}, nil)
	info := q.Info(t.Context())

	assert.Equal(t, 1, info.Stats.WaitingOnDeps, "gated task should be counted as WaitingOnDeps")
	assert.Equal(t, 0, info.Stats.Pending, "gated task must not appear in Pending")
}

// TestDatabaseQueueSetApprovalStateReleases verifies that calling
// SetApprovalState(approved) lets the previously gated task be dispatched on
// the next assignment pass. Mirrors TestFifoSetApprovalStateReleases for the
// DB-backed queue.
func TestDatabaseQueueSetApprovalStateReleases(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	gated := &model.Task{
		ID:               "gated",
		Data:             []byte("{}"),
		RequiresApproval: true,
		ApprovalState:    model.ApprovalPending,
	}
	mockStore.EXPECT().TaskInsert(mock.AnythingOfType("*model.Task")).Return(nil)
	assert.NoError(t, q.Push(t.Context(), gated))

	// Drain the push signal so we can observe the SetApprovalState signal.
	select {
	case <-q.assignTrigger:
	default:
	}

	// Initially, the task is gated — confirm via Info stats.
	mockStore.EXPECT().TaskList().Return([]*model.Task{gated}, nil)
	info := q.Info(t.Context())
	assert.Equal(t, 1, info.Stats.WaitingOnDeps)

	// Flip the gate to approved.
	assert.NoError(t, q.SetApprovalState(t.Context(), "gated", model.ApprovalApproved))

	// Verify the signal channel was fired so the assignment loop re-evaluates.
	select {
	case <-q.assignTrigger:
		// expected
	default:
		t.Fatal("SetApprovalState should signal the assignment loop")
	}

	// Register a polling instance and run an assignment pass.
	responseChan := make(chan *model.Task, 1)
	_, instCancel := testCancelCtx(t.Context())
	defer instCancel()
	q.pollingInstancesMu.Lock()
	q.pollingInstances["test-inst"] = &pollingInstance{
		instanceID: "test-inst",
		agentID:    10,
		filter:     filterFnTrue,
		response:   responseChan,
		pollTime:   time.Now(),
		cancel:     instCancel,
	}
	q.pollingInstancesMu.Unlock()

	mockStore.EXPECT().TaskList().Return([]*model.Task{gated}, nil)
	mockStore.EXPECT().TaskAtomicAssign("gated", int64(10)).Return(true, nil)
	q.tryBatchAssignment()

	select {
	case got := <-responseChan:
		assert.Equal(t, "gated", got.ID, "approved task should be dispatched")
	default:
		t.Fatal("Expected gated task to be delivered after approval")
	}
}

// TestDatabaseQueueSetApprovalStateUnknownTask verifies that SetApprovalState
// on a task with no cache entry (e.g. pushed by a different HA peer, or
// reloaded after a restart) still records the state so the next assignment
// pass dispatches the task. We assert the cache was populated.
func TestDatabaseQueueSetApprovalStateUnknownTask(t *testing.T) {
	q, _ := newTestDatabaseQueue(t)

	assert.NoError(t, q.SetApprovalState(t.Context(), "remote-task", model.ApprovalApproved))

	q.approvalGatesMu.RLock()
	gate, exists := q.approvalGates["remote-task"]
	q.approvalGatesMu.RUnlock()

	assert.True(t, exists, "SetApprovalState should populate the cache for unknown tasks")
	assert.True(t, gate.requires, "Unknown tasks should be marked as requires-approval so the new state is authoritative")
	assert.Equal(t, model.ApprovalApproved, gate.state, "Cached state should match the supplied state")
}

// TestDatabaseQueueDoneClearsApprovalCache verifies that completing a task
// removes its entry from the approval gate cache to prevent unbounded growth.
func TestDatabaseQueueDoneClearsApprovalCache(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	gated := &model.Task{
		ID:               "gated",
		Data:             []byte("{}"),
		RequiresApproval: true,
		ApprovalState:    model.ApprovalPending,
	}
	mockStore.EXPECT().TaskInsert(mock.AnythingOfType("*model.Task")).Return(nil)
	assert.NoError(t, q.Push(t.Context(), gated))

	q.approvalGatesMu.RLock()
	_, exists := q.approvalGates["gated"]
	q.approvalGatesMu.RUnlock()
	assert.True(t, exists, "approval gate should be cached after Push")

	mockStore.EXPECT().TaskList().Return([]*model.Task{}, nil)
	mockStore.EXPECT().TaskDelete("gated").Return(nil)
	assert.NoError(t, q.Done(t.Context(), "gated", model.StatusSuccess))

	q.approvalGatesMu.RLock()
	_, exists = q.approvalGates["gated"]
	q.approvalGatesMu.RUnlock()
	assert.False(t, exists, "approval gate should be cleared after Done")
}

// TestDatabaseQueueAwaitingApprovalCacheMissFallsBackToWorkflow verifies that
// on a cache miss (e.g. an HA peer pushed the task), awaitingApproval loads
// the workflow row to determine the gate state.
func TestDatabaseQueueAwaitingApprovalCacheMissFallsBackToWorkflow(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	// Task with numeric ID so it parses to a workflow ID.
	task := &model.Task{ID: "42", Data: []byte("{}")}

	// Cache miss → store.WorkflowLoad called with workflow ID 42.
	mockStore.EXPECT().WorkflowLoad(int64(42)).Return(&model.Workflow{
		ID:               42,
		RequiresApproval: true,
		ApprovalState:    model.ApprovalPending,
	}, nil)

	assert.True(t, q.awaitingApproval(task), "should treat task as gated based on workflow row")

	// Result should be cached now.
	q.approvalGatesMu.RLock()
	gate, exists := q.approvalGates["42"]
	q.approvalGatesMu.RUnlock()
	assert.True(t, exists, "workflow load result should be cached")
	assert.True(t, gate.requires)
	assert.Equal(t, model.ApprovalPending, gate.state)

	// Second call should hit the cache (no extra WorkflowLoad mock — mockery
	// would fail the test on an unexpected call).
	assert.True(t, q.awaitingApproval(task))
}

// TestDatabaseQueueAwaitingApprovalNonNumericTaskID verifies that tasks
// whose IDs aren't workflow IDs (e.g. legacy / synthetic IDs) are treated as
// not gated when the cache is empty — they cannot be looked up against the
// workflows table.
func TestDatabaseQueueAwaitingApprovalNonNumericTaskID(t *testing.T) {
	q, _ := newTestDatabaseQueue(t)

	task := &model.Task{ID: "not-a-number", Data: []byte("{}")}

	assert.False(t, q.awaitingApproval(task), "non-numeric task IDs should not be gated when cache is empty")
}

// TestDatabaseQueueBlockOnApprovalHookFiresWhenDepsClear verifies that the
// DB queue invokes the injected BlockOnApprovalFn when an ApprovalNone-gated
// task's upstream deps are satisfied, flips the cached gate to Pending so
// the task is held, and does not dispatch the task while activation is in
// flight.
func TestDatabaseQueueBlockOnApprovalHookFiresWhenDepsClear(t *testing.T) {
	q, mockStore := newTestDatabaseQueue(t)

	var (
		hookCalls   atomic.Int32
		hookTaskID  atomic.Value // string
		hookFiredCh = make(chan struct{}, 1)
	)
	q.SetBlockOnApprovalHook(func(_ context.Context, taskID string) {
		hookTaskID.Store(taskID)
		if hookCalls.Add(1) == 1 {
			hookFiredCh <- struct{}{}
		}
	})

	// Seed the approval gate cache for the gated task. With ApprovalNone
	// already cached we don't need to mock WorkflowLoad.
	q.approvalGatesMu.Lock()
	q.approvalGates["2"] = approvalGate{requires: true, state: model.ApprovalNone}
	q.approvalGatesMu.Unlock()

	// Gated task with a satisfied dependency (DepStatus reports success and
	// task1 is NOT in the returned task list, so areDependenciesMet succeeds).
	gated := &model.Task{
		ID:           "2",
		Data:         []byte("{}"),
		Dependencies: []string{"1"},
		DepStatus: map[string]model.StatusValue{
			"1": model.StatusSuccess,
		},
		AgentID: 0,
	}

	// Register a polling instance so tryBatchAssignment has something to
	// dispatch to (otherwise it returns early).
	responseChan := make(chan *model.Task, 1)
	_, instCancel := testCancelCtx(t.Context())
	defer instCancel()
	q.pollingInstancesMu.Lock()
	q.pollingInstances["test-inst"] = &pollingInstance{
		instanceID: "test-inst",
		agentID:    10,
		filter:     filterFnTrue,
		response:   responseChan,
		pollTime:   time.Now(),
		cancel:     instCancel,
	}
	q.pollingInstancesMu.Unlock()

	mockStore.EXPECT().TaskList().Return([]*model.Task{gated}, nil)
	// TaskAtomicAssign must NOT be called — the gate intercepts dispatch.

	q.tryBatchAssignment()

	// Hook fires asynchronously; wait for it.
	select {
	case <-hookFiredCh:
	case <-time.After(2 * time.Second):
		t.Fatal("BlockOnApprovalFn was not invoked after deps cleared")
	}
	assert.Equal(t, "2", hookTaskID.Load())

	// Cache must now report the gate as Pending so the next assignment pass
	// also withholds the task.
	q.approvalGatesMu.RLock()
	gate := q.approvalGates["2"]
	q.approvalGatesMu.RUnlock()
	assert.True(t, gate.requires)
	assert.Equal(t, model.ApprovalPending, gate.state)
	assert.True(t, q.awaitingApproval(gated),
		"awaitingApproval must hold the task after activation")

	// Verify dispatch was indeed blocked.
	select {
	case got := <-responseChan:
		t.Fatalf("gated task should not have been dispatched, got %v", got)
	default:
		// expected
	}
}

// TestDatabaseQueueActivateApprovalGateIdempotent verifies the gate
// activation is a no-op when called repeatedly — subsequent passes after the
// state flips to Pending must not re-fire the hook.
func TestDatabaseQueueActivateApprovalGateIdempotent(t *testing.T) {
	q, _ := newTestDatabaseQueue(t)

	var hookCalls atomic.Int32
	q.SetBlockOnApprovalHook(func(_ context.Context, _ string) {
		hookCalls.Add(1)
	})

	gated := &model.Task{ID: "9", Data: []byte("{}")}

	// First, seed ApprovalNone — activation should trigger.
	q.approvalGatesMu.Lock()
	q.approvalGates["9"] = approvalGate{requires: true, state: model.ApprovalNone}
	q.approvalGatesMu.Unlock()

	assert.True(t, q.shouldActivateApprovalGate(gated),
		"task in ApprovalNone with requires=true should activate")
	q.activateApprovalGate("9")

	// After activation, shouldActivateApprovalGate must return false on
	// subsequent calls (state is now Pending).
	assert.False(t, q.shouldActivateApprovalGate(gated),
		"task in ApprovalPending should not re-activate")

	// Give the goroutine time to fire.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), hookCalls.Load(),
		"hook should fire exactly once for the activation")
}

// TestDatabaseQueueSetApprovalStateDeclinedHolds checks that a task whose
// gate was declined or expired is held and never assigned to an agent. It is
// the counterpart of TestDatabaseQueueSetApprovalStateReleases and mirrors
// TestFifoSetApprovalStateDeclinedHolds for the DB-backed queue.
func TestDatabaseQueueSetApprovalStateDeclinedHolds(t *testing.T) {
	for _, state := range []model.ApprovalState{model.ApprovalDeclined, model.ApprovalExpired} {
		t.Run(string(state), func(t *testing.T) {
			q, mockStore := newTestDatabaseQueue(t)

			gated := &model.Task{
				ID:               "gated",
				Data:             []byte("{}"),
				RequiresApproval: true,
				ApprovalState:    model.ApprovalPending,
			}
			mockStore.EXPECT().TaskInsert(mock.AnythingOfType("*model.Task")).Return(nil)
			assert.NoError(t, q.Push(t.Context(), gated))

			mockStore.EXPECT().TaskList().Return([]*model.Task{gated}, nil)
			assert.Equal(t, 1, q.Info(t.Context()).Stats.WaitingOnDeps)

			assert.NoError(t, q.SetApprovalState(t.Context(), "gated", state))

			responseChan := make(chan *model.Task, 1)
			_, instCancel := testCancelCtx(t.Context())
			defer instCancel()
			q.pollingInstancesMu.Lock()
			q.pollingInstances["test-inst"] = &pollingInstance{
				instanceID: "test-inst",
				agentID:    10,
				filter:     filterFnTrue,
				response:   responseChan,
				pollTime:   time.Now(),
				cancel:     instCancel,
			}
			q.pollingInstancesMu.Unlock()

			// No TaskAtomicAssign expectation: assigning the gated task would
			// fail the strict mock before it ever reached the response channel.
			q.tryBatchAssignment()

			select {
			case got := <-responseChan:
				t.Fatalf("a %s gate must not be assigned, got task %q", state, got.ID)
			default:
			}

			assert.Equal(t, 1, q.Info(t.Context()).Stats.WaitingOnDeps, "a %s gate must stay held", state)
		})
	}
}
