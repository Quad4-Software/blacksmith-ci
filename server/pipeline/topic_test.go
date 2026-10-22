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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func TestAnnotateAutoscalerProvisioningReportsProgress(t *testing.T) {
	now := time.Now().Unix()
	s := store_mocks.NewMockStore(t)
	s.On("AutoscalerList", mock.Anything).Return([]*model.Autoscaler{{
		ID:               1,
		Enabled:          true,
		MaxAgents:        1,
		LastContact:      now,
		PendingAgents:    1,
		PendingSince:     now - 600,
		PendingAgentIDs:  model.JSONSliceInt64{5, 6, 7},
		ProvisionTimeout: 1800,
		AgentLabels:      model.JSONMapStringString{"group": "windows-vm"},
	}}, nil)
	s.On("TaskListPending").Return([]*model.Task{
		{PipelineID: 10, Name: "windows", Labels: map[string]string{"group": "windows-vm"}},
	}, nil)
	s.On("AgentFind", int64(5)).Return(&model.Agent{ID: 5, Name: "host-5", Provisioning: model.JSONSliceProvisioningPhase{
		{Phase: "Installing Hyper-V", At: now - 500},
		{Phase: "Building the base image", Detail: `C:\crow-images: access denied`, Failed: true, At: now - 100},
	}}, nil)
	// Connected agents are no longer provisioning, and a deleted one is skipped.
	s.On("AgentFind", int64(6)).Return(&model.Agent{ID: 6, Name: "host-6", LastContact: now}, nil)
	s.On("AgentFind", int64(7)).Return(nil, assert.AnError)

	pl := &model.Pipeline{ID: 10, Workflows: []*model.Workflow{{Name: "windows", State: model.StatusPending}}}
	AnnotateAutoscalerProvisioning(s, pl)

	wf := pl.Workflows[0]
	assert.True(t, wf.AutoscalerProvisioning)
	// The autoscaler's own timeout applies, not the 5-minute default.
	assert.False(t, wf.AutoscalerProvisioningStale)
	assert.Equal(t, now-600, wf.AutoscalerProvisioningSince)
	assert.Equal(t, int64(1800), wf.AutoscalerProvisionTimeout)
	require.Len(t, wf.ProvisioningAgents, 1)
	assert.Equal(t, &model.ProvisioningAgent{ID: 5, Name: "host-5", Phases: []model.ProvisioningPhase{
		{Phase: "Installing Hyper-V", At: now - 500},
		{Phase: "Building the base image", Failed: true, At: now - 100},
	}}, wf.ProvisioningAgents[0], "the host's free-text detail must not reach workflow viewers")
}
