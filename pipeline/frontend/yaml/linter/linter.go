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
	"regexp"
	"slices"
	"strings"
	"time"

	pipelineErrors "codefloe.com/crowci/crow/v6/pipeline/errors"
	errorTypes "codefloe.com/crowci/crow/v6/pipeline/errors/types"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/constraint"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/linter/schema"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/types"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/utils"
	"codefloe.com/crowci/crow/v6/shared/cachekey"
	"codefloe.com/crowci/crow/v6/shared/constant"
	"codefloe.com/crowci/crow/v6/shared/yaml"
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

	if err := l.lintStepOrder(config); err != nil {
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
	if err := l.lintCache(config); err != nil {
		linterErr = errors.Join(linterErr, err)
	}

	return linterErr
}

// maxCacheKeyLength matches the width of the cache key column on the server.
const maxCacheKeyLength = 255

func (l *Linter) lintCache(config *WorkflowConfig) error {
	var linterErr error
	addErr := func(field, message string) {
		linterErr = errors.Join(linterErr, newLinterError(message, config.File, field, false))
	}

	keys := make(map[string]int, len(config.Workflow.Cache))
	for i, cache := range config.Workflow.Cache {
		field := fmt.Sprintf("cache[%d]", i)
		if cache == nil {
			addErr(field, "cache entry must not be empty")
			continue
		}

		switch {
		case cache.Key == "":
			addErr(field+".key", "cache entry needs a `key`")
		case len(cache.Key) > maxCacheKeyLength:
			addErr(field+".key", fmt.Sprintf("cache key must not be longer than %d characters", maxCacheKeyLength))
		default:
			if err := cachekey.Parse(cache.Key); err != nil {
				addErr(field+".key", fmt.Sprintf("invalid cache key template: %v", err))
			}
			if first, ok := keys[cache.Key]; ok {
				addErr(field+".key", fmt.Sprintf("cache key is already used by cache[%d]", first))
			} else {
				keys[cache.Key] = i
			}
		}

		for j, restoreKey := range cache.RestoreKeys {
			restoreField := fmt.Sprintf("%s.restore_keys[%d]", field, j)
			if restoreKey == "" {
				addErr(restoreField, "restore key must not be empty")
			} else if err := cachekey.Parse(restoreKey); err != nil {
				addErr(restoreField, fmt.Sprintf("invalid restore key template: %v", err))
			}
		}

		if len(cache.Paths) == 0 {
			addErr(field+".paths", "cache entry needs at least one path")
		}
		for j, p := range cache.Paths {
			if err := cachekey.ValidatePath(p); err != nil {
				addErr(fmt.Sprintf("%s.paths[%d]", field, j), err.Error())
			}
		}

		switch cache.Save {
		case "", types.CacheSaveOnSuccess, types.CacheSaveAlways, types.CacheSaveNever:
		default:
			addErr(field+".save", fmt.Sprintf("invalid save policy %q, must be one of %q, %q or %q",
				cache.Save, types.CacheSaveOnSuccess, types.CacheSaveAlways, types.CacheSaveNever))
		}
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
		if err := l.lintTmpfs(config, container, area); err != nil {
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
	field := fmt.Sprintf("%s.%s.depends_on", area, c.Name)

	// Valid dependency targets are all work steps plus the clone steps. The
	// clone step is implicit (named "clone" by default) unless the user defines
	// custom clone containers, so a `depends_on: [clone]` must not be flagged.
	// The count of each name finds dependencies on a name that several steps
	// share.
	nameCount := make(map[string]int, len(config.Workflow.Steps.ContainerList)+len(config.Workflow.Clone.ContainerList)+1)
	for _, step := range config.Workflow.Steps.ContainerList {
		nameCount[step.Name]++
	}
	switch {
	case len(config.Workflow.Clone.ContainerList) > 0:
		for _, clone := range config.Workflow.Clone.ContainerList {
			nameCount[clone.Name]++
		}
	case !config.Workflow.SkipClone:
		// implicit default clone step
		nameCount[defaultCloneName]++
	case nameCount[defaultCloneName] == 0:
		// keep accepting `depends_on: [clone]` when the clone is skipped
		nameCount[defaultCloneName] = 1
	}

	var linterErr error
	for _, dep := range c.DependsOn {
		switch count := nameCount[dep]; {
		case count == 0:
			linterErr = errors.Join(linterErr, newLinterError(
				"One or more of the specified step dependencies do not exist. Double-check for typos.",
				config.File, field, false,
			))
		case count > 1:
			linterErr = errors.Join(linterErr, newLinterError(
				fmt.Sprintf("%d steps are named `%s`, so this step waits only for the first of them. The others are renamed `%s (2)` and so on. Give each step a unique name.", count, dep, dep),
				config.File, field, true,
			))
		}
	}
	return linterErr
}

// lintStepOrder warns about steps that run in parallel without the author
// asking for it. As soon as one step of a workflow sets `depends_on`, every
// step without it starts right after the clone instead of after the step above
// it. The first step is not flagged, since it starts right after the clone
// either way, and `depends_on: []` states that a step should start right away.
func (l *Linter) lintStepOrder(config *WorkflowConfig) error {
	steps := config.Workflow.Steps.ContainerList
	if !slices.ContainsFunc(steps, func(step *types.Container) bool { return step.DependsOn != nil }) {
		return nil
	}

	var linterErr error
	for _, step := range steps[1:] {
		if step.DependsOn != nil {
			continue
		}
		linterErr = errors.Join(linterErr, newLinterError(
			"This step has no `depends_on` while other steps of the workflow do, so it starts right after the clone instead of after the step above it. Add `depends_on` to wait for other steps, or `depends_on: []` to start it right away.",
			config.File, fmt.Sprintf("steps.%s", step.Name), true,
		))
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
				Docs:  "https://docs.crowci.com/dev/usage/secrets/#defining-secrets",
			},
		})
	}

	return err
}

