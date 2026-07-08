// Copyright 2023 Woodpecker Authors
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

package secret

import (
	"context"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/common"
	"codefloe.com/crowci/crow/v6/cli/internal"
	crow "codefloe.com/crowci/crow/v6/crow-go/crow"
)

var secretListCmd = &cli.Command{
	Name:      "ls",
	Usage:     "list secrets",
	ArgsUsage: "[org-id|org-name]",
	Action:    secretList,
	Flags:     append(common.OutputFlags("table"), common.OrgFlag),
}

func secretList(ctx context.Context, c *cli.Command) error {
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}

	orgID, err := parseTargetArgs(client, c)
	if err != nil {
		return err
	}

	opt := crow.SecretListOptions{}

	list, err := client.OrgSecretList(orgID, opt)
	if err != nil {
		return err
	}

	return secretOutput(c, list)
}
