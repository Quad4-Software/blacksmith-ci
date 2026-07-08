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

package datastore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
)

func TestTaskList(t *testing.T) {
	store, closer := newTestStore(t, new(model.Task))
	defer closer()

	assert.NoError(t, store.TaskInsert(&model.Task{
		ID:        "some_random_id",
		Data:      []byte("foo"),
		Labels:    map[string]string{"foo": "bar"},
		DepStatus: map[string]model.StatusValue{"test": "dep"},
	}))

	list, err := store.TaskList()
	assert.NoError(t, err)
	assert.Len(t, list, 1, "Expected one task in list")
	assert.Equal(t, "some_random_id", list[0].ID)
	assert.Equal(t, "foo", string(list[0].Data))
	assert.EqualValues(t, map[string]model.StatusValue{"test": "dep"}, list[0].DepStatus)

	assert.NoError(t, store.TaskDelete("some_random_id"))

	list, err = store.TaskList()
	assert.NoError(t, err)
	assert.Len(t, list, 0, "Want empty task list after delete")
}

func TestTaskUpdate(t *testing.T) {
	store, closer := newTestStore(t, new(model.Task))
	defer closer()

	task := &model.Task{
		ID:        "test_update_id",
		Data:      []byte("initial"),
		Labels:    map[string]string{"env": "test"},
		DepStatus: map[string]model.StatusValue{},
		AgentID:   0,
	}

	// Insert task
	assert.NoError(t, store.TaskInsert(task))

	// Update agent assignment
	task.AgentID = 123
	task.Data = []byte("updated")
	assert.NoError(t, store.TaskUpdate(task))

	// Verify update
	list, err := store.TaskList()
	assert.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, int64(123), list[0].AgentID)
	assert.Equal(t, "updated", string(list[0].Data))

	// Cleanup
	assert.NoError(t, store.TaskDelete("test_update_id"))
}

func TestTaskAtomicAssign(t *testing.T) {
	store, closer := newTestStore(t, new(model.Task))
	defer closer()

	// Insert unassigned task
	task := &model.Task{
		ID:        "test_atomic_assign",
		Data:      []byte("test"),
		Labels:    map[string]string{},
		DepStatus: map[string]model.StatusValue{},
		AgentID:   0, // Unassigned
	}
	assert.NoError(t, store.TaskInsert(task))

	// First agent should successfully assign the task
	assigned, err := store.TaskAtomicAssign("test_atomic_assign", 123)
	assert.NoError(t, err)
	assert.True(t, assigned, "First agent should successfully assign task")

	// Second agent should fail to assign the same task
	assigned, err = store.TaskAtomicAssign("test_atomic_assign", 456)
	assert.NoError(t, err)
	assert.False(t, assigned, "Second agent should fail to assign already assigned task")

	// Verify task is assigned to first agent
	list, err := store.TaskList()
	assert.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, int64(123), list[0].AgentID)

	// Test assigning non-existent task
	assigned, err = store.TaskAtomicAssign("non_existent", 789)
	assert.NoError(t, err)
	assert.False(t, assigned, "Should fail to assign non-existent task")

	// Cleanup
	assert.NoError(t, store.TaskDelete("test_atomic_assign"))
}

func TestTaskListPending(t *testing.T) {
	store, closer := newTestStore(t, new(model.Task), new(model.Workflow))
	defer closer()

	// Create workflows with different states
	pendingWorkflow := &model.Workflow{ID: 10, PipelineID: 1, PID: 1, Name: "pending-wf", State: model.StatusPending}
	runningWorkflow := &model.Workflow{ID: 20, PipelineID: 1, PID: 2, Name: "running-wf", State: model.StatusRunning}
	successWorkflow := &model.Workflow{ID: 30, PipelineID: 1, PID: 3, Name: "success-wf", State: model.StatusSuccess}
	blockedWorkflow := &model.Workflow{ID: 40, PipelineID: 1, PID: 4, Name: "blocked-wf", State: model.StatusBlocked}

	workflows := []*model.Workflow{pendingWorkflow, runningWorkflow, successWorkflow, blockedWorkflow}
	_, err := store.db.NewInsert().Model(&workflows).Exec(context.Background())
	assert.NoError(t, err)

	// Create tasks whose IDs match the workflow IDs (as strings)
	assert.NoError(t, store.TaskInsert(&model.Task{ID: "10", Data: []byte("pending-task")}))
	assert.NoError(t, store.TaskInsert(&model.Task{ID: "20", Data: []byte("running-task")}))
	assert.NoError(t, store.TaskInsert(&model.Task{ID: "30", Data: []byte("completed-task")}))
	assert.NoError(t, store.TaskInsert(&model.Task{ID: "40", Data: []byte("blocked-task")}))

	// TaskListPending should return tasks for pending/running AND blocked workflows.
	// Blocked workflows are paused awaiting human approval — their tasks must
	// survive a server restart so dispatch can resume after ApproveWorkflow runs.
	list, err := store.TaskListPending()
	assert.NoError(t, err)
	assert.Len(t, list, 3)

	ids := make(map[string]bool)
	for _, task := range list {
		ids[task.ID] = true
	}
	assert.True(t, ids["10"], "Should include task for pending workflow")
	assert.True(t, ids["20"], "Should include task for running workflow")
	assert.False(t, ids["30"], "Should exclude task for completed workflow")
	assert.True(t, ids["40"], "Should include task for blocked workflow awaiting approval")
}

