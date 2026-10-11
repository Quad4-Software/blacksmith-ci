// Copyright 2026 Crow Authors
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
	"context"
	"errors"
	"fmt"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/internal"
)

var userUpdateCmd = newUserUpdateCmd()

func newUserUpdateCmd() *cli.Command {
	return &cli.Command{
		Name:      "update",
		Usage:     "change the instance roles of a user",
		ArgsUsage: "<username>",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{Name: "role", Usage: "instance role (operator, secrets-manager, auditor); repeatable, replaces the current roles, so identity provider roles left out are removed"},
			&cli.BoolFlag{Name: "no-roles", Usage: "remove every role, including identity provider roles the user still holds"},
		},
		Action: userUpdate,
	}
}

func userUpdate(ctx context.Context, c *cli.Command) error {
	login := c.Args().First()
	if login == "" {
		return errors.New("missing username")
	}
	if !c.IsSet("role") && !c.Bool("no-roles") {
		return errors.New("set --role or --no-roles")
	}
	if c.IsSet("role") && c.Bool("no-roles") {
		return errors.New("--role and --no-roles cannot be used together")
	}
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}
	// The PATCH replaces email, avatar and more from the body, so start from the current user.
	user, err := client.User(login)
	if err != nil {
		return err
	}
	roles := []string{}
	if !c.Bool("no-roles") {
		roles = c.StringSlice("role")
	}
	user.Roles = &roles
	if _, err := client.UserPatch(user); err != nil {
		return err
	}
	fmt.Printf("Successfully updated roles of %s\n", login)
	return nil
}