// tmpfsModeLintPattern matches a 3-4 digit octal permission mode (e.g. "1777").
var tmpfsModeLintPattern = regexp.MustCompile(`^[0-7]{3,4}$`)

// tmpfsSizeLintPattern matches a memory size with an optional unit
// (e.g. "64m", "512Mi", "1G", "1024").
var tmpfsSizeLintPattern = regexp.MustCompile(`^\d+(?:\.\d+)?\s*[kKmMgGtT]?[iI]?[bB]?$`)

// lintTmpfs validates the syntax of each `tmpfs` entry up front, so authoring
// mistakes (an unknown option, a malformed mode or size) are reported at lint
// time instead of surfacing later on the agent when the container is created.
// Only the backend-agnostic syntax is checked here; backends may still reject an
// otherwise-valid entry they cannot honor (e.g. `mode=` on kubernetes).
func (l *Linter) lintTmpfs(config *WorkflowConfig, c *types.Container, area string) error {
	var err error
	for i, entry := range c.Tmpfs {
		field := fmt.Sprintf("%s.%s.tmpfs[%d]", area, c.Name, i)
		path, opts, _ := strings.Cut(entry, ":")
		if path == "" {
			err = errors.Join(err, newLinterError(
				fmt.Sprintf("Invalid tmpfs entry %q: missing mount path.", entry),
				config.File, field, false))
			continue
		}
		for _, opt := range strings.Split(opts, ",") {
			opt = strings.TrimSpace(opt)
			if opt == "" {
				continue
			}
			key, val, found := strings.Cut(opt, "=")
			if !found {
				err = errors.Join(err, newLinterError(
					fmt.Sprintf("Invalid tmpfs option %q: expected `key=value`.", opt),
					config.File, field, false))
				continue
			}
			switch key {
			case "size":
				if !tmpfsSizeLintPattern.MatchString(val) {
					err = errors.Join(err, newLinterError(
						fmt.Sprintf("Invalid tmpfs size %q: expected a value like `64m`, `512Mi` or `1G`.", val),
						config.File, field, false))
				}
			case "mode":
				if !tmpfsModeLintPattern.MatchString(val) {
					err = errors.Join(err, newLinterError(
						fmt.Sprintf("Invalid tmpfs mode %q: expected 3-4 octal digits (e.g. `1777`).", val),
						config.File, field, false))
				}
			default:
				err = errors.Join(err, newLinterError(
					fmt.Sprintf("Unsupported tmpfs option %q: only `size` and `mode` are allowed.", key),
					config.File, field, false))
			}
		}
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

const badHabitEventFilterMessage = "Every `when:` entry should include an `event:` filter, either on the workflow or on each step. Entries are OR'ed, so one without `event:` matches every event."

// hasEventFilter reports whether a when entry restricts the pipeline event,
// either via `event:` or via an `evaluate:` expression on CI_PIPELINE_EVENT.
func hasEventFilter(c constraint.Constraint) bool {
	return len(c.Event) > 0 || strings.Contains(c.Evaluate, "CI_PIPELINE_EVENT")
}

func (l *Linter) lintBadHabits(config *WorkflowConfig) (err error) {
	parsed := new(types.Workflow)
	err = yaml.Unmarshal([]byte(config.RawConfig), parsed)
	if err != nil {
		return err
	}

	rootEventFilters := len(parsed.When.Constraints) > 0
	for _, c := range parsed.When.Constraints {
		if !hasEventFilter(c) {
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
					if !hasEventFilter(c) {
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
					Message: badHabitEventFilterMessage,
					Data: pipelineErrors.BadHabitErrorData{
						File:  config.File,
						Field: field,
						Docs:  "https://docs.crowci.com/dev/usage/workflow-syntax/#event-filters-in-when",
					},
					IsWarning: true,
				})
			}
		}
	}

	return err
}
