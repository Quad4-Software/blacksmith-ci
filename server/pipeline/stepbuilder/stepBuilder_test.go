// Copyright 2022 Woodpecker Authors
// Copyright 2018 Drone.IO Inc.
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

package stepbuilder

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backend_types "codefloe.com/crowci/crow/v6/pipeline/backend/types"
	"codefloe.com/crowci/crow/v6/pipeline/errors"
	"codefloe.com/crowci/crow/v6/pipeline/frontend/yaml/compiler/settings"
	"codefloe.com/crowci/crow/v6/server/forge"
	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	forge_types "codefloe.com/crowci/crow/v6/server/forge/types"
	"codefloe.com/crowci/crow/v6/server/model"
)

func TestGlobalEnvsubst(t *testing.T) {
	t.Parallel()

	b := StepBuilder{
		Forge: getMockForge(),
		Envs: map[string]string{
			"KEY_K": "VALUE_V",
			"IMAGE": "scratch",
		},
		Repo: &model.Repo{},
		Curr: &model.Pipeline{
			Message: "aaa",
			Event:   model.EventPush,
		},
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			{Data: []byte(`
when:
  event: push
steps:
  build:
    image: ${IMAGE}
    settings:
      yyy: ${CI_COMMIT_MESSAGE}
`)},
		},
	}

	if pipelineItems, err := b.Build(); err != nil {
		t.Fatal(err)
	} else {
		fmt.Println(pipelineItems)
	}
}

func TestMissingGlobalEnvsubst(t *testing.T) {
	t.Parallel()

	b := StepBuilder{
		Forge: getMockForge(),
		Envs: map[string]string{
			"KEY_K":    "VALUE_V",
			"NO_IMAGE": "scratch",
		},
		Repo: &model.Repo{},
		Curr: &model.Pipeline{
			Message: "aaa",
			Event:   model.EventPush,
		},
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			{Data: []byte(`
when:
  event: push
steps:
  build:
    image: ${IMAGE}
    settings:
      yyy: ${CI_COMMIT_MESSAGE}
`)},
		},
	}

	if _, err := b.Build(); err != nil {
		fmt.Println("test rightfully failed")
	} else {
		t.Fatal("test erroneously succeeded")
	}
}

func TestMultilineEnvsubst(t *testing.T) {
	t.Parallel()

	b := StepBuilder{
		Forge: getMockForge(),
		Repo:  &model.Repo{},
		Curr: &model.Pipeline{
			Message: `aaa
bbb`,
		},
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			{Data: []byte(`
when:
  event: push
steps:
  xxx:
    image: scratch
    settings:
      yyy: ${CI_COMMIT_MESSAGE}
`)},
			{Data: []byte(`
when:
  event: push
steps:
  build:
    image: scratch
    settings:
      yyy: ${CI_COMMIT_MESSAGE}
`)},
		},
	}

	if pipelineItems, err := b.Build(); err != nil {
		t.Fatal(err)
	} else {
		fmt.Println(pipelineItems)
	}
}

func TestMultiPipeline(t *testing.T) {
	t.Parallel()

	b := StepBuilder{
		Forge: getMockForge(),
		Repo:  &model.Repo{},
		Curr: &model.Pipeline{
			Event: model.EventPush,
		},
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			{Data: []byte(`
when:
  event: push
steps:
  xxx:
    image: scratch
`)},
			{Data: []byte(`
when:
  event: push
steps:
  build:
    image: scratch
`)},
		},
	}

	pipelineItems, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(pipelineItems) != 2 {
		t.Fatal("Should have generated 2 pipelineItems")
	}
}

func TestDependsOn(t *testing.T) {
	t.Parallel()

	b := StepBuilder{
		Forge: getMockForge(),
		Repo:  &model.Repo{},
		Curr: &model.Pipeline{
			Event: model.EventPush,
		},
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			{Name: "lint", Data: []byte(`
when:
  event: push
steps:
  build:
    image: scratch
`)},
			{Name: "test", Data: []byte(`
when:
  event: push
steps:
  build:
    image: scratch
`)},
			{Data: []byte(`
when:
  event: push
steps:
  deploy:
    image: scratch

depends_on:
  - lint
  - test
`)},
		},
	}

	pipelineItems, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(pipelineItems[0].DependsOn) != 2 {
		t.Fatal("Should have 3 dependencies")
	}
	if pipelineItems[0].DependsOn[1] != "test" {
		t.Fatal("Should depend on test")
	}
}

