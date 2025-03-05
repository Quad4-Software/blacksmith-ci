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

package repo

import (
	"context"

	"github.com/urfave/cli/v3"

	"codeberg.org/crowci/crow/v3/cli/common"
	"codeberg.org/crowci/crow/v3/cli/internal"
	crow "codeberg.org/crowci/crow/v3/crow-go/crow"
)

var repoListCmd = &cli.Command{
	Name:      "ls",
	Usage:     "list all repos",
	ArgsUsage: " ",
	Action:    List,
	Flags: append(common.OutputFlags("table"), []cli.Flag{
		common.FormatFlag("", true),
		&cli.StringFlag{
			Name:  "org",
			Usage: "filter by organization",
		},
		&cli.BoolFlag{
			Name:  "all",
			Usage: "query all repos, including inactive ones",
		},
	}...),
}

func List(ctx context.Context, c *cli.Command) error {
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}
	repos, err := repoList(c, client)
	if err != nil {
		return err
	}
	return repoOutput(c, repos)
}

func repoList(c *cli.Command, client crow.Client) ([]*crow.Repo, error) {
	repos := make([]*crow.Repo, 0)
	opt := crow.RepoListOptions{
		All: c.Bool("all"),
	}

	raw, err := client.RepoList(opt)
	if err != nil || len(raw) == 0 {
		return nil, err
	}

	org := c.String("org")
	for _, repo := range raw {
		if org != "" && org != repo.Owner {
			continue
		}
		repos = append(repos, repo)
	}
	return repos, nil
}
