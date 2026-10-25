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
	"io"
	"os"
	"text/template"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/common"
	"codefloe.com/crowci/crow/v6/cli/internal"
	"codefloe.com/crowci/crow/v6/cli/output"
	crow "codefloe.com/crowci/crow/v6/crow-go/crow"
)

var cacheListCmd = &cli.Command{
	Name:   "ls",
	Usage:  "list the build caches and limits of all users and organizations",
	Action: cacheList,
	Flags:  common.OutputFlags("table"),
}

func cacheList(ctx context.Context, c *cli.Command) error {
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}
	caches, err := client.AdminCacheList()
	if err != nil {
		return err
	}
	return cacheListOutput(c, caches, os.Stdout, os.Stderr)
}

// cacheListOutput writes the caches in the requested format to out. The
// notice about caching being disabled goes to errOut so it never corrupts
// json, yaml or template output.
func cacheListOutput(c *cli.Command, caches *crow.AdminCaches, out, errOut io.Writer) error {
	outFmt, outOpt := output.ParseOutputOptions(c.String("output"))
	noHeader := c.Bool("output-no-headers")

	if !caches.Enabled {
		_, _ = fmt.Fprintln(errOut, "Note: build caching is disabled on the server")
	}

	switch outFmt {
	case "json":
		return output.WriteJSON(out, caches)
	case "yaml":
		return output.WriteYAML(out, caches)
	case "go-template":
		if len(outOpt) < 1 {
			return fmt.Errorf("%w: missing template", output.ErrOutputOptionRequired)
		}

		tmpl, err := template.New("_").Parse(outOpt[0] + "\n")
		if err != nil {
			return err
		}
		return tmpl.Execute(out, caches)
	case "table":
		fallthrough
	default:
		table := output.NewTable(out)

		field := func(name string, fn func(*crow.BuildCacheOrgSummary) string) {
			table.AddFieldFn(name, func(obj any) string {
				org, ok := obj.(*crow.BuildCacheOrgSummary)
				if !ok {
					return ""
				}
				return fn(org)
			})
		}
		field("ID", func(org *crow.BuildCacheOrgSummary) string { return fmt.Sprint(org.OrgID) })
		field("Name", func(org *crow.BuildCacheOrgSummary) string { return org.Name })
		field("Type", ownerType)
		field("Size", func(org *crow.BuildCacheOrgSummary) string { return formatBytes(org.Size) })
		field("Entries", func(org *crow.BuildCacheOrgSummary) string { return fmt.Sprint(org.Entries) })
		field("SizeLimit", func(org *crow.BuildCacheOrgSummary) string {
			return formatLimit(org.Limits.MaxSize, org.Settings.MaxSize)
		})
		field("Egress", func(org *crow.BuildCacheOrgSummary) string { return formatBytes(org.EgressMonth) })
		field("EgressLimit", func(org *crow.BuildCacheOrgSummary) string {
			return formatLimit(org.Limits.MaxEgress, org.Settings.MaxEgress)
		})
		field("Retention", func(org *crow.BuildCacheOrgSummary) string {
			return formatRetention(org.Limits.TTL, org.Settings.TTL)
		})
		field("HitRate", func(org *crow.BuildCacheOrgSummary) string { return formatHitRate(org.Usage) })
		field("Bucket", func(org *crow.BuildCacheOrgSummary) string { return output.YesNo(org.Bucket != nil) })

		cols := []string{"ID", "Name", "Type", "Size", "Size_Limit", "Egress", "Egress_Limit", "Retention", "Hit_Rate", "Bucket"}
		if len(outOpt) > 0 {
			cols = outOpt
		}
		if !noHeader {
			table.WriteHeader(cols)
		}
		if caches.Overview != nil {
			for _, org := range caches.Overview.Orgs {
				if err := table.Write(cols, org); err != nil {
					return err
				}
			}
		}
		return table.Flush()
	}
}
