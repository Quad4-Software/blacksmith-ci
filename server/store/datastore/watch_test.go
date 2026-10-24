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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

func TestWatchCRUD(t *testing.T) {
	store, closer := newTestStore(t, new(model.PipelineWatch), new(model.WatchWorkflowState))
	defer closer()

	w := &model.PipelineWatch{
		UserID:      1,
		RepoID:      2,
		Event:       model.EventPush,
		Target:      "main",
		InApp:       true,
		Email:       false,
		EmailEvents: model.JSONSliceNotificationEvent{model.NotificationEventFailure},
	}
	require.NoError(t, store.WatchCreate(w))
	assert.NotZero(t, w.ID)

	got, err := store.WatchFindByKey(1, 2, model.EventPush, "main")
	require.NoError(t, err)
	assert.Equal(t, w.ID, got.ID)
	assert.True(t, got.InApp)
	assert.False(t, got.Email, "false must round-trip")
	assert.Equal(t, model.JSONSliceNotificationEvent{model.NotificationEventFailure}, got.EmailEvents)

	// A watch with in_app=false must round-trip as well.
	off := &model.PipelineWatch{UserID: 1, RepoID: 2, Event: model.EventTag, Target: "", InApp: false, Email: true}
	require.NoError(t, store.WatchCreate(off))
	gotOff, err := store.WatchFind(off.ID)
	require.NoError(t, err)
	assert.False(t, gotOff.InApp)
	assert.True(t, gotOff.Email)

	// The key is unique per user.
	assert.Error(t, store.WatchCreate(&model.PipelineWatch{UserID: 1, RepoID: 2, Event: model.EventPush, Target: "main"}))
	// Another user may watch the same key.
	assert.NoError(t, store.WatchCreate(&model.PipelineWatch{UserID: 9, RepoID: 2, Event: model.EventPush, Target: "main"}))

	got.LastReadPipeline = 7
	require.NoError(t, store.WatchUpdate(got))
	reread, err := store.WatchFind(got.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(7), reread.LastReadPipeline)

	// Saving an unchanged watch must not be reported as an error.
	require.NoError(t, store.WatchUpdate(reread))

	require.NoError(t, store.WatchDelete(got.ID))
	_, err = store.WatchFind(got.ID)
	assert.ErrorIs(t, err, types.RecordNotExist)
	_, err = store.WatchFindByKey(1, 2, model.EventPush, "main")
	assert.ErrorIs(t, err, types.RecordNotExist)
}

func TestWatchLists(t *testing.T) {
	store, closer := newTestStore(t, new(model.PipelineWatch), new(model.WatchWorkflowState))
	defer closer()

	for _, w := range []*model.PipelineWatch{
		{UserID: 1, RepoID: 1, Event: model.EventPush, Target: "main"},
		{UserID: 1, RepoID: 1, Event: model.EventManual, Target: "main"},
		{UserID: 1, RepoID: 2, Event: model.EventPush, Target: "main"},
		{UserID: 2, RepoID: 1, Event: model.EventPush, Target: "main"},
	} {
		require.NoError(t, store.WatchCreate(w))
	}

	byUser, err := store.WatchListByUser(1)
	require.NoError(t, err)
	assert.Len(t, byUser, 3)

	byUserRepo, err := store.WatchListByUserRepo(1, 1)
	require.NoError(t, err)
	assert.Len(t, byUserRepo, 2)
}

func TestWatchListByKey(t *testing.T) {
	store, closer := newTestStore(t, new(model.PipelineWatch), new(model.WatchWorkflowState))
	defer closer()

	for _, w := range []*model.PipelineWatch{
		{UserID: 1, RepoID: 1, Event: model.EventPush, Target: "main"},
		{UserID: 2, RepoID: 1, Event: model.EventPush, Target: "main"},
		{UserID: 3, RepoID: 1, Event: model.EventManual, Target: "main"},
		{UserID: 4, RepoID: 1, Event: model.EventPush, Target: "dev"},
		{UserID: 5, RepoID: 2, Event: model.EventPush, Target: "main"},
	} {
		require.NoError(t, store.WatchCreate(w))
	}

	got, err := store.WatchListByKey(1, model.EventPush, "main")
	require.NoError(t, err)
	require.Len(t, got, 2)
	ids := []int64{got[0].UserID, got[1].UserID}
	assert.ElementsMatch(t, []int64{1, 2}, ids)

	manual, err := store.WatchListByKey(1, model.EventManual, "main")
	require.NoError(t, err)
	require.Len(t, manual, 1)
	assert.Equal(t, int64(3), manual[0].UserID)
}

