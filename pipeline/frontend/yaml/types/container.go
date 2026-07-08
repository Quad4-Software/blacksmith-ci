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

package types

import (
	"fmt"

	"github.com/goccy/go-yaml"

	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/constraint"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/types/base"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/utils"
)

type (
	// ContainerList denotes an ordered collection of containers.
	ContainerList struct {
		ContainerList []*Container
	}

	// Container defines a container.
	Container struct {
		// common
		Name       string             `yaml:"name,omitempty"`
		Image      string             `yaml:"image,omitempty"`
		Pull       bool               `yaml:"pull,omitempty"`
		Commands   base.StringOrSlice `yaml:"commands,omitempty"`
		Entrypoint base.StringOrSlice `yaml:"entrypoint,omitempty"`
		Directory  string             `yaml:"directory,omitempty"`
		Settings   map[string]any     `yaml:"settings"`
		// flow control
		DependsOn base.StringOrSlice `yaml:"depends_on,omitempty"`
		When      constraint.When    `yaml:"when,omitempty"`
		Failure   string             `yaml:"failure,omitempty"`
		Detached  bool               `yaml:"detach,omitempty"`
		// state
		Volumes     Volumes `yaml:"volumes,omitempty"`
		TempVolumes Volumes `yaml:"temp_volumes,omitempty"`
		// network
		Ports     []string           `yaml:"ports,omitempty"`
		DNS       base.StringOrSlice `yaml:"dns,omitempty"`
		DNSSearch base.StringOrSlice `yaml:"dns_search,omitempty"`
		// backend specific
		BackendOptions map[string]any `yaml:"backend_options,omitempty"`

		// ACTIVE DEVELOPMENT BELOW

		// TODO: remove base.EnvironmentMap and use map[string]any after v3.0.0 release
		Environment base.EnvironmentMap `yaml:"environment,omitempty"`

		// Remove after v3.1.0
		Secrets []any `yaml:"secrets,omitempty"`

		// Docker and Kubernetes Specific
		Privileged bool `yaml:"privileged,omitempty"`

		// Undocumented
		Devices     []string `yaml:"devices,omitempty"`
		ExtraHosts  []string `yaml:"extra_hosts,omitempty"`
		NetworkMode string   `yaml:"network_mode,omitempty"`
		Tmpfs       []string `yaml:"tmpfs,omitempty"`
	}
)

// UnmarshalYAML implements the Unmarshaler interface.
func (c *ContainerList) UnmarshalYAML(unmarshal func(any) error) error {
	// First decode into any to detect the type
	var raw any
	if err := unmarshal(&raw); err != nil {
		return fmt.Errorf("could not unmarshal container list: %w", err)
	}

	switch v := raw.(type) {
	case map[string]any:
		// It's a map - use yaml.MapSlice to preserve order
		var mapSlice yaml.MapSlice
		if err := unmarshal(&mapSlice); err != nil {
			return fmt.Errorf("could not unmarshal container map: %w", err)
		}

		c.ContainerList = make([]*Container, 0, len(mapSlice))
		for _, item := range mapSlice {
			// Re-marshal to YAML to trigger custom unmarshalers
			valueBytes, err := yaml.Marshal(item.Value)
			if err != nil {
				return err
			}
			container := &Container{}
			if err := yaml.Unmarshal(valueBytes, container); err != nil {
				return err
			}

			if container.Name == "" {
				container.Name = fmt.Sprintf("%v", item.Key)
			}

			c.ContainerList = append(c.ContainerList, container)
		}
		return nil

	case []any:
		// It's a list
		c.ContainerList = make([]*Container, 0, len(v))
		for i, item := range v {
			// Re-marshal to YAML to trigger custom unmarshalers
			itemBytes, err := yaml.Marshal(item)
			if err != nil {
				return err
			}
			container := &Container{}
			if err := yaml.Unmarshal(itemBytes, container); err != nil {
				return err
			}

			if container.Name == "" {
				container.Name = fmt.Sprintf("step-%d", i)
			}

			c.ContainerList = append(c.ContainerList, container)
		}
		return nil

	default:
		return fmt.Errorf("container list must be a map or sequence, got %T", raw)
	}
}

func (c *Container) IsPlugin() bool {
	return len(c.Commands) == 0 &&
		len(c.Entrypoint) == 0 &&
		len(c.Environment) == 0 &&
		len(c.Secrets) == 0
}

func (c *Container) IsTrustedCloneImage(trustedClonePlugins []string) bool {
	return c.IsPlugin() && utils.MatchImageDynamic(c.Image, trustedClonePlugins...)
}
