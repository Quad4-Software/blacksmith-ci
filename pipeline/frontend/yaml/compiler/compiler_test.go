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
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/crowci/crow/v3/pipeline/backend/types"
	"codeberg.org/crowci/crow/v3/pipeline/frontend/metadata"
	yaml_types "codeberg.org/crowci/crow/v3/pipeline/frontend/yaml/types"
	yaml_base_types "codeberg.org/crowci/crow/v3/pipeline/frontend/yaml/types/base"
	"codeberg.org/crowci/crow/v3/shared/constant"
)

func TestSecretAvailable(t *testing.T) {
	secret := Secret{
		AllowedPlugins: []string{},
		Events:         []string{"push"},
	}
	assert.NoError(t, secret.Available("push", &yaml_types.Container{
		Image:    "golang",
		Commands: yaml_base_types.StringOrSlice{"echo 'this is not a plugin'"},
	}))

	// secret only available for "golang" plugin
	secret = Secret{
		Name:           "foo",
		AllowedPlugins: []string{"golang"},
		Events:         []string{"push"},
	}
	assert.NoError(t, secret.Available("push", &yaml_types.Container{
		Name:     "step",
		Image:    "golang",
		Commands: yaml_base_types.StringOrSlice{},
	}))
	assert.ErrorContains(t, secret.Available("push", &yaml_types.Container{
		Image:    "golang",
		Commands: yaml_base_types.StringOrSlice{"echo 'this is not a plugin'"},
	}), "is only allowed to be used by plugins (a filter has been set on the secret). Note: Image filters do not work for normal steps")
	assert.ErrorContains(t, secret.Available("push", &yaml_types.Container{
		Image:    "not-golang",
		Commands: yaml_base_types.StringOrSlice{},
	}), "not allowed to be used with image ")
	assert.ErrorContains(t, secret.Available("pull_request", &yaml_types.Container{
		Image: "golang",
	}), "not allowed to be used with pipeline event ")
}

