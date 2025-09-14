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

package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/crowci/crow/v4/pipeline/rpc"
	"codeberg.org/crowci/crow/v4/server/model"
)

func TestFilterWithPriority(t *testing.T) {
	tests := []struct {
		name         string
		agentFilter  rpc.Filter
		customLabels map[string]string
		priority     int32
		task         *model.Task
		wantMatch    bool
		wantScore    int
		wantPriority int32
	}{
		{
			name: "agent with priority 5",
			agentFilter: rpc.Filter{
				Labels: map[string]string{
					"platform": "linux/amd64",
				},
			},
			customLabels: map[string]string{},
			priority:     5,
			task: &model.Task{
				Labels: map[string]string{
					"platform": "linux/amd64",
				},
			},
			wantMatch:    true,
			wantScore:    10, // 10 for exact match
			wantPriority: 5,
		},
		{
			name: "agent with priority 0 (no priority)",
			agentFilter: rpc.Filter{
				Labels: map[string]string{
					"platform": "linux/amd64",
				},
			},
			customLabels: map[string]string{},
			priority:     0,
			task: &model.Task{
				Labels: map[string]string{
					"platform": "linux/amd64",
				},
			},
			wantMatch:    true,
			wantScore:    10, // 10 for exact match
			wantPriority: 0,
		},
		{
			name: "agent with high priority but no match",
			agentFilter: rpc.Filter{
				Labels: map[string]string{
					"platform": "windows/amd64",
				},
			},
			customLabels: map[string]string{},
			priority:     10,
			task: &model.Task{
				Labels: map[string]string{
					"platform": "linux/amd64",
				},
			},
			wantMatch:    false,
			wantScore:    0,
			wantPriority: 10,
		},
		{
			name: "wildcard match with priority",
			agentFilter: rpc.Filter{
				Labels: map[string]string{
					"platform": "*",
				},
			},
			customLabels: map[string]string{},
			priority:     3,
			task: &model.Task{
				Labels: map[string]string{
					"platform": "linux/arm64",
				},
			},
			wantMatch:    true,
			wantScore:    1, // 1 for wildcard match
			wantPriority: 3,
		},
		{
			name: "custom labels with priority",
			agentFilter: rpc.Filter{
				Labels: map[string]string{
					"platform": "linux/amd64",
				},
			},
			customLabels: map[string]string{
				"team": "backend",
			},
			priority: 2,
			task: &model.Task{
				Labels: map[string]string{
					"platform": "linux/amd64",
					"team":     "backend",
				},
			},
			wantMatch:    true,
			wantScore:    20, // 10 for custom label match + 10 for platform exact match
			wantPriority: 2,
		},
		{
			name: "org-id match with priority",
			agentFilter: rpc.Filter{
				Labels: map[string]string{
					"org-id": "123|456|789",
				},
			},
			customLabels: map[string]string{},
			priority:     4,
			task: &model.Task{
				Labels: map[string]string{
					"org-id": "456",
				},
			},
			wantMatch:    true,
			wantScore:    10, // 10 for org-id match
			wantPriority: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filterFn := createFilterFuncWithPriority(tt.agentFilter, tt.customLabels, tt.priority)
			gotMatch, gotScore, gotPriority := filterFn(tt.task)

			assert.Equal(t, tt.wantMatch, gotMatch, "match result mismatch")
			if tt.wantMatch {
				assert.Equal(t, tt.wantScore, gotScore, "score mismatch")
				assert.Equal(t, tt.wantPriority, gotPriority, "priority mismatch")
			}
		})
	}
}

func TestPrioritySelection(t *testing.T) {
	// Test that priority values are returned correctly
	task := &model.Task{
		Labels: map[string]string{
			"platform": "linux/amd64",
		},
	}

	// Create filters for agents with different priorities
	agent0Filter := createFilterFuncWithPriority(
		rpc.Filter{Labels: map[string]string{"platform": "linux/amd64"}},
		map[string]string{},
		0, // No priority
	)

	agent5Filter := createFilterFuncWithPriority(
		rpc.Filter{Labels: map[string]string{"platform": "linux/amd64"}},
		map[string]string{},
		5, // Priority 5
	)

	agent3Filter := createFilterFuncWithPriority(
		rpc.Filter{Labels: map[string]string{"platform": "linux/amd64"}},
		map[string]string{},
		3, // Priority 3
	)

	// Get results for each agent
	match0, score0, priority0 := agent0Filter(task)
	match5, score5, priority5 := agent5Filter(task)
	match3, score3, priority3 := agent3Filter(task)

	// All should match with same label score, but different priorities
	assert.True(t, match0, "Agent 0 should match")
	assert.True(t, match5, "Agent 5 should match")
	assert.True(t, match3, "Agent 3 should match")

	assert.Equal(t, 10, score0, "All agents should have same label score")
	assert.Equal(t, 10, score5, "All agents should have same label score")
	assert.Equal(t, 10, score3, "All agents should have same label score")

	assert.Equal(t, int32(0), priority0, "Agent should return priority 0")
	assert.Equal(t, int32(5), priority5, "Agent should return priority 5")
	assert.Equal(t, int32(3), priority3, "Agent should return priority 3")
}
