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

var cacheCleanupCmd = &cli.Command{
	Name:   "cleanup",
	Usage:  "remove expired build caches and enforce the size limits now",
	Action: cacheCleanup,
}

func cacheCleanup(ctx context.Context, c *cli.Command) error {
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}
	return cacheCleanupWithClient(c, client)
}

func cacheCleanupWithClient(c *cli.Command, client crow.Client) error {
	result, err := client.AdminCacheCleanup()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(c.Root().Writer, "Removed %d build caches (%s)\n", result.Removed, formatBytes(result.Freed))
	return nil
}
