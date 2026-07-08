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

	"codefloe.com/crowci/crow/v6/cli/common"
	"codefloe.com/crowci/crow/v6/cli/internal"
)

var agentRemoveCmd = &cli.Command{
	Name:      "rm",
	Usage:     "remove an organization agent",
	ArgsUsage: "[org-id|org-name]",
	Action:    agentRemove,
	Flags: []cli.Flag{
		common.OrgFlag,
		&cli.StringFlag{
			Name:     "agent-id",
			Usage:    "agent id",
			Required: true,
		},
	},
}

func agentRemove(ctx context.Context, c *cli.Command) error {
	agentIDStr := c.String("agent-id")
	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid agent id: %w", err)
	}

	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}

	orgID, err := parseTargetArgs(client, c)
	if err != nil {
		return err
	}

	if err := client.OrgAgentDelete(orgID, agentID); err != nil {
		return err
	}

	fmt.Printf("Successfully removed agent %d\n", agentID)
	return nil
}
