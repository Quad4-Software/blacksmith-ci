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
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
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
func (f *fakeCleanupClient) ContainerList(_ context.Context, opts container.ListOptions) ([]container.Summary, error) {
	if opts.All {
		return f.listed, nil
	}
	var running []container.Summary
	for _, c := range f.listed {
		switch c.State {
		case container.StateRunning, container.StateRestarting, container.StatePaused:
			running = append(running, c)
		}
	}
	return running, nil
}

func (f *fakeCleanupClient) ContainerRemove(_ context.Context, containerID string, _ container.RemoveOptions) error {
	f.removed = append(f.removed, containerID)
	return nil
}

// NetworkList and VolumeList let CleanupStaleResources run end-to-end; the reaper
// only cares about containers in these tests, so both return nothing.
func (f *fakeCleanupClient) NetworkList(_ context.Context, _ network.ListOptions) ([]network.Summary, error) {
	return nil, nil
}

func (f *fakeCleanupClient) VolumeList(_ context.Context, _ volume.ListOptions) (volume.ListResponse, error) {
	return volume.ListResponse{}, nil
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
