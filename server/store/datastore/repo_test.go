// Copyright 2022 Woodpecker Authors
// Copyright 2018 Drone.IO Inc.
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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

func TestCreateRepo(t *testing.T) {
	store, closer := newTestStore(t, new(model.Repo))
	defer closer()

	repo := model.Repo{
		UserID:   1,
		FullName: "bradrydzewski/test",
		Owner:    "bradrydzewski",
		Name:     "test",
	}
	err := store.CreateRepo(&repo)
	assert.NoError(t, err)
	assert.NotZero(t, repo.ID)

	err2 := store.UpdateRepo(&repo)
	getRepo, err3 := store.GetRepo(repo.ID)

	assert.NoError(t, err2)
	assert.NoError(t, err3)
	assert.Equal(t, repo.ID, getRepo.ID)

	// test that repo has name/owner/fullname
	assert.Error(t, store.CreateRepo(&model.Repo{
		UserID:   1,
		FullName: "bradrydzewski/",
		Owner:    "bradrydzewski",
		Name:     "",
	}))
	assert.Error(t, store.CreateRepo(&model.Repo{
		UserID:   1,
		FullName: "/test",
		Owner:    "",
		Name:     "test",
	}))
	assert.Error(t, store.CreateRepo(&model.Repo{
		UserID:   1,
		FullName: "",
		Owner:    "bradrydzewski",
		Name:     "test",
	}))

	// test unique name
	repo2 := model.Repo{
		UserID:   2,
		FullName: "bradrydzewski/test",
		Owner:    "bradrydzewski",
		Name:     "test",
	}
	err2 = store.CreateRepo(&repo2)
	assert.Error(t, err2)
}

// repoDeleteTestStore builds a store with all tables involved in repo deletion.
func repoDeleteTestStore(t *testing.T) (*storage, func()) {
	return newTestStore(t,
		new(model.Repo), new(model.Org), new(model.Pipeline), new(model.Workflow),
		new(model.Step), new(model.Task), new(model.Perm), new(model.Registry),
		new(model.Secret), new(model.Redirection), new(model.Config), new(model.PipelineConfig))
}

// seedRepoWithHistory creates an org, a repo, and a finished pipeline (with a
// workflow, a queued task and a perm) so deletion behavior can be asserted.
func seedRepoWithHistory(t *testing.T, store *storage) (*model.Org, *model.Repo, *model.Pipeline) {
	org := &model.Org{Name: "acme", ForgeID: 1}
	assert.NoError(t, store.OrgCreate(org))

	repo := &model.Repo{UserID: 1, OrgID: org.ID, FullName: "acme/test", Owner: "acme", Name: "test"}
	assert.NoError(t, store.CreateRepo(repo))

	pipeline := &model.Pipeline{RepoID: repo.ID, Status: model.StatusSuccess, Started: 1_000, Finished: 1_060}
	assert.NoError(t, store.CreatePipeline(pipeline))
	assert.NoError(t, store.WorkflowsCreate([]*model.Workflow{{PipelineID: pipeline.ID, PID: 1, Name: "wf"}}))
	assert.NoError(t, store.TaskInsert(&model.Task{ID: "task-1", PipelineID: pipeline.ID, RepoID: repo.ID}))
	assert.NoError(t, store.PermUpsert(&model.Perm{UserID: 1, RepoID: repo.ID}))

	return org, repo, pipeline
}

