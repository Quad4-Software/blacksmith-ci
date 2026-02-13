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
	"strconv"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v5/cli/common"
	"codefloe.com/crowci/crow/v5/cli/internal"
	crow "codefloe.com/crowci/crow/v5/crow-go/crow"
)

var agentShowCmd = &cli.Command{
	Name:      "show",
	Usage:     "show agent details",
	ArgsUsage: "[org-id|org-name]",
	Action:    agentShow,
	Flags: append(common.OutputFlags("table"),
		common.OrgFlag,
		&cli.StringFlag{
			Name:     "agent-id",
			Usage:    "agent id",
			Required: true,
		},
	),
}

func agentShow(ctx context.Context, c *cli.Command) error {
	agentIDStr := c.String("agent-id")
	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid agent id: %w", err)
	}

	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}

	// Use global Agent endpoint since there's no org-specific GET endpoint
	agent, err := client.Agent(agentID)
	if err != nil {
		return err
	}

	return agentOutput(c, []*crow.Agent{agent})
}
