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

package tart

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

func TestName(t *testing.T) {
	assert.Equal(t, "tart", New().Name())
}

func TestIsAvailableOnlyOnDarwin(t *testing.T) {
	// tart is a macOS-only tool, so the backend must never claim to be
	// available anywhere else, whatever happens to be on PATH.
	if runtime.GOOS != "darwin" {
		assert.False(t, New().IsAvailable(context.Background()))
	}
}

func TestSetupWorkflowRejectsTempVolumes(t *testing.T) {
	e := &tart{config: config{Image: "img", Binary: "tart"}}
	err := e.SetupWorkflow(context.Background(), &types.Config{
		TempVolumes: []string{"crow_tmp_dind"},
	}, "task-uuid")
	assert.ErrorContains(t, err, "does not support `temp_volumes`")
}

func TestStateRoundTrip(t *testing.T) {
	e := &tart{}
	_, err := e.getState("missing")
	assert.ErrorIs(t, err, ErrWorkflowStateNotFound)

	want := newWorkflowState("crow-x", testPaths)
	e.saveState("task", want)
	got, err := e.getState("task")
	require.NoError(t, err)
	assert.Equal(t, want, got)

	e.deleteState("task")
	_, err = e.getState("task")
	assert.ErrorIs(t, err, ErrWorkflowStateNotFound)
}

func TestStartStepRejectsUnknownType(t *testing.T) {
	e := &tart{}
	e.saveState("task", newWorkflowState("crow-x", testPaths))
	err := e.StartStep(context.Background(), &types.Step{Type: types.StepType("nonsense")}, "task")
	assert.ErrorIs(t, err, ErrUnsupportedStepType)
}

func TestRingBufferKeepsTheTail(t *testing.T) {
	// The buffer exists so a VM that fails to boot can say why, not to carry
	// a log stream, so it must stay bounded however much tart writes.
	b := &ringBuffer{}
	_, err := b.Write([]byte("first "))
	require.NoError(t, err)
	_, err = b.Write([]byte("second"))
	require.NoError(t, err)
	assert.Equal(t, "first second", b.String())

	_, err = b.Write(make([]byte, runLogLimit+100))
	require.NoError(t, err)
	assert.Len(t, b.String(), runLogLimit)
}

// The agent only runs stale-resource cleanup for backends that advertise it,
// so the assertion that matters is that this one does.
func TestImplementsResourceCleaner(t *testing.T) {
	_, ok := New().(types.ResourceCleaner)
	assert.True(t, ok)
}

// A VM cannot outlive the agent that started it, so there is nothing to adopt
// and the discovery interfaces must stay unimplemented.
func TestDoesNotImplementWorkflowDiscovery(t *testing.T) {
	_, isDiscoverer := New().(types.WorkflowDiscoverer)
	assert.False(t, isDiscoverer)
	_, isResumer := New().(types.WorkflowResumer)
	assert.False(t, isResumer)
}

// A workflow whose setup failed stored no state, and SetupWorkflow already
// cleaned up after itself. Reporting an error here would bury the real cause
// under a second, meaningless one.
func TestDestroyWorkflowWithoutStateIsNotAnError(t *testing.T) {
	e := &tart{}
	assert.NoError(t, e.DestroyWorkflow(context.Background(), &types.Config{}, "never-set-up"))
}