func TestTaskAtomicResetOrphan(t *testing.T) {
	store, closer := newTestStore(t, new(model.Task))
	defer closer()

	task := &model.Task{
		ID:      "orphan-test",
		Data:    []byte("test"),
		AgentID: 100,
	}
	assert.NoError(t, store.TaskInsert(task))

	// Reset should succeed when agent_id matches
	reset, err := store.TaskAtomicResetOrphan("orphan-test", 100)
	assert.NoError(t, err)
	assert.True(t, reset, "Should reset when agent_id matches")

	// Verify agent_id is now 0
	list, err := store.TaskList()
	assert.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, int64(0), list[0].AgentID)

	// Reset should fail when agent_id doesn't match (task is now unassigned)
	reset, err = store.TaskAtomicResetOrphan("orphan-test", 100)
	assert.NoError(t, err)
	assert.False(t, reset, "Should fail when agent_id doesn't match")

	// Reset should fail for non-existent task
	reset, err = store.TaskAtomicResetOrphan("non-existent", 100)
	assert.NoError(t, err)
	assert.False(t, reset, "Should fail for non-existent task")
}

func TestTaskAtomicResetOrphanRaceWithReassign(t *testing.T) {
	store, closer := newTestStore(t, new(model.Task))
	defer closer()

	task := &model.Task{
		ID:      "race-test",
		Data:    []byte("test"),
		AgentID: 100,
	}
	assert.NoError(t, store.TaskInsert(task))

	// Simulate: orphan reset reads agent_id=100, but before it writes,
	// the agent reconnected and the task was reassigned to agent 200
	reassigned, err := store.TaskAtomicAssign("race-test", 200)
	// TaskAtomicAssign requires agent_id=0, so this should fail (agent_id=100)
	assert.NoError(t, err)
	assert.False(t, reassigned, "Can't assign task that's already assigned")

	// Reset from the "old" agent should still work since agent_id=100
	reset, err := store.TaskAtomicResetOrphan("race-test", 100)
	assert.NoError(t, err)
	assert.True(t, reset)

	// Now task is unassigned, new agent can claim it
	reassigned, err = store.TaskAtomicAssign("race-test", 200)
	assert.NoError(t, err)
	assert.True(t, reassigned)

	// Orphan reset with old agent_id should now fail (CAS protects against clobber)
	reset, err = store.TaskAtomicResetOrphan("race-test", 100)
	assert.NoError(t, err)
	assert.False(t, reset, "CAS should prevent clobbering new assignment")

	// Verify task is assigned to agent 200
	list, err := store.TaskList()
	assert.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, int64(200), list[0].AgentID)
}

func TestTaskSetAndGetCancelled(t *testing.T) {
	store, closer := newTestStore(t, new(model.Task))
	defer closer()

	task := &model.Task{
		ID:   "cancel-test",
		Data: []byte("test"),
	}
	assert.NoError(t, store.TaskInsert(task))

	// Initially not canceled
	canceled, exists, err := store.TaskGetCancelled("cancel-test")
	assert.NoError(t, err)
	assert.True(t, exists, "Task should exist")
	assert.False(t, canceled, "Task should not be canceled initially")

	// Mark as canceled
	assert.NoError(t, store.TaskSetCancelled("cancel-test"))

	// Should now be canceled
	canceled, exists, err = store.TaskGetCancelled("cancel-test")
	assert.NoError(t, err)
	assert.True(t, exists, "Task should still exist")
	assert.True(t, canceled, "Task should be canceled")

	// Non-existent task should return exists=false
	canceled, exists, err = store.TaskGetCancelled("non-existent")
	assert.NoError(t, err)
	assert.False(t, exists, "Non-existent task should not exist")
	assert.False(t, canceled)
}

func TestTaskSetCancelledSetsTimestamp(t *testing.T) {
	store, closer := newTestStore(t, new(model.Task))
	defer closer()

	task := &model.Task{
		ID:   "timestamp-test",
		Data: []byte("test"),
	}
	assert.NoError(t, store.TaskInsert(task))

	before := time.Now().Unix()
	assert.NoError(t, store.TaskSetCancelled("timestamp-test"))
	after := time.Now().Unix()

	// Verify canceled_at was set
	list, err := store.TaskList()
	assert.NoError(t, err)
	assert.Len(t, list, 1)
	assert.NotZero(t, list[0].Canceled)
	assert.GreaterOrEqual(t, list[0].CancelledAt, before)
	assert.LessOrEqual(t, list[0].CancelledAt, after)
}

