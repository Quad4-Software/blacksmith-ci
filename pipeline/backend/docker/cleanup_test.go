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
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"

	backend "codefloe.com/crowci/crow/v5/pipeline/backend/types"
)

// fakeCleanupClient implements just enough of client.APIClient for the stale
// container reaper. Any method the reaper does not call is left to the embedded
// nil interface and would panic if invoked, which keeps the fake honest.
type fakeCleanupClient struct {
	client.APIClient
	listed  []container.Summary
	removed []string
}

func (f *fakeCleanupClient) ContainerList(_ context.Context, _ container.ListOptions) ([]container.Summary, error) {
	return f.listed, nil
}

func (f *fakeCleanupClient) ContainerRemove(_ context.Context, containerID string, _ container.RemoveOptions) error {
	f.removed = append(f.removed, containerID)
	return nil
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

	err := e.cleanupStaleContainers(context.Background(), cutoff, result)
	assert.NoError(t, err)

	assert.ElementsMatch(t, []string{"exited_old_00", "dead_old_0000", "created_old_0"}, fake.removed,
		"reaper must remove only stale terminal/never-started containers and never active builds")
	assert.Equal(t, int64(3), result.ContainersRemoved)
	assert.Equal(t, int64(3), result.ContainersFound)
}
