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

	"github.com/danielgtaylor/huma/v2"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/store"
)

//
// Global Agents.
//

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

type PatchAgentBody struct {
	Name       string `json:"name" doc:"name of the agent"`
	NoSchedule bool   `json:"no_schedule" doc:"disable scheduling for this agent"`
	Priority   int32  `json:"priority" doc:"agent priority"`
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
}

type PostAgentInput struct {
	Body PostAgentBody
}

type PostAgentOutput struct {
	Body *model.Agent
}

func PostAgent(ctx context.Context, input *PostAgentInput) (*PostAgentOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)

	agent := &model.Agent{
		Name:         input.Body.Name,
		OwnerID:      user.ID,
		OrgID:        model.IDNotSet,
		NoSchedule:   input.Body.NoSchedule,
		Priority:     input.Body.Priority,
		CustomLabels: input.Body.CustomLabels,
		Token:        model.GenerateNewAgentToken(),
	}
	if err := store.FromContext(c).AgentCreate(agent); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return &PostAgentOutput{Body: agent}, nil
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
	user := session.User(c)

	agent := &model.Agent{
		Name:         input.Body.Name,
		OwnerID:      user.ID,
		OrgID:        input.OrgID,
		NoSchedule:   input.Body.NoSchedule,
		Priority:     input.Body.Priority,
		CustomLabels: input.Body.CustomLabels,
		Token:        model.GenerateNewAgentToken(),
	}

	if err := store.FromContext(c).AgentCreate(agent); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &PostAgentOutput{Body: agent}, nil
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

	if agent.OrgID != org.ID {
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

	if agent.OrgID != org.ID {
		return nil, huma.Error400BadRequest("Agent does not belong to this organization")
	}

	info := server.Config.Services.Queue.Info(c)
	for _, task := range info.Running {
		if task.AgentID == agent.ID {
			return nil, huma.Error409Conflict("Agent has running tasks")
		}
	}

	server.Config.Services.Queue.KickAgentWorkers(agent.ID)

	if err := _store.AgentDelete(agent); err != nil {
		return nil, huma.Error500InternalServerError("Error deleting agent", err)
	}

	return nil, nil
}
