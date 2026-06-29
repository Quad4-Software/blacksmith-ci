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

//go:build !windows

package podman

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	nettypes "go.podman.io/common/libnetwork/types"
	"go.podman.io/podman/v6/pkg/bindings/containers"
	"go.podman.io/podman/v6/pkg/bindings/images"
	"go.podman.io/podman/v6/pkg/specgen"

	"codefloe.com/crowci/crow/v5/pipeline/backend/common"
	backend "codefloe.com/crowci/crow/v5/pipeline/backend/types"
)

const (
	keepaliveLabel               = "crowci/podman-network-keepalive"
	keepaliveContainerNamePrefix = "crow-podman-keepalive-"
	keepaliveDefaultImage        = "docker.io/library/busybox:1.37.0"
	keepaliveDefaultHost         = "example.com"
)

func keepaliveContainerName(taskUUID string) string {
	return common.SanitizeAndTruncateName(keepaliveContainerNamePrefix+taskUUID, common.MaxContainerNameLength)
}

func keepaliveIntervalSeconds(interval time.Duration) int {
	if interval <= 0 {
		return 0
	}

	return max(1, int(math.Ceil(interval.Seconds())))
}

func keepaliveScript(host string, intervalSeconds int) string {
	return fmt.Sprintf("while :; do nslookup %s >/dev/null 2>&1 || true; sleep %d; done", host, intervalSeconds)
}

func (e *podman) startNetworkKeepaliveContainer(conf *backend.Config, taskUUID string) error {
	if e.config.networkKeepaliveInterval <= 0 {
		return nil
	}
	if conf.Network == "" {
		return nil
	}

	containerName := keepaliveContainerName(taskUUID)
	spec := specgen.NewSpecGenerator(e.config.networkKeepaliveImage, false)
	spec.Name = containerName
	spec.Labels = map[string]string{
		common.LabelTaskUUID: taskUUID,
		keepaliveLabel:       "true",
	}
	spec.Entrypoint = []string{
		"/bin/sh",
		"-ceu",
		keepaliveScript(e.config.networkKeepaliveHost, keepaliveIntervalSeconds(e.config.networkKeepaliveInterval)),
	}
	spec.Networks = map[string]nettypes.PerNetworkOptions{
		conf.Network: {},
	}

	response, err := containers.CreateWithSpec(e.conn, spec, nil)
	if err != nil {
		if strings.Contains(err.Error(), "name is in use") || strings.Contains(err.Error(), "already in use") {
			log.Debug().Str("container", containerName).Msg("network keepalive helper already exists, reusing")
			return nil
		}

		if strings.Contains(err.Error(), "image not known") || strings.Contains(err.Error(), "no such image") {
			if _, pullErr := images.Pull(e.conn, e.config.networkKeepaliveImage, new(images.PullOptions)); pullErr != nil {
				return fmt.Errorf("failed to pull keepalive image %s: %w", e.config.networkKeepaliveImage, pullErr)
			}
			response, err = containers.CreateWithSpec(e.conn, spec, nil)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to create network keepalive helper container: %w", err)
	}

	if err := containers.Start(e.conn, response.ID, nil); err != nil {
		return fmt.Errorf("failed to start network keepalive helper container: %w", err)
	}

	log.Debug().
		Str("container", containerName).
		Str("network", conf.Network).
		Dur("interval", e.config.networkKeepaliveInterval).
		Msg("started podman network keepalive helper")

	return nil
}

func (e *podman) stopNetworkKeepaliveContainer(taskUUID string) error {
	containerName := keepaliveContainerName(taskUUID)

	if err := containers.Kill(e.conn, containerName, new(containers.KillOptions).WithSignal("SIGKILL")); err != nil && !isContainerNotFoundOrNotRunning(err) {
		return fmt.Errorf("failed to stop keepalive helper container: %w", err)
	}

	_, err := containers.Remove(e.conn, containerName, new(containers.RemoveOptions).WithForce(true).WithVolumes(true))
	if err != nil && !isContainerNotFoundOrNotRunning(err) {
		return fmt.Errorf("failed to remove keepalive helper container: %w", err)
	}

	return nil
}
