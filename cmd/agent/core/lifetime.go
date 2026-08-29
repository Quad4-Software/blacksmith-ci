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
)

// workflowLifetime caps how many workflows an agent executes over its whole
// life, as opposed to max-workflows which caps how many it runs at once.
//
// It exists for single-use agents: on a public instance an agent host is
// reused between workflows, so whatever one repository's pipeline leaves
// behind — a poisoned build cache, a background process, a modified
// toolchain — is still there for the next repository's job. The only thing
// that partitions mutually distrusting repositories is a fresh machine per
// job, which means the agent has to retire itself after N workflows and let
// its supervisor discard the whole VM.
//
// A limit of zero (the default) means unlimited, in which case every method is
// a no-op, Reserve always succeeds and the agent behaves exactly as before.
//
// The budget is a pool of slots. A runner takes one before it polls and either
// spends it, when the poll returned a workflow, or releases it, when the poll
// found an empty queue — so waiting for work never costs an agent part of its
// life. A runner that finds the pool empty waits: the slots are held by its
// siblings, which may yet release them. Only when every slot has been spent is
// the budget gone for good, and Reserve stops blocking and starts refusing.
type workflowLifetime struct {
	limit int

	// slots holds one token per slot that has not been spent yet.
	slots chan struct{}
	// exhausted is closed once every slot has been spent, which releases all
	// waiting runners at once.
	exhausted     chan struct{}
	exhaustedOnce sync.Once

	mu    sync.Mutex
	spent int
}

func newWorkflowLifetime(limit int) *workflowLifetime {
	if limit < 0 {
		limit = 0
	}

	lifetime := &workflowLifetime{limit: limit}
	if limit > 0 {
		lifetime.slots = make(chan struct{}, limit)
		for range limit {
			lifetime.slots <- struct{}{}
		}
		lifetime.exhausted = make(chan struct{})
	}
	return lifetime
}

// Enabled reports whether a lifetime limit is configured at all.
func (l *workflowLifetime) Enabled() bool {
	return l.limit > 0
}

// Reserve claims one slot of the lifetime budget, blocking while all remaining
// slots are held by other runners that have not spent them yet. It returns
// false once the budget is used up — a runner's signal to stop polling and
// exit — or when ctx is done.
func (l *workflowLifetime) Reserve(ctx context.Context) bool {
	if !l.Enabled() {
		return true
	}

	// Never hand out a slot that is left over after the budget was overdrawn
	// by an adopted workflow.
	if l.Exhausted() {
		return false
	}

	select {
	case <-l.slots:
		return true
	default:
	}

	select {
	case <-l.slots:
		return true
	case <-l.exhausted:
		return false
	case <-ctx.Done():
		return false
	}
}

// Release hands an unused reservation back to the budget. It is called when a
// poll returned no work, so waiting for a task never counts as running one.
func (l *workflowLifetime) Release() {
	if !l.Enabled() {
		return
	}

	select {
	case l.slots <- struct{}{}:
	default:
		// The pool is already full because the budget was overdrawn by an
		// adopted workflow; this slot no longer exists.
	}
}

// Spend turns a reservation into a workflow that actually ran. The slot is
// gone for good.
func (l *workflowLifetime) Spend() {
	l.charge(true)
}

// Charge accounts for a workflow that runs without a prior reservation:
// workflows adopted from a previous agent instance were already claimed, and
// their steps already ran on this machine, so they are charged whatever the
// budget says. Overdrawing is allowed, the budget simply ends up empty and the
// agent retires once the workflow is done.
func (l *workflowLifetime) Charge() {
	l.charge(false)
}

func (l *workflowLifetime) charge(reserved bool) {
	if !l.Enabled() {
		return
	}

	if !reserved {
		// Take a slot if one is free, so a polling runner cannot spend the
		// same slot a second time.
		select {
		case <-l.slots:
		default:
		}
	}

	l.mu.Lock()
	l.spent++
	usedUp := l.spent >= l.limit
	l.mu.Unlock()

	if usedUp {
		l.exhaustedOnce.Do(func() { close(l.exhausted) })
	}
}

// Exhausted reports whether the budget is permanently used up, meaning every
// slot was spent on a workflow that ran rather than merely held by a runner
// that is still waiting for work.
func (l *workflowLifetime) Exhausted() bool {
	if !l.Enabled() {
		return false
	}

	select {
	case <-l.exhausted:
		return true
	default:
		return false
	}
}

// Spent returns how many workflows have been charged against the budget so
// far. It can exceed the limit when adopted workflows overdrew it.
func (l *workflowLifetime) Spent() int {
	if !l.Enabled() {
		return 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	return l.spent
}
