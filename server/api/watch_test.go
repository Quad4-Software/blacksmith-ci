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
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/services/watch"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/types"
	"codefloe.com/crowci/crow/v6/shared/token"
)

// watchFakeStore is an in-memory store for the watch handlers.
type watchFakeStore struct {
	store.Store
	nextID    int64
	watches   map[int64]*model.PipelineWatch
	states    map[model.WatchKey][]*model.WatchWorkflowState
	repos     map[int64]*model.Repo
	perms     map[int64]bool // repo id -> pull permission
	pipelines map[int64]*model.Pipeline
}

func newWatchFakeStore() *watchFakeStore {
	return &watchFakeStore{
		watches:   map[int64]*model.PipelineWatch{},
		states:    map[model.WatchKey][]*model.WatchWorkflowState{},
		repos:     map[int64]*model.Repo{},
		perms:     map[int64]bool{},
		pipelines: map[int64]*model.Pipeline{},
	}
}

func (f *watchFakeStore) addRepo(id int64, name string, vis model.RepoVisibility, pull bool) *model.Repo {
	r := &model.Repo{ID: id, FullName: name, Visibility: vis}
	f.repos[id] = r
	f.perms[id] = pull
	return r
}

func (f *watchFakeStore) GetRepo(id int64) (*model.Repo, error) {
	if r, ok := f.repos[id]; ok {
		return r, nil
	}
	return nil, types.RecordNotExist
}

func (f *watchFakeStore) PermFind(_ *model.User, r *model.Repo) (*model.Perm, error) {
	return &model.Perm{RepoID: r.ID, Pull: f.perms[r.ID]}, nil
}

func (f *watchFakeStore) GetPipelineNumber(_ *model.Repo, n int64) (*model.Pipeline, error) {
	if p, ok := f.pipelines[n]; ok {
		return p, nil
	}
	return nil, types.RecordNotExist
}

func (f *watchFakeStore) GetPipelineList(_ *model.Repo, _ *model.ListOptions, flt *model.PipelineFilter) ([]*model.Pipeline, error) {
	var out []*model.Pipeline
	for n := int64(100); n > 0; n-- {
		p, ok := f.pipelines[n]
		if !ok {
			continue
		}
		if flt != nil && len(flt.Events) > 0 {
			match := false
			for _, e := range flt.Events {
				match = match || e == p.Event
			}
			if !match {
				continue
			}
		}
		out = append(out, p)
	}
	return out, nil
}

func (f *watchFakeStore) WatchCreate(w *model.PipelineWatch) error {
	f.nextID++
	w.ID = f.nextID
	cp := *w
	f.watches[w.ID] = &cp
	return nil
}

func (f *watchFakeStore) WatchFind(id int64) (*model.PipelineWatch, error) {
	if w, ok := f.watches[id]; ok {
		cp := *w
		return &cp, nil
	}
	return nil, types.RecordNotExist
}

func (f *watchFakeStore) WatchFindByKey(u, r int64, e model.WebhookEvent, t string) (*model.PipelineWatch, error) {
	for _, w := range f.watches {
		if w.UserID == u && w.RepoID == r && w.Event == e && w.Target == t {
			cp := *w
			return &cp, nil
		}
	}
	return nil, types.RecordNotExist
}

func (f *watchFakeStore) WatchUpdate(w *model.PipelineWatch) error {
	cp := *w
	f.watches[w.ID] = &cp
	return nil
}

func (f *watchFakeStore) WatchDelete(id int64) error {
	delete(f.watches, id)
	return nil
}

func (f *watchFakeStore) sorted(match func(*model.PipelineWatch) bool) []*model.PipelineWatch {
	out := []*model.PipelineWatch{}
	for id := int64(1); id <= f.nextID; id++ {
		if w, ok := f.watches[id]; ok && match(w) {
			cp := *w
			out = append(out, &cp)
		}
	}
	return out
}