func TestRunsOn(t *testing.T) {
	t.Parallel()

	b := StepBuilder{
		Forge: getMockForge(),
		Repo:  &model.Repo{},
		Curr: &model.Pipeline{
			Event: model.EventPush,
		},
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			{Data: []byte(`
when:
  event: push
steps:
  deploy:
    image: scratch

runs_on:
  - success
  - failure
`)},
		},
	}

	pipelineItems, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(pipelineItems[0].RunsOn) != 2 {
		t.Fatal("Should run on success and failure")
	}
	if pipelineItems[0].RunsOn[1] != "failure" {
		t.Fatal("Should run on failure")
	}
}

func TestDependsOnWithNamedWorkflow(t *testing.T) {
	t.Parallel()

	b := StepBuilder{
		Forge: getMockForge(),
		Repo:  &model.Repo{},
		Curr: &model.Pipeline{
			Event: model.EventPush,
		},
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			{Name: "mat.yaml", Data: []byte(`
when:
  event: push
steps:
  build:
    image: scratch
`)},
			{Name: "myworkflow.yaml", Data: []byte(`
when:
  event: push

name: myname

depends_on:
 - mat

steps:
  test:
    image: bash
    commands:
      - sleep 30
`)},
		},
	}

	pipelineItems, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}

	// Should have 2 workflows: "mat" and "myname"
	if len(pipelineItems) != 2 {
		t.Fatalf("Expected 2 workflows, got %d", len(pipelineItems))
	}

	// Find the workflows
	var matWorkflow, mynameWorkflow *Item
	for _, item := range pipelineItems {
		switch item.Workflow.Name {
		case "mat":
			matWorkflow = item
		case "myname":
			mynameWorkflow = item
		}
	}

	if matWorkflow == nil {
		t.Fatal("mat workflow not found")
	}
	if mynameWorkflow == nil {
		t.Fatal("myname workflow not found")
	}

	// Check dependencies
	if len(matWorkflow.DependsOn) != 0 {
		t.Fatal("mat workflow should have no dependencies")
	}
	if len(mynameWorkflow.DependsOn) != 1 || mynameWorkflow.DependsOn[0] != "mat" {
		t.Fatalf("myname workflow should depend on mat, got: %v", mynameWorkflow.DependsOn)
	}
}

func TestDependsOnWithManualEvent(t *testing.T) {
	t.Parallel()

	b := StepBuilder{
		Forge: getMockForge(),
		Repo:  &model.Repo{},
		Curr: &model.Pipeline{
			Event: model.EventManual,
		},
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			{Name: "mat.yaml", Data: []byte(`
when:
  event: ['manual', 'push']
steps:
  build:
    image: bash
    commands:
      - echo "mat workflow"
`)},
			{Name: "myworkflow.yaml", Data: []byte(`
when:
  event: ['manual', 'push']

name: myname

depends_on:
 - mat

steps:
  - name: test
    image: bash
    commands:
      - sleep 30

  - name: a step with a very long title
    image: bash
    commands:
      - sleep 15

  - name: test3
    image: bash
    depends_on: test
    commands:
      - exit 1
`)},
		},
	}

	// The steps of myworkflow mix implicit and explicit ordering, which the
	// linter reports as a warning.
	pipelineItems, err := b.Build()
	if errors.HasBlockingErrors(err) {
		t.Fatal(err)
	}

	// Debug output
	t.Logf("Generated %d pipeline items", len(pipelineItems))
	for i, item := range pipelineItems {
		t.Logf("Item %d: name=%s, depends_on=%v", i, item.Workflow.Name, item.DependsOn)
	}

	// Should have 2 workflows: "mat" and "myname"
	if len(pipelineItems) != 2 {
		t.Fatalf("Expected 2 workflows, got %d", len(pipelineItems))
	}

	// Find the workflows
	var matWorkflow, mynameWorkflow *Item
	for _, item := range pipelineItems {
		switch item.Workflow.Name {
		case "mat":
			matWorkflow = item
		case "myname":
			mynameWorkflow = item
		}
	}

	if matWorkflow == nil {
		t.Fatal("mat workflow not found")
	}
	if mynameWorkflow == nil {
		t.Fatal("myname workflow not found")
	}

	// Check dependencies
	if len(matWorkflow.DependsOn) != 0 {
		t.Fatal("mat workflow should have no dependencies")
	}
	if len(mynameWorkflow.DependsOn) != 1 || mynameWorkflow.DependsOn[0] != "mat" {
		t.Fatalf("myname workflow should depend on mat, got: %v", mynameWorkflow.DependsOn)
	}
}