func TestDeleteRepoPreservesMetricsHistory(t *testing.T) {
	store, closer := repoDeleteTestStore(t)
	defer closer()
	store.metricsPreserveDeleted = true

	org, repo, pipeline := seedRepoWithHistory(t, store)

	assert.NoError(t, store.DeleteRepo(repo))

	// Repo, perms and live queue tasks are gone.
	_, err := store.GetRepo(repo.ID)
	assert.Error(t, err)
	taskCount, err := store.db.NewSelect().Model((*model.Task)(nil)).Where("repo_id = ?", repo.ID).Count(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), taskCount)

	// Pipelines and workflows survive so historic metrics still resolve.
	pipeCount, err := store.db.NewSelect().Model((*model.Pipeline)(nil)).Where("repo_id = ?", repo.ID).Count(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(1), pipeCount)
	wfCount, err := store.db.NewSelect().Model((*model.Workflow)(nil)).Where("pipeline_id = ?", pipeline.ID).Count(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(1), wfCount)

	// The org bar keeps its denormalized name even though the repo row is gone.
	orgStats, err := store.GetPipelineCountsByOrg(0, 0, nil)
	assert.NoError(t, err)
	assert.Len(t, orgStats, 1)
	assert.Equal(t, org.ID, orgStats[0].OrgID)
	assert.Equal(t, "acme", orgStats[0].OrgName)
	assert.Equal(t, int64(1), orgStats[0].PipelineCount)
}

func TestDeleteRepoPurgesWhenPreserveDisabled(t *testing.T) {
	store, closer := repoDeleteTestStore(t)
	defer closer()
	store.metricsPreserveDeleted = false

	_, repo, pipeline := seedRepoWithHistory(t, store)

	assert.NoError(t, store.DeleteRepo(repo))

	pipeCount, err := store.db.NewSelect().Model((*model.Pipeline)(nil)).Where("repo_id = ?", repo.ID).Count(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), pipeCount)
	wfCount, err := store.db.NewSelect().Model((*model.Workflow)(nil)).Where("pipeline_id = ?", pipeline.ID).Count(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), wfCount)
}

func TestGetRepo(t *testing.T) {
	store, closer := newTestStore(t, new(model.Repo))
	defer closer()
	repo := model.Repo{
		UserID:   1,
		FullName: "bradrydzewski/test",
		Owner:    "bradrydzewski",
		Name:     "test",
	}
	assert.NoError(t, store.CreateRepo(&repo))
	getrepo, err := store.GetRepo(repo.ID)
	assert.NoError(t, err)
	assert.Equal(t, repo.ID, getrepo.ID)
	assert.Equal(t, repo.UserID, getrepo.UserID)
	assert.Equal(t, repo.Owner, getrepo.Owner)
	assert.Equal(t, repo.Name, getrepo.Name)
}

func TestGetRepoName(t *testing.T) {
	store, closer := newTestStore(t, new(model.Repo))
	defer closer()
	repo := model.Repo{
		UserID:   1,
		FullName: "bradrydzewski/TEST",
		Owner:    "bradrydzewski",
		Name:     "TEST",
	}

	assert.NoError(t, store.CreateRepo(&repo))
	getrepo, err := store.GetRepoName(repo.FullName)
	assert.NoError(t, err)
	assert.Equal(t, repo.ID, getrepo.ID)
	assert.Equal(t, repo.UserID, getrepo.UserID)
	assert.Equal(t, repo.Owner, getrepo.Owner)
	assert.Equal(t, repo.Name, getrepo.Name)
}

func TestRepoList(t *testing.T) {
	store, closer := newTestStore(t, new(model.Repo), new(model.User), new(model.Perm), new(model.Org))
	defer closer()

	user := &model.User{
		Login:       "joe",
		Email:       "foo@bar.com",
		AccessToken: "e42080dddf012c718e476da161d21ad5",
	}
	assert.NoError(t, store.CreateUser(user))

	repo1 := &model.Repo{
		Owner:         "bradrydzewski",
		Name:          "test",
		FullName:      "bradrydzewski/test",
		ForgeRemoteID: "1",
	}
	repo2 := &model.Repo{
		Owner:         "test",
		Name:          "test",
		FullName:      "test/test",
		ForgeRemoteID: "2",
	}
	repo3 := &model.Repo{
		Owner:         "octocat",
		Name:          "hello-world",
		FullName:      "octocat/hello-world",
		ForgeRemoteID: "3",
	}
	assert.NoError(t, store.CreateRepo(repo1))
	assert.NoError(t, store.CreateRepo(repo2))
	assert.NoError(t, store.CreateRepo(repo3))

	for _, perm := range []*model.Perm{
		{UserID: user.ID, Repo: repo1},
		{UserID: user.ID, Repo: repo2},
	} {
		assert.NoError(t, store.PermUpsert(perm))
	}

	repos, err := store.RepoList(user, false, false)
	assert.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, repo1.ID, repos[0].ID)
	assert.Equal(t, repo2.ID, repos[1].ID)
}

