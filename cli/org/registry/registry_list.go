// Copyright 2024 Woodpecker Authors
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

package registry

import (
	"context"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/common"
	"codefloe.com/crowci/crow/v6/cli/internal"
	crow "codefloe.com/crowci/crow/v6/crow-go/crow"
)

var registryListCmd = &cli.Command{
	Name:      "ls",
	Usage:     "list registries",
	ArgsUsage: "[org-id|org-name]",
	Action:    registryList,
	Flags:     append(common.OutputFlags("table"), common.OrgFlag),
}

func registryList(ctx context.Context, c *cli.Command) error {
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}

	orgID, err := parseTargetArgs(client, c)
	if err != nil {
		return err
	}

	opt := crow.RegistryListOptions{}

	list, err := client.OrgRegistryList(orgID, opt)
	if err != nil {
		return err
	}

	return registryOutput(c, list)
}