func TestMatrixDependsOnIssue(t *testing.T) {
	t.Parallel()

	// This test reproduces the issue where matrix workflows break depends_on
	// because the workflow name changes from the filename to the matrix instance name
	b := StepBuilder{
		Forge: getMockForge(),
		Repo:  &model.Repo{},
		Curr: &model.Pipeline{
			Event: model.EventPush,
		},
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			// Matrix workflow - this should be named "mat" but will be named "run1"
			{Name: "mat.yaml", Data: []byte(`
matrix:
  include:
    - NAME: run1
      VAR: value1
    - NAME: run2
      VAR: value2
when:
  event: push
steps:
  build:
    image: bash
    commands:
      - echo "Building $VAR"
`)},
			// Workflow that depends on "mat" - this should fail to find the dependency
			{Name: "dependent.yaml", Data: []byte(`
when:
  event: push

depends_on:
 - mat

steps:
  test:
    image: bash
    commands:
      - echo "Testing"
`)},
		},
	}

	pipelineItems, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}

	// Print out what we actually got for debugging
	t.Logf("Found %d pipeline items:", len(pipelineItems))
	for i, item := range pipelineItems {
		t.Logf("  %d: Name=%s, DependsOn=%v, File=%s", i, item.Workflow.Name, item.DependsOn, item.Workflow.Name)
	}

	// Find the dependent workflow
	var dependentWorkflow *Item
	for _, item := range pipelineItems {
		if item.Workflow.Name == "dependent" {
			dependentWorkflow = item
			break
		}
	}

	if dependentWorkflow == nil {
		t.Fatal("dependent workflow not found")
	}

	// Check if the dependency is satisfied
	// The depends_on should be able to find "mat" even though matrix created "run1" and "run2"
	expectedDep := "mat"
	found := false
	for _, item := range pipelineItems {
		if item.Workflow.Name == expectedDep || item.OriginalName == expectedDep {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Expected to find workflow named or originally named '%s' but not found", expectedDep)
	}

	// The depends_on should reference the original filename, not the matrix instance name
	if len(dependentWorkflow.DependsOn) != 1 || dependentWorkflow.DependsOn[0] != "mat" {
		t.Fatalf("dependent workflow should depend on 'mat', got: %v", dependentWorkflow.DependsOn)
	}

	// Verify that matrix workflows have the correct original name
	matrixWorkflows := 0
	for _, item := range pipelineItems {
		if item.OriginalName == "mat" {
			matrixWorkflows++
		}
	}
	if matrixWorkflows != 2 {
		t.Fatalf("Expected 2 matrix workflows with OriginalName='mat', got %d", matrixWorkflows)
	}
}

func TestPipelineName(t *testing.T) {
	t.Parallel()

	b := StepBuilder{
		Forge: getMockForge(),
		Repo:  &model.Repo{Config: ".woodpecker"},
		Curr: &model.Pipeline{
			Event: model.EventPush,
		},
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			{Name: ".crow/lint.yml", Data: []byte(`
when:
  event: push
steps:
  build:
    image: scratch
`)},
			{Name: ".crow/.test.yml", Data: []byte(`
when:
  event: push
steps:
  build:
    image: scratch
`)},
			// Top-level name
			{Name: ".crow/named.yml", Data: []byte(`
name: Custom Workflow Name
when:
  event: push
steps:
  build:
    image: scratch
`)},
			// Matrix NAME with variable
			{Name: ".crow/matrix.yml", Data: []byte(`
matrix:
  include:
    - ENVIRONMENT: dev
      NAME: Matrix for ${ENVIRONMENT}
    - ENVIRONMENT: prod
      NAME: Matrix for ${ENVIRONMENT}
when:
  event: push
steps:
  build:
    image: scratch
`)},
			// Both matrix NAME and top-level name, matrix NAME should take precedence
			{Name: ".crow/matrix2.yml", Data: []byte(`
name: Top Level Name
matrix:
  include:
    - ENVIRONMENT: qa
      NAME: Matrix QA
    - ENVIRONMENT: staging
      NAME: Matrix Staging
when:
  event: push
steps:
  build:
    image: scratch
`)},
		},
	}

	pipelineItems, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	pipelineNames := make([]string, len(pipelineItems))
	for i, item := range pipelineItems {
		pipelineNames[i] = item.Workflow.Name
	}
	// File-based
	if !containsItemWithName("lint", pipelineItems) || !containsItemWithName("test", pipelineItems) {
		t.Fatalf("Pipeline name should be 'lint' and 'test' but are '%v'", pipelineNames)
	}
	// Top-level name
	if !containsItemWithName("Custom Workflow Name", pipelineItems) {
		t.Fatalf("Pipeline name should include 'Custom Workflow Name' but are '%v'", pipelineNames)
	}
	// Matrix NAME with variable substitution
	if !containsItemWithName("Matrix for dev", pipelineItems) || !containsItemWithName("Matrix for prod", pipelineItems) {
		t.Fatalf("Pipeline names should include 'Matrix for dev' and 'Matrix for prod' but are '%v'", pipelineNames)
	}
	// Matrix NAME takes precedence over top-level name
	if !containsItemWithName("Matrix QA", pipelineItems) || !containsItemWithName("Matrix Staging", pipelineItems) {
		t.Fatalf("Pipeline names should include 'Matrix QA' and 'Matrix Staging' but are '%v'", pipelineNames)
	}
}

