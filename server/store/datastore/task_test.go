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
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v5/server/model"
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