func (f *watchFakeStore) WatchListByUser(u int64) ([]*model.PipelineWatch, error) {
	return f.sorted(func(w *model.PipelineWatch) bool { return w.UserID == u }), nil
}

func (f *watchFakeStore) WatchListByUserRepo(u, r int64) ([]*model.PipelineWatch, error) {
	return f.sorted(func(w *model.PipelineWatch) bool { return w.UserID == u && w.RepoID == r }), nil
}

func (f *watchFakeStore) WatchStateList(r int64, e model.WebhookEvent, t string) ([]*model.WatchWorkflowState, error) {
	return f.states[model.WatchKey{RepoID: r, Event: e, Target: t}], nil
}

func (f *watchFakeStore) WatchStateListForKeys(keys []model.WatchKey) ([]*model.WatchWorkflowState, error) {
	var out []*model.WatchWorkflowState
	for _, k := range keys {
		for _, st := range f.states[k] {
			cp := *st
			cp.RepoID, cp.Event, cp.Target = k.RepoID, k.Event, k.Target
			out = append(out, &cp)
		}
	}
	return out, nil
}

// fakeSeeder installs the states configured for a key, like Seed would.
type fakeSeeder struct {
	watch.Service
	seeded map[model.WatchKey][]*model.WatchWorkflowState
	f      *watchFakeStore
}

func (s *fakeSeeder) Seed(_ context.Context, key model.WatchKey) error {
	s.f.states[key] = s.seeded[key]
	return nil
}

func watchCtx(f *watchFakeStore, user *model.User, repo *model.Repo) context.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	c.Set("store", f)
	c.Set("user", user)
	if repo != nil {
		c.Set("repo", repo)
	}
	return testCtx(c)
}

func failingState(wf string, number int64) *model.WatchWorkflowState {
	return &model.WatchWorkflowState{WorkflowName: wf, Failing: true, PipelineNumber: number}
}

func watchStatus(t *testing.T, err error) int {
	t.Helper()
	var se huma.StatusError
	require.ErrorAs(t, err, &se)
	return se.GetStatus()
}

func createBody(event model.WebhookEvent, target string, inApp, email bool) CreateWatchBody {
	return CreateWatchBody{Event: event, Target: target, WatchSettingsBody: WatchSettingsBody{InApp: inApp, Email: email}}
}

func TestCreateRepoWatch_Defaults(t *testing.T) {
	f := newWatchFakeStore()
	repo := f.addRepo(1, "o/r", model.VisibilityPublic, true)
	user := &model.User{ID: 7}
	server.Config.Services.WatchService = &fakeSeeder{f: f}
	t.Cleanup(func() { server.Config.Services.WatchService = nil })

	out, err := CreateRepoWatch(watchCtx(f, user, repo), &CreateRepoWatchInput{RepoID: 1, Body: createBody(model.EventPush, "main", true, false)})
	require.NoError(t, err)
	assert.Equal(t, model.JSONSliceNotificationEvent{model.NotificationEventFailure}, f.watches[out.Body.ID].EmailEvents)
	assert.Equal(t, int64(7), f.watches[out.Body.ID].UserID)

	// A duplicate is a conflict.
	_, err = CreateRepoWatch(watchCtx(f, user, repo), &CreateRepoWatchInput{RepoID: 1, Body: createBody(model.EventPush, "main", true, false)})
	assert.Equal(t, http.StatusConflict, watchStatus(t, err))
}

