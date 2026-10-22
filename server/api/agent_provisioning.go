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
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
)

type ProvisioningBody struct {
	Phase  string `json:"phase"            minLength:"1" maxLength:"100" doc:"what the host is doing, for example 'Building the base image'"`
	Detail string `json:"detail,omitempty" maxLength:"1000" doc:"free text returned with the agent but not with workflows, for example the error of a failed phase"`
	Failed bool   `json:"failed,omitempty" doc:"whether the phase failed"`
}

type PostAgentProvisioningInput struct {
	Body ProvisioningBody
}

// PostAgentProvisioning records a provisioning phase reported by the host that
// is being set up to run an agent. The host authenticates with the agent's own
// token, which an autoscaler hands it before the agent first connects.
func PostAgentProvisioning(ctx context.Context, input *PostAgentProvisioningInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	scheme, agentToken, ok := strings.Cut(c.GetHeader("Authorization"), " ")
	agentToken = strings.TrimSpace(agentToken)
	if !ok || !strings.EqualFold(scheme, "bearer") || agentToken == "" {
		return nil, huma.Error401Unauthorized("missing agent token")
	}
	agent, err := _store.AgentFindByToken(agentToken)
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid agent token")
	}

	agent.AddProvisioningPhase(model.ProvisioningPhase{
		Phase:  input.Body.Phase,
		Detail: input.Body.Detail,
		Failed: input.Body.Failed,
		At:     time.Now().Unix(),
	})
	if err := _store.AgentUpdate(agent); err != nil {
		return nil, dbErr(err)
	}
	return nil, nil
}
