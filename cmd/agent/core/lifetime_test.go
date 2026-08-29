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

package core

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWorkflowLifetimeDisabled(t *testing.T) {
	for _, limit := range []int{0, -1} {
		lifetime := newWorkflowLifetime(limit)
		assert.False(t, lifetime.Enabled(), "limit %d must not enable the budget", limit)

		// An unlimited budget hands out reservations forever and never counts.
		for range 1000 {
			assert.True(t, lifetime.Reserve(t.Context()))
			lifetime.Spend()
		}
		assert.Zero(t, lifetime.Spent())
		assert.False(t, lifetime.Exhausted())
	}
}

func TestWorkflowLifetimeReserveIsBounded(t *testing.T) {
	lifetime := newWorkflowLifetime(2)
	assert.True(t, lifetime.Enabled())

	assert.True(t, lifetime.Reserve(t.Context()))
	assert.True(t, lifetime.Reserve(t.Context()))

	// Reservations that never turned into a workflow go back into the budget,
	// so an idle runner does not burn the agent's lifetime by polling.
	lifetime.Release()
	assert.True(t, lifetime.Reserve(t.Context()))

	lifetime.Spend()
	lifetime.Spend()
	assert.False(t, lifetime.Reserve(t.Context()), "a spent slot must not come back")
	assert.True(t, lifetime.Exhausted())
	assert.Equal(t, 2, lifetime.Spent())
}

// A runner that finds every slot held by a sibling must wait for one rather
// than exit: the sibling may be idling on an empty queue and release it. The
// alternative is an agent that silently loses parallelism as it ages.
func TestWorkflowLifetimeReserveWaitsForSiblings(t *testing.T) {
	lifetime := newWorkflowLifetime(2)

	// Both slots are held by other runners, neither spent.
	assert.True(t, lifetime.Reserve(t.Context()))
	assert.True(t, lifetime.Reserve(t.Context()))

	reserved := make(chan bool, 1)
	go func() {
		reserved <- lifetime.Reserve(t.Context())
	}()

	select {
	case <-reserved:
		t.Fatal("Reserve returned while the budget still had unspent slots")
	case <-time.After(50 * time.Millisecond):
	}

	// One sibling's poll came back empty.
	lifetime.Release()

	select {
	case ok := <-reserved:
		assert.True(t, ok, "the released slot must go to the waiting runner")
	case <-time.After(time.Second):
		t.Fatal("Reserve did not pick up the released slot")
	}
}

// Once the last slot is spent, every waiting runner is released at once so the
// agent can retire instead of hanging on a budget that will never come back.
func TestWorkflowLifetimeReserveUnblocksWhenBudgetIsSpent(t *testing.T) {
	lifetime := newWorkflowLifetime(1)
	assert.True(t, lifetime.Reserve(t.Context()))

	waiting := make(chan bool, 3)
	for range cap(waiting) {
		go func() {
			waiting <- lifetime.Reserve(t.Context())
		}()
	}

	lifetime.Spend()

	for range cap(waiting) {
		select {
		case ok := <-waiting:
			assert.False(t, ok, "a waiting runner must be refused once the budget is spent")
		case <-time.After(time.Second):
			t.Fatal("Reserve did not return after the budget was spent")
		}
	}
}

func TestWorkflowLifetimeReserveGivesUpOnShutdown(t *testing.T) {
	lifetime := newWorkflowLifetime(1)
	assert.True(t, lifetime.Reserve(t.Context()))

	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(nil)

	assert.False(t, lifetime.Reserve(ctx))
	assert.False(t, lifetime.Exhausted(), "a shutdown is not a used-up budget")
}

func TestWorkflowLifetimeCharge(t *testing.T) {
	// Adopted workflows are already claimed and run regardless of the budget,
	// so charging them may overdraw it. The agent must still stop afterwards.
	lifetime := newWorkflowLifetime(1)

	lifetime.Charge()
	lifetime.Charge()
	assert.Equal(t, 2, lifetime.Spent())
	assert.True(t, lifetime.Exhausted())
	assert.False(t, lifetime.Reserve(t.Context()))
}

func TestWorkflowLifetimeIsConcurrencySafe(t *testing.T) {
	const limit = 50
	lifetime := newWorkflowLifetime(limit)

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		granted int
	)
	for range 200 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if lifetime.Reserve(t.Context()) {
				lifetime.Spend()
				mu.Lock()
				granted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, limit, granted)
	assert.Equal(t, limit, lifetime.Spent())
}
