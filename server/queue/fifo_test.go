// Copyright 2023 Woodpecker Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
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
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v6/server/model"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

var (
	filterFnTrue = func(*model.Task) (bool, int, int32) { return true, 1, 0 }
	genDummyTask = func() *model.Task {
		return &model.Task{
			ID:   "1",
			Data: []byte("{}"),
		}
	}
	waitForProcess = func() { time.Sleep(processTimeInterval + 50*time.Millisecond) }
)

func TestFifo(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	q := NewMemoryQueue(ctx, nil)
	dummyTask := genDummyTask()

	assert.NoError(t, q.Push(ctx, dummyTask))
	waitForProcess()
	info := q.Info(ctx)
	assert.Len(t, info.Pending, 1, "expect task in pending queue")

	got, err := q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, dummyTask, got)

	waitForProcess()
	info = q.Info(ctx)
	assert.Len(t, info.Pending, 0, "expect task removed from pending queue")
	assert.Len(t, info.Running, 1, "expect task in running queue")

	assert.NoError(t, q.Done(ctx, got.ID, model.StatusSuccess))

	waitForProcess()
	info = q.Info(ctx)
	assert.Len(t, info.Pending, 0, "expect task removed from pending queue")
	assert.Len(t, info.Running, 0, "expect task removed from running queue")
}

func TestFinalizeZombieWorkflowKillsPreparingStep(t *testing.T) {
	workflow := &model.Workflow{ID: 42, PipelineID: 7, State: model.StatusRunning}
	preparingStep := &model.Step{ID: 11, State: model.StatusPreparing}
	successfulStep := &model.Step{ID: 12, State: model.StatusSuccess}
	pipeline := &model.Pipeline{ID: 7, Status: model.StatusRunning}

	mockStore := new(store_mocks.MockStore)
	mockStore.On("WorkflowLoad", int64(42)).Return(workflow, nil)
	mockStore.On("StepListFromWorkflowFind", workflow).Return([]*model.Step{preparingStep, successfulStep}, nil)
	mockStore.On("StepUpdate", preparingStep).Return(nil)
	mockStore.On("WorkflowUpdate", workflow).Return(nil)
	mockStore.On("GetPipeline", int64(7)).Return(pipeline, nil)
	mockStore.On("WorkflowGetTree", pipeline).Return([]*model.Workflow{workflow}, nil)
	mockStore.On("UpdatePipeline", mock.AnythingOfType("*model.Pipeline")).Return(nil)

	q := &fifo{store: mockStore}
	q.finalizeZombieWorkflow("42")

	assert.Equal(t, model.StatusKilled, preparingStep.State)
	assert.NotZero(t, preparingStep.Started)
	assert.NotZero(t, preparingStep.Finished)
	assert.Equal(t, model.StatusSuccess, successfulStep.State)
	mockStore.AssertExpectations(t)
}

func TestFifoExpire(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)
	assert.NotNil(t, q)

	dummyTask := genDummyTask()

	q.extension = 0
	assert.NoError(t, q.Push(ctx, dummyTask))
	waitForProcess()
	info := q.Info(ctx)
	assert.Len(t, info.Pending, 1, "expect task in pending queue")

	got, err := q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, dummyTask, got)

	waitForProcess()
	// After deadline expires, task becomes orphaned but stays in running queue
	// during adoption grace period (waiting for agent to reclaim)
	info = q.Info(ctx)
	assert.Len(t, info.Running, 1, "expect task still in running queue (orphaned, waiting for adoption)")
	assert.Len(t, info.Pending, 0, "expect pending queue empty during adoption grace period")
}

func TestFifoWait(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)
	assert.NotNil(t, q)

	dummyTask := genDummyTask()

	assert.NoError(t, q.Push(ctx, dummyTask))

	waitForProcess()
	got, err := q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, dummyTask, got)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		assert.NoError(t, q.Wait(ctx, got.ID))
		wg.Done()
	}()

	<-time.After(time.Millisecond)
	assert.NoError(t, q.Done(ctx, got.ID, model.StatusSuccess))
	wg.Wait()
}

