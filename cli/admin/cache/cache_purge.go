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
	Usage:     "remove all build caches of a user or organization",
	ArgsUsage: "<org-id|org-name|user-login>",
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
	orgIDOrName := c.Args().First()
	orgID, err := parseOrg(client, orgIDOrName)
	if err != nil {
		return err
	}
	result, err := client.AdminCacheOrgPurge(orgID)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(c.Root().Writer, "Removed %d build caches (%s) of %s\n", result.Removed, formatBytes(result.Freed), orgIDOrName)
	return nil
}
