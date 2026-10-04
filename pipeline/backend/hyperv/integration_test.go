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

//go:build hyperv_integration

// Package-level integration tests that drive a real Hyper-V VM.
//
// They are behind a build tag because they need a Windows host with the
// Hyper-V role and a golden VHDX, which the Linux hosts that run the rest of
// the suite do not have. Run them with:
//
//	just test-hyperv-integration
//
// or directly:
//
//	CROW_BACKEND_HYPERV_BASE_VHDX=C:\golden\ws2022.vhdx \
//	CROW_BACKEND_HYPERV_GUEST_USERNAME=crow \
//	CROW_BACKEND_HYPERV_GUEST_PASSWORD=... \
//	  go test -tags hyperv_integration -timeout 60m ./pipeline/backend/hyperv/
//
// These are the assertions the unit tests cannot make. Everything the backend
// does against Hyper-V and PowerShell Direct is unverified until this suite
// runs somewhere real: see the notes on each test for what specifically is
// being pinned down.
package hyperv

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

// integrationFlagEnv maps each CROW_BACKEND_HYPERV_* variable to its flag.
//
// The values are passed as explicit arguments rather than left to the flags'
// own environment sources, because the flag set is a package-level slice of
// stateful pointers: a second Command.Run over the same flags does not
// re-apply an environment source, so every test after the first would see an
// unconfigured backend. Production runs one Command per process and never hits
// this, but a test binary runs many.
var integrationFlagEnv = map[string]string{
	"CROW_BACKEND_HYPERV_BASE_VHDX":       "--backend-hyperv-base-vhdx",
	"CROW_BACKEND_HYPERV_VM_PATH":         "--backend-hyperv-vm-path",
	"CROW_BACKEND_HYPERV_GUEST_USERNAME":  "--backend-hyperv-guest-username",
	"CROW_BACKEND_HYPERV_GUEST_PASSWORD":  "--backend-hyperv-guest-password",
	"CROW_BACKEND_HYPERV_GUEST_BASE":      "--backend-hyperv-guest-base",
	"CROW_BACKEND_HYPERV_SWITCH":          "--backend-hyperv-switch",
	"CROW_BACKEND_HYPERV_CPU":             "--backend-hyperv-cpu",
	"CROW_BACKEND_HYPERV_MEMORY":          "--backend-hyperv-memory",
	"CROW_BACKEND_HYPERV_MAX_VMS":         "--backend-hyperv-max-vms",
	"CROW_BACKEND_HYPERV_STARTUP_TIMEOUT": "--backend-hyperv-startup-timeout",
}

// newIntegrationBackend builds a loaded backend from the CROW_BACKEND_HYPERV_*
// environment, and skips the test when no golden disk is configured so a run
// on a machine without Hyper-V is a skip rather than a failure.
func newIntegrationBackend(t *testing.T) (*hyperv, context.Context) {
	t.Helper()

	if os.Getenv("CROW_BACKEND_HYPERV_BASE_VHDX") == "" {
		t.Skip("set CROW_BACKEND_HYPERV_BASE_VHDX to run the hyperv integration tests")
	}

	args := []string{"test"}
	for env, flag := range integrationFlagEnv {
		if v := os.Getenv(env); v != "" {
			args = append(args, flag, v)
		}
	}

	e := &hyperv{}
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
func runStep(t *testing.T, e *hyperv, ctx context.Context, step *types.Step, taskUUID string) (string, int) {
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
func crowVMsPresent(t *testing.T, e *hyperv, ctx context.Context) []string {
	t.Helper()

	raw, err := e.run(ctx, listVMsScript())
	require.NoError(t, err, string(raw))
	return staleVMs(parseVMList(raw), nil)
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
		Name: "writes", UUID: "step-1", Type: types.StepTypeCommands,
		Commands: []string{
			"[System.Environment]::OSVersion.Version.ToString()",
			"Set-Content -LiteralPath shared-file -Value marker",
			"Write-Output done",
		},
	}, taskUUID)
	assert.Equal(t, 0, code)
	assert.Contains(t, first, "done")

	second, code := runStep(t, e, ctx, &types.Step{
		Name: "reads", UUID: "step-2", Type: types.StepTypeCommands,
		Commands: []string{"Get-Content -LiteralPath shared-file"},
	}, taskUUID)
	assert.Equal(t, 0, code)
	assert.Contains(t, second, "marker", "the second step must see the first step's workspace")
}

