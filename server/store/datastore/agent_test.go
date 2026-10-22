// Copyright 2023 Woodpecker Authors
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

package datastore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
)

func TestAgentFindByToken(t *testing.T) {
	store, closer := newTestStore(t, new(model.Agent))
	defer closer()

	agent := &model.Agent{
		ID:    int64(1),
		Name:  "test",
		Token: "secret-token",
	}
	err := store.AgentCreate(agent)
	assert.NoError(t, err)

	_agent, err := store.AgentFindByToken(agent.Token)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, _agent.ID)

	_agent, err = store.AgentFindByToken("")
	assert.ErrorIs(t, err, ErrNoTokenProvided)
	assert.Nil(t, _agent)
}

func TestAgentFindByID(t *testing.T) {
	store, closer := newTestStore(t, new(model.Agent))
	defer closer()

	agent := &model.Agent{
		ID:    int64(1),
		Name:  "test",
		Token: "secret-token",
	}
	err := store.AgentCreate(agent)
	assert.NoError(t, err)

	_agent, err := store.AgentFind(agent.ID)
	assert.NoError(t, err)
	assert.Equal(t, "secret-token", _agent.Token)
}

func TestAgentFindSystemByHostname(t *testing.T) {
	store, closer := newTestStore(t, new(model.Agent))
	defer closer()

	// two orphaned system agents from the same host (e.g. repeated restarts)
	older := &model.Agent{OwnerID: model.IDNotSet, Token: "master", Hostname: "runner-1"}
	newer := &model.Agent{OwnerID: model.IDNotSet, Token: "master", Hostname: "runner-1"}
	// an unrelated system agent on a different host
	other := &model.Agent{OwnerID: model.IDNotSet, Token: "master", Hostname: "runner-2"}
	// a user-owned agent on the same host must never be matched
	owned := &model.Agent{OwnerID: 5, Token: "per-agent", Hostname: "runner-1"}
	for _, a := range []*model.Agent{older, newer, other, owned} {
		assert.NoError(t, store.AgentCreate(a))
	}

	// binds to the oldest system agent for the hostname
	found, err := store.AgentFindSystemByHostname("runner-1")
	assert.NoError(t, err)
	assert.Equal(t, older.ID, found.ID)
	assert.True(t, found.IsSystemAgent())

	// unknown hostname yields a not-found error
	_, err = store.AgentFindSystemByHostname("unknown")
	assert.Error(t, err)

	// empty hostname is rejected
	_, err = store.AgentFindSystemByHostname("")
	assert.ErrorIs(t, err, ErrNoHostnameProvided)
}

func TestAgentList(t *testing.T) {
	store, closer := newTestStore(t, new(model.Agent))
	defer closer()

	agent1 := &model.Agent{
		ID:   int64(1),
		Name: "test-1",
	}
	agent2 := &model.Agent{
		ID:   int64(2),
		Name: "test-2",
	}
	err := store.AgentCreate(agent1)
	assert.NoError(t, err)
	err = store.AgentCreate(agent2)
	assert.NoError(t, err)

	agents, err := store.AgentList(&model.AgentListOptions{ListOptions: model.ListOptions{All: true}})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(agents))

	agents, err = store.AgentList(&model.AgentListOptions{ListOptions: model.ListOptions{Page: 1, PerPage: 1}})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(agents))
}

func TestAgentUpdate(t *testing.T) {
	store, closer := newTestStore(t, new(model.Agent))
	defer closer()

	agent := &model.Agent{
		ID:    int64(1),
		Name:  "test",
		Token: "secret-token",
	}
	err := store.AgentCreate(agent)
	assert.NoError(t, err)

	agent.Backend = "local"
	agent.Capacity = 2
	agent.Version = "next-abcdef"
	agent.HostCPUs = 16
	// exceeds the 32-bit integer range, so this also guards the column type
	agent.HostMem = 68719476736
	err = store.AgentUpdate(agent)
	assert.NoError(t, err)

	updated, err := store.AgentFind(agent.ID)
	assert.NoError(t, err)
	assert.EqualValues(t, 16, updated.HostCPUs)
	assert.EqualValues(t, 68719476736, updated.HostMem)
}

