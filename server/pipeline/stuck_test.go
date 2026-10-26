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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v6/server"
	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pubsub"
	"codefloe.com/crowci/crow/v6/server/queue"
	queue_mocks "codefloe.com/crowci/crow/v6/server/queue/mocks"
	services_mocks "codefloe.com/crowci/crow/v6/server/services/mocks"
	notification_mocks "codefloe.com/crowci/crow/v6/server/services/notification/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func TestStuckReason(t *testing.T) {
	t.Parallel()

	now := time.Unix(10_000, 0)
	queued := map[string]bool{"1": true}
	old := now.Add(-2 * time.Hour).Unix()
	fresh := now.Add(-time.Minute).Unix()

	tests := []struct {
		name      string
		task      *model.Task
		waiting   bool
		created   int64
		threshold time.Duration
		want      string
	}{
		{
			name:      "running task is never stuck",
			task:      &model.Task{ID: "5", AgentID: 3},
			created:   old,
			threshold: time.Hour,
		},
		{
			name:      "task held by an approval gate is never stuck",
			task:      &model.Task{ID: "5", RequiresApproval: true, ApprovalState: model.ApprovalPending},
			waiting:   true,
			created:   old,
			threshold: time.Hour,
		},
		{
			name:    "waiting on a dependency that left the queue is blocked",
			task:    &model.Task{ID: "5", Dependencies: model.JSONSliceString{"2"}},
			waiting: true,
			created: fresh,
			want:    StuckReasonBlocked,
		},
		{
			name:      "waiting on a queued dependency is not blocked",
			task:      &model.Task{ID: "5", Dependencies: model.JSONSliceString{"1", "2"}},
			waiting:   true,
			created:   fresh,
			threshold: time.Hour,
		},
		{
			name:      "pending past the threshold waited too long",
			task:      &model.Task{ID: "5"},
			created:   old,
			threshold: time.Hour,
			want:      StuckReasonWaitedTooLong,
		},
		{
			name:      "pending within the threshold is fine",
			task:      &model.Task{ID: "5"},
			created:   fresh,
			threshold: time.Hour,
		},
		{
			name:    "zero threshold disables the age check",
			task:    &model.Task{ID: "5"},
			created: old,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, StuckReason(tt.task, tt.waiting, queued, tt.created, tt.threshold, now))
		})
	}
}

func TestSweepStuckTasksNotifiesOncePerThresholdCrossing(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	now := time.Unix(100_000, 0)
	since := now.Add(-time.Minute)
	threshold := time.Hour

	// crossed the threshold 30s ago, inside (since, now]
	crossing := &model.Pipeline{ID: 1, RepoID: 7, Number: 11, Sender: "alice", Created: now.Add(-threshold - 30*time.Second).Unix()}
	// crossed the threshold an hour ago and was reported then
	reported := &model.Pipeline{ID: 2, RepoID: 7, Number: 12, Sender: "alice", Created: now.Add(-2 * threshold).Unix()}
	repo := &model.Repo{ID: 7, FullName: "org/repo", Visibility: model.VisibilityPublic}

	q := queue_mocks.NewMockQueue(t)
	q.On("Info", mock.Anything).Return(queue.InfoT{
		Pending: []*model.Task{
			{ID: "100", Name: "build", PipelineID: 1},
			{ID: "101", Name: "test", PipelineID: 1},
			{ID: "200", Name: "build", PipelineID: 2},
		},
	})

	s := store_mocks.NewMockStore(t)
	s.On("GetPipeline", int64(1)).Return(crossing, nil).Once()
	s.On("GetPipeline", int64(2)).Return(reported, nil).Once()
	s.On("GetRepo", int64(7)).Return(repo, nil)
	s.On("GetUserLogin", "alice").Return(&model.User{Login: "alice", Email: "alice@example.com"}, nil)

	mailer := notification_mocks.NewMockService(t)
	mailer.On("SendStuckPipelineEmail", mock.Anything, "alice@example.com", repo, crossing, []string{"build", "test"}, threshold).Return(nil).Once()

	SweepStuckTasks(ctx, s, q, StuckSweeperConfig{Threshold: threshold, Mailer: mailer}, since, now)
}

func TestSweepStuckTasksWithoutMailerOnlyReaps(t *testing.T) {
	t.Parallel()

	q := queue_mocks.NewMockQueue(t)
	q.On("Info", mock.Anything).Return(queue.InfoT{
		Pending: []*model.Task{{ID: "100", Name: "build", PipelineID: 1}},
	})
	// no store calls are expected: nothing is blocked and email is disabled
	s := store_mocks.NewMockStore(t)

	now := time.Now()
	SweepStuckTasks(t.Context(), s, q, StuckSweeperConfig{Threshold: time.Hour}, now.Add(-time.Minute), now)
}

