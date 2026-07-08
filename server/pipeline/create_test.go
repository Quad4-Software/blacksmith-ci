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

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
)

func TestAgentCanMatch(t *testing.T) {
	tests := []struct {
		name           string
		agent          *model.Agent
		requiredLabels map[string]string
		want           bool
	}{
		{
			name: "hostname exact match",
			agent: &model.Agent{
				Hostname: "foo",
			},
			requiredLabels: map[string]string{
				"hostname": "foo",
			},
			want: true,
		},
		{
			name: "hostname mismatch",
			agent: &model.Agent{
				Hostname: "foo",
			},
			requiredLabels: map[string]string{
				"hostname": "other-host",
			},
			want: false,
		},
		{
			name: "agent has no hostname set but task requires one",
			agent: &model.Agent{
				Hostname: "",
			},
			requiredLabels: map[string]string{
				"hostname": "foo",
			},
			want: true, // Empty hostname on agent means it can match any
		},
		{
			name: "no hostname requirement",
			agent: &model.Agent{
				Hostname: "foo",
			},
			requiredLabels: map[string]string{},
			want:           true,
		},
		{
			name: "platform exact match",
			agent: &model.Agent{
				Platform: "linux/amd64",
			},
			requiredLabels: map[string]string{
				"platform": "linux/amd64",
			},
			want: true,
		},
		{
			name: "platform mismatch",
			agent: &model.Agent{
				Platform: "linux/amd64",
			},
			requiredLabels: map[string]string{
				"platform": "linux/arm64",
			},
			want: false,
		},
		{
			name: "backend exact match",
			agent: &model.Agent{
				Backend: "docker",
			},
			requiredLabels: map[string]string{
				"backend": "docker",
			},
			want: true,
		},
		{
			name: "backend mismatch",
			agent: &model.Agent{
				Backend: "docker",
			},
			requiredLabels: map[string]string{
				"backend": "kubernetes",
			},
			want: false,
		},
		{
			name: "agent name exact match",
			agent: &model.Agent{
				Name: "build-agent",
			},
			requiredLabels: map[string]string{
				"agent": "build-agent",
			},
			want: true,
		},
		{
			name: "agent name mismatch",
			agent: &model.Agent{
				Name: "build-agent",
			},
			requiredLabels: map[string]string{
				"agent": "deploy-agent",
			},
			want: false,
		},
		{
			name: "hostname and agent both match",
			agent: &model.Agent{
				Name:     "build-agent",
				Hostname: "foo",
			},
			requiredLabels: map[string]string{
				"agent":    "build-agent",
				"hostname": "foo",
			},
			want: true,
		},
		{
			name: "hostname matches but agent name mismatches",
			agent: &model.Agent{
				Name:     "build-agent",
				Hostname: "foo",
			},
			requiredLabels: map[string]string{
				"agent":    "deploy-agent",
				"hostname": "foo",
			},
			want: false,
		},
		{
			name: "custom label match",
			agent: &model.Agent{
				CustomLabels: map[string]string{
					"team": "backend",
				},
			},
			requiredLabels: map[string]string{
				"team": "backend",
			},
			want: true,
		},
		{
			name: "custom label mismatch",
			agent: &model.Agent{
				CustomLabels: map[string]string{
					"team": "backend",
				},
			},
			requiredLabels: map[string]string{
				"team": "frontend",
			},
			want: false,
		},
		{
			name: "missing custom label",
			agent: &model.Agent{
				CustomLabels: map[string]string{},
			},
			requiredLabels: map[string]string{
				"team": "backend",
			},
			want: false,
		},
		{
			name: "all labels match - comprehensive test",
			agent: &model.Agent{
				Platform: "linux/amd64",
				Backend:  "docker",
				Name:     "build-agent",
				Hostname: "foo",
				CustomLabels: map[string]string{
					"team": "backend",
				},
			},
			requiredLabels: map[string]string{
				"platform": "linux/amd64",
				"backend":  "docker",
				"agent":    "build-agent",
				"hostname": "foo",
				"team":     "backend",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := agentCanMatch(tt.agent, tt.requiredLabels)
			assert.Equal(t, tt.want, got)
		})
	}
}
