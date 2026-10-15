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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"codefloe.com/crowci/crow/v6/pipeline"
	"codefloe.com/crowci/crow/v6/pipeline/rpc"
	"codefloe.com/crowci/crow/v6/server/model"
	queue_mocks "codefloe.com/crowci/crow/v6/server/queue/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

// A custom label must never satisfy the org or repo scoping of an agent.
func TestFilterReservedCustomLabels(t *testing.T) {
	orgAgent := map[string]string{pipeline.LabelOrgID: "3", pipeline.LabelRepoFullName: "org3/app"}

	tests := []struct {
		name         string
		customLabels map[string]string
		task         map[string]string
		want         bool
	}{
		{
			name:         "custom org-id cannot claim another org",
			customLabels: map[string]string{pipeline.LabelOrgID: "5"},
			task:         map[string]string{pipeline.LabelOrgID: "5"},
		},
		{
			name:         "custom org-id does not break the own org",
			customLabels: map[string]string{pipeline.LabelOrgID: "5"},
			task:         map[string]string{pipeline.LabelOrgID: "3"},
			want:         true,
		},
		{
			name:         "custom repo-full-name cannot claim another repo",
			customLabels: map[string]string{pipeline.LabelRepoFullName: "org5/secret"},
			task:         map[string]string{pipeline.LabelOrgID: "3", pipeline.LabelRepoFullName: "org5/secret"},
		},
		{
			name:         "other reserved custom labels are ignored",
			customLabels: map[string]string{pipeline.LabelBranch: "release"},
			task:         map[string]string{pipeline.LabelOrgID: "3", pipeline.LabelBranch: "main"},
			want:         true,
		},
		{
			name:         "regular custom labels still match",
			customLabels: map[string]string{"tier": "heavy"},
			task:         map[string]string{pipeline.LabelOrgID: "3", "tier": "heavy"},
			want:         true,
		},
		{
			name:         "regular custom labels still reject",
			customLabels: map[string]string{"tier": "light"},
			task:         map[string]string{pipeline.LabelOrgID: "3", "tier": "heavy"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := createFilterFunc(rpc.Filter{Labels: orgAgent}, tt.customLabels)
			got, _, _ := filter(&model.Task{ID: "1", Labels: tt.task})
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRegisterAgentDropsReservedCustomLabels(t *testing.T) {
	db := store_mocks.NewMockStore(t)
	db.On("AgentFind", int64(1337)).Return(&model.Agent{ID: 1337, Name: "agent", OrgIDs: model.JSONSliceInt64{3}}, nil)
	db.On("AgentUpdate", mock.MatchedBy(func(a *model.Agent) bool {
		return assert.ObjectsAreEqual(model.JSONMapStringString{"tier": "heavy"}, a.CustomLabels)
	})).Return(nil).Once()

	s := RPC{store: db, metrics: newTestGRPCMetrics()}
	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("hostname", "host", "agent_id", "1337"))
	_, _, err := s.RegisterAgent(ctx, rpc.AgentInfo{
		Backend: "docker",
		CustomLabels: map[string]string{
			"tier":                     "heavy",
			pipeline.LabelOrgID:        "5",
			pipeline.LabelRepoFullName: "org5/secret",
		},
	})
	require.NoError(t, err)
}

// Next must not hand an org-scoped agent a workflow of a repo outside its orgs,
// even if the queue filter let the task through.
func TestNextWithholdsOutOfScopeWorkflow(t *testing.T) {
	setQuotaChecker(t, nil)

	tests := []struct {
		name     string
		agent    *model.Agent
		repoOrg  int64
		lookups  bool
		wantDeny bool
	}{
		{name: "org agent, repo in its org", agent: &model.Agent{ID: 1, OrgIDs: model.JSONSliceInt64{3}}, repoOrg: 3, lookups: true},
		{name: "org agent, repo of another org", agent: &model.Agent{ID: 1, OrgIDs: model.JSONSliceInt64{3}}, repoOrg: 5, lookups: true, wantDeny: true},
		{name: "global agent skips the lookup", agent: &model.Agent{ID: 1}, repoOrg: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := store_mocks.NewMockStore(t)
			q := queue_mocks.NewMockQueue(t)
			db.On("AgentFind", int64(1)).Return(tt.agent, nil)
			if tt.lookups {
				db.On("WorkflowLoad", int64(42)).Return(&model.Workflow{ID: 42, PipelineID: 8}, nil)
				db.On("GetPipeline", int64(8)).Return(&model.Pipeline{ID: 8, RepoID: 7}, nil)
				db.On("GetRepo", int64(7)).Return(&model.Repo{ID: 7, OrgID: tt.repoOrg, FullName: "org/repo"}, nil)
			}
			data, err := json.Marshal(&rpc.Workflow{ID: "42"})
			require.NoError(t, err)
			q.On("Poll", mock.Anything, int64(1), mock.Anything).
				Return(&model.Task{ID: "42", RepoID: 7, Data: data, RunOn: []string{"success"}}, nil).Once()

			s := RPC{store: db, queue: q}
			ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("agent_id", "1"))
			wf, err := s.Next(ctx, rpc.Filter{Labels: map[string]string{}})
			if tt.wantDeny {
				assert.Nil(t, wf)
				assert.Equal(t, codes.PermissionDenied, status.Code(err))
				return
			}
			require.NoError(t, err)
			require.NotNil(t, wf)
			assert.Equal(t, "42", wf.ID)
		})
	}
}