func TestOwnedRepoList(t *testing.T) {
	store, closer := newTestStore(t, new(model.Repo), new(model.User), new(model.Perm), new(model.Org))
	defer closer()

	user := &model.User{
		Login:       "joe",
		Email:       "foo@bar.com",
		AccessToken: "e42080dddf012c718e476da161d21ad5",
	}
	assert.NoError(t, store.CreateUser(user))

	repo1 := &model.Repo{
		Owner:         "bradrydzewski",
		Name:          "test",
		FullName:      "bradrydzewski/test",
		ForgeRemoteID: "1",
	}
	repo2 := &model.Repo{
		Owner:         "test",
		Name:          "test",
		FullName:      "test/test",
		ForgeRemoteID: "2",
	}
	repo3 := &model.Repo{
		Owner:         "octocat",
		Name:          "hello-world",
		FullName:      "octocat/hello-world",
		ForgeRemoteID: "3",
	}
	repo4 := &model.Repo{
		Owner:         "demo",
		Name:          "demo",
		FullName:      "demo/demo",
		ForgeRemoteID: "4",
	}
	assert.NoError(t, store.CreateRepo(repo1))
	assert.NoError(t, store.CreateRepo(repo2))
	assert.NoError(t, store.CreateRepo(repo3))
	assert.NoError(t, store.CreateRepo(repo4))

	for _, perm := range []*model.Perm{
		{UserID: user.ID, Repo: repo1, Push: true, Admin: false},
		{UserID: user.ID, Repo: repo2, Push: false, Admin: true},
		{UserID: user.ID, Repo: repo3},
		{UserID: user.ID, Repo: repo4},
	} {
		assert.NoError(t, store.PermUpsert(perm))
	}

	repos, err := store.RepoList(user, true, false)
	assert.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, repo1.ID, repos[0].ID)
	assert.Equal(t, repo2.ID, repos[1].ID)
}

func TestRepoCount(t *testing.T) {
	store, closer := newTestStore(t, new(model.Repo))
	defer closer()

	repo1 := &model.Repo{
		Owner:    "bradrydzewski",
		Name:     "test",
		FullName: "bradrydzewski/test",
		IsActive: true,
	}
	repo2 := &model.Repo{
		Owner:    "test",
		Name:     "test",
		FullName: "test/test",
		IsActive: true,
	}
	repo3 := &model.Repo{
		Owner:    "test",
		Name:     "test-ui",
		FullName: "test/test-ui",
		IsActive: false,
	}
	assert.NoError(t, store.CreateRepo(repo1))
	assert.NoError(t, store.CreateRepo(repo2))
	assert.NoError(t, store.CreateRepo(repo3))

	count, err := store.GetRepoCount()
	assert.NoError(t, err)
	assert.EqualValues(t, 2, count)
}

