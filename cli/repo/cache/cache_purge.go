// Copyright 2026 Crow Authors
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

package cache

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/internal"
	crow "codefloe.com/crowci/crow/v6/crow-go/crow"
)

var cachePurgeCmd = &cli.Command{
	Name:      "purge",
	Usage:     "remove all build caches of a repository",
	ArgsUsage: "<repo-id|repo-full-name>",
	Action:    cachePurge,
}

func cachePurge(ctx context.Context, c *cli.Command) error {
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}
	return cachePurgeWithClient(c, client)
}

func cachePurgeWithClient(c *cli.Command, client crow.Client) error {
	repoIDOrFullName := c.Args().First()
	if repoIDOrFullName == "" {
		return fmt.Errorf("missing required argument repo-id / repo-full-name")
	}
	repoID, err := internal.ParseRepo(client, repoIDOrFullName)
	if err != nil {
		return err
	}
	if err := client.RepoCachePurge(repoID); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(c.Root().Writer, "Successfully purged all build caches of repository %s\n", repoIDOrFullName)
	return nil
}
