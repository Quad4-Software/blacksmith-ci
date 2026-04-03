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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v5/server/model"
	mocks_store "codefloe.com/crowci/crow/v5/server/store/mocks"
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
		pollingInstances:     make(map[string]*pollingInstance),
		assignTrigger:        make(chan struct{}, 1),
		processLeaderLockTTL: 30 * time.Second,
	}

	return q, mockStore
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