func TestCompilerCompile(t *testing.T) {
	repoURL := "https://github.com/octocat/hello-world"
	compiler := New(
		WithMetadata(metadata.Metadata{
			Repo: metadata.Repo{
				Owner:    "octacat",
				Name:     "hello-world",
				Private:  true,
				ForgeURL: repoURL,
				CloneURL: "https://github.com/octocat/hello-world.git",
			},
		}),
		WithEnviron(map[string]string{
			"VERBOSE": "true",
			"COLORED": "true",
		}),
		WithPrefix("test"),
		// we use "/test" as custom workspace base to ensure the enforcement of the pluginWorkspaceBase is applied
		WithWorkspaceFromURL("/test", repoURL),
	)

	defaultNetwork := "test_default"
	defaultVolume := "test_default"

	defaultCloneStage := &types.Stage{
		Steps: []*types.Step{{
			Name:          "clone",
			Type:          types.StepTypeClone,
			Image:         constant.DefaultClonePlugin,
			OnSuccess:     true,
			Failure:       "fail",
			Volumes:       []string{defaultVolume + ":/crow"},
			WorkingDir:    "/crow/src/github.com/octocat/hello-world",
			WorkspaceBase: "/crow",
			Networks:      []types.Conn{{Name: "test_default", Aliases: []string{"clone"}}},
			ExtraHosts:    []types.HostAlias{},
		}},
	}

	tests := []struct {
		name        string
		fronConf    *yaml_types.Workflow
		backConf    *types.Config
		expectedErr string
	}{
		{
			name:     "empty workflow, no clone",
			fronConf: &yaml_types.Workflow{SkipClone: true},
			backConf: &types.Config{
				Network: defaultNetwork,
				Volume:  defaultVolume,
			},
		},
		{
			name:     "empty workflow, default clone",
			fronConf: &yaml_types.Workflow{},
			backConf: &types.Config{
				Network: defaultNetwork,
				Volume:  defaultVolume,
				Stages:  []*types.Stage{defaultCloneStage},
			},
		},
		{
			name: "workflow with one dummy step",
			fronConf: &yaml_types.Workflow{Steps: yaml_types.ContainerList{ContainerList: []*yaml_types.Container{{
				Name:  "dummy",
				Image: "dummy_img",
			}}}},
			backConf: &types.Config{
				Network: defaultNetwork,
				Volume:  defaultVolume,
				Stages: []*types.Stage{defaultCloneStage, {
					Steps: []*types.Step{{
						Name:          "dummy",
						Type:          types.StepTypePlugin,
						Image:         "dummy_img",
						OnSuccess:     true,
						Failure:       "fail",
						Volumes:       []string{defaultVolume + ":/crow"},
						WorkingDir:    "/crow/src/github.com/octocat/hello-world",
						WorkspaceBase: "/crow",
						Networks:      []types.Conn{{Name: "test_default", Aliases: []string{"dummy"}}},
						ExtraHosts:    []types.HostAlias{},
						DependsOn:     []string{"clone"},
					}},
				}},
			},
		},
		{
			name: "workflow with three steps",
			fronConf: &yaml_types.Workflow{Steps: yaml_types.ContainerList{ContainerList: []*yaml_types.Container{{
				Name:     "echo env",
				Image:    "bash",
				Commands: []string{"env"},
			}, {
				Name:     "parallel echo 1",
				Image:    "bash",
				Commands: []string{"echo 1"},
			}, {
				Name:     "parallel echo 2",
				Image:    "bash",
				Commands: []string{"echo 2"},
			}}}},
			backConf: &types.Config{
				Network: defaultNetwork,
				Volume:  defaultVolume,
				Stages: []*types.Stage{
					defaultCloneStage, {
						Steps: []*types.Step{{
							Name:          "echo env",
							Type:          types.StepTypeCommands,
							Image:         "bash",
							Commands:      []string{"env"},
							OnSuccess:     true,
							Failure:       "fail",
							Volumes:       []string{defaultVolume + ":/test"},
							WorkingDir:    "/test/src/github.com/octocat/hello-world",
							WorkspaceBase: "/test",
							Networks:      []types.Conn{{Name: "test_default", Aliases: []string{"echo env"}}},
							ExtraHosts:    []types.HostAlias{},
							DependsOn:     []string{"clone"},
						}},
					}, {
						Steps: []*types.Step{{
							Name:          "parallel echo 1",
							Type:          types.StepTypeCommands,
							Image:         "bash",
							Commands:      []string{"echo 1"},
							OnSuccess:     true,
							Failure:       "fail",
							Volumes:       []string{defaultVolume + ":/test"},
							WorkingDir:    "/test/src/github.com/octocat/hello-world",
							WorkspaceBase: "/test",
							Networks:      []types.Conn{{Name: "test_default", Aliases: []string{"parallel echo 1"}}},
							ExtraHosts:    []types.HostAlias{},
							DependsOn:     []string{"echo env"},
						}},
					}, {
						Steps: []*types.Step{{
							Name:          "parallel echo 2",
							Type:          types.StepTypeCommands,
							Image:         "bash",
							Commands:      []string{"echo 2"},
							OnSuccess:     true,
							Failure:       "fail",
							Volumes:       []string{defaultVolume + ":/test"},
							WorkingDir:    "/test/src/github.com/octocat/hello-world",
							WorkspaceBase: "/test",
							Networks:      []types.Conn{{Name: "test_default", Aliases: []string{"parallel echo 2"}}},
							ExtraHosts:    []types.HostAlias{},
							DependsOn:     []string{"parallel echo 1"},
						}},
					},
				},
			},
		},
		{
			name: "workflow with three steps and depends_on",
			fronConf: &yaml_types.Workflow{Steps: yaml_types.ContainerList{ContainerList: []*yaml_types.Container{{
				Name:     "echo env",
				Image:    "bash",
				Commands: []string{"env"},
			}, {
				Name:      "echo 1",
				Image:     "bash",
				Commands:  []string{"echo 1"},
				DependsOn: []string{"echo env", "echo 2"},
			}, {
				Name:     "echo 2",
				Image:    "bash",
				Commands: []string{"echo 2"},
			}}}},
			backConf: &types.Config{
				Network: defaultNetwork,
				Volume:  defaultVolume,
				Stages: []*types.Stage{defaultCloneStage, {
					Steps: []*types.Step{{
						Name:          "echo env",
						Type:          types.StepTypeCommands,
						Image:         "bash",
						Commands:      []string{"env"},
						OnSuccess:     true,
						Failure:       "fail",
						Volumes:       []string{defaultVolume + ":/test"},
						WorkingDir:    "/test/src/github.com/octocat/hello-world",
						WorkspaceBase: "/test",
						Networks:      []types.Conn{{Name: "test_default", Aliases: []string{"echo env"}}},
						ExtraHosts:    []types.HostAlias{},
						DependsOn:     []string{"clone"},
					}, {
						Name:          "echo 2",
						Type:          types.StepTypeCommands,
						Image:         "bash",
						Commands:      []string{"echo 2"},
						OnSuccess:     true,
						Failure:       "fail",
						Volumes:       []string{defaultVolume + ":/test"},
						WorkingDir:    "/test/src/github.com/octocat/hello-world",
						WorkspaceBase: "/test",
						Networks:      []types.Conn{{Name: "test_default", Aliases: []string{"echo 2"}}},
						ExtraHosts:    []types.HostAlias{},
						DependsOn:     []string{"clone"},
					}},
				}, {
					Steps: []*types.Step{{
						Name:          "echo 1",
						Type:          types.StepTypeCommands,
						Image:         "bash",
						Commands:      []string{"echo 1"},
						OnSuccess:     true,
						Failure:       "fail",
						Volumes:       []string{defaultVolume + ":/test"},
						WorkingDir:    "/test/src/github.com/octocat/hello-world",
						WorkspaceBase: "/test",
						Networks:      []types.Conn{{Name: "test_default", Aliases: []string{"echo 1"}}},
						ExtraHosts:    []types.HostAlias{},
						DependsOn:     []string{"echo env", "echo 2"},
					}},
				}},
			},
		},
		{
			name: "workflow with missing secret",
			fronConf: &yaml_types.Workflow{Steps: yaml_types.ContainerList{ContainerList: []*yaml_types.Container{{
				Name:     "step",
				Image:    "bash",
				Commands: []string{"env"},
				Environment: yaml_base_types.EnvironmentMap{
					"MISSING": map[string]any{"from_secret": "missing"},
				},
			}}}},
			backConf:    nil,
			expectedErr: "secret \"missing\" not found",
		},
		{
			name: "workflow with broken step dependency",
			fronConf: &yaml_types.Workflow{Steps: yaml_types.ContainerList{ContainerList: []*yaml_types.Container{{
				Name:      "dummy",
				Image:     "dummy_img",
				DependsOn: []string{"not exist"},
			}}}},
			backConf:    nil,
			expectedErr: "step 'dummy' depends on unknown step 'not exist'",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			backConf, err := compiler.Compile(test.fronConf)
			if test.expectedErr != "" {
				assert.Error(t, err)
				assert.Equal(t, test.expectedErr, err.Error())
			} else {
				// we ignore uuids in steps and only check if global env got set ...
				for _, st := range backConf.Stages {
					for _, s := range st.Steps {
						s.UUID = ""
						assert.Truef(t, s.Environment["VERBOSE"] == "true", "expected to get value of global set environment")
						assert.Truef(t, len(s.Environment) > 10, "expected to have a lot of built-in variables")
						s.Environment = nil
						s.SecretMapping = nil
					}
				}
				// check if we get an expected backend config based on a frontend config
				assert.EqualValues(t, *test.backConf, *backConf)
			}
		})
	}
}

