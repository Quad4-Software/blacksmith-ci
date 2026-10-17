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
	"strconv"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/internal"
	crow "codefloe.com/crowci/crow/v6/crow-go/crow"
)

var cacheRemoveCmd = &cli.Command{
	Name:      "rm",
	Usage:     "remove a build cache entry",
	ArgsUsage: "<repo-id|repo-full-name> <cache-id>",
	Action:    cacheRemove,
}

func cacheRemove(ctx context.Context, c *cli.Command) error {
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}
	return cacheRemoveWithClient(c, client)
}

func cacheRemoveWithClient(c *cli.Command, client crow.Client) error {
	if c.Args().Len() != 2 { //nolint:mnd
		return fmt.Errorf("expected arguments <repo-id|repo-full-name> <cache-id>")
	}
	repoIDOrFullName := c.Args().Get(0)
	cacheID, err := strconv.ParseInt(c.Args().Get(1), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid cache id '%s': %w", c.Args().Get(1), err)
	}
	repoID, err := internal.ParseRepo(client, repoIDOrFullName)
	if err != nil {
		return err
	}
	if err := client.RepoCacheDelete(repoID, cacheID); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(c.Root().Writer, "Successfully removed cache entry %d from repository %s\n", cacheID, repoIDOrFullName)
	return nil
}
