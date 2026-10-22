// Copyright 2026 Crow CI Authors
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

// Package mcp provides the "crow mcp" command, a Model Context Protocol server
// over stdio for debugging pipelines from coding agents.
package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/lint"
	crowmcp "codefloe.com/crowci/crow/v6/crow-go/mcp"
	"codefloe.com/crowci/crow/v6/shared/constant"
	"codefloe.com/crowci/crow/v6/version"
)

// Command exports the mcp command.
var Command = &cli.Command{
	Name:  "mcp",
	Usage: "run a Model Context Protocol server over stdio for coding agents",
	Description: `Serves tools to find, read and wait on pipelines across one or more Crow instances.
Instances come from the config file; without one, --server and --token are used.`,
	Action: run,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Sources: cli.EnvVars("CROW_MCP_CONFIG"),
			Name:    "config",
			Usage:   "path to the instances config file (default: $XDG_CONFIG_HOME/crow/mcp.yaml)",
		},
		&cli.BoolFlag{
			Sources: cli.EnvVars("CROW_MCP_ALLOW_WRITE"),
			Name:    "allow-write",
			Usage:   "register tools that restart or cancel pipelines",
		},
	},
}

func defaultConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "crow", "mcp.yaml")
}

// loadConfig reads the config file, falling back to a single instance from
// the CLI's server and token.
func loadConfig(path, server, token string) (*crowmcp.Config, error) {
	cfg, err := crowmcp.LoadConfig(path)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		return cfg, nil
	}
	if server == "" || token == "" {
		return nil, fmt.Errorf("no instances: create %s or set CROW_SERVER and CROW_TOKEN", path)
	}
	return &crowmcp.Config{Instances: []crowmcp.Instance{crowmcp.NewStaticInstance(strings.TrimRight(server, "/"), token)}}, nil
}

// confinedLinter lints config files below root only, so the tool cannot be
// used to read arbitrary files through parse error snippets.
func confinedLinter(root string) crowmcp.Linter {
	return func(ctx context.Context, paths []string) (string, error) {
		absRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			return "", err
		}
		resolved := make([]string, 0, len(paths))
		for _, p := range paths {
			if !filepath.IsAbs(p) {
				p = filepath.Join(absRoot, p)
			}
			real, err := filepath.EvalSymlinks(p)
			if err != nil {
				return "", fmt.Errorf("lint path %q: %w", p, err)
			}
			rel, err := filepath.Rel(absRoot, real)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return "", fmt.Errorf("lint path %q is outside the working directory", p)
			}
			info, err := os.Stat(real)
			if err != nil {
				return "", err
			}
			if !info.IsDir() && !isConfigFile(real) {
				return "", fmt.Errorf("lint path %q is not a .yaml, .yml or .jsonnet file", p)
			}
			resolved = append(resolved, real)
		}
		return lint.LintPaths(ctx, resolved, lint.Options{
			PluginsPrivilegedMatchType: "semver",
			PluginsTrustedClone:        constant.TrustedClonePlugins,
		})
	}
}

func isConfigFile(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".yaml" || ext == ".yml" || ext == ".jsonnet"
}

func run(ctx context.Context, c *cli.Command) error {
	path := c.String("config")
	if path == "" {
		path = defaultConfigPath()
	}
	cfg, err := loadConfig(path, c.String("server"), c.String("token"))
	if err != nil {
		return err
	}
	reg, err := crowmcp.NewRegistry(cfg, crowmcp.DefaultHTTPClientFactory(c.Bool("skip-verify")))
	if err != nil {
		return err
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	srv := crowmcp.NewServer(reg, crowmcp.Options{
		AllowWrite: c.Bool("allow-write"),
		Discoverer: gitDiscoverer{dir: wd},
		Linter:     confinedLinter(wd),
		Version:    version.String(),
	})
	return srv.Run(ctx, &mcp.StdioTransport{})
}