func TestFifoEvict(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	q := NewMemoryQueue(ctx, nil)
	dummyTask := genDummyTask()

	assert.NoError(t, q.Push(ctx, dummyTask))

	waitForProcess()
	info := q.Info(ctx)
	assert.Len(t, info.Pending, 1, "expect task in pending queue")

	err := q.Evict(ctx, dummyTask.ID)
	assert.NoError(t, err)

	waitForProcess()
	info = q.Info(ctx)
	assert.Len(t, info.Pending, 0)

	err = q.Evict(ctx, dummyTask.ID)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFifoDependencies(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	task1 := genDummyTask()
	task2 := &model.Task{
		ID:           "2",
		Dependencies: []string{"1"},
		DepStatus:    make(map[string]model.StatusValue),
	}

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)
	assert.NotNil(t, q)

	assert.NoError(t, q.PushAtOnce(ctx, []*model.Task{task2, task1}))

	waitForProcess()
	got, err := q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, task1, got)

	waitForProcess()
	assert.NoError(t, q.Done(ctx, got.ID, model.StatusSuccess))

	waitForProcess()
	got, err = q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, task2, got)
}

func TestFifoErrors(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	task1 := genDummyTask()
	task2 := &model.Task{
		ID:           "2",
		Dependencies: []string{"1"},
		DepStatus:    make(map[string]model.StatusValue),
	}
	task3 := &model.Task{
		ID:           "3",
		Dependencies: []string{"1"},
		DepStatus:    make(map[string]model.StatusValue),
		RunOn:        []string{"success", "failure"},
	}

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)
	assert.NotNil(t, q)

	assert.NoError(t, q.PushAtOnce(ctx, []*model.Task{task2, task3, task1}))

	waitForProcess()
	got, err := q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, task1, got)

	assert.NoError(t, q.Error(ctx, got.ID, fmt.Errorf("exit code 1, there was an error")))

	waitForProcess()
	got, err = q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, task2, got)
	assert.False(t, got.ShouldRun())

	waitForProcess()
	got, err = q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, task3, got)
	assert.True(t, got.ShouldRun())
}

func TestFifoErrors2(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	task1 := genDummyTask()
	task2 := &model.Task{
		ID: "2",
	}
	task3 := &model.Task{
		ID:           "3",
		Dependencies: []string{"1", "2"},
		DepStatus:    make(map[string]model.StatusValue),
	}

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)
	assert.NotNil(t, q)

	assert.NoError(t, q.PushAtOnce(ctx, []*model.Task{task2, task3, task1}))

	for i := 0; i < 2; i++ {
		waitForProcess()
		got, err := q.Poll(ctx, 1, filterFnTrue)
		assert.NoError(t, err)
		assert.False(t, got != task1 && got != task2, "expect task1 or task2 returned from queue as task3 depends on them")

		if got != task1 {
			assert.NoError(t, q.Done(ctx, got.ID, model.StatusSuccess))
		}
		if got != task2 {
			assert.NoError(t, q.Error(ctx, got.ID, fmt.Errorf("exit code 1, there was an error")))
		}
	}

	waitForProcess()
	got, err := q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, task3, got)
	assert.False(t, got.ShouldRun())
}

