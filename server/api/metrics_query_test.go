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

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func metricsQueryCtx(store *store_mocks.MockStore, user *model.User) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	c.Set("store", store)
	c.Set("user", user)
	return c
}

func TestGetMetricsQueryScoping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	nonAdmin := &model.User{ID: 1, Login: "alice"}
	admin := &model.User{ID: 2, Login: "root", Admin: true}

	t.Run("non-admin is restricted to accessible repos", func(t *testing.T) {
		store := new(store_mocks.MockStore)
		store.On("RepoList", nonAdmin, false, true).Return([]*model.Repo{{ID: 7}, {ID: 9}}, nil)
		store.On("GetMetricSeries", mock.MatchedBy(func(o model.MetricQueryOptions) bool {
			return assert.ObjectsAreEqual([]int64{7, 9}, o.RepoIDs)
		})).Return([]model.MetricPoint{{Key: "7", Label: "a/b", Value: 3}}, nil)

		out, err := GetMetricsQuery(testCtx(metricsQueryCtx(store, nonAdmin)),
			&GetMetricsQueryInput{Dimension: model.MetricDimensionRepo, Metric: model.MetricPipelineCount})
		require.NoError(t, err)
		assert.Len(t, out.Body.Points, 1)
		store.AssertExpectations(t)
	})

	t.Run("non-admin cannot widen scope via all=true", func(t *testing.T) {
		store := new(store_mocks.MockStore)
		store.On("RepoList", nonAdmin, false, true).Return([]*model.Repo{{ID: 7}}, nil)
		store.On("GetMetricSeries", mock.MatchedBy(func(o model.MetricQueryOptions) bool {
			return assert.ObjectsAreEqual([]int64{7}, o.RepoIDs)
		})).Return([]model.MetricPoint{}, nil)

		_, err := GetMetricsQuery(testCtx(metricsQueryCtx(store, nonAdmin)),
			&GetMetricsQueryInput{Dimension: model.MetricDimensionRepo, Metric: model.MetricPipelineCount, All: true})
		require.NoError(t, err)
		store.AssertExpectations(t)
	})

	t.Run("non-admin with no accessible repos gets empty series without querying", func(t *testing.T) {
		store := new(store_mocks.MockStore)
		store.On("RepoList", nonAdmin, false, true).Return([]*model.Repo{}, nil)

		out, err := GetMetricsQuery(testCtx(metricsQueryCtx(store, nonAdmin)),
			&GetMetricsQueryInput{Dimension: model.MetricDimensionRepo, Metric: model.MetricPipelineCount})
		require.NoError(t, err)
		assert.Empty(t, out.Body.Points)
		store.AssertNotCalled(t, "GetMetricSeries", mock.Anything)
	})

	t.Run("admin all=true queries instance-wide (no repo filter)", func(t *testing.T) {
		store := new(store_mocks.MockStore)
		store.On("GetMetricSeries", mock.MatchedBy(func(o model.MetricQueryOptions) bool {
			return len(o.RepoIDs) == 0
		})).Return([]model.MetricPoint{{Key: "1", Label: "x", Value: 5}}, nil)

		out, err := GetMetricsQuery(testCtx(metricsQueryCtx(store, admin)),
			&GetMetricsQueryInput{Dimension: model.MetricDimensionOrg, Metric: model.MetricPipelineCount, All: true})
		require.NoError(t, err)
		assert.Len(t, out.Body.Points, 1)
		store.AssertExpectations(t)
	})

	t.Run("org filter narrows within the accessible set", func(t *testing.T) {
		store := new(store_mocks.MockStore)
		store.On("RepoList", nonAdmin, false, true).Return([]*model.Repo{{ID: 7}, {ID: 9}}, nil)
		store.On("RepoIDsByOrgIDs", []int64{42}).Return([]int64{9, 11}, nil)
		store.On("GetMetricSeries", mock.MatchedBy(func(o model.MetricQueryOptions) bool {
			return assert.ObjectsAreEqual([]int64{9}, o.RepoIDs) // intersection of {7,9} and {9,11}
		})).Return([]model.MetricPoint{}, nil)

		_, err := GetMetricsQuery(testCtx(metricsQueryCtx(store, nonAdmin)),
			&GetMetricsQueryInput{Dimension: model.MetricDimensionRepo, Metric: model.MetricPipelineCount, Orgs: "42"})
		require.NoError(t, err)
		store.AssertExpectations(t)
	})

	t.Run("unsupported dimension/metric combination is rejected", func(t *testing.T) {
		store := new(store_mocks.MockStore)

		_, err := GetMetricsQuery(testCtx(metricsQueryCtx(store, admin)),
			&GetMetricsQueryInput{Dimension: model.MetricDimensionAgent, Metric: model.MetricAvgSteps, All: true})
		require.Error(t, err)
		store.AssertNotCalled(t, "GetMetricSeries", mock.Anything)
	})
}
