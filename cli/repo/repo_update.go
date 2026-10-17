// Copyright 2022 Woodpecker Authors
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

package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/internal"
	crow "codefloe.com/crowci/crow/v6/crow-go/crow"
)

var repoUpdateCmd = &cli.Command{
	Name:      "update",
	Usage:     "update a repository",
	ArgsUsage: "<repo-id|repo-full-name>",
	Action:    repoUpdate,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "trusted",
			Usage: "set all trusted settings (network, volumes, security) to true",
		},
		&cli.BoolFlag{
			Name:  "trusted-network",
			Usage: "allow trusted network access",
		},
		&cli.BoolFlag{
			Name:  "trusted-volumes",
			Usage: "allow trusted volume mounts",
		},
		&cli.BoolFlag{
			Name:  "trusted-security",
			Usage: "allow trusted security settings (privileged mode)",
		},
		&cli.StringFlag{
			Name:  "require-approval",
			Usage: "repository requires approval for",
		},
		&cli.DurationFlag{
			Name:  "timeout",
			Usage: "repository timeout",
		},
		&cli.StringFlag{
			Name:  "visibility",
			Usage: "repository visibility",
		},
		&cli.StringFlag{
			Name:  "config",
			Usage: "repository configuration path. Example: .woodpecker.yml",
		},
		&cli.Int64Flag{
			Name:  "pipeline-counter",
			Usage: "repository starting pipeline number",
		},
		&cli.BoolFlag{
			Name:  "unsafe",
			Usage: "allow unsafe operations",
		},
		&cli.Int64Flag{
			Name:  "logs-pipelines-keep-min",
			Usage: "minimum amount of pipelines to keep logs for",
		},
		&cli.StringFlag{
			Name:  "logs-duration-keep",
			Usage: "maximum age of pipeline logs to keep",
		},
	},
}

func repoUpdate(ctx context.Context, c *cli.Command) error {
	repoIDOrFullName := c.Args().First()
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}
	repoID, err := internal.ParseRepo(client, repoIDOrFullName)
	if err != nil {
		return err
	}

	var (
		visibility           = c.String("visibility")
		config               = c.String("config")
		timeout              = c.Duration("timeout")
		LogsPipelinesKeepMin = int(c.Int64("logs-pipelines-keep-min"))
		LogsDurationKeep     = c.String("logs-duration-keep")
		requireApproval      = c.String("require-approval")
		pipelineCounter      = int(c.Int64("pipeline-counter"))
		unsafe               = c.Bool("unsafe")
	)

	patch := new(crow.RepoPatch)

	// Handle trusted flags - either the convenience --trusted flag or individual flags
	if c.IsSet("trusted") || c.IsSet("trusted-network") || c.IsSet("trusted-volumes") || c.IsSet("trusted-security") {
		trustedPatch := &crow.TrustedConfigurationPatch{}

		// If --trusted is set, it sets all three to true
		if c.IsSet("trusted") {
			allTrue := true
			trustedPatch.Network = &allTrue
			trustedPatch.Volumes = &allTrue
			trustedPatch.Security = &allTrue
		}

		// Individual flags override the --trusted flag if specified
		if c.IsSet("trusted-network") {
			v := c.Bool("trusted-network")
			trustedPatch.Network = &v
		}
		if c.IsSet("trusted-volumes") {
			v := c.Bool("trusted-volumes")
			trustedPatch.Volumes = &v
		}
		if c.IsSet("trusted-security") {
			v := c.Bool("trusted-security")
			trustedPatch.Security = &v
		}

		patch.Trusted = trustedPatch
	}

	if c.IsSet("require-approval") {
		if mode := crow.ApprovalMode(requireApproval); mode.Valid() {
			patch.RequireApproval = &mode
		} else {
			return fmt.Errorf("update approval mode failed: '%s' is no valid mode", mode)
		}
	}
	if c.IsSet("timeout") {
		v := int64(timeout / time.Minute)
		patch.Timeout = &v
	}
	if c.IsSet("logs-pipelines-keep-min") {
		v := int64(LogsPipelinesKeepMin)
		patch.LogsPipelinesKeepMin = &v
	}
	if c.IsSet("logs-duration-keep") {
		patch.LogsDurationKeep = &LogsDurationKeep
	}
	if c.IsSet("config") {
		patch.Config = &config
	}
	if c.IsSet("visibility") {
		switch visibility {
		case "public", "private", "internal":
			patch.Visibility = &visibility
		}
	}
	if c.IsSet("pipeline-counter") && !unsafe {
		fmt.Printf("Setting the pipeline counter is an unsafe operation that could put your repository in an inconsistent state. Please use --unsafe to proceed")
	}
	if c.IsSet("pipeline-counter") && unsafe {
		patch.PipelineCounter = &pipelineCounter
	}

	repo, err := client.RepoPatch(repoID, patch)
	if err != nil {
		// Provide helpful context for permission errors when using trusted flags
		if patch.Trusted != nil {
			clientErr := &crow.ClientError{}
			if errors.As(err, &clientErr) {
				return fmt.Errorf("%w\nNote: Changing trusted settings requires instance admin privileges", err)
			}
		}
		return err
	}

	fmt.Printf("Successfully updated repository %s\n", repo.FullName)
	return nil
}
