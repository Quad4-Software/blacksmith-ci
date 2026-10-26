// Copyright 2026 Crow CI Authors
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

package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/forge"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/queue"
	"codefloe.com/crowci/crow/v6/server/services/lock"
	"codefloe.com/crowci/crow/v6/server/services/notification"
	"codefloe.com/crowci/crow/v6/server/services/watch"
	"codefloe.com/crowci/crow/v6/server/store"
)

const (
	// StuckReasonBlocked marks a task held back by dependencies that have all
	// left the queue without succeeding, so the queue will never assign it.
	StuckReasonBlocked = "blocked"
	// StuckReasonWaitedTooLong marks a task that has waited for an agent
	// longer than the configured threshold.
	StuckReasonWaitedTooLong = "waited_too_long"

	stuckSweepLeaderLock = "stuck-queue-sweeper"
)

// QueuedTaskIDs returns the IDs of every task the queue still holds.
func QueuedTaskIDs(info queue.InfoT) map[string]bool {
	ids := make(map[string]bool, len(info.Pending)+len(info.WaitingOnDeps)+len(info.Running))
	for _, list := range [][]*model.Task{info.Pending, info.WaitingOnDeps, info.Running} {
		for _, task := range list {
			ids[task.ID] = true
		}
	}
	return ids
}

// StuckReason classifies a queued task, returning an empty string for tasks
// that are not stuck.
// Running tasks and tasks held by an approval gate are never stuck: the first
// are on an agent, the second wait for a person on purpose.
// A waiting task whose dependencies have all left the queue is blocked; any
// other task created longer ago than threshold has waited too long.
// A zero threshold disables the age check.
func StuckReason(task *model.Task, waiting bool, queued map[string]bool, created int64, threshold time.Duration, now time.Time) string {
	if task.AgentID != 0 || (task.RequiresApproval && !task.ApprovalState.Granted()) {
		return ""
	}
	if waiting && len(task.Dependencies) > 0 && !anyQueued(task.Dependencies, queued) {
		return StuckReasonBlocked
	}
	if threshold > 0 && created > 0 && now.Sub(time.Unix(created, 0)) >= threshold {
		return StuckReasonWaitedTooLong
	}
	return ""
}

func anyQueued(ids []string, queued map[string]bool) bool {
	for _, id := range ids {
		if queued[id] {
			return true
		}
	}
	return false
}

// StuckPipelineTasks is a pipeline together with its stuck workflows.
type StuckPipelineTasks struct {
	Pipeline  *model.Pipeline
	Workflows []model.StuckWorkflow
}

// FindStuckPipelines groups the stuck tasks of the queue by pipeline, oldest
// pipeline first. Pipelines that cannot be loaded are left out.
func FindStuckPipelines(ctx context.Context, s store.Store, q queue.Queue, threshold time.Duration, now time.Time) []StuckPipelineTasks {
	info := q.Info(ctx)
	queued := QueuedTaskIDs(info)

	byPipeline := make(map[int64]*StuckPipelineTasks)
	loaded := make(map[int64]*model.Pipeline)
	var order []int64
	collect := func(tasks []*model.Task, waiting bool) {
		for _, task := range tasks {
			if task.PipelineID == 0 {
				continue
			}
			p, ok := loaded[task.PipelineID]
			if !ok {
				var err error
				if p, err = s.GetPipeline(task.PipelineID); err != nil {
					p = nil
				}
				loaded[task.PipelineID] = p
			}
			if p == nil {
				continue
			}
			reason := StuckReason(task, waiting, queued, p.Created, threshold, now)
			if reason == "" {
				continue
			}
			entry, seen := byPipeline[p.ID]
			if !seen {
				entry = &StuckPipelineTasks{Pipeline: p}
				byPipeline[p.ID] = entry
				order = append(order, p.ID)
			}
			entry.Workflows = append(entry.Workflows, model.StuckWorkflow{Name: task.Name, Reason: reason})
		}
	}
	collect(info.Pending, false)
	collect(info.WaitingOnDeps, true)

	result := make([]StuckPipelineTasks, 0, len(order))
	for _, id := range order {
		result = append(result, *byPipeline[id])
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Pipeline.Created < result[j].Pipeline.Created })
	return result
}

// TriggeredBy reports whether the user triggered the pipeline, matching the
// sender login or, without a sender, the commit author login.
func TriggeredBy(p *model.Pipeline, user *model.User) bool {
	if p.Sender != "" {
		return p.Sender == user.Login
	}
	return p.Author != "" && p.Author == user.Login
}

