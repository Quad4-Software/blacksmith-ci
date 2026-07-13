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

package agent

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	backend "codefloe.com/crowci/crow/v6/pipeline/backend/types"
	"codefloe.com/crowci/crow/v6/pipeline/rpc"
	rpcmocks "codefloe.com/crowci/crow/v6/pipeline/rpc/mocks"
)

var errResumeProbeAbort = errors.New("resume probe abort")

// resumeProbeBackend is a minimal backend implementing both types.Backend and
// types.WorkflowResumer. ResumeStep is the first backend call RunAdopted makes,
// and it happens only after the build has been registered in the active-task
// counter. The probe snapshots that counter and then aborts the run so the
// assertion is deterministic and no real step work is attempted.
type resumeProbeBackend struct {
	counter  *State
	captured map[string]struct{}
}

func (b *resumeProbeBackend) Name() string                     { return "resume-probe" }
func (b *resumeProbeBackend) IsAvailable(context.Context) bool { return true }
func (b *resumeProbeBackend) Flags() []cli.Flag                { return nil }

func (b *resumeProbeBackend) Load(context.Context) (*backend.BackendInfo, error) {
	return &backend.BackendInfo{}, nil
}

func (b *resumeProbeBackend) SetupWorkflow(context.Context, *backend.Config, string) error {
	return nil
}
func (b *resumeProbeBackend) StartStep(context.Context, *backend.Step, string) error { return nil }

func (b *resumeProbeBackend) WaitStep(context.Context, *backend.Step, string) (*backend.State, error) {
	return &backend.State{Exited: true}, nil
}

func (b *resumeProbeBackend) TailStep(context.Context, *backend.Step, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (b *resumeProbeBackend) DestroyStep(context.Context, *backend.Step, string) error { return nil }

func (b *resumeProbeBackend) DestroyWorkflow(context.Context, *backend.Config, string) error {
	return nil
}

// ResumeStep records the active-task set the reaper would see, then aborts.
func (b *resumeProbeBackend) ResumeStep(context.Context, *backend.Step, string) error {
	b.captured = b.counter.ActiveTaskUUIDs()
	return errResumeProbeAbort
}

// TestRunAdoptedTracksTaskUUIDForReaper guards the fix for adopted workflows
// being reaped mid-run: RunAdopted must register the build under its taskUUID
// (the value stamped on its containers/volumes/networks as LabelTaskUUID), not
// under workflow.ID. For adopted workflows the two identifiers differ, and the
// stale-resource reaper matches its protected set (State.ActiveTaskUUIDs)
// against the resource labels. Keying by workflow.ID would leave the resources
// unmatched, so the reaper could reclaim a live build's workspace volume during
// the between-steps window where it has no running container to protect it.
func TestRunAdoptedTracksTaskUUIDForReaper(t *testing.T) {
	const (
		workflowID = "12345"
		taskUUID   = "task-abcdef0123" // distinct from workflowID, as for adopted workflows
	)

	counter := &State{Metadata: map[string]Info{}}
	probe := &resumeProbeBackend{counter: counter}
	var engine backend.Backend = probe

	client := rpcmocks.NewMockPeer(t)
	// RunAdopted spawns a goroutine that blocks on Wait until the workflow
	// context is canceled; keep it blocked until the run tears down.
	client.EXPECT().Wait(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, _ string) error {
		<-ctx.Done()
		return ctx.Err()
	}).Maybe()
	client.EXPECT().Extend(mock.Anything, mock.Anything).Return(nil).Maybe()
	client.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	client.EXPECT().Done(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	runner := NewRunner(client, rpc.Filter{}, "test-host", counter, &engine)

	workflow := &rpc.Workflow{
		ID:      workflowID,
		Timeout: 1,
		Config: &backend.Config{
			Stages: []*backend.Stage{
				{Steps: []*backend.Step{{Name: "build", UUID: "step-1", OnSuccess: true}}},
			},
		},
	}

	ctx := context.Background()
	require.NoError(t, runner.RunAdopted(ctx, ctx, workflow, taskUUID))

	require.NotNil(t, probe.captured, "ResumeStep should have been reached")
	assert.Contains(t, probe.captured, taskUUID,
		"adopted build must be tracked under its taskUUID so the reaper protects its resources")
	assert.NotContains(t, probe.captured, workflowID,
		"tracking by workflow.ID would not match the taskUUID stamped on the resources")
}