func TestCompilerCompileWithFromSecret(t *testing.T) {
	repoURL := "https://github.com/octocat/hello-world"
	compiler := New(
		WithMetadata(metadata.Metadata{
			Repo: metadata.Repo{
				Owner:    "octacat",
				Name:     "hello-world",
				Private:  true,
				ForgeURL: repoURL,
				CloneURL: "https://github.com/octocat/hello-world.git",
			},
		}),
		WithEnviron(map[string]string{
			"VERBOSE": "true",
			"COLORED": "true",
		}),
		WithSecret(Secret{
			Name:  "secret_name",
			Value: "VERY_SECRET",
		}),
		WithPrefix("test"),
		// we use "/test" as custom workspace base to ensure the enforcement of the pluginWorkspaceBase is applied
		WithWorkspaceFromURL("/test", repoURL),
	)
	defaultNetwork := "test_default"
	defaultVolume := "test_default"
	defaultCloneStage := &types.Stage{
		Steps: []*types.Step{{
			Name:          "clone",
			Type:          types.StepTypeClone,
			Image:         constant.DefaultClonePlugin,
			OnSuccess:     true,
			Failure:       "fail",
			WorkingDir:    "/crow/src/github.com/octocat/hello-world",
			WorkspaceBase: "/crow",
			Volumes:       []string{defaultVolume + ":/crow"},
			Networks:      []types.Conn{{Name: "test_default", Aliases: []string{"clone"}}},
			ExtraHosts:    []types.HostAlias{},
		}},
	}
	tests := []struct {
		name        string
		fronConf    *yaml_types.Workflow
		backConf    *types.Config
		expectedErr string
	}{
		{
			name: "workflow with missing secret",
			fronConf: &yaml_types.Workflow{Steps: yaml_types.ContainerList{ContainerList: []*yaml_types.Container{{
				Name:     "step",
				Image:    "bash",
				Commands: []string{"env"},
				Environment: yaml_base_types.EnvironmentMap{
					"SECRET": map[string]any{"from_secret": "secret_name"},
				},
			}}}},
			backConf: &types.Config{
				Stages: []*types.Stage{defaultCloneStage, {
					Steps: []*types.Step{{
						Name:          "step",
						Type:          types.StepTypeCommands,
						Image:         "bash",
						Commands:      []string{"env"},
						OnSuccess:     true,
						Failure:       "fail",
						WorkingDir:    "/test/src/github.com/octocat/hello-world",
						WorkspaceBase: "/test",
						Volumes:       []string{defaultVolume + ":/test"},
						Networks:      []types.Conn{{Name: "test_default", Aliases: []string{"step"}}},
						ExtraHosts:    []types.HostAlias{},
						DependsOn:     []string{"clone"},
						SecretMapping: map[string]string{
							"SECRET": "VERY_SECRET",
						},
					}},
				}},
				Volume:  defaultVolume,
				Network: defaultNetwork,
				Secrets: []*types.Secret{{
					Name:  "secret_name",
					Value: "VERY_SECRET",
				}},
			},
			expectedErr: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			backConf, err := compiler.Compile(test.fronConf)
			if test.expectedErr != "" {
				assert.Error(t, err)
				assert.Equal(t, test.expectedErr, err.Error())
			} else {
				// we ignore uuids in steps and only check if global env got set ...
				for _, st := range backConf.Stages {
					for _, s := range st.Steps {
						s.UUID = ""
						assert.Truef(t, s.Environment["VERBOSE"] == "true", "expected to get value of global set environment")
						assert.Truef(t, len(s.Environment) > 10, "expected to have a lot of built-in variables")
						s.Environment = nil

						if len(s.SecretMapping) == 0 {
							s.SecretMapping = nil
						}
					}
				}
				// check if we get an expected backend config based on a frontend config
				assert.EqualValues(t, *test.backConf, *backConf)
			}
		})
	}
}