func TestFifoErrorsMultiThread(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	task1 := genDummyTask()
	task2 := &model.Task{
		ID:           "2",
		Dependencies: []string{"1"},
		DepStatus:    make(map[string]model.StatusValue),
	}
	task3 := &model.Task{
		ID:           "3",
		Dependencies: []string{"1", "2"},
		DepStatus:    make(map[string]model.StatusValue),
	}

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)
	assert.NotNil(t, q)

	assert.NoError(t, q.PushAtOnce(ctx, []*model.Task{task2, task3, task1}))

	obtainedWorkCh := make(chan *model.Task)
	defer func() { close(obtainedWorkCh) }()

	for i := 0; i < 10; i++ {
		go func(i int) {
			for {
				fmt.Printf("Worker %d started\n", i)
				got, err := q.Poll(ctx, 1, filterFnTrue)
				if err != nil && errors.Is(err, context.Canceled) {
					return
				}
				assert.NoError(t, err)
				obtainedWorkCh <- got
			}
		}(i)
	}

	task1Processed := false
	task2Processed := false

	for {
		select {
		case got := <-obtainedWorkCh:
			fmt.Println(got.ID)

			switch {
			case !task1Processed:
				assert.Equal(t, task1, got)
				task1Processed = true
				assert.NoError(t, q.Error(ctx, got.ID, fmt.Errorf("exit code 1, there was an error")))
				go func() {
					for {
						fmt.Printf("Worker spawned\n")
						got, err := q.Poll(ctx, 1, filterFnTrue)
						if err != nil && errors.Is(err, context.Canceled) {
							return
						}
						assert.NoError(t, err)
						obtainedWorkCh <- got
					}
				}()
			case !task2Processed:
				assert.Equal(t, task2, got)
				task2Processed = true
				assert.NoError(t, q.Done(ctx, got.ID, model.StatusSuccess))
				go func() {
					for {
						fmt.Printf("Worker spawned\n")
						got, err := q.Poll(ctx, 1, filterFnTrue)
						if err != nil && errors.Is(err, context.Canceled) {
							return
						}
						assert.NoError(t, err)
						obtainedWorkCh <- got
					}
				}()
			default:
				assert.Equal(t, task3, got)
				assert.False(t, got.ShouldRun(), "expect task3 should not run, task1 succeeded but task2 failed")
				return
			}

		case <-time.After(5 * time.Second):
			info := q.Info(ctx)
			fmt.Println(info.String())
			t.Errorf("test timed out")
			return
		}
	}
}

func TestFifoTransitiveErrors(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	task1 := genDummyTask()
	task2 := &model.Task{
		ID:           "2",
		Dependencies: []string{"1"},
		DepStatus:    make(map[string]model.StatusValue),
	}
	task3 := &model.Task{
		ID:           "3",
		Dependencies: []string{"2"},
		DepStatus:    make(map[string]model.StatusValue),
	}

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)
	assert.NotNil(t, q)

	assert.NoError(t, q.PushAtOnce(ctx, []*model.Task{task2, task3, task1}))

	waitForProcess()
	got, err := q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, task1, got)
	assert.NoError(t, q.Error(ctx, got.ID, fmt.Errorf("exit code 1, there was an error")))

	waitForProcess()
	got, err = q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, task2, got)
	assert.False(t, got.ShouldRun(), "expect task2 should not run, since task1 failed")
	assert.NoError(t, q.Done(ctx, got.ID, model.StatusSkipped))

	waitForProcess()
	got, err = q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, task3, got)
	assert.False(t, got.ShouldRun(), "expect task3 should not run, task1 failed, thus task2 was skipped, task3 should be skipped too")
}

func TestFifoCancel(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	task1 := genDummyTask()
	task2 := &model.Task{
		ID:           "2",
		Dependencies: []string{"1"},
		DepStatus:    make(map[string]model.StatusValue),
	}
	task3 := &model.Task{
		ID:           "3",
		Dependencies: []string{"1"},
		DepStatus:    make(map[string]model.StatusValue),
		RunOn:        []string{"success", "failure"},
	}

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)
	assert.NotNil(t, q)

	assert.NoError(t, q.PushAtOnce(ctx, []*model.Task{task2, task3, task1}))

	// task1 has no deps and is polled into running; task2 and task3 depend on
	// task1 and therefore sit in waitingOnDeps.
	_, _ = q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, q.Error(ctx, task1.ID, fmt.Errorf("canceled")))
	assert.NoError(t, q.Error(ctx, task2.ID, fmt.Errorf("canceled")))
	assert.NoError(t, q.Error(ctx, task3.ID, fmt.Errorf("canceled")))
	info := q.Info(ctx)
	assert.Len(t, info.Pending, 0, "all pipelines should be canceled")

	// Canceled tasks must be fully removed from every internal list. Previously
	// tasks waiting on dependencies were left in waitingOnDeps and got promoted
	// back to pending once their (canceled) dependency left the queue, so an
	// agent would still execute them — see issue #1147.
	time.Sleep(processTimeInterval * 2)
	info = q.Info(ctx)
	assert.Len(t, info.Pending, 0, "canceled tasks must not be rescheduled")
	assert.Len(t, info.Running, 0, "canceled tasks must not be rescheduled")
	assert.Len(t, info.WaitingOnDeps, 0, "canceled tasks must not be rescheduled")
}

