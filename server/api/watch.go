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
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pipeline"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/services/watch"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

// suggestionWindow is the number of recent pipelines scanned for watch suggestions.
const suggestionWindow = 500

// maxWatchTargetLength is the size of the watch_target column.
const maxWatchTargetLength = 255

// WatchSettingsBody holds the notification channels of a watch.
type WatchSettingsBody struct {
	InApp       bool                      `json:"in_app" doc:"show failures in the web UI"`
	Email       bool                      `json:"email" doc:"send emails for the selected events"`
	EmailEvents []model.NotificationEvent `json:"email_events,omitempty" doc:"events to email about: success, failure, status_change; defaults to failure"`
}

// CreateWatchBody creates a watch on a (event, target) key of a repository.
type CreateWatchBody struct {
	Event             model.WebhookEvent `json:"event" doc:"the watched event: push, manual, cron, deployment, tag or release"`
	Target            string             `json:"target" maxLength:"255" doc:"branch for push and manual, cron name for cron, environment for deployment; empty for tag and release"`
	WatchSettingsBody `json:",inline"`
}

// WatchFailingWorkflow is a workflow that currently fails for a watch.
type WatchFailingWorkflow struct {
	Name           string `json:"name" doc:"the workflow name, empty for a pipeline-level (configuration) failure"`
	PipelineNumber int64  `json:"pipeline_number"`
}

// WatchOverviewItem is a watch together with its current state.
type WatchOverviewItem struct {
	Watch            *model.PipelineWatch   `json:"watch"`
	RepoFullName     string                 `json:"repo_full_name"`
	Failing          bool                   `json:"failing" doc:"a workflow fails and the watch has the in-app channel enabled"`
	Unread           bool                   `json:"unread" doc:"the watch is failing in-app with a failure the user has not read"`
	FailingWorkflows []WatchFailingWorkflow `json:"failing_workflows"`
	LatestPipeline   *model.Pipeline        `json:"latest_pipeline,omitempty" doc:"the most recent failing run if failing, else the latest run of the key"`
}

// WatchSuggestion is a key that has recent runs and can be watched.
type WatchSuggestion struct {
	Event  model.WebhookEvent `json:"event"`
	Target string             `json:"target"`
}

// normalizeWatchSettings validates the channels and email events and returns
// the events to store.
func normalizeWatchSettings(b WatchSettingsBody) ([]model.NotificationEvent, error) {
	if !b.InApp && !b.Email {
		return nil, huma.Error400BadRequest("at least one of in_app and email must be enabled")
	}
	events := make([]model.NotificationEvent, 0, len(b.EmailEvents))
	for _, e := range b.EmailEvents {
		switch e {
		case model.NotificationEventSuccess, model.NotificationEventFailure, model.NotificationEventStatusChange:
		default:
			return nil, huma.Error400BadRequest("unknown email event: " + string(e))
		}
		if !slices.Contains(events, e) {
			events = append(events, e)
		}
	}
	if len(events) == 0 {
		events = append(events, model.NotificationEventFailure)
	}
	return events, nil
}

// buildWatchOverview computes the current state of the watches, dropping the
// ones whose repository is gone or no longer readable by the user.
func buildWatchOverview(_store store.Store, user *model.User, watches []*model.PipelineWatch) []WatchOverviewItem {
	repos := map[int64]*model.Repo{}
	repoFor := func(id int64) *model.Repo {
		if r, ok := repos[id]; ok {
			return r
		}
		r, err := _store.GetRepo(id)
		if err != nil {
			if !errors.Is(err, types.RecordNotExist) {
				log.Error().Err(err).Int64("repo_id", id).Msg("watch overview: cannot load repo")
			}
			r = nil
		} else if !watch.CanPull(_store, user, r) {
			r = nil
		}
		repos[id] = r
		return r
	}

	visible := make([]*model.PipelineWatch, 0, len(watches))
	keys := make([]model.WatchKey, 0, len(watches))
	seen := map[model.WatchKey]bool{}
	for _, w := range watches {
		if repoFor(w.RepoID) == nil {
			continue
		}
		visible = append(visible, w)
		k := model.WatchKey{RepoID: w.RepoID, Event: w.Event, Target: w.Target}
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}

	byKey := map[model.WatchKey][]*model.WatchWorkflowState{}
	if len(keys) > 0 {
		states, err := _store.WatchStateListForKeys(keys)
		if err != nil {
			log.Error().Err(err).Msg("watch overview: cannot load watch states")
		}
		for _, st := range states {
			k := model.WatchKey{RepoID: st.RepoID, Event: st.Event, Target: st.Target}
			byKey[k] = append(byKey[k], st)
		}
	}

	items := make([]WatchOverviewItem, 0, len(visible))
	for _, w := range visible {
		repo := repoFor(w.RepoID)
		states := byKey[model.WatchKey{RepoID: w.RepoID, Event: w.Event, Target: w.Target}]
		// Failing and Unread drive the in-app bell and Failing tab, so a watch
		// without the in-app channel never reports them.
		failing := watch.IsFailing(states)
		item := WatchOverviewItem{
			Watch:            w,
			RepoFullName:     repo.FullName,
			Failing:          w.InApp && failing,
			Unread:           w.InApp && watch.IsUnread(w, states),
			FailingWorkflows: []WatchFailingWorkflow{},
		}

		var latest int64
		for _, st := range states {
			latest = max(latest, st.PipelineNumber)
			if st.Failing {
				item.FailingWorkflows = append(item.FailingWorkflows, WatchFailingWorkflow{Name: st.WorkflowName, PipelineNumber: st.PipelineNumber})
			}
		}
		sort.SliceStable(item.FailingWorkflows, func(i, j int) bool {
			return item.FailingWorkflows[i].Name < item.FailingWorkflows[j].Name
		})
		if failing {
			latest = watch.NewestFailingNumber(states)
		}
		if latest > 0 {
			p, err := _store.GetPipelineNumber(repo, latest)
			if err != nil {
				log.Warn().Err(err).Int64("repo_id", repo.ID).Int64("number", latest).Msg("watch overview: cannot load latest pipeline")
			} else {
				item.LatestPipeline = p
			}
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Failing != b.Failing {
			return a.Failing
		}
		if a.Failing && a.Unread != b.Unread {
			return a.Unread
		}
		return latestCreated(a) > latestCreated(b)
	})
	return items
}

