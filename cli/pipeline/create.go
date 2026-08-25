// Copyright 2022 Woodpecker Authors
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

package pipeline

import (
	"context"
	"strings"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/common"
	"codefloe.com/crowci/crow/v6/cli/internal"
	crow "codefloe.com/crowci/crow/v6/crow-go/crow"
)

var pipelineCreateCmd = &cli.Command{
	Name:                      "create",
	Usage:                     "create new pipeline",
	ArgsUsage:                 "<repo-id|repo-full-name>",
	Action:                    pipelineCreate,
	DisableSliceFlagSeparator: true,
	Flags: append(common.OutputFlags("table"), []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "var",
			Usage: "key=value",
		},
	}...),
	// Exactly one of the two, matching the server: a pipeline runs against a
	// branch or a tag, never both.
	MutuallyExclusiveFlags: []cli.MutuallyExclusiveFlags{
		{
			Required: true,
			Flags: [][]cli.Flag{
				{
					&cli.StringFlag{
						Name:  "branch",
						Usage: "branch to create pipeline from",
					},
				},
				{
					&cli.StringFlag{
						Name:  "tag",
						Usage: "tag to create pipeline from",
					},
				},
			},
		},
	},
}

func pipelineCreate(ctx context.Context, c *cli.Command) error {
	repoIDOrFullName := c.Args().First()
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}
	repoID, err := internal.ParseRepo(client, repoIDOrFullName)
	if err != nil {
		return err
	}

	variables := make(map[string]string)

	for _, vaz := range c.StringSlice("var") {
		before, after, _ := strings.Cut(vaz, "=")
		if before != "" && after != "" {
			variables[before] = after
		}
	}

	options := &crow.PipelineOptions{
		Branch:    c.String("branch"),
		Tag:       c.String("tag"),
		Variables: variables,
	}

	pipeline, err := client.PipelineCreate(repoID, options)
	if err != nil {
		return err
	}

	return pipelineOutput(c, []*crow.Pipeline{pipeline})
}
