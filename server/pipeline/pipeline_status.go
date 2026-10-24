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
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/pipeline/errors"
	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
)

func UpdateToStatusRunning(store store.Store, pipeline model.Pipeline, started int64) (*model.Pipeline, error) {
	pipeline.Status = model.StatusRunning
	pipeline.Started = started
	return &pipeline, store.UpdatePipeline(&pipeline)
}

func UpdateToStatusPending(store store.Store, pipeline model.Pipeline, reviewer string) (*model.Pipeline, error) {
	if reviewer != "" {
		pipeline.Reviewer = reviewer
		pipeline.Reviewed = time.Now().Unix()
	}
	pipeline.Status = model.StatusPending
	return &pipeline, store.UpdatePipeline(&pipeline)
}

func UpdateToStatusDeclined(store store.Store, pipeline model.Pipeline, reviewer string) (*model.Pipeline, error) {
	pipeline.Reviewer = reviewer
	pipeline.Status = model.StatusDeclined
	pipeline.Reviewed = time.Now().Unix()
	return &pipeline, store.UpdatePipeline(&pipeline)
}

func UpdateStatusToDone(store store.Store, pipeline model.Pipeline, status model.StatusValue, stopped int64) (*model.Pipeline, error) {
	pipeline.Status = status
	pipeline.Finished = stopped
	return &pipeline, store.UpdatePipeline(&pipeline)
}

func UpdateToStatusError(store store.Store, pipeline model.Pipeline, err error) (*model.Pipeline, error) {
	pipeline.Errors = errors.GetPipelineErrors(err)
	pipeline.Status = model.StatusError
	pipeline.Started = time.Now().Unix()
	pipeline.Finished = pipeline.Started
	return &pipeline, store.UpdatePipeline(&pipeline)
}

func UpdateToStatusKilled(store store.Store, pipeline model.Pipeline) (*model.Pipeline, error) {
	pipeline.Status = model.StatusKilled
	pipeline.Finished = time.Now().Unix()
	return &pipeline, store.UpdatePipeline(&pipeline)
}

// notifyWatchers evaluates the watches of a finished pipeline outside the
// agent's Done report, for example after a configuration error.
// It runs in the background and is a no-op when no watch service is configured.
func notifyWatchers(ctx context.Context, repo *model.Repo, pipeline *model.Pipeline) {
	svc := server.Config.Services.WatchService
	if svc == nil || repo == nil || pipeline == nil {
		return
	}
	p := *pipeline
	// detach from cancellation so evaluation outlives the triggering request
	bg := context.WithoutCancel(ctx)
	go func() {
		if err := svc.PipelineFinished(bg, repo, &p); err != nil {
			log.Error().Err(err).Msgf("watch evaluation failed for %s#%d", repo.FullName, p.Number)
		}
	}()
}

// notifyWatchersIfFinished evaluates the watches of a pipeline whose status was
// recomputed outside the agent's Done report, such as after an approval gate
// decision or a workflow cancel. Like Done, it only acts once no workflow is
// pending or running and the status is a genuine outcome.
func notifyWatchersIfFinished(ctx context.Context, repo *model.Repo, pipeline *model.Pipeline) {
	if pipeline == nil || !model.IsGenuineOutcome(pipeline.Status) || model.IsThereRunningStage(pipeline.Workflows) {
		return
	}
	notifyWatchers(ctx, repo, pipeline)
}