func latestCreated(i WatchOverviewItem) int64 {
	if i.LatestPipeline == nil {
		return 0
	}
	return i.LatestPipeline.Created
}

// ownedWatch loads a watch of the user, answering 404 for unknown watches and
// for the ones of other users.
func ownedWatch(_store store.Store, user *model.User, id int64) (*model.PipelineWatch, error) {
	w, err := _store.WatchFind(id)
	if err != nil {
		if errors.Is(err, types.RecordNotExist) {
			return nil, huma.Error404NotFound("watch not found")
		}
		return nil, huma.Error500InternalServerError("failed to load watch", err)
	}
	if w.UserID != user.ID {
		return nil, huma.Error404NotFound("watch not found")
	}
	return w, nil
}

// markRead sets the read marker of the watch to the newest failing run.
func markRead(_store store.Store, w *model.PipelineWatch) error {
	states, err := _store.WatchStateList(w.RepoID, w.Event, w.Target)
	if err != nil {
		return err
	}
	n := watch.NewestFailingNumber(states)
	if n <= w.LastReadPipeline {
		return nil
	}
	w.LastReadPipeline = n
	return _store.WatchUpdate(w)
}

type WatchListOutput struct {
	Body []WatchOverviewItem
}

type WatchOutput struct {
	Body *model.PipelineWatch
}

type WatchIDInput struct {
	WatchID int64 `path:"watch_id" doc:"the watch id"`
}

func ListUserWatches(ctx context.Context, _ *struct{}) (*WatchListOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)

	watches, err := _store.WatchListByUser(user.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list watches", err)
	}
	return &WatchListOutput{Body: buildWatchOverview(_store, user, watches)}, nil
}

type StuckPipelineListOutput struct {
	Body []model.StuckPipeline
}

// ListUserStuckPipelines lists the pipelines the current user triggered that
// have a workflow stuck in the queue, for the header bell.
func ListUserStuckPipelines(ctx context.Context, _ *struct{}) (*StuckPipelineListOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)

	stuck := pipeline.FindStuckPipelines(c, _store, server.Config.Services.Queue, server.Config.Pipeline.QueueStuckThreshold, time.Now())
	result := make([]model.StuckPipeline, 0)
	repos := make(map[int64]*model.Repo)
	for _, entry := range stuck {
		p := entry.Pipeline
		if !pipeline.TriggeredBy(p, user) {
			continue
		}
		repo, ok := repos[p.RepoID]
		if !ok {
			var err error
			if repo, err = _store.GetRepo(p.RepoID); err != nil {
				repo = nil
			}
			repos[p.RepoID] = repo
		}
		// a matching login on another forge must not reveal the repository
		if !pipeline.MayReceiveStuckNotice(_store, user, repo) {
			continue
		}
		result = append(result, model.StuckPipeline{
			RepoID:         repo.ID,
			RepoFullName:   repo.FullName,
			PipelineNumber: p.Number,
			Created:        p.Created,
			Workflows:      entry.Workflows,
		})
	}
	return &StuckPipelineListOutput{Body: result}, nil
}

func MarkUserWatchRead(ctx context.Context, input *WatchIDInput) (*WatchOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	w, err := ownedWatch(_store, session.User(c), input.WatchID)
	if err != nil {
		return nil, err
	}
	if err := markRead(_store, w); err != nil {
		return nil, huma.Error500InternalServerError("failed to mark watch read", err)
	}
	return &WatchOutput{Body: w}, nil
}

func MarkUserWatchesRead(ctx context.Context, _ *struct{}) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	watches, err := _store.WatchListByUser(session.User(c).ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list watches", err)
	}
	for _, w := range watches {
		if err := markRead(_store, w); err != nil {
			return nil, huma.Error500InternalServerError("failed to mark watches read", err)
		}
	}
	return nil, nil
}