func TestFifoPause(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)
	assert.NotNil(t, q)

	dummyTask := genDummyTask()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		_, _ = q.Poll(ctx, 1, filterFnTrue)
		wg.Done()
	}()

	q.Pause()
	t0 := time.Now()
	assert.NoError(t, q.Push(ctx, dummyTask))
	waitForProcess()
	q.Resume()

	wg.Wait()
	t1 := time.Now()

	assert.Greater(t, t1.Sub(t0), 20*time.Millisecond, "should have waited til resume")

	q.Pause()
	assert.NoError(t, q.Push(ctx, dummyTask))
	q.Resume()
	_, _ = q.Poll(ctx, 1, filterFnTrue)
}

func TestFifoPauseResume(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)
	assert.NotNil(t, q)

	dummyTask := genDummyTask()

	q.Pause()
	assert.NoError(t, q.Push(ctx, dummyTask))
	q.Resume()

	_, _ = q.Poll(ctx, 1, filterFnTrue)
}

func TestWaitingVsPending(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	task1 := genDummyTask()
	task2 := &model.Task{
		ID:           "2",
		Dependencies: []string{"1"},
		DepStatus:    make(map[string]model.StatusValue),
	}
	task3 := &model.Task{
		ID:           "3",
		Dependencies: []string{"1"},
		DepStatus:    make(map[string]model.StatusValue),
		RunOn:        []string{"success", "failure"},
	}

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)
	assert.NotNil(t, q)

	assert.NoError(t, q.PushAtOnce(ctx, []*model.Task{task2, task3, task1}))

	got, _ := q.Poll(ctx, 1, filterFnTrue)

	waitForProcess()
	info := q.Info(ctx)
	assert.Equal(t, 2, info.Stats.WaitingOnDeps)

	assert.NoError(t, q.Error(ctx, got.ID, fmt.Errorf("exit code 1, there was an error")))
	got, err := q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.EqualValues(t, task2, got)

	waitForProcess()
	info = q.Info(ctx)
	assert.Equal(t, 0, info.Stats.WaitingOnDeps)
	assert.Equal(t, 1, info.Stats.Pending)
}

// TestFifoFairOrderingAfterWaiting reproduces issue #1208: a workflow that is
// briefly held in waitingOnDeps must not lose its queue position to pipelines
// that were enqueued later. The older workflow must be served first (FIFO).
func TestFifoFairOrderingAfterWaiting(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)
	assert.NotNil(t, q)

	// Older pipeline: task1 plus task2 which depends on task1.
	task1 := &model.Task{ID: "1", Data: []byte("{}")}
	task2 := &model.Task{
		ID:           "2",
		Dependencies: []string{"1"},
		DepStatus:    make(map[string]model.StatusValue),
		Data:         []byte("{}"),
	}
	assert.NoError(t, q.PushAtOnce(ctx, []*model.Task{task1, task2}))

	// Drain task1 so task2 is held in waitingOnDeps.
	waitForProcess()
	got, err := q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, "1", got.ID)

	waitForProcess()
	info := q.Info(ctx)
	assert.Equal(t, 1, info.Stats.WaitingOnDeps, "task2 should wait on task1")

	// A newer pipeline arrives while task2 is still blocked.
	task3 := &model.Task{ID: "3", Data: []byte("{}")}
	assert.NoError(t, q.Push(ctx, task3))

	// task1 finishes, unblocking task2.
	assert.NoError(t, q.Done(ctx, "1", model.StatusSuccess))

	// The older pipeline's workflow (task2) must be served before the newer
	// pipeline's workflow (task3).
	waitForProcess()
	got, err = q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, "2", got.ID, "older queued workflow must run before a newer one")
}

