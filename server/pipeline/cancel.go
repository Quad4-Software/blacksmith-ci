// Copyright 2022 Woodpecker Authors
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
	"fmt"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/forge"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/queue"
	"codefloe.com/crowci/crow/v5/server/store"
)

// Cancel the pipeline and returns the status.
func Cancel(ctx context.Context, _forge forge.Forge, store store.Store, repo *model.Repo, user *model.User, pipeline *model.Pipeline) error {
	if pipeline.Status != model.StatusRunning && pipeline.Status != model.StatusPending && pipeline.Status != model.StatusBlocked {
		return &ErrBadRequest{Msg: "Cannot cancel a non-running or non-pending or non-blocked pipeline"}
	}

	workflows, err := store.WorkflowGetTree(pipeline)
	if err != nil {
		return &ErrNotFound{Msg: err.Error()}
	}

	// First cancel/evict steps in the queue in one go
	var (
		stepsToCancel []string
		stepsToEvict  []string
	)
	for _, workflow := range workflows {
		if workflow.State == model.StatusRunning {
			stepsToCancel = append(stepsToCancel, fmt.Sprint(workflow.ID))
		}
		if workflow.State == model.StatusPending {
			stepsToEvict = append(stepsToEvict, fmt.Sprint(workflow.ID))
		}
	}

	if len(stepsToEvict) != 0 {
		if err := server.Config.Services.Queue.EvictAtOnce(ctx, stepsToEvict); err != nil {
			log.Error().Err(err).Msgf("queue: evict_at_once: %v", stepsToEvict)
		}
		if err := server.Config.Services.Queue.ErrorAtOnce(ctx, stepsToEvict, queue.ErrCancel); err != nil {
			log.Error().Err(err).Msgf("queue: evict_at_once: %v", stepsToEvict)
		}
	}
	if len(stepsToCancel) != 0 {
		if err := server.Config.Services.Queue.ErrorAtOnce(ctx, stepsToCancel, queue.ErrCancel); err != nil {
			log.Error().Err(err).Msgf("queue: evict_at_once: %v", stepsToCancel)
		}
	}

	// Update the DB status for each workflow and its steps. Pending work is
	// marked skipped; running work is marked killed immediately rather than
	// waiting for the agent to ack the cancel signal. Marking running workflows
	// killed here makes a canceled/superseded pipeline deterministically
	// resolve to StatusKilled: otherwise the agent's Done report (which carries
	// the cancel error) can race the step's killed-exit update and flip the
	// workflow to StatusFailure, wrongly triggering a failure notification.
	for _, workflow := range workflows {
		switch workflow.State {
		case model.StatusPending:
			if _, err = UpdateWorkflowToStatusSkipped(store, *workflow); err != nil {
				log.Error().Err(err).Msgf("cannot update workflow with id %d state", workflow.ID)
			}
		case model.StatusRunning:
			if _, err = UpdateWorkflowToStatusKilled(store, *workflow); err != nil {
				log.Error().Err(err).Msgf("cannot update workflow with id %d state to killed", workflow.ID)
			}
		}
		for _, step := range workflow.Children {
			switch step.State {
			case model.StatusPending:
				if _, err = UpdateStepToStatusSkipped(store, *step, 0); err != nil {
					log.Error().Err(err).Msgf("cannot update step with id %d state", step.ID)
				}
			case model.StatusRunning:
				if _, err = UpdateStepToStatusKilled(store, *step); err != nil {
					log.Error().Err(err).Msgf("cannot update step with id %d state to killed", step.ID)
				}
			}
		}
	}

	killedPipeline, err := UpdateToStatusKilled(store, *pipeline)
	if err != nil {
		log.Error().Err(err).Msgf("UpdateToStatusKilled: %v", pipeline)
		return err
	}

	updatePipelineStatus(ctx, _forge, killedPipeline, repo, user)

	if killedPipeline.Workflows, err = store.WorkflowGetTree(killedPipeline); err != nil {
		return err
	}
	publishToTopic(killedPipeline, repo)

	return nil
}

func cancelPreviousPipelines(
	ctx context.Context,
	_forge forge.Forge,
	_store store.Store,
	pipeline *model.Pipeline,
	repo *model.Repo,
	user *model.User,
) error {
	// check this event should cancel previous pipelines
	eventIncluded := false
	for _, ev := range repo.CancelPreviousPipelineEvents {
		if ev == pipeline.Event {
			eventIncluded = true
			break
		}
	}
	if !eventIncluded {
		return nil
	}

	// get all active activeBuilds
	activeBuilds, err := _store.GetActivePipelineList(repo)
	if err != nil {
		return err
	}

	pipelineNeedsCancel := func(active *model.Pipeline) bool {
		// always filter on same event
		if active.Event != pipeline.Event {
			return false
		}

		// find events for the same context
		switch pipeline.Event {
		case model.EventPush:
			return pipeline.Branch == active.Branch
		default:
			return pipeline.Refspec == active.Refspec
		}
	}

	for _, active := range activeBuilds {
		if active.ID == pipeline.ID {
			// same pipeline. e.g. self
			continue
		}

		cancel := pipelineNeedsCancel(active)

		if !cancel {
			continue
		}

		if err = Cancel(ctx, _forge, _store, repo, user, active); err != nil {
			log.Error().
				Err(err).
				Str("ref", active.Ref).
				Int64("id", active.ID).
				Msg("failed to cancel pipeline")
		}
	}

	return nil
}