type UpdateUserWatchInput struct {
	WatchID int64 `path:"watch_id" doc:"the watch id"`
	Body    WatchSettingsBody
}

func UpdateUserWatch(ctx context.Context, input *UpdateUserWatchInput) (*WatchOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	w, err := ownedWatch(_store, session.User(c), input.WatchID)
	if err != nil {
		return nil, err
	}
	events, err := normalizeWatchSettings(input.Body)
	if err != nil {
		return nil, err
	}
	w.InApp = input.Body.InApp
	w.Email = input.Body.Email
	w.EmailEvents = events
	if err := _store.WatchUpdate(w); err != nil {
		return nil, huma.Error500InternalServerError("failed to update watch", err)
	}
	return &WatchOutput{Body: w}, nil
}

func DeleteUserWatch(ctx context.Context, input *WatchIDInput) (*struct{}, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	w, err := ownedWatch(_store, session.User(c), input.WatchID)
	if err != nil {
		return nil, err
	}
	if err := _store.WatchDelete(w.ID); err != nil && !errors.Is(err, types.RecordNotExist) {
		return nil, huma.Error500InternalServerError("failed to delete watch", err)
	}
	return nil, nil
}

type ListRepoWatchesInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
}

func ListRepoWatches(ctx context.Context, _ *ListRepoWatchesInput) (*WatchListOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)
	repo := session.Repo(c)

	watches, err := _store.WatchListByUserRepo(user.ID, repo.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list watches", err)
	}
	return &WatchListOutput{Body: buildWatchOverview(_store, user, watches)}, nil
}

type CreateRepoWatchInput struct {
	RepoID int64 `path:"repo_id" doc:"the repository id"`
	Body   CreateWatchBody
}

func CreateRepoWatch(ctx context.Context, input *CreateRepoWatchInput) (*WatchOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)
	repo := session.Repo(c)
	body := input.Body

	if !model.IsWatchableEvent(body.Event) {
		return nil, huma.Error400BadRequest("event cannot be watched: " + string(body.Event))
	}
	target := body.Target
	switch body.Event {
	case model.EventTag, model.EventRelease:
		target = ""
	default:
		if target == "" {
			return nil, huma.Error400BadRequest("target is required for event " + string(body.Event))
		}
		if utf8.RuneCountInString(target) > maxWatchTargetLength {
			return nil, huma.Error400BadRequest(fmt.Sprintf("target must not be longer than %d characters", maxWatchTargetLength))
		}
	}
	events, err := normalizeWatchSettings(body.WatchSettingsBody)
	if err != nil {
		return nil, err
	}

	_, err = _store.WatchFindByKey(user.ID, repo.ID, body.Event, target)
	if err == nil {
		return nil, huma.Error409Conflict("already watching this target")
	}
	if !errors.Is(err, types.RecordNotExist) {
		return nil, huma.Error500InternalServerError("failed to check existing watch", err)
	}

	w := &model.PipelineWatch{
		UserID:      user.ID,
		RepoID:      repo.ID,
		Event:       body.Event,
		Target:      target,
		InApp:       body.InApp,
		Email:       body.Email,
		EmailEvents: events,
	}
	if err := _store.WatchCreate(w); err != nil {
		return nil, huma.Error500InternalServerError("failed to create watch", err)
	}

	// A watch created while the key is failing starts read.
	key := model.WatchKey{RepoID: repo.ID, Event: w.Event, Target: w.Target}
	if svc := server.Config.Services.WatchService; svc != nil {
		if err := svc.Seed(ctx, key); err != nil {
			log.Error().Err(err).Int64("repo_id", repo.ID).Msg("failed to seed watch state")
		}
	}
	if err := markRead(_store, w); err != nil {
		log.Error().Err(err).Int64("watch_id", w.ID).Msg("failed to start watch read")
	}
	return &WatchOutput{Body: w}, nil
}

type ListRepoWatchSuggestionsOutput struct {
	Body []WatchSuggestion
}

func ListRepoWatchSuggestions(ctx context.Context, _ *ListRepoWatchesInput) (*ListRepoWatchSuggestionsOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	repo := session.Repo(c)

	pipelines, err := _store.GetPipelineList(repo, &model.ListOptions{Page: 1, PerPage: suggestionWindow}, &model.PipelineFilter{
		Events: []model.WebhookEvent{model.EventPush, model.EventManual, model.EventCron, model.EventDeploy, model.EventTag, model.EventRelease},
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list pipelines", err)
	}

	out := []WatchSuggestion{}
	seen := map[WatchSuggestion]bool{}
	for _, p := range pipelines {
		event, target, ok := model.WatchKeyFor(p)
		if !ok {
			continue
		}
		if target == "" && event != model.EventTag && event != model.EventRelease {
			continue
		}
		s := WatchSuggestion{Event: event, Target: target}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return &ListRepoWatchSuggestionsOutput{Body: out}, nil
}
