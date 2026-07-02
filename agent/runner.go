// Copyright 2022 Woodpecker Authors
// Copyright 2018 Drone.IO Inc.
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
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/metadata"

	"codefloe.com/crowci/crow/v5/pipeline"
	backend "codefloe.com/crowci/crow/v5/pipeline/backend/types"
	"codefloe.com/crowci/crow/v5/pipeline/rpc"
	"codefloe.com/crowci/crow/v5/shared/constant"
	"codefloe.com/crowci/crow/v5/shared/utils"
)

type Runner struct {
	client   rpc.Peer
	filter   rpc.Filter
	hostname string
	counter  *State
	backend  *backend.Backend
}

func NewRunner(workEngine rpc.Peer, f rpc.Filter, h string, state *State, backend *backend.Backend) Runner {
	return Runner{
		client:   workEngine,
		filter:   f,
		hostname: h,
		counter:  state,
		backend:  backend,
	}
}

func (r *Runner) Run(runnerCtx, shutdownCtx context.Context) error { //nolint:contextcheck
	log.Trace().Msg("request next execution")

	meta, _ := metadata.FromOutgoingContext(runnerCtx)
	ctxMeta := metadata.NewOutgoingContext(context.Background(), meta)

	// get the next workflow from the queue
	workflow, err := r.client.Next(runnerCtx, r.filter)
	if err != nil {
		return err
	}
	if workflow == nil {
		return nil
	}

	timeout := time.Hour
	if minutes := workflow.Timeout; minutes != 0 {
		timeout = time.Duration(minutes) * time.Minute
	}

	repoName := extractRepositoryName(workflow.Config)       // hack
	pipelineNumber := extractPipelineNumber(workflow.Config) // hack

	r.counter.Add(
		workflow.ID,
		timeout,
		repoName,
		pipelineNumber,
	)
	defer r.counter.Done(workflow.ID)

	logger := log.With().
		Str("repo", repoName).
		Str("pipeline", pipelineNumber).
		Str("workflow_id", workflow.ID).
		Logger()

	logger.Debug().Msg("received execution")

	workflowCtx, cancel := context.WithTimeout(ctxMeta, timeout)
	defer cancel()

	// Add sigterm support for internal context.
	// Required when the pipeline is terminated by external signals
	// like kubernetes.
	workflowCtx = utils.WithContextSigtermCallback(workflowCtx, func() {
		logger.Error().Msg("Received sigterm termination signal")
	})

	var canceled atomic.Bool
	go func() {
		logger.Debug().Msg("listen for cancel signal")

		if err := r.client.Wait(workflowCtx, workflow.ID); err != nil {
			canceled.Store(true)
			logger.Warn().Err(err).Msg("cancel signal received")
			cancel()
		} else {
			logger.Debug().Msg("done listening for cancel signal")
		}
	}()

	go func() {
		for {
			select {
			case <-workflowCtx.Done():
				logger.Debug().Msg("pipeline done")
				return

			case <-time.After(constant.TaskTimeout / 3):
				logger.Debug().Msg("pipeline lease renewed")
				if err := r.client.Extend(workflowCtx, workflow.ID); err != nil {
					log.Error().Err(err).Msg("extending pipeline deadline failed")
				}
			}
		}
	}()

	state := rpc.WorkflowState{}
	state.Started = time.Now().Unix()

	err = r.client.Init(runnerCtx, workflow.ID, state)
	if err != nil {
		logger.Error().Err(err).Msg("workflow initialization failed")
		// TODO: should we return here?
	}

	var uploads sync.WaitGroup
	//nolint:contextcheck
	err = pipeline.New(workflow.Config,
		pipeline.WithContext(workflowCtx),
		pipeline.WithTaskUUID(fmt.Sprint(workflow.ID)),
		pipeline.WithLogger(r.createLogger(logger, &uploads, workflow)),
		pipeline.WithTracer(r.createTracer(ctxMeta, &uploads, logger, workflow)),
		pipeline.WithBackend(*r.backend),
		pipeline.WithDescription(map[string]string{
			"workflow_id":     workflow.ID,
			"repo":            repoName,
			"pipeline_number": pipelineNumber,
		}),
	).Run(runnerCtx)

	state.Finished = time.Now().Unix()

	if errors.Is(err, pipeline.ErrCancel) {
		canceled.Store(true)
	} else if canceled.Load() {
		err = errors.Join(err, pipeline.ErrCancel)
	}

	if err != nil {
		state.Error = err.Error()
	}

	logger.Debug().
		Str("error", state.Error).
		Bool("canceled", canceled.Load()).
		Msg("workflow finished")

	// If workflows are being preserved for adoption, don't report Done to server.
	// The workflow isn't actually done - it will be picked up by the restarted agent.
	if pipeline.ShouldPreserveWorkflows() {
		logger.Info().Msg("workflow preserved for adoption, skipping Done() call")
		return nil
	}

	logger.Debug().Msg("uploading logs and traces / states ...")
	uploads.Wait()
	logger.Debug().Msg("uploaded logs and traces / states")

	logger.Debug().
		Str("error", state.Error).
		Msg("updating workflow status")

	doneCtx := runnerCtx
	if doneCtx.Err() != nil {
		doneCtx = shutdownCtx
	}
	if err := r.client.Done(doneCtx, workflow.ID, state); err != nil {
		logger.Error().Err(err).Msg("updating workflow status failed")
	} else {
		logger.Debug().Msg("updating workflow status complete")
	}

	return nil
}

