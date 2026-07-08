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

package pipeline

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/queue"
	"codefloe.com/crowci/crow/v6/server/store"
)

// ErrApprovalNotPending is returned when an approve/decline/expire action is
// attempted on a workflow whose ApprovalState is not Pending. The API layer
// maps this to HTTP 409 so the caller can distinguish "already decided" from
// other failures.
var ErrApprovalNotPending = errors.New("approval gate is not in pending state")

// BlockOnApproval transitions a workflow into "blocked awaiting approval".
// Called by the queue when a gated workflow's upstream deps have all cleared.
// The deadline is fixed here (now + effective timeout), not at queue time, so
// long-running upstream workflows do not consume the gate's clock.
func BlockOnApproval(
	ctx context.Context,
	s store.Store,
	q queue.Queue,
	workflowID int64,
	defaultTimeout time.Duration,
) error {
	wf, err := s.WorkflowLoad(workflowID)
	if err != nil {
		return fmt.Errorf("workflow %d load: %w", workflowID, err)
	}
	if !wf.RequiresApproval {
		return nil
	}
	if wf.ApprovalState != model.ApprovalNone {
		return nil // already blocked/approved/declined — idempotent
	}

	effective := time.Duration(wf.ApprovalTimeoutSeconds) * time.Second
	if effective <= 0 {
		effective = defaultTimeout
	}

	wf.State = model.StatusBlocked
	wf.ApprovalState = model.ApprovalPending
	wf.ApprovalDeadline = time.Now().Add(effective).Unix()

	if err := s.WorkflowUpdate(wf); err != nil {
		return fmt.Errorf("workflow %d update: %w", workflowID, err)
	}

	// Mirror to the in-queue task. Best-effort — if the task isn't in the queue
	// any more (server restart racing with state change) the next poll-cycle
	// reload will pick up the new state from the workflow row.
	_ = q.SetApprovalState(ctx, fmt.Sprint(workflowID), model.ApprovalPending)
	// Refresh + publish so SSE clients see the just-activated gate without
	// needing a full page reload. This path is system-driven (queue-triggered
	// when upstream deps clear) so there is no acting user — the helper falls
	// back to the repo owner for the forge.Status auth call.
	refreshAndPublishPipeline(ctx, s, wf.PipelineID, nil)
	return nil
}

// ApproveWorkflow transitions a pending approval gate to Approved, records an
// audit row, and notifies the queue so the workflow can be dispatched.
func ApproveWorkflow(
	ctx context.Context,
	s store.Store,
	q queue.Queue,
	user *model.User,
	workflowID int64,
	reason string,
) error {
	wf, err := s.WorkflowLoad(workflowID)
	if err != nil {
		return fmt.Errorf("workflow %d load: %w", workflowID, err)
	}
	if wf.ApprovalState != model.ApprovalPending {
		return ErrApprovalNotPending
	}

	wf.State = model.StatusPending
	wf.ApprovalState = model.ApprovalApproved
	if err := s.WorkflowUpdate(wf); err != nil {
		return fmt.Errorf("workflow %d update: %w", workflowID, err)
	}
	ev := &model.WorkflowApprovalEvent{
		WorkflowID: workflowID,
		UserID:     user.ID,
		Action:     model.ApprovalActionApproved,
		Reason:     reason,
		CreatedAt:  time.Now().Unix(),
	}
	if err := s.WorkflowApprovalEventCreate(ev); err != nil {
		return fmt.Errorf("audit insert: %w", err)
	}
	if err := q.SetApprovalState(ctx, fmt.Sprint(workflowID), model.ApprovalApproved); err != nil {
		return fmt.Errorf("queue notify: %w", err)
	}
	// Refresh + publish so SSE clients see the gate-decision update without
	// having to wait for another workflow to republish. Errors here must not
	// fail the transition — the gate is already approved in the DB.
	refreshAndPublishPipeline(ctx, s, wf.PipelineID, user)
	return nil
}

// DeclineWorkflow transitions a pending approval gate to Declined, records an
// audit row, and notifies the queue. Downstream workflows that depend on this
// one are cascade-skipped (see cascadeSkipFromDeclined, Task 23).
func DeclineWorkflow(
	ctx context.Context,
	s store.Store,
	q queue.Queue,
	user *model.User,
	workflowID int64,
	reason string,
) error {
	return declineWorkflowInternal(ctx, s, q, user, workflowID, reason,
		model.ApprovalDeclined, model.ApprovalActionDeclined)
}

