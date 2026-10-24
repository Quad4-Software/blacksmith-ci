// Copyright 2022 Woodpecker Authors
// Copyright 2019 mhmxs.
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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	watch_mocks "codefloe.com/crowci/crow/v6/server/services/watch/mocks"
	"codefloe.com/crowci/crow/v6/server/store"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func mockStorePipeline() store.Store {
	s := new(store_mocks.MockStore)
	s.On("UpdatePipeline", mock.Anything).Return(nil)
	return s
}

func TestUpdateToStatusRunning(t *testing.T) {
	t.Parallel()

	pipeline, _ := UpdateToStatusRunning(mockStorePipeline(), model.Pipeline{}, int64(1))
	assert.Equal(t, model.StatusRunning, pipeline.Status)
	assert.EqualValues(t, 1, pipeline.Started)
}

func TestUpdateToStatusPending(t *testing.T) {
	t.Parallel()

	now := time.Now().Unix()

	pipeline, _ := UpdateToStatusPending(mockStorePipeline(), model.Pipeline{}, "Reviewer")

	assert.Equal(t, model.StatusPending, pipeline.Status)
	assert.Equal(t, "Reviewer", pipeline.Reviewer)
	assert.LessOrEqual(t, now, pipeline.Reviewed)
}

func TestUpdateToStatusDeclined(t *testing.T) {
	t.Parallel()

	now := time.Now().Unix()

	pipeline, _ := UpdateToStatusDeclined(mockStorePipeline(), model.Pipeline{}, "Reviewer")

	assert.Equal(t, model.StatusDeclined, pipeline.Status)
	assert.Equal(t, "Reviewer", pipeline.Reviewer)
	assert.LessOrEqual(t, now, pipeline.Reviewed)
}

func TestUpdateToStatusToDone(t *testing.T) {
	t.Parallel()

	pipeline, _ := UpdateStatusToDone(mockStorePipeline(), model.Pipeline{}, "status", int64(1))

	assert.Equal(t, model.StatusValue("status"), pipeline.Status)
	assert.EqualValues(t, 1, pipeline.Finished)
}

func TestUpdateToStatusError(t *testing.T) {
	t.Parallel()

	now := time.Now().Unix()

	pipeline, _ := UpdateToStatusError(mockStorePipeline(), model.Pipeline{}, errors.New("this is an error"))

	assert.Len(t, pipeline.Errors, 1)
	assert.Equal(t, "[generic] this is an error", pipeline.Errors[0].Error())
	assert.Equal(t, model.StatusError, pipeline.Status)
	assert.Equal(t, pipeline.Started, pipeline.Finished)
	assert.LessOrEqual(t, now, pipeline.Started)
	assert.Equal(t, pipeline.Started, pipeline.Finished)
}

func TestUpdateToStatusKilled(t *testing.T) {
	t.Parallel()

	now := time.Now().Unix()

	pipeline, _ := UpdateToStatusKilled(mockStorePipeline(), model.Pipeline{})

	assert.Equal(t, model.StatusKilled, pipeline.Status)
	assert.LessOrEqual(t, now, pipeline.Finished)
}

func TestNotifyWatchersIfFinished(t *testing.T) {
	// not parallel: swaps the global watch service
	svc := watch_mocks.NewMockService(t)
	orig := server.Config.Services.WatchService
	server.Config.Services.WatchService = svc
	t.Cleanup(func() { server.Config.Services.WatchService = orig })

	repo := &model.Repo{ID: 1, FullName: "o/r"}
	done := &model.Workflow{ID: 1, State: model.StatusFailure}
	pending := &model.Workflow{ID: 2, State: model.StatusPending}

	// still running, and not a genuine outcome: nothing is evaluated
	notifyWatchersIfFinished(t.Context(), repo, &model.Pipeline{ID: 1, Status: model.StatusFailure, Workflows: []*model.Workflow{done, pending}})
	notifyWatchersIfFinished(t.Context(), repo, &model.Pipeline{ID: 2, Status: model.StatusKilled, Workflows: []*model.Workflow{done}})

	called := make(chan *model.Pipeline, 1)
	svc.EXPECT().PipelineFinished(mock.Anything, repo, mock.Anything).RunAndReturn(func(_ context.Context, _ *model.Repo, p *model.Pipeline) error {
		called <- p
		return nil
	}).Once()
	notifyWatchersIfFinished(t.Context(), repo, &model.Pipeline{ID: 3, Status: model.StatusFailure, Workflows: []*model.Workflow{done}})
	select {
	case p := <-called:
		assert.Equal(t, int64(3), p.ID)
		assert.Equal(t, model.StatusFailure, p.Status)
	case <-time.After(2 * time.Second):
		t.Fatal("watch service was not called for a finished pipeline")
	}
}