func TestRepoCrud(t *testing.T) {
	store, closer := newTestStore(t,
		new(model.Repo),
		new(model.User),
		new(model.Perm),
		new(model.Pipeline),
		new(model.PipelineConfig),
		new(model.LogEntry),
		new(model.Step),
		new(model.Secret),
		new(model.Registry),
		new(model.Config),
		new(model.Redirection),
		new(model.Workflow),
		new(model.Task))
	defer closer()

	repo := model.Repo{
		UserID:   1,
		FullName: "bradrydzewski/test",
		Owner:    "bradrydzewski",
		Name:     "test",
	}
	assert.NoError(t, store.CreateRepo(&repo))
	pipeline := model.Pipeline{
		RepoID: repo.ID,
	}
	step := model.Step{
		Name: "a step",
	}
	assert.NoError(t, store.CreatePipeline(&pipeline, &step))

	// create unrelated
	repoUnrelated := model.Repo{
		UserID:   2,
		FullName: "x/x",
		Owner:    "x",
		Name:     "x",
	}
	assert.NoError(t, store.CreateRepo(&repoUnrelated))
	pipelineUnrelated := model.Pipeline{
		RepoID: repoUnrelated.ID,
	}
	stepUnrelated := model.Step{
		UUID: "44c0de71-a6be-41c9-b860-e3716d1dfcef",
		Name: "a unrelated step",
	}
	assert.NoError(t, store.CreatePipeline(&pipelineUnrelated, &stepUnrelated))

	_, err := store.GetRepo(repo.ID)
	assert.NoError(t, err)
	assert.NoError(t, store.DeleteRepo(&repo))
	_, err = store.GetRepo(repo.ID)
	assert.Error(t, err)

	stepCount, err := store.db.NewSelect().Model((*model.Step)(nil)).Count(context.Background())
	assert.NoError(t, err)
	assert.EqualValues(t, 1, stepCount)
	pipelineCount, err := store.db.NewSelect().Model((*model.Pipeline)(nil)).Count(context.Background())
	assert.NoError(t, err)
	assert.EqualValues(t, 1, pipelineCount)
}

func TestGetRepoByForgeAndRemoteID(t *testing.T) {
	store, closer := newTestStore(t, new(model.Repo))
	defer closer()

	forgeAID := int64(1)
	forgeBID := int64(2)

	// Regression for issue #1019: two repos on different forges share the same
	// forge_remote_id. The lookup must scope by forge_id so each returns its own row.
	repoA := &model.Repo{
		UserID:        1,
		ForgeID:       forgeAID,
		ForgeRemoteID: "1",
		FullName:      "octocat/hello-world",
		Owner:         "octocat",
		Name:          "hello-world",
		IsActive:      true,
	}
	repoB := &model.Repo{
		UserID:        1,
		ForgeID:       forgeBID,
		ForgeRemoteID: "1",
		FullName:      "other/repo",
		Owner:         "other",
		Name:          "repo",
	}
	assert.NoError(t, store.CreateRepo(repoA))
	assert.NoError(t, store.CreateRepo(repoB))

	got, err := store.GetRepoByForgeAndRemoteID(forgeAID, "1")
	assert.NoError(t, err)
	assert.Equal(t, repoA.ID, got.ID)
	assert.Equal(t, forgeAID, got.ForgeID)

	got, err = store.GetRepoByForgeAndRemoteID(forgeBID, "1")
	assert.NoError(t, err)
	assert.Equal(t, repoB.ID, got.ID)
	assert.Equal(t, forgeBID, got.ForgeID)

	_, err = store.GetRepoByForgeAndRemoteID(forgeAID, "999")
	assert.ErrorIs(t, err, types.RecordNotExist)
}

