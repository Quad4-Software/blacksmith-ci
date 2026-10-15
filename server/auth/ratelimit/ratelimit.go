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

// Package ratelimit implements a memory-based rate limiter for failed
// attempts with exponential backoff.
//
// Callers record an attempt in advance with [Limiter.Attempt], which checks and
// records under one lock so concurrent requests cannot all slip past the limit,
// and undo it with [Limiter.Forgive] or [Limiter.Reset] when the attempt turns
// out not to be a failure. The number of tracked keys is bounded.
package ratelimit

import (
	"sync"
	"time"
)

// maxEntries is the default upper bound on the number of tracked keys.
const maxEntries = 10000

type entry struct {
	failures     int
	last         time.Time
	blockedUntil time.Time
	block        time.Duration
}

// Limiter blocks a key after too many recorded attempts. An entry expires once
// a whole window passes without a recorded attempt, so failures spaced closer
// than the window keep adding up. Each attempt recorded after the threshold
// doubles the block, up to a cap. Attempts
// are recorded as failures in advance and are undone by Forgive or Reset. The
// attempt that reaches the threshold is itself allowed; the next one is blocked.
// It is safe for concurrent use, keeps its state in memory and tracks at most
// maxEntries keys. When full, it evicts the key with the fewest failures; if
// every key has reached the threshold, attempts for new keys are blocked.
type Limiter struct {
	mu          sync.Mutex
	entries     map[string]*entry
	maxFailures int
	window      time.Duration
	maxBackoff  time.Duration
	entryCap    int
	lastSweep   time.Time
}

// New creates a new Limiter with the given constraints.
func New(maxFailures int, window, maxBackoff time.Duration) *Limiter {
	return &Limiter{
		entries:     make(map[string]*entry),
		maxFailures: maxFailures,
		window:      window,
		maxBackoff:  maxBackoff,
		entryCap:    maxEntries,
	}
}

// Attempt atomically checks key and, when allowed, records the attempt as a
// failure in advance. Callers undo it with Forgive or Reset when the attempt
// turns out not to be a failure. A blocked attempt records nothing.
func (l *Limiter) Attempt(key string, now time.Time) (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, exists := l.entries[key]
	switch {
	case !exists:
		if ok, wait := l.makeRoom(now); !ok {
			return false, roundUpToSecond(wait)
		}
		e = &entry{}
		l.entries[key] = e
	case now.Before(e.blockedUntil):
		return false, roundUpToSecond(e.blockedUntil.Sub(now))
	case now.Sub(e.last) > l.window:
		// A whole window passed without a recorded attempt and no block is
		// active: start fresh.
		e.failures = 0
		e.blockedUntil = time.Time{}
		e.block = 0
	}

	e.failures++
	e.last = now

	if e.failures == l.maxFailures {
		e.block = time.Minute
	} else if e.failures > l.maxFailures {
		e.block *= 2
		if e.block > l.maxBackoff {
			e.block = l.maxBackoff
		}
	}

	if e.failures >= l.maxFailures {
		e.blockedUntil = now.Add(e.block)
	}

	return true, 0
}

// Forgive undoes one attempt recorded by Attempt (failures-1, never below 0;
// when failures drops below maxFailures the block is cleared). Used when an
// attempt succeeded on a shared key (client IP) or failed for a reason that
// is not the client's fault (directory unreachable).
func (l *Limiter) Forgive(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, exists := l.entries[key]
	if !exists {
		return
	}
	if e.failures > 0 {
		e.failures--
	}
	if e.failures == 0 {
		delete(l.entries, key)
		return
	}
	if e.failures < l.maxFailures {
		e.blockedUntil = time.Time{}
		e.block = 0
	}
}

// Reset forgets key entirely, e.g. after a successful sign-in.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.entries, key)
}

// makeRoom tries to free a slot for one more key and reports whether it did.
// It sweeps expired entries at most once per window/10. If the map is still
// full, it evicts the entry with the fewest failures, the least recently
// attempted first. Entries that reached maxFailures are never evicted, so
// flooding the limiter with fresh keys cannot wipe the count of a key under
// attack. When every entry has reached maxFailures, wait is how long until the
// first of them expires.
func (l *Limiter) makeRoom(now time.Time) (ok bool, wait time.Duration) {
	if len(l.entries) < l.entryCap {
		return true, 0
	}
	if now.Sub(l.lastSweep) >= l.window/10 {
		l.lastSweep = now
		for k, e := range l.entries {
			if l.expired(e, now) {
				delete(l.entries, k)
			}
		}
	}
	for len(l.entries) >= l.entryCap {
		var victim string
		var victimEntry *entry
		var firstExpiry time.Time
		for k, e := range l.entries {
			if l.expired(e, now) {
				victim, victimEntry = k, e
				break
			}
			if e.failures >= l.maxFailures {
				if expiry := l.expiry(e); firstExpiry.IsZero() || expiry.Before(firstExpiry) {
					firstExpiry = expiry
				}
				continue
			}
			if victimEntry == nil || e.failures < victimEntry.failures ||
				(e.failures == victimEntry.failures && e.last.Before(victimEntry.last)) {
				victim, victimEntry = k, e
			}
		}
		if victimEntry == nil {
			return false, max(firstExpiry.Sub(now), time.Second)
		}
		delete(l.entries, victim)
	}
	return true, 0
}

// expiry is when e may be forgotten: a whole window after its last attempt,
// but not before its block ends.
func (l *Limiter) expiry(e *entry) time.Time {
	if t := e.last.Add(l.window); t.After(e.blockedUntil) {
		return t
	}
	return e.blockedUntil
}

// expired reports whether e may be forgotten at now.
func (l *Limiter) expired(e *entry, now time.Time) bool {
	return now.Sub(e.last) > l.window && !now.Before(e.blockedUntil)
}

// roundUpToSecond rounds d up to whole seconds.
func roundUpToSecond(d time.Duration) time.Duration {
	r := d.Truncate(time.Second)
	if r < d {
		r += time.Second
	}
	return r
}