func TestSweepStuckTasksSkipsBlockedPipeline(t *testing.T) {
	ctx := t.Context()

	pl := &model.Pipeline{ID: 100, RepoID: 7, Status: model.StatusRunning}
	repo := &model.Repo{ID: 7, FullName: "org/repo"}
	failed := &model.Workflow{ID: 1, PipelineID: 100, Name: "build", State: model.StatusFailure}
	blocked := &model.Workflow{ID: 2, PipelineID: 100, Name: "deploy", State: model.StatusPending, DependsOn: model.JSONSliceString{"build"}}

	q := queue_mocks.NewMockQueue(t)
	q.On("Info", mock.Anything).Return(queue.InfoT{
		WaitingOnDeps: []*model.Task{{ID: "2", PipelineID: 100, Dependencies: model.JSONSliceString{"1"}}},
	})
	q.On("EvictAtOnce", mock.Anything, []string{"2"}).Return(nil)

	s := store_mocks.NewMockStore(t)
	s.On("GetPipeline", int64(100)).Return(pl, nil)
	// the cascade marks the in-memory tree, so the second load sees it skipped
	s.On("WorkflowGetTree", mock.Anything).Return([]*model.Workflow{failed, blocked}, nil)
	s.On("TaskListPending").Return([]*model.Task{{ID: "2"}}, nil)
	s.On("WorkflowUpdate", mock.MatchedBy(func(wf *model.Workflow) bool {
		return wf.ID == 2 && wf.State == model.StatusSkipped
	})).Return(nil)
	var finalStatus model.StatusValue
	s.On("UpdatePipeline", mock.Anything).Run(func(args mock.Arguments) {
		if p, ok := args.Get(0).(*model.Pipeline); ok {
			finalStatus = p.Status
		}
	}).Return(nil)
	s.On("GetRepo", int64(7)).Return(repo, nil)

	manager := services_mocks.NewMockManager(t)
	manager.On("ForgeFromRepo", repo).Return(nil, errors.New("no forge in test"))

	originalManager := server.Config.Services.Manager
	originalPubsub := server.Config.Services.Pubsub
	server.Config.Services.Manager = manager
	server.Config.Services.Pubsub = pubsub.New()
	t.Cleanup(func() {
		server.Config.Services.Manager = originalManager
		server.Config.Services.Pubsub = originalPubsub
	})

	now := time.Now()
	SweepStuckTasks(ctx, s, q, StuckSweeperConfig{}, now.Add(-time.Minute), now)

	assert.Equal(t, model.StatusSkipped, blocked.State)
	assert.Equal(t, model.StatusFailure, finalStatus)
}

func TestCancelQueuedTasksEvictsLeftoverRows(t *testing.T) {
	ctx := t.Context()

	s := store_mocks.NewMockStore(t)
	s.On("WorkflowLoad", int64(1)).Return(nil, errors.New("not found"))
	s.On("WorkflowLoad", int64(2)).Return(&model.Workflow{ID: 2, State: model.StatusSuccess}, nil)

	q := queue_mocks.NewMockQueue(t)
	for _, id := range []string{"1", "2"} {
		q.On("Evict", mock.Anything, id).Return(nil)
		q.On("Error", mock.Anything, id, queue.ErrCancel).Return(queue.ErrNotFound)
	}
	originalQueue := server.Config.Services.Queue
	server.Config.Services.Queue = q
	t.Cleanup(func() { server.Config.Services.Queue = originalQueue })

	failed := CancelQueuedTasks(ctx, s, []string{"1", "2", "not-a-number"})

	assert.Equal(t, map[string]string{"not-a-number": `invalid task id "not-a-number"`}, failed)
}

func TestCancelQueuedTasksKillsPipelineWhenSelectionCoversIt(t *testing.T) {
	ctx := t.Context()

	pl := &model.Pipeline{ID: 100, RepoID: 7, Status: model.StatusPending}
	repo := &model.Repo{ID: 7, UserID: 1, ForgeID: 1, FullName: "org/repo"}
	done := &model.Workflow{ID: 1, PipelineID: 100, State: model.StatusSuccess}
	stuck := &model.Workflow{ID: 2, PipelineID: 100, State: model.StatusPending}

	s := store_mocks.NewMockStore(t)
	s.On("WorkflowLoad", int64(2)).Return(stuck, nil)
	s.On("GetPipeline", int64(100)).Return(pl, nil)
	s.On("GetRepo", int64(7)).Return(repo, nil)
	s.On("GetUser", int64(1)).Return(&model.User{ID: 1, Expiry: time.Now().Add(time.Hour).Unix()}, nil)
	s.On("UserForgeGet", int64(1), int64(1)).Return(nil, errors.New("none"))
	s.On("WorkflowGetTree", mock.Anything).Return([]*model.Workflow{done, stuck}, nil)
	s.On("WorkflowUpdate", mock.Anything).Return(nil)
	var finalStatus model.StatusValue
	s.On("UpdatePipeline", mock.Anything).Run(func(args mock.Arguments) {
		if p, ok := args.Get(0).(*model.Pipeline); ok {
			finalStatus = p.Status
		}
	}).Return(nil)

	forge := forge_mocks.NewMockForge(t)
	forge.On("Status", mock.Anything, mock.Anything, repo, mock.Anything, mock.Anything).Return(nil).Maybe()
	manager := services_mocks.NewMockManager(t)
	manager.On("ForgeFromRepo", repo).Return(forge, nil)

	q := queue_mocks.NewMockQueue(t)
	q.On("EvictAtOnce", mock.Anything, []string{"2"}).Return(nil)
	q.On("ErrorAtOnce", mock.Anything, []string{"2"}, queue.ErrCancel).Return(nil)

	originalManager := server.Config.Services.Manager
	originalQueue := server.Config.Services.Queue
	originalPubsub := server.Config.Services.Pubsub
	server.Config.Services.Manager = manager
	server.Config.Services.Queue = q
	server.Config.Services.Pubsub = pubsub.New()
	t.Cleanup(func() {
		server.Config.Services.Manager = originalManager
		server.Config.Services.Queue = originalQueue
		server.Config.Services.Pubsub = originalPubsub
	})

	failed := CancelQueuedTasks(ctx, s, []string{"2"})

	assert.Empty(t, failed)
	// a canceled pipeline must not roll up to success from skipped workflows
	assert.Equal(t, model.StatusKilled, finalStatus)
}

