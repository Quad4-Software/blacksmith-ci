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

package api

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"codefloe.com/crowci/crow/v6/pipeline"
	"codefloe.com/crowci/crow/v6/pipeline/rpc"
	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/services/agentrestart"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/version"
)

// validateOrgIDs ensures every referenced organization exists.
func validateOrgIDs(s store.Store, orgIDs []int64) error {
	for _, id := range orgIDs {
		if _, err := s.OrgGet(id); err != nil {
			return huma.Error400BadRequest(fmt.Sprintf("organization %d does not exist", id))
		}
	}
	return nil
}

//
// Global Agents.
//

// The org sets are the same when they hold the same ids in any order.
func sameOrgSet(a, b []int64) bool {
	x, y := slices.Clone(a), slices.Clone(b)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(slices.Compact(x), slices.Compact(y))
}

type GetAgentsInput struct {
	PaginationParams
	Search   string `query:"search" doc:"search in name, platform, backend, custom labels, ID"`
	Platform string `query:"platform" doc:"filter by platform"`
	Backend  string `query:"backend" doc:"filter by backend"`
	Sort     string `query:"sort" default:"id" doc:"sort field: id, name, platform, backend, last_contact, capacity"`
	Order    string `query:"order" default:"asc" doc:"sort order: asc or desc"`
}

type GetAgentsOutput struct {
	Body []model.AgentWithStats
}

func GetAgents(ctx context.Context, input *GetAgentsInput) (*GetAgentsOutput, error) {
	c := ginCtx(ctx)
	opts := &model.AgentListOptions{
		ListOptions: model.ListOptions{Page: input.Page, PerPage: input.PerPage},
		Search:      input.Search,
		Platform:    input.Platform,
		Backend:     input.Backend,
		Sort:        input.Sort,
		Order:       input.Order,
	}

	agents, err := store.FromContext(c).AgentList(opts)
	if err != nil {
		return nil, huma.Error500InternalServerError("Error getting agent list", err)
	}

	info := server.Config.Services.Queue.Info(c)
	runningCount := make(map[int64]int)
	for _, task := range info.Running {
		runningCount[task.AgentID]++
	}

	resp := make([]model.AgentWithStats, 0, len(agents))
	for _, agent := range agents {
		if agent == nil {
			continue
		}
		resp = append(resp, model.AgentWithStats{
			Agent:            *agent,
			RunningWorkflows: runningCount[agent.ID],
		})
	}

	return &GetAgentsOutput{Body: resp}, nil
}

type GetAgentInput struct {
	AgentID int64 `path:"agent_id" doc:"the agent's id"`
}

type GetAgentOutput struct {
	Body *model.AgentWithStats
}

func GetAgent(ctx context.Context, input *GetAgentInput) (*GetAgentOutput, error) {
	c := ginCtx(ctx)
	agent, err := store.FromContext(c).AgentFind(input.AgentID)
	if err != nil {
		return nil, dbErr(err)
	}

	info := server.Config.Services.Queue.Info(c)
	running := 0
	for _, task := range info.Running {
		if task.AgentID == agent.ID {
			running++
		}
	}

	return &GetAgentOutput{Body: &model.AgentWithStats{
		Agent:            *agent,
		RunningWorkflows: running,
	}}, nil
}

type GetAgentTasksInput struct {
	AgentID int64 `path:"agent_id" doc:"the agent's id"`
}

type GetAgentTasksOutput struct {
	Body []model.QueueTask
}

func GetAgentTasks(ctx context.Context, input *GetAgentTasksInput) (*GetAgentTasksOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	agent, err := _store.AgentFind(input.AgentID)
	if err != nil {
		return nil, dbErr(err)
	}

	var running []*model.Task
	info := server.Config.Services.Queue.Info(c)
	for _, task := range info.Running {
		if task.AgentID == agent.ID {
			running = append(running, task)
		}
	}

	enriched, err := processQueueTasks(_store, running, map[int64]string{})
	if err != nil {
		return nil, huma.Error500InternalServerError("error enriching agent tasks", err)
	}

	return &GetAgentTasksOutput{Body: enriched}, nil
}

type GetAgentWorkflowsInput struct {
	PaginationParams
	AgentID int64 `path:"agent_id" doc:"the agent's id"`
}

type GetAgentWorkflowsOutput struct {
	Body []*model.AgentWorkflow
}