func TestShouldRun(t *testing.T) {
	task := &model.Task{
		ID:           "2",
		Dependencies: []string{"1"},
		DepStatus: map[string]model.StatusValue{
			"1": model.StatusSuccess,
		},
		RunOn: []string{"failure"},
	}
	assert.False(t, task.ShouldRun(), "expect task to not run, it runs on failure only")

	task = &model.Task{
		ID:           "2",
		Dependencies: []string{"1"},
		DepStatus: map[string]model.StatusValue{
			"1": model.StatusSuccess,
		},
		RunOn: []string{"failure", "success"},
	}
	assert.True(t, task.ShouldRun())

	task = &model.Task{
		ID:           "2",
		Dependencies: []string{"1"},
		DepStatus: map[string]model.StatusValue{
			"1": model.StatusFailure,
		},
	}
	assert.False(t, task.ShouldRun())

	task = &model.Task{
		ID:           "2",
		Dependencies: []string{"1"},
		DepStatus: map[string]model.StatusValue{
			"1": model.StatusSuccess,
		},
		RunOn: []string{"success"},
	}
	assert.True(t, task.ShouldRun())

	task = &model.Task{
		ID:           "2",
		Dependencies: []string{"1"},
		DepStatus: map[string]model.StatusValue{
			"1": model.StatusFailure,
		},
		RunOn: []string{"failure"},
	}
	assert.True(t, task.ShouldRun())

	task = &model.Task{
		ID:           "2",
		Dependencies: []string{"1"},
		DepStatus: map[string]model.StatusValue{
			"1": model.StatusSkipped,
		},
	}
	assert.False(t, task.ShouldRun(), "task should not run if dependency is skipped")

	task = &model.Task{
		ID:           "2",
		Dependencies: []string{"1"},
		DepStatus: map[string]model.StatusValue{
			"1": model.StatusSkipped,
		},
		RunOn: []string{"failure"},
	}
	assert.True(t, task.ShouldRun(), "on failure, tasks should run on skipped deps, something failed higher up the chain")
}

func TestFifoWithScoring(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	q := NewMemoryQueue(ctx, nil)

	// Create tasks with different labels
	tasks := []*model.Task{
		{ID: "1", Labels: map[string]string{"org-id": "123", "platform": "linux"}},
		{ID: "2", Labels: map[string]string{"org-id": "456", "platform": "linux"}},
		{ID: "3", Labels: map[string]string{"org-id": "789", "platform": "windows"}},
		{ID: "4", Labels: map[string]string{"org-id": "123", "platform": "linux"}},
		{ID: "5", Labels: map[string]string{"org-id": "*", "platform": "linux"}},
	}

	assert.NoError(t, q.PushAtOnce(ctx, tasks))

	// Create filter functions for different workers
	filters := map[int]FilterFn{
		1: func(task *model.Task) (bool, int, int32) {
			if task.Labels["org-id"] == "123" {
				return true, 20, 0
			}
			if task.Labels["platform"] == "linux" {
				return true, 10, 0
			}
			return true, 1, 0
		},
		2: func(task *model.Task) (bool, int, int32) {
			if task.Labels["org-id"] == "456" {
				return true, 20, 0
			}
			if task.Labels["platform"] == "linux" {
				return true, 10, 0
			}
			return true, 1, 0
		},
		3: func(task *model.Task) (bool, int, int32) {
			if task.Labels["platform"] == "windows" {
				return true, 20, 0
			}
			return true, 1, 0
		},
		4: func(task *model.Task) (bool, int, int32) {
			if task.Labels["org-id"] == "123" {
				return true, 20, 0
			}
			if task.Labels["platform"] == "linux" {
				return true, 10, 0
			}
			return true, 1, 0
		},
		5: func(task *model.Task) (bool, int, int32) {
			if task.Labels["org-id"] == "*" {
				return true, 15, 0
			}
			return true, 1, 0
		},
	}

	// Start polling in separate goroutines
	results := make(chan *model.Task, 5)
	for i := 1; i <= 5; i++ {
		go func(n int) {
			task, err := q.Poll(ctx, int64(n), filters[n])
			assert.NoError(t, err)
			results <- task
		}(i)
	}

	// Collect results
	receivedTasks := make(map[string]int64)
	for i := 0; i < 5; i++ {
		select {
		case task := <-results:
			receivedTasks[task.ID] = task.AgentID
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for tasks")
		}
	}

	assert.Len(t, receivedTasks, 5, "All tasks should be assigned")

	// Define possible agent assignments (any matching agent can be selected randomly)
	// All agents can handle all tasks with different scores, but all return true from filter
	possibleAssignments := map[string][]int64{
		"1": {1, 2, 3, 4, 5}, // All agents can handle this task
		"2": {1, 2, 3, 4, 5}, // All agents can handle this task
		"3": {1, 2, 3, 4, 5}, // All agents can handle this task
		"4": {1, 2, 3, 4, 5}, // All agents can handle this task
		"5": {1, 2, 3, 4, 5}, // All agents can handle this task
	}

	// Check that tasks are assigned to agents that can handle them
	// With random selection, any matching agent can be selected
	for taskID, possibleAgents := range possibleAssignments {
		agentID, ok := receivedTasks[taskID]
		assert.True(t, ok, "Task %s should be assigned", taskID)
		assert.Contains(t, possibleAgents, agentID, "Task %s should be assigned to an agent that can handle it", taskID)
	}
}

