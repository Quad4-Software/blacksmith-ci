// Copyright 2023 Woodpecker Authors
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

package lint

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/common"
	crow_jsonnet "codefloe.com/crowci/crow/v6/pipeline/frontend/jsonnet"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/linter"
	"codefloe.com/crowci/crow/v6/shared/constant"
)

// Command exports the info command.
var Command = &cli.Command{
	Name:      "lint",
	Usage:     "lint a pipeline configuration file",
	ArgsUsage: "[path/to/.crow.yaml | path/to/.crow/]",
	Action:    lint,
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Sources: cli.EnvVars("WOODPECKER_PLUGINS_PRIVILEGED", "CROW_PLUGINS_PRIVILEGED"),
			Name:    "plugins-privileged",
			Usage:   "plugins allowed to run 'privileged'",
		},
		&cli.StringFlag{
			Sources: cli.EnvVars("WOODPECKER_PLUGINS_PRIVILEGED_MATCH_TYPE", "CROW_PLUGINS_PRIVILEGED_MATCH_TYPE"),
			Name:    "plugins-privileged-match-type",
			Usage:   "Type of match for privileged plugins: semver, semver-range, regex, or exact (default: semver)",
			Value:   "semver",
		},
		&cli.StringSliceFlag{
			Sources: cli.EnvVars("WOODPECKER_PLUGINS_TRUSTED_CLONE", "CROW_PLUGINS_TRUSTED_CLONE"),
			Name:    "plugins-trusted-clone",
			Usage:   "plugins that are trusted to handle Git credentials in clone steps",
			Value:   constant.TrustedClonePlugins,
		},
		&cli.BoolFlag{
			Sources: cli.EnvVars("WOODPECKER_LINT_STRICT", "CROW_LINT_STRICT"),
			Name:    "strict",
			Usage:   "treat warnings as errors",
		},
		&cli.StringMapFlag{
			Name:  "jsonnet-ext-var",
			Usage: "set Jsonnet external variables for use with std.extVar() (e.g. --jsonnet-ext-var CI_PIPELINE_EVENT=push)",
		},
	},
}

func lint(ctx context.Context, c *cli.Command) error {
	return common.RunPipelineFunc(ctx, c, lintFile, lintDir)
}

func lintDir(ctx context.Context, c *cli.Command, dir string) error {
	var errorStrings []string
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}

		// check if it is a regular file (not dir)
		if info.Mode().IsRegular() && (strings.HasSuffix(info.Name(), ".yaml") || strings.HasSuffix(info.Name(), ".yml") || strings.HasSuffix(info.Name(), ".jsonnet")) {
			fmt.Println("#", info.Name())
			if err := lintFile(ctx, c, path); err != nil {
				errorStrings = append(errorStrings, err.Error())
			}
			fmt.Println("")
			return nil
		}

		return nil
	}); err != nil {
		return err
	}

	if len(errorStrings) != 0 {
		return fmt.Errorf("ERRORS: %s", strings.Join(errorStrings, "; "))
	}
	return nil
}

// Options configures linting outside of the lint command.
type Options struct {
	Strict                     bool
	PluginsPrivileged          []string
	PluginsPrivilegedMatchType string
	PluginsTrustedClone        []string
	JsonnetExtVars             map[string]string
}

func optionsFromCommand(c *cli.Command) Options {
	return Options{
		Strict:                     c.Bool("strict"),
		PluginsPrivileged:          c.StringSlice("plugins-privileged"),
		PluginsPrivilegedMatchType: c.String("plugins-privileged-match-type"),
		PluginsTrustedClone:        c.StringSlice("plugins-trusted-clone"),
		JsonnetExtVars:             c.StringMap("jsonnet-ext-var"),
	}
}

func isConfigFile(name string) bool {
	return strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".jsonnet")
}

// LintPaths lints config files and directories and returns the report as
// text instead of printing it. The error is non-nil if any file has errors.
func LintPaths(_ context.Context, paths []string, opt Options) (string, error) {
	var (
		b      strings.Builder
		failed []string
	)
	lintOne := func(file string) {
		fmt.Fprintf(&b, "# %s\n", file)
		report, err := lintFileReport(file, opt)
		b.WriteString(report)
		if err != nil {
			if report == "" {
				fmt.Fprintf(&b, "%s\n", err)
			}
			failed = append(failed, file)
		}
		b.WriteString("\n")
	}

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return b.String(), err
		}
		if !info.IsDir() {
			lintOne(p)
			continue
		}
		if err := filepath.Walk(p, func(path string, info os.FileInfo, e error) error {
			if e != nil {
				return e
			}
			if info.Mode().IsRegular() && isConfigFile(info.Name()) {
				lintOne(path)
			}
			return nil
		}); err != nil {
			return b.String(), err
		}
	}

	if len(failed) > 0 {
		return b.String(), fmt.Errorf("config errors in %s", strings.Join(failed, ", "))
	}
	return b.String(), nil
}

func lintFile(_ context.Context, c *cli.Command, file string) error {
	report, err := lintFileReport(file, optionsFromCommand(c))
	fmt.Print(report)
	return err
}

func lintFileReport(file string, opt Options) (string, error) {
	buf, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}

	rawConfig := string(buf)

	// If this is a Jsonnet file, evaluate it first to produce YAML
	if crow_jsonnet.IsJsonnetFile(file) {
		// For CLI linting, load sibling files from the same directory for imports
		dir := filepath.Dir(file)
		siblings, err := common.LoadSiblingFiles(dir, file)
		if err != nil {
			return "", fmt.Errorf("failed to load sibling files for Jsonnet imports: %w", err)
		}

		evaluated, err := crow_jsonnet.Evaluate(file, buf, siblings, opt.JsonnetExtVars)
		if err != nil {
			return "", err
		}

		// Lint each evaluated document
		var report strings.Builder
		for i, doc := range evaluated {
			docName := path.Base(file)
			if len(evaluated) > 1 {
				docName = fmt.Sprintf("%s[%d]", docName, i)
			}
			str, err := lintYAMLConfig(opt, docName, string(doc))
			report.WriteString(str)
			if err != nil {
				return report.String(), err
			}
		}
		return report.String(), nil
	}

	return lintYAMLConfig(opt, path.Base(file), rawConfig)
}

func lintYAMLConfig(opt Options, name, rawConfig string) (string, error) {
	parsedConfig, err := yaml.ParseString(rawConfig)
	if err != nil {
		return "", err
	}

	config := &linter.WorkflowConfig{
		File:      name,
		RawConfig: rawConfig,
		Workflow:  parsedConfig,
	}

	// TODO: lint multiple files at once to allow checks for sth like "depends_on" to work
	err = linter.New(
		linter.WithTrusted(linter.TrustedConfiguration{
			Network:  true,
			Volumes:  true,
			Security: true,
		}),
		linter.PrivilegedPluginsWithMatchType(opt.PluginsPrivileged, opt.PluginsPrivilegedMatchType),
		linter.WithTrustedClonePlugins(opt.PluginsTrustedClone),
		// CLI is for local dev; disable the per-gate timeout cap.
		linter.WithApprovalTimeoutMax(0),
	).Lint([]*linter.WorkflowConfig{config})
	if err != nil {
		return FormatLintError(config.File, err, opt.Strict)
	}

	return "✅ Config is valid\n", nil
}
