// Copyright 2021 Woodpecker Authors
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

package main

import (
	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/admin"
	"codefloe.com/crowci/crow/v6/cli/common"
	"codefloe.com/crowci/crow/v6/cli/exec"
	"codefloe.com/crowci/crow/v6/cli/info"
	"codefloe.com/crowci/crow/v6/cli/lint"
	"codefloe.com/crowci/crow/v6/cli/org"
	"codefloe.com/crowci/crow/v6/cli/pipeline"
	"codefloe.com/crowci/crow/v6/cli/repo"
	"codefloe.com/crowci/crow/v6/cli/update"
	"codefloe.com/crowci/crow/v6/version"
)

//go:generate go run docs.go app.go
func newApp() *cli.Command {
	app := &cli.Command{}
	app.Name = "crow"
	app.Description = "Crow CI command line utility"
	app.Version = version.String()
	app.Usage = "command line utility"
	app.Flags = common.GlobalFlags
	app.Before = common.Before
	app.After = common.After
	app.Suggest = true
	app.ConfigureShellCompletionCommand = func(c *cli.Command) {
		c.Hidden = false
		c.Usage = "generate completion script for the specified shell"
	}
	app.Commands = []*cli.Command{
		admin.Command,
		exec.Command,
		info.Command,
		lint.Command,
		org.Command,
		pipeline.Command,
		repo.Command,
		update.Command,
	}

	return app
}