func TestBranchFilter(t *testing.T) {
	t.Parallel()

	b := StepBuilder{
		Forge: getMockForge(),
		Repo:  &model.Repo{},
		Curr: &model.Pipeline{
			Branch: "dev",
			Event:  model.EventPush,
		},
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			{Data: []byte(`
when:
  event: push
  branch: main
steps:
  xxx:
    image: scratch
`)},
			{Data: []byte(`
when:
  event: push
steps:
  build:
    image: scratch
`)},
		},
	}

	pipelineItems, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !assert.Len(t, pipelineItems, 1) {
		t.Fatal("Should have generated 1 pipeline")
	}
	if pipelineItems[0].Workflow.State != model.StatusPending {
		t.Fatal("Should run on dev branch")
	}
}

func TestRootWhenFilter(t *testing.T) {
	t.Parallel()

	b := StepBuilder{
		Forge: getMockForge(),
		Repo:  &model.Repo{},
		Curr:  &model.Pipeline{Event: "tag"},
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			{Data: []byte(`
when:
  event:
    - tag
steps:
  xxx:
    image: scratch
`)},
			{Data: []byte(`
when:
  event:
    - push
steps:
  xxx:
    image: scratch
`)},
			{Data: []byte(`
steps:
  build:
    image: scratch
`)},
		},
	}

	pipelineItems, err := b.Build()
	assert.False(t, errors.HasBlockingErrors(err))

	if len(pipelineItems) != 2 {
		t.Fatal("Should have generated 2 pipelineItems")
	}
}

func TestZeroSteps(t *testing.T) {
	t.Parallel()

	pipeline := &model.Pipeline{
		Branch: "dev",
		Event:  model.EventPush,
	}

	b := StepBuilder{
		Forge: getMockForge(),
		Repo:  &model.Repo{},
		Curr:  pipeline,
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			{Data: []byte(`
when:
  event: push
skip_clone: true
steps:
  build:
    when:
      branch: notdev
    image: scratch
`)},
		},
	}

	pipelineItems, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(pipelineItems) != 0 {
		t.Fatal("Should not generate a pipeline item if there are no steps")
	}
}

func TestZeroStepsAsMultiPipelineDeps(t *testing.T) {
	t.Parallel()

	pipeline := &model.Pipeline{
		Branch: "dev",
		Event:  model.EventPush,
	}

	b := StepBuilder{
		Forge: getMockForge(),
		Repo:  &model.Repo{},
		Curr:  pipeline,
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			{Name: "zerostep", Data: []byte(`
when:
  event: push
skip_clone: true
steps:
  build:
    when:
      branch: notdev
    image: scratch
`)},
			{Name: "justastep", Data: []byte(`
when:
  event: push
steps:
  build:
    image: scratch
`)},
			{Name: "shouldbefiltered", Data: []byte(`
when:
  event: push
steps:
  build:
    image: scratch
depends_on: [ zerostep ]
`)},
		},
	}

	pipelineItems, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(pipelineItems) != 1 {
		t.Fatal("Zerostep and the step that depends on it should not generate a pipeline item")
	}
	if pipelineItems[0].Workflow.Name != "justastep" {
		t.Fatal("justastep should have been generated")
	}
}

