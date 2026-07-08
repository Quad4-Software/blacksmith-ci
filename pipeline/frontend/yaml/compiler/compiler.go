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

package compiler

import (
	"fmt"
	"maps"
	"path"
	"slices"

	"github.com/rs/zerolog/log"

	backend_types "codefloe.com/crowci/crow/v6/pipeline/backend/types"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/metadata"
	yaml_types "codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/types"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/utils"
	"codefloe.com/crowci/crow/v6/shared/constant"
)

const (
	defaultCloneName = "clone"
)

// Registry represents registry credentials.
type Registry struct {
	Hostname string
	Username string
	Password string
}

type Secret struct {
	Name           string
	Value          string
	AllowedPlugins []string
	Events         []string
}

func (s *Secret) Available(event string, container *yaml_types.Container) error {
	onlyAllowSecretForPlugins := len(s.AllowedPlugins) > 0
	if onlyAllowSecretForPlugins && !container.IsPlugin() {
		return fmt.Errorf("Secret %q is only allowed to be used by plugins (a filter has been set on the secret). Note: Image filters do not work for normal steps", s.Name)
	}

	if onlyAllowSecretForPlugins && !utils.MatchImageDynamic(container.Image, s.AllowedPlugins...) {
		return fmt.Errorf("Secret %q is not allowed to be used with image %q by step %q", s.Name, container.Image, container.Name)
	}

	if !s.Match(event) {
		return fmt.Errorf("Secret %q is not allowed to be used with pipeline event %q", s.Name, event)
	}

	return nil
}

// Match returns true if an image and event match the restricted list.
// Note that EventPullClosed, EventPullMerged and EventPullEdited are treated as EventPull.
func (s *Secret) Match(event string) bool {
	// if there is no filter set secret matches all webhook events
	if len(s.Events) == 0 {
		return true
	}
	// treat all pull events the same way
	switch event {
	case "pull_request_closed", "pull_request_merged", "pull_request_edited":
		event = "pull_request"
	}
	// one match is enough
	return slices.Contains(s.Events, event)
}

// Compiler compiles the yaml.
type Compiler struct {
	local                   bool
	escalated               []string
	escalatedMatchType      string
	prefix                  string
	volumes                 []string
	tempVolumes             map[string]string
	networks                []string
	env                     map[string]string
	cloneEnv                map[string]string
	workspaceBase           string
	workspacePath           string
	metadata                metadata.Metadata
	registries              []Registry
	secrets                 map[string]Secret
	defaultClonePlugin      string
	trustedClonePlugins     []string
	securityTrustedPipeline bool
	ContainerNameScheme     string // 'hash' or 'descriptive'
	orgID                   int64
	file                    string // config file name for error context
}

// New creates a new Compiler with options.
func New(opts ...Option) *Compiler {
	compiler := &Compiler{
		env:                 map[string]string{},
		cloneEnv:            map[string]string{},
		secrets:             map[string]Secret{},
		defaultClonePlugin:  constant.DefaultClonePlugin,
		trustedClonePlugins: constant.TrustedClonePlugins,
	}
	for _, opt := range opts {
		opt(compiler)
	}
	return compiler
}

