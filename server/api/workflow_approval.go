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

package api

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/authz"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/pipeline"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/store"
)

// WorkflowApprovalInput describes the path/body shape for approve and decline.
type WorkflowApprovalInput struct {
	RepoID         int64 `path:"repo_id" doc:"the repository id"`
	PipelineNumber int64 `path:"number"  doc:"the pipeline number"`
	WorkflowPID    int   `path:"wf_pid"  doc:"the workflow PID inside the pipeline"`
	Body           *struct {
		Reason string `json:"reason,omitempty" doc:"optional human-readable note recorded in the audit trail"`
	}
}

// WorkflowApprovalOutput returns the updated workflow row.
type WorkflowApprovalOutput struct {
	Body *model.Workflow
}

// PostWorkflowApprove approves a per-workflow approval gate.
func PostWorkflowApprove(ctx context.Context, input *WorkflowApprovalInput) (*WorkflowApprovalOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)

	wf, err := loadWorkflowByPID(_store, repo, input.PipelineNumber, input.WorkflowPID)
	if err != nil {
		return nil, err
	}

	allowed, _, err := authz.CanApproveWorkflow(ctx, _store, server.Config.Services.Manager, user, wf, repo)
	if err != nil {
		return nil, huma.Error500InternalServerError("authorization check failed", err)
	}
	if !allowed {
		return nil, huma.Error403Forbidden("you are not authorized to approve this gate")
	}

	reason := ""
	if input.Body != nil {
		reason = input.Body.Reason
	}
	if err := pipeline.ApproveWorkflow(ctx, _store, server.Config.Services.Queue, user, wf.ID, reason); err != nil {
		if errors.Is(err, pipeline.ErrApprovalNotPending) {
			return nil, huma.Error409Conflict("approval gate is not pending")
		}
		return nil, huma.Error500InternalServerError("approve failed", err)
	}

	updated, err := _store.WorkflowLoad(wf.ID)
	if err != nil {
		return nil, dbErr(err)
	}
	return &WorkflowApprovalOutput{Body: updated}, nil
}

// PostWorkflowDecline declines a per-workflow approval gate.
func PostWorkflowDecline(ctx context.Context, input *WorkflowApprovalInput) (*WorkflowApprovalOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)

	wf, err := loadWorkflowByPID(_store, repo, input.PipelineNumber, input.WorkflowPID)
	if err != nil {
		return nil, err
	}

	allowed, _, err := authz.CanApproveWorkflow(ctx, _store, server.Config.Services.Manager, user, wf, repo)
	if err != nil {
		return nil, huma.Error500InternalServerError("authorization check failed", err)
	}
	if !allowed {
		return nil, huma.Error403Forbidden("you are not authorized to approve this gate")
	}

	reason := ""
	if input.Body != nil {
		reason = input.Body.Reason
	}
	if err := pipeline.DeclineWorkflow(ctx, _store, server.Config.Services.Queue, user, wf.ID, reason); err != nil {
		if errors.Is(err, pipeline.ErrApprovalNotPending) {
			return nil, huma.Error409Conflict("approval gate is not pending")
		}
		return nil, huma.Error500InternalServerError("decline failed", err)
	}

	updated, err := _store.WorkflowLoad(wf.ID)
	if err != nil {
		return nil, dbErr(err)
	}
	return &WorkflowApprovalOutput{Body: updated}, nil
}

// GetWorkflowApprovalInput identifies a workflow gate by pipeline number + PID.
type GetWorkflowApprovalInput struct {
	RepoID         int64 `path:"repo_id" doc:"the repository id"`
	PipelineNumber int64 `path:"number"  doc:"the pipeline number"`
	WorkflowPID    int   `path:"wf_pid"  doc:"the workflow PID inside the pipeline"`
}

// GetWorkflowApprovalBody is the response shape for the approval-state endpoint.
type GetWorkflowApprovalBody struct {
	Workflow *model.Workflow                `json:"workflow"`
	Events   []*model.WorkflowApprovalEvent `json:"events"`
}

// GetWorkflowApprovalOutput wraps the response body for huma.
type GetWorkflowApprovalOutput struct {
	Body GetWorkflowApprovalBody
}

// GetWorkflowApproval returns the workflow row plus its full audit trail.
func GetWorkflowApproval(ctx context.Context, input *GetWorkflowApprovalInput) (*GetWorkflowApprovalOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)

	wf, err := loadWorkflowByPID(_store, repo, input.PipelineNumber, input.WorkflowPID)
	if err != nil {
		return nil, err
	}
	events, err := _store.WorkflowApprovalEventsForWorkflow(wf.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("audit load failed", err)
	}
	return &GetWorkflowApprovalOutput{Body: GetWorkflowApprovalBody{
		Workflow: wf,
		Events:   events,
	}}, nil
}

// GetUserPendingApprovalsBody is the response shape for the user-pending list.
type GetUserPendingApprovalsBody struct {
	Workflows []*model.Workflow `json:"workflows"`
}

// GetUserPendingApprovalsOutput wraps the response body for huma.
type GetUserPendingApprovalsOutput struct {
	Body GetUserPendingApprovalsBody
}

// GetUserPendingApprovals lists pending approval gates that the current user
// is authorized to act on. The store returns all pending candidates; the
// handler filters them via authz.CanApproveWorkflow because that requires
// forge.Teams() calls we don't want in raw SQL.
func GetUserPendingApprovals(ctx context.Context, _ *struct{}) (*GetUserPendingApprovalsOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)

	candidates, err := _store.WorkflowsPendingApprovalForUser(user.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("pending approvals query failed", err)
	}

	approvable := []*model.Workflow{}
	for _, wf := range candidates {
		pl, err := _store.GetPipeline(wf.PipelineID)
		if err != nil {
			continue
		}
		repo, err := _store.GetRepo(pl.RepoID)
		if err != nil {
			continue
		}
		ok, _, err := authz.CanApproveWorkflow(ctx, _store, server.Config.Services.Manager, user, wf, repo)
		if err == nil && ok {
			approvable = append(approvable, wf)
		}
	}

	return &GetUserPendingApprovalsOutput{Body: GetUserPendingApprovalsBody{
		Workflows: approvable,
	}}, nil
}

// loadWorkflowByPID resolves a (pipeline number, workflow PID) tuple against a
// repo, returning a typed huma error suitable for direct return from a handler.
func loadWorkflowByPID(s store.Store, repo *model.Repo, pipelineNumber int64, wfPID int) (*model.Workflow, error) {
	pl, err := s.GetPipelineNumber(repo, pipelineNumber)
	if err != nil {
		return nil, dbErr(err)
	}
	tree, err := s.WorkflowGetTree(pl)
	if err != nil {
		return nil, huma.Error500InternalServerError("workflow tree load failed", err)
	}
	for _, w := range tree {
		if w.PID == wfPID {
			return w, nil
		}
	}
	return nil, huma.Error404NotFound("workflow not found")
}
