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
	"fmt"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/common"
	"codefloe.com/crowci/crow/v6/cli/internal"
	crow "codefloe.com/crowci/crow/v6/crow-go/crow"
)

var secretShowCmd = &cli.Command{
	Name:      "show",
	Usage:     "show secret information",
	ArgsUsage: "[org-id|org-name]",
	Action:    secretShow,
	Flags: append(common.OutputFlags("table"),
		common.OrgFlag,
		&cli.StringFlag{
			Name:  "name",
			Usage: "secret name",
		},
		&cli.BoolFlag{
			Name:  "show-value",
			Usage: "show abbreviated secret value (requires org admin access)",
		},
	),
}

func secretShow(ctx context.Context, c *cli.Command) error {
	secretName := c.String("name")
	showValue := c.Bool("show-value")
	if secretName == "" {
		return fmt.Errorf("secret name is missing")
	}

	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}

	orgID, err := parseTargetArgs(client, c)
	if err != nil {
		return err
	}

	var secret *crow.Secret
	if showValue {
		secret, err = client.OrgSecretAbbreviated(orgID, secretName)
	} else {
		secret, err = client.OrgSecret(orgID, secretName)
	}
	if err != nil {
		return err
	}

	return secretOutput(c, []*crow.Secret{secret})
}
