// Copyright 2024 Woodpecker Authors
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

package registry

import (
	"context"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v5/cli/common"
	"codefloe.com/crowci/crow/v5/cli/internal"
	crow "codefloe.com/crowci/crow/v5/crow-go/crow"
)

var registryShowCmd = &cli.Command{
	Name:      "show",
	Usage:     "show registry information",
	ArgsUsage: "[org-id|org-name]",
	Action:    registryShow,
	Flags: append(common.OutputFlags("table"),
		common.OrgFlag,
		&cli.StringFlag{
			Name:  "hostname",
			Usage: "registry hostname",
			Value: "docker.io",
		},
	),
}

func registryShow(ctx context.Context, c *cli.Command) error {
	hostname := c.String("hostname")

	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}

	orgID, err := parseTargetArgs(client, c)
	if err != nil {
		return err
	}

	registry, err := client.OrgRegistry(orgID, hostname)
	if err != nil {
		return err
	}

	return registryOutput(c, []*crow.Registry{registry})
}