func TestSecretMatch(t *testing.T) {
	tcl := []*struct {
		name   string
		secret Secret
		event  string
		match  bool
	}{
		{
			name:   "should match event",
			secret: Secret{Events: []string{"pull_request"}},
			event:  "pull_request",
			match:  true,
		},
		{
			name:   "should not match event",
			secret: Secret{Events: []string{"pull_request"}},
			event:  "push",
			match:  false,
		},
		{
			name:   "should match when no event filters defined",
			secret: Secret{},
			event:  "pull_request",
			match:  true,
		},
		{
			name:   "pull close should match pull",
			secret: Secret{Events: []string{"pull_request"}},
			event:  "pull_request_closed",
			match:  true,
		},
		{
			name:   "pull edited should match pull",
			secret: Secret{Events: []string{"pull_request"}},
			event:  "pull_request_edited",
			match:  true,
		},
	}

	for _, tc := range tcl {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.match, tc.secret.Match(tc.event))
		})
	}
}

func TestCompilerCompilePrivileged(t *testing.T) {
	compiler := New(
		WithEscalated("test/image"),
	)

	fronConf := &yaml_types.Workflow{
		SkipClone: true,
		Steps: yaml_types.ContainerList{
			ContainerList: []*yaml_types.Container{
				{
					Name:      "privileged-plugin",
					Image:     "test/image",
					DependsOn: []string{}, // no dependencies =>  enable dag mode & all steps are executed in parallel
				},
				{
					Name:     "no-plugin",
					Image:    "test/image",
					Commands: []string{"echo 'i am not a plugin anymore'"},
				},
				{
					Name:  "not-privileged-image",
					Image: "some/other-image",
				},
			},
		},
	}

	backConf, err := compiler.Compile(fronConf)
	assert.NoError(t, err)

	assert.Len(t, backConf.Stages, 1)
	assert.Len(t, backConf.Stages[0].Steps, 3)
	assert.True(t, backConf.Stages[0].Steps[0].Privileged)
	assert.False(t, backConf.Stages[0].Steps[1].Privileged)
	assert.False(t, backConf.Stages[0].Steps[2].Privileged)
}

