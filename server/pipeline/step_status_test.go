// Copyright 2022 Woodpecker Authors
// Copyright 2019 mhmxs
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

package pipeline

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v6/pipeline"
	"codefloe.com/crowci/crow/v6/pipeline/rpc"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func mockStoreStep() store.Store {
	s := new(store_mocks.MockStore)
	s.On("StepUpdate", mock.Anything).Return(nil)
	return s
}

func TestUpdateStepStatusNotExited(t *testing.T) {
	t.Parallel()
	// step in db before update
	step := &model.Step{}

	// advertised step status
	state := rpc.StepState{
		Started: int64(42),
		Exited:  false,
		// Dummy data
		Finished: int64(1),
		ExitCode: pipeline.ExitCodeKilled,
		Error:    "not an error",
	}

	err := UpdateStepStatus(mockStoreStep(), step, state)
	assert.NoError(t, err)
	assert.EqualValues(t, model.StatusRunning, step.State)
	assert.EqualValues(t, 42, step.Started)
	assert.EqualValues(t, 0, step.Finished)
	assert.EqualValues(t, 0, step.ExitCode)
	assert.EqualValues(t, "", step.Error)
}

func TestUpdateStepStatusNotExitedButStopped(t *testing.T) {
	t.Parallel()

	// step in db before update
	step := &model.Step{Started: 42, Finished: 64, State: model.StatusKilled}

	// advertised step status
	state := rpc.StepState{
		Exited: false,
		// Dummy data
		Finished: int64(1),
		ExitCode: pipeline.ExitCodeKilled,
		Error:    "not an error",
	}

	err := UpdateStepStatus(mockStoreStep(), step, state)
	assert.NoError(t, err)
	assert.EqualValues(t, model.StatusKilled, step.State)
	assert.EqualValues(t, 42, step.Started)
	assert.EqualValues(t, 64, step.Finished)
	assert.EqualValues(t, 0, step.ExitCode)
	assert.EqualValues(t, "", step.Error)
}

func TestUpdateStepStatusExitedWith137IsFailure(t *testing.T) {
	t.Parallel()

	// step in db before update: a running step that was never canceled
	step := &model.Step{Started: 42, State: model.StatusRunning}

	// advertised step status: exit 137 (SIGKILL) without a preceding cancel,
	// e.g. an OOM kill or a host/daemon SIGKILL of the container
	state := rpc.StepState{
		Started:  int64(42),
		Exited:   true,
		Finished: int64(34),
		ExitCode: pipeline.ExitCodeKilled,
		Error:    "an error",
	}

	err := UpdateStepStatus(mockStoreStep(), step, state)
	assert.NoError(t, err)
	// 137 is not, by itself, a user cancellation: it must surface as a failure.
	assert.EqualValues(t, model.StatusFailure, step.State)
	assert.EqualValues(t, 42, step.Started)
	assert.EqualValues(t, 34, step.Finished)
	assert.EqualValues(t, pipeline.ExitCodeKilled, step.ExitCode)
	assert.EqualValues(t, "an error", step.Error)
}

func TestUpdateStepStatusExitedPreservesKilled(t *testing.T) {
	t.Parallel()

	// step in db before update: already marked killed by the cancel path
	step := &model.Step{Started: 42, State: model.StatusKilled}

	// advertised step status: the torn-down container's final report (exit 137)
	state := rpc.StepState{
		Started:  int64(42),
		Exited:   true,
		Finished: int64(34),
		ExitCode: pipeline.ExitCodeKilled,
		Error:    "an error",
	}

	err := UpdateStepStatus(mockStoreStep(), step, state)
	assert.NoError(t, err)
	// A genuine cancellation stays killed; the late 137 report must not downgrade it.
	assert.EqualValues(t, model.StatusKilled, step.State)
	assert.EqualValues(t, 34, step.Finished)
	assert.EqualValues(t, pipeline.ExitCodeKilled, step.ExitCode)
}

func TestUpdateStepStatusExitedButNot137(t *testing.T) {
	t.Parallel()

	// step in db before update
	step := &model.Step{Started: 42}

	// advertised step status
	state := rpc.StepState{
		Started:  int64(42),
		Exited:   true,
		Finished: int64(34),
		Error:    "an error",
	}

	err := UpdateStepStatus(mockStoreStep(), step, state)
	assert.NoError(t, err)
	assert.EqualValues(t, model.StatusFailure, step.State)
	assert.EqualValues(t, 42, step.Started)
	assert.EqualValues(t, 34, step.Finished)
	assert.EqualValues(t, 0, step.ExitCode)
	assert.EqualValues(t, "an error", step.Error)
}

