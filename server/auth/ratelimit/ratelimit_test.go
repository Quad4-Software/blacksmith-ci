// Copyright 2026 Crow Authors
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

package ratelimit_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/auth/ratelimit"
)

func TestLimiter(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	l := ratelimit.New(3, 15*time.Minute, 4*time.Minute)

	for i := range 3 {
		ok, _ := l.Attempt("jane", t0.Add(time.Duration(i)*time.Second))
		assert.True(t, ok, "attempt %d is allowed, including the one reaching the threshold", i+1)
	}

	ok, wait := l.Attempt("jane", t0.Add(2*time.Second))
	assert.False(t, ok)
	assert.Equal(t, time.Minute, wait)

	ok, _ = l.Attempt("bob", t0)
	assert.True(t, ok, "keys are independent")

	// Blocked attempts record nothing, so the block does not grow.
	_, wait = l.Attempt("jane", t0.Add(32*time.Second))
	assert.Equal(t, 30*time.Second, wait)

	// Each further recorded attempt doubles the block up to the cap.
	ok, _ = l.Attempt("jane", t0.Add(70*time.Second))
	assert.True(t, ok)
	_, wait = l.Attempt("jane", t0.Add(70*time.Second))
	assert.Equal(t, 2*time.Minute, wait)
	ok, _ = l.Attempt("jane", t0.Add(200*time.Second))
	assert.True(t, ok)
	_, wait = l.Attempt("jane", t0.Add(201*time.Second))
	assert.Equal(t, 4*time.Minute-time.Second, wait)
	ok, _ = l.Attempt("jane", t0.Add(450*time.Second))
	assert.True(t, ok)
	_, wait = l.Attempt("jane", t0.Add(450*time.Second))
	assert.Equal(t, 4*time.Minute, wait, "capped")

	ok, _ = l.Attempt("jane", t0.Add(450*time.Second+4*time.Minute))
	assert.True(t, ok, "block expires")

	l.Reset("jane")
	for i := range 3 {
		ok, _ = l.Attempt("jane", t0.Add(20*time.Minute+time.Duration(i)*time.Second))
		assert.True(t, ok, "reset clears the count")
	}
}

func TestLimiterWindowExpiry(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	l := ratelimit.New(2, time.Minute, time.Hour)
	ok, _ := l.Attempt("ip", t0)
	assert.True(t, ok)
	ok, _ = l.Attempt("ip", t0.Add(2*time.Minute))
	assert.True(t, ok)
	ok, _ = l.Attempt("ip", t0.Add(2*time.Minute))
	assert.True(t, ok, "attempts outside the window do not add up")
	ok, _ = l.Attempt("ip", t0.Add(2*time.Minute))
	assert.False(t, ok)
}

func TestLimiterRetryAfterRoundsUp(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	l := ratelimit.New(1, time.Minute, time.Hour)
	ok, _ := l.Attempt("k", t0)
	assert.True(t, ok)
	_, wait := l.Attempt("k", t0.Add(500*time.Millisecond))
	assert.Equal(t, 60*time.Second, wait)
}

func TestLimiterConcurrentAttempts(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	l := ratelimit.New(5, time.Minute, time.Hour)

	var allowed atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if ok, _ := l.Attempt("jane", t0); ok {
				allowed.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	assert.Equal(t, int32(5), allowed.Load())
}

func TestLimiterForgive(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	l := ratelimit.New(5, time.Minute, time.Hour)

	for range 4 {
		ok, _ := l.Attempt("k", t0)
		assert.True(t, ok)
	}
	l.Forgive("k")
	for range 2 {
		ok, _ := l.Attempt("k", t0)
		assert.True(t, ok)
	}
	ok, _ := l.Attempt("k", t0)
	assert.False(t, ok)

	// Forgiving below the threshold lifts the block.
	l.Forgive("k")
	ok, _ = l.Attempt("k", t0)
	assert.True(t, ok)
}

func TestLimiterForgiveEdges(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	l := ratelimit.New(2, time.Minute, time.Hour)

	l.Forgive("unknown")

	ok, _ := l.Attempt("k", t0)
	assert.True(t, ok)
	for range 5 {
		l.Forgive("k")
	}
	// Never negative: a full budget of two attempts remains, no more.
	ok, _ = l.Attempt("k", t0)
	assert.True(t, ok)
	ok, _ = l.Attempt("k", t0)
	assert.True(t, ok)
	ok, _ = l.Attempt("k", t0)
	assert.False(t, ok)
}

// TestLimiterSustainedAttack guesses one username for 24 hours, retrying the
// moment each block ends. The window runs from the last recorded failure, so
// the block grows to the cap and stays there instead of restarting every
// window: 5 free attempts, then blocks of 1, 2, 4 and 8 minutes, then one
// attempt every 15 minutes (95 more in the remaining 1425 minutes), 103 in
// total. Anchoring the window on the first failure allowed 432.
func TestLimiterSustainedAttack(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	l := ratelimit.New(5, 15*time.Minute, 15*time.Minute)
	end := t0.Add(24 * time.Hour)

	allowed := 0
	var waits []time.Duration
	for now := t0; now.Before(end); {
		ok, wait := l.Attempt("jane", now)
		if ok {
			allowed++
			continue
		}
		waits = append(waits, wait)
		now = now.Add(wait)
	}

	assert.Equal(t, 103, allowed)
	if assert.GreaterOrEqual(t, len(waits), 5) {
		assert.Equal(t, []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute}, waits[:4])
		for i, w := range waits[4:] {
			assert.Equal(t, 15*time.Minute, w, "block %d stays at the cap", i+5)
		}
	}
}

func TestLimiterWindowRunsFromLastFailure(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	l := ratelimit.New(3, 10*time.Minute, time.Hour)

	// Failures spaced under the window keep adding up, even though the
	// first one is long past.
	for i := range 3 {
		ok, _ := l.Attempt("k", t0.Add(time.Duration(i)*9*time.Minute))
		assert.True(t, ok)
	}
	ok, _ := l.Attempt("k", t0.Add(18*time.Minute+30*time.Second))
	assert.False(t, ok, "the third failure at 18m reached the threshold")

	// A quiet window after the last failure starts afresh.
	for i := range 3 {
		ok, _ = l.Attempt("k", t0.Add(29*time.Minute+time.Duration(i)*time.Second))
		assert.True(t, ok)
	}
}