func TestFifo_GetRunningWorkflowIDs(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	q := NewMemoryQueue(ctx, nil)

	t.Run("empty queue", func(t *testing.T) {
		ids := q.GetRunningWorkflowIDs()
		assert.Empty(t, ids)
	})

	t.Run("with running workflows", func(t *testing.T) {
		// Add some tasks to the queue
		task1 := &model.Task{ID: "1", Labels: map[string]string{}}
		task2 := &model.Task{ID: "2", Labels: map[string]string{}}
		task3 := &model.Task{ID: "3", Labels: map[string]string{}}

		err := q.PushAtOnce(ctx, []*model.Task{task1, task2, task3})
		assert.NoError(t, err)

		// Poll tasks to make them running
		filter := func(_ *model.Task) (bool, int, int32) {
			return true, 1, 0
		}

		// Poll all tasks
		_, err = q.Poll(ctx, 1, filter)
		assert.NoError(t, err)
		_, err = q.Poll(ctx, 1, filter)
		assert.NoError(t, err)
		_, err = q.Poll(ctx, 1, filter)
		assert.NoError(t, err)

		// Check running workflow IDs
		ids := q.GetRunningWorkflowIDs()
		assert.Len(t, ids, 3)
		assert.Contains(t, ids, "1")
		assert.Contains(t, ids, "2")
		assert.Contains(t, ids, "3")

		// Complete one workflow
		err = q.Done(ctx, "1", model.StatusSuccess)
		assert.NoError(t, err)

		// Check running workflow IDs again
		ids = q.GetRunningWorkflowIDs()
		assert.Len(t, ids, 2)
		assert.NotContains(t, ids, "1")
		assert.Contains(t, ids, "2")
		assert.Contains(t, ids, "3")
	})

	t.Run("after queue pause and resume", func(t *testing.T) {
		var err error
		// Complete previous running workflows before adding the new one
		err = q.Done(ctx, "2", model.StatusSuccess)
		assert.NoError(t, err)
		err = q.Done(ctx, "3", model.StatusSuccess)
		assert.NoError(t, err)

		// Add a task
		task := &model.Task{ID: "4", Labels: map[string]string{}}
		err = q.Push(ctx, task)
		assert.NoError(t, err)

		// Poll to make it running
		filter := func(_ *model.Task) (bool, int, int32) {
			return true, 1, 0
		}
		_, err = q.Poll(ctx, 1, filter)
		assert.NoError(t, err)

		// Pause queue
		q.Pause()

		// Check running workflow IDs
		ids := q.GetRunningWorkflowIDs()
		assert.Len(t, ids, 1)
		assert.Contains(t, ids, "4")

		// Resume queue
		q.Resume()

		// Check running workflow IDs again
		ids = q.GetRunningWorkflowIDs()
		assert.Len(t, ids, 1)
		assert.Contains(t, ids, "4")
	})
}

func TestFifoReclaim(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)
	assert.NotNil(t, q)

	task := &model.Task{
		ID:      "reclaim-1",
		AgentID: 42,
		Data:    []byte("{}"),
	}

	// Reclaim a task (simulating agent restart)
	err := q.Reclaim(ctx, task)
	assert.NoError(t, err)

	waitForProcess()
	info := q.Info(ctx)
	assert.Len(t, info.Running, 1, "expect task in running queue after reclaim")
	assert.Equal(t, "reclaim-1", info.Running[0].ID)

	// Verify we can extend the reclaimed task
	err = q.Extend(ctx, 42, "reclaim-1")
	assert.NoError(t, err)

	// Verify we can complete the reclaimed task
	err = q.Done(ctx, "reclaim-1", model.StatusSuccess)
	assert.NoError(t, err)

	waitForProcess()
	info = q.Info(ctx)
	assert.Len(t, info.Running, 0, "expect task removed after done")
}

