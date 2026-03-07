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

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/rs/zerolog/log"

	backend "codefloe.com/crowci/crow/v5/pipeline/backend/types"
)

// CleanupStaleResources finds and removes stale Docker containers, networks, and volumes
// created by Crow pipelines that were not properly cleaned up.
func (e *docker) CleanupStaleResources(ctx context.Context, staleThreshold time.Duration) (*backend.CleanupResult, error) {
	result := &backend.CleanupResult{}
	cutoff := time.Now().Add(-staleThreshold)

	// Clean up stale containers (identified by crowci labels)
	if err := e.cleanupStaleContainers(ctx, cutoff, result); err != nil {
		log.Error().Err(err).Msg("error during stale container cleanup")
	}

	// Clean up stale networks (matching wp_*_default or crow_*_default patterns)
	if err := e.cleanupStaleNetworks(ctx, cutoff, result); err != nil {
		log.Error().Err(err).Msg("error during stale network cleanup")
	}

	// Clean up stale volumes (matching wp_*_default or crow_*_default patterns)
	if err := e.cleanupStaleVolumes(ctx, cutoff, result); err != nil {
		log.Error().Err(err).Msg("error during stale volume cleanup")
	}

	return result, nil
}

// isCrowResource returns true if the name matches the crow/wp pipeline naming convention.
func isCrowResource(name string) bool {
	return (strings.HasPrefix(name, "wp_") || strings.HasPrefix(name, "crow_")) &&
		strings.HasSuffix(name, "_default")
}

func (e *docker) cleanupStaleContainers(ctx context.Context, cutoff time.Time, result *backend.CleanupResult) error {
	containers, err := e.client.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", LabelTaskUUID),
		),
	})
	if err != nil {
		return err
	}

	for _, c := range containers {
		created := time.Unix(c.Created, 0)
		if created.Before(cutoff) {
			result.ContainersFound++
			containerID := c.ID[:12]

			log.Debug().
				Str("container", containerID).
				Str("name", strings.Join(c.Names, ",")).
				Time("created", created).
				Msg("removing stale container")

			if err := e.client.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true, RemoveVolumes: true}); err != nil {
				log.Warn().Err(err).Str("container", containerID).Msg("failed to remove stale container")
			} else {
				result.ContainersRemoved++
			}
		}
	}

	return nil
}

func (e *docker) cleanupStaleNetworks(ctx context.Context, cutoff time.Time, result *backend.CleanupResult) error {
	networks, err := e.client.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return err
	}

	for _, n := range networks {
		if !isCrowResource(n.Name) {
			continue
		}

		created, err := time.Parse(time.RFC3339Nano, n.Created.Format(time.RFC3339Nano))
		if err != nil {
			continue
		}

		if !created.Before(cutoff) {
			continue
		}

		// Only remove networks with zero connected containers
		if len(n.Containers) > 0 {
			continue
		}

		result.NetworksFound++

		log.Debug().
			Str("network", n.Name).
			Time("created", created).
			Msg("removing stale network")

		if err := e.client.NetworkRemove(ctx, n.ID); err != nil {
			log.Warn().Err(err).Str("network", n.Name).Msg("failed to remove stale network")
		} else {
			result.NetworksRemoved++
		}
	}

	return nil
}

func (e *docker) cleanupStaleVolumes(ctx context.Context, cutoff time.Time, result *backend.CleanupResult) error {
	volumes, err := e.client.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return err
	}

	for _, v := range volumes.Volumes {
		if !isCrowResource(v.Name) {
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

		if err := e.client.VolumeRemove(ctx, v.Name, true); err != nil {
			log.Warn().Err(err).Str("volume", v.Name).Msg("failed to remove stale volume")
		} else {
			result.VolumesRemoved++
		}
	}

	return nil
}