func TestUpdateStepStatusExitedWithCode(t *testing.T) {
	t.Parallel()

	// advertised step status
	state := rpc.StepState{
		Started:  int64(42),
		Exited:   true,
		Finished: int64(34),
		ExitCode: 1,
		Error:    "an error",
	}
	step := &model.Step{}
	err := UpdateStepStatus(mockStoreStep(), step, state)
	assert.NoError(t, err)

	assert.Equal(t, model.StatusFailure, step.State)
	assert.Equal(t, 1, step.ExitCode)
}

func TestUpdateStepToStatusStarted(t *testing.T) {
	t.Parallel()

	state := rpc.StepState{Started: int64(42)}
	step, _ := UpdateStepToStatusStarted(mockStoreStep(), model.Step{}, state)

	assert.Equal(t, model.StatusRunning, step.State)
	assert.EqualValues(t, 42, step.Started)
}

func TestUpdateStepToStatusSkipped(t *testing.T) {
	t.Parallel()

	step, _ := UpdateStepToStatusSkipped(mockStoreStep(), model.Step{}, int64(1))

	assert.Equal(t, model.StatusSkipped, step.State)
	assert.EqualValues(t, 0, step.Finished)
}

func TestUpdateStepToStatusSkippedButStarted(t *testing.T) {
	t.Parallel()

	step := &model.Step{
		Started: int64(42),
	}

	step, _ = UpdateStepToStatusSkipped(mockStoreStep(), *step, int64(1))

	assert.Equal(t, model.StatusSuccess, step.State)
	assert.EqualValues(t, 1, step.Finished)
}

func TestUpdateStepStatusToDoneSkipped(t *testing.T) {
	t.Parallel()

	state := rpc.StepState{
		Finished: int64(34),
	}

	step, _ := UpdateStepStatusToDone(mockStoreStep(), model.Step{}, state)

	assert.Equal(t, model.StatusSkipped, step.State)
	assert.EqualValues(t, 34, step.Finished)
	assert.Empty(t, step.Error)
	assert.Equal(t, 0, step.ExitCode)
}

func TestUpdateStepStatusToDoneSuccess(t *testing.T) {
	t.Parallel()

	state := rpc.StepState{
		Started:  int64(42),
		Finished: int64(34),
	}

	step, _ := UpdateStepStatusToDone(mockStoreStep(), model.Step{}, state)

	assert.Equal(t, model.StatusSuccess, step.State)
	assert.EqualValues(t, 34, step.Finished)
	assert.Empty(t, step.Error)
	assert.Equal(t, 0, step.ExitCode)
}

func TestUpdateStepStatusToDoneFailureWithError(t *testing.T) {
	t.Parallel()

	state := rpc.StepState{Error: "an error"}

	step, _ := UpdateStepStatusToDone(mockStoreStep(), model.Step{}, state)

	assert.Equal(t, model.StatusFailure, step.State)
}

func TestUpdateStepStatusToDoneFailureWithExitCode(t *testing.T) {
	t.Parallel()

	state := rpc.StepState{ExitCode: 43}

	step, _ := UpdateStepStatusToDone(mockStoreStep(), model.Step{}, state)

	assert.Equal(t, model.StatusFailure, step.State)
}

func TestUpdateStepToStatusKilledStarted(t *testing.T) {
	t.Parallel()

	now := time.Now().Unix()

	step, _ := UpdateStepToStatusKilled(mockStoreStep(), model.Step{})

	assert.Equal(t, model.StatusKilled, step.State)
	assert.LessOrEqual(t, now, step.Finished)
	assert.Equal(t, step.Finished, step.Started)
	assert.Equal(t, 137, step.ExitCode)
}

func TestUpdateStepToStatusKilledNotStarted(t *testing.T) {
	t.Parallel()

	step, _ := UpdateStepToStatusKilled(mockStoreStep(), model.Step{Started: int64(1)})

	assert.EqualValues(t, 1, step.Started)
}