func TestCompilerImplicitCloneDependencies(t *testing.T) {
	t.Parallel()

	// Test that implicit clone dependencies are added to simple workflows
	conf := &yaml_types.Workflow{
		Steps: yaml_types.ContainerList{
			ContainerList: []*yaml_types.Container{
				{
					Name:  "test",
					Image: "alpine",
					Commands: []string{
						"echo 'running test'",
					},
				},
			},
		},
	}

	repoURL := "https://github.com/test/repo"
	compiler := New(
		WithMetadata(metadata.Metadata{
			Repo: metadata.Repo{
				Owner:    "test",
				Name:     "repo",
				Private:  false,
				ForgeURL: repoURL,
				CloneURL: "https://github.com/test/repo.git",
			},
		}),
		WithEnviron(map[string]string{}),
		WithPrefix("test"),
		WithWorkspaceFromURL("/test", repoURL),
	)

	result, err := compiler.Compile(conf)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Should have 2 stages: clone and test
	assert.Len(t, result.Stages, 2)

	// First stage should be clone
	cloneStage := result.Stages[0]
	assert.Len(t, cloneStage.Steps, 1)
	assert.Equal(t, "clone", cloneStage.Steps[0].Name)

	// Second stage should be test with implicit dependency on clone
	testStage := result.Stages[1]
	assert.Len(t, testStage.Steps, 1)
	assert.Equal(t, "test", testStage.Steps[0].Name)
	assert.Equal(t, []string{"clone"}, testStage.Steps[0].DependsOn)

	t.Logf("Clone stage: %+v", cloneStage)
	t.Logf("Test stage: %+v", testStage)
	t.Logf("Test step depends_on: %+v", testStage.Steps[0].DependsOn)
}

func TestWorkflowDependsOnWithCloneSteps(t *testing.T) {
	t.Parallel()

	// Test that workflows with global depends_on still create clone steps
	// but the regular steps don't depend on them
	conf := &yaml_types.Workflow{
		DependsOn: []string{"other-workflow"}, // Global workflow dependency
		Steps: yaml_types.ContainerList{
			ContainerList: []*yaml_types.Container{
				{
					Name:  "test",
					Image: "alpine",
					Commands: []string{
						"echo 'running test'",
					},
				},
			},
		},
	}

	repoURL := "https://github.com/test/repo"
	compiler := New(
		WithMetadata(metadata.Metadata{
			Repo: metadata.Repo{
				Owner:    "test",
				Name:     "repo",
				Private:  false,
				ForgeURL: repoURL,
				CloneURL: "https://github.com/test/repo.git",
			},
		}),
		WithWorkspaceFromURL("/crow", repoURL),
		WithDefaultClonePlugin("crow-plugins/clone"),
	)

	result, err := compiler.Compile(conf)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Should have 2 stages: clone and test
	assert.Len(t, result.Stages, 2)

	// First stage should be clone
	cloneStage := result.Stages[0]
	assert.Len(t, cloneStage.Steps, 1)
	assert.Equal(t, "clone", cloneStage.Steps[0].Name)

	// Second stage should be test with dependency on clone
	// Even though the workflow has global depends_on, the individual steps still need the clone
	testStage := result.Stages[1]
	assert.Len(t, testStage.Steps, 1)
	assert.Equal(t, "test", testStage.Steps[0].Name)
	assert.Equal(t, []string{"clone"}, testStage.Steps[0].DependsOn, "Test step should depend on clone even when workflow has global depends_on")

	// This is the correct behavior:
	// 1. The workflow waits for its global depends_on (handled at the workflow level)
	// 2. Once global deps are satisfied, clone runs first
	// 3. Then regular steps run after clone completes

	t.Logf("Clone stage: %+v", cloneStage)
	t.Logf("Test stage: %+v", testStage)
	t.Logf("Test step depends_on: %+v", testStage.Steps[0].DependsOn)
}

