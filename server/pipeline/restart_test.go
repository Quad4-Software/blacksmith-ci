package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v6/server"
	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	forge_types "codefloe.com/crowci/crow/v6/server/forge/types"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pubsub"
	config_mocks "codefloe.com/crowci/crow/v6/server/services/config/mocks"
	manager_mocks "codefloe.com/crowci/crow/v6/server/services/mocks"
	registry_mocks "codefloe.com/crowci/crow/v6/server/services/registry/mocks"
	secret_mocks "codefloe.com/crowci/crow/v6/server/services/secret/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func setupRestartMocks(t *testing.T, repo *model.Repo, lastPipeline *model.Pipeline, fetchedConfigs []*forge_types.FileMeta) (
	*store_mocks.MockStore, func(),
) {
	t.Helper()

	mockStore := new(store_mocks.MockStore)
	mockForge := new(forge_mocks.MockForge)
	mockConfigService := new(config_mocks.MockService)

	// Secret and registry service mocks
	mockSecretService := new(secret_mocks.MockService)
	mockSecretService.On("SecretListPipeline", mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	mockRegistryService := new(registry_mocks.MockService)
	mockRegistryService.On("RegistryListPipeline", mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	// Config service — return whatever the test provides
	mockConfigService.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, true,
	).Return(fetchedConfigs, nil)

	// Manager
	mockManager := new(manager_mocks.MockManager)
	mockManager.On("ForgeFromRepo", repo).Return(mockForge, nil)
	mockManager.On("ConfigServiceFromRepo", repo).Return(mockConfigService)
	mockManager.On("SecretServiceFromRepo", mock.Anything).Return(mockSecretService).Maybe()
	mockManager.On("RegistryServiceFromRepo", mock.Anything).Return(mockRegistryService).Maybe()
	mockManager.On("EnvironmentService").Return(nil).Maybe()

	// Store — common calls
	mockStore.On("ConfigsForPipeline", lastPipeline.ID).Return([]*model.Config{}, nil)
	mockStore.On("CreatePipeline", mock.AnythingOfType("*model.Pipeline")).Return(nil)
	mockStore.On("UpdatePipeline", mock.AnythingOfType("*model.Pipeline")).Return(nil).Maybe()
	mockStore.On("GetPipelineLastBefore", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	mockStore.On("WorkflowsCreate", mock.Anything).Return(nil).Maybe()
	mockStore.On("WorkflowGetTree", mock.Anything).Return([]*model.Workflow{}, nil).Maybe()

	// Forge — mock all methods that parsePipeline/stepBuilder might call
	mockForge.On("Netrc", mock.Anything, mock.Anything).Return(&model.Netrc{}, nil).Maybe()
	mockForge.On("Name").Return("test").Maybe()
	mockForge.On("URL").Return("https://test.example.com").Maybe()
	mockForge.On("Status", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	// Save and restore globals
	originalManager := server.Config.Services.Manager
	originalPubsub := server.Config.Services.Pubsub
	server.Config.Services.Manager = mockManager
	server.Config.Services.Pubsub = pubsub.New()

	cleanup := func() {
		server.Config.Services.Manager = originalManager
		server.Config.Services.Pubsub = originalPubsub
	}

	return mockStore, cleanup
}

func TestRestartWithEmptyDBConfigsFetchesFromForge(t *testing.T) {
	repo := &model.Repo{ID: 1, ForgeID: 1, FullName: "test/repo"}
	user := &model.User{ID: 1, Login: "testuser"}
	lastPipeline := &model.Pipeline{
		ID:     10,
		RepoID: 1,
		Number: 5,
		Status: model.StatusError,
		Event:  model.EventPush,
	}

	freshConfig := []*forge_types.FileMeta{
		{Name: ".crow.yml", Data: []byte("# empty config")},
	}

	persistedConfig := &model.Config{
		ID:     1,
		RepoID: 1,
		Name:   ".crow.yml",
		Data:   []byte("# empty config"),
	}

	mockStore, cleanup := setupRestartMocks(t, repo, lastPipeline, freshConfig)
	defer cleanup()

	// Fresh configs get persisted and linked
	mockStore.On("ConfigPersist", mock.AnythingOfType("*model.Config")).Return(persistedConfig, nil)
	mockStore.On("PipelineConfigCreate", mock.AnythingOfType("*model.PipelineConfig")).Return(nil)

	// Execute
	_, err := Restart(context.Background(), mockStore, lastPipeline, user, repo, map[string]string{})

	// ConfigPersist must have been called — fresh configs were persisted
	mockStore.AssertCalled(t, "ConfigPersist", mock.AnythingOfType("*model.Config"))
	mockStore.AssertCalled(t, "PipelineConfigCreate", mock.AnythingOfType("*model.PipelineConfig"))

	// The error (if any) should be from downstream processing, not the stale config error
	if err != nil {
		assert.NotContains(t, err.Error(), "failed to load pipeline config from database or forge")
	}
}

func TestRestartWithEmptyDBConfigsAndEmptyForgeFetchReturnsError(t *testing.T) {
	repo := &model.Repo{ID: 1, ForgeID: 1, FullName: "test/repo"}
	user := &model.User{ID: 1, Login: "testuser"}
	lastPipeline := &model.Pipeline{
		ID:     10,
		RepoID: 1,
		Number: 5,
		Status: model.StatusError,
		Event:  model.EventPush,
	}

	mockStore, cleanup := setupRestartMocks(t, repo, lastPipeline, nil)
	defer cleanup()

	// Execute
	result, err := Restart(context.Background(), mockStore, lastPipeline, user, repo, map[string]string{})

	// Should return the pipeline with error status, not a Go error
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, model.StatusError, result.Status)

	// Should NOT have tried to persist any configs
	mockStore.AssertNotCalled(t, "ConfigPersist", mock.Anything)
}

func TestCreateNewOutOfOld(t *testing.T) {
	tests := []struct {
		name string
		old  *model.Pipeline
	}{
		{
			name: "pipeline with existing title",
			old: &model.Pipeline{
				ID:     100,
				Number: 42,
				Title:  "Fix bug in authentication",
				Status: model.StatusSuccess,
			},
		},
		{
			name: "pipeline without title",
			old: &model.Pipeline{
				ID:     200,
				Number: 73,
				Title:  "",
				Status: model.StatusFailure,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newPipeline := createNewOutOfOld(tt.old)

			// Check that the new pipeline has been reset properly
			assert.Equal(t, int64(0), newPipeline.ID)
			assert.Equal(t, int64(0), newPipeline.Number)
			assert.Equal(t, model.StatusPending, newPipeline.Status)
			assert.Equal(t, int64(0), newPipeline.Started)
			assert.Equal(t, int64(0), newPipeline.Finished)
			assert.Nil(t, newPipeline.Errors)

			// Check that the title is preserved unchanged
			assert.Equal(t, tt.old.Title, newPipeline.Title)
		})
	}
}
