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
	"errors"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/common"
	"codefloe.com/crowci/crow/v6/cli/internal"
	crow "codefloe.com/crowci/crow/v6/crow-go/crow"
)

var cacheSetCmd = &cli.Command{
	Name:  "set",
	Usage: "set the build cache limits of a user or organization",
	Description: `Limits not given keep their current value. Each takes "default" for the
server default or "unlimited" to lift the limit. Sizes take units such as
500MB or 10GiB, the retention durations such as 36h, 7d or 2w.

The limits cover caches in the server's storage; a bucket the user or
organization connected is limited by its owner. Lowering the size limit
evicts the least recently used caches right away.`,
	ArgsUsage: "<org-id|org-name|user-login>",
	Action:    cacheSet,
	Flags:     cacheSetFlags(),
}

// cacheSetFlags returns new flags on each call, so tests running the command
// repeatedly do not see values set by an earlier run.
func cacheSetFlags() []cli.Flag {
	return append([]cli.Flag{
		&cli.StringFlag{
			Name:  "max-size",
			Usage: "maximum size of all caches, e.g. 10GiB, default or unlimited",
		},
		&cli.StringFlag{
			Name:  "max-egress",
			Usage: "maximum size restored per calendar month (UTC), e.g. 50GiB, default or unlimited",
		},
		&cli.StringFlag{
			Name:  "ttl",
			Usage: "remove caches not restored for this long, e.g. 7d, default or unlimited",
		},
	}, common.OutputFlags("table")...)
}

func cacheSet(ctx context.Context, c *cli.Command) error {
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}
	return cacheSetWithClient(c, client)
}

func cacheSetWithClient(c *cli.Command, client crow.Client) error {
	if !c.IsSet("max-size") && !c.IsSet("max-egress") && !c.IsSet("ttl") {
		return errors.New("nothing to set: pass --max-size, --max-egress or --ttl")
	}
	orgID, err := parseOrg(client, c.Args().First())
	if err != nil {
		return err
	}

	// The API replaces all limits, so start from the current settings.
	current, err := client.AdminCacheOrg(orgID)
	if err != nil {
		return err
	}
	limits := current.Settings
	for _, flag := range []struct {
		name  string
		parse func(string) (int64, error)
		value *int64
	}{
		{"max-size", parseSizeLimit, &limits.MaxSize},
		{"max-egress", parseSizeLimit, &limits.MaxEgress},
		{"ttl", parseTTL, &limits.TTL},
	} {
		if !c.IsSet(flag.name) {
			continue
		}
		if *flag.value, err = flag.parse(c.String(flag.name)); err != nil {
			return err
		}
	}

	org, err := client.AdminCacheOrgUpdate(orgID, &limits)
	if err != nil {
		return err
	}
	return cacheOrgOutput(c, org, c.Root().Writer)
}