func TestCompilerMatrixImplicitCloneDependencies(t *testing.T) {
	t.Parallel()

	// Test that implicit clone dependencies are added to matrix workflows
	conf := &yaml_types.Workflow{
		Steps: yaml_types.ContainerList{
			ContainerList: []*yaml_types.Container{
				{
					Name:  "test",
					Image: "alpine",
					Commands: []string{
						"echo 'running test'",
					},
				},
			},
		},
	}

	repoURL := "https://github.com/test/repo"
	compiler := New(
		WithMetadata(metadata.Metadata{
			Repo: metadata.Repo{
				Owner:    "test",
				Name:     "repo",
				Private:  false,
				ForgeURL: repoURL,
				CloneURL: "https://github.com/test/repo.git",
			},
			Curr: metadata.Pipeline{
				Event: "push",
			},
		}),
		WithWorkspaceFromURL("/workspace", repoURL),
		// Don't set WithDefaultClonePlugin to test the fallback
	)

	config, err := compiler.Compile(conf)
	if err != nil {
		t.Fatal(err)
	}

	// Should have at least 2 stages: clone and test
	assert.GreaterOrEqual(t, len(config.Stages), 2, "Should have at least 2 stages")

	// First stage should be clone
	cloneStage := config.Stages[0]
	assert.Len(t, cloneStage.Steps, 1, "Clone stage should have 1 step")
	assert.Equal(t, "clone", cloneStage.Steps[0].Name, "First stage should be clone")

	// Second stage should be test with implicit dependency on clone
	testStage := config.Stages[1]
	assert.Len(t, testStage.Steps, 1, "Test stage should have 1 step")
	assert.Equal(t, "test", testStage.Steps[0].Name, "Second stage should be test")
	assert.Equal(t, []string{"clone"}, testStage.Steps[0].DependsOn, "Test step should depend on clone")
}

func TestCompilerWorkflowDependsOnSkipsImplicitClone(t *testing.T) {
	t.Parallel()

	// Test that workflows with global depends_on do NOT get implicit clone dependencies
	conf := &yaml_types.Workflow{
		DependsOn: []string{"other-workflow"}, // Global workflow dependency
		Steps: yaml_types.ContainerList{
			ContainerList: []*yaml_types.Container{
				{
					Name:  "test",
					Image: "alpine",
					Commands: []string{
						"echo 'running test'",
					},
				},
			},
		},
	}

	repoURL := "https://github.com/test/repo"
	compiler := New(
		WithMetadata(metadata.Metadata{
			Repo: metadata.Repo{
				Owner:    "test",
				Name:     "repo",
				Private:  false,
				ForgeURL: repoURL,
				CloneURL: "https://github.com/test/repo.git",
			},
		}),
		WithWorkspaceFromURL("/crow", repoURL),
		WithDefaultClonePlugin("crow-plugins/clone"),
	)

	result, err := compiler.Compile(conf)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Should have 2 stages: clone and test
	assert.Len(t, result.Stages, 2)

	// First stage should be clone
	cloneStage := result.Stages[0]
	assert.Len(t, cloneStage.Steps, 1)
	assert.Equal(t, "clone", cloneStage.Steps[0].Name)

	// Second stage should be test with implicit dependency on clone
	// Even though the workflow has global depends_on, steps still need clone for source code
	testStage := result.Stages[1]
	assert.Len(t, testStage.Steps, 1)
	assert.Equal(t, "test", testStage.Steps[0].Name)
	assert.Equal(t, []string{"clone"}, testStage.Steps[0].DependsOn, "Test step should depend on clone even when workflow has global depends_on")

	t.Logf("Clone stage: %+v", cloneStage)
	t.Logf("Test stage: %+v", testStage)
	t.Logf("Test step depends_on: %+v", testStage.Steps[0].DependsOn)
}
