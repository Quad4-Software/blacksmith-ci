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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	forge_types "codefloe.com/crowci/crow/v6/server/forge/types"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pipeline/stepbuilder"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

// Regression test for codefloe.com/crowci/crow#1029: persisting a jsonnet
// config under a name without the .jsonnet extension makes preprocessJsonnet
// unable to recognize it on restart, causing the raw jsonnet source to be
// passed to the YAML parser. The stored name must preserve the extension.
func TestFindOrPersistPipelineConfigPreservesJsonnetExtension(t *testing.T) {
	mockStore := new(store_mocks.MockStore)
	pipeline := &model.Pipeline{ID: 1, RepoID: 42}

	var capturedConfig *model.Config
	mockStore.On("ConfigPersist", mock.AnythingOfType("*model.Config")).
		Run(func(args mock.Arguments) {
			cfg, ok := args.Get(0).(*model.Config)
			if !ok {
				t.Fatalf("expected *model.Config, got %T", args.Get(0))
			}
			capturedConfig = cfg
		}).
		Return(&model.Config{ID: 100}, nil)

	forgeConfig := &forge_types.FileMeta{
		Name: ".crow/test.jsonnet",
		Data: []byte("{ name: 'x' }"),
	}

	_, err := findOrPersistPipelineConfig(mockStore, pipeline, forgeConfig)

	assert.NoError(t, err)
	assert.NotNil(t, capturedConfig)
	assert.Contains(t, capturedConfig.Name, ".jsonnet",
		"persisted name must keep the .jsonnet extension so preprocessJsonnet recognizes it on restart")
}

func TestFindOrPersistPipelineConfigPreservesLibsonnetExtension(t *testing.T) {
	mockStore := new(store_mocks.MockStore)
	pipeline := &model.Pipeline{ID: 1, RepoID: 42}

	var capturedConfig *model.Config
	mockStore.On("ConfigPersist", mock.AnythingOfType("*model.Config")).
		Run(func(args mock.Arguments) {
			cfg, ok := args.Get(0).(*model.Config)
			if !ok {
				t.Fatalf("expected *model.Config, got %T", args.Get(0))
			}
			capturedConfig = cfg
		}).
		Return(&model.Config{ID: 100}, nil)

	forgeConfig := &forge_types.FileMeta{
		Name: ".crow/lib.libsonnet",
		Data: []byte("{ helper:: 'value' }"),
	}

	_, err := findOrPersistPipelineConfig(mockStore, pipeline, forgeConfig)

	assert.NoError(t, err)
	assert.NotNil(t, capturedConfig)
	assert.Contains(t, capturedConfig.Name, ".libsonnet")
}

func TestForgeConfigsDropsGeneratedConfigs(t *testing.T) {
	configs := []*model.Config{
		{ID: 1, Name: ".crow/lib.libsonnet"},
		{ID: 2, Name: ".crow/matrix.jsonnet"},
		{ID: 3, Name: ".crow/matrix-0.yaml", GeneratedFrom: ".crow/matrix.jsonnet"},
		{ID: 4, Name: ".crow/matrix-1.yaml", GeneratedFrom: ".crow/matrix.jsonnet"},
		{ID: 5, Name: ".crow/release.yaml"},
	}

	result := forgeConfigs(configs)

	// Feeding a generated document back in would evaluate matrix.jsonnet a second
	// time and build every workflow twice.
	assert.Equal(t, []string{".crow/lib.libsonnet", ".crow/matrix.jsonnet", ".crow/release.yaml"},
		configNames(result))
}

// Every path that re-parses a stored pipeline (approve, restart, rerun) loads
// its files through this helper, so filtering here covers all of them.
func TestForgeConfigsForPipelineDropsGeneratedConfigs(t *testing.T) {
	mockStore := new(store_mocks.MockStore)
	mockStore.On("ConfigsForPipeline", int64(7)).Return([]*model.Config{
		{ID: 1, Name: ".crow/matrix.jsonnet"},
		{ID: 2, Name: ".crow/matrix-0.yaml", GeneratedFrom: ".crow/matrix.jsonnet"},
		{ID: 3, Name: ".crow/release.yaml"},
	}, nil)

	result, err := forgeConfigsForPipeline(mockStore, 7)

	require.NoError(t, err)
	assert.Equal(t, []string{".crow/matrix.jsonnet", ".crow/release.yaml"}, configNames(result))
}