func TestCreateRepoWatch_Validation(t *testing.T) {
	f := newWatchFakeStore()
	repo := f.addRepo(1, "o/r", model.VisibilityPublic, true)
	user := &model.User{ID: 7}
	server.Config.Services.WatchService = &fakeSeeder{f: f}
	t.Cleanup(func() { server.Config.Services.WatchService = nil })

	_, err := CreateRepoWatch(watchCtx(f, user, repo), &CreateRepoWatchInput{RepoID: 1, Body: createBody(model.EventPull, "main", true, false)})
	assert.Equal(t, http.StatusBadRequest, watchStatus(t, err), "pull request")

	_, err = CreateRepoWatch(watchCtx(f, user, repo), &CreateRepoWatchInput{RepoID: 1, Body: createBody(model.EventPush, "main", false, false)})
	assert.Equal(t, http.StatusBadRequest, watchStatus(t, err), "both channels off")

	body := createBody(model.EventPush, "main", true, true)
	body.EmailEvents = []model.NotificationEvent{"bogus"}
	_, err = CreateRepoWatch(watchCtx(f, user, repo), &CreateRepoWatchInput{RepoID: 1, Body: body})
	assert.Equal(t, http.StatusBadRequest, watchStatus(t, err), "unknown email event")

	out, err := CreateRepoWatch(watchCtx(f, user, repo), &CreateRepoWatchInput{RepoID: 1, Body: createBody(model.EventTag, "ignored", true, false)})
	require.NoError(t, err)
	assert.Equal(t, "", out.Body.Target, "tag target is forced empty")
}

func TestCreateRepoWatch_TargetTooLong(t *testing.T) {
	f := newWatchFakeStore()
	repo := f.addRepo(1, "o/r", model.VisibilityPublic, true)
	user := &model.User{ID: 7}
	server.Config.Services.WatchService = &fakeSeeder{f: f}
	t.Cleanup(func() { server.Config.Services.WatchService = nil })

	_, err := CreateRepoWatch(watchCtx(f, user, repo), &CreateRepoWatchInput{RepoID: 1, Body: createBody(model.EventPush, strings.Repeat("ä", 256), true, false)})
	assert.Equal(t, http.StatusBadRequest, watchStatus(t, err))
	assert.Empty(t, f.watches)

	_, err = CreateRepoWatch(watchCtx(f, user, repo), &CreateRepoWatchInput{RepoID: 1, Body: createBody(model.EventPush, strings.Repeat("ä", 255), true, false)})
	assert.NoError(t, err, "255 characters fit the column")
}

func TestCreateRepoWatch_TargetWithSlash(t *testing.T) {
	f := newWatchFakeStore()
	repo := f.addRepo(1, "o/r", model.VisibilityPublic, true)
	user := &model.User{ID: 7}
	server.Config.Services.WatchService = &fakeSeeder{f: f}
	t.Cleanup(func() { server.Config.Services.WatchService = nil })

	_, err := CreateRepoWatch(watchCtx(f, user, repo), &CreateRepoWatchInput{RepoID: 1, Body: createBody(model.EventPush, "release/2.x", true, false)})
	require.NoError(t, err)
	list, err := ListRepoWatches(watchCtx(f, user, repo), &ListRepoWatchesInput{RepoID: 1})
	require.NoError(t, err)
	require.Len(t, list.Body, 1)
	assert.Equal(t, "release/2.x", list.Body[0].Watch.Target)
}

func TestCreateRepoWatch_StartsRead(t *testing.T) {
	f := newWatchFakeStore()
	repo := f.addRepo(1, "o/r", model.VisibilityPublic, true)
	user := &model.User{ID: 7}
	key := model.WatchKey{RepoID: 1, Event: model.EventPush, Target: "main"}
	f.pipelines[9] = &model.Pipeline{Number: 9}
	server.Config.Services.WatchService = &fakeSeeder{f: f, seeded: map[model.WatchKey][]*model.WatchWorkflowState{
		key: {failingState("build", 7), failingState("lint", 9)},
	}}
	t.Cleanup(func() { server.Config.Services.WatchService = nil })

	out, err := CreateRepoWatch(watchCtx(f, user, repo), &CreateRepoWatchInput{RepoID: 1, Body: createBody(model.EventPush, "main", true, false)})
	require.NoError(t, err)
	assert.Equal(t, int64(9), f.watches[out.Body.ID].LastReadPipeline)

	list, err := ListRepoWatches(watchCtx(f, user, repo), &ListRepoWatchesInput{RepoID: 1})
	require.NoError(t, err)
	require.Len(t, list.Body, 1)
	item := list.Body[0]
	assert.True(t, item.Failing)
	assert.False(t, item.Unread)
	assert.Equal(t, []WatchFailingWorkflow{{Name: "build", PipelineNumber: 7}, {Name: "lint", PipelineNumber: 9}}, item.FailingWorkflows)
	require.NotNil(t, item.LatestPipeline)
	assert.Equal(t, int64(9), item.LatestPipeline.Number)
	assert.Equal(t, "o/r", item.RepoFullName)
}

