// Copyright 2023 Woodpecker Authors
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

package user

import (
	"fmt"
	"io"
	"os"
	"text/template"

	"github.com/urfave/cli/v3"

	"codeberg.org/crowci/crow/v5/cli/output"
	crow "codeberg.org/crowci/crow/v5/crow-go/crow"
)

// Command exports the user command set.
var Command = &cli.Command{
	Name:  "user",
	Usage: "manage users",
	Commands: []*cli.Command{
		userAddCmd,
		userListCmd,
		userRemoveCmd,
		userShowCmd,
	},
}

func userOutput(c *cli.Command, users []*crow.User, fd ...io.Writer) error {
	outFmt, outOpt := output.ParseOutputOptions(c.String("output"))
	noHeader := c.Bool("output-no-headers")

	var out io.Writer = os.Stdout
	if len(fd) > 0 {
		out = fd[0]
	}

	switch outFmt {
	case "json":
		return output.WriteJSON(out, users)
	case "yaml":
		return output.WriteYAML(out, users)
	case "go-template":
		if len(outOpt) < 1 {
			return fmt.Errorf("%w: missing template", output.ErrOutputOptionRequired)
		}
		tmpl, err := template.New("_").Parse(outOpt[0] + "\n")
		if err != nil {
			return err
		}
		return tmpl.Execute(out, users)
	case "table":
		fallthrough
	default:
		table := output.NewTable(out)
		cols := []string{"ID", "Login", "Email", "Admin"}
		if len(outOpt) > 0 {
			cols = outOpt
		}
		if !noHeader {
			table.WriteHeader(cols)
		}
		for _, user := range users {
			if err := table.Write(cols, user); err != nil {
				return err
			}
		}
		table.Flush()
	}

	return nil
}
