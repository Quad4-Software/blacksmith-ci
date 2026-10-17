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

package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backend "codefloe.com/crowci/crow/v6/pipeline/backend/types"
	"codefloe.com/crowci/crow/v6/pipeline/rpc"
	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/services/buildcache"
)

func cacheTestWorkflow() *rpc.Workflow {
	return &rpc.Workflow{ID: "42", Config: &backend.Config{Stages: []*backend.Stage{
		{Steps: []*backend.Step{{Name: "clone", Type: backend.StepTypeClone}}},
		{Steps: []*backend.Step{{Name: "restore-cache", Type: backend.StepTypeCache, Environment: map[string]string{"PLUGIN_MODE": "restore"}}}},
		{Steps: []*backend.Step{{Name: "build", Type: backend.StepTypeCommands}}},
	}}}
}

func TestInjectCacheToken(t *testing.T) {
	previousService, previousHost := server.Config.Services.BuildCache, server.Config.Server.Host
	t.Cleanup(func() {
		server.Config.Services.BuildCache, server.Config.Server.Host = previousService, previousHost
	})

	t.Run("disabled", func(t *testing.T) {
		server.Config.Services.BuildCache = nil
		workflow := cacheTestWorkflow()
		require.NoError(t, injectCacheToken(workflow))
		assert.NotContains(t, workflow.Config.Stages[1].Steps[0].Environment, "CI_CACHE_TOKEN")
	})

	t.Run("enabled", func(t *testing.T) {
		server.Config.Services.BuildCache = buildcache.New(nil, nil, buildcache.Config{Secret: "secret"})
		server.Config.Server.Host = "https://ci.example.com"
		workflow := cacheTestWorkflow()
		require.NoError(t, injectCacheToken(workflow))

		cache := workflow.Config.Stages[1].Steps[0]
		assert.Equal(t, "https://ci.example.com/api/v1/cache", cache.Environment["CI_CACHE_URL"])
		assert.NotEmpty(t, cache.Environment["CI_CACHE_TOKEN"])
		assert.Equal(t, "restore", cache.Environment["PLUGIN_MODE"], "existing environment is kept")
		assert.Nil(t, workflow.Config.Stages[0].Steps[0].Environment, "clone steps get no token")
		assert.Nil(t, workflow.Config.Stages[2].Steps[0].Environment, "work steps get no token")

		require.Len(t, workflow.Config.Secrets, 1)
		assert.Equal(t, cache.Environment["CI_CACHE_TOKEN"], workflow.Config.Secrets[0].Value, "the token is masked in logs")
	})
}