func TestTaskGetAgentID(t *testing.T) {
	store, closer := newTestStore(t, new(model.Task))
	defer closer()

	task := &model.Task{
		ID:      "agent-check",
		Data:    []byte("test"),
		AgentID: 42,
	}
	assert.NoError(t, store.TaskInsert(task))

	// Correct agent should match
	matched, err := store.TaskGetAgentID("agent-check", 42)
	assert.NoError(t, err)
	assert.True(t, matched, "Should match correct agent")

	// Wrong agent should not match
	matched, err = store.TaskGetAgentID("agent-check", 99)
	assert.NoError(t, err)
	assert.False(t, matched, "Should not match wrong agent")

	// Non-existent task should not match
	matched, err = store.TaskGetAgentID("non-existent", 42)
	assert.NoError(t, err)
	assert.False(t, matched, "Should not match non-existent task")
}

func TestTaskDeleteCancelledBefore(t *testing.T) {
	store, closer := newTestStore(t, new(model.Task))
	defer closer()

	now := time.Now().Unix()

	// Insert tasks with various canceled states
	assert.NoError(t, store.TaskInsert(&model.Task{ID: "active", Data: []byte("a")}))
	assert.NoError(t, store.TaskInsert(&model.Task{ID: "old-cancel", Data: []byte("b"), Canceled: 1, CancelledAt: now - 600}))
	assert.NoError(t, store.TaskInsert(&model.Task{ID: "recent-cancel", Data: []byte("c"), Canceled: 1, CancelledAt: now - 10}))
	assert.NoError(t, store.TaskInsert(&model.Task{ID: "not-canceled-but-has-ts", Data: []byte("d"), Canceled: 0, CancelledAt: now - 600}))

	// Delete canceled tasks older than 5 minutes ago
	cutoff := now - 300
	assert.NoError(t, store.TaskDeleteCancelledBefore(cutoff))

	// Verify only the old-cancel task was deleted
	list, err := store.TaskList()
	assert.NoError(t, err)
	assert.Len(t, list, 3, "Should have 3 tasks remaining")

	ids := make(map[string]bool)
	for _, task := range list {
		ids[task.ID] = true
	}
	assert.True(t, ids["active"], "Active task should remain")
	assert.False(t, ids["old-cancel"], "Old canceled task should be deleted")
	assert.True(t, ids["recent-cancel"], "Recent canceled task should remain")
	assert.True(t, ids["not-canceled-but-has-ts"], "Non-canceled task should remain even with old timestamp")
}

func TestTaskDeleteStale(t *testing.T) {
	store, closer := newTestStore(t, new(model.Task), new(model.Workflow))
	defer closer()

	// Create workflows with different states
	workflows := []*model.Workflow{
		{ID: 10, PipelineID: 1, PID: 1, Name: "pending-wf", State: model.StatusPending},
		{ID: 20, PipelineID: 1, PID: 2, Name: "running-wf", State: model.StatusRunning},
		{ID: 30, PipelineID: 1, PID: 3, Name: "success-wf", State: model.StatusSuccess},
		{ID: 40, PipelineID: 1, PID: 4, Name: "failure-wf", State: model.StatusFailure},
		{ID: 50, PipelineID: 1, PID: 5, Name: "skipped-wf", State: model.StatusSkipped},
		{ID: 60, PipelineID: 1, PID: 6, Name: "blocked-wf", State: model.StatusBlocked},
	}
	_, err := store.db.NewInsert().Model(&workflows).Exec(context.Background())
	assert.NoError(t, err)

	// Create tasks — some with active workflows, some stale, one orphaned (no workflow)
	assert.NoError(t, store.TaskInsert(&model.Task{ID: "10", Data: []byte("pending")}))
	assert.NoError(t, store.TaskInsert(&model.Task{ID: "20", Data: []byte("running")}))
	assert.NoError(t, store.TaskInsert(&model.Task{ID: "30", Data: []byte("success-stale")}))
	assert.NoError(t, store.TaskInsert(&model.Task{ID: "40", Data: []byte("failure-stale")}))
	assert.NoError(t, store.TaskInsert(&model.Task{ID: "50", Data: []byte("skipped-stale")}))
	assert.NoError(t, store.TaskInsert(&model.Task{ID: "60", Data: []byte("blocked-waiting-approval")}))
	assert.NoError(t, store.TaskInsert(&model.Task{ID: "999", Data: []byte("orphaned-no-workflow")}))

	// Delete stale tasks. Blocked workflows are paused awaiting approval —
	// their tasks must survive so dispatch can resume after ApproveWorkflow.
	deleted, err := store.TaskDeleteStale()
	assert.NoError(t, err)
	assert.Equal(t, int64(4), deleted, "Should delete 4 stale tasks (success + failure + skipped + orphaned); blocked must be preserved")

	// Pending, running, and blocked tasks should remain
	list, err := store.TaskList()
	assert.NoError(t, err)
	assert.Len(t, list, 3)

	ids := make(map[string]bool)
	for _, task := range list {
		ids[task.ID] = true
	}
	assert.True(t, ids["10"], "Pending workflow task should remain")
	assert.True(t, ids["20"], "Running workflow task should remain")
	assert.True(t, ids["60"], "Blocked workflow task awaiting approval must NOT be deleted")
}