// TestStepOutputStreamsWhileTheStepRuns is the single biggest unknown in this
// backend: PowerShell remoting may buffer a native command's output until it
// finishes, which would make live logs useless even though the step's result
// is still correct.
//
// The step writes a marker, sleeps, then exits. Reading the marker off the log
// stream before the step has exited proves the output is not being held back
// until completion.
func TestStepOutputStreamsWhileTheStepRuns(t *testing.T) {
	e, ctx := newIntegrationBackend(t)
	const taskUUID = "01integrationstreaming0000"

	conf := &types.Config{}
	require.NoError(t, e.SetupWorkflow(ctx, conf, taskUUID))
	t.Cleanup(func() { assert.NoError(t, e.DestroyWorkflow(ctx, conf, taskUUID)) })

	step := &types.Step{
		Name: "streams", UUID: "step-stream", Type: types.StepTypeCommands,
		Commands: []string{"Write-Output early-marker", "Start-Sleep -Seconds 20", "Write-Output late-marker"},
	}
	require.NoError(t, e.StartStep(ctx, step, taskUUID))

	logs, err := e.TailStep(ctx, step, taskUUID)
	require.NoError(t, err)

	// Read only as far as the first marker. If the transport buffers, this
	// blocks until the step finishes and the test times out rather than
	// silently passing.
	got := make([]byte, 0, 64)
	buf := make([]byte, 32)
	for !strings.Contains(string(got), "early-marker") {
		n, readErr := logs.Read(buf)
		got = append(got, buf[:n]...)
		require.NoError(t, readErr, "log stream ended before the first marker: %q", got)
	}

	state, err := e.WaitStep(ctx, step, taskUUID)
	require.NoError(t, err)
	assert.Equal(t, 0, state.ExitCode)
}

// TestStepEnvironmentReachesTheGuest checks that a step's environment arrives
// intact, including a value with the quote that PowerShell's own string
// literals would otherwise choke on.
func TestStepEnvironmentReachesTheGuest(t *testing.T) {
	e, ctx := newIntegrationBackend(t)
	const taskUUID = "01integrationenvironment00"

	conf := &types.Config{}
	require.NoError(t, e.SetupWorkflow(ctx, conf, taskUUID))
	t.Cleanup(func() { assert.NoError(t, e.DestroyWorkflow(ctx, conf, taskUUID)) })

	out, code := runStep(t, e, ctx, &types.Step{
		Name: "env", UUID: "step-env", Type: types.StepTypeCommands,
		Environment: map[string]string{"CROW_TEST_VALUE": "it's fine"},
		Commands:    []string{"Write-Output $env:CROW_TEST_VALUE"},
	}, taskUUID)

	assert.Equal(t, 0, code)
	assert.Contains(t, out, "it's fine")
}

// TestWorkspaceIsTheWorkingDirectory pins the guest layout down: a step starts
// in the workspace, not in the guest account's profile directory.
func TestWorkspaceIsTheWorkingDirectory(t *testing.T) {
	e, ctx := newIntegrationBackend(t)
	const taskUUID = "01integrationworkingdir000"

	conf := &types.Config{}
	require.NoError(t, e.SetupWorkflow(ctx, conf, taskUUID))
	t.Cleanup(func() { assert.NoError(t, e.DestroyWorkflow(ctx, conf, taskUUID)) })

	out, code := runStep(t, e, ctx, &types.Step{
		Name: "pwd", UUID: "step-pwd", Type: types.StepTypeCommands,
		Commands: []string{"(Get-Location).Path"},
	}, taskUUID)

	assert.Equal(t, 0, code)
	assert.Contains(t, out, newGuestPaths(e.config.GuestBase).Workspace)
}

// TestFailingStepReportsItsExitCode is what the three-round-trip step protocol
// exists for: a native command's exit code has to survive PowerShell Direct
// and become the host process's own.
func TestFailingStepReportsItsExitCode(t *testing.T) {
	e, ctx := newIntegrationBackend(t)
	const taskUUID = "01integrationexitcode00000"

	conf := &types.Config{}
	require.NoError(t, e.SetupWorkflow(ctx, conf, taskUUID))
	t.Cleanup(func() { assert.NoError(t, e.DestroyWorkflow(ctx, conf, taskUUID)) })

	_, code := runStep(t, e, ctx, &types.Step{
		Name: "fails", UUID: "step-fail", Type: types.StepTypeCommands,
		Commands: []string{"Write-Output before", "cmd.exe /c exit 42", "Write-Output unreachable"},
	}, taskUUID)

	assert.Equal(t, 42, code)
}

