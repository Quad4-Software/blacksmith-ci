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

package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/crowci/crow/v3/pipeline/rpc"
	"codeberg.org/crowci/crow/v3/server/model"
)

func TestCreateFilterFunc(t *testing.T) {
	tests := []struct {
		name        string
		agentFilter rpc.Filter
		task        *model.Task
		wantMatched bool
		wantScore   int
	}{
		{
			name: "Two exact matches",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"org-id": "123", "platform": "linux"},
			},
			task: &model.Task{
				Labels: map[string]string{"org-id": "123", "platform": "linux"},
			},
			wantMatched: true,
			wantScore:   20,
		},
		{
			name: "Wildcard and exact match",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"org-id": "*", "platform": "linux"},
			},
			task: &model.Task{
				Labels: map[string]string{"org-id": "123", "platform": "linux"},
			},
			wantMatched: true,
			wantScore:   11,
		},
		{
			name: "Partial match",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"org-id": "123", "platform": "linux"},
			},
			task: &model.Task{
				Labels: map[string]string{"org-id": "123", "platform": "windows"},
			},
			wantMatched: false,
			wantScore:   0,
		},
		{
			name: "No match",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"org-id": "456", "platform": "linux"},
			},
			task: &model.Task{
				Labels: map[string]string{"org-id": "123", "platform": "windows"},
			},
			wantMatched: false,
			wantScore:   0,
		},
		{
			name: "Missing label",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"platform": "linux"},
			},
			task: &model.Task{
				Labels: map[string]string{"needed": "some"},
			},
			wantMatched: false,
			wantScore:   0,
		},
		{
			name: "Empty task labels",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"org-id": "123", "platform": "linux"},
			},
			task: &model.Task{
				Labels: map[string]string{},
			},
			wantMatched: true,
			wantScore:   0,
		},
		{
			name: "Agent with additional label",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"org-id": "123", "platform": "linux", "extra": "value"},
			},
			task: &model.Task{
				Labels: map[string]string{"org-id": "123", "platform": "linux", "empty": ""},
			},
			wantMatched: true,
			wantScore:   20,
		},
		{
			name: "Two wildcard matches",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"org-id": "*", "platform": "*"},
			},
			task: &model.Task{
				Labels: map[string]string{"org-id": "123", "platform": "linux"},
			},
			wantMatched: true,
			wantScore:   2,
		},
		// New test cases for pipe-separated org and repo labels
		{
			name: "Org pipe-separated match - first value",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"org-id": "123|456|789", "platform": "linux"},
			},
			task: &model.Task{
				Labels: map[string]string{"org-id": "123", "platform": "linux"},
			},
			wantMatched: true,
			wantScore:   20,
		},
		{
			name: "Org pipe-separated match - middle value",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"org-id": "123|456|789", "platform": "linux"},
			},
			task: &model.Task{
				Labels: map[string]string{"org-id": "456", "platform": "linux"},
			},
			wantMatched: true,
			wantScore:   20,
		},
		{
			name: "Org pipe-separated match - last value",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"org-id": "123|456|789", "platform": "linux"},
			},
			task: &model.Task{
				Labels: map[string]string{"org-id": "789", "platform": "linux"},
			},
			wantMatched: true,
			wantScore:   20,
		},
		{
			name: "Org pipe-separated no match",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"org-id": "123|456|789", "platform": "linux"},
			},
			task: &model.Task{
				Labels: map[string]string{"org-id": "999", "platform": "linux"},
			},
			wantMatched: false,
			wantScore:   0,
		},
		{
			name: "Repo pipe-separated match",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"repo": "test|demo|prod", "platform": "linux"},
			},
			task: &model.Task{
				Labels: map[string]string{"repo": "demo", "platform": "linux"},
			},
			wantMatched: true,
			wantScore:   20,
		},
		{
			name: "Repo pipe-separated no match",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"repo": "test|demo|prod", "platform": "linux"},
			},
			task: &model.Task{
				Labels: map[string]string{"repo": "other", "platform": "linux"},
			},
			wantMatched: false,
			wantScore:   0,
		},
		{
			name: "Both org and repo pipe-separated matches",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"org-id": "123|456", "repo": "test|demo", "platform": "linux"},
			},
			task: &model.Task{
				Labels: map[string]string{"org-id": "456", "repo": "demo", "platform": "linux"},
			},
			wantMatched: true,
			wantScore:   30,
		},
		{
			name: "Org wildcard with repo pipe-separated match",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"org-id": "*", "repo": "test|demo", "platform": "linux"},
			},
			task: &model.Task{
				Labels: map[string]string{"org-id": "any", "repo": "demo", "platform": "linux"},
			},
			wantMatched: true,
			wantScore:   21,
		},
		{
			name: "Repo wildcard with org pipe-separated match",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"org-id": "123|456", "repo": "*", "platform": "linux"},
			},
			task: &model.Task{
				Labels: map[string]string{"org-id": "456", "repo": "any", "platform": "linux"},
			},
			wantMatched: true,
			wantScore:   21,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filterFunc := createFilterFunc(tt.agentFilter)
			gotMatched, gotScore := filterFunc(tt.task)

			assert.Equal(t, tt.wantMatched, gotMatched, "Matched result")
			assert.Equal(t, tt.wantScore, gotScore, "Score")
		})
	}
}
