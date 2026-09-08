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
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/rs/zerolog/log"

	backend "codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// CleanupStaleResources finds and removes stale Docker containers, networks, and volumes
// created by Crow pipelines that were not properly cleaned up. Resources belonging to a
// build that is still executing are never touched, regardless of age or state.
func (e *docker) CleanupStaleResources(ctx context.Context, staleThreshold time.Duration, activeTaskUUIDs map[string]struct{}) (*backend.CleanupResult, error) {
	result := &backend.CleanupResult{}
	cutoff := time.Now().Add(-staleThreshold)

	// Build the set of task UUIDs whose resources must be protected. Two
	// independent signals are combined so a live build is never reclaimed:
	//
	//  1. activeTaskUUIDs — workflows this agent tracks in memory. This covers the
	//     brief between-steps window where a build momentarily has no running
	//     container (previous step torn down, next step not yet created).
	//  2. live containers on this daemon — any task UUID that still has a running
	//     container. This is the authoritative signal and does not depend on the
	//     agent's in-memory bookkeeping, so it also protects builds whose in-memory
	//     ID differs from the container label (adopted workflows, whose task UUID
	//     and workflow ID are distinct) and builds owned by another agent sharing
	//     this daemon. An actively executing build always has at least one running
	//     container.
	protected := make(map[string]struct{}, len(activeTaskUUIDs))
	for uuid := range activeTaskUUIDs {
		protected[uuid] = struct{}{}
	}
	if err := e.addLiveTaskUUIDs(ctx, protected); err != nil {
		log.Warn().Err(err).Msg("could not determine live task UUIDs for cleanup; relying on agent-reported active set only")
	}

	// Clean up stale containers (identified by crowci labels)
	if err := e.cleanupStaleContainers(ctx, cutoff, protected, result); err != nil {
		log.Error().Err(err).Msg("error during stale container cleanup")
	}

	// Clean up stale networks (matching wp_*_default or crow_*_default patterns)
	if err := e.cleanupStaleNetworks(ctx, cutoff, protected, result); err != nil {
		log.Error().Err(err).Msg("error during stale network cleanup")
	}

	// Clean up stale volumes (matching wp_*_default or crow_*_default patterns)
	if err := e.cleanupStaleVolumes(ctx, cutoff, protected, result); err != nil {
		log.Error().Err(err).Msg("error during stale volume cleanup")
	}

	return result, nil
}

// addLiveTaskUUIDs adds to protected the task UUID of every crow container that is
// currently running on this daemon. A ListOptions with All left false returns only
// containers Docker considers active (running, restarting, paused), which is exactly
// the set that proves a build is still executing.
func (e *docker) addLiveTaskUUIDs(ctx context.Context, protected map[string]struct{}) error {
	containers, err := e.client.ContainerList(ctx, client.ContainerListOptions{
		Filters: client.Filters{}.Add("label", LabelTaskUUID),
	})
	if err != nil {
		return err
	}

	for _, c := range containers.Items {
		if uuid := c.Labels[LabelTaskUUID]; uuid != "" {
			protected[uuid] = struct{}{}
		}
	}

	return nil
}

// isCrowResource returns true if the name matches the crow/wp pipeline naming convention.
func isCrowResource(name string) bool {
	return (strings.HasPrefix(name, "wp_") || strings.HasPrefix(name, "crow_")) &&
		strings.HasSuffix(name, "_default")
}

// belongsToActiveBuild reports whether the given LabelTaskUUID value identifies a
// build that is still executing (see CleanupStaleResources for how the protected
// set is built). Such resources are never reaped: an in-flight build legitimately
// passes its containers through terminal (exited) and never-started (created)
// states between steps, and keeps its volume and network for the whole run, so age
// and container state alone cannot distinguish a live build from an orphan.
func belongsToActiveBuild(taskUUID string, protectedTaskUUIDs map[string]struct{}) bool {
	if taskUUID == "" {
		return false
	}
	_, ok := protectedTaskUUIDs[taskUUID]
	return ok
}

// isReapableContainerState reports whether a container in the given state is safe
// for the stale reaper to force-remove. Only terminal states (exited/dead) and
// "created" (created but never started, i.e. an orphan) qualify. Anything that
// could belong to an active build (running, paused, restarting) or is already
// being torn down (removing) is left alone, as is any unrecognized future state.
// This prevents the reaper from ever killing a still-running pipeline, even one
// that has been running longer than the stale threshold.
func isReapableContainerState(state container.ContainerState) bool {
	switch state {
	case container.StateExited, container.StateDead, container.StateCreated:
		return true
	default:
		return false
	}
}

