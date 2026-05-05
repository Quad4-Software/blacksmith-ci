// Copyright 2022 Woodpecker Authors
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
	"time"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v5/server/model"
)

func TestCronCreate(t *testing.T) {
	store, closer := newTestStore(t, new(model.Cron))
	defer closer()

	repo := &model.Repo{ID: 1, Name: "repo"}
	cron1 := &model.Cron{RepoID: repo.ID, CreatorID: 1, Name: "sync", NextExec: 10000, Schedule: "@every 1h"}
	assert.NoError(t, store.CronCreate(cron1))
	assert.NotEqualValues(t, 0, cron1.ID)

	// cannot insert cron job with same repoID and title
	assert.Error(t, store.CronCreate(cron1))

	oldID := cron1.ID
	assert.NoError(t, store.CronDelete(repo, oldID))
	cron1.ID = 0
	assert.NoError(t, store.CronCreate(cron1))
	assert.NotEqual(t, oldID, cron1.ID)
}

func TestCronListNextExecute(t *testing.T) {
	store, closer := newTestStore(t, new(model.Cron))
	defer closer()

	jobs, err := store.CronListNextExecute(0, 10)
	assert.NoError(t, err)
	assert.Len(t, jobs, 0)

	now := time.Now().Unix()

	assert.NoError(t, store.CronCreate(&model.Cron{Schedule: "@every 1h", Name: "some", RepoID: 1, NextExec: now}))
	assert.NoError(t, store.CronCreate(&model.Cron{Schedule: "@every 1h", Name: "aaaa", RepoID: 1, NextExec: now}))
	assert.NoError(t, store.CronCreate(&model.Cron{Schedule: "@every 1h", Name: "bbbb", RepoID: 1, NextExec: now}))
	assert.NoError(t, store.CronCreate(&model.Cron{Schedule: "@every 1h", Name: "none", RepoID: 1, NextExec: now + 1000}))
	assert.NoError(t, store.CronCreate(&model.Cron{Schedule: "@every 1h", Name: "test", RepoID: 1, NextExec: now + 2000}))

	jobs, err = store.CronListNextExecute(now, 10)
	assert.NoError(t, err)
	assert.Len(t, jobs, 3)

	jobs, err = store.CronListNextExecute(now+1500, 10)
	assert.NoError(t, err)
	assert.Len(t, jobs, 4)
}

func TestCronGetLock(t *testing.T) {
	store, closer := newTestStore(t, new(model.Cron))
	defer closer()

	nonExistingJob := &model.Cron{ID: 1000, Name: "locales", NextExec: 10000}
	gotLock, err := store.CronGetLock(nonExistingJob, time.Now().Unix()+100)
	assert.NoError(t, err)
	assert.False(t, gotLock)

	cron1 := &model.Cron{RepoID: 1, Name: "some-title", NextExec: 10000, Schedule: "@every 1h"}
	assert.NoError(t, store.CronCreate(cron1))

	oldJob := *cron1
	gotLock, err = store.CronGetLock(cron1, cron1.NextExec+1000)
	assert.NoError(t, err)
	assert.True(t, gotLock)
	assert.NotEqualValues(t, oldJob.NextExec, cron1.NextExec)

	gotLock, err = store.CronGetLock(&oldJob, oldJob.NextExec+1000)
	assert.NoError(t, err)
	assert.False(t, gotLock)
	assert.EqualValues(t, oldJob.NextExec, oldJob.NextExec)
}