func TestGetRepoByForgeOwnerName(t *testing.T) {
	store, closer := newTestStore(t, new(model.Repo))
	defer closer()

	forgeAID := int64(1)
	forgeBID := int64(2)

	repoA := &model.Repo{
		UserID:        1,
		ForgeID:       forgeAID,
		ForgeRemoteID: "100",
		FullName:      "octocat/hello-world",
		Owner:         "octocat",
		Name:          "hello-world",
	}
	// Same (owner, name) on a different forge — must not be returned for forgeAID.
	repoB := &model.Repo{
		UserID:        1,
		ForgeID:       forgeBID,
		ForgeRemoteID: "200",
		FullName:      "gitlab-octocat/hello-world",
		Owner:         "octocat",
		Name:          "hello-world",
	}
	assert.NoError(t, store.CreateRepo(repoA))
	assert.NoError(t, store.CreateRepo(repoB))

	got, err := store.GetRepoByForgeOwnerName(forgeAID, "octocat", "hello-world")
	assert.NoError(t, err)
	assert.Equal(t, repoA.ID, got.ID)
	assert.Equal(t, forgeAID, got.ForgeID)

	got, err = store.GetRepoByForgeOwnerName(forgeBID, "octocat", "hello-world")
	assert.NoError(t, err)
	assert.Equal(t, repoB.ID, got.ID)
	assert.Equal(t, forgeBID, got.ForgeID)

	_, err = store.GetRepoByForgeOwnerName(forgeAID, "octocat", "does-not-exist")
	assert.ErrorIs(t, err, types.RecordNotExist)

	// Regression for issue #1009: a repo deleted+recreated on the forge gets a
	// new forge_remote_id, but (forge_id, owner, name) still resolves the stale
	// row so PostRepo can update it instead of inserting a duplicate.
	stale := &model.Repo{
		UserID:        1,
		ForgeID:       forgeAID,
		ForgeRemoteID: "300",
		FullName:      "octocat/recreated",
		Owner:         "octocat",
		Name:          "recreated",
		IsActive:      false,
	}
	assert.NoError(t, store.CreateRepo(stale))

	got, err = store.GetRepoByForgeOwnerName(forgeAID, "octocat", "recreated")
	assert.NoError(t, err)
	assert.Equal(t, stale.ID, got.ID)
	assert.Equal(t, model.ForgeRemoteID("300"), got.ForgeRemoteID)
}

func TestRepoListOrphanedOwner(t *testing.T) {
	store, closer := newTestStore(t, new(model.Repo))
	defer closer()

	good := &model.Repo{FullName: "alice/active", Owner: "alice", Name: "active", UserID: 1}
	orphan1 := &model.Repo{FullName: "alice/orphan1", Owner: "alice", Name: "orphan1", UserID: 0}
	orphan2 := &model.Repo{FullName: "bob/orphan2", Owner: "bob", Name: "orphan2", UserID: 0}
	assert.NoError(t, store.CreateRepo(good))
	assert.NoError(t, store.CreateRepo(orphan1))
	assert.NoError(t, store.CreateRepo(orphan2))

	rows, err := store.RepoListOrphanedOwner()
	assert.NoError(t, err)
	assert.Len(t, rows, 2)
	names := []string{rows[0].FullName, rows[1].FullName}
	assert.Contains(t, names, "alice/orphan1")
	assert.Contains(t, names, "bob/orphan2")
}

func TestRepoRedirection(t *testing.T) {
	store, closer := newTestStore(t,
		new(model.Repo),
		new(model.Redirection))
	defer closer()

	repo := model.Repo{
		UserID:        1,
		ForgeRemoteID: "1",
		FullName:      "bradrydzewski/test",
		Owner:         "bradrydzewski",
		Name:          "test",
	}
	assert.NoError(t, store.CreateRepo(&repo))

	repoUpdated := model.Repo{
		ID:            repo.ID,
		ForgeRemoteID: "1",
		FullName:      "bradrydzewski/test-renamed",
		Owner:         "bradrydzewski",
		Name:          "test-renamed",
	}

	assert.NoError(t, store.UpdateRepo(&repoUpdated))
	assert.NoError(t, store.CreateRedirection(&model.Redirection{
		RepoID:   repo.ID,
		FullName: repo.FullName,
	}))

	// test redirection from old repo name
	repoFromStore, err := store.GetRepoNameFallback(0, "1", "bradrydzewski/test")
	assert.NoError(t, err)
	assert.Equal(t, repoFromStore.FullName, repoUpdated.FullName)

	// test getting repo without forge ID (use name fallback)
	repo = model.Repo{
		UserID:   1,
		FullName: "bradrydzewski/test-no-forge-id",
		Owner:    "bradrydzewski",
		Name:     "test-no-forge-id",
	}
	assert.NoError(t, store.CreateRepo(&repo))

	repoFromStore, err = store.GetRepoNameFallback(0, "", "bradrydzewski/test-no-forge-id")
	assert.NoError(t, err)
	assert.Equal(t, repoFromStore.FullName, repo.FullName)
}

