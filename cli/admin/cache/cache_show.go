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
	"text/tabwriter"
	"text/template"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/common"
	"codefloe.com/crowci/crow/v6/cli/internal"
	"codefloe.com/crowci/crow/v6/cli/output"
	crow "codefloe.com/crowci/crow/v6/crow-go/crow"
)

var cacheShowCmd = &cli.Command{
	Name:      "show",
	Usage:     "show the build caches and limits of a user or organization",
	ArgsUsage: "<org-id|org-name|user-login>",
	Action:    cacheShow,
	Flags:     common.OutputFlags("table"),
}

func cacheShow(ctx context.Context, c *cli.Command) error {
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}
	return cacheShowWithClient(c, client)
}

func cacheShowWithClient(c *cli.Command, client crow.Client) error {
	orgID, err := parseOrg(client, c.Args().First())
	if err != nil {
		return err
	}
	org, err := client.AdminCacheOrg(orgID)
	if err != nil {
		return err
	}
	return cacheOrgOutput(c, org, c.Root().Writer)
}

// cacheOrgOutput writes the caches of one user or org in the requested
// format: a summary followed by the caches per repository for table output.
func cacheOrgOutput(c *cli.Command, org *crow.BuildCacheOrg, out io.Writer) error {
	outFmt, outOpt := output.ParseOutputOptions(c.String("output"))

	switch outFmt {
	case "json":
		return output.WriteJSON(out, org)
	case "yaml":
		return output.WriteYAML(out, org)
	case "go-template":
		if len(outOpt) < 1 {
			return fmt.Errorf("%w: missing template", output.ErrOutputOptionRequired)
		}

		tmpl, err := template.New("_").Parse(outOpt[0] + "\n")
		if err != nil {
			return err
		}
		return tmpl.Execute(out, org)
	case "table":
		fallthrough
	default:
		const padding = 2
		w := tabwriter.NewWriter(out, 0, 0, padding, ' ', 0)
		line := func(key, value string) {
			_, _ = fmt.Fprintf(w, "%s:\t%s\n", key, value)
		}
		line("Name", fmt.Sprintf("%s (%s)", org.Name, ownerType(&org.BuildCacheOrgSummary)))
		line("ID", fmt.Sprint(org.OrgID))
		line("Size", fmt.Sprintf("%s in %d entries across %d repositories", formatBytes(org.Size), org.Entries, org.Repos))
		line("Size limit", formatLimit(org.Limits.MaxSize, org.Settings.MaxSize))
		line("Egress", formatBytes(org.EgressMonth)+" this month")
		line("Egress limit", formatLimit(org.Limits.MaxEgress, org.Settings.MaxEgress))
		line("Retention", formatRetention(org.Limits.TTL, org.Settings.TTL))
		line("Hit rate", formatHitRate(org.Usage)+" over 30 days")
		if bucket := org.Bucket; bucket != nil {
			line("Bucket", fmt.Sprintf("%s/%s, %s in %d entries, limits set by the owner",
				bucket.Endpoint, bucket.Bucket, formatBytes(org.BucketSize), org.BucketEntries))
		}
		if err := w.Flush(); err != nil {
			return err
		}
		if len(org.RepoList) == 0 {
			return nil
		}

		_, _ = fmt.Fprintln(out)
		table := output.NewTable(out)
		field := func(name string, fn func(*crow.BuildCacheRepoSummary) string) {
			table.AddFieldFn(name, func(obj any) string {
				repo, ok := obj.(*crow.BuildCacheRepoSummary)
				if !ok {
					return ""
				}
				return fn(repo)
			})
		}
		field("Repo", func(repo *crow.BuildCacheRepoSummary) string { return repo.FullName })
		field("Size", func(repo *crow.BuildCacheRepoSummary) string { return formatBytes(repo.Size) })
		field("Entries", func(repo *crow.BuildCacheRepoSummary) string { return fmt.Sprint(repo.Entries) })
		field("HitRate", func(repo *crow.BuildCacheRepoSummary) string { return formatHitRate(repo.Usage) })
		field("Egress30d", func(repo *crow.BuildCacheRepoSummary) string { return formatBytes(repo.Usage.Egress) })

		cols := []string{"Repo", "Size", "Entries", "Hit_Rate", "Egress_30d"}
		if !c.Bool("output-no-headers") {
			table.WriteHeader(cols)
		}
		for _, repo := range org.RepoList {
			if err := table.Write(cols, repo); err != nil {
				return err
			}
		}
		return table.Flush()
	}
}
