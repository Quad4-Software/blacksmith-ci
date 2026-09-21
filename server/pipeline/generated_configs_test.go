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

package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pubsub"
	"codefloe.com/crowci/crow/v6/server/queue"
	config_mocks "codefloe.com/crowci/crow/v6/server/services/config/mocks"
	manager_mocks "codefloe.com/crowci/crow/v6/server/services/mocks"
	registry_mocks "codefloe.com/crowci/crow/v6/server/services/registry/mocks"
	secret_mocks "codefloe.com/crowci/crow/v6/server/services/secret/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

// generatedConfigsFixture is a pipeline whose stored configs include a
// .jsonnet source and the two YAML documents it evaluated to. Every code path
// that re-parses a stored pipeline must hand only the source to the step
// builder; feeding the generated documents back in evaluates the source a
// second time and builds every workflow twice.
//
// Unlike Restart, neither Approve nor the rerun paths go through the config
// service, so the only observable is the set of workflows they end up with.
type generatedConfigsFixture struct {
	repo     *model.Repo
	user     *model.User
	pipeline *model.Pipeline
	store    *store_mocks.MockStore
	forge    *forge_mocks.MockForge
}

func newGeneratedConfigsFixture(t *testing.T, status model.StatusValue) *generatedConfigsFixture {
	t.Helper()

	f := &generatedConfigsFixture{
		repo: &model.Repo{ID: 1, ForgeID: 1, FullName: "test/repo"},
		user: &model.User{ID: 1, Login: "testuser"},
		pipeline: &model.Pipeline{
			ID:     10,
			RepoID: 1,
			Number: 5,
			Status: status,
			Event:  model.EventPush,
		},
		store: new(store_mocks.MockStore),
		forge: new(forge_mocks.MockForge),
	}

	jsonnetSource := []byte(`[
		{
			name: "build-" + os,
			when: { event: "push" },
			steps: { build: { image: "alpine", commands: ["echo " + os] } },
		}
		for os in ["linux", "darwin"]
	]`)

	storedConfigs := []*model.Config{
		{ID: 1, RepoID: 1, Name: ".crow/matrix.jsonnet", Data: jsonnetSource},
		{ID: 2, RepoID: 1, Name: ".crow/matrix-0.yaml", Data: generatedDoc("linux"), GeneratedFrom: ".crow/matrix.jsonnet"},
		{ID: 3, RepoID: 1, Name: ".crow/matrix-1.yaml", Data: generatedDoc("darwin"), GeneratedFrom: ".crow/matrix.jsonnet"},
	}

	mockConfigService := new(config_mocks.MockService)

	mockSecretService := new(secret_mocks.MockService)
	mockSecretService.On("SecretListPipeline", mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	mockRegistryService := new(registry_mocks.MockService)
	mockRegistryService.On("RegistryListPipeline", mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	mockManager := new(manager_mocks.MockManager)
	mockManager.On("ForgeFromRepo", f.repo).Return(f.forge, nil)
	mockManager.On("ConfigServiceFromRepo", f.repo).Return(mockConfigService)
	mockManager.On("SecretServiceFromRepo", mock.Anything).Return(mockSecretService).Maybe()
	mockManager.On("RegistryServiceFromRepo", mock.Anything).Return(mockRegistryService).Maybe()
	mockManager.On("EnvironmentService").Return(nil).Maybe()

	f.store.On("ConfigsForPipeline", f.pipeline.ID).Return(storedConfigs, nil)
	f.store.On("UpdatePipeline", mock.AnythingOfType("*model.Pipeline")).Return(nil).Maybe()
	f.store.On("GetPipelineLastBefore", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	f.store.On("ConfigPersist", mock.AnythingOfType("*model.Config")).Return(&model.Config{ID: 99}, nil).Maybe()
	f.store.On("PipelineConfigCreate", mock.AnythingOfType("*model.PipelineConfig")).Return(nil).Maybe()
	f.store.On("GetPipelineCountByRepo", mock.Anything).Return(int64(1), nil).Maybe()
	f.store.On("GetRepo", mock.Anything).Return(f.repo, nil).Maybe()

	f.forge.On("Netrc", mock.Anything, mock.Anything).Return(&model.Netrc{}, nil).Maybe()
	f.forge.On("Name").Return("test").Maybe()
	f.forge.On("URL").Return("https://test.example.com").Maybe()
	f.forge.On("Status", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	originalManager := server.Config.Services.Manager
	originalPubsub := server.Config.Services.Pubsub
	originalQueue := server.Config.Services.Queue
	server.Config.Services.Manager = mockManager
	server.Config.Services.Pubsub = pubsub.New()
	server.Config.Services.Queue = queue.NewMemoryQueue(t.Context(), f.store)
	t.Cleanup(func() {
		server.Config.Services.Manager = originalManager
		server.Config.Services.Pubsub = originalPubsub
		server.Config.Services.Queue = originalQueue
	})

	return f
}

// generatedDoc is what the jsonnet above evaluates to for one os: a complete,
// parseable workflow, so feeding it back in silently doubles the workflow
// rather than failing loudly.
func generatedDoc(os string) []byte {
	return []byte("name: build-" + os + "\nwhen:\n  event: push\nsteps:\n  build:\n    image: alpine\n    commands:\n      - echo " + os + "\n")
}

func workflowNames(workflows []*model.Workflow) []string {
	names := make([]string, 0, len(workflows))
	for _, workflow := range workflows {
		names = append(names, workflow.Name)
	}
	return names
}

// A blocked Jsonnet pipeline already has its generated documents linked from
// creation, so approval is the first path after Create that reads them back.
func TestApproveDoesNotFeedGeneratedConfigsBackIn(t *testing.T) {
	f := newGeneratedConfigsFixture(t, model.StatusBlocked)

	blocked := []*model.Workflow{
		{ID: 1, PipelineID: 10, PID: 1, Name: "build-linux", State: model.StatusBlocked},
		{ID: 2, PipelineID: 10, PID: 2, Name: "build-darwin", State: model.StatusBlocked},
	}
	f.store.On("WorkflowGetTree", mock.Anything).Return(blocked, nil)
	f.store.On("WorkflowUpdate", mock.Anything).Return(nil)
	f.store.On("StepUpdate", mock.Anything).Return(nil).Maybe()

	var replaced []*model.Workflow
	f.store.On("WorkflowsReplace", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		workflows, ok := args.Get(1).([]*model.Workflow)
		require.True(t, ok, "expected []*model.Workflow, got %T", args.Get(1))
		replaced = workflows
	}).Return(nil)

	pipeline, err := Approve(context.Background(), f.store, f.pipeline, f.user, f.repo)
	require.NoError(t, err)

	// Two workflows, not four: the jsonnet was evaluated exactly once.
	assert.ElementsMatch(t, []string{"build-linux", "build-darwin"}, workflowNames(replaced))
	if assert.NotNil(t, pipeline) {
		assert.ElementsMatch(t, []string{"build-linux", "build-darwin"}, workflowNames(pipeline.Workflows))
	}
}

func TestRerunWorkflowDoesNotFeedGeneratedConfigsBackIn(t *testing.T) {
	f := newGeneratedConfigsFixture(t, model.StatusFailure)

	finished := []*model.Workflow{
		{ID: 1, PipelineID: 10, PID: 1, Name: "build-linux", State: model.StatusFailure},
		{ID: 2, PipelineID: 10, PID: 2, Name: "build-darwin", State: model.StatusSuccess},
	}
	f.store.On("WorkflowGetTree", mock.Anything).Return(finished, nil)

	var created []*model.Workflow
	f.store.On("WorkflowsCreate", mock.Anything).Run(func(args mock.Arguments) {
		workflows, ok := args.Get(0).([]*model.Workflow)
		require.True(t, ok, "expected []*model.Workflow, got %T", args.Get(0))
		created = workflows
	}).Return(nil)

	err := RerunWorkflow(context.Background(), f.forge, f.store, f.pipeline, 1, f.user, f.repo)
	require.NoError(t, err)

	// One new attempt of build-linux, not two.
	assert.Equal(t, []string{"build-linux"}, workflowNames(created))
}
