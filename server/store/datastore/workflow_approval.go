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

package datastore

import (
	"context"

	"codefloe.com/crowci/crow/v5/server/model"
)

// WorkflowsAwaitingApprovalBefore returns workflows whose approval gate has
// expired (deadline > 0 and <= now) and which are still in the pending state.
// Used by the timeout sweeper.
func (s storage) WorkflowsAwaitingApprovalBefore(now int64) ([]*model.Workflow, error) {
	var out []*model.Workflow
	err := s.db.NewSelect().
		Model(&out).
		Where("requires_approval = ?", true).
		Where("approval_state = ?", model.ApprovalPending).
		Where("approval_deadline > 0").
		Where("approval_deadline <= ?", now).
		Scan(context.Background())
	return out, err
}

// WorkflowApprovalEventCreate inserts a single audit row recording an
// approval decision (or auto-expiry) against a workflow gate.
func (s storage) WorkflowApprovalEventCreate(ev *model.WorkflowApprovalEvent) error {
	_, err := s.db.NewInsert().Model(ev).Exec(context.Background())
	return err
}

// WorkflowApprovalEventsForWorkflow returns the full audit trail for a
// workflow, ordered by created_at ascending.
func (s storage) WorkflowApprovalEventsForWorkflow(workflowID int64) ([]*model.WorkflowApprovalEvent, error) {
	var out []*model.WorkflowApprovalEvent
	err := s.db.NewSelect().
		Model(&out).
		Where("workflow_id = ?", workflowID).
		OrderExpr("created_at ASC").
		Scan(context.Background())
	return out, err
}

// WorkflowsPendingApprovalForUser returns workflows whose approval gate is in
// the pending state. The caller is expected to filter the result set through
// authz.CanApproveWorkflow because team membership resolution requires forge
// calls outside the scope of a raw SQL query.
func (s storage) WorkflowsPendingApprovalForUser(_ int64) ([]*model.Workflow, error) {
	var out []*model.Workflow
	err := s.db.NewSelect().
		Model(&out).
		Where("approval_state = ?", model.ApprovalPending).
		Scan(context.Background())
	return out, err
}