func TestZeroStepsAsMultiPipelineTransitiveDeps(t *testing.T) {
	t.Parallel()

	pipeline := &model.Pipeline{
		Branch: "dev",
		Event:  model.EventPush,
	}

	b := StepBuilder{
		Forge: getMockForge(),
		Repo:  &model.Repo{},
		Curr:  pipeline,
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			{Name: "zerostep", Data: []byte(`
when:
  event: push
skip_clone: true
steps:
  build:
    when:
      branch: notdev
    image: scratch
`)},
			{Name: "justastep", Data: []byte(`
when:
  event: push
steps:
  build:
    image: scratch
`)},
			{Name: "shouldbefiltered", Data: []byte(`
when:
  event: push
steps:
  build:
    image: scratch
depends_on: [ zerostep ]
`)},
			{Name: "shouldbefilteredtoo", Data: []byte(`
when:
  event: push
steps:
  build:
    image: scratch
depends_on: [ shouldbefiltered ]
`)},
		},
	}

	pipelineItems, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(pipelineItems) != 1 {
		t.Fatal("Zerostep and the step that depends on it, and the one depending on it should not generate a pipeline item")
	}
	if pipelineItems[0].Workflow.Name != "justastep" {
		t.Fatal("justastep should have been generated")
	}
}

func TestSanitizePath(t *testing.T) {
	t.Parallel()

	testTable := []struct {
		path          string
		sanitizedPath string
	}{
		{
			path:          ".crow/test.yml",
			sanitizedPath: "test",
		},
		{
			path:          ".woodpecker.yml",
			sanitizedPath: "woodpecker",
		},
		{
			path:          "folder/sub-folder/test.yml",
			sanitizedPath: "test",
		},
		{
			path:          ".crow/test.yaml",
			sanitizedPath: "test",
		},
		{
			path:          ".woodpecker.yaml",
			sanitizedPath: "woodpecker",
		},
		{
			path:          "folder/sub-folder/test.yaml",
			sanitizedPath: "test",
		},
		{
			path:          ".crow/test.jsonnet",
			sanitizedPath: "test",
		},
		{
			path:          ".crow.jsonnet",
			sanitizedPath: "crow",
		},
	}

	for _, test := range testTable {
		if test.sanitizedPath != SanitizePath(test.path) {
			t.Fatal("Path hasn't been sanitized correctly")
		}
	}
}

func getMockForge() forge.Forge {
	forge := new(forge_mocks.MockForge)
	forge.On("Name").Return("mock")
	forge.On("URL").Return("https://codeberg.org")
	return forge
}

func TestStepBuilderOrgID(t *testing.T) {
	t.Parallel()

	orgID := int64(42)
	b := StepBuilder{
		Forge: getMockForge(),
		Envs:  map[string]string{},
		Repo: &model.Repo{
			OrgID: orgID,
		},
		Curr: &model.Pipeline{
			Event: model.EventPush,
		},
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			{Data: []byte(`
when:
  event: push
steps:
  build:
    image: alpine
    commands:
      - echo "test"
  deploy:
    image: alpine
    commands:
      - echo "deploy"
`)},
		},
	}

	pipelineItems, err := b.Build()
	assert.NoError(t, err)
	assert.NotNil(t, pipelineItems)

	// Verify that all steps have the correct OrgID
	for _, item := range pipelineItems {
		for _, stage := range item.Config.Stages {
			for _, step := range stage.Steps {
				assert.Equal(t, orgID, step.OrgID, "Step %s should have OrgID %d", step.Name, orgID)
			}
		}
	}
}

func TestVariablesInterpolation(t *testing.T) {
	t.Parallel()

	b := StepBuilder{
		Forge: getMockForge(),
		Repo:  &model.Repo{},
		Curr: &model.Pipeline{
			Event: model.EventPush,
		},
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			{Data: []byte(`
variables:
  FOO_VERSION: "3.12"
  BAR_VERSION: "0.3.1"

when:
  event: push
steps:
  test:
    image: example.com/myorg/myimage:${FOO_VERSION}-${BAR_VERSION}
    commands:
      - echo test
`)},
		},
	}

	pipelineItems, err := b.Build()
	assert.NoError(t, err)
	assert.Len(t, pipelineItems, 1)

	// Find the test step (skip clone step)
	var testStep *backend_types.Step
	for _, stage := range pipelineItems[0].Config.Stages {
		for _, step := range stage.Steps {
			if step.Name == "test" {
				testStep = step
			}
		}
	}
	assert.NotNil(t, testStep)
	assert.Equal(t, "example.com/myorg/myimage:3.12-0.3.1", testStep.Image)
}

