// Copyright 2025 Crow Authors
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
	"time"

	"github.com/danielgtaylor/huma/v2"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/store"
)

type GetAutoscalersInput struct {
	PaginationParams
	Search   string `query:"search" doc:"search in name, provider, region, ID"`
	Provider string `query:"provider" doc:"filter by provider"`
	Enabled  string `query:"enabled" doc:"filter by enabled status (true/false, empty=all)"`
	Sort     string `query:"sort" default:"id" doc:"sort field: id, name, provider, last_contact"`
	Order    string `query:"order" default:"asc" doc:"sort order: asc or desc"`
}

type GetAutoscalersOutput struct {
	Body []*model.Autoscaler
}

func GetAutoscalers(ctx context.Context, input *GetAutoscalersInput) (*GetAutoscalersOutput, error) {
	c := ginCtx(ctx)
	var enabledFilter *bool
	if input.Enabled != "" {
		v := input.Enabled == "true"
		enabledFilter = &v
	}

	opts := &model.AutoscalerListOptions{
		ListOptions: model.ListOptions{Page: input.Page, PerPage: input.PerPage},
		Search:      input.Search,
		Provider:    input.Provider,
		Enabled:     enabledFilter,
		Sort:        input.Sort,
		Order:       input.Order,
	}

	autoscalers, err := store.FromContext(c).AutoscalerList(opts)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting autoscaler list. %s", err))
	}

	return &GetAutoscalersOutput{Body: autoscalers}, nil
}

type GetAutoscalerInput struct {
	AutoscalerID int64 `path:"autoscaler_id" doc:"the autoscaler's id"`
}

type GetAutoscalerOutput struct {
	Body *model.Autoscaler
}

func GetAutoscaler(ctx context.Context, input *GetAutoscalerInput) (*GetAutoscalerOutput, error) {
	c := ginCtx(ctx)
	autoscaler, err := store.FromContext(c).AutoscalerFind(input.AutoscalerID)
	if err != nil {
		return nil, dbErr(err)
	}
	return &GetAutoscalerOutput{Body: autoscaler}, nil
}

type GetAutoscalerSelfInput struct{}

type GetAutoscalerSelfOutput struct {
	Body *model.Autoscaler
}

func GetAutoscalerSelf(ctx context.Context, _ *GetAutoscalerSelfInput) (*GetAutoscalerSelfOutput, error) {
	c := ginCtx(ctx)
	autoscaler := session.Autoscaler(c)
	return &GetAutoscalerSelfOutput{Body: autoscaler}, nil
}

type PostAutoscalerBody struct {
	Name string `json:"name" minLength:"1" doc:"name of the autoscaler"`
}

type PostAutoscalerInput struct {
	Body PostAutoscalerBody
}

type PostAutoscalerOutput struct {
	Body *model.AutoscalerWithToken
}

func PostAutoscaler(ctx context.Context, input *PostAutoscalerInput) (*PostAutoscalerOutput, error) {
	c := ginCtx(ctx)

	user := session.User(c)

	autoscaler := &model.Autoscaler{
		Name:    input.Body.Name,
		OwnerID: user.ID,
		OrgID:   model.IDNotSet,
		Enabled: true,
		Token:   model.GenerateNewAutoscalerToken(),
	}

	if err := store.FromContext(c).AutoscalerCreate(autoscaler); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	// Return token only on creation
	return &PostAutoscalerOutput{Body: &model.AutoscalerWithToken{
		Autoscaler: autoscaler,
		Token:      autoscaler.Token,
	}}, nil
}

type PatchAutoscalerBody struct {
	Enabled bool `json:"enabled" doc:"whether the autoscaler is enabled"`
}

type PatchAutoscalerInput struct {
	AutoscalerID int64 `path:"autoscaler_id" doc:"the autoscaler's id"`
	Body         PatchAutoscalerBody
}

type PatchAutoscalerOutput struct {
	Body *model.Autoscaler
}

func PatchAutoscaler(ctx context.Context, input *PatchAutoscalerInput) (*PatchAutoscalerOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	autoscaler, err := _store.AutoscalerFind(input.AutoscalerID)
	if err != nil {
		return nil, dbErr(err)
	}

	autoscaler.Enabled = input.Body.Enabled

	if err = _store.AutoscalerUpdate(autoscaler); err != nil {
		return nil, huma.Error409Conflict("conflict updating autoscaler")
	}

	return &PatchAutoscalerOutput{Body: autoscaler}, nil
}

type DeleteAutoscalerInput struct {
	AutoscalerID int64 `path:"autoscaler_id" doc:"the autoscaler's id"`
}

