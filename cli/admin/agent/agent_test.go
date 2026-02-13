// Copyright 2025 Crow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package agent

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v5/cli/common"
	"codefloe.com/crowci/crow/v5/crow-go/crow"
	crow_mocks "codefloe.com/crowci/crow/v5/crow-go/crow/mocks"
)

func TestAgentList(t *testing.T) {
	tests := []struct {
		name      string
		agents    []*crow.Agent
		agentsErr error
		wantErr   bool
	}{
		{
			name: "success",
			agents: []*crow.Agent{
				{ID: 1, Name: "agent-1", Platform: "linux/amd64", Backend: "docker", Capacity: 2},
				{ID: 2, Name: "agent-2", Platform: "linux/arm64", Backend: "kubernetes", Capacity: 4},
			},
		},
		{
			name:      "error",
			agentsErr: errors.New("api error"),
			wantErr:   true,
		},
		{
			name:   "empty list",
			agents: []*crow.Agent{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := crow_mocks.NewMockClient(t)
			mockClient.On("AgentList").Return(tt.agents, tt.agentsErr).Maybe()

			command := &cli.Command{
				Name:   "ls",
				Flags:  common.OutputFlags("table"),
				Writer: io.Discard,
				Action: func(_ context.Context, c *cli.Command) error {
					agents, err := mockClient.AgentList()
					if err != nil {
						return err
					}
					var buf bytes.Buffer
					return agentOutput(c, agents, &buf)
				},
			}

			err := command.Run(t.Context(), []string{"ls"})
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAgentShow(t *testing.T) {
	tests := []struct {
		name     string
		agentID  string
		agent    *crow.Agent
		agentErr error
		wantErr  bool
	}{
		{
			name:    "success",
			agentID: "1",
			agent:   &crow.Agent{ID: 1, Name: "test-agent", Platform: "linux/amd64"},
		},
		{
			name:    "invalid agent id",
			agentID: "invalid",
			wantErr: true,
		},
		{
			name:     "agent not found",
			agentID:  "999",
			agentErr: errors.New("not found"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := crow_mocks.NewMockClient(t)
			if tt.agentID != "invalid" {
				mockClient.On("Agent", mock.AnythingOfType("int64")).Return(tt.agent, tt.agentErr).Maybe()
			}

			command := &cli.Command{
				Name:   "show",
				Flags:  common.OutputFlags("table"),
				Writer: io.Discard,
				Action: func(_ context.Context, c *cli.Command) error {
					agentID, err := strconv.ParseInt(tt.agentID, 10, 64)
					if err != nil {
						return err
					}
					agent, err := mockClient.Agent(agentID)
					if err != nil {
						return err
					}
					var buf bytes.Buffer
					return agentOutput(c, []*crow.Agent{agent}, &buf)
				},
			}

			err := command.Run(t.Context(), []string{"show", tt.agentID})
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAgentRemove(t *testing.T) {
	tests := []struct {
		name      string
		agentID   string
		deleteErr error
		wantErr   bool
	}{
		{
			name:    "success",
			agentID: "1",
		},
		{
			name:    "invalid agent id",
			agentID: "invalid",
			wantErr: true,
		},
		{
			name:      "delete error",
			agentID:   "1",
			deleteErr: errors.New("delete failed"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := crow_mocks.NewMockClient(t)
			if tt.agentID != "invalid" {
				mockClient.On("AgentDelete", mock.AnythingOfType("int64")).Return(tt.deleteErr).Maybe()
			}

			command := &cli.Command{
				Name:   "rm",
				Writer: io.Discard,
				Action: func(_ context.Context, _ *cli.Command) error {
					agentID, err := strconv.ParseInt(tt.agentID, 10, 64)
					if err != nil {
						return err
					}
					return mockClient.AgentDelete(agentID)
				},
			}

			err := command.Run(t.Context(), []string{"rm", tt.agentID})
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAgentOutput(t *testing.T) {
	agents := []*crow.Agent{
		{ID: 1, Name: "agent-1", Platform: "linux/amd64", Backend: "docker", Capacity: 2, Version: "1.0.0"},
	}

	tests := []struct {
		name     string
		output   string
		contains string
		wantErr  bool
	}{
		{
			name:     "table output",
			output:   "table",
			contains: "agent-1",
		},
		{
			name:     "json output",
			output:   "json",
			contains: `"name": "agent-1"`,
		},
		{
			name:     "yaml output",
			output:   "yaml",
			contains: "name: agent-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := &cli.Command{
				Name:  "test",
				Flags: common.OutputFlags("table"),
				Action: func(_ context.Context, c *cli.Command) error {
					var buf bytes.Buffer
					err := agentOutput(c, agents, &buf)
					if err != nil {
						return err
					}
					assert.Contains(t, buf.String(), tt.contains)
					return nil
				},
			}

			args := []string{"test"}
			if tt.output != "table" {
				args = append(args, "-o", tt.output)
			}

			err := command.Run(t.Context(), args)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
