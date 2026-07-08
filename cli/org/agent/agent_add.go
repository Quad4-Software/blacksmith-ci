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
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/common"
	"codefloe.com/crowci/crow/v6/cli/internal"
	crow "codefloe.com/crowci/crow/v6/crow-go/crow"
)

var agentAddCmd = &cli.Command{
	Name:      "add",
	Usage:     "add an organization agent",
	ArgsUsage: "[org-id|org-name]",
	Action:    agentAdd,
	Flags: append(common.OutputFlags("table"),
		common.OrgFlag,
		&cli.StringFlag{
			Name:     "name",
			Usage:    "agent name",
			Required: true,
		},
		&cli.BoolFlag{
			Name:  "no-schedule",
			Usage: "disable scheduling for this agent",
		},
	),
}

func agentAdd(ctx context.Context, c *cli.Command) error {
	name := c.String("name")
	if name == "" {
		return fmt.Errorf("agent name is required")
	}

	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}

	orgID, err := parseTargetArgs(client, c)
	if err != nil {
		return err
	}

	agent := &crow.Agent{
		Name:       name,
		NoSchedule: c.Bool("no-schedule"),
	}

	agent, err = client.OrgAgentCreate(orgID, agent)
	if err != nil {
		return err
	}

	fmt.Printf("Agent created with ID %d and token: %s\n", agent.ID, agent.Token)
	return agentOutput(c, []*crow.Agent{agent})
}