func TestExternalSecretsMaskedInLogs(t *testing.T) {
	t.Parallel()

	// Simulate fetching multiple keys from the same integration/path.
	// Before the fix, all external secrets were stored under the same compiler
	// map key "external", so only the last value survived for log masking.
	fetcher := func(_ context.Context, refs []settings.ExternalSecretRef) (map[settings.ExternalSecretRef]settings.ExternalSecret, error) {
		result := make(map[settings.ExternalSecretRef]settings.ExternalSecret, len(refs))
		for _, ref := range refs {
			switch ref.Key {
			case "username":
				result[ref] = settings.ExternalSecret{Value: "smtp-user@example.com"}
			case "password":
				result[ref] = settings.ExternalSecret{Value: "super-secret-password"}
			default:
				return nil, fmt.Errorf("unknown key: %s", ref.Key)
			}
		}
		return result, nil
	}

	b := StepBuilder{
		Forge: getMockForge(),
		Repo:  &model.Repo{},
		Curr: &model.Pipeline{
			Event: model.EventPush,
		},
		Prev:                  &model.Pipeline{},
		Netrc:                 &model.Netrc{},
		Secs:                  []*model.Secret{},
		Regs:                  []*model.Registry{},
		Host:                  "",
		ExternalSecretFetcher: fetcher,
		Yamls: []*forge_types.FileMeta{
			{Data: []byte(`
when:
  event: push
steps:
  deploy:
    image: alpine
    environment:
      SMTP_USER:
        from_secret:
          integration: 1
          path: zoho/smtp
          key: username
      SMTP_PASS:
        from_secret:
          integration: 1
          path: zoho/smtp
          key: password
`)},
		},
	}

	pipelineItems, err := b.Build()
	assert.NoError(t, err)
	assert.Len(t, pipelineItems, 1)

	// Collect all secret values registered for masking
	maskedValues := make(map[string]bool)
	for _, secret := range pipelineItems[0].Config.Secrets {
		maskedValues[secret.Value] = true
	}

	assert.True(t, maskedValues["smtp-user@example.com"],
		"username secret should be masked in logs")
	assert.True(t, maskedValues["super-secret-password"],
		"password secret should be masked in logs")
}

