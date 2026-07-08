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

	"github.com/stretchr/testify/assert"

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
	err = store.AgentUpdate(agent)
	assert.NoError(t, err)
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