func DeleteAutoscaler(ctx context.Context, input *DeleteAutoscalerInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	autoscaler, err := _store.AutoscalerFind(input.AutoscalerID)
	if err != nil {
		return nil, dbErr(err)
	}

	if err = _store.AutoscalerDelete(autoscaler); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error deleting autoscaler. %s", err))
	}
	return nil, nil
}

type HeartbeatBody struct {
	ActiveAgents      int32             `json:"active_agents"`
	PendingAgents     int32             `json:"pending_agents"`
	Version           string            `json:"version"`
	MinAgents         int32             `json:"min_agents"`
	MaxAgents         int32             `json:"max_agents"`
	WorkflowsPerAgent int32             `json:"workflows_per_agent"`
	Provider          string            `json:"provider"`
	InstanceType      string            `json:"instance_type"`
	Region            string            `json:"region"`
	AgentLabels       map[string]string `json:"agent_labels"`
	LimitCPUQuota     string            `json:"limit_cpu_quota"`
	LimitMem          string            `json:"limit_mem"`
}

type PostAutoscalerHeartbeatInput struct {
	AutoscalerID int64 `path:"autoscaler_id" doc:"the autoscaler's id"`
	Body         HeartbeatBody
}

type HeartbeatResponse struct {
	Success     bool  `json:"success"`
	LastContact int64 `json:"last_contact"`
}

type PostAutoscalerHeartbeatOutput struct {
	Body HeartbeatResponse
}

func PostAutoscalerHeartbeat(ctx context.Context, input *PostAutoscalerHeartbeatInput) (*PostAutoscalerHeartbeatOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	// Verify autoscaler exists and token matches
	autoscaler, err := _store.AutoscalerFind(input.AutoscalerID)
	if err != nil {
		return nil, dbErr(err)
	}

	// Update autoscaler with heartbeat data and configuration
	now := time.Now().Unix()

	// Track when agents first became pending
	if input.Body.PendingAgents > 0 && autoscaler.PendingAgents == 0 {
		// Transition from no pending to pending: record the start time
		autoscaler.PendingSince = now
	} else if input.Body.PendingAgents == 0 {
		// No longer pending: clear the timestamp
		autoscaler.PendingSince = 0
	}
	// If already pending, keep the existing PendingSince value

	autoscaler.ActiveAgents = input.Body.ActiveAgents
	autoscaler.PendingAgents = input.Body.PendingAgents
	autoscaler.Version = input.Body.Version
	autoscaler.LastContact = now

	// Update configuration fields if provided
	if input.Body.MinAgents > 0 || input.Body.MaxAgents > 0 {
		autoscaler.MinAgents = input.Body.MinAgents
		autoscaler.MaxAgents = input.Body.MaxAgents
	}
	if input.Body.WorkflowsPerAgent > 0 {
		autoscaler.WorkflowsPerAgent = input.Body.WorkflowsPerAgent
	}
	if input.Body.Provider != "" {
		autoscaler.Provider = input.Body.Provider
	}
	if input.Body.InstanceType != "" {
		autoscaler.InstanceType = input.Body.InstanceType
	}
	if input.Body.Region != "" {
		autoscaler.Region = input.Body.Region
	}
	if input.Body.AgentLabels != nil {
		autoscaler.AgentLabels = input.Body.AgentLabels
	}
	if input.Body.LimitCPUQuota != "" {
		autoscaler.LimitCPUQuota = input.Body.LimitCPUQuota
	}
	if input.Body.LimitMem != "" {
		autoscaler.LimitMem = input.Body.LimitMem
	}

	if err = _store.AutoscalerUpdate(autoscaler); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error updating heartbeat. %s", err))
	}

	return &PostAutoscalerHeartbeatOutput{Body: HeartbeatResponse{
		Success:     true,
		LastContact: time.Now().Unix(),
	}}, nil
}

type PostAutoscalerTokenInput struct {
	AutoscalerID int64 `path:"autoscaler_id" doc:"the autoscaler's id"`
}

type TokenResponse struct {
	Token string `json:"token"`
}

type PostAutoscalerTokenOutput struct {
	Body TokenResponse
}

func PostAutoscalerToken(ctx context.Context, input *PostAutoscalerTokenInput) (*PostAutoscalerTokenOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	autoscaler, err := _store.AutoscalerFind(input.AutoscalerID)
	if err != nil {
		return nil, dbErr(err)
	}

	// Generate new token
	autoscaler.Token = model.GenerateNewAutoscalerToken()

	if err = _store.AutoscalerUpdate(autoscaler); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error updating token. %s", err))
	}

	return &PostAutoscalerTokenOutput{Body: TokenResponse{
		Token: autoscaler.Token,
	}}, nil
}