func TestFifoReclaimRemovesFromPending(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)
	assert.NotNil(t, q)

	task := &model.Task{
		ID:      "reclaim-2",
		AgentID: 42,
		Data:    []byte("{}"),
	}

	// Push task to pending first
	assert.NoError(t, q.Push(ctx, task))
	waitForProcess()
	info := q.Info(ctx)
	assert.Len(t, info.Pending, 1, "expect task in pending queue")

	// Reclaim should move it from pending to running
	err := q.Reclaim(ctx, task)
	assert.NoError(t, err)

	waitForProcess()
	info = q.Info(ctx)
	assert.Len(t, info.Pending, 0, "expect task removed from pending")
	assert.Len(t, info.Running, 1, "expect task in running queue")
}

func TestFifoOrphanedTaskGracePeriod(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)
	assert.NotNil(t, q)

	task := &model.Task{
		ID:      "orphan-1",
		AgentID: 42,
		Data:    []byte("{}"),
	}

	// Set very short extension for testing
	q.extension = 10 * time.Millisecond

	assert.NoError(t, q.Push(ctx, task))
	waitForProcess()

	// Poll to move to running
	got, err := q.Poll(ctx, 42, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, task.ID, got.ID)

	waitForProcess()
	info := q.Info(ctx)
	assert.Len(t, info.Running, 1, "expect task in running queue")

	// Wait for deadline to expire
	time.Sleep(50 * time.Millisecond)

	// Task should be orphaned but still in running (grace period)
	q.Lock()
	entry := q.running["orphan-1"]
	q.Unlock()
	assert.NotNil(t, entry, "expect task still in running during grace period")
}

func TestFifoAwaitingApproval(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)

	gated := &model.Task{
		ID:               "gated",
		Data:             []byte("{}"),
		RequiresApproval: true,
		ApprovalState:    model.ApprovalPending,
	}
	free := &model.Task{
		ID:   "free",
		Data: []byte("{}"),
	}
	assert.NoError(t, q.PushAtOnce(ctx, []*model.Task{gated, free}))

	waitForProcess()
	info := q.Info(ctx)
	// The gated task must be held in waitingOnDeps (1) and only the free task
	// available for polling.
	assert.Equal(t, 1, info.Stats.WaitingOnDeps, "expect gated task in waitingOnDeps")
	assert.Len(t, info.Pending, 1, "expect free task pending")

	got, err := q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, "free", got.ID)
}

func TestFifoSetApprovalStateReleases(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)

	gated := &model.Task{
		ID:               "gated",
		Data:             []byte("{}"),
		RequiresApproval: true,
		ApprovalState:    model.ApprovalPending,
	}
	assert.NoError(t, q.Push(ctx, gated))
	waitForProcess()
	info := q.Info(ctx)
	assert.Equal(t, 1, info.Stats.WaitingOnDeps)

	assert.NoError(t, q.SetApprovalState(ctx, "gated", model.ApprovalApproved))

	waitForProcess()
	got, err := q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, "gated", got.ID)
}