// TestGetRepoNameFallbackForgeScoped verifies the lookup is forge-scoped on
// both the forge_remote_id step and the legacy full_name step, so a repo on a
// different forge with the same remote ID or full name is never returned.
func TestGetRepoNameFallbackForgeScoped(t *testing.T) {
	store, closer := newTestStore(t, new(model.Repo))
	defer closer()

	forgeAID := int64(1)
	forgeBID := int64(2)

	// (forgeA, "10") — primary lookup target.
	repoA := &model.Repo{
		UserID: 1, ForgeID: forgeAID, ForgeRemoteID: "10",
		FullName: "octocat/repo", Owner: "octocat", Name: "repo",
	}
	// (forgeB, "10") — same remote ID on another forge, must not be returned for forgeA.
	repoB := &model.Repo{
		UserID: 1, ForgeID: forgeBID, ForgeRemoteID: "10",
		FullName: "other/repo", Owner: "other", Name: "repo",
	}
	// (forgeB, "") — same full_name on another forge, must not be returned for forgeA's fallback.
	repoBLegacy := &model.Repo{
		UserID: 1, ForgeID: forgeBID, ForgeRemoteID: "",
		FullName: "octocat/legacy", Owner: "octocat", Name: "legacy",
	}
	// (forgeA, "") — legacy row on forgeA that the full_name fallback should resolve.
	repoALegacy := &model.Repo{
		UserID: 1, ForgeID: forgeAID, ForgeRemoteID: "",
		FullName: "octocat/legacy", Owner: "octocat", Name: "legacy",
	}
	assert.NoError(t, store.CreateRepo(repoA))
	assert.NoError(t, store.CreateRepo(repoB))
	assert.NoError(t, store.CreateRepo(repoBLegacy))
	assert.NoError(t, store.CreateRepo(repoALegacy))

	got, err := store.GetRepoNameFallback(forgeAID, "10", "octocat/repo")
	assert.NoError(t, err)
	assert.Equal(t, repoA.ID, got.ID)

	got, err = store.GetRepoNameFallback(forgeBID, "10", "other/repo")
	assert.NoError(t, err)
	assert.Equal(t, repoB.ID, got.ID)

	// Legacy fallback on forgeA must return forgeA's legacy row, not forgeB's.
	got, err = store.GetRepoNameFallback(forgeAID, "missing", "octocat/legacy")
	assert.NoError(t, err)
	assert.Equal(t, repoALegacy.ID, got.ID)
	assert.Equal(t, forgeAID, got.ForgeID)
}

func TestDeleteRepoPurgesMorePipelinesThanOneBatch(t *testing.T) {
	store, closer := repoDeleteTestStore(t)
	defer closer()
	store.metricsPreserveDeleted = false

	repo := &model.Repo{UserID: 1, FullName: "acme/many", Owner: "acme", Name: "many"}
	assert.NoError(t, store.CreateRepo(repo))

	const total = 2*perPage + 7
	pipelines := make([]*model.Pipeline, 0, total)
	for i := range total {
		pipelines = append(pipelines, &model.Pipeline{RepoID: repo.ID, Number: int64(i + 1)})
	}
	_, err := store.db.NewInsert().Model(&pipelines).Exec(context.Background())
	assert.NoError(t, err)

	assert.NoError(t, store.DeleteRepo(repo))

	pipeCount, err := store.db.NewSelect().Model((*model.Pipeline)(nil)).Where("repo_id = ?", repo.ID).Count(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), pipeCount)
}
