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

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v5/server/model"
	store_mocks "codefloe.com/crowci/crow/v5/server/store/mocks"
	"codefloe.com/crowci/crow/v5/server/store/types"
)

func TestDeleteApprovalAlways(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fakeRepo := &model.Repo{ID: 1}
	fakePl := &model.Pipeline{
		ID:     10,
		Number: 5,
		Ref:    "refs/pull/42/head",
	}

	t.Run("should revoke always-approve", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("GetPipelineNumber", mock.Anything, int64(5)).Return(fakePl, nil)
		mockStore.On("ApprovedPullRequestDelete", int64(1), "refs/pull/42/head").Return(nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Set("repo", fakeRepo)

		_, err := DeleteApprovalAlways(testCtx(c), &DeleteApprovalAlwaysInput{Number: 5})

		assert.NoError(t, err)
		mockStore.AssertCalled(t, "ApprovedPullRequestDelete", int64(1), "refs/pull/42/head")
	})

	t.Run("should return 404 when pipeline not found", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("GetPipelineNumber", mock.Anything, int64(99)).Return((*model.Pipeline)(nil), types.RecordNotExist)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Set("repo", fakeRepo)

		_, err := DeleteApprovalAlways(testCtx(c), &DeleteApprovalAlwaysInput{Number: 99})

		var humaErr huma.StatusError
		assert.ErrorAs(t, err, &humaErr)
		assert.Equal(t, http.StatusNotFound, humaErr.GetStatus())
	})

	t.Run("should return 404 when approved record not found", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		mockStore.On("GetPipelineNumber", mock.Anything, int64(5)).Return(fakePl, nil)
		mockStore.On("ApprovedPullRequestDelete", int64(1), "refs/pull/42/head").Return(types.RecordNotExist)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", mockStore)
		c.Set("repo", fakeRepo)

		_, err := DeleteApprovalAlways(testCtx(c), &DeleteApprovalAlwaysInput{Number: 5})

		var humaErr huma.StatusError
		assert.ErrorAs(t, err, &humaErr)
		assert.Equal(t, http.StatusNotFound, humaErr.GetStatus())
	})
}

func TestPopulateAlwaysApproved(t *testing.T) {
	t.Run("sets flag when approved record exists", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		repo := &model.Repo{ID: 1}
		pl := &model.Pipeline{Ref: "refs/pull/42/head"}

		mockStore.On("ApprovedPullRequestFind", int64(1), "refs/pull/42/head").Return(&model.ApprovedPullRequest{
			ID:     1,
			RepoID: 1,
			Ref:    "refs/pull/42/head",
		}, nil)

		populateAlwaysApproved(mockStore, repo, pl)

		assert.True(t, pl.IsAlwaysApproved)
	})

	t.Run("does not set flag when no approved record", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		repo := &model.Repo{ID: 1}
		pl := &model.Pipeline{Ref: "refs/pull/42/head"}

		mockStore.On("ApprovedPullRequestFind", int64(1), "refs/pull/42/head").Return(nil, types.RecordNotExist)

		populateAlwaysApproved(mockStore, repo, pl)

		assert.False(t, pl.IsAlwaysApproved)
	})

	t.Run("does not set flag when ref is empty", func(t *testing.T) {
		mockStore := new(store_mocks.MockStore)
		repo := &model.Repo{ID: 1}
		pl := &model.Pipeline{Ref: ""}

		populateAlwaysApproved(mockStore, repo, pl)

		assert.False(t, pl.IsAlwaysApproved)
		mockStore.AssertNotCalled(t, "ApprovedPullRequestFind", mock.Anything, mock.Anything)
	})
}
