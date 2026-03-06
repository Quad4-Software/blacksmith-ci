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

package datastore

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store/types"
)

func TestApprovedPullRequestCreateAndFind(t *testing.T) {
	store, closer := newTestStore(t, new(model.ApprovedPullRequest))
	defer closer()

	apr := &model.ApprovedPullRequest{
		RepoID:     1,
		Ref:        "refs/pull/42/head",
		ApprovedBy: "testuser",
	}
	assert.NoError(t, store.ApprovedPullRequestCreate(apr))
	assert.NotZero(t, apr.ID)

	found, err := store.ApprovedPullRequestFind(1, "refs/pull/42/head")
	assert.NoError(t, err)
	assert.Equal(t, apr.ID, found.ID)
	assert.Equal(t, int64(1), found.RepoID)
	assert.Equal(t, "refs/pull/42/head", found.Ref)
	assert.Equal(t, "testuser", found.ApprovedBy)
}

func TestApprovedPullRequestFindNotExist(t *testing.T) {
	store, closer := newTestStore(t, new(model.ApprovedPullRequest))
	defer closer()

	_, err := store.ApprovedPullRequestFind(1, "refs/pull/99/head")
	assert.ErrorIs(t, err, types.RecordNotExist)
}

func TestApprovedPullRequestDelete(t *testing.T) {
	store, closer := newTestStore(t, new(model.ApprovedPullRequest))
	defer closer()

	apr := &model.ApprovedPullRequest{
		RepoID:     1,
		Ref:        "refs/pull/42/head",
		ApprovedBy: "testuser",
	}
	assert.NoError(t, store.ApprovedPullRequestCreate(apr))

	assert.NoError(t, store.ApprovedPullRequestDelete(1, "refs/pull/42/head"))

	_, err := store.ApprovedPullRequestFind(1, "refs/pull/42/head")
	assert.ErrorIs(t, err, types.RecordNotExist)
}

func TestApprovedPullRequestDeleteNotExist(t *testing.T) {
	store, closer := newTestStore(t, new(model.ApprovedPullRequest))
	defer closer()

	err := store.ApprovedPullRequestDelete(1, "refs/pull/99/head")
	assert.ErrorIs(t, err, types.RecordNotExist)
}

func TestApprovedPullRequestUniqueConstraint(t *testing.T) {
	store, closer := newTestStore(t, new(model.ApprovedPullRequest))
	defer closer()

	apr1 := &model.ApprovedPullRequest{
		RepoID:     1,
		Ref:        "refs/pull/42/head",
		ApprovedBy: "user1",
	}
	assert.NoError(t, store.ApprovedPullRequestCreate(apr1))

	apr2 := &model.ApprovedPullRequest{
		RepoID:     1,
		Ref:        "refs/pull/42/head",
		ApprovedBy: "user2",
	}
	err := store.ApprovedPullRequestCreate(apr2)
	assert.Error(t, err, "should fail on duplicate (repo_id, ref)")
}

func TestApprovedPullRequestDifferentRepos(t *testing.T) {
	store, closer := newTestStore(t, new(model.ApprovedPullRequest))
	defer closer()

	apr1 := &model.ApprovedPullRequest{
		RepoID:     1,
		Ref:        "refs/pull/42/head",
		ApprovedBy: "user1",
	}
	apr2 := &model.ApprovedPullRequest{
		RepoID:     2,
		Ref:        "refs/pull/42/head",
		ApprovedBy: "user2",
	}
	assert.NoError(t, store.ApprovedPullRequestCreate(apr1))
	assert.NoError(t, store.ApprovedPullRequestCreate(apr2))

	found1, err := store.ApprovedPullRequestFind(1, "refs/pull/42/head")
	assert.NoError(t, err)
	assert.Equal(t, "user1", found1.ApprovedBy)

	found2, err := store.ApprovedPullRequestFind(2, "refs/pull/42/head")
	assert.NoError(t, err)
	assert.Equal(t, "user2", found2.ApprovedBy)
}

func TestApprovedPullRequestDifferentRefs(t *testing.T) {
	store, closer := newTestStore(t, new(model.ApprovedPullRequest))
	defer closer()

	apr1 := &model.ApprovedPullRequest{
		RepoID:     1,
		Ref:        "refs/pull/42/head",
		ApprovedBy: "user1",
	}
	apr2 := &model.ApprovedPullRequest{
		RepoID:     1,
		Ref:        "refs/pull/43/head",
		ApprovedBy: "user1",
	}
	assert.NoError(t, store.ApprovedPullRequestCreate(apr1))
	assert.NoError(t, store.ApprovedPullRequestCreate(apr2))

	found1, err := store.ApprovedPullRequestFind(1, "refs/pull/42/head")
	assert.NoError(t, err)
	assert.Equal(t, "refs/pull/42/head", found1.Ref)

	found2, err := store.ApprovedPullRequestFind(1, "refs/pull/43/head")
	assert.NoError(t, err)
	assert.Equal(t, "refs/pull/43/head", found2.Ref)
}
