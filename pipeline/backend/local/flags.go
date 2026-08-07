// Copyright 2023 Woodpecker Authors
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

package local

import (
	"os"

	"github.com/urfave/cli/v3"
)

var Flags = []cli.Flag{
	&cli.StringFlag{
		Name:    "backend-local-temp-dir",
		Sources: cli.EnvVars("CROW_BACKEND_LOCAL_TEMP_DIR"),
		Usage:   "set a different temp dir to clone workflows into",
		Value:   os.TempDir(),
	},
	&cli.StringFlag{
		Name:    "backend-local-sandbox-level",
		Sources: cli.EnvVars("CROW_BACKEND_LOCAL_SANDBOX_LEVEL"),
		Usage:   "sandbox security level on macOS (none, standard, strict)",
		Value:   "standard",
	},
	&cli.BoolFlag{
		Name:    "backend-local-use-host-home",
		Sources: cli.EnvVars("CROW_BACKEND_LOCAL_USE_HOST_HOME"),
		Usage: "run steps with the agent account's own home instead of a per-workflow one. " +
			"Needed by tools that resolve the home directory from the account rather than $HOME, " +
			"such as Xcode, which reads provisioning profiles and writes DerivedData there. " +
			"Trades home isolation between workflows for compatibility with them",
		Value: false,
	},
}