func TestAgentListForOrg(t *testing.T) {
	store, closer := newTestStore(t, new(model.Agent))
	defer closer()

	agent1 := &model.Agent{
		ID:     int64(1),
		Name:   "test-1",
		OrgIDs: model.JSONSliceInt64{100},
	}
	agent2 := &model.Agent{
		ID:     int64(2),
		Name:   "test-2",
		OrgIDs: model.JSONSliceInt64{100},
	}
	agent3 := &model.Agent{
		ID:     int64(3),
		Name:   "test-3",
		OrgIDs: model.JSONSliceInt64{200},
	}
	assert.NoError(t, store.AgentCreate(agent1))
	assert.NoError(t, store.AgentCreate(agent2))
	assert.NoError(t, store.AgentCreate(agent3))

	agents, err := store.AgentListForOrg(100, &model.ListOptions{All: true})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(agents))
	assert.Equal(t, "test-1", agents[0].Name)
	assert.Equal(t, "test-2", agents[1].Name)

	agents, err = store.AgentListForOrg(200, &model.ListOptions{All: true})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(agents))
	assert.Equal(t, "test-3", agents[0].Name)

	agents, err = store.AgentListForOrg(100, &model.ListOptions{Page: 1, PerPage: 1})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(agents))
	assert.Equal(t, "test-1", agents[0].Name)
}

func TestAgentListForOrgMultiOrg(t *testing.T) {
	store, closer := newTestStore(t, new(model.Agent))
	defer closer()

	global := &model.Agent{Name: "global", OrgIDs: nil, Token: "t-global"}
	orgFive := &model.Agent{Name: "five", OrgIDs: model.JSONSliceInt64{5}, Token: "t-5"}
	shared := &model.Agent{Name: "shared", OrgIDs: model.JSONSliceInt64{5, 7}, Token: "t-57"}
	orgNine := &model.Agent{Name: "nine", OrgIDs: model.JSONSliceInt64{9}, Token: "t-9"}
	for _, a := range []*model.Agent{global, orgFive, shared, orgNine} {
		assert.NoError(t, store.AgentCreate(a))
	}

	got, err := store.AgentListForOrg(5, &model.ListOptions{All: true})
	assert.NoError(t, err)

	names := make([]string, 0, len(got))
	for _, a := range got {
		names = append(names, a.Name)
	}
	assert.ElementsMatch(t, []string{"five", "shared"}, names)
}

// TestAgentTimestamps pins down what XORM's created and updated tags used to
// do. The autoscaler times out an agent that never connects from its creation
// time, so an agent stored without one was never cleaned up.
func TestAgentTimestamps(t *testing.T) {
	store, closer := newTestStore(t, new(model.Agent))
	defer closer()

	before := time.Now().Unix()
	agent := &model.Agent{Name: "timestamps", Token: "timestamps-token", Capacity: 1}
	require.NoError(t, store.AgentCreate(agent))

	stored, err := store.AgentFind(agent.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, stored.Created, before)
	assert.GreaterOrEqual(t, stored.Updated, before)

	// An update moves Updated and leaves Created alone.
	stored.Created = 1
	stored.Updated = 1
	stored.Name = "renamed"
	require.NoError(t, store.AgentUpdate(stored))
	updated, err := store.AgentFind(agent.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), updated.Created)
	assert.GreaterOrEqual(t, updated.Updated, before)
}

func TestAgentCreateKeepsAGivenCreationTime(t *testing.T) {
	store, closer := newTestStore(t, new(model.Agent))
	defer closer()

	agent := &model.Agent{Name: "preset", Token: "preset-token", Created: 42}
	require.NoError(t, store.AgentCreate(agent))
	stored, err := store.AgentFind(agent.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(42), stored.Created)
}

