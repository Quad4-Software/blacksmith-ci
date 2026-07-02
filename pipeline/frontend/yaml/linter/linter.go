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

package linter

import (
	"errors"
	"fmt"
	"time"

	pipelineErrors "codefloe.com/crowci/crow/v5/pipeline/errors"
	errorTypes "codefloe.com/crowci/crow/v5/pipeline/errors/types"
	"codefloe.com/crowci/crow/v5/pipeline/frontend/yaml/linter/schema"
	"codefloe.com/crowci/crow/v5/pipeline/frontend/yaml/types"
	"codefloe.com/crowci/crow/v5/pipeline/frontend/yaml/utils"
	"codefloe.com/crowci/crow/v5/shared/constant"
	"codefloe.com/crowci/crow/v5/shared/yaml"
)

// defaultApprovalTimeoutMax is the default upper bound on `approval.timeout`
// in a workflow's YAML. It is applied when callers do not pass
// WithApprovalTimeoutMax.
const defaultApprovalTimeoutMax = 7 * 24 * time.Hour

// defaultCloneName is the name of the implicit clone step that the compiler
// injects when no custom clone containers are defined. It must match
// compiler.defaultCloneName so that `depends_on: [clone]` lints correctly.
const defaultCloneName = "clone"

// A Linter lints a pipeline configuration.
type Linter struct {
	trusted                    TrustedConfiguration
	privilegedPlugins          *[]string
	privilegedPluginsMatchType string
	trustedClonePlugins        *[]string
	approvalTimeoutMax         time.Duration
}

type TrustedConfiguration struct {
	Network  bool
	Volumes  bool
	Security bool
}

// New creates a new Linter with options.
func New(opts ...Option) *Linter {
	linter := new(Linter)
	linter.approvalTimeoutMax = defaultApprovalTimeoutMax
	for _, opt := range opts {
		opt(linter)
	}
	return linter
}

type WorkflowConfig struct {
	// File is the path to the configuration file.
	File string

	// RawConfig is the raw configuration.
	RawConfig string

	// Config is the parsed configuration.
	Workflow *types.Workflow
}

// Lint lints the configuration.
func (l *Linter) Lint(configs []*WorkflowConfig) error {
	var linterErr error

	for _, config := range configs {
		if err := l.lintFile(config); err != nil {
			linterErr = errors.Join(linterErr, err)
		}
	}

	return linterErr
}

func (l *Linter) lintFile(config *WorkflowConfig) error {
	var linterErr error

	if len(config.Workflow.Steps.ContainerList) == 0 {
		linterErr = errors.Join(linterErr, newLinterError("Invalid or missing `steps` section", config.File, "steps", false))
	}

	if err := l.lintCloneSteps(config); err != nil {
		linterErr = errors.Join(linterErr, err)
	}

	if err := l.lintContainers(config, "clone"); err != nil {
		linterErr = errors.Join(linterErr, err)
	}
	if err := l.lintContainers(config, "steps"); err != nil {
		linterErr = errors.Join(linterErr, err)
	}
	if err := l.lintContainers(config, "services"); err != nil {
		linterErr = errors.Join(linterErr, err)
	}

	if err := l.lintSchema(config); err != nil {
		linterErr = errors.Join(linterErr, err)
	}
	if err := l.lintDeprecations(config); err != nil {
		linterErr = errors.Join(linterErr, err)
	}
	if err := l.lintBadHabits(config); err != nil {
		linterErr = errors.Join(linterErr, err)
	}
	if err := l.lintApproval(config.File, config.Workflow.Approval); err != nil {
		linterErr = errors.Join(linterErr, err)
	}

	return linterErr
}

func (l *Linter) lintApproval(file string, ap *types.ApprovalConfig) error {
	if ap == nil {
		return nil
	}
	if !ap.Required && ap.Approvers != nil &&
		(len(ap.Approvers.Users) > 0 || len(ap.Approvers.Teams) > 0) {
		return newLinterError("approvers without required: true", file, "approval", false)
	}
	if ap.Timeout != "" {
		d, err := time.ParseDuration(ap.Timeout)
		if err != nil {
			return newLinterError(fmt.Sprintf("invalid timeout: %v", err), file, "approval.timeout", false)
		}
		if d <= 0 {
			return newLinterError("timeout must be positive", file, "approval.timeout", false)
		}
		if l.approvalTimeoutMax > 0 && d > l.approvalTimeoutMax {
			return newLinterError(
				fmt.Sprintf("timeout exceeds max (%s)", l.approvalTimeoutMax),
				file, "approval.timeout", false)
		}
	}
	return nil
}