func TestPersistGeneratedConfigsLinksEachDocument(t *testing.T) {
	mockStore := new(store_mocks.MockStore)
	pipeline := &model.Pipeline{ID: 7, RepoID: 42}

	mockStore.On("ConfigsForPipeline", int64(7)).Return([]*model.Config{}, nil)

	var persisted []*model.Config
	nextID := int64(100)
	mockStore.On("ConfigPersist", mock.AnythingOfType("*model.Config")).
		Run(func(args mock.Arguments) {
			cfg, ok := args.Get(0).(*model.Config)
			if !ok {
				t.Fatalf("expected *model.Config, got %T", args.Get(0))
			}
			persisted = append(persisted, cfg)
		}).
		Return(func(cfg *model.Config) *model.Config {
			nextID++
			return &model.Config{ID: nextID, Name: cfg.Name, GeneratedFrom: cfg.GeneratedFrom}
		}, nil)

	var linked []int64
	mockStore.On("PipelineConfigCreate", mock.AnythingOfType("*model.PipelineConfig")).
		Run(func(args mock.Arguments) {
			pc, ok := args.Get(0).(*model.PipelineConfig)
			if !ok {
				t.Fatalf("expected *model.PipelineConfig, got %T", args.Get(0))
			}
			linked = append(linked, pc.ConfigID)
		}).
		Return(nil)

	err := persistGeneratedConfigs(mockStore, pipeline, []*stepbuilder.GeneratedConfig{
		{Name: ".crow/matrix-0.yaml", SourceName: ".crow/matrix.jsonnet", Data: []byte("name: build\n")},
		{Name: ".crow/matrix-1.yaml", SourceName: ".crow/matrix.jsonnet", Data: []byte("name: deploy\n")},
	})

	require.NoError(t, err)
	require.Len(t, persisted, 2)
	assert.Equal(t, ".crow/matrix-0.yaml", persisted[0].Name)
	assert.Equal(t, ".crow/matrix.jsonnet", persisted[0].GeneratedFrom)
	assert.Equal(t, int64(42), persisted[0].RepoID)
	assert.Equal(t, []int64{101, 102}, linked)
}

// A blocked pipeline is parsed at creation and again on approval. The second
// pass must not trip the unique constraint on pipeline_configs.
func TestPersistGeneratedConfigsSkipsAlreadyLinked(t *testing.T) {
	mockStore := new(store_mocks.MockStore)
	pipeline := &model.Pipeline{ID: 7, RepoID: 42}

	existing := &model.Config{ID: 101, Name: ".crow/matrix-0.yaml", GeneratedFrom: ".crow/matrix.jsonnet"}
	mockStore.On("ConfigsForPipeline", int64(7)).Return([]*model.Config{existing}, nil)
	mockStore.On("ConfigPersist", mock.AnythingOfType("*model.Config")).Return(existing, nil)

	err := persistGeneratedConfigs(mockStore, pipeline, []*stepbuilder.GeneratedConfig{
		{Name: ".crow/matrix-0.yaml", SourceName: ".crow/matrix.jsonnet", Data: []byte("name: build\n")},
	})

	require.NoError(t, err)
	mockStore.AssertNotCalled(t, "PipelineConfigCreate", mock.Anything)
}

func TestPersistGeneratedConfigsNoopWithoutJsonnet(t *testing.T) {
	mockStore := new(store_mocks.MockStore)

	err := persistGeneratedConfigs(mockStore, &model.Pipeline{ID: 7, RepoID: 42}, nil)

	require.NoError(t, err)
	mockStore.AssertNotCalled(t, "ConfigsForPipeline", mock.Anything)
}

func configNames(configs []*model.Config) []string {
	names := make([]string, 0, len(configs))
	for _, config := range configs {
		names = append(names, config.Name)
	}
	return names
}
