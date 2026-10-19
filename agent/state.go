// Copyright 2022 Woodpecker Authors
// Copyright 2018 Drone.IO Inc.
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

package agent

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

type State struct {
	sync.Mutex   `json:"-"`
	Polling      int             `json:"polling_count"`
	Running      int             `json:"running_count"`
	Metadata     map[string]Info `json:"running"`
	agentHealthy bool            `json:"-"` // tracks if agent is operational

	// runnersStarted counts the workflow runner goroutines ever started,
	// runnersLive how many of them are still alive. Together they let Healthy()
	// detect an agent that lost all of its runners (e.g. to a panic) and would
	// otherwise keep reporting 200 on /healthz while never picking up work again.
	runnersStarted int
	runnersLive    int

	// retiring records that the agent is winding down on purpose, because its
	// configured workflow lifetime budget is used up. A retiring agent ends up
	// with no runners left, which is exactly the zombie signature below, so the
	// flag tells the two states apart and keeps /healthz green while the agent
	// finishes its last workflow and exits.
	retiring bool
}

type Info struct {
	ID       string        `json:"id"`
	Repo     string        `json:"repository"`
	Pipeline string        `json:"pipeline_number"`
	Started  time.Time     `json:"pipeline_started"`
	Timeout  time.Duration `json:"pipeline_timeout"`
}

func (s *State) Add(id string, timeout time.Duration, repo, pipeline string) {
	s.Lock()
	s.Polling--
	s.Running++
	s.Metadata[id] = Info{
		ID:       id,
		Repo:     repo,
		Pipeline: pipeline,
		Timeout:  timeout,
		Started:  time.Now().UTC(),
	}
	s.Unlock()
}

func (s *State) Done(id string) {
	s.Lock()
	s.Polling++
	s.Running--
	delete(s.Metadata, id)
	s.Unlock()
}

// ActiveTaskUUIDs returns the set of task UUIDs for workflows currently running
// on this agent. The keys are the taskUUID values passed to Add, which are exactly
// the values the Docker backend stamps on every container, volume, and network as
// the LabelTaskUUID label. For freshly scheduled workflows the taskUUID equals the
// workflow ID; for adopted workflows the two differ, and Add is deliberately keyed
// by the taskUUID so the set still matches the resource labels. The stale-resource
// reaper uses this set to skip resources that belong to an in-flight build, so a
// pipeline running longer than the stale threshold is never reclaimed out from
// under itself.
func (s *State) ActiveTaskUUIDs() map[string]struct{} {
	s.Lock()
	defer s.Unlock()
	active := make(map[string]struct{}, len(s.Metadata))
	for id := range s.Metadata {
		active[id] = struct{}{}
	}
	return active
}

// Idle reports whether no workflow is running.
func (s *State) Idle() bool {
	s.Lock()
	defer s.Unlock()
	return s.Running == 0
}

// RunnerStarted records that a workflow runner goroutine has started.
func (s *State) RunnerStarted() {
	s.Lock()
	s.runnersStarted++
	s.runnersLive++
	s.Unlock()
}

// RunnerStopped records that a workflow runner goroutine has exited, whether
// normally, because its agent retired, or because it panicked. The runner was
// polling when it stopped — a runner that was executing a workflow has already
// been through Done — so the polling slot goes away with it.
func (s *State) RunnerStopped() {
	s.Lock()
	s.runnersLive--
	if s.Polling > 0 {
		s.Polling--
	}
	s.Unlock()
}

// SetRetiring marks the agent as winding down by design. Called when a
// single-use agent has used up its workflow lifetime budget.
func (s *State) SetRetiring(retiring bool) {
	s.Lock()
	defer s.Unlock()
	s.retiring = retiring
}

// Retiring reports whether the agent is winding down because its workflow
// lifetime budget is used up, as opposed to being stopped from the outside.
func (s *State) Retiring() bool {
	s.Lock()
	defer s.Unlock()
	return s.retiring
}

func (s *State) Healthy() bool {
	s.Lock()
	defer s.Unlock()

	// If agent has crashed or is not operational, report unhealthy
	if !s.agentHealthy {
		return false
	}

	// An agent that once had workflow runners but has none left cannot process
	// any pipeline, even though every other signal still looks fine. Report
	// unhealthy so the supervisor restarts it instead of leaving a zombie behind.
	if !s.retiring && s.runnersStarted > 0 && s.runnersLive == 0 {
		return false
	}

	// Check if any running workflows have timed out
	now := time.Now()
	buf := time.Hour // 1 hour buffer
	for _, item := range s.Metadata {
		if now.After(item.Started.Add(item.Timeout).Add(buf)) {
			return false
		}
	}
	return true
}

// SetHealthy marks the agent as healthy/operational.
func (s *State) SetHealthy(healthy bool) {
	s.Lock()
	defer s.Unlock()
	s.agentHealthy = healthy
}

func (s *State) WriteTo(w io.Writer) (int64, error) {
	s.Lock()
	out, err := json.Marshal(s)
	s.Unlock()
	if err != nil {
		return 0, err
	}
	ret, err := w.Write(out)
	return int64(ret), err
}
