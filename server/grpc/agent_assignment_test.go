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

package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"codefloe.com/crowci/crow/v6/pipeline/rpc"
	"codefloe.com/crowci/crow/v6/server/model"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func TestCheckAgentAssignment(t *testing.T) {
	tests := []struct {
		name        string
		assigned    bool
		rowExists   bool
		recordedBy  int64
		wantAllowed bool
	}{
		{name: "task row assigned to the agent", assigned: true, wantAllowed: true},
		{name: "task row assigned to another agent", rowExists: true, recordedBy: 1},
		{name: "row gone, Init recorded the agent", recordedBy: 1, wantAllowed: true},
		{name: "row gone, Init recorded another agent", recordedBy: 2},
		{name: "row gone, never initialized", recordedBy: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := store_mocks.NewMockStore(t)
			store.EXPECT().TaskGetAgentID("42", int64(1)).Return(tt.assigned, nil).Once()
			if !tt.assigned {
				store.EXPECT().TaskGetCancelled("42").Return(false, tt.rowExists, nil).Once()
			}
			s := &RPC{store: store}

			err := s.checkAgentAssigned(&model.Agent{ID: 1}, &model.Workflow{ID: 42, AgentID: tt.recordedBy})
			if tt.wantAllowed {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, errAgentNotAssigned)
			}
		})
	}
}

// An agent that can access the repo but was not handed the workflow must not
// be able to claim it through Init.
func TestInitRejectsUnassignedAgent(t *testing.T) {
	store := store_mocks.NewMockStore(t)
	workflow := &model.Workflow{ID: 42, PipelineID: 8, AgentID: 2}
	store.EXPECT().WorkflowLoad(int64(42)).Return(workflow, nil).Once()
	store.EXPECT().AgentFind(int64(1)).Return(&model.Agent{ID: 1}, nil).Once()
	store.EXPECT().GetPipeline(int64(8)).Return(&model.Pipeline{ID: 8, RepoID: 7}, nil).Once()
	store.EXPECT().GetRepo(int64(7)).Return(&model.Repo{ID: 7}, nil).Once()
	store.EXPECT().TaskGetAgentID("42", int64(1)).Return(false, nil).Once()
	store.EXPECT().TaskGetCancelled("42").Return(false, true, nil).Once()
	s := &RPC{store: store}

	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("agent_id", "1"))
	err := s.Init(ctx, "42", rpc.WorkflowState{})
	require.ErrorIs(t, err, errAgentNotAssigned)
	assert.Equal(t, int64(2), workflow.AgentID, "Init must not reassign the workflow")
}
