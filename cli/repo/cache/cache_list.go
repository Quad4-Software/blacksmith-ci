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
	"strings"
	"text/template"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/common"
	"codefloe.com/crowci/crow/v6/cli/internal"
	"codefloe.com/crowci/crow/v6/cli/output"
	crow "codefloe.com/crowci/crow/v6/crow-go/crow"
)

var cacheListCmd = &cli.Command{
	Name:      "ls",
	Usage:     "list build caches",
	ArgsUsage: "[repo-id|repo-full-name]",
	Action:    cacheList,
	Flags:     common.OutputFlags("table"),
}

func cacheList(ctx context.Context, c *cli.Command) error {
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}
	repoID, err := internal.ParseRepo(client, c.Args().First())
	if err != nil {
		return err
	}
	caches, err := client.RepoCacheList(repoID)
	if err != nil {
		return err
	}
	return cacheOutput(c, caches, os.Stdout, os.Stderr)
}

// cacheOutput writes the caches in the requested format to out. The notice
// about caching being disabled goes to errOut so it never corrupts json, yaml
// or template output.
func cacheOutput(c *cli.Command, caches *crow.RepoCaches, out, errOut io.Writer) error {
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
		if err := tmpl.Execute(out, caches); err != nil {
			return err
		}
	case "table":
		fallthrough
	default:
		table := output.NewTable(out)

		table.AddFieldFn("Scope", func(obj any) string {
			entry, ok := obj.(*crow.BuildCache)
			if !ok {
				return ""
			}
			return strings.TrimPrefix(entry.Scope, "refs/heads/")
		})
		table.AddFieldFn("Size", func(obj any) string {
			entry, ok := obj.(*crow.BuildCache)
			if !ok {
				return ""
			}
			return humanize.IBytes(uint64(max(entry.Size, 0)))
		})
		table.AddFieldFn("LastUsed", func(obj any) string {
			entry, ok := obj.(*crow.BuildCache)
			if !ok {
				return ""
			}
			if entry.LastUsed == 0 {
				return output.NA("")
			}
			return humanize.Time(time.Unix(entry.LastUsed, 0))
		})
		table.AddFieldFn("Pending", func(obj any) string {
			entry, ok := obj.(*crow.BuildCache)
			if !ok {
				return ""
			}
			return output.YesNo(!entry.Committed)
		})

		cols := []string{"ID", "Key", "Scope", "Size", "Last_Used", "Pending"}

		if len(outOpt) > 0 {
			cols = outOpt
		}
		if !noHeader {
			table.WriteHeader(cols)
		}
		for _, entry := range caches.Entries {
			if err := table.Write(cols, entry); err != nil {
				return err
			}
		}
		table.Flush()
	}

	return nil
}
