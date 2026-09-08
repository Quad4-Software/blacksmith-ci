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
	"fmt"
	"io"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/pipeline/backend/common"
	backend "codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// initWorkspace seeds deterministic ownership and permissions on the freshly
// created workspace volume before any pipeline step or plugin runs.
//
// The workspace is a single named volume mounted at the same base for every
// container. A fresh named volume is created empty and root-owned, so its
// ownership ends up being decided by whichever container writes into it first:
// a command step runs a generated `mkdir` as the step image's (often non-root)
// user, while a plugin runs its own entrypoint as root. That made workspace
// ownership depend purely on whether the first pipeline item was a step or a
// plugin (#1345).
//
// Running this agent-controlled init container up front makes the workspace
// tree world-writable with the sticky bit (1777, like /tmp) so that any step
// user, root or not, can populate it regardless of ordering. It deliberately
// lives in SetupWorkflow rather than in a synthesized pipeline step so it stays
// invisible to and uninfluenceable by untrusted pipeline YAML, and it only
// touches the workspace data directory, never plugin executables.
func (e *docker) initWorkspace(ctx context.Context, conf *backend.Config, taskUUID string) error {
	if e.config.workspaceInitImage == "" {
		// operator opted out of workspace initialization
		return nil
	}
	if e.info.OSType == "windows" {
		// Windows volumes do not share the POSIX ownership model that causes
		// this inconsistency, and the Linux init image cannot run there.
		return nil
	}

	base, workdirs := common.CollectWorkspacePaths(conf)
	if base == "" {
		return nil
	}

	containerName := common.SanitizeAndTruncateName(common.WorkspaceInitPrefix+taskUUID, common.MaxContainerNameLength)
	config := &container.Config{
		Image:      e.config.workspaceInitImage,
		Entrypoint: []string{"/bin/sh", "-c", common.BuildWorkspaceInitScript(base, workdirs)},
		Labels: map[string]string{
			LabelTaskUUID: taskUUID,
		},
		// no User override: the init image runs as root so it may chmod the
		// workspace tree.
	}
	hostConfig := &container.HostConfig{
		Binds: []string{conf.Volume + ":" + base},
	}

	if err := e.runWorkspaceInitContainer(ctx, config, hostConfig, containerName); err != nil {
		return fmt.Errorf("initialize workspace ownership: %w", err)
	}
	return nil
}

func (e *docker) runWorkspaceInitContainer(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, name string) error {
	_, err := e.client.ContainerCreate(ctx, client.ContainerCreateOptions{Config: config, HostConfig: hostConfig, Name: name})
	if errdefs.IsNotFound(err) {
		if pullErr := e.pullImageSilently(ctx, config.Image); pullErr != nil {
			return pullErr
		}
		_, err = e.client.ContainerCreate(ctx, client.ContainerCreateOptions{Config: config, HostConfig: hostConfig, Name: name})
	}
	if err != nil {
		return wrapDaemonError("create container", name, err)
	}
	defer func() {
		if _, rmErr := e.client.ContainerRemove(ctx, name, removeOpts); rmErr != nil && !isErrContainerNotFoundOrNotRunning(rmErr) {
			log.Error().Err(rmErr).Msgf("could not remove workspace-init container '%s'", name)
		}
	}()

	if _, err := e.client.ContainerStart(ctx, name, client.ContainerStartOptions{}); err != nil {
		return wrapDaemonError("start container", name, err)
	}

	wait := e.client.ContainerWait(ctx, name, client.ContainerWaitOptions{})
	select {
	case <-wait.Result:
	case waitErr := <-wait.Error:
		if waitErr != nil {
			return wrapDaemonError("wait", name, waitErr)
		}
	}

	info, err := e.client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if err != nil {
		return wrapDaemonError("inspect", name, err)
	}
	if info.Container.State != nil && info.Container.State.ExitCode != 0 {
		return fmt.Errorf("workspace-init container exited with code %d", info.Container.State.ExitCode)
	}
	return nil
}

func (e *docker) pullImageSilently(ctx context.Context, ref string) error {
	rc, err := e.client.ImagePull(ctx, ref, client.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer rc.Close()
	_, _ = io.Copy(io.Discard, rc)
	return nil
}
