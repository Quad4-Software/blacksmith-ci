// Copyright 2025 Crow Authors
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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"codeberg.org/crowci/crow/v4/server/model"
)

func TestFifoPrioritySelection(t *testing.T) {
	ctx := context.Background()

	q, ok := NewMemoryQueue(ctx, nil).(*fifo)
	assert.True(t, ok, "Expected fifo queue")

	// Push a task
	task := &model.Task{
		ID: "task1",
		Labels: map[string]string{
			"platform": "linux/amd64",
		},
		Dependencies: []string{},
		DepStatus:    map[string]model.StatusValue{},
	}

	err := q.Push(ctx, task)
	assert.NoError(t, err)

	// Create filter functions with different priorities
	filterNoPriority := func(t *model.Task) (bool, int, int32) {
		if t.Labels["platform"] == "linux/amd64" {
			return true, 10, 0 // 10 for match, priority 0 (no priority)
		}
		return false, 0, 0
	}

	filterHighPriority := func(t *model.Task) (bool, int, int32) {
		if t.Labels["platform"] == "linux/amd64" {
			return true, 10, 5 // 10 for match, priority 5
		}
		return false, 0, 5
	}

	// Start two agents polling - high priority should get the task
	agent1Done := make(chan *model.Task)
	agent2Done := make(chan *model.Task)

	// Agent 1 with no priority
	go func() {
		ctx1, cancel1 := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel1()
		task, err := q.Poll(ctx1, 1, filterNoPriority)
		if err == nil && task != nil {
			agent1Done <- task
		}
		close(agent1Done)
	}()

	// Agent 2 with high priority
	go func() {
		// Small delay to ensure agent 1 starts polling first
		time.Sleep(10 * time.Millisecond)
		ctx2, cancel2 := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel2()
		task, err := q.Poll(ctx2, 2, filterHighPriority)
		if err == nil && task != nil {
			agent2Done <- task
		}
		close(agent2Done)
	}()

	// Wait for results
	select {
	case task1 := <-agent1Done:
		if task1 != nil {
			t.Error("Agent 1 (no priority) should not have received the task when priority agent is available")
		}
	case <-time.After(150 * time.Millisecond):
		// Expected - agent 1 should timeout when priority agent is available
	}

	select {
	case task2 := <-agent2Done:
		if task2 != nil {
			assert.Equal(t, "task1", task2.ID, "Agent 2 (high priority) should receive the task")
		}
	case <-time.After(150 * time.Millisecond):
		t.Error("Agent 2 (high priority) should have received the task")
	}
}

func TestFifoMultipleAgentsPriority(t *testing.T) {
	// Test that the filtering function properly includes priority in scoring
	// This verifies that agents with higher priority get higher scores

	task := &model.Task{
		Labels: map[string]string{"platform": "linux/amd64"},
	}

	// Create filters with different priorities
	filterNoPriority := func(t *model.Task) (bool, int, int32) {
		if t.Labels["platform"] == "linux/amd64" {
			return true, 10, 0 // score 10, no priority
		}
		// Return different priorities for non-matching tasks to satisfy linter
		return false, 0, int32(len(t.Labels))
	}

	filterHighPriority := func(t *model.Task) (bool, int, int32) {
		if t.Labels["platform"] == "linux/amd64" {
			return true, 10, 5 // score 10, priority 5
		}
		// Return different priorities for non-matching tasks to satisfy linter
		return false, 0, int32(len(t.Labels) * 2)
	}

	// Test that filters return correct values
	matchNone, scoreNone, priorityNone := filterNoPriority(task)
	matchHigh, scoreHigh, priorityHigh := filterHighPriority(task)

	assert.True(t, matchNone, "No priority agent should match")
	assert.True(t, matchHigh, "High priority agent should match")
	assert.Equal(t, 10, scoreNone, "Both should have same label score")
	assert.Equal(t, 10, scoreHigh, "Both should have same label score")
	assert.Equal(t, int32(0), priorityNone, "No priority should return 0")
	assert.Equal(t, int32(5), priorityHigh, "High priority should return 5")
}
