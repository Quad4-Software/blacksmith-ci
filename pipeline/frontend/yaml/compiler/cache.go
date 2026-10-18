// Copyright 2026 Crow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package compiler

import (
	"encoding/json"
	"math"

	backend_types "codefloe.com/crowci/crow/v6/pipeline/backend/types"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/metadata"
	yaml_types "codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/types"
)

const (
	// CacheRestoreStepName names the step restoring the workflow's caches.
	CacheRestoreStepName = "restore-cache"
	// CacheSaveStepName names the step saving caches after successful steps.
	CacheSaveStepName = "save-cache"
	// CacheSaveAlwaysStepName names the step saving caches with `save: always`.
	CacheSaveAlwaysStepName = "save-cache-always"

	// The restore step runs after clone; negative positions mark the steps
	// every work step implicitly depends on (see dag.go).
	cacheRestorePosition = math.MinInt32
	cacheSavePosition    = math.MaxInt32
)

// CacheSpec is one cache as handed to the cache step in PLUGIN_CACHES.
type CacheSpec struct {
	Key         string   `json:"key"`
	RestoreKeys []string `json:"restore_keys,omitempty"`
	Paths       []string `json:"paths"`
}

func cacheSpecs(caches []*yaml_types.Cache, include func(*yaml_types.Cache) bool) []CacheSpec {
	specs := make([]CacheSpec, 0, len(caches))
	for _, cache := range caches {
		if cache == nil || !include(cache) {
			continue
		}
		specs = append(specs, CacheSpec{Key: cache.Key, RestoreKeys: cache.RestoreKeys, Paths: cache.Paths})
	}
	return specs
}

// cacheEnabled reports whether the workflow gets cache steps. Local runs have
// no server to store caches, and an empty plugin means caching is disabled.
func (c *Compiler) cacheEnabled(conf *yaml_types.Workflow) bool {
	return !c.local && c.cachePlugin != "" && len(conf.Cache) > 0
}

// cacheStep creates a cache step running the cache plugin in mode for specs.
// Cache steps never fail the workflow: a cache is an optimization, so the
// plugin reports problems as warnings and the step ignores its own failure.
func (c *Compiler) cacheStep(conf *yaml_types.Workflow, name, mode string, specs []CacheSpec) (*backend_types.Step, error) {
	encoded, err := json.Marshal(specs)
	if err != nil {
		return nil, err
	}
	container := &yaml_types.Container{
		Name:  name,
		Image: c.cachePlugin,
		// The plugin runs under a moving tag such as v6. Without a pull, an
		// agent keeps the first image it pulled and never gets plugin fixes. A
		// failed pull only skips the cache, as cache steps never fail.
		Pull: true,
		Environment: map[string]any{
			"PLUGIN_MODE":   mode,
			"PLUGIN_CACHES": string(encoded),
		},
	}
	step, err := c.createProcess(container, conf, backend_types.StepTypeCache)
	if err != nil {
		return nil, err
	}
	step.Failure = metadata.FailureIgnore
	return step, nil
}

// cacheRestoreStep returns the step restoring every cache of the workflow.
func (c *Compiler) cacheRestoreStep(conf *yaml_types.Workflow) (*dagCompilerStep, error) {
	specs := cacheSpecs(conf.Cache, func(*yaml_types.Cache) bool { return true })
	step, err := c.cacheStep(conf, CacheRestoreStepName, "restore", specs)
	if err != nil {
		return nil, err
	}
	return &dagCompilerStep{step: step, position: cacheRestorePosition, name: step.Name, role: roleCacheRestore}, nil
}

// cacheSaveSteps returns the steps saving the caches of the workflow after all
// work steps finished: one for caches saved on success, one for caches saved
// always. A step cannot tell at runtime whether the steps before it failed,
// so the two policies need separate steps.
func (c *Compiler) cacheSaveSteps(conf *yaml_types.Workflow) ([]*dagCompilerStep, error) {
	groups := []struct {
		name   string
		policy yaml_types.CacheSavePolicy
	}{
		{CacheSaveStepName, yaml_types.CacheSaveOnSuccess},
		{CacheSaveAlwaysStepName, yaml_types.CacheSaveAlways},
	}

	var steps []*dagCompilerStep
	for i, group := range groups {
		specs := cacheSpecs(conf.Cache, func(cache *yaml_types.Cache) bool {
			policy := cache.Save
			if policy == "" {
				policy = yaml_types.CacheSaveOnSuccess
			}
			return policy == group.policy
		})
		if len(specs) == 0 {
			continue
		}
		step, err := c.cacheStep(conf, group.name, "save", specs)
		if err != nil {
			return nil, err
		}
		if group.policy == yaml_types.CacheSaveAlways {
			step.OnFailure = true
		}
		steps = append(steps, &dagCompilerStep{step: step, position: cacheSavePosition - len(groups) + i, name: step.Name, role: roleCacheSave})
	}
	return steps, nil
}
