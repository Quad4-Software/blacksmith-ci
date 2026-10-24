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

package watch

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/services/notification"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/types"
	"codefloe.com/crowci/crow/v6/shared/token"
)

// seedLimit is the number of recent pipelines used to build initial state.
const seedLimit = 50

// seedScanFactor widens the pipeline window for keys filtered in Go.
const seedScanFactor = 4

// keyLockStripes is the number of mutexes the watch keys are spread over.
const keyLockStripes = 64

// Service evaluates finished pipelines against user watches.
type Service interface {
	// PipelineFinished updates the watch state of the pipeline's key and sends
	// emails to watchers whose settings match.
	PipelineFinished(ctx context.Context, repo *model.Repo, pipeline *model.Pipeline) error
	// Seed builds the state of a key from recent runs. It rebuilds when no
	// state rows exist or when the key has no watcher besides a new one, since
	// state is not kept up to date while nobody watches the key.
	Seed(ctx context.Context, key model.WatchKey) error
	// UnsubscribeURL returns the signed link that turns off the email channel of a watch.
	UnsubscribeURL(w *model.PipelineWatch) string
}

type service struct {
	store      store.Store
	mailer     notification.Service
	serverHost string
	jwtSecret  string
	// keyLocks serialize the read, apply and write of a key's state within
	// this process, so concurrent finishes cannot overwrite a newer state.
	keyLocks [keyLockStripes]sync.Mutex
}

// NewService creates a watch service. The mailer may be nil when SMTP is not configured.
func NewService(s store.Store, mailer notification.Service, serverHost, jwtSecret string) Service {
	return &service{store: s, mailer: mailer, serverHost: serverHost, jwtSecret: jwtSecret}
}

func (s *service) PipelineFinished(ctx context.Context, repo *model.Repo, pipeline *model.Pipeline) error {
	event, target, ok := model.WatchKeyFor(pipeline)
	if !ok {
		return nil
	}
	if !model.IsGenuineOutcome(pipeline.Status) {
		return nil
	}
	watches, err := s.store.WatchListByKey(repo.ID, event, target)
	if err != nil {
		return fmt.Errorf("listing watches for repo %d: %w", repo.ID, err)
	}
	if len(watches) == 0 {
		return nil
	}

	workflows := pipeline.Workflows
	if workflows == nil {
		workflows, err = s.store.WorkflowGetTree(pipeline)
		if err != nil {
			return fmt.Errorf("loading workflows of pipeline %d: %w", pipeline.ID, err)
		}
	}
	run, ok := ResultFromPipeline(pipeline, workflows)
	if !ok {
		return nil
	}

	key := model.WatchKey{RepoID: repo.ID, Event: event, Target: target}
	prev, next, applied, err := s.applyAndStore(key, run)
	if err != nil {
		return err
	}
	if !applied {
		// the same result of this pipeline was evaluated already
		return nil
	}

	if s.mailer == nil {
		return nil
	}
	wasFailing, nowFailing := IsFailing(prev), IsFailing(next)
	label := string(pipeline.Status)
	switch {
	case wasFailing && !nowFailing:
		label = "fixed"
	case !wasFailing && nowFailing:
		label = "broken"
	}
	runFailing := pipeline.Status == model.StatusFailure || pipeline.Status == model.StatusError

	for _, w := range watches {
		if !w.Email || !wantsEmail(w, runFailing, pipeline.Status == model.StatusSuccess, wasFailing != nowFailing) {
			continue
		}
		user, err := s.store.GetUser(w.UserID)
		if err != nil || user == nil {
			if err != nil && !errors.Is(err, types.RecordNotExist) {
				log.Error().Err(err).Int64("user_id", w.UserID).Msg("watch: loading user failed")
			}
			continue
		}
		if user.Email == "" || !CanPull(s.store, user, repo) {
			continue
		}
		if err := s.mailer.SendPipelineEmail(ctx, user.Email, repo, pipeline, label, s.UnsubscribeURL(w)); err != nil {
			log.Error().Err(err).Int64("watch_id", w.ID).Msg("watch: sending email failed")
		}
	}
	return nil
}

// lockKey locks the stripe of the key and returns its unlock function.
func (s *service) lockKey(key model.WatchKey) func() {
	h := fnv.New32a()
	_, _ = fmt.Fprintf(h, "%d\x00%s\x00%s", key.RepoID, key.Event, key.Target)
	m := &s.keyLocks[h.Sum32()%keyLockStripes]
	m.Lock()
	return m.Unlock
}

