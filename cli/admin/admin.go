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

package admin

import (
	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v5/cli/admin/agent"
	"codefloe.com/crowci/crow/v5/cli/admin/loglevel"
	"codefloe.com/crowci/crow/v5/cli/admin/org"
	"codefloe.com/crowci/crow/v5/cli/admin/registry"
	"codefloe.com/crowci/crow/v5/cli/admin/secret"
	"codefloe.com/crowci/crow/v5/cli/admin/user"
)

// Command exports the admin command set.
var Command = &cli.Command{
	Name:  "admin",
	Usage: "manage server settings",
	Commands: []*cli.Command{
		agent.Command,
		loglevel.Command,
		org.Command,
		registry.Command,
		secret.Command,
		user.Command,
	},
}
