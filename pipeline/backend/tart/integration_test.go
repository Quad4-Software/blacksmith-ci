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

//go:build tart_integration

// Package-level integration tests that drive a real Tart VM.
//
// They are behind a build tag because they need macOS on Apple silicon with
// tart installed, which the Linux hosts that run the rest of the suite do not
// have. Run them with:
//
//	just test-tart-integration
//
// or directly:
//
//	CROW_BACKEND_TART_IMAGE=ghcr.io/cirruslabs/macos-tahoe-base:latest \
//	  go test -tags tart_integration -timeout 30m ./pipeline/backend/tart/
package tart

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// integrationFlagEnv maps each CROW_BACKEND_TART_* variable to its flag.
//
// The values are passed as explicit arguments rather than left to the flags'
// own environment sources, because the flag set is a package-level slice of
// stateful pointers: a second Command.Run over the same flags does not
// re-apply an environment source, so every test after the first would see an
// unconfigured backend. Production runs one Command per process and never hits
// this, but a test binary runs many.
var integrationFlagEnv = map[string]string{
	"CROW_BACKEND_TART_IMAGE":  "--backend-tart-image",
	"CROW_BACKEND_TART_BINARY": "--backend-tart-binary",
	"CROW_BACKEND_TART_HOME":   "--backend-tart-home",
	"CROW_BACKEND_TART_CPU":    "--backend-tart-cpu",
	"CROW_BACKEND_TART_MEMORY": "--backend-tart-memory",
}

// newIntegrationBackend builds a loaded backend from the CROW_BACKEND_TART_*
// environment, and skips the test when no image is configured so a run on a
// machine without tart is a skip rather than a failure.
func newIntegrationBackend(t *testing.T) (*tart, context.Context) {
	t.Helper()

	if os.Getenv("CROW_BACKEND_TART_IMAGE") == "" {
		t.Skip("set CROW_BACKEND_TART_IMAGE to run the tart integration tests")
	}

	args := []string{"test"}
	for env, flag := range integrationFlagEnv {
		if v := os.Getenv(env); v != "" {
			args = append(args, flag, v)
		}
	}

	e := &tart{}
	var ctx context.Context
	cmd := &cli.Command{
		Flags: Flags,
		Action: func(c context.Context, cmd *cli.Command) error {
			ctx = context.WithValue(c, types.CliCommand, cmd)
			_, err := e.Load(ctx)
			return err
		},
	}
	require.NoError(t, cmd.Run(context.Background(), args))
	return e, ctx
}

// runStep starts a step, drains its log stream and waits for it to finish.
func runStep(t *testing.T, e *tart, ctx context.Context, step *types.Step, taskUUID string) (string, int) {
	t.Helper()

	require.NoError(t, e.StartStep(ctx, step, taskUUID))

	logs, err := e.TailStep(ctx, step, taskUUID)
	require.NoError(t, err)
	out, err := io.ReadAll(logs)
	require.NoError(t, err)

	state, err := e.WaitStep(ctx, step, taskUUID)
	require.NoError(t, err)
	require.True(t, state.Exited)

	// Strip the stream tag byte the agent uses so assertions read normally.
	return strings.ReplaceAll(strings.ReplaceAll(string(out), "\x01", ""), "\x02", ""), state.ExitCode
}

// crowVMsPresent reports whether any VM this backend owns is still registered.
func crowVMsPresent(t *testing.T, e *tart, ctx context.Context) []string {
	t.Helper()

	raw, err := e.command(ctx, listArgs()...).Output()
	require.NoError(t, err)
	entries, err := parseVMList(raw)
	require.NoError(t, err)
	return staleVMs(entries, nil)
}

// TestWorkflowSharesOneVM is the behavior the whole design rests on: steps of a
// workflow share a VM and a workspace, and the VM is gone afterwards.
func TestWorkflowSharesOneVM(t *testing.T) {
	e, ctx := newIntegrationBackend(t)
	const taskUUID = "01integrationsharesonevm00"

	conf := &types.Config{}
	require.NoError(t, e.SetupWorkflow(ctx, conf, taskUUID))
	t.Cleanup(func() {
		assert.NoError(t, e.DestroyWorkflow(ctx, conf, taskUUID))
		assert.Empty(t, crowVMsPresent(t, e, ctx), "DestroyWorkflow must leave no VM behind")
	})

	first, code := runStep(t, e, ctx, &types.Step{
		Name: "writes", UUID: "step-1", Type: types.StepTypeCommands, Image: "/bin/sh",
		Commands: []string{"sw_vers -productVersion", "echo marker > shared-file", "echo done"},
	}, taskUUID)
	assert.Equal(t, 0, code)
	assert.Contains(t, first, "done")

	second, code := runStep(t, e, ctx, &types.Step{
		Name: "reads", UUID: "step-2", Type: types.StepTypeCommands, Image: "/bin/sh",
		Commands: []string{"cat shared-file"},
	}, taskUUID)
	assert.Equal(t, 0, code)
	assert.Contains(t, second, "marker", "the second step must see what the first wrote")
}

