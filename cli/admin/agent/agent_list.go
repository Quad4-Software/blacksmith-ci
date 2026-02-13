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

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v5/cli/common"
	"codefloe.com/crowci/crow/v5/cli/internal"
)

var agentListCmd = &cli.Command{
	Name:      "ls",
	Usage:     "list all agents",
	ArgsUsage: " ",
	Action:    agentList,
	Flags:     common.OutputFlags("table"),
}

func agentList(ctx context.Context, c *cli.Command) error {
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}

	agents, err := client.AgentList()
	if err != nil {
		return err
	}

	return agentOutput(c, agents)
}
