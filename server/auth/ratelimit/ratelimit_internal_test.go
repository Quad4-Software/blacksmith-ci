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

package ratelimit

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLimiterEntryCap(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	l := New(5, time.Hour, time.Hour)
	l.entryCap = 3

	for i := range 3 {
		ok, _ := l.Attempt(fmt.Sprintf("k%d", i), t0.Add(time.Duration(i)*time.Second))
		assert.True(t, ok)
	}
	assert.Len(t, l.entries, 3)

	ok, _ := l.Attempt("k3", t0.Add(10*time.Second))
	assert.True(t, ok)
	assert.Len(t, l.entries, 3)
	assert.NotContains(t, l.entries, "k0", "the least recently attempted key is evicted")
	assert.Contains(t, l.entries, "k3")

	for i := 4; i < 50; i++ {
		l.Attempt(fmt.Sprintf("k%d", i), t0.Add(time.Duration(i)*time.Second))
		assert.LessOrEqual(t, len(l.entries), 3)
	}
}

func TestLimiterEntryCapSweepsExpiredFirst(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	l := New(5, time.Minute, time.Hour)
	l.entryCap = 3

	l.Attempt("old", t0)
	l.Attempt("a", t0.Add(time.Minute+30*time.Second))
	l.Attempt("b", t0.Add(time.Minute+31*time.Second))
	l.Attempt("c", t0.Add(2*time.Minute))

	assert.NotContains(t, l.entries, "old")
	assert.Contains(t, l.entries, "a")
	assert.Contains(t, l.entries, "b")
	assert.Len(t, l.entries, 3)
}

func TestLimiterEntryCapEvictsLeastRecentlyAttempted(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	l := New(5, time.Hour, time.Hour)
	l.entryCap = 2

	l.Attempt("busy", t0)
	l.Attempt("idle", t0.Add(time.Second))
	l.Attempt("busy", t0.Add(2*time.Second))
	l.Attempt("new", t0.Add(3*time.Second))

	assert.Contains(t, l.entries, "busy", "a key failing recently is kept although its first failure is the oldest")
	assert.NotContains(t, l.entries, "idle")
	assert.Contains(t, l.entries, "new")
}

func TestLimiterEntryCapKeepsKeysAtThreshold(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	l := New(2, time.Hour, time.Hour)
	l.entryCap = 3

	l.Attempt("victim", t0)
	l.Attempt("victim", t0.Add(time.Second))
	for i := range 20 {
		ok, _ := l.Attempt(fmt.Sprintf("junk%d", i), t0.Add(time.Duration(2+i)*time.Second))
		assert.True(t, ok)
	}

	assert.Contains(t, l.entries, "victim", "a flood of fresh keys cannot evict a key at the threshold")
	ok, _ := l.Attempt("victim", t0.Add(30*time.Second))
	assert.False(t, ok)
}

func TestLimiterEntryCapEvictsFewestFailures(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	l := New(5, time.Hour, time.Hour)
	l.entryCap = 2

	l.Attempt("twice", t0)
	l.Attempt("twice", t0.Add(time.Second))
	l.Attempt("once", t0.Add(2*time.Second))
	l.Attempt("new", t0.Add(3*time.Second))

	assert.Contains(t, l.entries, "twice", "the key with more failures is kept although it is older")
	assert.NotContains(t, l.entries, "once")
	assert.Contains(t, l.entries, "new")
}

func TestLimiterEntryCapFullOfBlockedKeys(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	l := New(1, time.Hour, time.Hour)
	l.entryCap = 2

	l.Attempt("a", t0)
	l.Attempt("b", t0.Add(time.Minute))

	ok, retryAfter := l.Attempt("c", t0.Add(2*time.Minute))
	assert.False(t, ok, "a new key is blocked while every tracked key reached the threshold")
	assert.Equal(t, 58*time.Minute, retryAfter, "until the first tracked key expires")
	assert.NotContains(t, l.entries, "c")

	ok, _ = l.Attempt("c", t0.Add(time.Hour+time.Second))
	assert.True(t, ok, "an expired key makes room")
	assert.NotContains(t, l.entries, "a")
	assert.Contains(t, l.entries, "b")
}
