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

package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/agent"
)

func TestHealthzBackendProbe(t *testing.T) {
	prev := counter
	t.Cleanup(func() { counter = prev })
	counter = &agent.State{Metadata: map[string]agent.Info{}}
	counter.SetHealthy(true)

	t.Cleanup(func() { SetBackendHealthy(true) })

	// Default: backend assumed healthy → 200.
	SetBackendHealthy(true)
	rec := httptest.NewRecorder()
	handleHeartbeat(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assert.Equal(t, http.StatusOK, rec.Code, "healthy counter + healthy backend should return 200")

	// Backend probe failing → 500 even though counter is healthy.
	SetBackendHealthy(false)
	rec = httptest.NewRecorder()
	handleHeartbeat(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assert.Equal(t, http.StatusInternalServerError, rec.Code, "healthy counter + unhealthy backend should return 500")

	// Counter unhealthy → 500 regardless of backend state.
	counter.SetHealthy(false)
	SetBackendHealthy(true)
	rec = httptest.NewRecorder()
	handleHeartbeat(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assert.Equal(t, http.StatusInternalServerError, rec.Code, "unhealthy counter should return 500 even if backend is healthy")
}

// TestHealthzRunnerLiveness covers the second half of issue #1396: when the
// agent's workflow runners are gone there is nobody left to poll for work, so
// /healthz must stop reporting 200 and let the supervisor restart the agent.
func TestHealthzRunnerLiveness(t *testing.T) {
	prev := counter
	t.Cleanup(func() { counter = prev })
	counter = &agent.State{Metadata: map[string]agent.Info{}}
	counter.SetHealthy(true)

	t.Cleanup(func() { SetBackendHealthy(true) })
	SetBackendHealthy(true)

	status := func() int {
		rec := httptest.NewRecorder()
		handleHeartbeat(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		return rec.Code
	}

	// Before any runner is started (e.g. during startup) liveness is not asserted.
	assert.Equal(t, http.StatusOK, status())

	counter.RunnerStarted()
	counter.RunnerStarted()
	assert.Equal(t, http.StatusOK, status(), "running agent with live runners should return 200")

	counter.RunnerStopped()
	assert.Equal(t, http.StatusOK, status(), "one remaining runner is still enough to be healthy")

	counter.RunnerStopped()
	assert.Equal(t, http.StatusInternalServerError, status(), "agent without any live runner should return 500")
}

func TestHealthy(t *testing.T) {
	s := agent.State{}
	s.Metadata = map[string]agent.Info{}

	s.Add("1", time.Hour, "octocat/hello-world", "42")

	assert.Equal(t, "1", s.Metadata["1"].ID)
	assert.Equal(t, time.Hour, s.Metadata["1"].Timeout)
	assert.Equal(t, "octocat/hello-world", s.Metadata["1"].Repo)

	s.SetHealthy(true)

	s.Metadata["1"] = agent.Info{
		Timeout: time.Hour,
		Started: time.Now().UTC(),
	}
	assert.True(t, s.Healthy(), "want healthy status when timeout not exceeded, got false")

	s.Metadata["1"] = agent.Info{
		Started: time.Now().UTC().Add(-(time.Minute * 30)),
	}
	assert.True(t, s.Healthy(), "want healthy status when timeout+buffer not exceeded, got false")

	s.Metadata["1"] = agent.Info{
		Started: time.Now().UTC().Add(-(time.Hour + time.Minute)),
	}
	assert.False(t, s.Healthy(), "want unhealthy status when timeout+buffer not exceeded, got true")
}