// applyAndStore applies the run to the stored state and persists it while
// holding the key's lock. Applied is false when the state already holds this
// run's result, for example when a pipeline is reported finished twice. A
// failed replace is retried once on fresh state, since finishes handled by
// another server process can still collide on the unique index.
func (s *service) applyAndStore(key model.WatchKey, run RunResult) (prev, next []*model.WatchWorkflowState, applied bool, err error) {
	defer s.lockKey(key)()
	for attempt := 0; attempt < 2; attempt++ {
		prev, err = s.store.WatchStateList(key.RepoID, key.Event, key.Target)
		if err != nil {
			return nil, nil, false, fmt.Errorf("loading watch state: %w", err)
		}
		if alreadyApplied(prev, run) {
			return prev, prev, false, nil
		}
		next = ApplyRun(key, prev, run)
		if err = s.store.WatchStateReplace(key.RepoID, key.Event, key.Target, next); err == nil {
			return prev, next, true, nil
		}
		log.Warn().Err(err).Int64("repo_id", key.RepoID).Msg("watch: replacing state failed")
	}
	return nil, nil, false, fmt.Errorf("storing watch state: %w", err)
}

// alreadyApplied reports whether every workflow of the run is stored with
// this run's pipeline and the same outcome.
func alreadyApplied(prev []*model.WatchWorkflowState, run RunResult) bool {
	if len(run.Workflows) == 0 {
		return false
	}
	rows := make(map[string]*model.WatchWorkflowState, len(prev))
	for _, row := range prev {
		rows[row.WorkflowName] = row
	}
	for name, failing := range run.Workflows {
		row, ok := rows[name]
		if !ok || row.PipelineID != run.PipelineID || row.Failing != failing {
			return false
		}
	}
	return true
}

func wantsEmail(w *model.PipelineWatch, runFailing, runSuccess, flipped bool) bool {
	return (slices.Contains(w.EmailEvents, model.NotificationEventFailure) && runFailing) ||
		(slices.Contains(w.EmailEvents, model.NotificationEventSuccess) && runSuccess) ||
		(slices.Contains(w.EmailEvents, model.NotificationEventStatusChange) && flipped)
}

func (s *service) Seed(_ context.Context, key model.WatchKey) error {
	watches, err := s.store.WatchListByKey(key.RepoID, key.Event, key.Target)
	if err != nil {
		return fmt.Errorf("listing watches for repo %d: %w", key.RepoID, err)
	}
	defer s.lockKey(key)()
	existing, err := s.store.WatchStateList(key.RepoID, key.Event, key.Target)
	if err != nil {
		return fmt.Errorf("loading watch state: %w", err)
	}
	// Another watcher kept the state up to date. Without one, finished runs
	// were not evaluated, so rows left from an earlier watch may be stale.
	if len(existing) > 0 && len(watches) > 1 {
		return nil
	}
	repo, err := s.store.GetRepo(key.RepoID)
	if err != nil {
		return fmt.Errorf("loading repo %d: %w", key.RepoID, err)
	}

	filter := &model.PipelineFilter{Events: []model.WebhookEvent{key.Event}}
	limit := seedLimit
	if key.Event == model.EventPush || key.Event == model.EventManual {
		filter.Branch = key.Target
	} else {
		// cron and deployment targets are filtered in Go, so read a wider window
		limit = seedLimit * seedScanFactor
	}
	pipelines, err := s.store.GetPipelineList(repo, &model.ListOptions{Page: 1, PerPage: limit}, filter)
	if err != nil {
		return fmt.Errorf("listing pipelines: %w", err)
	}

	// the list is newest-first; keep the newest matches and apply oldest-first
	var matched []*model.Pipeline
	for _, p := range pipelines {
		if _, target, ok := model.WatchKeyFor(p); ok && target == key.Target && len(matched) < seedLimit {
			matched = append(matched, p)
		}
	}
	var states []*model.WatchWorkflowState
	for i := len(matched) - 1; i >= 0; i-- {
		p := matched[i]
		workflows, err := s.store.WorkflowGetTree(p)
		if err != nil {
			return fmt.Errorf("loading workflows of pipeline %d: %w", p.ID, err)
		}
		run, ok := ResultFromPipeline(p, workflows)
		if !ok {
			continue
		}
		states = ApplyRun(key, states, run)
	}
	if len(states) == 0 && len(existing) == 0 {
		return nil
	}
	if err := s.store.WatchStateReplace(key.RepoID, key.Event, key.Target, states); err != nil {
		return fmt.Errorf("storing seeded state: %w", err)
	}
	return nil
}

func (s *service) UnsubscribeURL(w *model.PipelineWatch) string {
	if s.jwtSecret == "" {
		return ""
	}
	t := token.New(token.UnsubscribeToken)
	t.Set("watch_id", strconv.FormatInt(w.ID, 10))
	t.Set("user_id", strconv.FormatInt(w.UserID, 10))
	signed, err := t.Sign(s.jwtSecret)
	if err != nil {
		log.Error().Err(err).Msg("failed to sign unsubscribe token")
		return ""
	}
	return fmt.Sprintf("%s/api/v1/notifications/unsubscribe?token=%s",
		strings.TrimRight(s.serverHost, "/"), url.QueryEscape(signed))
}