func TestCronListOrphaned(t *testing.T) {
	store, closer := newTestStore(t, new(model.Cron), new(model.Repo))
	defer closer()

	// repo1 exists, repo2 will be deleted leaving its cron orphaned
	repo1 := &model.Repo{FullName: "alice/active", Owner: "alice", Name: "active", UserID: 1}
	repo2 := &model.Repo{FullName: "alice/gone", Owner: "alice", Name: "gone", UserID: 1}
	assert.NoError(t, store.CreateRepo(repo1))
	assert.NoError(t, store.CreateRepo(repo2))

	cronA := &model.Cron{RepoID: repo1.ID, Name: "cron_a", Schedule: "@every 1h"}
	cronB := &model.Cron{RepoID: repo2.ID, Name: "cron_b", Schedule: "@every 1h"}
	cronC := &model.Cron{RepoID: 9999, Name: "cron_c", Schedule: "@every 1h"} // never had a repo
	assert.NoError(t, store.CronCreate(cronA))
	assert.NoError(t, store.CronCreate(cronB))
	assert.NoError(t, store.CronCreate(cronC))

	// Delete repo2 (simulates the broken state — there is no FK cascade in production).
	// Note: store.DeleteRepo cascades to crons via the existing delete logic, so we
	// must instead bypass it for this test to leave the cron orphaned. Use raw bun.
	_, err := store.db.NewDelete().Model((*model.Repo)(nil)).Where("id = ?", repo2.ID).Exec(t.Context())
	assert.NoError(t, err)

	orphans, err := store.CronListOrphaned()
	assert.NoError(t, err)
	assert.Len(t, orphans, 2)
	names := []string{orphans[0].Name, orphans[1].Name}
	assert.Contains(t, names, "cron_b")
	assert.Contains(t, names, "cron_c")
}

func TestCronListOrphanedOwner(t *testing.T) {
	store, closer := newTestStore(t, new(model.Cron), new(model.Repo))
	defer closer()

	goodRepo := &model.Repo{FullName: "alice/good", Owner: "alice", Name: "good", UserID: 1}
	orphanRepo := &model.Repo{FullName: "alice/orphan", Owner: "alice", Name: "orphan", UserID: 0}
	assert.NoError(t, store.CreateRepo(goodRepo))
	assert.NoError(t, store.CreateRepo(orphanRepo))

	good := &model.Cron{RepoID: goodRepo.ID, Name: "good", Schedule: "@every 1h"}
	orphan := &model.Cron{RepoID: orphanRepo.ID, Name: "orphan_active", Schedule: "@every 1h"}
	disabled := &model.Cron{RepoID: orphanRepo.ID, Name: "orphan_disabled", Schedule: "@every 1h", Disabled: true}
	assert.NoError(t, store.CronCreate(good))
	assert.NoError(t, store.CronCreate(orphan))
	assert.NoError(t, store.CronCreate(disabled))

	rows, err := store.CronListOrphanedOwner()
	assert.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "orphan_active", rows[0].Name)
}

func TestCronDeleteByID(t *testing.T) {
	store, closer := newTestStore(t, new(model.Cron))
	defer closer()

	cron := &model.Cron{RepoID: 1, Name: "to-delete", Schedule: "@every 1h"}
	assert.NoError(t, store.CronCreate(cron))
	assert.NotEqualValues(t, 0, cron.ID)

	assert.NoError(t, store.CronDeleteByID(cron.ID))

	// Second delete is idempotent (no row to delete) — should return RecordNotExist via wrapDelete
	err := store.CronDeleteByID(cron.ID)
	assert.Error(t, err)
}

func TestCronDisableOrphaned(t *testing.T) {
	store, closer := newTestStore(t, new(model.Cron))
	defer closer()

	cron := &model.Cron{RepoID: 1, Name: "to-disable", Schedule: "@every 1h"}
	assert.NoError(t, store.CronCreate(cron))

	const msg = "auto-disabled by maintenance: repo owner missing"
	assert.NoError(t, store.CronDisableOrphaned(cron.ID, msg))

	got := new(model.Cron)
	assert.NoError(t, store.db.NewSelect().Model(got).Where("id = ?", cron.ID).Scan(t.Context()))
	assert.True(t, got.Disabled)
	assert.Equal(t, msg, got.FailMsg)
}