// CancelWorkflow cancels a specific workflow and returns the status.
//
// Note: We intentionally do NOT gate on the parent pipeline's status here. A
// workflow can legitimately be in pending/running while the pipeline is in a
// terminal state — the typical case is an autoscaler-provisioned agent that
// dies before acking a queue.Error from a previous Cancel(), leaving the
// workflow as a "ghost" in the DB. The UI keeps showing the cancel button
// (it's gated on workflow state, not pipeline state), and the user must be
// able to clean up that ghost. The actual cancel-eligibility check happens
// against the target workflow's state below.
func CancelWorkflow(ctx context.Context, _forge forge.Forge, store store.Store, repo *model.Repo, user *model.User, pipeline *model.Pipeline, workflowID int64) error {
	workflows, err := store.WorkflowGetTree(pipeline)
	if err != nil {
		return &ErrNotFound{Msg: err.Error()}
	}

	// Find the specific workflow to cancel
	var targetWorkflow *model.Workflow
	for _, workflow := range workflows {
		if workflow.ID == workflowID {
			targetWorkflow = workflow
			break
		}
	}

	if targetWorkflow == nil {
		return &ErrNotFound{Msg: fmt.Sprintf("Workflow with ID %d not found in pipeline", workflowID)}
	}

	if targetWorkflow.State != model.StatusRunning && targetWorkflow.State != model.StatusPending {
		return &ErrBadRequest{Msg: fmt.Sprintf("Cannot cancel a workflow with status %s", targetWorkflow.State)}
	}

	// Cancel/evict the specific workflow
	switch targetWorkflow.State {
	case model.StatusRunning:
		if err := server.Config.Services.Queue.Error(ctx, fmt.Sprint(targetWorkflow.ID), queue.ErrCancel); err != nil {
			log.Error().Err(err).Msgf("queue: error: %v", targetWorkflow.ID)
		}
	case model.StatusPending:
		if err := server.Config.Services.Queue.Evict(ctx, fmt.Sprint(targetWorkflow.ID)); err != nil {
			log.Error().Err(err).Msgf("queue: evict: %v", targetWorkflow.ID)
		}
		if err := server.Config.Services.Queue.Error(ctx, fmt.Sprint(targetWorkflow.ID), queue.ErrCancel); err != nil {
			log.Error().Err(err).Msgf("queue: evict: %v", targetWorkflow.ID)
		}
	}

	// Update the DB status for the specific workflow
	switch targetWorkflow.State {
	case model.StatusPending:
		if _, err = UpdateWorkflowToStatusSkipped(store, *targetWorkflow); err != nil {
			log.Error().Err(err).Msgf("cannot update workflow with id %d state", targetWorkflow.ID)
		}
	case model.StatusRunning:
		if _, err = UpdateWorkflowToStatusKilled(store, *targetWorkflow); err != nil {
			log.Error().Err(err).Msgf("cannot update workflow with id %d state to killed", targetWorkflow.ID)
		}
	}
	for _, step := range targetWorkflow.Children {
		switch step.State {
		case model.StatusPending:
			if _, err = UpdateStepToStatusSkipped(store, *step, 0); err != nil {
				log.Error().Err(err).Msgf("cannot update step with id %d state", step.ID)
			}
		case model.StatusRunning:
			if _, err = UpdateStepToStatusKilled(store, *step); err != nil {
				log.Error().Err(err).Msgf("cannot update step with id %d state to killed", step.ID)
			}
		}
	}

	// Update pipeline status if needed
	if pipeline.Workflows, err = store.WorkflowGetTree(pipeline); err != nil {
		return err
	}

	// Check if all workflows are now completed/failed
	allCompleted := true
	for _, workflow := range pipeline.Workflows {
		if workflow.Running() {
			allCompleted = false
			break
		}
	}

	// If all workflows are completed, update pipeline status — but only if
	// the pipeline is still in a non-terminal state. Ghost-workflow cleanup on
	// an already-terminal pipeline must not downgrade or refresh that
	// outcome (e.g., re-pushing a stale "killed" forge status).
	pipelineActive := pipeline.Status == model.StatusRunning ||
		pipeline.Status == model.StatusPending ||
		pipeline.Status == model.StatusBlocked
	if allCompleted && pipelineActive {
		pipeline.Status = model.PipelineStatus(pipeline.Workflows)
		if err := store.UpdatePipeline(pipeline); err != nil {
			log.Error().Err(err).Msgf("UpdatePipeline: %v", pipeline)
			return err
		}
		updatePipelineStatus(ctx, _forge, pipeline, repo, user)
	}

	publishToTopic(pipeline, repo)

	return nil
}