// ExpireWorkflow is invoked by the timeout sweeper when an approval deadline
// passes without a decision. UserID is 0 to mark the action as system-driven.
func ExpireWorkflow(
	ctx context.Context,
	s store.Store,
	q queue.Queue,
	workflowID int64,
) error {
	return declineWorkflowInternal(ctx, s, q, &model.User{ID: 0}, workflowID,
		"approval timed out", model.ApprovalExpired, model.ApprovalActionExpired)
}

// declineWorkflowInternal is the shared implementation behind DeclineWorkflow
// and ExpireWorkflow. The only difference between the two is the terminal
// ApprovalState (Declined vs Expired) and the audit action verb.
func declineWorkflowInternal(
	ctx context.Context,
	s store.Store,
	q queue.Queue,
	user *model.User,
	workflowID int64,
	reason string,
	newState model.ApprovalState,
	action model.ApprovalAction,
) error {
	wf, err := s.WorkflowLoad(workflowID)
	if err != nil {
		return fmt.Errorf("workflow %d load: %w", workflowID, err)
	}
	if wf.ApprovalState != model.ApprovalPending {
		return ErrApprovalNotPending
	}

	wf.State = model.StatusDeclined
	wf.ApprovalState = newState
	if err := s.WorkflowUpdate(wf); err != nil {
		return fmt.Errorf("workflow %d update: %w", workflowID, err)
	}
	// Skip the declined gate's own pending child steps. WorkflowLoad does not
	// populate Children, so we re-fetch them from the step store. Without
	// this, the step rows linger in Pending — which leaves WorkflowStatus()
	// (when recomputed from children, e.g. by frontend per-workflow rollups)
	// returning Success and the per-workflow icon showing the wrong state.
	// The downstream cascade in cascadeSkipFromDeclined only handles
	// transitively-dependent workflows, not the declined workflow's own steps.
	if err := skipDeclinedWorkflowSteps(s, wf); err != nil {
		log.Error().Err(err).Int64("workflow_id", workflowID).
			Msg("decline: failed to skip pending child steps")
		// fall through — the gate transition is already persisted; downstream
		// cascade and publish should still happen.
	}
	ev := &model.WorkflowApprovalEvent{
		WorkflowID: workflowID,
		UserID:     user.ID,
		Action:     action,
		Reason:     reason,
		CreatedAt:  time.Now().Unix(),
	}
	if err := s.WorkflowApprovalEventCreate(ev); err != nil {
		return fmt.Errorf("audit insert: %w", err)
	}
	if err := q.SetApprovalState(ctx, fmt.Sprint(workflowID), newState); err != nil {
		return fmt.Errorf("queue notify: %w", err)
	}
	// Trigger downstream skip cascade — implemented in Task 23.
	if err := cascadeSkipFromDeclined(ctx, s, q, wf); err != nil {
		return err
	}
	// Refresh + publish so SSE clients see the cascade'd state immediately.
	// For expire/decline, the acting user may be the system stub (ID=0); the
	// helper falls back to the repo owner for the forge call.
	refreshAndPublishPipeline(ctx, s, wf.PipelineID, user)
	return nil
}

// cascadeSkipFromDeclined marks all workflows whose dependency chain
// transitively includes the declined workflow as Skipped (unless they have
// run_on:failure).
//
// The existing CascadeSkipBlockedWorkflows already treats any non-success
// terminal dep (Failure / Killed / Error / Skipped / Declined) as a
// skip-trigger via shouldSkipDueToBlockedDep — Running() returns false for
// Declined and StatusDeclined != StatusSuccess. So this entrypoint just loads
// the parent pipeline and reuses that walker. Splitting it out as its own
// function keeps the call site at decline time intent-revealing and gives us
// a single place to add decline-specific behavior later (e.g. metrics).
func cascadeSkipFromDeclined(ctx context.Context, s store.Store, q queue.Queue, declined *model.Workflow) error {
	pipeline, err := s.GetPipeline(declined.PipelineID)
	if err != nil {
		return fmt.Errorf("cascade skip from declined: load pipeline %d: %w", declined.PipelineID, err)
	}
	return CascadeSkipBlockedWorkflows(ctx, s, q, pipeline)
}

