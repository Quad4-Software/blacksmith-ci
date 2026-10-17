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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backend_types "codefloe.com/crowci/crow/v6/pipeline/backend/types"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/metadata"
	yaml_types "codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/types"
)

const testCachePlugin = "example.com/crow-cache:test"

func cacheTestCompiler(opts ...Option) *Compiler {
	repoURL := "https://example.com/test/repo"
	return New(append([]Option{
		WithMetadata(metadata.Metadata{Repo: metadata.Repo{Owner: "test", Name: "repo", ForgeURL: repoURL}}),
		WithPrefix("test"),
		WithWorkspaceFromURL("/crow", repoURL),
		WithCachePlugin(testCachePlugin),
	}, opts...)...)
}

func commandStep(name string, dependsOn ...string) *yaml_types.Container {
	return &yaml_types.Container{Name: name, Image: "alpine", Commands: []string{"true"}, DependsOn: dependsOn}
}

func cacheWorkflow(caches []*yaml_types.Cache, steps ...*yaml_types.Container) *yaml_types.Workflow {
	return &yaml_types.Workflow{Cache: caches, Steps: yaml_types.ContainerList{ContainerList: steps}}
}

var goCache = &yaml_types.Cache{Key: `go-{{ checksum "go.sum" }}`, RestoreKeys: []string{"go-"}, Paths: []string{".cache/go"}}

// stepsByName flattens the compiled stages.
func stepsByName(t *testing.T, config *backend_types.Config) ([]string, map[string]*backend_types.Step) {
	t.Helper()
	var names []string
	byName := map[string]*backend_types.Step{}
	for _, stage := range config.Stages {
		for _, step := range stage.Steps {
			names = append(names, step.Name)
			byName[step.Name] = step
		}
	}
	return names, byName
}

func TestCompileCacheSequence(t *testing.T) {
	t.Parallel()

	config, err := cacheTestCompiler().Compile(cacheWorkflow([]*yaml_types.Cache{goCache}, commandStep("build"), commandStep("test")))
	require.NoError(t, err)

	names, steps := stepsByName(t, config)
	assert.Equal(t, []string{"clone", "restore-cache", "build", "test", "save-cache"}, names)

	restore := steps["restore-cache"]
	assert.Equal(t, backend_types.StepTypeCache, restore.Type)
	assert.Equal(t, testCachePlugin, restore.Image)
	assert.Equal(t, []string{"clone"}, restore.DependsOn)
	assert.Equal(t, "ignore", restore.Failure)
	assert.Equal(t, "restore", restore.Environment["PLUGIN_MODE"])
	var specs []CacheSpec
	require.NoError(t, json.Unmarshal([]byte(restore.Environment["PLUGIN_CACHES"]), &specs))
	assert.Equal(t, []CacheSpec{{Key: goCache.Key, RestoreKeys: []string{"go-"}, Paths: []string{".cache/go"}}}, specs)

	assert.Equal(t, []string{"restore-cache"}, steps["build"].DependsOn)
	assert.Equal(t, []string{"build"}, steps["test"].DependsOn)

	save := steps["save-cache"]
	assert.Equal(t, "save", save.Environment["PLUGIN_MODE"])
	assert.Equal(t, []string{"build", "test"}, save.DependsOn)
	assert.True(t, save.OnSuccess)
	assert.False(t, save.OnFailure, "on_success caches are not saved after a failure")
	assert.NotContains(t, save.Environment, "CI_CACHE_TOKEN", "the token is only added when an agent picks the workflow up")
}

func TestCompileCacheDAG(t *testing.T) {
	t.Parallel()

	// An explicit depends_on on one step switches the workflow to DAG mode.
	config, err := cacheTestCompiler().Compile(cacheWorkflow([]*yaml_types.Cache{goCache},
		commandStep("lint"),
		commandStep("build"),
		commandStep("test", "build"),
	))
	require.NoError(t, err)

	_, steps := stepsByName(t, config)
	assert.Equal(t, []string{"clone"}, steps["restore-cache"].DependsOn)
	assert.ElementsMatch(t, []string{"clone", "restore-cache"}, steps["lint"].DependsOn)
	assert.ElementsMatch(t, []string{"clone", "restore-cache"}, steps["build"].DependsOn)
	assert.Equal(t, []string{"build"}, steps["test"].DependsOn)
	assert.ElementsMatch(t, []string{"lint", "build", "test"}, steps["save-cache"].DependsOn)

	last := config.Stages[len(config.Stages)-1]
	require.Len(t, last.Steps, 1)
	assert.Equal(t, "save-cache", last.Steps[0].Name)
}