// MayReceiveStuckNotice reports whether a stuck notice about a pipeline of
// repo may go to user: the user must have an identity on the repo's forge,
// since logins only identify a person within one forge, and read access to
// the repo.
func MayReceiveStuckNotice(s store.Store, user *model.User, repo *model.Repo) bool {
	if user == nil || repo == nil {
		return false
	}
	if user.ForgeID != repo.ForgeID {
		if userForge, err := s.UserForgeGet(user.ID, repo.ForgeID); err != nil || userForge == nil {
			return false
		}
	}
	return watch.CanPull(s, user, repo)
}

// CancelQueuedTasks cancels the workflows behind the given queue task IDs.
// When the tasks cover every pending or running workflow of a pipeline, the
// whole pipeline is canceled and ends as killed; otherwise each workflow is
// canceled on its own through CancelWorkflow.
// Either way the workflow states, the pipeline status, the forge commit status
// and subscribers are updated like a cancel from the pipeline page.
// A task whose workflow is gone or already finished is a leftover queue row;
// it is evicted from the queue.
// The returned map holds an error message for every task that failed.
func CancelQueuedTasks(ctx context.Context, s store.Store, taskIDs []string) map[string]string {
	failed := make(map[string]string)
	fail := func(taskID string, err error) {
		log.Error().Err(err).Str("task_id", taskID).Msg("queue: batch cancel failed")
		failed[taskID] = err.Error()
	}

	selected := make(map[int64][]int64)
	var order []int64
	for _, taskID := range taskIDs {
		workflowID, err := strconv.ParseInt(taskID, 10, 64)
		if err != nil {
			fail(taskID, fmt.Errorf("invalid task id %q", taskID))
			continue
		}
		workflow, err := s.WorkflowLoad(workflowID)
		if err != nil || !workflow.Running() {
			if err := evictLeftoverTask(ctx, taskID); err != nil {
				fail(taskID, err)
			}
			continue
		}
		if _, seen := selected[workflow.PipelineID]; !seen {
			order = append(order, workflow.PipelineID)
		}
		selected[workflow.PipelineID] = append(selected[workflow.PipelineID], workflowID)
	}

	for _, pipelineID := range order {
		for workflowID, err := range cancelPipelineWorkflows(ctx, s, pipelineID, selected[pipelineID]) {
			fail(strconv.FormatInt(workflowID, 10), err)
		}
	}
	return failed
}

// cancelPipelineWorkflows cancels the given workflows of one pipeline and
// returns the errors by workflow ID.
func cancelPipelineWorkflows(ctx context.Context, s store.Store, pipelineID int64, workflowIDs []int64) map[int64]error {
	failAll := func(err error) map[int64]error {
		errs := make(map[int64]error, len(workflowIDs))
		for _, id := range workflowIDs {
			errs[id] = err
		}
		return errs
	}

	pl, err := s.GetPipeline(pipelineID)
	if err != nil {
		// the workflows outlived their pipeline, so only the queue rows remain
		errs := make(map[int64]error)
		for _, id := range workflowIDs {
			if err := evictLeftoverTask(ctx, strconv.FormatInt(id, 10)); err != nil {
				errs[id] = err
			}
		}
		return errs
	}
	repo, err := s.GetRepo(pl.RepoID)
	if err != nil {
		return failAll(fmt.Errorf("load repo: %w", err))
	}
	_forge, repoUser, err := repoForgeUser(ctx, s, repo)
	if err != nil {
		return failAll(err)
	}

	if coversRemainingWork(s, pl, workflowIDs) {
		err := Cancel(ctx, _forge, s, repo, repoUser, pl)
		var badRequest *ErrBadRequest
		if !errors.As(err, &badRequest) {
			if err != nil {
				return failAll(err)
			}
			return nil
		}
		// the pipeline is already finished; clean up its workflows one by one
	}

	errs := make(map[int64]error)
	for _, id := range workflowIDs {
		err := CancelWorkflow(ctx, _forge, s, repo, repoUser, pl, id)
		var badRequest *ErrBadRequest
		if errors.As(err, &badRequest) {
			err = evictLeftoverTask(ctx, strconv.FormatInt(id, 10))
		}
		if err != nil {
			errs[id] = err
		}
	}
	return errs
}