func TestFindStuckPipelinesGroupsByPipelineOldestFirst(t *testing.T) {
	t.Parallel()

	now := time.Unix(100_000, 0)
	older := &model.Pipeline{ID: 1, Number: 7, Created: now.Add(-5 * time.Hour).Unix()}
	newer := &model.Pipeline{ID: 2, Number: 9, Created: now.Add(-2 * time.Hour).Unix()}
	fresh := &model.Pipeline{ID: 3, Number: 10, Created: now.Add(-time.Minute).Unix()}

	q := queue_mocks.NewMockQueue(t)
	q.On("Info", mock.Anything).Return(queue.InfoT{
		Pending: []*model.Task{
			{ID: "20", Name: "build", PipelineID: 2},
			{ID: "10", Name: "lint", PipelineID: 1},
			{ID: "30", Name: "test", PipelineID: 3},
		},
		WaitingOnDeps: []*model.Task{
			{ID: "11", Name: "deploy", PipelineID: 1, Dependencies: model.JSONSliceString{"9"}},
		},
	})
	s := store_mocks.NewMockStore(t)
	s.On("GetPipeline", int64(1)).Return(older, nil).Once()
	s.On("GetPipeline", int64(2)).Return(newer, nil).Once()
	s.On("GetPipeline", int64(3)).Return(fresh, nil).Once()

	got := FindStuckPipelines(t.Context(), s, q, time.Hour, now)

	assert.Equal(t, []StuckPipelineTasks{
		{Pipeline: older, Workflows: []model.StuckWorkflow{
			{Name: "lint", Reason: StuckReasonWaitedTooLong},
			{Name: "deploy", Reason: StuckReasonBlocked},
		}},
		{Pipeline: newer, Workflows: []model.StuckWorkflow{{Name: "build", Reason: StuckReasonWaitedTooLong}}},
	}, got)
}

func TestTriggeredBy(t *testing.T) {
	t.Parallel()

	alice := &model.User{Login: "alice"}
	assert.True(t, TriggeredBy(&model.Pipeline{Sender: "alice", Author: "bob"}, alice))
	assert.False(t, TriggeredBy(&model.Pipeline{Sender: "bob", Author: "alice"}, alice))
	assert.True(t, TriggeredBy(&model.Pipeline{Author: "alice"}, alice))
	assert.False(t, TriggeredBy(&model.Pipeline{}, &model.User{}))
}

func TestMayReceiveStuckNotice(t *testing.T) {
	t.Parallel()

	private := &model.Repo{ID: 7, ForgeID: 1, Visibility: model.VisibilityPrivate}
	public := &model.Repo{ID: 8, ForgeID: 2, Visibility: model.VisibilityPublic}
	member := &model.User{ID: 1, ForgeID: 1}
	outsider := &model.User{ID: 2, ForgeID: 1}
	linked := &model.User{ID: 3, ForgeID: 1}

	s := store_mocks.NewMockStore(t)
	s.On("PermFind", member, private).Return(&model.Perm{Pull: true}, nil)
	s.On("PermFind", outsider, private).Return(&model.Perm{}, nil)
	// a login on forge 1 says nothing about who owns that login on forge 2
	s.On("UserForgeGet", int64(1), int64(2)).Return(nil, errors.New("not linked"))
	s.On("UserForgeGet", int64(3), int64(2)).Return(&model.UserForge{UserID: 3, ForgeID: 2}, nil)

	assert.True(t, MayReceiveStuckNotice(s, member, private))
	assert.False(t, MayReceiveStuckNotice(s, outsider, private))
	assert.False(t, MayReceiveStuckNotice(s, member, public))
	assert.True(t, MayReceiveStuckNotice(s, linked, public))
	assert.False(t, MayReceiveStuckNotice(s, nil, public))
}
