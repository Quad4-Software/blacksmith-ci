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
	"path"

	backend_types "codeberg.org/crowci/crow/v3/pipeline/backend/types"
	"codeberg.org/crowci/crow/v3/pipeline/frontend/metadata"
	yaml_types "codeberg.org/crowci/crow/v3/pipeline/frontend/yaml/types"
	"codeberg.org/crowci/crow/v3/pipeline/frontend/yaml/utils"
	"codeberg.org/crowci/crow/v3/shared/constant"
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
		return fmt.Errorf("secret %q is only allowed to be used by plugins (a filter has been set on the secret). Note: Image filters do not work for normal steps", s.Name)
	}

	if onlyAllowSecretForPlugins && !utils.MatchImageDynamic(container.Image, s.AllowedPlugins...) {
		return fmt.Errorf("secret %q is not allowed to be used with image %q by step %q", s.Name, container.Image, container.Name)
	}

	if !s.Match(event) {
		return fmt.Errorf("secret %q is not allowed to be used with pipeline event %q", s.Name, event)
	}

	return nil
}

// Match returns true if an image and event match the restricted list.
// Note that EventPullClosed and EventPullEdited are treated as EventPull.
func (s *Secret) Match(event string) bool {
	// if there is no filter set secret matches all webhook events
	if len(s.Events) == 0 {
		return true
	}
	// treat all pull events the same way
	if event == "pull_request_closed" {
		event = "pull_request"
	}
	if event == "pull_request_edited" {
		event = "pull_request"
	}
	// one match is enough
	for _, e := range s.Events {
		if e == event {
			return true
		}
	}
	// a filter is set but the webhook did not match it
	return false
}

// Compiler compiles the yaml.
type Compiler struct {
	local                   bool
	escalated               []string
	escalatedMatchType      string
	prefix                  string
	volumes                 []string
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
	config.Volume = &backend_types.Volume{
		Name: fmt.Sprintf("%s_default", c.prefix),
	}

	// create a default network
	config.Network = &backend_types.Network{
		Name: fmt.Sprintf("%s_default", c.prefix),
	}

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
	cloneStepNames := make([]string, 0)

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
		step, err := c.createProcess(container, backend_types.StepTypeClone)
		if err != nil {
			return nil, err
		}

		allSteps = append(allSteps, &dagCompilerStep{
			step:      step,
			position:  -1, // Clone steps come first
			name:      step.Name,
			dependsOn: nil,
		})
		cloneStepNames = append(cloneStepNames, step.Name)
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
		step, err := c.createProcess(container, backend_types.StepTypeClone)
		if err != nil {
			return nil, err
		}

		allSteps = append(allSteps, &dagCompilerStep{
			step:      step,
			position:  -1, // Clone steps come first
			name:      step.Name,
			dependsOn: nil,
		})
		cloneStepNames = append(cloneStepNames, step.Name)
	case !c.local && !conf.SkipClone:
		// Custom clone containers
		for pos, container := range conf.Clone.ContainerList {
			if match, err := container.When.Match(c.metadata, false, c.env); !match && err == nil {
				continue
			} else if err != nil {
				return nil, err
			}

			step, err := c.createProcess(container, backend_types.StepTypeClone)
			if err != nil {
				return nil, err
			}

			// only inject netrc if it's a trusted repo or a trusted plugin
			if c.securityTrustedPipeline || (container.IsPlugin() && container.IsTrustedCloneImage(c.trustedClonePlugins)) {
				for k, v := range c.cloneEnv {
					step.Environment[k] = v
				}
			}

			allSteps = append(allSteps, &dagCompilerStep{
				step:      step,
				position:  -(pos + 1), // Clone steps come first, negative positions
				name:      step.Name,
				dependsOn: container.DependsOn,
			})
			cloneStepNames = append(cloneStepNames, step.Name)
		}
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

		stepType := backend_types.StepTypeCommands
		if container.IsPlugin() {
			stepType = backend_types.StepTypePlugin
		}
		step, err := c.createProcess(container, stepType)
		if err != nil {
			return nil, err
		}

		// only inject netrc if it's a trusted repo or a trusted plugin
		if c.securityTrustedPipeline || (container.IsPlugin() && container.IsTrustedCloneImage(c.trustedClonePlugins)) {
			for k, v := range c.cloneEnv {
				step.Environment[k] = v
			}
		}

		dependsOn := container.DependsOn
		// Add implicit clone dependency for all steps without explicit depends_on if clone steps exist
		if len(dependsOn) == 0 && len(cloneStepNames) > 0 {
			dependsOn = cloneStepNames
		}

		// Set the DependsOn field on the backend_types.Step as well
		step.DependsOn = dependsOn

		allSteps = append(allSteps, &dagCompilerStep{
			step:      step,
			position:  pos,
			name:      container.Name,
			dependsOn: dependsOn,
		})
	}

	// add services steps
	if len(conf.Services.ContainerList) != 0 {
		stage := new(backend_types.Stage)

		for _, container := range conf.Services.ContainerList {
			if match, err := container.When.Match(c.metadata, false, c.env); !match && err == nil {
				continue
			} else if err != nil {
				return nil, err
			}

			step, err := c.createProcess(container, backend_types.StepTypeService)
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

	return config, nil
}