func TestListUserWatches_HidesReposWithoutAccess(t *testing.T) {
	f := newWatchFakeStore()
	f.addRepo(1, "o/open", model.VisibilityPublic, true)
	f.addRepo(2, "o/secret", model.VisibilityPrivate, false)
	user := &model.User{ID: 7}
	require.NoError(t, f.WatchCreate(&model.PipelineWatch{UserID: 7, RepoID: 1, Event: model.EventPush, Target: "main", InApp: true}))
	require.NoError(t, f.WatchCreate(&model.PipelineWatch{UserID: 7, RepoID: 2, Event: model.EventPush, Target: "main", InApp: true}))
	require.NoError(t, f.WatchCreate(&model.PipelineWatch{UserID: 7, RepoID: 3, Event: model.EventPush, Target: "main", InApp: true}))

	out, err := ListUserWatches(watchCtx(f, user, nil), &struct{}{})
	require.NoError(t, err)
	require.Len(t, out.Body, 1)
	assert.Equal(t, "o/open", out.Body[0].RepoFullName)
}

func TestListUserWatches_Ordering(t *testing.T) {
	f := newWatchFakeStore()
	f.addRepo(1, "o/r", model.VisibilityPublic, true)
	user := &model.User{ID: 7}
	mk := func(target string, lastRead int64, failingAt int64) {
		require.NoError(t, f.WatchCreate(&model.PipelineWatch{UserID: 7, RepoID: 1, Event: model.EventPush, Target: target, InApp: true, LastReadPipeline: lastRead}))
		key := model.WatchKey{RepoID: 1, Event: model.EventPush, Target: target}
		if failingAt > 0 {
			f.states[key] = []*model.WatchWorkflowState{failingState("build", failingAt)}
		} else {
			f.states[key] = []*model.WatchWorkflowState{{WorkflowName: "build", PipelineNumber: 3}}
		}
	}
	mk("ok", 0, 0)
	mk("read-failing", 5, 5)
	mk("unread-failing", 0, 6)

	out, err := ListUserWatches(watchCtx(f, user, nil), &struct{}{})
	require.NoError(t, err)
	require.Len(t, out.Body, 3)
	assert.Equal(t, "unread-failing", out.Body[0].Watch.Target)
	assert.Equal(t, "read-failing", out.Body[1].Watch.Target)
	assert.Equal(t, "ok", out.Body[2].Watch.Target)
}

func TestListUserWatches_EmailOnlyNeverFailingOrUnread(t *testing.T) {
	f := newWatchFakeStore()
	f.addRepo(1, "o/r", model.VisibilityPublic, true)
	user := &model.User{ID: 7}
	require.NoError(t, f.WatchCreate(&model.PipelineWatch{UserID: 7, RepoID: 1, Event: model.EventPush, Target: "main", InApp: false, Email: true}))
	f.states[model.WatchKey{RepoID: 1, Event: model.EventPush, Target: "main"}] = []*model.WatchWorkflowState{failingState("build", 4)}

	out, err := ListUserWatches(watchCtx(f, user, nil), &struct{}{})
	require.NoError(t, err)
	require.Len(t, out.Body, 1, "email-only watches stay listed")
	item := out.Body[0]
	assert.False(t, item.Failing, "email-only watch must not count as failing in-app")
	assert.False(t, item.Unread, "email-only watch must not be unread")
	assert.Equal(t, []WatchFailingWorkflow{{Name: "build", PipelineNumber: 4}}, item.FailingWorkflows)
}

