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
	"html/template"
	"os"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v5/cli/common"
	"codefloe.com/crowci/crow/v5/cli/internal"
	crow "codefloe.com/crowci/crow/v5/crow-go/crow"
)

var secretShowCmd = &cli.Command{
	Name:      "show",
	Usage:     "show secret information",
	ArgsUsage: "[repo-id|repo-full-name]",
	Action:    secretShow,
	Flags: []cli.Flag{
		common.RepoFlag,
		&cli.StringFlag{
			Name:  "name",
			Usage: "secret name",
		},
		&cli.BoolFlag{
			Name:  "show-value",
			Usage: "show abbreviated secret value (requires write access)",
		},
		common.FormatFlag(tmplSecretList, true),
	},
}

func secretShow(ctx context.Context, c *cli.Command) error {
	var (
		secretName = c.String("name")
		showValue  = c.Bool("show-value")
		format     = c.String("format") + "\n"
	)

	if secretName == "" {
		return fmt.Errorf("secret name is missing")
	}

	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}

	repoID, err := parseTargetArgs(client, c)
	if err != nil {
		return err
	}

	var secret *crow.Secret
	if showValue {
		secret, err = client.SecretAbbreviated(repoID, secretName)
	} else {
		secret, err = client.Secret(repoID, secretName)
	}
	if err != nil {
		return err
	}

	tmpl, err := template.New("_").Funcs(secretFuncMap).Parse(format)
	if err != nil {
		return err
	}
	return tmpl.Execute(os.Stdout, secret)
}
