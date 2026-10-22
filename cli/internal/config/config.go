package config

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

type Config struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"-"`
	LogLevel  string `json:"log_level"`
}

var skipSetupForCommands = []string{"help", "h", "version", "update", "lint", "exec", "completion", "mcp"}

func Load(ctx context.Context, c *cli.Command) error {
	if firstArg := c.Args().First(); slices.Contains(skipSetupForCommands, firstArg) {
		return nil
	}

	config, err := Get(ctx, c, c.String("config"))
	if err != nil {
		return err
	}

	if config.ServerURL == "" || config.Token == "" {
		fmt.Fprintln(os.Stderr, "crow requires configuration via environment variables (CROW_SERVER & CROW_TOKEN) or command line flags (--server <url> --token <token>)")
		return fmt.Errorf("missing required configuration")
	}

	if !strings.HasPrefix(config.ServerURL, "http://") && !strings.HasPrefix(config.ServerURL, "https://") {
		fmt.Fprintf(os.Stderr, "Server address %q is missing protocol scheme.\n", config.ServerURL)
		fmt.Fprintf(os.Stderr, "Please set CROW_SERVER with http:// or https:// prefix (e.g. CROW_SERVER=https://%s)\n", config.ServerURL)
		return fmt.Errorf("invalid server configuration")
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

	log.Debug().Str("server", config.ServerURL).Msg("using server")

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

func Save(_ context.Context, _ *cli.Command, _ string, _ *Config) error {
	// Config file persistence is not implemented - users should use environment variables
	return nil
}