func TestUpdateUserWatch_BothChannelsOff(t *testing.T) {
	f := newWatchFakeStore()
	f.addRepo(1, "o/r", model.VisibilityPublic, true)
	require.NoError(t, f.WatchCreate(&model.PipelineWatch{UserID: 7, RepoID: 1, Event: model.EventPush, Target: "main", InApp: true}))
	_, err := UpdateUserWatch(watchCtx(f, &model.User{ID: 7}, nil), &UpdateUserWatchInput{WatchID: 1, Body: WatchSettingsBody{}})
	assert.Equal(t, http.StatusBadRequest, watchStatus(t, err))
}

func TestUpdateUserWatch_EmailDefaultsEvents(t *testing.T) {
	f := newWatchFakeStore()
	f.addRepo(1, "o/r", model.VisibilityPublic, true)
	require.NoError(t, f.WatchCreate(&model.PipelineWatch{UserID: 7, RepoID: 1, Event: model.EventPush, Target: "main", InApp: true}))
	out, err := UpdateUserWatch(watchCtx(f, &model.User{ID: 7}, nil), &UpdateUserWatchInput{WatchID: 1, Body: WatchSettingsBody{Email: true}})
	require.NoError(t, err)
	assert.True(t, out.Body.Email)
	assert.False(t, out.Body.InApp)
	assert.Equal(t, model.JSONSliceNotificationEvent{model.NotificationEventFailure}, f.watches[1].EmailEvents)
}

func TestUserWatchRoutes_OtherUsersWatch(t *testing.T) {
	f := newWatchFakeStore()
	f.addRepo(1, "o/r", model.VisibilityPublic, true)
	require.NoError(t, f.WatchCreate(&model.PipelineWatch{UserID: 99, RepoID: 1, Event: model.EventPush, Target: "main", InApp: true}))
	ctx := watchCtx(f, &model.User{ID: 7}, nil)

	_, err := MarkUserWatchRead(ctx, &WatchIDInput{WatchID: 1})
	assert.Equal(t, http.StatusNotFound, watchStatus(t, err))
	_, err = UpdateUserWatch(ctx, &UpdateUserWatchInput{WatchID: 1, Body: WatchSettingsBody{InApp: true}})
	assert.Equal(t, http.StatusNotFound, watchStatus(t, err))
	_, err = DeleteUserWatch(ctx, &WatchIDInput{WatchID: 1})
	assert.Equal(t, http.StatusNotFound, watchStatus(t, err))
	_, err = DeleteUserWatch(ctx, &WatchIDInput{WatchID: 42})
	assert.Equal(t, http.StatusNotFound, watchStatus(t, err))
	assert.Contains(t, f.watches, int64(1))
}

func TestMarkUserWatchRead(t *testing.T) {
	f := newWatchFakeStore()
	f.addRepo(1, "o/r", model.VisibilityPublic, true)
	require.NoError(t, f.WatchCreate(&model.PipelineWatch{UserID: 7, RepoID: 1, Event: model.EventPush, Target: "main", InApp: true}))
	require.NoError(t, f.WatchCreate(&model.PipelineWatch{UserID: 7, RepoID: 1, Event: model.EventPush, Target: "dev", InApp: true}))
	f.states[model.WatchKey{RepoID: 1, Event: model.EventPush, Target: "main"}] = []*model.WatchWorkflowState{failingState("build", 4)}
	f.states[model.WatchKey{RepoID: 1, Event: model.EventPush, Target: "dev"}] = []*model.WatchWorkflowState{failingState("build", 8)}
	ctx := watchCtx(f, &model.User{ID: 7}, nil)

	before, err := ListUserWatches(ctx, &struct{}{})
	require.NoError(t, err)
	assert.True(t, before.Body[0].Unread)

	_, err = MarkUserWatchRead(ctx, &WatchIDInput{WatchID: 1})
	require.NoError(t, err)
	// Marking twice must not fail.
	_, err = MarkUserWatchRead(ctx, &WatchIDInput{WatchID: 1})
	require.NoError(t, err)
	assert.Equal(t, int64(4), f.watches[1].LastReadPipeline)
	assert.Equal(t, int64(0), f.watches[2].LastReadPipeline)

	_, err = MarkUserWatchesRead(ctx, &struct{}{})
	require.NoError(t, err)
	assert.Equal(t, int64(8), f.watches[2].LastReadPipeline)
	after, err := ListUserWatches(ctx, &struct{}{})
	require.NoError(t, err)
	for _, it := range after.Body {
		assert.False(t, it.Unread)
	}
}

