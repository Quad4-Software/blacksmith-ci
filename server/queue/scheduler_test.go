// Copyright 2025 Crow CI Authors
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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseStrategy(t *testing.T) {
	tests := []struct {
		in      string
		want    Strategy
		wantErr bool
	}{
		{"", DefaultStrategy, false},
		{"least-loaded", StrategyLeastLoaded, false},
		{"pack", StrategyPack, false},
		{"round-robin", StrategyRoundRobin, false},
		{"random", StrategyRandom, false},
		{"bogus", "", true},
	}
	for _, tt := range tests {
		got, err := ParseStrategy(tt.in)
		if tt.wantErr {
			assert.Error(t, err, "ParseStrategy(%q)", tt.in)
			continue
		}
		assert.NoError(t, err, "ParseStrategy(%q)", tt.in)
		assert.Equal(t, tt.want, got, "ParseStrategy(%q)", tt.in)
	}
}

func TestSelectAgentEmpty(t *testing.T) {
	_, ok := selectAgent(nil, StrategyLeastLoaded, &schedulerState{})
	assert.False(t, ok, "no candidates should not select an agent")
}

func TestSelectAgentSingleCandidate(t *testing.T) {
	// A single candidate must be chosen regardless of strategy.
	for _, s := range []Strategy{StrategyLeastLoaded, StrategyPack, StrategyRoundRobin, StrategyRandom} {
		got, ok := selectAgent([]agentCandidate{{agentID: 7, running: 5, capacity: 6}}, s, &schedulerState{})
		assert.True(t, ok)
		assert.Equal(t, int64(7), got, "strategy %s", s)
	}
}

func TestSelectAgentPriorityDominates(t *testing.T) {
	// The high-priority agent is heavily loaded; the low-priority agent is idle.
	// Priority must win for every strategy, so load never overrides it.
	candidates := []agentCandidate{
		{agentID: 1, priority: 5, running: 3, capacity: 4}, // high priority, loaded
		{agentID: 2, priority: 1, running: 0, capacity: 4}, // low priority, idle
	}
	for _, s := range []Strategy{StrategyLeastLoaded, StrategyPack, StrategyRoundRobin, StrategyRandom} {
		got, ok := selectAgent(candidates, s, &schedulerState{})
		assert.True(t, ok)
		assert.Equal(t, int64(1), got, "strategy %s must honor priority over load", s)
	}
}

func TestSelectAgentLeastLoaded(t *testing.T) {
	t.Run("fewest running wins for equal capacity", func(t *testing.T) {
		candidates := []agentCandidate{
			{agentID: 1, running: 2, capacity: 4},
			{agentID: 2, running: 1, capacity: 4},
		}
		got, _ := selectAgent(candidates, StrategyLeastLoaded, &schedulerState{})
		assert.Equal(t, int64(2), got)
	})

	t.Run("normalized by capacity, not absolute count", func(t *testing.T) {
		// Agent 2 runs more tasks but has far more capacity, so its load
		// fraction (2/8) is lower than agent 1's (1/2). Least-loaded picks it.
		candidates := []agentCandidate{
			{agentID: 1, running: 1, capacity: 2}, // 0.50
			{agentID: 2, running: 2, capacity: 8}, // 0.25
		}
		got, _ := selectAgent(candidates, StrategyLeastLoaded, &schedulerState{})
		assert.Equal(t, int64(2), got)
	})

	t.Run("equal load ties resolve to smallest agent ID", func(t *testing.T) {
		candidates := []agentCandidate{
			{agentID: 3, running: 0, capacity: 4},
			{agentID: 1, running: 0, capacity: 4},
			{agentID: 2, running: 0, capacity: 4},
		}
		got, _ := selectAgent(candidates, StrategyLeastLoaded, &schedulerState{})
		assert.Equal(t, int64(1), got)
	})
}

func TestSelectAgentPack(t *testing.T) {
	// Pack prefers the most-loaded agent that still has a free slot.
	candidates := []agentCandidate{
		{agentID: 1, running: 1, capacity: 2}, // 0.50
		{agentID: 2, running: 2, capacity: 8}, // 0.25
	}
	got, _ := selectAgent(candidates, StrategyPack, &schedulerState{})
	assert.Equal(t, int64(1), got)
}

func TestSelectAgentRoundRobin(t *testing.T) {
	candidates := []agentCandidate{
		{agentID: 1, running: 0, capacity: 4},
		{agentID: 2, running: 0, capacity: 4},
		{agentID: 3, running: 0, capacity: 4},
	}
	state := &schedulerState{}
	var got []int64
	for i := 0; i < 4; i++ {
		id, _ := selectAgent(candidates, StrategyRoundRobin, state)
		got = append(got, id)
	}
	// Cycles ascending, then wraps around.
	assert.Equal(t, []int64{1, 2, 3, 1}, got)
}

func TestSelectAgentRandom(t *testing.T) {
	candidates := []agentCandidate{
		{agentID: 1, running: 0, capacity: 4},
		{agentID: 2, running: 0, capacity: 4},
		{agentID: 3, running: 0, capacity: 4},
	}
	// Inject a deterministic source: index 1 → second candidate (sorted by ID).
	state := &schedulerState{intn: func(int) int { return 1 }}
	got, ok := selectAgent(candidates, StrategyRandom, state)
	assert.True(t, ok)
	assert.Equal(t, int64(2), got)
}
