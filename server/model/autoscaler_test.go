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

package model

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/pipeline"
)

// realTaskLabels returns the label set a task actually carries in production:
// the reserved crowci/* infrastructure labels stamped on every task plus the
// runtime labels (e.g. platform) that agents are matched against.
func realTaskLabels(extra map[string]string) map[string]string {
	labels := map[string]string{
		pipeline.LabelForgeRemoteID: "gitea",
		pipeline.LabelRepoForgeID:   "42",
		pipeline.LabelRepoID:        "5",
		pipeline.LabelRepoName:      "myrepo",
		pipeline.LabelRepoFullName:  "myorg/myrepo",
		pipeline.LabelBranch:        "main",
		pipeline.LabelOrgID:         "7",
		pipeline.LabelPlatform:      "linux/amd64",
	}
	maps.Copy(labels, extra)
	return labels
}

func TestAutoscalerCanProvision(t *testing.T) {
	tests := []struct {
		name       string
		autoscaler Autoscaler
		labels     map[string]string
		want       bool
	}{
		{
			name: "real task labels with matching platform",
			autoscaler: Autoscaler{
				Enabled:      true,
				MaxAgents:    2,
				ActiveAgents: 0,
				OrgID:        IDNotSet,
				AgentLabels:  JSONMapStringString{"platform": "linux/amd64"},
			},
			labels: realTaskLabels(nil),
			want:   true,
		},
		{
			name: "platform mismatch",
			autoscaler: Autoscaler{
				Enabled:     true,
				MaxAgents:   2,
				OrgID:       IDNotSet,
				AgentLabels: JSONMapStringString{"platform": "linux/arm64"},
			},
			labels: realTaskLabels(nil),
			want:   false,
		},
		{
			name: "wildcard platform matches",
			autoscaler: Autoscaler{
				Enabled:     true,
				MaxAgents:   2,
				OrgID:       IDNotSet,
				AgentLabels: JSONMapStringString{"platform": "*"},
			},
			labels: realTaskLabels(nil),
			want:   true,
		},
		{
			name: "disabled autoscaler never provisions",
			autoscaler: Autoscaler{
				Enabled:     false,
				MaxAgents:   2,
				OrgID:       IDNotSet,
				AgentLabels: JSONMapStringString{"platform": "linux/amd64"},
			},
			labels: realTaskLabels(nil),
			want:   false,
		},
		{
			name: "at capacity never provisions",
			autoscaler: Autoscaler{
				Enabled:      true,
				MaxAgents:    2,
				ActiveAgents: 2,
				OrgID:        IDNotSet,
				AgentLabels:  JSONMapStringString{"platform": "linux/amd64"},
			},
			labels: realTaskLabels(nil),
			want:   false,
		},
		{
			name: "org-scoped autoscaler matches its org",
			autoscaler: Autoscaler{
				Enabled:     true,
				MaxAgents:   2,
				OrgID:       7,
				AgentLabels: JSONMapStringString{"platform": "linux/amd64"},
			},
			labels: realTaskLabels(nil),
			want:   true,
		},
		{
			name: "org-scoped autoscaler rejects other org",
			autoscaler: Autoscaler{
				Enabled:     true,
				MaxAgents:   2,
				OrgID:       99,
				AgentLabels: JSONMapStringString{"platform": "linux/amd64"},
			},
			labels: realTaskLabels(nil),
			want:   false,
		},
		{
			name: "empty-valued task label is ignored",
			autoscaler: Autoscaler{
				Enabled:     true,
				MaxAgents:   2,
				OrgID:       IDNotSet,
				AgentLabels: JSONMapStringString{"platform": "linux/amd64"},
			},
			labels: realTaskLabels(map[string]string{"custom": ""}),
			want:   true,
		},
		{
			name: "required custom label matches",
			autoscaler: Autoscaler{
				Enabled:     true,
				MaxAgents:   2,
				OrgID:       IDNotSet,
				AgentLabels: JSONMapStringString{"platform": "linux/amd64", "gpu": "true"},
			},
			labels: realTaskLabels(map[string]string{"gpu": "true"}),
			want:   true,
		},
		{
			name: "required custom label missing on autoscaler",
			autoscaler: Autoscaler{
				Enabled:     true,
				MaxAgents:   2,
				OrgID:       IDNotSet,
				AgentLabels: JSONMapStringString{"platform": "linux/amd64"},
			},
			labels: realTaskLabels(map[string]string{"gpu": "true"}),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.autoscaler.CanProvision(tt.labels))
		})
	}
}