// skipDeclinedWorkflowSteps marks every pending child step of the declined
// workflow as Skipped so the workflow's terminal state reflects in the
// per-step rows. WorkflowLoad does not populate Children, so we re-fetch them
// from the step store. Mirrors the pattern in Cancel (server/pipeline/cancel.go)
// and the downstream-cascade step skip in CascadeSkipBlockedWorkflows
// (server/pipeline/cascade_skip.go).
func skipDeclinedWorkflowSteps(s store.Store, wf *model.Workflow) error {
	steps, err := s.StepListFromWorkflowFind(wf)
	if err != nil {
		return fmt.Errorf("load steps for workflow %d: %w", wf.ID, err)
	}
	for _, step := range steps {
		if step.State != model.StatusPending {
			continue
		}
		if _, err := UpdateStepToStatusSkipped(s, *step, 0); err != nil {
			log.Error().Err(err).Int64("step_id", step.ID).
				Msg("decline: failed to mark pending step as skipped")
		}
	}
	return nil
}

// refreshAndPublishPipeline reloads the parent pipeline, recomputes its
// rollup status from the latest workflow tree, persists the updated status,
// and publishes the change to SSE/websocket subscribers so the UI updates
// without a page reload.
//
// Errors here are logged but never propagated: the underlying gate
// transition has already succeeded and persisted, so a publish failure
// must not roll that back. The next state change (or a manual refresh)
// will reconcile the view.
//
// Actor is the user attributed to the action — for approve/decline this is
// the requesting user, for expire it is the system stub (UserID=0). When
// the actor has no ID (system-driven path) we fall back to the repo owner
// so the forge.Status call has a token to authenticate with, matching the
// idiom used in server/pipeline/approve.go.
func refreshAndPublishPipeline(ctx context.Context, s store.Store, pipelineID int64, actor *model.User) {
	pl, err := s.GetPipeline(pipelineID)
	if err != nil {
		log.Error().Err(err).Int64("pipeline_id", pipelineID).
			Msg("refresh and publish: load pipeline failed")
		return
	}
	repo, err := s.GetRepo(pl.RepoID)
	if err != nil {
		log.Error().Err(err).Int64("repo_id", pl.RepoID).
			Msg("refresh and publish: load repo failed")
		return
	}
	pl.Workflows, err = s.WorkflowGetTree(pl)
	if err != nil {
		log.Error().Err(err).Int64("pipeline_id", pipelineID).
			Msg("refresh and publish: load workflow tree failed")
		return
	}

	// Recompute and persist the pipeline rollup. Only overwrite if the
	// pipeline isn't already terminal in a way the rollup doesn't see — a
	// previously running pipeline becomes Declined the moment a gate
	// rejects, even if other workflows are still pending.
	newStatus := model.PipelineStatus(pl.Workflows)
	if newStatus != pl.Status {
		pl.Status = newStatus
		if err := s.UpdatePipeline(pl); err != nil {
			log.Error().Err(err).Int64("pipeline_id", pipelineID).
				Msg("refresh and publish: persist pipeline status failed")
			// fall through — still publish what we have
		}
	}

	// For the forge.Status call (commit status push), the acting user must
	// have a token. System-driven sweeper actions arrive with UserID=0;
	// fall back to the repo owner the same way other system paths do.
	forgeUser := actor
	if forgeUser == nil || forgeUser.ID == 0 {
		owner, err := s.GetUser(repo.UserID)
		if err != nil {
			log.Error().Err(err).Int64("repo_id", pl.RepoID).
				Msg("refresh and publish: load repo owner failed")
			// Still publish to the topic — only the forge.Status call needs the user.
			publishToTopic(pl, repo)
			return
		}
		forgeUser = owner
	}

	forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Str("repo", repo.FullName).
			Msg("refresh and publish: load forge failed")
		// Still publish to the topic — pubsub doesn't need the forge.
		publishToTopic(pl, repo)
		return
	}

	publishPipeline(ctx, forge, pl, repo, forgeUser)
}