func TestWatchStateReplace(t *testing.T) {
	store, closer := newTestStore(t, new(model.PipelineWatch), new(model.WatchWorkflowState))
	defer closer()

	state := func(wf string, failing bool) *model.WatchWorkflowState {
		return &model.WatchWorkflowState{
			RepoID: 1, Event: model.EventPush, Target: "main", WorkflowName: wf,
			Failing: failing, PipelineID: 10, PipelineNumber: 3,
		}
	}
	other := &model.WatchWorkflowState{RepoID: 1, Event: model.EventManual, Target: "main", WorkflowName: "build", Failing: true, PipelineID: 11, PipelineNumber: 4}
	require.NoError(t, store.WatchStateReplace(1, model.EventManual, "main", []*model.WatchWorkflowState{other}))

	require.NoError(t, store.WatchStateReplace(1, model.EventPush, "main", []*model.WatchWorkflowState{state("build", true), state("lint", false)}))
	got, err := store.WatchStateList(1, model.EventPush, "main")
	require.NoError(t, err)
	assert.Len(t, got, 2)
	for _, s := range got {
		if s.WorkflowName == "lint" {
			assert.False(t, s.Failing, "false must round-trip")
		}
	}

	require.NoError(t, store.WatchStateReplace(1, model.EventPush, "main", []*model.WatchWorkflowState{state("build", false)}))
	got, err = store.WatchStateList(1, model.EventPush, "main")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "build", got[0].WorkflowName)

	// The other key is untouched.
	otherGot, err := store.WatchStateList(1, model.EventManual, "main")
	require.NoError(t, err)
	assert.Len(t, otherGot, 1)

	both, err := store.WatchStateListForKeys([]model.WatchKey{
		{RepoID: 1, Event: model.EventPush, Target: "main"},
		{RepoID: 1, Event: model.EventManual, Target: "main"},
	})
	require.NoError(t, err)
	assert.Len(t, both, 2)

	none, err := store.WatchStateListForKeys(nil)
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestWatchStateListForKeysChunked(t *testing.T) {
	store, closer := newTestStore(t, new(model.PipelineWatch), new(model.WatchWorkflowState))
	defer closer()

	const n = 450
	keys := make([]model.WatchKey, 0, n)
	for i := range n {
		target := fmt.Sprintf("branch-%d", i)
		keys = append(keys, model.WatchKey{RepoID: 1, Event: model.EventPush, Target: target})
		require.NoError(t, store.WatchStateReplace(1, model.EventPush, target, []*model.WatchWorkflowState{
			{RepoID: 1, Event: model.EventPush, Target: target, WorkflowName: "build", PipelineID: 1, PipelineNumber: 1},
		}))
	}
	got, err := store.WatchStateListForKeys(keys)
	require.NoError(t, err)
	assert.Len(t, got, n)
}

func TestWatchDeleteRemovesStateOfLastWatch(t *testing.T) {
	store, closer := newTestStore(t, new(model.PipelineWatch), new(model.WatchWorkflowState))
	defer closer()

	failing := func() []*model.WatchWorkflowState {
		return []*model.WatchWorkflowState{{WorkflowName: "build", Failing: true, PipelineID: 10, PipelineNumber: 3}}
	}
	first := &model.PipelineWatch{UserID: 1, RepoID: 1, Event: model.EventPush, Target: "main"}
	second := &model.PipelineWatch{UserID: 2, RepoID: 1, Event: model.EventPush, Target: "main"}
	tag := &model.PipelineWatch{UserID: 1, RepoID: 1, Event: model.EventTag}
	for _, w := range []*model.PipelineWatch{first, second, tag} {
		require.NoError(t, store.WatchCreate(w))
	}
	require.NoError(t, store.WatchStateReplace(1, model.EventPush, "main", failing()))
	require.NoError(t, store.WatchStateReplace(1, model.EventTag, "", failing()))

	// Another user still watches the key, so its state stays.
	require.NoError(t, store.WatchDelete(first.ID))
	got, err := store.WatchStateList(1, model.EventPush, "main")
	require.NoError(t, err)
	assert.Len(t, got, 1)

	// The last watch of the key takes the state with it, other keys keep theirs.
	require.NoError(t, store.WatchDelete(second.ID))
	got, err = store.WatchStateList(1, model.EventPush, "main")
	require.NoError(t, err)
	assert.Empty(t, got)
	got, err = store.WatchStateList(1, model.EventTag, "")
	require.NoError(t, err)
	assert.Len(t, got, 1)

	assert.ErrorIs(t, store.WatchDelete(second.ID), types.RecordNotExist)
}