func TestExternalSecretFilters(t *testing.T) {
	t.Parallel()

	const workflowWhen = `
when:
  event: [push, tag, pull_request, pull_request_closed]
`
	commandsStep := workflowWhen + `
steps:
  run:
    image: alpine
    commands: [env]
    environment:
      TOKEN:
        from_secret:
          integration: vault
          path: app
          key: token
`
	pluginStep := func(image string) string {
		return workflowWhen + `
steps:
  publish:
    image: ` + image + `
    settings:
      token:
        from_secret:
          integration: vault
          path: app
          key: token
`
	}

	tests := []struct {
		name           string
		event          model.WebhookEvent
		events         []string
		plugins        []string
		yaml           string
		wantErr        string
		wantStep       string
		wantEnvKey     string
		wantNoFetching bool
	}{
		{
			name:       "no filters allow a commands step",
			event:      model.EventPush,
			yaml:       commandsStep,
			wantStep:   "run",
			wantEnvKey: "TOKEN",
		},
		{
			name:       "event filter allows matching event",
			event:      model.EventPush,
			events:     []string{"push"},
			yaml:       commandsStep,
			wantStep:   "run",
			wantEnvKey: "TOKEN",
		},
		{
			name:    "event filter denies pull_request",
			event:   model.EventPull,
			events:  []string{"push", "tag"},
			yaml:    commandsStep,
			wantErr: `is not allowed to be used with pipeline event "pull_request"`,
		},
		{
			name:       "event filter normalizes pull_request_closed",
			event:      model.EventPullClosed,
			events:     []string{"pull_request"},
			yaml:       commandsStep,
			wantStep:   "run",
			wantEnvKey: "TOKEN",
		},
		{
			name:    "plugin filter denies a commands step",
			event:   model.EventPush,
			plugins: []string{"plugins/s3"},
			yaml:    commandsStep,
			wantErr: "is only allowed to be used by plugins",
		},
		{
			name:       "plugin filter allows a matching plugin",
			event:      model.EventPush,
			plugins:    []string{"plugins/s3"},
			yaml:       pluginStep("plugins/s3"),
			wantStep:   "publish",
			wantEnvKey: "PLUGIN_TOKEN",
		},
		{
			name:    "plugin filter denies another plugin",
			event:   model.EventPush,
			plugins: []string{"plugins/s3"},
			yaml:    pluginStep("evil/exfiltrate"),
			wantErr: `is not allowed to be used with image "evil/exfiltrate"`,
		},
		{
			name:  "skipped step does not fetch",
			event: model.EventPush,
			yaml: workflowWhen + `
steps:
  build:
    image: alpine
    commands: ["true"]
  deploy:
    image: alpine
    commands: [env]
    when:
      event: tag
    environment:
      TOKEN:
        from_secret:
          integration: vault
          path: app
          key: token
`,
			wantStep:       "build",
			wantNoFetching: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var fetchedRefs []settings.ExternalSecretRef
			fetcher := func(_ context.Context, refs []settings.ExternalSecretRef) (map[settings.ExternalSecretRef]settings.ExternalSecret, error) {
				fetchedRefs = append(fetchedRefs, refs...)
				result := make(map[settings.ExternalSecretRef]settings.ExternalSecret, len(refs))
				for _, ref := range refs {
					result[ref] = settings.ExternalSecret{
						Value:          "external-token-value",
						Events:         tt.events,
						AllowedPlugins: tt.plugins,
					}
				}
				return result, nil
			}

			b := StepBuilder{
				Forge:                 getMockForge(),
				Repo:                  &model.Repo{},
				Curr:                  &model.Pipeline{Event: tt.event},
				Prev:                  &model.Pipeline{},
				Netrc:                 &model.Netrc{},
				Secs:                  []*model.Secret{},
				Regs:                  []*model.Registry{},
				ExternalSecretFetcher: fetcher,
				Yamls:                 []*forge_types.FileMeta{{Data: []byte(tt.yaml)}},
			}

			items, err := b.Build()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, items, 1)

			if tt.wantNoFetching {
				assert.Empty(t, fetchedRefs, "secrets of skipped steps must not be fetched")
				for _, secret := range items[0].Config.Secrets {
					assert.NotEqual(t, "external-token-value", secret.Value)
				}
				return
			}

			var step *backend_types.Step
			for _, stage := range items[0].Config.Stages {
				for _, s := range stage.Steps {
					if s.Name == tt.wantStep {
						step = s
					}
				}
			}
			require.NotNil(t, step)
			assert.Equal(t, "external-token-value", step.Environment[tt.wantEnvKey])
			assert.Equal(t, "external-token-value", step.SecretMapping[tt.wantEnvKey],
				"external secrets must be in the secret mapping so backends treat them as secrets")

			masked := false
			for _, secret := range items[0].Config.Secrets {
				if secret.Value == "external-token-value" {
					masked = true
				}
			}
			assert.True(t, masked, "external secret must be registered for log masking")
		})
	}
}

func TestVariablesDoNotOverrideCIVars(t *testing.T) {
	t.Parallel()

	b := StepBuilder{
		Forge: getMockForge(),
		Repo:  &model.Repo{},
		Curr: &model.Pipeline{
			Event:   model.EventPush,
			Message: "real commit message",
		},
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		Yamls: []*forge_types.FileMeta{
			{Data: []byte(`
variables:
  CI_COMMIT_MESSAGE: "should not override"

when:
  event: push
steps:
  test:
    image: scratch
    settings:
      msg: ${CI_COMMIT_MESSAGE}
`)},
		},
	}

	pipelineItems, err := b.Build()
	assert.NoError(t, err)
	assert.Len(t, pipelineItems, 1)

	// CI_COMMIT_MESSAGE should come from the pipeline, not from variables
	env := pipelineItems[0].Config.Stages[0].Steps[0].Environment
	assert.Equal(t, "real commit message", env["CI_COMMIT_MESSAGE"])
}