// TestFifoBlockOnApprovalHookFiresWhenDepsClear verifies that the queue
// invokes the injected BlockOnApprovalFn exactly once when an
// ApprovalNone-gated task's upstream deps finish, and that the task's
// in-queue ApprovalState flips to Pending so subsequent passes keep gating
// it.
func TestFifoBlockOnApprovalHookFiresWhenDepsClear(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)
	assert.NotNil(t, q)

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

	// task1 is the upstream dep. task2 requires approval but starts in
	// ApprovalNone — the hook should fire when task1 completes.
	task1 := &model.Task{
		ID:   "1",
		Data: []byte("{}"),
	}
	task2 := &model.Task{
		ID:               "2",
		Data:             []byte("{}"),
		Dependencies:     []string{"1"},
		DepStatus:        make(map[string]model.StatusValue),
		RequiresApproval: true,
		ApprovalState:    model.ApprovalNone,
	}

	assert.NoError(t, q.PushAtOnce(ctx, []*model.Task{task1, task2}))

	// While task1 is in queue, the gate should NOT activate — deps haven't
	// cleared yet.
	waitForProcess()
	assert.Equal(t, int32(0), hookCalls.Load(),
		"hook should not fire while upstream dep is still queued")

	// Drain task1.
	got, err := q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, "1", got.ID)
	assert.NoError(t, q.Done(ctx, "1", model.StatusSuccess))

	// Wait for the gate activation. Use a generous timeout to absorb the
	// goroutine-scheduling jitter of the async hook invocation.
	select {
	case <-hookFiredCh:
	case <-time.After(2 * time.Second):
		t.Fatal("BlockOnApprovalFn was not invoked after deps cleared")
	}

	assert.Equal(t, "2", hookTaskID.Load(), "hook should fire for the gated task")

	// task2 must remain held — local ApprovalState flipped to Pending.
	waitForProcess()
	info := q.Info(ctx)
	assert.Equal(t, 1, info.Stats.WaitingOnDeps,
		"gated task should remain in waitingOnDeps after activation")
	assert.Empty(t, info.Pending, "gated task must not be dispatchable")

	// Verify in-queue ApprovalState mirror.
	q.Lock()
	var liveTask *model.Task
	for e := q.waitingOnDeps.Front(); e != nil; e = e.Next() {
		if t, _ := e.Value.(*model.Task); t.ID == "2" {
			liveTask = t
			break
		}
	}
	q.Unlock()
	assert.NotNil(t, liveTask, "task2 must be present in waitingOnDeps")
	assert.Equal(t, model.ApprovalPending, liveTask.ApprovalState,
		"queue must mirror ApprovalPending so future passes keep gating the task")

	// Subsequent filter passes must not re-fire the hook — state is no
	// longer ApprovalNone, so activation is idempotent.
	waitForProcess()
	waitForProcess()
	assert.Equal(t, int32(1), hookCalls.Load(),
		"BlockOnApprovalFn must fire exactly once per activation")
}

// TestFifoBlockOnApprovalHookNotFiredForUngatedTask verifies the hook is
// never invoked for tasks that don't require approval.
func TestFifoBlockOnApprovalHookNotFiredForUngatedTask(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)

	var hookCalls atomic.Int32
	q.SetBlockOnApprovalHook(func(_ context.Context, _ string) {
		hookCalls.Add(1)
	})

	ungated := genDummyTask()
	assert.NoError(t, q.Push(ctx, ungated))

	waitForProcess()
	got, err := q.Poll(ctx, 1, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, "1", got.ID)

	assert.Equal(t, int32(0), hookCalls.Load(),
		"hook should not fire for non-gated tasks")
}

func TestFifoOrphanedTaskReclaimedDuringGracePeriod(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	q, _ := NewMemoryQueue(ctx, nil).(*fifo)
	assert.NotNil(t, q)

	task := &model.Task{
		ID:      "orphan-reclaim-1",
		AgentID: 42,
		Data:    []byte("{}"),
	}

	// Set very short extension for testing
	q.extension = 10 * time.Millisecond

	assert.NoError(t, q.Push(ctx, task))
	waitForProcess()

	// Poll to move to running
	got, err := q.Poll(ctx, 42, filterFnTrue)
	assert.NoError(t, err)
	assert.Equal(t, task.ID, got.ID)

	// Wait for deadline to expire (task becomes orphaned)
	time.Sleep(50 * time.Millisecond)
	waitForProcess()

	// Reclaim the task (simulating agent restart)
	err = q.Reclaim(ctx, task)
	assert.NoError(t, err)

	// Verify task is back in running with fresh deadline
	q.Lock()
	entry := q.running["orphan-reclaim-1"]
	q.Unlock()
	assert.NotNil(t, entry)
	assert.True(t, entry.orphanedAt.IsZero(), "orphanedAt should be reset after reclaim")
	assert.True(t, time.Now().Before(entry.deadline), "deadline should be in the future")

	// Should be able to complete normally
	err = q.Done(ctx, "orphan-reclaim-1", model.StatusSuccess)
	assert.NoError(t, err)
}
