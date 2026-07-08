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
	"strings"
	"time"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
)

var (
	// ErrCancel indicates the task was canceled.
	ErrCancel = errors.New("queue: task canceled")

	// ErrNotFound indicates the task was not found in the queue.
	ErrNotFound = errors.New("queue: task not found")

	// ErrAgentMissMatch indicates a task is assigned to a different agent.
	ErrAgentMissMatch = errors.New("task assigned to different agent")
)

// InfoT provides runtime information.
type InfoT struct {
	Pending       []*model.Task `json:"pending"`
	WaitingOnDeps []*model.Task `json:"waiting_on_deps"`
	Running       []*model.Task `json:"running"`
	Stats         struct {
		Workers       int `json:"worker_count"`
		Pending       int `json:"pending_count"`
		WaitingOnDeps int `json:"waiting_on_deps_count"`
		Running       int `json:"running_count"`
	} `json:"stats"`
	Paused bool `json:"paused"`
} //	@name	InfoT

func (t *InfoT) String() string {
	var sb strings.Builder

	for _, task := range t.Pending {
		sb.WriteString("\t" + task.String())
	}

	for _, task := range t.Running {
		sb.WriteString("\t" + task.String())
	}

	for _, task := range t.WaitingOnDeps {
		sb.WriteString("\t" + task.String())
	}

	return sb.String()
}

// Filter filters tasks in the queue. If the Filter returns false,
// the Task is skipped and not returned to the subscriber.
// The int return value represents the matching score (higher is better).
// The int32 return value represents the agent priority (0 = no priority).
type FilterFn func(*model.Task) (bool, int, int32)

// BlockOnApprovalFn is called by the queue when a gated task's upstream deps
// have all reached a terminal state and the gate must now be activated.
// Implementations live in the pipeline package and persist the workflow row
// (State=Blocked, ApprovalState=Pending, ApprovalDeadline=now+timeout). The
// queue cannot import the pipeline package directly (import cycle), so the
// hook is injected at server startup via SetBlockOnApprovalHook.
type BlockOnApprovalFn func(ctx context.Context, taskID string)

// Queue defines a task queue for scheduling tasks among
// a pool of workers.
type Queue interface {
	// Push pushes a task to the tail of this queue.
	Push(c context.Context, task *model.Task) error

	// PushAtOnce pushes multiple tasks to the tail of this queue.
	PushAtOnce(c context.Context, tasks []*model.Task) error

	// Poll retrieves and removes a task head of this queue.
	Poll(c context.Context, agentID int64, f FilterFn) (*model.Task, error)

	// Extend extends the deadline for a task.
	Extend(c context.Context, agentID int64, workflowID string) error

	// Done signals the task is complete.
	Done(c context.Context, id string, exitStatus model.StatusValue) error

	// Error signals the task is done with an error.
	Error(c context.Context, id string, err error) error

	// ErrorAtOnce signals multiple done are complete with an error.
	ErrorAtOnce(c context.Context, ids []string, err error) error

	// Evict removes a pending task from the queue.
	Evict(c context.Context, id string) error

	// EvictAtOnce removes multiple pending tasks from the queue.
	EvictAtOnce(c context.Context, ids []string) error

	// Wait waits until the task is complete.
	Wait(c context.Context, id string) error

	// Reclaim re-adds a task to the running state after agent restart.
	// This is used when an agent reclaims an orphaned task that it was previously running.
	Reclaim(c context.Context, task *model.Task) error

	// Info returns internal queue information.
	Info(c context.Context) InfoT

	// GetRunningWorkflowIDs returns a list of workflow IDs that are currently running
	// This is useful for cleanup operations
	GetRunningWorkflowIDs() []string

	// Pause stops the queue from handing out new work items in Poll
	Pause()

	// Resume starts the queue again.
	Resume()

	// KickAgentWorkers kicks all workers for a given agent.
	KickAgentWorkers(agentID int64)

	// SetApprovalState updates the in-queue task's approval state and triggers
	// a re-evaluation of which waiting tasks are now eligible to run.
	SetApprovalState(ctx context.Context, taskID string, state model.ApprovalState) error

	// SetBlockOnApprovalHook injects a callback that the queue invokes when a
	// gated task's upstream dependencies have all cleared. The callback is
	// expected to persist the workflow row (State=Blocked, ApprovalState=
	// Pending, ApprovalDeadline=now+timeout). Implementations call it at most
	// once per task per transition out of ApprovalNone. A nil function disables
	// the hook (useful in tests).
	SetBlockOnApprovalHook(BlockOnApprovalFn)
}

// Config holds the configuration for the queue.
type Config struct {
	Backend       Type
	Store         store.Store
	LockService   LockService
	InstanceID    string
	LeaderLockTTL time.Duration // TTL for leader election lock
}

// LockService defines the interface for distributed locking.
type LockService interface {
	AcquireLock(ctx context.Context, lockName string, ttl time.Duration) (context.Context, context.CancelFunc, error)
}

// Queue type.
type Type string

const (
	TypeMemory   Type = "memory"
	TypeDatabase Type = "database"

	// DefaultLeaderLockTTL is the default TTL for leader election locks.
	DefaultLeaderLockTTL = 30 * time.Second
)

// New creates a new queue based on the provided configuration.
func New(ctx context.Context, config Config) (Queue, error) {
	var q Queue

	switch config.Backend {
	case TypeMemory:
		q = NewMemoryQueue(ctx, config.Store)
		q = WithMetrics(q)
		if config.Store != nil {
			q = WithTaskStore(ctx, q, config.Store)
		}
	case TypeDatabase:
		if config.Store == nil {
			return nil, fmt.Errorf("database queue requires a store")
		}
		if config.LockService == nil {
			return nil, fmt.Errorf("database queue requires a lock service")
		}
		// Use configured TTL or default
		ttl := config.LeaderLockTTL
		if ttl == 0 {
			ttl = DefaultLeaderLockTTL
		}
		q = WithMetrics(NewDatabaseQueue(ctx, config.Store, config.LockService, config.InstanceID, ttl))
	default:
		return nil, fmt.Errorf("unsupported queue backend: %s", config.Backend)
	}

	return q, nil
}
