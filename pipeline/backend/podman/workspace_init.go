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
	"strings"

	"github.com/rs/zerolog/log"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/bindings/containers"
	"go.podman.io/podman/v6/pkg/bindings/images"
	"go.podman.io/podman/v6/pkg/specgen"

	"codefloe.com/crowci/crow/v6/pipeline/backend/common"
	backend "codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// initWorkspace seeds deterministic ownership and permissions on the freshly
// created workspace volume before any pipeline step or plugin runs.
//
// Like the Docker backend, Podman mounts a single empty named volume as the
// workspace for every container. A fresh named volume is root-owned, so its
// ownership ends up decided by whichever container writes into it first: a
// command step runs a generated `mkdir` as the step image's (often non-root)
// user, while a plugin runs its own entrypoint as root. That made workspace
// ownership depend purely on whether the first pipeline item was a step or a
// plugin (#1345).
//
// Running this agent-controlled init container up front makes the workspace
// tree world-writable with the sticky bit (1777, like /tmp) so any step user,
// root or not, can populate it regardless of ordering. It lives in
// SetupWorkflow rather than in a synthesized pipeline step so it stays
// invisible to and uninfluenceable by untrusted pipeline YAML, and it only
// touches the workspace data directory, never plugin executables.
func (e *podman) initWorkspace(conf *backend.Config, taskUUID string) error {
	if e.config.workspaceInitImage == "" {
		// operator opted out of workspace initialization
		return nil
	}

	base, workdirs := common.CollectWorkspacePaths(conf)
	if base == "" {
		return nil
	}

	containerName := common.SanitizeAndTruncateName(common.WorkspaceInitPrefix+taskUUID, common.MaxContainerNameLength)
	spec := specgen.NewSpecGenerator(e.config.workspaceInitImage, false)
	spec.Name = containerName
	spec.Labels = map[string]string{
		common.LabelTaskUUID: taskUUID,
	}
	spec.Entrypoint = []string{"/bin/sh", "-c", common.BuildWorkspaceInitScript(base, workdirs)}
	spec.Volumes = []*specgen.NamedVolume{
		{Name: conf.Volume, Dest: base},
	}

	if err := e.runWorkspaceInitContainer(spec, containerName); err != nil {
		return fmt.Errorf("initialize workspace ownership: %w", err)
	}
	return nil
}

func (e *podman) runWorkspaceInitContainer(spec *specgen.SpecGenerator, name string) error {
	response, err := containers.CreateWithSpec(e.conn, spec, nil)
	if err != nil {
		if strings.Contains(err.Error(), "image not known") || strings.Contains(err.Error(), "no such image") {
			if _, pErr := images.Pull(e.conn, e.config.workspaceInitImage, new(images.PullOptions)); pErr != nil {
				return fmt.Errorf("failed to pull workspace-init image %s: %w", e.config.workspaceInitImage, pErr)
			}
			response, err = containers.CreateWithSpec(e.conn, spec, nil)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to create workspace-init container: %w", err)
	}
	defer func() {
		if _, rmErr := containers.Remove(e.conn, name, new(containers.RemoveOptions).WithForce(true).WithVolumes(true)); rmErr != nil && !isContainerNotFoundOrNotRunning(rmErr) {
			log.Error().Err(rmErr).Msgf("could not remove workspace-init container '%s'", name)
		}
	}()

	if err := containers.Start(e.conn, response.ID, nil); err != nil {
		return fmt.Errorf("failed to start workspace-init container: %w", err)
	}

	exitCode, err := containers.Wait(e.conn, name, new(containers.WaitOptions).WithCondition([]define.ContainerStatus{define.ContainerStateStopped, define.ContainerStateExited}))
	if err != nil {
		return fmt.Errorf("failed to wait for workspace-init container: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("workspace-init container exited with code %d", exitCode)
	}
	return nil
}