func TestAgentSetRestartRequested(t *testing.T) {
	store, closer := newTestStore(t, new(model.Agent))
	defer closer()

	agent := &model.Agent{ID: 1, Name: "test", Token: "token"}
	assert.NoError(t, store.AgentCreate(agent))

	// a health report loaded the agent before the request was made
	stale, err := store.AgentFind(agent.ID)
	assert.NoError(t, err)

	assert.NoError(t, store.AgentSetRestartRequested(agent.ID, true))

	// writing the stale copy back must not withdraw the request
	stale.LastContact = 42
	assert.NoError(t, store.AgentUpdate(stale))

	got, err := store.AgentFind(agent.ID)
	assert.NoError(t, err)
	assert.True(t, got.RestartRequested)
	assert.EqualValues(t, 42, got.LastContact)

	assert.NoError(t, store.AgentSetRestartRequested(agent.ID, false))
	got, err = store.AgentFind(agent.ID)
	assert.NoError(t, err)
	assert.False(t, got.RestartRequested)
}

func TestAgentRestartQueue(t *testing.T) {
	store, closer := newTestStore(t, new(model.Agent))
	defer closer()

	queued := &model.Agent{ID: 1, Name: "queued", Token: "token-1"}
	requested := &model.Agent{ID: 2, Name: "requested", Token: "token-2"}
	assert.NoError(t, store.AgentCreate(queued))
	assert.NoError(t, store.AgentCreate(requested))
	assert.NoError(t, store.AgentSetRestartRequested(requested.ID, true))

	stale, err := store.AgentFind(queued.ID)
	assert.NoError(t, err)

	assert.NoError(t, store.AgentQueueRestart(queued.ID))
	// an agent already draining is not queued again
	assert.NoError(t, store.AgentQueueRestart(requested.ID))

	// writing a stale copy back must not drop the queued restart
	assert.NoError(t, store.AgentUpdate(stale))

	got, err := store.AgentFind(queued.ID)
	assert.NoError(t, err)
	assert.True(t, got.RestartQueued)
	got, err = store.AgentFind(requested.ID)
	assert.NoError(t, err)
	assert.False(t, got.RestartQueued)

	assert.NoError(t, store.AgentRequestAutoRestart(queued.ID, "6.12.0"))
	stale.LastContact = 7
	assert.NoError(t, store.AgentUpdate(stale))
	got, err = store.AgentFind(queued.ID)
	assert.NoError(t, err)
	assert.True(t, got.RestartRequested)
	assert.False(t, got.RestartQueued)
	assert.Equal(t, "6.12.0", got.AutoRestartVersion)

	// clearing the request on re-registration keeps the release it was requested under
	assert.NoError(t, store.AgentSetRestartRequested(queued.ID, false))
	got, err = store.AgentFind(queued.ID)
	assert.NoError(t, err)
	assert.False(t, got.RestartRequested)
	assert.Equal(t, "6.12.0", got.AutoRestartVersion)
}

func TestAgentProvisioningRoundTrip(t *testing.T) {
	store, closer := newTestStore(t, new(model.Agent))
	defer closer()

	agent := &model.Agent{Name: "provisioning", Token: "provisioning-token"}
	require.NoError(t, store.AgentCreate(agent))
	agent.AddProvisioningPhase(model.ProvisioningPhase{Phase: "Installing Hyper-V", At: 10})
	agent.AddProvisioningPhase(model.ProvisioningPhase{Phase: "Building the golden image", Detail: "DISM failed", Failed: true, At: 20})
	require.NoError(t, store.AgentUpdate(agent))

	stored, err := store.AgentFindByToken("provisioning-token")
	require.NoError(t, err)
	assert.Equal(t, agent.Provisioning, stored.Provisioning)
}

func TestAutoscalerProvisioningFieldsRoundTrip(t *testing.T) {
	store, closer := newTestStore(t, new(model.Autoscaler))
	defer closer()

	autoscaler := &model.Autoscaler{Name: "hyperv", Token: "hyperv-token"}
	require.NoError(t, store.AutoscalerCreate(autoscaler))
	autoscaler.PendingAgentIDs = model.JSONSliceInt64{4, 5}
	autoscaler.ProvisionTimeout = 1500
	require.NoError(t, store.AutoscalerUpdate(autoscaler))

	stored, err := store.AutoscalerFind(autoscaler.ID)
	require.NoError(t, err)
	assert.Equal(t, model.JSONSliceInt64{4, 5}, stored.PendingAgentIDs)
	assert.Equal(t, int64(1500), stored.ProvisionTimeout)
}
