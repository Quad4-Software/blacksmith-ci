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

	forge_types "codefloe.com/crowci/crow/v5/server/forge/types"
	"codefloe.com/crowci/crow/v5/server/model"
	store_mocks "codefloe.com/crowci/crow/v5/server/store/mocks"
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