// coversRemainingWork reports whether workflowIDs include every pending or
// running workflow of the pipeline.
func coversRemainingWork(s store.Store, pl *model.Pipeline, workflowIDs []int64) bool {
	workflows, err := s.WorkflowGetTree(pl)
	if err != nil {
		return false
	}
	selected := make(map[int64]bool, len(workflowIDs))
	for _, id := range workflowIDs {
		selected[id] = true
	}
	for _, workflow := range workflows {
		if workflow.Running() && !selected[workflow.ID] {
			return false
		}
	}
	return true
}

// evictLeftoverTask removes a queue row that no workflow will ever pick up.
func evictLeftoverTask(ctx context.Context, taskID string) error {
	q := server.Config.Services.Queue
	if err := q.Evict(ctx, taskID); err != nil && !errors.Is(err, queue.ErrNotFound) {
		return fmt.Errorf("evict task: %w", err)
	}
	// wake anything waiting on the task, mirroring CancelWorkflow
	if err := q.Error(ctx, taskID, queue.ErrCancel); err != nil && !errors.Is(err, queue.ErrNotFound) {
		log.Debug().Err(err).Str("task_id", taskID).Msg("queue: signal canceled leftover task")
	}
	return nil
}

// repoForgeUser returns the forge of a repo and its owner with the tokens of
// that forge, for system actions that push a commit status.
func repoForgeUser(ctx context.Context, s store.Store, repo *model.Repo) (forge.Forge, *model.User, error) {
	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		return nil, nil, fmt.Errorf("load forge: %w", err)
	}
	user, err := s.GetUser(repo.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("load repo owner: %w", err)
	}
	if userForge, ufErr := s.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
	}
	if err := forge.RefreshUserForge(ctx, _forge, s, user, repo.ForgeID); err != nil {
		log.Warn().Err(err).Str("repo", repo.FullName).Msg("refresh repo owner token failed")
	}
	return _forge, user, nil
}

// StuckSweeperConfig configures the stuck queue sweeper.
type StuckSweeperConfig struct {
	// Threshold is how long a task may wait before it counts as stuck.
	// Zero disables the email notification.
	Threshold time.Duration
	// Interval is how often the sweeper runs.
	Interval time.Duration
	// Mailer sends the stuck notification; nil disables it.
	Mailer notification.Service
}

// RunStuckSweeper periodically skips blocked workflows and notifies pipeline
// authors once their workflow has waited longer than the threshold.
// It runs on the instance holding the leader lock, so each notification is
// sent by one server only.
func RunStuckSweeper(ctx context.Context, s store.Store, q queue.Queue, lockService *lock.Service, leaderLockTTL time.Duration, cfg StuckSweeperConfig) {
	for {
		lockCtx, releaseLock, err := lockService.AcquireLock(ctx, stuckSweepLeaderLock, leaderLockTTL)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(cfg.Interval):
				continue
			}
		}

		runStuckSweeperAsLeader(ctx, lockCtx, s, q, cfg)
		releaseLock()

		if ctx.Err() != nil {
			return
		}
	}
}

func runStuckSweeperAsLeader(ctx, lockCtx context.Context, s store.Store, q queue.Queue, cfg StuckSweeperConfig) {
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	// notify for tasks that crossed the threshold since the previous sweep;
	// starting one interval back keeps a leader change from skipping a window
	last := time.Now().Add(-cfg.Interval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-lockCtx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			SweepStuckTasks(ctx, s, q, cfg, last, now)
			last = now
		}
	}
}

