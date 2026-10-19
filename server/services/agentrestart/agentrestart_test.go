// Copyright 2026 Crow CI Authors
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

package agentrestart

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/pipeline/rpc"
	"codefloe.com/crowci/crow/v6/pipeline/rpc/proto"
	"codefloe.com/crowci/crow/v6/server/model"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

const serverVersion = "6.12.0"

var now = time.Unix(1_800_000_000, 0)

// optedIn returns an agent that accepts restarts and reported health just now.
func optedIn(id int64, version string, protocol int32) *model.Agent {
	return &model.Agent{
		ID:              id,
		Version:         version,
		ProtocolVersion: protocol,
		RemoteUpdate:    rpc.RemoteUpdateRestart,
		LastContact:     now.Unix(),
	}
}

// reconcile runs Reconcile against agents and returns the IDs it asked to restart.
func reconcile(t *testing.T, cfg Config, agents ...*model.Agent) []int64 {
	t.Helper()
	db := store_mocks.NewMockStore(t)
	db.On("AgentList", mock.Anything).Return(agents, nil)

	var requested, kicked []int64
	db.EXPECT().AgentRequestAutoRestart(mock.Anything, serverVersion).RunAndReturn(func(id int64, _ string) error {
		requested = append(requested, id)
		return nil
	}).Maybe()
	cfg.ServerVersion = serverVersion
	cfg.Kick = func(id int64) { kicked = append(kicked, id) }
	if cfg.MaxParallel == 0 {
		cfg.MaxParallel = 10
	}

	n, err := Reconcile(db, cfg, now)
	require.NoError(t, err)
	assert.Len(t, requested, n)
	assert.Equal(t, requested, kicked)
	return requested
}

func TestParsePolicy(t *testing.T) {
	for in, want := range map[string]Policy{"": PolicyOff, "off": PolicyOff, "outdated": PolicyOutdated, "release": PolicyRelease} {
		got, err := ParsePolicy(in)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
	_, err := ParsePolicy("always")
	assert.Error(t, err)
}

func TestReconcileOffOnlyPromotesQueuedAgents(t *testing.T) {
	queued := optedIn(1, "6.11.0", proto.Version-1)
	queued.RestartQueued = true
	outdated := optedIn(2, "6.11.0", proto.Version-1)

	assert.Equal(t, []int64{1}, reconcile(t, Config{Policy: PolicyOff}, queued, outdated))
}

func TestReconcileOutdatedPolicy(t *testing.T) {
	outdated := optedIn(1, "6.11.0", proto.Version-1)
	otherRelease := optedIn(2, "6.11.1", proto.Version)
	current := optedIn(3, serverVersion, proto.Version)
	// came back on the same old release after this server release asked once
	alreadyAsked := optedIn(4, "6.11.0", proto.Version-1)
	alreadyAsked.AutoRestartVersion = serverVersion
	notOptedIn := optedIn(5, "6.11.0", proto.Version-1)
	notOptedIn.RemoteUpdate = ""
	offline := optedIn(6, "6.11.0", proto.Version-1)
	offline.LastContact = now.Add(-time.Hour).Unix()

	got := reconcile(t, Config{Policy: PolicyOutdated}, outdated, otherRelease, current, alreadyAsked, notOptedIn, offline)
	assert.Equal(t, []int64{1}, got)
}

func TestReconcileReleasePolicy(t *testing.T) {
	outdated := optedIn(1, "6.11.0", proto.Version-1)
	otherRelease := optedIn(2, "6.11.1", proto.Version)
	current := optedIn(3, serverVersion, proto.Version)

	assert.Equal(t, []int64{1, 2}, reconcile(t, Config{Policy: PolicyRelease}, outdated, otherRelease, current))
}

func TestReconcileKeepsToTheParallelLimit(t *testing.T) {
	draining := optedIn(1, "6.11.0", proto.Version-1)
	draining.RestartRequested = true
	due := optedIn(2, "6.11.0", proto.Version-1)

	assert.Empty(t, reconcile(t, Config{Policy: PolicyOutdated, MaxParallel: 1}, draining, due))

	// a draining agent that stopped reporting no longer holds a slot
	draining.LastContact = now.Add(-time.Hour).Unix()
	assert.Equal(t, []int64{2}, reconcile(t, Config{Policy: PolicyOutdated, MaxParallel: 1}, draining, due))
}

func TestReconcilePrefersQueuedAgents(t *testing.T) {
	due := optedIn(1, "6.11.0", proto.Version-1)
	queued := optedIn(2, serverVersion, proto.Version)
	queued.RestartQueued = true

	assert.Equal(t, []int64{2}, reconcile(t, Config{Policy: PolicyOutdated, MaxParallel: 1}, due, queued))
}