func TestCompileCacheSavePolicies(t *testing.T) {
	t.Parallel()

	caches := []*yaml_types.Cache{
		{Key: "a", Paths: []string{"a"}},
		{Key: "b", Paths: []string{"b"}, Save: yaml_types.CacheSaveAlways},
		{Key: "c", Paths: []string{"c"}, Save: yaml_types.CacheSaveNever},
	}
	config, err := cacheTestCompiler().Compile(cacheWorkflow(caches, commandStep("build")))
	require.NoError(t, err)

	names, steps := stepsByName(t, config)
	assert.Equal(t, []string{"clone", "restore-cache", "build", "save-cache", "save-cache-always"}, names)

	keys := func(step *backend_types.Step) []string {
		var specs []CacheSpec
		require.NoError(t, json.Unmarshal([]byte(step.Environment["PLUGIN_CACHES"]), &specs))
		var out []string
		for _, spec := range specs {
			out = append(out, spec.Key)
		}
		return out
	}
	assert.Equal(t, []string{"a", "b", "c"}, keys(steps["restore-cache"]))
	assert.Equal(t, []string{"a"}, keys(steps["save-cache"]))
	assert.Equal(t, []string{"b"}, keys(steps["save-cache-always"]))
	assert.True(t, steps["save-cache-always"].OnFailure)
	assert.Equal(t, []string{"build"}, steps["save-cache-always"].DependsOn, "save steps do not wait for each other")
}

func TestCompileCacheDisabled(t *testing.T) {
	t.Parallel()

	tests := map[string]*Compiler{
		"no plugin configured": cacheTestCompiler(WithCachePlugin("")),
		"local run":            cacheTestCompiler(WithLocal(true)),
	}
	for name, compiler := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			config, err := compiler.Compile(cacheWorkflow([]*yaml_types.Cache{goCache}, commandStep("build")))
			require.NoError(t, err)
			for _, stage := range config.Stages {
				for _, s := range stage.Steps {
					assert.NotEqual(t, backend_types.StepTypeCache, s.Type)
				}
			}
		})
	}
}

func TestCompileCacheStepNamesStayUnique(t *testing.T) {
	t.Parallel()

	config, err := cacheTestCompiler().Compile(cacheWorkflow([]*yaml_types.Cache{goCache}, commandStep("restore-cache"), commandStep("save-cache")))
	require.NoError(t, err)

	names, steps := stepsByName(t, config)
	assert.Equal(t, []string{"clone", "restore-cache", "restore-cache (2)", "save-cache", "save-cache (2)"}, names)
	assert.Equal(t, backend_types.StepTypeCache, steps["save-cache (2)"].Type)
	assert.Equal(t, []string{"restore-cache (2)", "save-cache"}, steps["save-cache (2)"].DependsOn)
}

func TestCompileCacheDAGExplicitCloneDependency(t *testing.T) {
	t.Parallel()

	// A step that names clone explicitly still has to wait for the restored
	// caches, otherwise it reads the workspace while they are being extracted.
	config, err := cacheTestCompiler().Compile(cacheWorkflow([]*yaml_types.Cache{goCache},
		commandStep("lint", "clone"),
		commandStep("build"),
		commandStep("test", "build"),
	))
	require.NoError(t, err)

	_, steps := stepsByName(t, config)
	assert.ElementsMatch(t, []string{"clone", "restore-cache"}, steps["lint"].DependsOn)
	assert.ElementsMatch(t, []string{"clone", "restore-cache"}, steps["build"].DependsOn)
	assert.Equal(t, []string{"build"}, steps["test"].DependsOn, "steps after another work step wait for the cache through it")
}
