// Copyright 2025 Crow Authors
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

package exec

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestSecretsStringMapFlag(t *testing.T) {
	runCommand := func(flags []cli.Flag, fn func(c *cli.Command)) {
		c := &cli.Command{
			Flags: flags,
			Action: func(_ context.Context, c *cli.Command) error {
				fn(c)
				return nil
			},
		}
		assert.NoError(t, c.Run(t.Context(), []string{"crow-cli"}))
	}

	t.Run("ParseSecretsFromCommandLine", func(t *testing.T) {
		flags := []cli.Flag{
			&cli.StringMapFlag{
				Sources: cli.EnvVars("WOODPECKER_SECRETS", "CROW_SECRETS"),
				Name:    "secrets",
				Usage:   "map of secrets, ex. 'secret=\"val\",secret2=\"value2\"'",
			},
		}

		runCommand(flags, func(c *cli.Command) {
			_ = c.Set("secrets", "DB_PASSWORD=mysecret123,API_KEY=abc-def-ghi,TOKEN=xyz789")

			secrets := c.StringMap("secrets")
			assert.Len(t, secrets, 3)
			assert.Equal(t, "mysecret123", secrets["DB_PASSWORD"])
			assert.Equal(t, "abc-def-ghi", secrets["API_KEY"])
			assert.Equal(t, "xyz789", secrets["TOKEN"])
		})
	})

	t.Run("ParseSecretsFromWoodpeckerEnvVar", func(t *testing.T) {
		t.Setenv("WOODPECKER_SECRETS", "SECRET1=value1,SECRET2=value2")

		flags := []cli.Flag{
			&cli.StringMapFlag{
				Sources: cli.EnvVars("WOODPECKER_SECRETS", "CROW_SECRETS"),
				Name:    "secrets",
				Usage:   "map of secrets, ex. 'secret=\"val\",secret2=\"value2\"'",
			},
		}

		runCommand(flags, func(c *cli.Command) {
			secrets := c.StringMap("secrets")
			assert.Len(t, secrets, 2)
			assert.Equal(t, "value1", secrets["SECRET1"])
			assert.Equal(t, "value2", secrets["SECRET2"])
		})
	})

	t.Run("ParseSecretsFromCrowEnvVar", func(t *testing.T) {
		t.Setenv("CROW_SECRETS", "MY_SECRET=crow-secret,ANOTHER=crow-value")

		flags := []cli.Flag{
			&cli.StringMapFlag{
				Sources: cli.EnvVars("WOODPECKER_SECRETS", "CROW_SECRETS"),
				Name:    "secrets",
				Usage:   "map of secrets, ex. 'secret=\"val\",secret2=\"value2\"'",
			},
		}

		runCommand(flags, func(c *cli.Command) {
			secrets := c.StringMap("secrets")
			assert.Len(t, secrets, 2)
			assert.Equal(t, "crow-secret", secrets["MY_SECRET"])
			assert.Equal(t, "crow-value", secrets["ANOTHER"])
		})
	})

	t.Run("WoodpeckerEnvVarTakesPrecedence", func(t *testing.T) {
		// Set both environment variables
		t.Setenv("WOODPECKER_SECRETS", "SECRET=woodpecker-value")
		t.Setenv("CROW_SECRETS", "SECRET=crow-value")

		flags := []cli.Flag{
			&cli.StringMapFlag{
				Sources: cli.EnvVars("WOODPECKER_SECRETS", "CROW_SECRETS"),
				Name:    "secrets",
				Usage:   "map of secrets, ex. 'secret=\"val\",secret2=\"value2\"'",
			},
		}

		runCommand(flags, func(c *cli.Command) {
			secrets := c.StringMap("secrets")
			assert.Len(t, secrets, 1)
			assert.Equal(t, "woodpecker-value", secrets["SECRET"])
		})
	})

	t.Run("CommandLineOverridesEnvVar", func(t *testing.T) {
		t.Setenv("WOODPECKER_SECRETS", "SECRET=env-value")

		flags := []cli.Flag{
			&cli.StringMapFlag{
				Sources: cli.EnvVars("WOODPECKER_SECRETS", "CROW_SECRETS"),
				Name:    "secrets",
				Usage:   "map of secrets, ex. 'secret=\"val\",secret2=\"value2\"'",
			},
		}

		runCommand(flags, func(c *cli.Command) {
			_ = c.Set("secrets", "SECRET=cli-value")

			secrets := c.StringMap("secrets")
			assert.Len(t, secrets, 1)
			assert.Equal(t, "cli-value", secrets["SECRET"])
		})
	})

	t.Run("EmptySecrets", func(t *testing.T) {
		flags := []cli.Flag{
			&cli.StringMapFlag{
				Sources: cli.EnvVars("WOODPECKER_SECRETS", "CROW_SECRETS"),
				Name:    "secrets",
				Usage:   "map of secrets, ex. 'secret=\"val\",secret2=\"value2\"'",
			},
		}

		runCommand(flags, func(c *cli.Command) {
			secrets := c.StringMap("secrets")
			assert.Len(t, secrets, 0)
		})
	})

	t.Run("SingleSecret", func(t *testing.T) {
		flags := []cli.Flag{
			&cli.StringMapFlag{
				Sources: cli.EnvVars("WOODPECKER_SECRETS", "CROW_SECRETS"),
				Name:    "secrets",
				Usage:   "map of secrets, ex. 'secret=\"val\",secret2=\"value2\"'",
			},
		}

		runCommand(flags, func(c *cli.Command) {
			_ = c.Set("secrets", "SINGLE_SECRET=single-value")

			secrets := c.StringMap("secrets")
			assert.Len(t, secrets, 1)
			assert.Equal(t, "single-value", secrets["SINGLE_SECRET"])
		})
	})

	t.Run("SecretsWithSpecialCharacters", func(t *testing.T) {
		flags := []cli.Flag{
			&cli.StringMapFlag{
				Sources: cli.EnvVars("WOODPECKER_SECRETS", "CROW_SECRETS"),
				Name:    "secrets",
				Usage:   "map of secrets, ex. 'secret=\"val\",secret2=\"value2\"'",
			},
		}

		runCommand(flags, func(c *cli.Command) {
			_ = c.Set("secrets", "SECRET_WITH_UNDERSCORE=value_with_underscore,SECRET-WITH-DASH=value-with-dash,SECRET.WITH.DOT=value.with.dot")

			secrets := c.StringMap("secrets")
			assert.Len(t, secrets, 3)
			assert.Equal(t, "value_with_underscore", secrets["SECRET_WITH_UNDERSCORE"])
			assert.Equal(t, "value-with-dash", secrets["SECRET-WITH-DASH"])
			assert.Equal(t, "value.with.dot", secrets["SECRET.WITH.DOT"])
		})
	})

	t.Run("SecretsWithEmptyValue", func(t *testing.T) {
		flags := []cli.Flag{
			&cli.StringMapFlag{
				Sources: cli.EnvVars("WOODPECKER_SECRETS", "CROW_SECRETS"),
				Name:    "secrets",
				Usage:   "map of secrets, ex. 'secret=\"val\",secret2=\"value2\"'",
			},
		}

		runCommand(flags, func(c *cli.Command) {
			_ = c.Set("secrets", "EMPTY_SECRET=,NON_EMPTY=has-value")

			secrets := c.StringMap("secrets")
			assert.Len(t, secrets, 2)
			assert.Equal(t, "", secrets["EMPTY_SECRET"])
			assert.Equal(t, "has-value", secrets["NON_EMPTY"])
		})
	})

	t.Run("SecretsWithSpaces", func(t *testing.T) {
		flags := []cli.Flag{
			&cli.StringMapFlag{
				Sources: cli.EnvVars("WOODPECKER_SECRETS", "CROW_SECRETS"),
				Name:    "secrets",
				Usage:   "map of secrets, ex. 'secret=\"val\",secret2=\"value2\"'",
			},
		}

		runCommand(flags, func(c *cli.Command) {
			_ = c.Set("secrets", "SECRET_WITH_SPACES=value with spaces,TRIMMED=  trimmed  ")

			secrets := c.StringMap("secrets")
			assert.Len(t, secrets, 2)
			assert.Equal(t, "value with spaces", secrets["SECRET_WITH_SPACES"])
			assert.Equal(t, "  trimmed  ", secrets["TRIMMED"])
		})
	})

	t.Run("SecretsWithCommasInValue", func(t *testing.T) {
		flags := []cli.Flag{
			&cli.StringMapFlag{
				Sources: cli.EnvVars("WOODPECKER_SECRETS", "CROW_SECRETS"),
				Name:    "secrets",
				Usage:   "map of secrets, ex. 'secret=\"val\",secret2=\"value2\"'",
			},
		}

		runCommand(flags, func(c *cli.Command) {
			// Note: urfave/cli v3 doesn't support escaping commas in StringMapFlag values
			// Values with commas need to be passed via environment variables or separate flags
			_ = c.Set("secrets", "CSV_DATA=value1;value2;value3,NORMAL=normal-value")

			secrets := c.StringMap("secrets")
			assert.Len(t, secrets, 2)
			assert.Equal(t, "value1;value2;value3", secrets["CSV_DATA"])
			assert.Equal(t, "normal-value", secrets["NORMAL"])
		})
	})

	t.Run("SecretsWithEqualSignsInValue", func(t *testing.T) {
		flags := []cli.Flag{
			&cli.StringMapFlag{
				Sources: cli.EnvVars("WOODPECKER_SECRETS", "CROW_SECRETS"),
				Name:    "secrets",
				Usage:   "map of secrets, ex. 'secret=\"val\",secret2=\"value2\"'",
			},
		}

		runCommand(flags, func(c *cli.Command) {
			_ = c.Set("secrets", "BASE64=dGVzdD09,EQUATION=x=y+z")

			secrets := c.StringMap("secrets")
			assert.Len(t, secrets, 2)
			assert.Equal(t, "dGVzdD09", secrets["BASE64"])
			assert.Equal(t, "x=y+z", secrets["EQUATION"])
		})
	})
}
