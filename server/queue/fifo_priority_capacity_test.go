package queue

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
)

func TestFifoPriorityWithCapacity(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	queue := NewMemoryQueue(ctx, nil)
	q, ok := queue.(*fifo)
	assert.True(t, ok, "Queue should be of type *fifo")

	// Create 3 tasks
	task1 := &model.Task{
		ID:        "task1",
		RunOn:     []string{},
		DepStatus: make(map[string]model.StatusValue),
	}
	task2 := &model.Task{
		ID:        "task2",
		RunOn:     []string{},
		DepStatus: make(map[string]model.StatusValue),
	}
	task3 := &model.Task{
		ID:        "task3",
		RunOn:     []string{},
		DepStatus: make(map[string]model.StatusValue),
	}

	// Push all tasks
	assert.NoError(t, q.Push(ctx, task1))
	assert.NoError(t, q.Push(ctx, task2))
	assert.NoError(t, q.Push(ctx, task3))

	// Create filter function that returns priority
	highPriorityFilter := func(task *model.Task) (bool, int, int32) {
		return true, 0, 10 // high priority
	}
	lowPriorityFilter := func(task *model.Task) (bool, int, int32) {
		return true, 0, 1 // low priority
	}

	// Agent 1: High priority (10) with capacity 4
	// Create 4 workers for agent 1
	var wg sync.WaitGroup
	agent1Tasks := make(chan *model.Task, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, err := q.Poll(ctx, 1, highPriorityFilter)
			if err == nil && task != nil {
				agent1Tasks <- task
			}
		}()
	}

	// Agent 2: Low priority (1) with capacity 2
	// Create 2 workers for agent 2
	agent2Tasks := make(chan *model.Task, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, err := q.Poll(ctx, 2, lowPriorityFilter)
			if err == nil && task != nil {
				agent2Tasks <- task
			}
		}()
	}

	// Wait a bit for workers to register
	time.Sleep(200 * time.Millisecond)

	// Collect results
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel(nil)
	}()

	// Wait for all goroutines
	wg.Wait()
	close(agent1Tasks)
	close(agent2Tasks)

	// Count tasks assigned to each agent
	agent1Count := 0
	for task := range agent1Tasks {
		assert.NotNil(t, task)
		agent1Count++
		t.Logf("Agent 1 (high priority) got task: %s", task.ID)
	}

	agent2Count := 0
	for task := range agent2Tasks {
		assert.NotNil(t, task)
		agent2Count++
		t.Logf("Agent 2 (low priority) got task: %s", task.ID)
	}

	// Agent 1 with higher priority and 4 slots should get all 3 tasks
	assert.Equal(t, 3, agent1Count, "High priority agent with capacity 4 should get all 3 tasks")
	assert.Equal(t, 0, agent2Count, "Low priority agent should get 0 tasks when high priority agent has capacity")
}

func TestFifoPrioritySamePriorityCapacityDistribution(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	queue := NewMemoryQueue(ctx, nil)
	q, ok := queue.(*fifo)
	assert.True(t, ok, "Queue should be of type *fifo")

	// Create 5 tasks
	for i := 1; i <= 5; i++ {
		task := &model.Task{
			ID:        string(rune('0' + i)),
			RunOn:     []string{},
			DepStatus: make(map[string]model.StatusValue),
		}
		assert.NoError(t, q.Push(ctx, task))
	}

	// Same priority filter for both agents
	samePriorityFilter := func(task *model.Task) (bool, int, int32) {
		return true, 0, 5 // same priority
	}

	// Agent 1: Priority 5 with capacity 3
	var wg sync.WaitGroup
	agent1Tasks := make(chan *model.Task, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, err := q.Poll(ctx, 1, samePriorityFilter)
			if err == nil && task != nil {
				agent1Tasks <- task
			}
		}()
	}

	// Agent 2: Priority 5 with capacity 2
	agent2Tasks := make(chan *model.Task, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, err := q.Poll(ctx, 2, samePriorityFilter)
			if err == nil && task != nil {
				agent2Tasks <- task
			}
		}()
	}

	// Wait a bit for workers to register
	time.Sleep(200 * time.Millisecond)

	// Collect results
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel(nil)
	}()

	// Wait for all goroutines
	wg.Wait()
	close(agent1Tasks)
	close(agent2Tasks)

	// Count tasks assigned to each agent
	agent1Count := 0
	for task := range agent1Tasks {
		assert.NotNil(t, task)
		agent1Count++
		t.Logf("Agent 1 got task: %s", task.ID)
	}

	agent2Count := 0
	for task := range agent2Tasks {
		assert.NotNil(t, task)
		agent2Count++
		t.Logf("Agent 2 got task: %s", task.ID)
	}

	// With same priority, agent with more capacity (3 workers) should get more tasks
	// Agent 1 should get 3 tasks, Agent 2 should get 2 tasks
	assert.Equal(t, 3, agent1Count, "Agent 1 with 3 workers should get 3 tasks")
	assert.Equal(t, 2, agent2Count, "Agent 2 with 2 workers should get 2 tasks")
	assert.Equal(t, 5, agent1Count+agent2Count, "All 5 tasks should be assigned")
}
