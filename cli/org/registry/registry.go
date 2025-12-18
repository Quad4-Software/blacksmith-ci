// Copyright 2024 Woodpecker Authors
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

package registry

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"text/template"

	"github.com/urfave/cli/v3"

	"codeberg.org/crowci/crow/v5/cli/output"
	crow "codeberg.org/crowci/crow/v5/crow-go/crow"
)

// Command exports the registry command set.
var Command = &cli.Command{
	Name:  "registry",
	Usage: "manage organization registries",
	Commands: []*cli.Command{
		registryCreateCmd,
		registryDeleteCmd,
		registryListCmd,
		registryShowCmd,
		registryUpdateCmd,
	},
}

func parseTargetArgs(client crow.Client, c *cli.Command) (orgID int64, err error) {
	orgIDOrName := c.String("organization")
	if orgIDOrName == "" {
		orgIDOrName = c.Args().First()
	}

	if orgIDOrName == "" {
		if err := cli.ShowSubcommandHelp(c); err != nil {
			return -1, err
		}
	}

	if orgID, err := strconv.ParseInt(orgIDOrName, 10, 64); err == nil {
		return orgID, nil
	}

	org, err := client.OrgLookup(orgIDOrName)
	if err != nil {
		return -1, err
	}

	return org.ID, nil
}

func registryOutput(c *cli.Command, registries []*crow.Registry, fd ...io.Writer) error {
	outFmt, outOpt := output.ParseOutputOptions(c.String("output"))
	noHeader := c.Bool("output-no-headers")

	var out io.Writer = os.Stdout
	if len(fd) > 0 {
		out = fd[0]
	}

	switch outFmt {
	case "json":
		return output.WriteJSON(out, registries)
	case "yaml":
		return output.WriteYAML(out, registries)
	case "go-template":
		if len(outOpt) < 1 {
			return fmt.Errorf("%w: missing template", output.ErrOutputOptionRequired)
		}
		tmpl, err := template.New("_").Parse(outOpt[0] + "\n")
		if err != nil {
			return err
		}
		return tmpl.Execute(out, registries)
	case "table":
		fallthrough
	default:
		table := output.NewTable(out)
		cols := []string{"ID", "Address", "Username"}
		if len(outOpt) > 0 {
			cols = outOpt
		}
		if !noHeader {
			table.WriteHeader(cols)
		}
		for _, registry := range registries {
			if err := table.Write(cols, registry); err != nil {
				return err
			}
		}
		table.Flush()
	}

	return nil
}