func (l *Linter) lintCloneSteps(config *WorkflowConfig) error {
	if len(config.Workflow.Clone.ContainerList) == 0 {
		return nil
	}

	trustedClonePlugins := constant.TrustedClonePlugins
	if l.trustedClonePlugins != nil {
		trustedClonePlugins = *l.trustedClonePlugins
	}

	var linterErr error
	for _, container := range config.Workflow.Clone.ContainerList {
		if !utils.MatchImageDynamic(container.Image, trustedClonePlugins...) {
			linterErr = errors.Join(linterErr,
				newLinterError(
					"The specified clone image does not match the images in the allow list (`CROW_PLUGINS_TRUSTED_CLONE`), not injecting `.netrc` credentials.",
					config.File, fmt.Sprintf("clone.%s", container.Name), true),
			)
		}
	}
	return linterErr
}

func (l *Linter) lintContainers(config *WorkflowConfig, area string) error {
	var linterErr error

	var containers []*types.Container

	switch area {
	case "clone":
		containers = config.Workflow.Clone.ContainerList
	case "steps":
		containers = config.Workflow.Steps.ContainerList
	case "services":
		containers = config.Workflow.Services.ContainerList
	}

	for _, container := range containers {
		if err := l.lintImage(config, container, area); err != nil {
			linterErr = errors.Join(linterErr, err)
		}
		if err := l.lintTrusted(config, container, area); err != nil {
			linterErr = errors.Join(linterErr, err)
		}
		if err := l.lintSettings(config, container, area); err != nil {
			linterErr = errors.Join(linterErr, err)
		}
		if err := l.lintPrivilegedPlugins(config, container, area); err != nil {
			linterErr = errors.Join(linterErr, err)
		}
		if err := l.lintContainerDeprecations(config, container, area); err != nil {
			linterErr = errors.Join(linterErr, err)
		}
		if err := l.lintPluginRestrictions(config, container, area); err != nil {
			linterErr = errors.Join(linterErr, err)
		}
		if err := l.lintDependsOn(config, container, area); err != nil {
			linterErr = errors.Join(linterErr, err)
		}
	}

	return linterErr
}

// isUsedAsPlugin reports whether a step is a plugin or is meant to be run as
// one, independent of the `environment:`/`secrets:` fields that
// Container.IsPlugin() also inspects. A step is treated as a plugin when it
// carries plugin `settings:` (which mandate plugin execution) or when its image
// is on one of the plugin allow lists (privileged plugins or trusted clone
// plugins). We deliberately do NOT consider `environment:`/`secrets:` here so
// that we can surface the conflict below instead of silently downgrading the
// step to a non-plugin.
func (l *Linter) isUsedAsPlugin(c *types.Container) bool {
	if len(c.Settings) > 0 {
		return true
	}
	if l.privilegedPlugins != nil && utils.MatchImageWithType(c.Image, l.privilegedPluginsMatchType, *l.privilegedPlugins...) {
		return true
	}
	trustedClonePlugins := constant.TrustedClonePlugins
	if l.trustedClonePlugins != nil {
		trustedClonePlugins = *l.trustedClonePlugins
	}
	return utils.MatchImageDynamic(c.Image, trustedClonePlugins...)
}

// lintPluginRestrictions hard-errors when a step is used as a plugin but also
// sets `environment:` or `secrets:`. Such a step would silently stop being
// treated as a plugin (see Container.IsPlugin): it would lose plugin-restricted
// secret access and would not be escalated to privileged even if its image is
// on the privileged/escalated allow list. Surfacing this as an explicit lint
// error avoids the silent semantic change.
func (l *Linter) lintPluginRestrictions(config *WorkflowConfig, c *types.Container, area string) error {
	if !l.isUsedAsPlugin(c) {
		return nil
	}

	var linterErr error
	yamlPath := fmt.Sprintf("%s.%s", area, c.Name)
	if len(c.Environment) != 0 {
		linterErr = errors.Join(linterErr, newLinterError(
			"Plugin steps may not set `environment:`. Pass configuration via `settings:` instead. A plugin with `environment:` set is not treated as a plugin and is neither granted plugin-restricted secret access nor escalated to privileged mode.",
			config.File, yamlPath, false,
		))
	}
	if len(c.Secrets) != 0 {
		linterErr = errors.Join(linterErr, newLinterError(
			"Plugin steps may not set `secrets:`. Pass secrets via `settings:` with `from_secret:` instead. A plugin with `secrets:` set is not treated as a plugin and is neither granted plugin-restricted secret access nor escalated to privileged mode.",
			config.File, yamlPath, false,
		))
	}
	return linterErr
}

