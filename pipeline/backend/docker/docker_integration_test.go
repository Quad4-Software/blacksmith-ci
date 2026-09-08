//go:build integration

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
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backend "codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// TestDockerWorkflowLifecycle requires a running Linux Docker daemon.
func TestDockerWorkflowLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	cl, err := client.New(client.FromEnv)
	require.NoError(t, err)
	defer cl.Close()
	info, err := cl.Info(ctx, client.InfoOptions{})
	require.NoError(t, err)
	if info.Info.OSType != "linux" {
		t.Skip("requires Linux containers")
	}
	e := &docker{client: cl, info: info.Info, config: config{workspaceInitImage: "alpine:3.23"}}
	id := uuid.NewString()
	name := "crow_" + id + "_default"
	step := &backend.Step{
		UUID: id, Name: "sdk-smoke", Image: "alpine:3.23", Pull: true,
		WorkspaceBase: "/workspace", WorkingDir: "/workspace/repo",
		Volumes:  []string{name + ":/workspace"},
		Networks: []backend.Conn{{Name: name, Aliases: []string{"sdk-smoke"}}},
		Commands: []string{"test -w /workspace/repo", "echo sdk-stdout", "echo sdk-stderr >&2", "exit 7"},
		DNS:      []string{"9.9.9.9"},
	}
	conf := &backend.Config{
		Volume: name, Network: name, TempVolumes: []string{name + "_temp"},
		Stages: []*backend.Stage{{Steps: []*backend.Step{step}}},
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cleanupCancel()
		assert.NoError(t, e.DestroyWorkflow(cleanupCtx, conf, id))
	}()
	require.NoError(t, e.Ping(ctx))
	require.NoError(t, e.SetupWorkflow(ctx, conf, id))
	require.NoError(t, e.StartStep(ctx, step, id))
	state, err := e.WaitStep(ctx, step, id)
	require.NoError(t, err)
	assert.Equal(t, 7, state.ExitCode)
	assert.True(t, state.Exited)
	assert.False(t, state.OOMKilled)
	logs, err := e.TailStep(ctx, step, id)
	require.NoError(t, err)
	output, err := io.ReadAll(logs)
	require.NoError(t, err)
	require.NoError(t, logs.Close())
	assert.Contains(t, string(output), "\x01sdk-stdout\n")
	assert.Contains(t, string(output), "\x02sdk-stderr\n")
	require.NoError(t, e.ResumeStep(ctx, step, id))
	require.NoError(t, e.DestroyStep(ctx, step, id))
	require.NoError(t, e.DestroyWorkflow(ctx, conf, id))
	_, err = cl.ContainerInspect(ctx, toContainerName(step), client.ContainerInspectOptions{})
	require.Error(t, err)
	_, err = cl.NetworkInspect(ctx, name, client.NetworkInspectOptions{})
	require.Error(t, err)
	_, err = cl.VolumeInspect(ctx, name, client.VolumeInspectOptions{})
	require.Error(t, err)
}
