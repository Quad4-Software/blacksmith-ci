package setup

import (
	"context"
	"errors"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"codeberg.org/crowci/crow/v3/cli/internal/config"
)

// Command exports the setup command.
var Command = &cli.Command{
	Name:      "setup",
	Usage:     "setup the crow-cli for the first time",
	ArgsUsage: "[server]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Sources: cli.EnvVars("WOODPECKER_SERVER", "CROW_SERVER"),
			Name:    "server",
			Usage:   "URL of the Crow server",
		},
		&cli.StringFlag{
			Sources: cli.EnvVars("WOODPECKER_TOKEN", "CROW_TOKEN"),
			Name:    "token",
			Usage:   "token to authenticate with the Crow server",
		},
	},
	Action: setup,
}

func setup(ctx context.Context, c *cli.Command) error {
	serverURL := c.String("server")
	if serverURL == "" {
		serverURL = c.Args().First()
	}

	if serverURL == "" {
		log.Error().Msg("Server URL is required.")
		log.Info().Msg("Please provide server URL via one of these methods:")
		log.Info().Msg("  1. Environment variables: CROW_SERVER or WOODPECKER_SERVER")
		log.Info().Msg("  2. Command line flag: --server <url>")
		return errors.New("server URL is required")
	}

	if !strings.Contains(serverURL, "://") {
		serverURL = "https://" + serverURL
	}

	token := c.String("token")
	if token == "" {
		log.Error().Msg("Authentication token is required.")
		log.Info().Msg("Please provide authentication token via one of these methods:")
		log.Info().Msg("  1. Environment variables: CROW_TOKEN or WOODPECKER_TOKEN")
		log.Info().Msg("  2. Command line flag: --token <token>")
		log.Info().Msg("")
		log.Info().Msg("To generate a token, go to the user settings panel of the respective Crow instance.")
		return errors.New("authentication token is required")
	}

	err := config.Save(ctx, c, c.String("config"), &config.Config{
		ServerURL: serverURL,
		Token:     token,
		LogLevel:  "info",
	})
	if err != nil {
		return err
	}

	log.Info().Msg("crow-cli has been successfully setup")

	return nil
}
