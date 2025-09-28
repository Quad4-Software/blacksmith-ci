package config

import (
	"context"
	"errors"
	"slices"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"codeberg.org/crowci/crow/v4/cli/internal"
)

type Config struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"-"`
	LogLevel  string `json:"log_level"`
}

var skipSetupForCommands = []string{"setup", "help", "h", "version", "update", "lint", "exec", "completion"}

func Load(ctx context.Context, c *cli.Command) error {
	if firstArg := c.Args().First(); slices.Contains(skipSetupForCommands, firstArg) {
		return nil
	}

	config, err := Get(ctx, c, c.String("config"))
	if err != nil {
		return err
	}

	if config.ServerURL == "" || config.Token == "" {
		log.Info().Msg("crow-cli requires configuration via environment variables (CROW_SERVER & CROW_TOKEN) or command line flags (--server <url> --token <token>)")
		return errors.New("crow-cli is not configured")
	}

	err = c.Set("server", config.ServerURL)
	if err != nil {
		return err
	}

	err = c.Set("token", config.Token)
	if err != nil {
		return err
	}

	err = c.Set("log-level", config.LogLevel)
	if err != nil {
		return err
	}

	log.Debug().Any("config", config).Msg("loaded config")

	err = displayInstanceInfo(ctx, c, config)
	if err != nil {
		log.Debug().Err(err).Msg("failed to fetch instance/user info")
	}

	return nil
}

func Get(_ context.Context, c *cli.Command, _configPath string) (*Config, error) {
	conf := &Config{
		LogLevel:  c.String("log-level"),
		Token:     c.String("token"),
		ServerURL: c.String("server"),
	}

	log.Debug().Msg("config loaded from environment variables and command line arguments only")

	return conf, nil
}

func Save(_ context.Context, c *cli.Command, _configPath string, conf *Config) error {
	log.Info().Msg("Configuration not saved - please provide via environment variables or command line arguments in future runs.")
	return nil
}

func displayInstanceInfo(ctx context.Context, c *cli.Command, config *Config) error {
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}

	user, err := client.Self()
	if err != nil {
		return err
	}

	log.Info().
		Str("server", config.ServerURL).
		Str("user", user.Login).
		Msg("Crow instance info:")

	return nil
}