// Compile compiles the YAML configuration to the pipeline intermediate
// representation configuration format.
func (c *Compiler) Compile(conf *yaml_types.Workflow) (*backend_types.Config, error) {
	config := new(backend_types.Config)

	if match, err := conf.When.Match(c.metadata, true, c.env); !match && err == nil {
		// This pipeline does not match the configured filter so return an empty config and stop further compilation.
		// An empty pipeline will just be skipped completely.
		return config, nil
	} else if err != nil {
		return nil, err
	}

	// create a default volume
	config.Volume = fmt.Sprintf("%s_default", c.prefix)

	// create a default network
	config.Network = fmt.Sprintf("%s_default", c.prefix)

	c.tempVolumes = make(map[string]string)
	defer func() {
		c.tempVolumes = nil
	}()

	// create secrets for mask
	for _, sec := range c.secrets {
		config.Secrets = append(config.Secrets, &backend_types.Secret{
			Name:  sec.Name,
			Value: sec.Value,
		})
	}

	// overrides the default workspace paths when specified
	// in the YAML file.
	if len(conf.Workspace.Base) != 0 {
		c.workspaceBase = path.Clean(conf.Workspace.Base)
	}
	if len(conf.Workspace.Path) != 0 {
		c.workspacePath = path.Clean(conf.Workspace.Path)
	}

	// Collect all steps for DAG compilation, including clone steps
	allSteps := make([]*dagCompilerStep, 0)

	// add clone steps based on configuration
	switch {
	case !c.local && len(conf.Clone.ContainerList) == 0 && !conf.SkipClone && len(c.defaultClonePlugin) != 0:
		// Default clone with specified plugin
		cloneSettings := map[string]any{"depth": "0"}
		if c.metadata.Curr.Event == metadata.EventTag {
			cloneSettings["tags"] = "true"
		}
		container := &yaml_types.Container{
			Name:        defaultCloneName,
			Image:       c.defaultClonePlugin,
			Settings:    cloneSettings,
			Environment: make(map[string]any),
		}
		for k, v := range c.cloneEnv {
			container.Environment[k] = v
		}
		step, err := c.createProcess(container, conf, backend_types.StepTypeClone)
		if err != nil {
			return nil, err
		}

		allSteps = append(allSteps, &dagCompilerStep{
			step:      step,
			position:  -1, // Clone steps come first
			name:      step.Name,
			dependsOn: nil,
		})
	case !c.local && len(conf.Clone.ContainerList) == 0 && !conf.SkipClone:
		// Fallback: use constant.DefaultClonePlugin if c.defaultClonePlugin is empty
		clonePlugin := c.defaultClonePlugin
		if len(clonePlugin) == 0 {
			clonePlugin = constant.DefaultClonePlugin
		}

		cloneSettings := map[string]any{"depth": "0"}
		if c.metadata.Curr.Event == metadata.EventTag {
			cloneSettings["tags"] = "true"
		}
		container := &yaml_types.Container{
			Name:        defaultCloneName,
			Image:       clonePlugin,
			Settings:    cloneSettings,
			Environment: make(map[string]any),
		}
		for k, v := range c.cloneEnv {
			container.Environment[k] = v
		}
		step, err := c.createProcess(container, conf, backend_types.StepTypeClone)
		if err != nil {
			return nil, err
		}

		allSteps = append(allSteps, &dagCompilerStep{
			step:      step,
			position:  -1, // Clone steps come first
			name:      step.Name,
			dependsOn: nil,
		})
	case !c.local && !conf.SkipClone:
		// Custom clone containers
		for pos, container := range conf.Clone.ContainerList {
			if match, err := container.When.Match(c.metadata, false, c.env); !match && err == nil {
				continue
			} else if err != nil {
				return nil, err
			}

			step, err := c.createProcess(container, conf, backend_types.StepTypeClone)
			if err != nil {
				return nil, err
			}

			// only inject netrc if it's a trusted repo or a trusted plugin
			if c.securityTrustedPipeline || (container.IsPlugin() && container.IsTrustedCloneImage(c.trustedClonePlugins)) {
				maps.Copy(step.Environment, c.cloneEnv)
			}

			allSteps = append(allSteps, &dagCompilerStep{
				step:      step,
				position:  -(pos + 1), // Clone steps come first, negative positions
				name:      step.Name,
				dependsOn: container.DependsOn,
			})
		}
	}

	// Names already used by clone steps. Work step names are deduplicated against
	// this set so that every step in a workflow has a unique name. Uniqueness is
	// required: the DAG compiler keys steps by name (see dag.go), so two steps
	// sharing a name would otherwise silently overwrite each other. It also keeps
	// name-based rerun matching and the UI unambiguous.
	usedStepNames := make(map[string]struct{}, len(allSteps))
	for _, s := range allSteps {
		usedStepNames[s.name] = struct{}{}
	}

	// add pipeline steps
	for pos, container := range conf.Steps.ContainerList {
		// Skip if local and should not run local
		if c.local && !container.When.IsLocal() {
			continue
		}

		if match, err := container.When.Match(c.metadata, false, c.env); !match && err == nil {
			continue
		} else if err != nil {
			return nil, err
		}

		// The first occurrence keeps its name; later duplicates get a deterministic
		// " (N)" suffix. Derived from step order, so reruns reproduce the same names.
		container.Name = uniqueStepName(container.Name, usedStepNames)

		stepType := backend_types.StepTypeCommands
		if container.IsPlugin() {
			stepType = backend_types.StepTypePlugin
		}
		step, err := c.createProcess(container, conf, stepType)
		if err != nil {
			return nil, err
		}

		// only inject netrc if it's a trusted repo or a trusted plugin
		if c.securityTrustedPipeline || (container.IsPlugin() && container.IsTrustedCloneImage(c.trustedClonePlugins)) {
			maps.Copy(step.Environment, c.cloneEnv)
		}

		dependsOn := container.DependsOn
		hasExplicitDependencies := container.DependsOn != nil

		allSteps = append(allSteps, &dagCompilerStep{
			step:                    step,
			position:                pos,
			name:                    container.Name,
			dependsOn:               dependsOn,
			hasExplicitDependencies: hasExplicitDependencies,
		})
	}

	// Check if workflow has any work steps (commands or plugins).
	// If only clone and service steps remain after filtering, skip the workflow entirely.
	if !hasWorkSteps(allSteps) {
		log.Debug().Str("workflow", conf.Name).Msg("workflow has no steps to run after filtering, skipping")
		return config, nil
	}

	// add services steps (only if we have work steps to run)
	if len(conf.Services.ContainerList) != 0 {
		stage := new(backend_types.Stage)

		for _, container := range conf.Services.ContainerList {
			if match, err := container.When.Match(c.metadata, false, c.env); !match && err == nil {
				continue
			} else if err != nil {
				return nil, err
			}

			step, err := c.createProcess(container, conf, backend_types.StepTypeService)
			if err != nil {
				return nil, err
			}

			stage.Steps = append(stage.Steps, step)
		}
		config.Stages = append(config.Stages, stage)
	}

	// generate stages out of all steps (including clone steps)
	allStages, err := newDAGCompiler(allSteps).compile()
	if err != nil {
		return nil, err
	}

	config.Stages = append(config.Stages, allStages...)
	for _, volume := range c.tempVolumes {
		config.TempVolumes = append(config.TempVolumes, volume)
	}
	slices.Sort(config.TempVolumes)

	return config, nil
}

// uniqueStepName returns name if it is not already in used; otherwise it appends
// " (N)" using the lowest N >= 2 that is free. The returned name is recorded in
// used so subsequent calls keep producing unique names.
func uniqueStepName(name string, used map[string]struct{}) string {
	candidate := name
	for i := 2; ; i++ {
		if _, taken := used[candidate]; !taken {
			used[candidate] = struct{}{}
			return candidate
		}
		candidate = fmt.Sprintf("%s (%d)", name, i)
	}
}

// hasWorkSteps checks if the step list contains at least one work step
// (commands or plugin type). Clone and service steps are infrastructure
// and don't constitute actual work to be done.
func hasWorkSteps(steps []*dagCompilerStep) bool {
	for _, step := range steps {
		if step.step.Type == backend_types.StepTypeCommands ||
			step.step.Type == backend_types.StepTypePlugin {
			return true
		}
	}
	return false
}
