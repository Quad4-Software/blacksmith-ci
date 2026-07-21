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
	"fmt"
	"math/rand/v2"
	"sort"
	"sync"
)

// Strategy selects which agent receives a task when several agents are equally
// eligible to run it (same labels, same top priority). Priority is always
// honored first; the strategy only decides between agents that tie on priority.
type Strategy string

const (
	// StrategyLeastLoaded assigns to the agent with the lowest current load,
	// where load is running tasks divided by the agent's total slots. It spreads
	// work evenly across agents and is the default.
	StrategyLeastLoaded Strategy = "least-loaded"

	// StrategyPack concentrates work onto the fewest agents by preferring the
	// most-loaded agent that still has a free slot. Useful together with the
	// autoscaler so that idle agents can be scaled down.
	StrategyPack Strategy = "pack"

	// StrategyRoundRobin cycles through eligible agents in a stable order,
	// ignoring their current load.
	StrategyRoundRobin Strategy = "round-robin"

	// StrategyRandom picks a uniformly random eligible agent.
	StrategyRandom Strategy = "random"

	// DefaultStrategy is used when no strategy is configured.
	DefaultStrategy = StrategyLeastLoaded
)

// ParseStrategy validates a strategy name from configuration. An empty string
// resolves to DefaultStrategy; any other unknown value is an error.
func ParseStrategy(s string) (Strategy, error) {
	switch Strategy(s) {
	case StrategyLeastLoaded, StrategyPack, StrategyRoundRobin, StrategyRandom:
		return Strategy(s), nil
	case "":
		return DefaultStrategy, nil
	default:
		return "", fmt.Errorf("unknown scheduling strategy %q (valid: %s, %s, %s, %s)",
			s, StrategyLeastLoaded, StrategyPack, StrategyRoundRobin, StrategyRandom)
	}
}

// orDefault resolves an unset strategy to the default.
func (s Strategy) orDefault() Strategy {
	if s == "" {
		return DefaultStrategy
	}
	return s
}

// agentCandidate describes one agent eligible to run a task, carrying the
// agent-scoped attributes the selector needs to break ties between agents.
type agentCandidate struct {
	agentID  int64
	priority int32
	running  int // tasks currently running/assigned on this agent
	capacity int // total slots (running + free); <= 0 means unknown
}

// schedulerState holds the mutable state a strategy carries across calls (the
// round-robin cursor and an optional injectable random source). Its zero value
// is ready to use.
type schedulerState struct {
	mu     sync.Mutex
	rrLast int64         // agent ID last chosen by round-robin
	intn   func(int) int // random source; nil falls back to the global rng
}

// randIntn returns a random int in [0,n) using the injected source when set.
func (st *schedulerState) randIntn(n int) int {
	if st.intn != nil {
		return st.intn(n)
	}
	return rand.IntN(n)
}

// selectAgent chooses which agent should receive the next task from the set of
// eligible candidates. Agent priority is always honored first (higher wins);
// the configured strategy only decides between agents that share the top
// priority. Returns the chosen agent ID, or (0, false) when there are no
// candidates.
func selectAgent(candidates []agentCandidate, strategy Strategy, state *schedulerState) (int64, bool) {
	if len(candidates) == 0 {
		return 0, false
	}

	top := topPriority(candidates)
	if len(top) == 1 {
		return top[0].agentID, true
	}

	// Stable, deterministic ordering by agent ID so every strategy is
	// reproducible and load ties resolve predictably.
	sort.Slice(top, func(i, j int) bool { return top[i].agentID < top[j].agentID })

	switch strategy.orDefault() {
	case StrategyPack:
		return pickMostLoaded(top), true
	case StrategyRoundRobin:
		return pickRoundRobin(top, state), true
	case StrategyRandom:
		return pickRandom(top, state), true
	case StrategyLeastLoaded:
		return pickLeastLoaded(top), true
	default:
		return pickLeastLoaded(top), true
	}
}

// topPriority returns the candidates sharing the highest priority value.
func topPriority(candidates []agentCandidate) []agentCandidate {
	maxPriority := candidates[0].priority
	for _, c := range candidates[1:] {
		if c.priority > maxPriority {
			maxPriority = c.priority
		}
	}
	top := make([]agentCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.priority == maxPriority {
			top = append(top, c)
		}
	}
	return top
}

// lighter reports whether a is less loaded than b. When both capacities are
// known it compares load fractions (running/capacity) via cross-multiplication
// to avoid floating point; otherwise it falls back to absolute running counts.
func lighter(a, b agentCandidate) bool {
	if a.capacity > 0 && b.capacity > 0 {
		return a.running*b.capacity < b.running*a.capacity
	}
	return a.running < b.running
}

// pickLeastLoaded returns the least-loaded agent. Input must be sorted by agent
// ID so equal-load ties resolve to the smallest agent ID.
func pickLeastLoaded(sorted []agentCandidate) int64 {
	best := sorted[0]
	for _, c := range sorted[1:] {
		if lighter(c, best) {
			best = c
		}
	}
	return best.agentID
}

// pickMostLoaded returns the most-loaded agent that still has a free slot; every
// candidate has at least one free slot by construction. Input must be sorted by
// agent ID so equal-load ties resolve to the smallest agent ID.
func pickMostLoaded(sorted []agentCandidate) int64 {
	best := sorted[0]
	for _, c := range sorted[1:] {
		if lighter(best, c) { // c is heavier than best
			best = c
		}
	}
	return best.agentID
}

// pickRoundRobin returns the next agent after the previously chosen one in
// ascending agent-ID order, wrapping around. Input must be sorted by agent ID.
func pickRoundRobin(sorted []agentCandidate, state *schedulerState) int64 {
	state.mu.Lock()
	defer state.mu.Unlock()
	for _, c := range sorted {
		if c.agentID > state.rrLast {
			state.rrLast = c.agentID
			return c.agentID
		}
	}
	state.rrLast = sorted[0].agentID
	return sorted[0].agentID
}

// pickRandom returns a uniformly random candidate.
func pickRandom(sorted []agentCandidate, state *schedulerState) int64 {
	return sorted[state.randIntn(len(sorted))].agentID
}