// TestLaterCommandDoesNotMaskAnEarlierFailure covers the reason buildScript
// resets $LASTEXITCODE and checks after every command: PowerShell has no
// `set -e` for native commands, so without that a failing step would report
// success as long as its last line happened to be a cmdlet.
func TestLaterCommandDoesNotMaskAnEarlierFailure(t *testing.T) {
	e, ctx := newIntegrationBackend(t)
	const taskUUID = "01integrationmaskfailure00"

	conf := &types.Config{}
	require.NoError(t, e.SetupWorkflow(ctx, conf, taskUUID))
	t.Cleanup(func() { assert.NoError(t, e.DestroyWorkflow(ctx, conf, taskUUID)) })

	out, code := runStep(t, e, ctx, &types.Step{
		Name: "masks", UUID: "step-mask", Type: types.StepTypeCommands,
		Commands: []string{"cmd.exe /c exit 7", "Write-Output should-not-run"},
	}, taskUUID)

	assert.Equal(t, 7, code)
	assert.NotContains(t, out, "should-not-run")
}

// TestCommandsKeepPowerShellSemantics pins down what a step's commands share
// and how they fail: a variable set by one command is visible to the next, and
// a failing cmdlet stops the step before the command after it runs.
func TestCommandsKeepPowerShellSemantics(t *testing.T) {
	e, ctx := newIntegrationBackend(t)
	const taskUUID = "01integrationsemantics0000"

	conf := &types.Config{}
	require.NoError(t, e.SetupWorkflow(ctx, conf, taskUUID))
	t.Cleanup(func() { assert.NoError(t, e.DestroyWorkflow(ctx, conf, taskUUID)) })

	out, code := runStep(t, e, ctx, &types.Step{
		Name: "vars", UUID: "step-vars", Type: types.StepTypeCommands,
		Commands: []string{"$crowTestValue = 'kept-across-commands'", "Write-Output $crowTestValue"},
	}, taskUUID)
	assert.Equal(t, 0, code, out)
	assert.Contains(t, out, "kept-across-commands")

	out, code = runStep(t, e, ctx, &types.Step{
		Name: "cmdlet", UUID: "step-cmdlet", Type: types.StepTypeCommands,
		Commands: []string{"Get-Content -LiteralPath does-not-exist", "Write-Output should-not-run"},
	}, taskUUID)
	assert.Equal(t, 1, code, out)
	assert.NotContains(t, out, "should-not-run")
}

// TestGuestStderrReachesTheLog documents the merge rather than asserting a
// separation the transport cannot provide: PowerShell Direct returns one
// channel, so a step's stderr has to arrive in the log even though it cannot
// be tagged apart from stdout.
func TestGuestStderrReachesTheLog(t *testing.T) {
	e, ctx := newIntegrationBackend(t)
	const taskUUID = "01integrationstderr0000000"

	conf := &types.Config{}
	require.NoError(t, e.SetupWorkflow(ctx, conf, taskUUID))
	t.Cleanup(func() { assert.NoError(t, e.DestroyWorkflow(ctx, conf, taskUUID)) })

	out, code := runStep(t, e, ctx, &types.Step{
		Name: "stderr", UUID: "step-stderr", Type: types.StepTypeCommands,
		// The redirection is inside cmd's argument: PowerShell 5.1 parses a
		// bare 1>&2 itself and rejects it as reserved.
		Commands: []string{`cmd.exe /c "echo to-stderr 1>&2"`},
	}, taskUUID)

	assert.Equal(t, 0, code, out)
	assert.Contains(t, out, "to-stderr")
}

// TestSetupRejectsAnUnusableGoldenDisk checks that a bad base disk fails the
// workflow instead of leaving a half-built VM and a burned license slot.
func TestSetupRejectsAnUnusableGoldenDisk(t *testing.T) {
	e, ctx := newIntegrationBackend(t)
	e.config.BaseVHDX = `C:\definitely\not\a\real\disk.vhdx`

	const taskUUID = "01integrationbaddisk000000"
	err := e.SetupWorkflow(ctx, &types.Config{}, taskUUID)

	require.Error(t, err)
	assert.Empty(t, crowVMsPresent(t, e, ctx), "a failed setup must not leave a VM behind")
	assert.Zero(t, e.leased, "a failed setup must give its slot back")
}

// TestVMLimitIsEnforced is the licensing bound, checked against real VMs
// rather than the counter alone.
func TestVMLimitIsEnforced(t *testing.T) {
	e, ctx := newIntegrationBackend(t)
	e.config.MaxVMs = 1

	const firstUUID = "01integrationlimitfirst000"
	conf := &types.Config{}
	require.NoError(t, e.SetupWorkflow(ctx, conf, firstUUID))
	t.Cleanup(func() { assert.NoError(t, e.DestroyWorkflow(ctx, conf, firstUUID)) })

	err := e.SetupWorkflow(ctx, conf, "01integrationlimitsecond00")
	require.ErrorIs(t, err, ErrVMLimitReached)
}