func (e *docker) cleanupStaleContainers(ctx context.Context, cutoff time.Time, protectedTaskUUIDs map[string]struct{}, result *backend.CleanupResult) error {
	containers, err := e.client.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: client.Filters{}.Add("label", LabelTaskUUID),
	})
	if err != nil {
		return err
	}

	for _, c := range containers.Items {
		// Never touch containers belonging to a build that is still executing,
		// regardless of age or state: a live build cycles its step containers
		// through exited/created states between steps.
		if belongsToActiveBuild(c.Labels[LabelTaskUUID], protectedTaskUUIDs) {
			continue
		}
		// Leave running/paused/restarting/removing and any unknown state alone.
		if !isReapableContainerState(c.State) {
			continue
		}
		created := time.Unix(c.Created, 0)
		if created.Before(cutoff) {
			result.ContainersFound++
			containerID := c.ID[:12]

			log.Debug().
				Str("container", containerID).
				Str("name", strings.Join(c.Names, ",")).
				Time("created", created).
				Msg("removing stale container")

			if _, err := e.client.ContainerRemove(ctx, c.ID, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true}); err != nil {
				log.Warn().Err(err).Str("container", containerID).Msg("failed to remove stale container")
			} else {
				result.ContainersRemoved++
			}
		}
	}

	return nil
}

func (e *docker) cleanupStaleNetworks(ctx context.Context, cutoff time.Time, protectedTaskUUIDs map[string]struct{}, result *backend.CleanupResult) error {
	networks, err := e.client.NetworkList(ctx, client.NetworkListOptions{})
	if err != nil {
		return err
	}

	for _, n := range networks.Items {
		if !isCrowResource(n.Name) {
			continue
		}

		// Networks created by SetupWorkflow carry the task UUID label; never remove
		// one that belongs to a build running on this agent, even in the brief
		// between-steps window where no container is attached.
		if belongsToActiveBuild(n.Labels[LabelTaskUUID], protectedTaskUUIDs) {
			continue
		}

		created, err := time.Parse(time.RFC3339Nano, n.Created.Format(time.RFC3339Nano))
		if err != nil {
			continue
		}

		if !created.Before(cutoff) {
			continue
		}

		// List summaries omit endpoints; inspect before removing a network.
		inspection, err := e.client.NetworkInspect(ctx, n.ID, client.NetworkInspectOptions{})
		if err != nil {
			log.Warn().Err(err).Str("network", n.Name).Msg("could not inspect network for cleanup")
			continue
		}
		if len(inspection.Network.Containers) > 0 {
			continue
		}

		result.NetworksFound++

		log.Debug().
			Str("network", n.Name).
			Time("created", created).
			Msg("removing stale network")

		if _, err := e.client.NetworkRemove(ctx, n.ID, client.NetworkRemoveOptions{}); err != nil {
			log.Warn().Err(err).Str("network", n.Name).Msg("failed to remove stale network")
		} else {
			result.NetworksRemoved++
		}
	}

	return nil
}

func (e *docker) cleanupStaleVolumes(ctx context.Context, cutoff time.Time, protectedTaskUUIDs map[string]struct{}, result *backend.CleanupResult) error {
	volumes, err := e.client.VolumeList(ctx, client.VolumeListOptions{})
	if err != nil {
		return err
	}

	for _, v := range volumes.Items {
		if !isCrowResource(v.Name) && v.Labels[LabelTaskUUID] == "" {
			continue
		}

		// Never remove a volume that belongs to a build running on this agent; the
		// workspace and temp volumes live for the whole run.
		if belongsToActiveBuild(v.Labels[LabelTaskUUID], protectedTaskUUIDs) {
			continue
		}

		created, err := time.Parse(time.RFC3339Nano, v.CreatedAt)
		if err != nil {
			continue
		}

		if !created.Before(cutoff) {
			continue
		}

		// UsageData.RefCount == -1 or > 0 means in use
		if v.UsageData != nil && v.UsageData.RefCount > 0 {
			continue
		}

		result.VolumesFound++

		log.Debug().
			Str("volume", v.Name).
			Time("created", created).
			Msg("removing stale volume")

		if _, err := e.client.VolumeRemove(ctx, v.Name, client.VolumeRemoveOptions{Force: true}); err != nil {
			log.Warn().Err(err).Str("volume", v.Name).Msg("failed to remove stale volume")
		} else {
			result.VolumesRemoved++
		}
	}

	return nil
}