func (l *Linter) lintDependsOn(config *WorkflowConfig, c *types.Container, area string) error {
	if area != "steps" {
		return nil
	}

	// Valid dependency targets are all work steps plus the clone steps. The
	// clone step is implicit (named "clone" by default) unless the user defines
	// custom clone containers, so a `depends_on: [clone]` must not be flagged.
	validDeps := make(map[string]struct{}, len(config.Workflow.Steps.ContainerList)+len(config.Workflow.Clone.ContainerList)+1)
	for _, step := range config.Workflow.Steps.ContainerList {
		validDeps[step.Name] = struct{}{}
	}
	if len(config.Workflow.Clone.ContainerList) > 0 {
		for _, clone := range config.Workflow.Clone.ContainerList {
			validDeps[clone.Name] = struct{}{}
		}
	} else {
		// implicit default clone step
		validDeps[defaultCloneName] = struct{}{}
	}

	var linterErr error
	for _, dep := range c.DependsOn {
		if _, ok := validDeps[dep]; ok {
			continue
		}
		linterErr = errors.Join(linterErr,
			newLinterError(
				"One or more of the specified step dependencies do not exist. Double-check for typos.",
				config.File, fmt.Sprintf("%s.%s.depends_on", area, c.Name), false,
			),
		)
	}
	return linterErr
}

func (l *Linter) lintImage(config *WorkflowConfig, c *types.Container, area string) error {
	if len(c.Image) == 0 {
		return newLinterError("Invalid or missing image", config.File, fmt.Sprintf("%s.%s", area, c.Name), false)
	}
	return nil
}

func (l *Linter) lintPrivilegedPlugins(config *WorkflowConfig, c *types.Container, area string) error {
	// lint for conflicts of https://github.com/woodpecker-ci/woodpecker/pull/3918
	if utils.MatchImage(c.Image, "plugins/docker", "plugins/gcr", "plugins/ecr", "woodpeckerci/plugin-docker-buildx") {
		if l.privilegedPlugins != nil && !utils.MatchImageWithType(c.Image, l.privilegedPluginsMatchType, *l.privilegedPlugins...) {
			msg := fmt.Sprintf(
				"The plugin image '%s' is not allowed to run in privileged mode.\n"+
					"Required pattern(s): %v\n"+
					"Match type: '%s'.\n"+
					"Only plugins set via the admin env var `CROW_PLUGINS_PRIVILEGED` can run in privileged mode.",
				c.Image, *l.privilegedPlugins, l.privilegedPluginsMatchType,
			)
			return newLinterError(msg, config.File, fmt.Sprintf("%s.%s", area, c.Name), false)
		} else if l.privilegedPlugins == nil {
			msg := fmt.Sprintf(
				"The formerly privileged plugin '%s' is no longer privileged by default since v3. To use it, add it to (server) env var `CROW_PLUGINS_PRIVILEGED` (must be done by an instance administrator).",
				c.Image,
			)
			// if linter has no info of current privileged plugins, it's just a warning
			return newLinterError(msg, config.File, fmt.Sprintf("%s.%s", area, c.Name), true)
		}
	}

	return nil
}

func (l *Linter) lintSettings(config *WorkflowConfig, c *types.Container, field string) error {
	if len(c.Settings) == 0 {
		return nil
	}
	if len(c.Commands) != 0 {
		return newLinterError("Using both `commands:` and `settings:` at the same time is not allowed.", config.File, fmt.Sprintf("%s.%s", field, c.Name), false)
	}
	if len(c.Entrypoint) != 0 {
		return newLinterError("Using both `entrypoint:` and `settings:` at the same time is not allowed.", config.File, fmt.Sprintf("%s.%s", field, c.Name), false)
	}
	if len(c.Environment) != 0 {
		return newLinterError("Using both `environment:` and `settings:` at the same time is not allowed.", config.File, fmt.Sprintf("%s.%s", field, c.Name), false)
	}
	return nil
}

func (l *Linter) lintContainerDeprecations(config *WorkflowConfig, c *types.Container, field string) (err error) {
	if len(c.Secrets) != 0 {
		err = errors.Join(err, &errorTypes.PipelineError{
			Type:    errorTypes.PipelineErrorTypeDeprecation,
			Message: "The usage of `secrets:` is deprecated since v2.8.x and was removed in >=3.x. Use `environment:` in combination with `from_secret:`.",
			Data: pipelineErrors.DeprecationErrorData{
				File:  config.File,
				Field: fmt.Sprintf("%s.%s.secrets", field, c.Name),
				Docs:  "https://crowci.dev/dev/usage/secrets/#defining-secrets",
			},
		})
	}

	return err
}