// TestStepEnvironmentReachesTheGuest checks that the environment the server
// sends arrives intact, and that the guest paths are real absolute paths rather
// than an unexpanded "$HOME/...".
func TestStepEnvironmentReachesTheGuest(t *testing.T) {
	e, ctx := newIntegrationBackend(t)
	const taskUUID = "01integrationenvironment00"

	conf := &types.Config{}
	require.NoError(t, e.SetupWorkflow(ctx, conf, taskUUID))
	t.Cleanup(func() { assert.NoError(t, e.DestroyWorkflow(ctx, conf, taskUUID)) })

	out, code := runStep(t, e, ctx, &types.Step{
		Name: "env", UUID: "step-1", Type: types.StepTypeCommands, Image: "/bin/sh",
		Environment: map[string]string{"CI_COMMIT_SHA": "deadbeef", "WITH_SPACES": "a b c"},
		Commands: []string{
			`echo "sha=$CI_COMMIT_SHA"`,
			`echo "spaces=$WITH_SPACES"`,
			`echo "workspace=$CI_WORKSPACE"`,
			`test "$CI_WORKSPACE" = "$(pwd)"`,
		},
	}, taskUUID)

	assert.Equal(t, 0, code)
	assert.Contains(t, out, "sha=deadbeef")
	assert.Contains(t, out, "spaces=a b c")
	assert.NotContains(t, out, "$HOME", "guest paths must be resolved, not left for a quoted shell to expand")
}

// TestFailingStepReportsItsExitCode guards the difference between a pipeline
// failure and an agent error. Tart itself returns 125 for a missing binary,
// so a step's own code has to survive unchanged.
func TestFailingStepReportsItsExitCode(t *testing.T) {
	e, ctx := newIntegrationBackend(t)
	const taskUUID = "01integrationexitcode00000"

	conf := &types.Config{}
	require.NoError(t, e.SetupWorkflow(ctx, conf, taskUUID))
	t.Cleanup(func() { assert.NoError(t, e.DestroyWorkflow(ctx, conf, taskUUID)) })

	out, code := runStep(t, e, ctx, &types.Step{
		Name: "fails", UUID: "step-1", Type: types.StepTypeCommands, Image: "/bin/sh",
		Commands: []string{"echo before", "exit 3", "echo unreachable"},
	}, taskUUID)

	assert.Equal(t, 3, code)
	assert.Contains(t, out, "before")
	assert.NotContains(t, out, "unreachable", "set -e must stop the script at the failing command")
}

// TestStderrIsTagged checks the stream tagging the agent relies on to color
// step logs, which only works because tart exec keeps the two streams apart.
func TestStderrIsTagged(t *testing.T) {
	e, ctx := newIntegrationBackend(t)
	const taskUUID = "01integrationstderrtag0000"

	conf := &types.Config{}
	require.NoError(t, e.SetupWorkflow(ctx, conf, taskUUID))
	t.Cleanup(func() { assert.NoError(t, e.DestroyWorkflow(ctx, conf, taskUUID)) })

	step := &types.Step{
		Name: "streams", UUID: "step-1", Type: types.StepTypeCommands, Image: "/bin/sh",
		Commands: []string{"echo to-stdout", "echo to-stderr 1>&2"},
	}
	require.NoError(t, e.StartStep(ctx, step, taskUUID))

	logs, err := e.TailStep(ctx, step, taskUUID)
	require.NoError(t, err)
	raw, err := io.ReadAll(logs)
	require.NoError(t, err)
	_, err = e.WaitStep(ctx, step, taskUUID)
	require.NoError(t, err)

	assert.Contains(t, string(raw), "\x01to-stdout", "stdout lines carry the 0x01 tag")
	assert.Contains(t, string(raw), "\x02to-stderr", "stderr lines carry the 0x02 tag")
}

// TestSetupRejectsAnUnusableImage checks that a setup which cannot get off the
// ground reports why and leaves nothing on disk.
func TestSetupRejectsAnUnusableImage(t *testing.T) {
	e, ctx := newIntegrationBackend(t)
	e.config.Image = "ghcr.io/cirruslabs/definitely-not-a-real-image:latest"

	const taskUUID = "01integrationbadimage00000"
	err := e.SetupWorkflow(ctx, &types.Config{}, taskUUID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "definitely-not-a-real-image")
	assert.Empty(t, crowVMsPresent(t, e, ctx), "a failed setup must not leave a VM behind")
}
