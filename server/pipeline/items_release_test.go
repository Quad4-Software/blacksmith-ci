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

	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	forge_types "codefloe.com/crowci/crow/v6/server/forge/types"
	"codefloe.com/crowci/crow/v6/server/model"
	integration_mocks "codefloe.com/crowci/crow/v6/server/services/integration/mocks"
)

// Once the workflows are built, the external secrets live in the compiled
// configs, so the pipeline's cached providers and secret values are released.
func TestParsePipelineReleasesIntegrationCache(t *testing.T) {
	f := newGeneratedConfigsFixture(t, model.StatusPending)

	integrations := integration_mocks.NewMockService(t)
	integrations.EXPECT().ReleasePipeline(f.pipeline.ID).Once()
	original := server.Config.Services.IntegrationService
	server.Config.Services.IntegrationService = integrations
	t.Cleanup(func() { server.Config.Services.IntegrationService = original })

	yamls := []*forge_types.FileMeta{{Name: ".crow/build.yaml", Data: generatedDoc("linux")}}
	items, _, err := parsePipeline(f.forge, f.store, f.pipeline, f.user, f.repo, yamls, nil, nil)
	require.NoError(t, err)
	require.Len(t, items, 1)
}