func extractRepositoryName(config *backend.Config) string {
	if len(config.Stages) == 0 || len(config.Stages[0].Steps) == 0 {
		return ""
	}
	return config.Stages[0].Steps[0].Environment["CI_REPO"]
}

func extractPipelineNumber(config *backend.Config) string {
	if len(config.Stages) == 0 || len(config.Stages[0].Steps) == 0 {
		return ""
	}
	return config.Stages[0].Steps[0].Environment["CI_PIPELINE_NUMBER"]
}

// RunAdopted resumes monitoring of an adopted workflow (from a previous agent instance).
// Unlike Run, this skips fetching a new workflow and uses RunAdopted which doesn't recreate containers.
func (r *Runner) RunAdopted(runnerCtx, shutdownCtx context.Context, workflow *rpc.Workflow, taskUUID string) error { //nolint:contextcheck
	timeout := time.Hour
	if minutes := workflow.Timeout; minutes != 0 {
		timeout = time.Duration(minutes) * time.Minute
	}

	repoName := extractRepositoryName(workflow.Config)
	pipelineNumber := extractPipelineNumber(workflow.Config)

	r.counter.Add(
		workflow.ID,
		timeout,
		repoName,
		pipelineNumber,
	)
	defer r.counter.Done(workflow.ID)

	logger := log.With().
		Str("repo", repoName).
		Str("pipeline", pipelineNumber).
		Str("workflow_id", workflow.ID).
		Str("task_uuid", taskUUID).
		Bool("adopted", true).
		Logger()

	logger.Info().Msg("resuming adopted workflow")

	meta, _ := metadata.FromOutgoingContext(runnerCtx)
	ctxMeta := metadata.NewOutgoingContext(context.Background(), meta)

	workflowCtx, cancel := context.WithTimeout(ctxMeta, timeout)
	defer cancel()

	// Add sigterm support for internal context.
	workflowCtx = utils.WithContextSigtermCallback(workflowCtx, func() {
		logger.Error().Msg("Received sigterm termination signal")
	})

	var canceled atomic.Bool
	go func() {
		logger.Debug().Msg("listen for cancel signal")

		if err := r.client.Wait(workflowCtx, workflow.ID); err != nil {
			canceled.Store(true)
			logger.Warn().Err(err).Msg("cancel signal received")
			cancel()
		} else {
			logger.Debug().Msg("done listening for cancel signal")
		}
	}()

	go func() {
		for {
			select {
			case <-workflowCtx.Done():
				logger.Debug().Msg("pipeline done")
				return

			case <-time.After(constant.TaskTimeout / 3):
				logger.Debug().Msg("pipeline lease renewed")
				if err := r.client.Extend(workflowCtx, workflow.ID); err != nil {
					log.Error().Err(err).Msg("extending pipeline deadline failed")
				}
			}
		}
	}()

	state := rpc.WorkflowState{}
	state.Started = time.Now().Unix()

	// Don't call Init for adopted workflows - they were already initialized
	// The server already knows about this workflow

	var uploads sync.WaitGroup
	//nolint:contextcheck
	err := pipeline.New(workflow.Config,
		pipeline.WithContext(workflowCtx),
		pipeline.WithTaskUUID(taskUUID),
		pipeline.WithLogger(r.createLogger(logger, &uploads, workflow)),
		pipeline.WithTracer(r.createTracer(ctxMeta, &uploads, logger, workflow)),
		pipeline.WithBackend(*r.backend),
		pipeline.WithDescription(map[string]string{
			"workflow_id":     workflow.ID,
			"repo":            repoName,
			"pipeline_number": pipelineNumber,
			"adopted":         "true",
		}),
	).RunAdopted(runnerCtx)

	state.Finished = time.Now().Unix()

	if errors.Is(err, pipeline.ErrCancel) {
		canceled.Store(true)
	} else if canceled.Load() {
		err = errors.Join(err, pipeline.ErrCancel)
	}

	if err != nil {
		state.Error = err.Error()
	}

	logger.Debug().
		Str("error", state.Error).
		Bool("canceled", canceled.Load()).
		Msg("adopted workflow finished")

	logger.Debug().Msg("uploading logs and traces / states ...")
	uploads.Wait()
	logger.Debug().Msg("uploaded logs and traces / states")

	logger.Debug().
		Str("error", state.Error).
		Msg("updating workflow status")

	doneCtx := runnerCtx
	if doneCtx.Err() != nil {
		doneCtx = shutdownCtx
	}
	if err := r.client.Done(doneCtx, workflow.ID, state); err != nil {
		logger.Error().Err(err).Msg("updating workflow status failed")
	} else {
		logger.Debug().Msg("updating workflow status complete")
	}

	return nil
}