func TestListRepoWatchSuggestions(t *testing.T) {
	f := newWatchFakeStore()
	repo := f.addRepo(1, "o/r", model.VisibilityPublic, true)
	f.pipelines[1] = &model.Pipeline{Number: 1, Event: model.EventPush, Branch: "main"}
	f.pipelines[2] = &model.Pipeline{Number: 2, Event: model.EventPull, Branch: "feature"}
	f.pipelines[3] = &model.Pipeline{Number: 3, Event: model.EventCron, Sender: "nightly"}
	f.pipelines[4] = &model.Pipeline{Number: 4, Event: model.EventPush, Branch: "main"}
	f.pipelines[5] = &model.Pipeline{Number: 5, Event: model.EventTag, Branch: "v1"}

	out, err := ListRepoWatchSuggestions(watchCtx(f, &model.User{ID: 7}, repo), &ListRepoWatchesInput{RepoID: 1})
	require.NoError(t, err)
	assert.Equal(t, []WatchSuggestion{
		{Event: model.EventTag, Target: ""},
		{Event: model.EventPush, Target: "main"},
		{Event: model.EventCron, Target: "nightly"},
	}, out.Body)
}

func TestGetUnsubscribe_DisablesWatchEmail(t *testing.T) {
	prev := server.Config.Server.JWTSecret
	server.Config.Server.JWTSecret = "secret"
	t.Cleanup(func() { server.Config.Server.JWTSecret = prev })

	f := newWatchFakeStore()
	f.addRepo(1, "o/r", model.VisibilityPublic, true)
	require.NoError(t, f.WatchCreate(&model.PipelineWatch{UserID: 7, RepoID: 1, Event: model.EventPush, Target: "main", InApp: true, Email: true}))
	require.NoError(t, f.WatchCreate(&model.PipelineWatch{UserID: 7, RepoID: 1, Event: model.EventPush, Target: "dev", InApp: false, Email: true}))

	sign := func(watchID, userID string) string {
		tk := token.New(token.UnsubscribeToken)
		tk.Set("watch_id", watchID)
		tk.Set("user_id", userID)
		s, err := tk.Sign("secret")
		require.NoError(t, err)
		return s
	}
	call := func(tok string) {
		_, err := GetUnsubscribe(watchCtx(f, nil, nil), &GetUnsubscribeInput{Token: tok})
		require.NoError(t, err)
	}

	call(sign("1", "7"))
	assert.False(t, f.watches[1].Email)
	assert.True(t, f.watches[1].InApp)

	call(sign("2", "7"))
	assert.NotContains(t, f.watches, int64(2), "in-app off watch is deleted")

	// A token for another user's watch changes nothing.
	require.NoError(t, f.WatchCreate(&model.PipelineWatch{UserID: 8, RepoID: 1, Event: model.EventTag, InApp: true, Email: true}))
	call(sign("3", "7"))
	assert.True(t, f.watches[3].Email)

	// A vanished watch renders the friendly page instead of failing.
	call(sign("99", "7"))
}