func TestManualVariablesOverrideDefaults(t *testing.T) {
	t.Parallel()

	// A manual-run variable (passed via Envs / AdditionalVariables) must take
	// precedence over the matching default declared in the "variables:" block,
	// both for substitution and for when-filter evaluation. Otherwise a manual
	// override can never differ from the declared default.
	b := StepBuilder{
		Forge: getMockForge(),
		Repo:  &model.Repo{},
		Curr: &model.Pipeline{
			Event: model.EventManual,
		},
		Prev:  &model.Pipeline{},
		Netrc: &model.Netrc{},
		Secs:  []*model.Secret{},
		Regs:  []*model.Registry{},
		Host:  "",
		// simulates the manual-run form passing target_arch=arm64
		Envs: map[string]string{"target_arch": "arm64"},
		Yamls: []*forge_types.FileMeta{
			{Data: []byte(`
variables:
  target_arch: amd64

when:
  - event: manual
    evaluate: 'target_arch == "arm64"'
steps:
  test:
    image: example.com/build:${target_arch}
    commands:
      - echo test
`)},
		},
	}

	pipelineItems, err := b.Build()
	assert.NoError(t, err)
	// the override makes the when-filter match, so the workflow is not filtered out
	assert.Len(t, pipelineItems, 1)

	var testStep *backend_types.Step
	for _, stage := range pipelineItems[0].Config.Stages {
		for _, step := range stage.Steps {
			if step.Name == "test" {
				testStep = step
			}
		}
	}
	assert.NotNil(t, testStep)
	assert.Equal(t, "example.com/build:arm64", testStep.Image)
}

// Build must record every YAML document it evaluated out of a .jsonnet source, so
// the server can persist the evaluation and the config view can show what was
// actually parsed rather than only the Jsonnet that produced it.
func TestStepBuilderRecordsGeneratedConfigs(t *testing.T) {
	t.Parallel()

	newBuilder := func(yamls []*forge_types.FileMeta) *StepBuilder {
		return &StepBuilder{
			Forge: getMockForge(),
			Envs:  map[string]string{},
			Repo:  &model.Repo{},
			Curr:  &model.Pipeline{Event: model.EventPush},
			Prev:  &model.Pipeline{},
			Netrc: &model.Netrc{},
			Secs:  []*model.Secret{},
			Regs:  []*model.Registry{},
			Yamls: yamls,
		}
	}

	singleDoc := []byte(`{
		when: { event: "push" },
		steps: { build: { image: "alpine", commands: ["echo hi"] } },
	}`)

	multiDoc := []byte(`[
		{
			name: "build-" + os,
			when: { event: "push" },
			steps: { build: { image: "alpine", commands: ["echo " + os] } },
		}
		for os in ["linux", "darwin"]
	]`)

	t.Run("SingleDocumentIsNamedAfterItsContent", func(t *testing.T) {
		t.Parallel()

		b := newBuilder([]*forge_types.FileMeta{{Name: ".crow/build.jsonnet", Data: singleDoc}})
		_, err := b.Build()
		require.NoError(t, err)

		require.Len(t, b.GeneratedConfigs, 1)
		// The evaluated document is YAML, so it is recorded as such even though the
		// FileMeta the builder works on keeps the .jsonnet name.
		assert.Equal(t, ".crow/build.yaml", b.GeneratedConfigs[0].Name)
		assert.Equal(t, ".crow/build.jsonnet", b.GeneratedConfigs[0].SourceName)
		assert.Contains(t, string(b.GeneratedConfigs[0].Data), "image: alpine")
	})

	t.Run("ArrayYieldsOneRecordPerDocument", func(t *testing.T) {
		t.Parallel()

		b := newBuilder([]*forge_types.FileMeta{{Name: ".crow/matrix.jsonnet", Data: multiDoc}})
		_, err := b.Build()
		require.NoError(t, err)

		require.Len(t, b.GeneratedConfigs, 2)
		assert.Equal(t, ".crow/matrix-0.yaml", b.GeneratedConfigs[0].Name)
		assert.Equal(t, ".crow/matrix-1.yaml", b.GeneratedConfigs[1].Name)
		assert.Contains(t, string(b.GeneratedConfigs[0].Data), "build-linux")
		assert.Contains(t, string(b.GeneratedConfigs[1].Data), "build-darwin")
		for _, generated := range b.GeneratedConfigs {
			assert.Equal(t, ".crow/matrix.jsonnet", generated.SourceName)
		}
	})

	t.Run("PlainYAMLRecordsNothing", func(t *testing.T) {
		t.Parallel()

		b := newBuilder([]*forge_types.FileMeta{
			{Name: ".crow/release.yaml", Data: []byte("when:\n  event: push\nsteps:\n  build:\n    image: alpine\n    commands:\n      - echo hi\n")},
		})
		_, err := b.Build()
		require.NoError(t, err)

		assert.Empty(t, b.GeneratedConfigs)
	})
}
