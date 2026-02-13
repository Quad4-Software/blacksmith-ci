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

func TestOrgAgentList(t *testing.T) {
	tests := []struct {
		name      string
		orgID     int64
		agents    []*crow.Agent
		agentsErr error
		wantErr   bool
	}{
		{
			name:  "success",
			orgID: 1,
			agents: []*crow.Agent{
				{ID: 1, Name: "org-agent-1", Platform: "linux/amd64", Backend: "docker", Capacity: 2},
				{ID: 2, Name: "org-agent-2", Platform: "linux/arm64", Backend: "kubernetes", Capacity: 4},
			},
		},
		{
			name:      "error",
			orgID:     1,
			agentsErr: errors.New("api error"),
			wantErr:   true,
		},
		{
			name:   "empty list",
			orgID:  1,
			agents: []*crow.Agent{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := crow_mocks.NewMockClient(t)
			mockClient.On("OrgAgentList", tt.orgID, mock.Anything).Return(tt.agents, tt.agentsErr).Maybe()

			command := &cli.Command{
				Name:   "ls",
				Flags:  common.OutputFlags("table"),
				Writer: io.Discard,
				Action: func(_ context.Context, c *cli.Command) error {
					agents, err := mockClient.OrgAgentList(tt.orgID, crow.ListOptions{})
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

func TestOrgAgentAdd(t *testing.T) {
	tests := []struct {
		name      string
		orgID     int64
		agentName string
		agent     *crow.Agent
		createErr error
		wantErr   bool
	}{
		{
			name:      "success",
			orgID:     1,
			agentName: "new-agent",
			agent:     &crow.Agent{ID: 1, Name: "new-agent", Token: "secret-token"},
		},
		{
			name:      "create error",
			orgID:     1,
			agentName: "new-agent",
			createErr: errors.New("create failed"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := crow_mocks.NewMockClient(t)
			mockClient.On("OrgAgentCreate", tt.orgID, mock.AnythingOfType("*crow.Agent")).Return(tt.agent, tt.createErr).Maybe()

			command := &cli.Command{
				Name:   "add",
				Flags:  common.OutputFlags("table"),
				Writer: io.Discard,
				Action: func(_ context.Context, c *cli.Command) error {
					agent := &crow.Agent{Name: tt.agentName}
					agent, err := mockClient.OrgAgentCreate(tt.orgID, agent)
					if err != nil {
						return err
					}
					var buf bytes.Buffer
					return agentOutput(c, []*crow.Agent{agent}, &buf)
				},
			}

			err := command.Run(t.Context(), []string{"add"})
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOrgAgentRemove(t *testing.T) {
	tests := []struct {
		name      string
		orgID     int64
		agentID   string
		deleteErr error
		wantErr   bool
	}{
		{
			name:    "success",
			orgID:   1,
			agentID: "1",
		},
		{
			name:    "invalid agent id",
			orgID:   1,
			agentID: "invalid",
			wantErr: true,
		},
		{
			name:      "delete error",
			orgID:     1,
			agentID:   "1",
			deleteErr: errors.New("delete failed"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := crow_mocks.NewMockClient(t)
			if tt.agentID != "invalid" {
				mockClient.On("OrgAgentDelete", tt.orgID, mock.AnythingOfType("int64")).Return(tt.deleteErr).Maybe()
			}

			command := &cli.Command{
				Name:   "rm",
				Writer: io.Discard,
				Action: func(_ context.Context, _ *cli.Command) error {
					agentID, err := strconv.ParseInt(tt.agentID, 10, 64)
					if err != nil {
						return err
					}
					return mockClient.OrgAgentDelete(tt.orgID, agentID)
				},
			}

			err := command.Run(t.Context(), []string{"rm"})
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOrgAgentOutput(t *testing.T) {
	agents := []*crow.Agent{
		{ID: 1, Name: "org-agent-1", Platform: "linux/amd64", Backend: "docker", Capacity: 2, Version: "1.0.0"},
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
			contains: "org-agent-1",
		},
		{
			name:     "json output",
			output:   "json",
			contains: `"name": "org-agent-1"`,
		},
		{
			name:     "yaml output",
			output:   "yaml",
			contains: "name: org-agent-1",
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

func TestParseTargetArgs(t *testing.T) {
	tests := []struct {
		name        string
		orgIDOrName string
		org         *crow.Org
		orgErr      error
		wantOrgID   int64
		wantErr     bool
	}{
		{
			name:        "org id as number",
			orgIDOrName: "123",
			wantOrgID:   123,
		},
		{
			name:        "org name lookup success",
			orgIDOrName: "my-org",
			org:         &crow.Org{ID: 456, Name: "my-org"},
			wantOrgID:   456,
		},
		{
			name:        "org name lookup error",
			orgIDOrName: "unknown-org",
			orgErr:      errors.New("org not found"),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := crow_mocks.NewMockClient(t)

			// Only set up OrgLookup mock if it's not a numeric ID
			if _, err := strconv.ParseInt(tt.orgIDOrName, 10, 64); err != nil {
				mockClient.On("OrgLookup", tt.orgIDOrName).Return(tt.org, tt.orgErr).Maybe()
			}

			// Try to parse as int first
			if orgID, err := strconv.ParseInt(tt.orgIDOrName, 10, 64); err == nil {
				assert.Equal(t, tt.wantOrgID, orgID)
				return
			}

			// Otherwise do org lookup
			org, err := mockClient.OrgLookup(tt.orgIDOrName)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantOrgID, org.ID)
			}
		})
	}
}
