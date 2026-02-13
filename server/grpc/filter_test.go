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

	"codefloe.com/crowci/crow/v5/pipeline/rpc"
	"codefloe.com/crowci/crow/v5/server/model"
)

func TestCreateFilterFunc(t *testing.T) {
	tests := []struct {
		name         string
		agentFilter  rpc.Filter
		customLabels map[string]string
		task         *model.Task
		wantMatched  bool
		wantScore    int
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
		// Custom label tests - Updated for partial matching
		{
			name: "Agent with custom label - task has matching label",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"repo": "*", "platform": "linux"},
			},
			customLabels: map[string]string{"scope": "lint"},
			task: &model.Task{
				Labels: map[string]string{"repo": "test", "platform": "linux", "scope": "lint"},
			},
			wantMatched: true,
			wantScore:   21, // 10 for custom label match + 1 for repo wildcard + 10 for platform exact
		},
		{
			name: "Agent with custom label - task doesn't require it",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"repo": "*", "platform": "linux"},
			},
			customLabels: map[string]string{"scope": "lint"},
			task: &model.Task{
				Labels: map[string]string{"repo": "test", "platform": "linux"},
			},
			wantMatched: true, // Changed: task doesn't require the custom label, so it matches
			wantScore:   11,   // 1 for repo wildcard + 10 for platform exact
		},
		{
			name: "Agent with custom label - task requires different value",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"repo": "*", "platform": "linux"},
			},
			customLabels: map[string]string{"scope": "lint"},
			task: &model.Task{
				Labels: map[string]string{"repo": "test", "platform": "linux", "scope": "build"},
			},
			wantMatched: false, // Task requires scope=build but agent has scope=lint
			wantScore:   0,
		},
		{
			name: "Agent with multiple custom labels - task requires both",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"repo": "*"},
			},
			customLabels: map[string]string{"scope": "lint", "environment": "production"},
			task: &model.Task{
				Labels: map[string]string{"repo": "test", "scope": "lint", "environment": "production"},
			},
			wantMatched: true,
			wantScore:   21, // 10 for scope match + 10 for environment match + 1 for repo wildcard
		},
		{
			name: "Agent with multiple custom labels - task requires only one",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"repo": "*"},
			},
			customLabels: map[string]string{"scope": "lint", "environment": "production"},
			task: &model.Task{
				Labels: map[string]string{"repo": "test", "scope": "lint"},
			},
			wantMatched: true, // Changed: partial match is sufficient
			wantScore:   11,   // 10 for scope match + 1 for repo wildcard
		},
		{
			name: "Agent without custom label - task requires it",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"repo": "*", "platform": "linux"},
			},
			customLabels: map[string]string{}, // Agent has no custom labels
			task: &model.Task{
				Labels: map[string]string{"repo": "test", "platform": "linux", "scope": "lint"},
			},
			wantMatched: false, // Task requires scope=lint but agent doesn't have it
			wantScore:   0,
		},
		// Backend label tests
		{
			name: "Backend label exact match - kubernetes",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"backend": "kubernetes", "repo": "*"},
			},
			customLabels: map[string]string{},
			task: &model.Task{
				Labels: map[string]string{"backend": "kubernetes", "repo": "test"},
			},
			wantMatched: true,
			wantScore:   11,
		},
		{
			name: "Backend label mismatch - task wants kubernetes, agent has docker",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"backend": "docker", "repo": "*"},
			},
			customLabels: map[string]string{},
			task: &model.Task{
				Labels: map[string]string{"backend": "kubernetes", "repo": "test"},
			},
			wantMatched: false,
			wantScore:   0,
		},
		{
			name: "Backend label - task has no backend requirement",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"backend": "kubernetes", "repo": "*"},
			},
			customLabels: map[string]string{},
			task: &model.Task{
				Labels: map[string]string{"repo": "test"},
			},
			wantMatched: true,
			wantScore:   1,
		},
		{
			name: "Global agent with kubernetes backend - task specifies kubernetes",
			agentFilter: rpc.Filter{
				Labels: map[string]string{
					"backend":  "kubernetes",
					"repo":     "*",
					"platform": "linux/amd64",
					"hostname": "test-host",
					"org-id":   "*", // Global agent
				},
			},
			customLabels: nil, // No custom labels (exactly like DB)
			task: &model.Task{
				Labels: map[string]string{
					"backend": "kubernetes",
					"repo":    "myrepo",
					"org-id":  "123",
				},
			},
			wantMatched: true,
			wantScore:   12,
		},
		{
			name: "Agent with nil custom labels - standard workflow",
			agentFilter: rpc.Filter{
				Labels: map[string]string{
					"backend": "kubernetes",
					"repo":    "*",
					"org-id":  "*",
				},
			},
			customLabels: nil, // Exactly like your DB shows
			task: &model.Task{
				Labels: map[string]string{
					"repo":   "test-repo",
					"org-id": "456",
				},
			},
			wantMatched: true,
			wantScore:   2,
		},
		// Agent label filtering tests
		{
			name: "Agent label exact match",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"agent": "build-agent-1", "repo": "*"},
			},
			customLabels: map[string]string{},
			task: &model.Task{
				Labels: map[string]string{"agent": "build-agent-1", "repo": "test"},
			},
			wantMatched: true,
			wantScore:   11, // 10 for agent exact match + 1 for repo wildcard
		},
		{
			name: "Agent label mismatch",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"agent": "build-agent-1", "repo": "*"},
			},
			customLabels: map[string]string{},
			task: &model.Task{
				Labels: map[string]string{"agent": "build-agent-2", "repo": "test"},
			},
			wantMatched: false,
			wantScore:   0,
		},
		{
			name: "Agent label - task has no agent requirement",
			agentFilter: rpc.Filter{
				Labels: map[string]string{"agent": "build-agent-1", "repo": "*"},
			},
			customLabels: map[string]string{},
			task: &model.Task{
				Labels: map[string]string{"repo": "test"},
			},
			wantMatched: true,
			wantScore:   1, // Only repo wildcard score
		},
		{
			name: "Agent label with platform and backend - all match",
			agentFilter: rpc.Filter{
				Labels: map[string]string{
					"agent":    "build-agent-1",
					"platform": "linux",
					"backend":  "docker",
					"repo":     "*",
				},
			},
			customLabels: map[string]string{},
			task: &model.Task{
				Labels: map[string]string{
					"agent":    "build-agent-1",
					"platform": "linux",
					"backend":  "docker",
					"repo":     "test",
				},
			},
			wantMatched: true,
			wantScore:   31, // 10 (agent) + 10 (platform) + 10 (backend) + 1 (repo wildcard)
		},
		{
			name: "Agent label with platform and backend - agent mismatch",
			agentFilter: rpc.Filter{
				Labels: map[string]string{
					"agent":    "build-agent-1",
					"platform": "linux",
					"backend":  "docker",
					"repo":     "*",
				},
			},
			customLabels: map[string]string{},
			task: &model.Task{
				Labels: map[string]string{
					"agent":    "build-agent-2",
					"platform": "linux",
					"backend":  "docker",
					"repo":     "test",
				},
			},
			wantMatched: false,
			wantScore:   0,
		},
		{
			name: "Agent label and custom labels - both match",
			agentFilter: rpc.Filter{
				Labels: map[string]string{
					"agent": "build-agent-1",
					"repo":  "*",
				},
			},
			customLabels: map[string]string{"scope": "lint"},
			task: &model.Task{
				Labels: map[string]string{
					"agent": "build-agent-1",
					"repo":  "test",
					"scope": "lint",
				},
			},
			wantMatched: true,
			wantScore:   21, // 10 (agent) + 10 (scope custom) + 1 (repo wildcard)
		},
		{
			name: "Agent label match but custom label mismatch",
			agentFilter: rpc.Filter{
				Labels: map[string]string{
					"agent": "build-agent-1",
					"repo":  "*",
				},
			},
			customLabels: map[string]string{"scope": "lint"},
			task: &model.Task{
				Labels: map[string]string{
					"agent": "build-agent-1",
					"repo":  "test",
					"scope": "build",
				},
			},
			wantMatched: false, // Custom label mismatch
			wantScore:   0,
		},
		// Hostname label filtering tests (backward compatibility with old Woodpecker syntax)
		{
			name: "Hostname label exact match - task targets specific agent by hostname",
			agentFilter: rpc.Filter{
				Labels: map[string]string{
					"hostname": "foo",
					"repo":     "*",
					"platform": "linux",
				},
			},
			customLabels: map[string]string{},
			task: &model.Task{
				Labels: map[string]string{
					"hostname": "foo",
					"repo":     "test",
				},
			},
			wantMatched: true,
			wantScore:   11, // 10 for hostname exact match + 1 for repo wildcard
		},
		{
			name: "Hostname label mismatch - task targets different hostname",
			agentFilter: rpc.Filter{
				Labels: map[string]string{
					"hostname": "foo",
					"repo":     "*",
					"platform": "linux",
				},
			},
			customLabels: map[string]string{},
			task: &model.Task{
				Labels: map[string]string{
					"hostname": "other-host",
					"repo":     "test",
				},
			},
			wantMatched: false,
			wantScore:   0,
		},
		{
			name: "Hostname label - task has no hostname requirement (runs on any agent)",
			agentFilter: rpc.Filter{
				Labels: map[string]string{
					"hostname": "foo",
					"repo":     "*",
					"platform": "linux",
				},
			},
			customLabels: map[string]string{},
			task: &model.Task{
				Labels: map[string]string{
					"repo": "test",
				},
			},
			wantMatched: true,
			wantScore:   1, // Only repo wildcard score
		},
		{
			name: "Both hostname and agent labels - all match",
			agentFilter: rpc.Filter{
				Labels: map[string]string{
					"hostname": "foo",
					"agent":    "build-agent",
					"repo":     "*",
				},
			},
			customLabels: map[string]string{},
			task: &model.Task{
				Labels: map[string]string{
					"hostname": "foo",
					"agent":    "build-agent",
					"repo":     "test",
				},
			},
			wantMatched: true,
			wantScore:   21, // 10 (hostname) + 10 (agent) + 1 (repo wildcard)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filterFunc := createFilterFunc(tt.agentFilter, tt.customLabels)
			gotMatched, gotScore, _ := filterFunc(tt.task)

			assert.Equal(t, tt.wantMatched, gotMatched, "Matched result")
			assert.Equal(t, tt.wantScore, gotScore, "Score")
		})
	}
}
