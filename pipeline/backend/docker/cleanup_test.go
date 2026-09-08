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

package docker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"

	backend "codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// fakeCleanupClient implements just enough of client.APIClient for the stale
// container reaper. Any method the reaper does not call is left to the embedded
// nil interface and would panic if invoked, which keeps the fake honest.
type fakeCleanupClient struct {
	client.APIClient
	listed  []container.Summary
	removed []string
}

// ContainerList mirrors Docker: with All left false only containers Docker
// considers active (running, restarting, paused) are returned, which is what the
// live-task detection relies on. With All set, every container is returned.
func (f *fakeCleanupClient) ContainerList(_ context.Context, opts client.ContainerListOptions) (client.ContainerListResult, error) {
	if opts.All {
		return client.ContainerListResult{Items: f.listed}, nil
	}
	var running []container.Summary
	for _, c := range f.listed {
		switch c.State {
		case container.StateRunning, container.StateRestarting, container.StatePaused:
			running = append(running, c)
		}
	}
	return client.ContainerListResult{Items: running}, nil
}

func (f *fakeCleanupClient) ContainerRemove(_ context.Context, containerID string, _ client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
	f.removed = append(f.removed, containerID)
	return client.ContainerRemoveResult{}, nil
}

// NetworkList and VolumeList let CleanupStaleResources run end-to-end; the reaper
// only cares about containers in these tests, so both return nothing.
func (f *fakeCleanupClient) NetworkList(_ context.Context, _ client.NetworkListOptions) (client.NetworkListResult, error) {
	return client.NetworkListResult{}, nil
}

func (f *fakeCleanupClient) VolumeList(_ context.Context, _ client.VolumeListOptions) (client.VolumeListResult, error) {
	return client.VolumeListResult{}, nil
}

func TestCleanupStaleContainersNeverRemovesActiveBuilds(t *testing.T) {
	old := time.Now().Add(-2 * time.Hour).Unix()
	fresh := time.Now().Unix()

	fake := &fakeCleanupClient{listed: []container.Summary{
		// Active builds must never be reaped, however old they are.
		{ID: "running_old_0", State: container.StateRunning, Created: old},
		{ID: "paused_old_00", State: container.StatePaused, Created: old},
		{ID: "restarting_old", State: container.StateRestarting, Created: old},
		{ID: "removing_old_0", State: container.StateRemoving, Created: old},
		// Terminal / never-started orphans past the cutoff are reapable.
		{ID: "exited_old_00", State: container.StateExited, Created: old},
		{ID: "dead_old_0000", State: container.StateDead, Created: old},
		{ID: "created_old_0", State: container.StateCreated, Created: old},
		// Terminal but still fresh: below the cutoff, so keep it.
		{ID: "exited_fresh_0", State: container.StateExited, Created: fresh},
	}}

	e := &docker{client: fake}
	result := &backend.CleanupResult{}
	cutoff := time.Now().Add(-time.Hour)

	err := e.cleanupStaleContainers(context.Background(), cutoff, nil, result)
	assert.NoError(t, err)

	assert.ElementsMatch(t, []string{"exited_old_00", "dead_old_0000", "created_old_0"}, fake.removed,
		"reaper must remove only stale terminal/never-started containers and never active builds")
	assert.Equal(t, int64(3), result.ContainersRemoved)
	assert.Equal(t, int64(3), result.ContainersFound)
}

func TestCleanupStaleContainersSkipsActiveTaskUUIDs(t *testing.T) {
	old := time.Now().Add(-2 * time.Hour).Unix()

	// A build that has legitimately been running longer than the stale threshold
	// cycles its step containers through terminal (exited) and never-started
	// (created) states between steps. Those must survive as long as the task is
	// still running on this agent, even though their state and age would otherwise
	// make them reapable.
	fake := &fakeCleanupClient{listed: []container.Summary{
		{ID: "live_exited_00", State: container.StateExited, Created: old, Labels: map[string]string{LabelTaskUUID: "task-live"}},
		{ID: "live_created_0", State: container.StateCreated, Created: old, Labels: map[string]string{LabelTaskUUID: "task-live"}},
		{ID: "live_dead_0000", State: container.StateDead, Created: old, Labels: map[string]string{LabelTaskUUID: "task-live"}},
		// A terminal orphan from a task that is no longer running is still reaped.
		{ID: "orphan_exited0", State: container.StateExited, Created: old, Labels: map[string]string{LabelTaskUUID: "task-gone"}},
	}}

	e := &docker{client: fake}
	result := &backend.CleanupResult{}
	cutoff := time.Now().Add(-time.Hour)
	active := map[string]struct{}{"task-live": {}}

	err := e.cleanupStaleContainers(context.Background(), cutoff, active, result)
	assert.NoError(t, err)

	assert.ElementsMatch(t, []string{"orphan_exited0"}, fake.removed,
		"reaper must skip every container of an in-flight build and reap only orphans")
	assert.Equal(t, int64(1), result.ContainersRemoved)
	assert.Equal(t, int64(1), result.ContainersFound)
}

