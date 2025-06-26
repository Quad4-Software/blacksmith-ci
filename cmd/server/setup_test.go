package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestServerPrivilegedPluginsMatchTypeEnvVar(t *testing.T) {
	runCommand := func(flags []cli.Flag, fn func(c *cli.Command)) {
		c := &cli.Command{
			Flags: flags,
			Action: func(_ context.Context, c *cli.Command) error {
				fn(c)
				return nil
			},
		}
		assert.NoError(t, c.Run(t.Context(), []string{"test-server"}))
	}

	t.Run("ReadFromCrowEnvVar", func(t *testing.T) {
		// Set the environment variable
		t.Setenv("CROW_PLUGINS_PRIVILEGED_MATCH_TYPE", "semver-range")

		flags := []cli.Flag{
			&cli.StringFlag{
				Sources: cli.EnvVars("WOODPECKER_PLUGINS_PRIVILEGED_MATCH_TYPE", "CROW_PLUGINS_PRIVILEGED_MATCH_TYPE"),
				Name:    "plugins-privileged-match-type",
				Usage:   "Type of match for privileged plugins: semver, semver-range, regex, or exact (default: semver)",
				Value:   "semver",
			},
		}

		runCommand(flags, func(c *cli.Command) {
			matchType := c.String("plugins-privileged-match-type")
			assert.Equal(t, "semver-range", matchType)
		})
	})

	t.Run("ReadFromWoodpeckerEnvVar", func(t *testing.T) {
		// Set the environment variable
		t.Setenv("WOODPECKER_PLUGINS_PRIVILEGED_MATCH_TYPE", "regex")

		flags := []cli.Flag{
			&cli.StringFlag{
				Sources: cli.EnvVars("WOODPECKER_PLUGINS_PRIVILEGED_MATCH_TYPE", "CROW_PLUGINS_PRIVILEGED_MATCH_TYPE"),
				Name:    "plugins-privileged-match-type",
				Usage:   "Type of match for privileged plugins: semver, semver-range, regex, or exact (default: semver)",
				Value:   "semver",
			},
		}

		runCommand(flags, func(c *cli.Command) {
			matchType := c.String("plugins-privileged-match-type")
			assert.Equal(t, "regex", matchType)
		})
	})

	t.Run("UseDefaultWhenNoEnvVar", func(t *testing.T) {
		flags := []cli.Flag{
			&cli.StringFlag{
				Sources: cli.EnvVars("WOODPECKER_PLUGINS_PRIVILEGED_MATCH_TYPE", "CROW_PLUGINS_PRIVILEGED_MATCH_TYPE"),
				Name:    "plugins-privileged-match-type",
				Usage:   "Type of match for privileged plugins: semver, semver-range, regex, or exact (default: semver)",
				Value:   "semver",
			},
		}

		runCommand(flags, func(c *cli.Command) {
			matchType := c.String("plugins-privileged-match-type")
			assert.Equal(t, "semver", matchType)
		})
	})

	t.Run("WoodpeckerEnvVarTakesPrecedence", func(t *testing.T) {
		// Set both environment variables, WOODPECKER should take precedence (first in the list)
		t.Setenv("CROW_PLUGINS_PRIVILEGED_MATCH_TYPE", "exact")
		t.Setenv("WOODPECKER_PLUGINS_PRIVILEGED_MATCH_TYPE", "regex")

		flags := []cli.Flag{
			&cli.StringFlag{
				Sources: cli.EnvVars("WOODPECKER_PLUGINS_PRIVILEGED_MATCH_TYPE", "CROW_PLUGINS_PRIVILEGED_MATCH_TYPE"),
				Name:    "plugins-privileged-match-type",
				Usage:   "Type of match for privileged plugins: semver, semver-range, regex, or exact (default: semver)",
				Value:   "semver",
			},
		}

		runCommand(flags, func(c *cli.Command) {
			matchType := c.String("plugins-privileged-match-type")
			assert.Equal(t, "regex", matchType)
		})
	})
}