// SweepStuckTasks runs one pass of the stuck queue sweeper.
// Pipelines with blocked workflows are cascade-skipped and finalized.
// Authors are emailed about pipelines with a workflow whose wait crossed the
// threshold within (since, now], so every pipeline is reported once without
// keeping state across sweeps or restarts.
func SweepStuckTasks(ctx context.Context, s store.Store, q queue.Queue, cfg StuckSweeperConfig, since, now time.Time) {
	info := q.Info(ctx)
	queued := QueuedTaskIDs(info)

	pipelines := make(map[int64]*model.Pipeline)
	loadPipeline := func(id int64) *model.Pipeline {
		if p, ok := pipelines[id]; ok {
			return p
		}
		p, err := s.GetPipeline(id)
		if err != nil {
			log.Debug().Err(err).Int64("pipeline_id", id).Msg("stuck sweeper: load pipeline failed")
			p = nil
		}
		pipelines[id] = p
		return p
	}

	reaped := make(map[int64]bool)
	for _, task := range info.WaitingOnDeps {
		if task.PipelineID == 0 || reaped[task.PipelineID] {
			continue
		}
		if StuckReason(task, true, queued, 0, 0, now) != StuckReasonBlocked {
			continue
		}
		if p := loadPipeline(task.PipelineID); p != nil {
			reaped[task.PipelineID] = true
			skipBlockedPipeline(ctx, s, q, p)
		}
	}

	if cfg.Mailer == nil || cfg.Threshold <= 0 {
		return
	}

	stuckWorkflows := make(map[int64][]string)
	var order []int64
	collect := func(tasks []*model.Task, waiting bool) {
		for _, task := range tasks {
			if task.PipelineID == 0 || reaped[task.PipelineID] {
				continue
			}
			p := loadPipeline(task.PipelineID)
			if p == nil || StuckReason(task, waiting, queued, p.Created, cfg.Threshold, now) != StuckReasonWaitedTooLong {
				continue
			}
			crossed := time.Unix(p.Created, 0).Add(cfg.Threshold)
			if !crossed.After(since) || crossed.After(now) {
				continue
			}
			if _, seen := stuckWorkflows[p.ID]; !seen {
				order = append(order, p.ID)
			}
			stuckWorkflows[p.ID] = append(stuckWorkflows[p.ID], task.Name)
		}
	}
	collect(info.Pending, false)
	collect(info.WaitingOnDeps, true)

	for _, id := range order {
		notifyStuckPipeline(ctx, s, cfg, pipelines[id], stuckWorkflows[id])
	}
}

// skipBlockedPipeline skips the workflows of a pipeline that can never run and
// finalizes the pipeline once nothing is left to run.
func skipBlockedPipeline(ctx context.Context, s store.Store, q queue.Queue, p *model.Pipeline) {
	if p.Status != model.StatusRunning && p.Status != model.StatusPending && p.Status != model.StatusBlocked {
		return
	}
	if err := CascadeSkipBlockedWorkflows(ctx, s, q, p); err != nil {
		log.Error().Err(err).Int64("pipeline_id", p.ID).Msg("stuck sweeper: cascade skip failed")
		return
	}

	workflows, err := s.WorkflowGetTree(p)
	if err != nil {
		log.Error().Err(err).Int64("pipeline_id", p.ID).Msg("stuck sweeper: load workflow tree failed")
		return
	}
	if model.IsThereRunningStage(workflows) {
		return
	}

	log.Info().Int64("pipeline_id", p.ID).Msg("stuck sweeper: finalized pipeline blocked on failed dependencies")
	done, err := UpdateStatusToDone(s, *p, model.PipelineStatus(workflows), time.Now().Unix())
	if err != nil {
		log.Error().Err(err).Int64("pipeline_id", p.ID).Msg("stuck sweeper: update pipeline status failed")
		return
	}
	done.Workflows = workflows

	repo, err := s.GetRepo(done.RepoID)
	if err != nil {
		log.Error().Err(err).Int64("repo_id", done.RepoID).Msg("stuck sweeper: load repo failed")
		return
	}
	notifyWatchersIfFinished(ctx, repo, done)
	_forge, repoUser, err := repoForgeUser(ctx, s, repo)
	if err != nil {
		log.Error().Err(err).Str("repo", repo.FullName).Msg("stuck sweeper: load forge user failed")
		publishToTopic(done, repo)
		return
	}
	publishPipeline(ctx, _forge, done, repo, repoUser)
}

// notifyStuckPipeline emails the user who triggered the pipeline, as decided
// by TriggeredBy.
func notifyStuckPipeline(ctx context.Context, s store.Store, cfg StuckSweeperConfig, p *model.Pipeline, workflowNames []string) {
	repo, err := s.GetRepo(p.RepoID)
	if err != nil {
		log.Error().Err(err).Int64("repo_id", p.RepoID).Msg("stuck sweeper: load repo failed")
		return
	}

	login := p.Sender
	if login == "" {
		login = p.Author
	}
	var recipient *model.User
	if login != "" {
		if user, err := s.GetUserLogin(login); err == nil && user != nil && user.Email != "" && MayReceiveStuckNotice(s, user, repo) {
			recipient = user
		}
	}
	if recipient == nil {
		log.Debug().Str("repo", repo.FullName).Int64("pipeline", p.Number).Msg("stuck sweeper: no Crow user with an email and repo access to notify")
		return
	}

	if err := cfg.Mailer.SendStuckPipelineEmail(ctx, recipient.Email, repo, p, workflowNames, cfg.Threshold); err != nil {
		log.Error().Err(err).Str("repo", repo.FullName).Int64("pipeline", p.Number).Msg("stuck sweeper: send email failed")
	}
}