// GetAgentWorkflows returns the historic workflows processed by an agent,
// most recently finished first, paginated.
func GetAgentWorkflows(ctx context.Context, input *GetAgentWorkflowsInput) (*GetAgentWorkflowsOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	agent, err := _store.AgentFind(input.AgentID)
	if err != nil {
		return nil, dbErr(err)
	}

	workflows, err := _store.WorkflowListByAgent(agent.ID, &model.ListOptions{
		Page:    input.Page,
		PerPage: input.PerPage,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("error getting agent workflows", err)
	}

	return &GetAgentWorkflowsOutput{Body: workflows}, nil
}

type PatchAgentBody struct {
	Name       string   `json:"name" doc:"name of the agent"`
	NoSchedule bool     `json:"no_schedule" doc:"disable scheduling for this agent"`
	Priority   int32    `json:"priority" doc:"agent priority"`
	OrgIDs     *[]int64 `json:"org_ids,omitempty" doc:"organizations this agent is scoped to; empty means global. Changing them requires agents:register; ignored on org-scoped patch"`
}

type PatchAgentInput struct {
	AgentID int64 `path:"agent_id" doc:"the agent's id"`
	Body    PatchAgentBody
}

type PatchAgentOutput struct {
	Body *model.Agent
}

func PatchAgent(ctx context.Context, input *PatchAgentInput) (*PatchAgentOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	agent, err := _store.AgentFind(input.AgentID)
	if err != nil {
		return nil, dbErr(err)
	}

	agent.Name = input.Body.Name
	agent.NoSchedule = input.Body.NoSchedule
	if input.Body.Priority >= 0 {
		agent.Priority = input.Body.Priority
	}
	if agent.NoSchedule {
		server.Config.Services.Queue.KickAgentWorkers(agent.ID)
	}

	if input.Body.OrgIDs != nil && !sameOrgSet(agent.OrgIDs, *input.Body.OrgIDs) {
		// Changing the org scope can turn an agent global, so it needs the
		// same permission as registering one.
		if !session.User(c).Can(model.PermAgentsRegister) {
			return nil, huma.Error403Forbidden("changing an agent's organizations requires the agents:register permission")
		}
		if err := validateOrgIDs(_store, *input.Body.OrgIDs); err != nil {
			return nil, err
		}
		agent.OrgIDs = *input.Body.OrgIDs
	}

	if err = _store.AgentUpdate(agent); err != nil {
		return nil, huma.Error409Conflict("conflict updating agent")
	}

	return &PatchAgentOutput{Body: agent}, nil
}

type PostAgentBody struct {
	Name         string            `json:"name" minLength:"1" doc:"name of the agent"`
	NoSchedule   bool              `json:"no_schedule,omitempty" doc:"disable scheduling for this agent"`
	Priority     int32             `json:"priority,omitempty" doc:"agent priority"`
	CustomLabels map[string]string `json:"custom_labels,omitempty" doc:"custom labels for the agent"`
	OrgIDs       []int64           `json:"org_ids,omitempty" doc:"organizations this agent is scoped to; empty means global"`
}

type PostAgentInput struct {
	Body PostAgentBody
}

type PostAgentOutput struct {
	Body *model.AgentWithToken
}

func PostAgent(ctx context.Context, input *PostAgentInput) (*PostAgentOutput, error) {
	c := ginCtx(ctx)
	if err := validateCustomLabels(input.Body.CustomLabels); err != nil {
		return nil, err
	}
	user := session.User(c)
	_store := store.FromContext(c)

	if err := validateOrgIDs(_store, input.Body.OrgIDs); err != nil {
		return nil, err
	}

	agent := &model.Agent{
		Name:         input.Body.Name,
		OwnerID:      user.ID,
		OrgIDs:       input.Body.OrgIDs,
		NoSchedule:   input.Body.NoSchedule,
		Priority:     input.Body.Priority,
		CustomLabels: input.Body.CustomLabels,
		Token:        model.GenerateNewAgentToken(),
	}
	if err := _store.AgentCreate(agent); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	// Return the token only on creation.
	return &PostAgentOutput{Body: &model.AgentWithToken{
		Agent: *agent,
		Token: agent.Token,
	}}, nil
}

type AgentRestartInput struct {
	AgentID int64 `path:"agent_id" doc:"the agent's id"`
}

// PostAgentRestart asks an agent to restart onto a newer release. The server
// stops handing the agent work at once, the agent finishes its running
// workflows, exits, and its supervisor starts it again. The request is cleared
// when the agent registers again.
func PostAgentRestart(ctx context.Context, input *AgentRestartInput) (*PatchAgentOutput, error) {
	return setAgentRestartRequested(ctx, input.AgentID, true)
}

// DeleteAgentRestart withdraws a restart request the agent has not acted on yet.
func DeleteAgentRestart(ctx context.Context, input *AgentRestartInput) (*PatchAgentOutput, error) {
	return setAgentRestartRequested(ctx, input.AgentID, false)
}

func setAgentRestartRequested(ctx context.Context, agentID int64, requested bool) (*PatchAgentOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	agent, err := _store.AgentFind(agentID)
	if err != nil {
		return nil, dbErr(err)
	}

	if requested && agent.RemoteUpdate != rpc.RemoteUpdateRestart {
		return nil, huma.Error409Conflict("the agent does not accept restart requests, set CROW_AGENT_REMOTE_UPDATE=restart on the agent to allow them")
	}

	if err := _store.AgentSetRestartRequested(agent.ID, requested); err != nil {
		return nil, huma.Error500InternalServerError("Error updating agent", err)
	}
	agent.RestartRequested = requested
	agent.RestartQueued = false
	if requested {
		// Release the agent's waiting polls so none of them picks up new work.
		server.Config.Services.Queue.KickAgentWorkers(agent.ID)
	}

	return &PatchAgentOutput{Body: agent}, nil
}

type PostAgentRestartsOutput struct {
	Body struct {
		Queued int `json:"queued" doc:"number of agents queued for a restart"`
	}
}

// PostAgentRestarts queues a restart for every agent that accepts restart
// requests and runs a release other than the server's. The queued agents
// restart a few at a time, limited by CROW_AGENT_AUTO_RESTART_MAX_PARALLEL, so
// the instance keeps capacity while they update.
func PostAgentRestarts(ctx context.Context, _ *struct{}) (*PostAgentRestartsOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	agents, err := _store.AgentList(&model.AgentListOptions{ListOptions: model.ListOptions{All: true}})
	if err != nil {
		return nil, huma.Error500InternalServerError("Error listing agents", err)
	}

	out := &PostAgentRestartsOutput{}
	serverVersion := version.String()
	for _, agent := range agents {
		if agent.RemoteUpdate != rpc.RemoteUpdateRestart || agent.RestartRequested || agent.RestartQueued ||
			!agentrestart.ReleaseDiffers(agent, serverVersion) {
			continue
		}
		if err := _store.AgentQueueRestart(agent.ID); err != nil {
			return nil, huma.Error500InternalServerError("Error queueing agent restart", err)
		}
		out.Body.Queued++
	}
	return out, nil
}

type DeleteAgentInput struct {
	AgentID int64 `path:"agent_id" doc:"the agent's id"`
}

func DeleteAgent(ctx context.Context, input *DeleteAgentInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	agent, err := _store.AgentFind(input.AgentID)
	if err != nil {
		return nil, dbErr(err)
	}

	info := server.Config.Services.Queue.Info(c)
	for _, task := range info.Running {
		if task.AgentID == agent.ID {
			return nil, huma.Error409Conflict("Agent has running tasks")
		}
	}

	server.Config.Services.Queue.KickAgentWorkers(agent.ID)

	if err = _store.AgentDelete(agent); err != nil {
		return nil, huma.Error500InternalServerError("Error deleting agent", err)
	}
	return nil, nil
}

//
// Org/User Agents.
//

type PostOrgAgentInput struct {
	OrgID int64 `path:"org_id" doc:"the organization's id"`
	Body  PostAgentBody
}

func PostOrgAgent(ctx context.Context, input *PostOrgAgentInput) (*PostAgentOutput, error) {
	c := ginCtx(ctx)
	if err := validateCustomLabels(input.Body.CustomLabels); err != nil {
		return nil, err
	}
	user := session.User(c)

	agent := &model.Agent{
		Name:         input.Body.Name,
		OwnerID:      user.ID,
		OrgIDs:       model.JSONSliceInt64{input.OrgID},
		NoSchedule:   input.Body.NoSchedule,
		Priority:     input.Body.Priority,
		CustomLabels: input.Body.CustomLabels,
		Token:        model.GenerateNewAgentToken(),
	}

	if err := store.FromContext(c).AgentCreate(agent); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	// Return the token only on creation.
	return &PostAgentOutput{Body: &model.AgentWithToken{
		Agent: *agent,
		Token: agent.Token,
	}}, nil
}

type GetOrgAgentsInput struct {
	PaginationParams
	OrgID int64 `path:"org_id" doc:"the organization's id"`
}

func GetOrgAgents(ctx context.Context, input *GetOrgAgentsInput) (*GetAgentsOutput, error) {
	c := ginCtx(ctx)
	org := session.Org(c)

	agents, err := store.FromContext(c).AgentListForOrg(org.ID, &model.ListOptions{
		Page:    input.Page,
		PerPage: input.PerPage,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("Error getting agent list", err)
	}

	info := server.Config.Services.Queue.Info(c)
	runningCount := make(map[int64]int)
	for _, task := range info.Running {
		runningCount[task.AgentID]++
	}

	resp := make([]model.AgentWithStats, 0, len(agents))
	for _, agent := range agents {
		if agent == nil {
			continue
		}
		resp = append(resp, model.AgentWithStats{
			Agent:            *agent,
			RunningWorkflows: runningCount[agent.ID],
		})
	}

	return &GetAgentsOutput{Body: resp}, nil
}

type PatchOrgAgentInput struct {
	OrgID   int64 `path:"org_id" doc:"the organization's id"`
	AgentID int64 `path:"agent_id" doc:"the agent's id"`
	Body    PatchAgentBody
}

func PatchOrgAgent(ctx context.Context, input *PatchOrgAgentInput) (*PatchAgentOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	org := session.Org(c)

	agent, err := _store.AgentFind(input.AgentID)
	if err != nil {
		return nil, huma.Error404NotFound("Agent not found")
	}

	if !slices.Contains(agent.OrgIDs, org.ID) {
		return nil, huma.Error400BadRequest("Agent does not belong to this organization")
	}

	agent.Name = input.Body.Name
	agent.NoSchedule = input.Body.NoSchedule
	if input.Body.Priority >= 0 {
		agent.Priority = input.Body.Priority
	}
	if agent.NoSchedule {
		server.Config.Services.Queue.KickAgentWorkers(agent.ID)
	}

	if err := _store.AgentUpdate(agent); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &PatchAgentOutput{Body: agent}, nil
}

type DeleteOrgAgentInput struct {
	OrgID   int64 `path:"org_id" doc:"the organization's id"`
	AgentID int64 `path:"agent_id" doc:"the agent's id"`
}

func DeleteOrgAgent(ctx context.Context, input *DeleteOrgAgentInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	org := session.Org(c)

	agent, err := _store.AgentFind(input.AgentID)
	if err != nil {
		return nil, huma.Error404NotFound("Agent not found")
	}

	if !slices.Contains(agent.OrgIDs, org.ID) {
		return nil, huma.Error400BadRequest("Agent does not belong to this organization")
	}

	info := server.Config.Services.Queue.Info(c)
	for _, task := range info.Running {
		if task.AgentID == agent.ID {
			return nil, huma.Error409Conflict("Agent has running tasks")
		}
	}

	// De-scope: drop this org from the agent. Keep the agent alive if other
	// orgs still use it; only delete when this was its sole scope.
	remaining := slices.DeleteFunc(slices.Clone(agent.OrgIDs), func(id int64) bool {
		return id == org.ID
	})
	if len(remaining) > 0 {
		agent.OrgIDs = remaining
		if err := _store.AgentUpdate(agent); err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		return nil, nil
	}

	server.Config.Services.Queue.KickAgentWorkers(agent.ID)

	if err := _store.AgentDelete(agent); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return nil, nil
}

// The validateCustomLabels check rejects custom labels with the reserved
// crowci/ prefix, which the server uses for task metadata and agent scoping.
func validateCustomLabels(labels map[string]string) error {
	if _, dropped := model.StripReservedAgentLabels(labels); len(dropped) > 0 {
		return huma.Error422UnprocessableEntity(fmt.Sprintf("custom labels must not use the reserved %s/ prefix: %s",
			pipeline.InternalLabelPrefix, strings.Join(dropped, ", ")))
	}
	return nil
}