func (l *Linter) lintTrusted(config *WorkflowConfig, c *types.Container, area string) error {
	yamlPath := fmt.Sprintf("%s.%s", area, c.Name)
	type trustedMsg struct {
		msg     string
		trusted string
	}
	var msgs []trustedMsg
	if !l.trusted.Security {
		if c.Privileged {
			msgs = append(msgs, trustedMsg{"Insufficient `security` privileges. To allow, check the 'security' option in the 'Privileges & Security' repository settings.", "security"})
		}
	}
	if !l.trusted.Network {
		if len(c.DNS) != 0 {
			msgs = append(msgs, trustedMsg{"Insufficient `network` privileges. To allow using 'dns' options, check the 'Network' option in the 'Privileges & Security' repository settings.", "network"})
		}
		if len(c.DNSSearch) != 0 {
			msgs = append(msgs, trustedMsg{"Insufficient `network` privileges. To allow using 'dns_search' options, check the 'Network' option in the 'Privileges & Security' repository settings.", "network"})
		}
		if len(c.ExtraHosts) != 0 {
			msgs = append(msgs, trustedMsg{"Insufficient `network` privileges. To allow using 'extra_hosts' options, check the 'Network' option in the 'Privileges & Security' repository settings.", "network"})
		}
		if len(c.NetworkMode) != 0 {
			msgs = append(msgs, trustedMsg{"Insufficient `network` privileges. To allow using 'network_mode' options, check the 'Network' option in the 'Privileges & Security' repository settings.", "network"})
		}
	}
	if !l.trusted.Volumes {
		if len(c.Devices) != 0 {
			msgs = append(msgs, trustedMsg{"Insufficient `volumes` privileges. To allow using 'devices' options, check the 'Volumes' option in the 'Privileges & Security' repository settings.", "volumes"})
		}
		if len(c.Volumes.Volumes) != 0 {
			msgs = append(msgs, trustedMsg{"Insufficient `volumes` privileges. To allow using 'volumes' options, check the 'Volumes' option in the 'Privileges & Security' repository settings.", "volumes"})
		}
	}

	if len(msgs) > 0 {
		var err error
		for _, e := range msgs {
			err = errors.Join(err, newTrustedLinterError(e.msg, config.File, yamlPath, e.trusted))
		}
		return err
	}

	return nil
}

func (l *Linter) lintSchema(config *WorkflowConfig) error {
	var linterErr error
	schemaErrors, err := schema.LintString(config.RawConfig)
	if err != nil {
		for _, schemaError := range schemaErrors {
			linterErr = errors.Join(linterErr, newLinterErrorWithLocation(
				schemaError.Description(),
				config.File,
				schemaError.Field(),
				schemaError.Line(),
				schemaError.Column(),
				true, // TODO: let pipelines fail if the schema is invalid
			))
		}
	}
	return linterErr
}

func (l *Linter) lintDeprecations(config *WorkflowConfig) (err error) {
	parsed := new(types.Workflow)
	err = yaml.Unmarshal([]byte(config.RawConfig), parsed)
	if err != nil {
		return err
	}

	return nil
}

func (l *Linter) lintBadHabits(config *WorkflowConfig) (err error) {
	parsed := new(types.Workflow)
	err = yaml.Unmarshal([]byte(config.RawConfig), parsed)
	if err != nil {
		return err
	}

	rootEventFilters := len(parsed.When.Constraints) > 0
	for _, c := range parsed.When.Constraints {
		if len(c.Event) == 0 {
			rootEventFilters = false
			break
		}
	}
	if !rootEventFilters {
		// root whens do not necessarily have an event filter, check steps
		for _, step := range parsed.Steps.ContainerList {
			var field string
			if len(step.When.Constraints) == 0 {
				field = fmt.Sprintf("steps.%s", step.Name)
			} else {
				stepEventIndex := -1
				for i, c := range step.When.Constraints {
					if len(c.Event) == 0 {
						stepEventIndex = i
						break
					}
				}
				if stepEventIndex > -1 {
					field = fmt.Sprintf("steps.%s.when[%d]", step.Name, stepEventIndex)
				}
			}
			if field != "" {
				err = errors.Join(err, &errorTypes.PipelineError{
					Type:    errorTypes.PipelineErrorTypeBadHabit,
					Message: "Either set an event filter (`when:`) for all individual steps or for the entire workflow.",
					Data: pipelineErrors.BadHabitErrorData{
						File:  config.File,
						Field: field,
						Docs:  "https://crowci.dev/dev/usage/workflow-syntax/#when---conditional-execution",
					},
					IsWarning: true,
				})
			}
		}
	}

	return err
}
