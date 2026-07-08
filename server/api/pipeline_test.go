// Copyright 2024 Woodpecker Authors
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

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v6/server"
	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	"codefloe.com/crowci/crow/v6/server/model"
	mocks_manager "codefloe.com/crowci/crow/v6/server/services/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

var fakePipeline = &model.Pipeline{
	ID:     2,
	Number: 2,
	Status: model.StatusSuccess,
}

func TestGetPipelines(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("should get pipelines", func(t *testing.T) {
		pipelines := []*model.Pipeline{fakePipeline}

		mockStore := new(store_mocks.MockStore)
		mockStore.On("GetPipelineList", mock.Anything, mock.Anything, mock.Anything).Return(pipelines, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)

		output, err := GetPipelines(testCtx(c), &GetPipelinesInput{})

		assert.NoError(t, err)
		assert.NotNil(t, output)
		mockStore.AssertCalled(t, "GetPipelineList", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should not parse pipeline filter", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())

		_, err := GetPipelines(testCtx(c), &GetPipelinesInput{
			Before: "2023-01-16",
			After:  "2023-01-15",
		})

		var humaErr huma.StatusError
		assert.ErrorAs(t, err, &humaErr)
		assert.Equal(t, http.StatusBadRequest, humaErr.GetStatus())
	})

	t.Run("should parse pipeline filter", func(t *testing.T) {
		pipelines := []*model.Pipeline{fakePipeline}

		mockStore := new(store_mocks.MockStore)
		mockStore.On("GetPipelineList", mock.Anything, mock.Anything, mock.Anything).Return(pipelines, nil)

		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("store", mockStore)

		output, err := GetPipelines(testCtx(c), &GetPipelinesInput{
			After: "2023-01-15T15:00:00Z",
		})

		assert.NoError(t, err)
		assert.NotNil(t, output)
	})

	t.Run("should parse pipeline filter with tz offset", func(t *testing.T) {
		pipelines := []*model.Pipeline{fakePipeline}

		mockStore := new(store_mocks.MockStore)
		mockStore.On("GetPipelineList", mock.Anything, mock.Anything, mock.Anything).Return(pipelines, nil)

		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("store", mockStore)

		output, err := GetPipelines(testCtx(c), &GetPipelinesInput{
			Before: "2023-01-16T15:00:00+01:00",
			After:  "2023-01-15T15:00:00+01:00",
		})

		assert.NoError(t, err)
		assert.NotNil(t, output)
	})

	t.Run("should filter pipelines by events", func(t *testing.T) {
		pipelines := []*model.Pipeline{fakePipeline}
		mockStore := new(store_mocks.MockStore)
		mockStore.On("GetPipelineList", mock.Anything, mock.Anything, mock.Anything).Return(pipelines, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)

		output, err := GetPipelines(testCtx(c), &GetPipelinesInput{
			Event: "push,pull_request",
		})

		assert.NoError(t, err)
		assert.NotNil(t, output)
		mockStore.AssertCalled(t, "GetPipelineList", mock.Anything, mock.Anything, &model.PipelineFilter{
			Events: model.WebhookEventList{model.EventPush, model.EventPull},
		})
	})
}

func TestDeletePipeline(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("should delete pipeline", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("GetPipelineNumber", mock.Anything, mock.Anything).Return(fakePipeline, nil)
		mockStore.On("DeletePipeline", mock.Anything).Return(nil)

		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("store", mockStore)

		_, err := DeletePipeline(testCtx(c), &DeletePipelineInput{Number: 2})

		assert.NoError(t, err)
		mockStore.AssertCalled(t, "GetPipelineNumber", mock.Anything, mock.Anything)
		mockStore.AssertCalled(t, "DeletePipeline", mock.Anything)
	})

	// Note: The old "should not delete without pipeline number" test validated gin param
	// parsing of string path params. With huma, Number is int64 and validated by the
	// framework before the handler is called, so that case is no longer testable here.

	t.Run("should not delete pending", func(t *testing.T) {
		fakePipeline := *fakePipeline
		fakePipeline.Status = model.StatusPending

		mockStore := new(store_mocks.MockStore)
		mockStore.On("GetPipelineNumber", mock.Anything, mock.Anything).Return(&fakePipeline, nil)

		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("store", mockStore)

		_, err := DeletePipeline(testCtx(c), &DeletePipelineInput{Number: 2})

		var humaErr huma.StatusError
		assert.ErrorAs(t, err, &humaErr)
		assert.Equal(t, http.StatusUnprocessableEntity, humaErr.GetStatus())
		mockStore.AssertCalled(t, "GetPipelineNumber", mock.Anything, mock.Anything)
		mockStore.AssertNotCalled(t, "DeletePipeline", mock.Anything)
	})
}

func TestGetPipelineMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prevPipeline := &model.Pipeline{
		ID:     1,
		Number: 1,
		Status: model.StatusFailure,
	}

	fakeRepo := &model.Repo{ID: 1}

	// mockForge := forge_new(forge_mocks.MockForge)
	mockForge := new(forge_mocks.MockForge)
	mockForge.On("Name").Return("mock")
	mockForge.On("URL").Return("https://codeberg.org")

	mockManager := new(mocks_manager.MockManager)
	mockManager.On("ForgeFromRepo", fakeRepo).Return(mockForge, nil)
	server.Config.Services.Manager = mockManager

	mockStore := new(store_mocks.MockStore)
	mockStore.On("GetPipelineNumber", mock.Anything, int64(2)).Return(fakePipeline, nil)
	mockStore.On("GetPipelineLastBefore", mock.Anything, mock.Anything, int64(2)).Return(prevPipeline, nil)

	t.Run("PipelineMetadata", func(t *testing.T) {
		t.Run("should get pipeline metadata", func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("store", mockStore)
			c.Set("forge", mockForge)
			c.Set("repo", fakeRepo)

			output, err := GetPipelineMetadata(testCtx(c), &GetPipelineMetadataInput{Number: 2})

			assert.NoError(t, err)
			assert.NotNil(t, output)

			response := output.Body
			assert.Equal(t, int64(1), response.Repo.ID)
			assert.Equal(t, int64(2), response.Curr.Number)
			assert.Equal(t, int64(1), response.Prev.Number)
		})

		// Note: The old "should return bad request for invalid pipeline number" test validated
		// gin param parsing of string "invalid". With huma, Number is int64 and validated by
		// the framework before the handler is called, so that case is no longer testable here.

		t.Run("should return not found for non-existent pipeline", func(t *testing.T) {
			mockStore := new(store_mocks.MockStore)
			mockStore.On("GetPipelineNumber", mock.Anything, int64(3)).Return((*model.Pipeline)(nil), types.RecordNotExist)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("store", mockStore)
			c.Set("repo", fakeRepo)

			_, err := GetPipelineMetadata(testCtx(c), &GetPipelineMetadataInput{Number: 3})

			var humaErr huma.StatusError
			assert.ErrorAs(t, err, &humaErr)
			assert.Equal(t, http.StatusNotFound, humaErr.GetStatus())
		})
	})
}

func TestFilterConfigsByExecutedWorkflows(t *testing.T) {
	// Note: SanitizePath transforms file names:
	// - ".crow.yml" -> "crow" (removes .yml suffix and leading .)
	// - ".crow/deploy.yml" -> "deploy" (takes base, removes .yml)

	t.Run("returns all configs when executedPipelineNames is empty", func(t *testing.T) {
		configs := []*model.Config{
			{ID: 1, Name: ".crow.yml", Data: []byte("steps:\n  - name: test")},
			{ID: 2, Name: ".crow/deploy.yml", Data: []byte("steps:\n  - name: deploy")},
		}
		executedPipelineNames := make(map[string]struct{})

		result := filterConfigsByExecutedWorkflows(configs, executedPipelineNames)

		assert.Equal(t, configs, result)
	})

	t.Run("filters configs to only executed workflows", func(t *testing.T) {
		configs := []*model.Config{
			{ID: 1, Name: ".crow.yml", Data: []byte("steps:\n  - name: test")},
			{ID: 2, Name: ".crow/deploy.yml", Data: []byte("steps:\n  - name: deploy")},
		}
		executedPipelineNames := map[string]struct{}{
			"crow": {}, // SanitizePath(".crow.yml") = "crow"
		}

		result := filterConfigsByExecutedWorkflows(configs, executedPipelineNames)

		assert.Len(t, result, 1)
		assert.Equal(t, ".crow.yml", result[0].Name)
	})

	t.Run("handles config with YAML name field", func(t *testing.T) {
		configs := []*model.Config{
			{ID: 1, Name: ".crow.yml", Data: []byte("name: build\nsteps:\n  - name: test")},
			{ID: 2, Name: ".crow/deploy.yml", Data: []byte("name: deploy\nsteps:\n  - name: deploy")},
		}
		executedPipelineNames := map[string]struct{}{
			"build": {}, // The YAML name, not the file name
		}

		result := filterConfigsByExecutedWorkflows(configs, executedPipelineNames)

		assert.Len(t, result, 1)
		assert.Equal(t, ".crow.yml", result[0].Name)
	})

	t.Run("returns empty when no configs match", func(t *testing.T) {
		configs := []*model.Config{
			{ID: 1, Name: ".crow.yml", Data: []byte("steps:\n  - name: test")},
		}
		executedPipelineNames := map[string]struct{}{
			"nonexistent": {},
		}

		result := filterConfigsByExecutedWorkflows(configs, executedPipelineNames)

		assert.Empty(t, result)
	})
}

func TestGetConfigPipelineName(t *testing.T) {
	t.Run("returns YAML name field when present", func(t *testing.T) {
		config := &model.Config{
			Name: ".crow.yml",
			Data: []byte("name: my-pipeline\nsteps:\n  - name: test"),
		}

		result := getConfigPipelineName(config)

		assert.Equal(t, "my-pipeline", result)
	})

	t.Run("returns sanitized file name when no YAML name", func(t *testing.T) {
		config := &model.Config{
			Name: ".crow/build.yml",
			Data: []byte("steps:\n  - name: test"),
		}

		result := getConfigPipelineName(config)

		// SanitizePath(".crow/build.yml") = "build" (takes base, removes .yml)
		assert.Equal(t, "build", result)
	})

	t.Run("returns sanitized file name on YAML parse error", func(t *testing.T) {
		config := &model.Config{
			Name: ".crow.yml",
			Data: []byte("invalid: yaml: content: ["),
		}

		result := getConfigPipelineName(config)

		// SanitizePath(".crow.yml") = "crow"
		assert.Equal(t, "crow", result)
	})
}

func TestParseNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "simple number", input: "42", want: 42},
		{name: "zero", input: "0", want: 0},
		{name: "large number", input: "1234567890", want: 1234567890},
		{name: "empty string", input: "", wantErr: true},
		{name: "non-digit", input: "12a", wantErr: true},
		{name: "negative sign", input: "-1", wantErr: true},
		{name: "whitespace", input: " 1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseNumber(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
