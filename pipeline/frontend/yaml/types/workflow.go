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
	"strings"

	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/constraint"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/types/base"
)

type (
	// Workflow defines a workflow configuration.
	Workflow struct {
		Name      string          `yaml:"name,omitempty"`
		When      constraint.When `yaml:"when,omitempty"`
		Workspace Workspace       `yaml:"workspace,omitempty"`
		Clone     ContainerList   `yaml:"clone,omitempty"`
		Steps     ContainerList   `yaml:"steps,omitempty"`
		Services  ContainerList   `yaml:"services,omitempty"`
		Labels    Labels          `yaml:"labels,omitempty"`
		DependsOn []string        `yaml:"depends_on,omitempty"`
		RunsOn    []string        `yaml:"runs_on,omitempty"`
		SkipClone bool            `yaml:"skip_clone"`
		Notify    *NotifyConfig   `yaml:"notify,omitempty"`
		Approval  *ApprovalConfig `yaml:"approval,omitempty"`
		Cache     []*Cache        `yaml:"cache,omitempty"`
	}

	// Cache declares workspace paths restored before the steps run and saved
	// after them, under a key that may be built from file checksums.
	Cache struct {
		Key         string             `yaml:"key"`
		RestoreKeys base.StringOrSlice `yaml:"restore_keys,omitempty"`
		Paths       base.StringOrSlice `yaml:"paths"`
		Save        CacheSavePolicy    `yaml:"save,omitempty"`
	}

	// CacheSavePolicy decides when a cache is saved.
	CacheSavePolicy string

	// Labels maps a label key to one or more values. Each value may be given
	// as a scalar (`agent: name`) or a list (`agent: [a, b]`), the latter
	// meaning "match any of these". Values are flattened to the pipe-separated
	// encoding understood by the agent filter (see Flatten).
	Labels map[string]base.StringOrSlice

	// NotifyConfig allows per-pipeline notification overrides in .crow.yml.
	NotifyConfig struct {
		Disabled bool     `yaml:"disabled,omitempty"`
		Events   []string `yaml:"events,omitempty"`
	}

	// ApprovalConfig declares a workflow-level approval gate that blocks the
	// workflow from running until an authorized user approves it.
	ApprovalConfig struct {
		Required  bool               `yaml:"required,omitempty"`
		Approvers *ApprovalApprovers `yaml:"approvers,omitempty"`
		Timeout   string             `yaml:"timeout,omitempty"` // Go duration; "24h", "30m", etc.
	}

	// ApprovalApprovers names the users and teams permitted to approve.
	// Names are resolved against the repo's forge at approval time.
	ApprovalApprovers struct {
		Users []string `yaml:"users,omitempty"`
		Teams []string `yaml:"teams,omitempty"`
	}

	// Workspace defines a pipeline workspace.
	Workspace struct {
		Base string
		Path string
	}
)

const (
	// CacheSaveOnSuccess saves the cache only when every step succeeded (default).
	CacheSaveOnSuccess CacheSavePolicy = "on_success"
	// CacheSaveAlways saves the cache whether the steps succeeded or failed.
	CacheSaveAlways CacheSavePolicy = "always"
	// CacheSaveNever only restores the cache.
	CacheSaveNever CacheSavePolicy = "never"
)

// Flatten converts the label map into the pipe-separated string encoding used
// throughout the queue and agent filter. A list value like `[a, b]` becomes
// `a|b`; a scalar stays as-is. Empty values are preserved (the filter ignores
// them), keeping behavior identical for single-value labels.
func (l Labels) Flatten() map[string]string {
	if l == nil {
		return nil
	}
	out := make(map[string]string, len(l))
	for key, value := range l {
		out[key] = strings.Join([]string(value), "|")
	}
	return out
}