func TestCleanupStaleResourcesProtectsLiveButUntrackedTasks(t *testing.T) {
	old := time.Now().Add(-2 * time.Hour).Unix()

	// task-adopted models a build the agent does not track in its in-memory active
	// set (e.g. an adopted workflow whose in-memory ID differs from the container
	// label, or a build owned by another agent sharing this daemon). It has one
	// running step plus an old exited step from earlier in the same build. Because
	// a running container proves the build is live, none of its containers may be
	// reaped. task-gone is a genuine orphan with no running container and is reaped.
	fake := &fakeCleanupClient{listed: []container.Summary{
		{ID: "adopted_run00", State: container.StateRunning, Created: old, Labels: map[string]string{LabelTaskUUID: "task-adopted"}},
		{ID: "adopted_exit0", State: container.StateExited, Created: old, Labels: map[string]string{LabelTaskUUID: "task-adopted"}},
		{ID: "gone_exited_0", State: container.StateExited, Created: old, Labels: map[string]string{LabelTaskUUID: "task-gone"}},
	}}

	e := &docker{client: fake}

	// The agent reports no active tasks: protection must come from daemon liveness.
	result, err := e.CleanupStaleResources(context.Background(), time.Hour, nil)
	assert.NoError(t, err)

	assert.ElementsMatch(t, []string{"gone_exited_0"}, fake.removed,
		"a build with any running container must be fully protected even when the agent does not track it")
	assert.Equal(t, int64(1), result.ContainersRemoved)
}

type fakeNetworkCleanupClient struct {
	client.APIClient
	inspection client.NetworkInspectResult
	inspectErr error
	removed    []string
}

func (f *fakeNetworkCleanupClient) NetworkList(context.Context, client.NetworkListOptions) (client.NetworkListResult, error) {
	return client.NetworkListResult{Items: []network.Summary{{Network: network.Network{
		ID: "network-id", Name: "crow_test_default", Created: time.Now().Add(-2 * time.Hour),
	}}}}, nil
}

func (f *fakeNetworkCleanupClient) NetworkInspect(context.Context, string, client.NetworkInspectOptions) (client.NetworkInspectResult, error) {
	return f.inspection, f.inspectErr
}

func (f *fakeNetworkCleanupClient) NetworkRemove(_ context.Context, id string, _ client.NetworkRemoveOptions) (client.NetworkRemoveResult, error) {
	f.removed = append(f.removed, id)
	return client.NetworkRemoveResult{}, nil
}

func TestCleanupStaleNetworksInspectsEndpoints(t *testing.T) {
	for _, tt := range []struct {
		name       string
		endpoints  map[string]network.EndpointResource
		inspectErr error
		removed    int
	}{
		{name: "empty network", removed: 1},
		{name: "attached container", endpoints: map[string]network.EndpointResource{"container-id": {}}},
		{name: "inspection failed", inspectErr: errors.New("daemon unavailable")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeNetworkCleanupClient{
				inspection: client.NetworkInspectResult{Network: network.Inspect{Containers: tt.endpoints}},
				inspectErr: tt.inspectErr,
			}
			e := &docker{client: fake}
			result := &backend.CleanupResult{}
			assert.NoError(t, e.cleanupStaleNetworks(t.Context(), time.Now().Add(-time.Hour), nil, result))
			assert.Len(t, fake.removed, tt.removed)
			assert.Equal(t, int64(tt.removed), result.NetworksRemoved)
		})
	}
}
