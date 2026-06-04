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

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/queue"
	"codefloe.com/crowci/crow/v5/server/store"
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
	return cascadeSkipFromDeclined(ctx, s, q, wf)
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
