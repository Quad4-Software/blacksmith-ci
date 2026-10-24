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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v6/server/model"
	smocks "codefloe.com/crowci/crow/v6/server/services/mocks"
	nmocks "codefloe.com/crowci/crow/v6/server/services/notification/mocks"
)

var svcRepo = &model.Repo{ID: 1, FullName: "o/r", Visibility: model.VisibilityPublic}

func finishedPipeline(number int64, status model.StatusValue, wfs ...*model.Workflow) *model.Pipeline {
	if wfs == nil {
		wfs = []*model.Workflow{}
	}
	return &model.Pipeline{ID: number * 100, Number: number, Status: status, Event: model.EventPush, Branch: "main", Workflows: wfs}
}

func wf(name string, st model.StatusValue) *model.Workflow {
	return &model.Workflow{Name: name, State: st}
}

func emailWatch(id, userID int64, events ...model.NotificationEvent) *model.PipelineWatch {
	return &model.PipelineWatch{ID: id, UserID: userID, RepoID: 1, Event: model.EventPush, Target: "main", Email: true, EmailEvents: events}
}

func TestPipelineFinished_NoWatchersNoWrites(t *testing.T) {
	st := smocks.NewMockStore(t)
	st.EXPECT().WatchListByKey(int64(1), model.EventPush, "main").Return(nil, nil)
	svc := NewService(st, nil, "https://ci", "secret")
	assert.NoError(t, svc.PipelineFinished(t.Context(), svcRepo, finishedPipeline(1, model.StatusFailure, wf("a", model.StatusFailure))))
}

func TestPipelineFinished_PullRequestIgnored(t *testing.T) {
	st := smocks.NewMockStore(t)
	svc := NewService(st, nil, "https://ci", "secret")
	p := finishedPipeline(1, model.StatusFailure)
	p.Event = model.EventPull
	assert.NoError(t, svc.PipelineFinished(t.Context(), svcRepo, p))
}

func TestPipelineFinished_KilledIgnored(t *testing.T) {
	st := smocks.NewMockStore(t)
	svc := NewService(st, nil, "https://ci", "secret")
	assert.NoError(t, svc.PipelineFinished(t.Context(), svcRepo, finishedPipeline(1, model.StatusKilled, wf("a", model.StatusKilled))))
}

func TestPipelineFinished_ConfigErrorCountsAsFailure(t *testing.T) {
	st := smocks.NewMockStore(t)
	st.EXPECT().WatchListByKey(int64(1), model.EventPush, "main").Return([]*model.PipelineWatch{emailWatch(1, 1)}, nil)
	st.EXPECT().WatchStateList(int64(1), model.EventPush, "main").Return(nil, nil)
	st.EXPECT().WatchStateReplace(int64(1), model.EventPush, "main", mock.MatchedBy(func(s []*model.WatchWorkflowState) bool {
		return len(s) == 1 && s[0].WorkflowName == "" && s[0].Failing
	})).Return(nil)
	svc := NewService(st, nil, "https://ci", "secret")
	assert.NoError(t, svc.PipelineFinished(t.Context(), svcRepo, finishedPipeline(1, model.StatusError)))
}

func TestPipelineFinished_LoadsWorkflowsWhenNil(t *testing.T) {
	st := smocks.NewMockStore(t)
	p := finishedPipeline(1, model.StatusFailure)
	p.Workflows = nil
	st.EXPECT().WatchListByKey(int64(1), model.EventPush, "main").Return([]*model.PipelineWatch{emailWatch(1, 1)}, nil)
	st.EXPECT().WorkflowGetTree(p).Return([]*model.Workflow{wf("a", model.StatusFailure)}, nil)
	st.EXPECT().WatchStateList(int64(1), model.EventPush, "main").Return(nil, nil)
	st.EXPECT().WatchStateReplace(int64(1), model.EventPush, "main", mock.Anything).Return(nil)
	svc := NewService(st, nil, "https://ci", "secret")
	assert.NoError(t, svc.PipelineFinished(t.Context(), svcRepo, p))
}

func TestPipelineFinished_EmailsOnlyOptedInWatchers(t *testing.T) {
	st := smocks.NewMockStore(t)
	mailer := nmocks.NewMockService(t)
	off := emailWatch(2, 2, model.NotificationEventFailure)
	off.Email = false
	st.EXPECT().WatchListByKey(int64(1), model.EventPush, "main").Return([]*model.PipelineWatch{emailWatch(1, 1, model.NotificationEventFailure), off}, nil)
	st.EXPECT().WatchStateList(int64(1), model.EventPush, "main").Return(nil, nil)
	st.EXPECT().WatchStateReplace(int64(1), model.EventPush, "main", mock.Anything).Return(nil)
	st.EXPECT().GetUser(int64(1)).Return(&model.User{ID: 1, Email: "a@x.io"}, nil)
	p := finishedPipeline(1, model.StatusFailure, wf("a", model.StatusFailure))
	mailer.EXPECT().SendPipelineEmail(mock.Anything, "a@x.io", svcRepo, p, "broken", mock.Anything).Return(nil).Once()
	svc := NewService(st, mailer, "https://ci", "secret")
	assert.NoError(t, svc.PipelineFinished(t.Context(), svcRepo, p))
}

func TestPipelineFinished_StatusChangeFixed(t *testing.T) {
	st := smocks.NewMockStore(t)
	mailer := nmocks.NewMockService(t)
	st.EXPECT().WatchListByKey(int64(1), model.EventPush, "main").Return([]*model.PipelineWatch{emailWatch(1, 1, model.NotificationEventStatusChange)}, nil)
	st.EXPECT().WatchStateList(int64(1), model.EventPush, "main").Return([]*model.WatchWorkflowState{state("a", true, 1)}, nil)
	st.EXPECT().WatchStateReplace(int64(1), model.EventPush, "main", mock.Anything).Return(nil)
	st.EXPECT().GetUser(int64(1)).Return(&model.User{ID: 1, Email: "a@x.io"}, nil)
	p := finishedPipeline(2, model.StatusSuccess, wf("a", model.StatusSuccess))
	mailer.EXPECT().SendPipelineEmail(mock.Anything, "a@x.io", svcRepo, p, "fixed", mock.Anything).Return(nil).Once()
	svc := NewService(st, mailer, "https://ci", "secret")
	assert.NoError(t, svc.PipelineFinished(t.Context(), svcRepo, p))
}

func TestPipelineFinished_StatusChangeDocsOnlyPassNoEmail(t *testing.T) {
	st := smocks.NewMockStore(t)
	mailer := nmocks.NewMockService(t)
	// workflow "a" still fails, the new run only contains passing workflow "b"
	st.EXPECT().WatchListByKey(int64(1), model.EventPush, "main").Return([]*model.PipelineWatch{emailWatch(1, 1, model.NotificationEventStatusChange)}, nil)
	st.EXPECT().WatchStateList(int64(1), model.EventPush, "main").Return([]*model.WatchWorkflowState{state("a", true, 1)}, nil)
	st.EXPECT().WatchStateReplace(int64(1), model.EventPush, "main", mock.Anything).Return(nil)
	svc := NewService(st, mailer, "https://ci", "secret")
	assert.NoError(t, svc.PipelineFinished(t.Context(), svcRepo, finishedPipeline(2, model.StatusSuccess, wf("b", model.StatusSuccess))))
}

func TestPipelineFinished_SkipsEmailWithoutAddressOrAccess(t *testing.T) {
	st := smocks.NewMockStore(t)
	mailer := nmocks.NewMockService(t)
	private := &model.Repo{ID: 1, FullName: "o/r", Visibility: model.VisibilityPrivate}
	u1 := &model.User{ID: 1, Email: ""}
	u2 := &model.User{ID: 2, Email: "b@x.io"}
	st.EXPECT().WatchListByKey(int64(1), model.EventPush, "main").Return([]*model.PipelineWatch{emailWatch(1, 1, model.NotificationEventFailure), emailWatch(2, 2, model.NotificationEventFailure), emailWatch(3, 3, model.NotificationEventFailure)}, nil)
	st.EXPECT().WatchStateList(int64(1), model.EventPush, "main").Return(nil, nil)
	st.EXPECT().WatchStateReplace(int64(1), model.EventPush, "main", mock.Anything).Return(nil)
	st.EXPECT().GetUser(int64(1)).Return(u1, nil)
	st.EXPECT().GetUser(int64(2)).Return(u2, nil)
	st.EXPECT().GetUser(int64(3)).Return(nil, errors.New("gone"))
	st.EXPECT().PermFind(u2, private).Return(&model.Perm{Pull: false}, nil)
	svc := NewService(st, mailer, "https://ci", "secret")
	assert.NoError(t, svc.PipelineFinished(t.Context(), private, finishedPipeline(1, model.StatusFailure, wf("a", model.StatusFailure))))
}

func TestPipelineFinished_NilMailer(t *testing.T) {
	st := smocks.NewMockStore(t)
	st.EXPECT().WatchListByKey(int64(1), model.EventPush, "main").Return([]*model.PipelineWatch{emailWatch(1, 1, model.NotificationEventFailure)}, nil)
	st.EXPECT().WatchStateList(int64(1), model.EventPush, "main").Return(nil, nil)
	st.EXPECT().WatchStateReplace(int64(1), model.EventPush, "main", mock.Anything).Return(nil)
	svc := NewService(st, nil, "https://ci", "secret")
	assert.NoError(t, svc.PipelineFinished(t.Context(), svcRepo, finishedPipeline(1, model.StatusFailure, wf("a", model.StatusFailure))))
}

func TestPipelineFinished_RetriesReplaceOnce(t *testing.T) {
	st := smocks.NewMockStore(t)
	st.EXPECT().WatchListByKey(int64(1), model.EventPush, "main").Return([]*model.PipelineWatch{emailWatch(1, 1)}, nil)
	st.EXPECT().WatchStateList(int64(1), model.EventPush, "main").Return(nil, nil).Times(2)
	st.EXPECT().WatchStateReplace(int64(1), model.EventPush, "main", mock.Anything).Return(errors.New("unique violation")).Once()
	st.EXPECT().WatchStateReplace(int64(1), model.EventPush, "main", mock.Anything).Return(nil).Once()
	svc := NewService(st, nil, "https://ci", "secret")
	assert.NoError(t, svc.PipelineFinished(t.Context(), svcRepo, finishedPipeline(1, model.StatusFailure, wf("a", model.StatusFailure))))
}

func TestPipelineFinished_ReplaceFailsTwiceReturnsError(t *testing.T) {
	st := smocks.NewMockStore(t)
	st.EXPECT().WatchListByKey(int64(1), model.EventPush, "main").Return([]*model.PipelineWatch{emailWatch(1, 1)}, nil)
	st.EXPECT().WatchStateList(int64(1), model.EventPush, "main").Return(nil, nil).Times(2)
	st.EXPECT().WatchStateReplace(int64(1), model.EventPush, "main", mock.Anything).Return(errors.New("boom")).Times(2)
	svc := NewService(st, nil, "https://ci", "secret")
	assert.Error(t, svc.PipelineFinished(t.Context(), svcRepo, finishedPipeline(1, model.StatusFailure, wf("a", model.StatusFailure))))
}

func TestSeed_KeepsStateWhenAnotherWatcherExists(t *testing.T) {
	st := smocks.NewMockStore(t)
	st.EXPECT().WatchListByKey(int64(1), model.EventPush, "main").Return([]*model.PipelineWatch{emailWatch(1, 1), emailWatch(2, 2)}, nil)
	st.EXPECT().WatchStateList(int64(1), model.EventPush, "main").Return([]*model.WatchWorkflowState{state("a", false, 1)}, nil)
	assert.NoError(t, NewService(st, nil, "", "").Seed(t.Context(), testKey))
}

func TestSeed_ReseedsWhenThereIsNoOtherWatcher(t *testing.T) {
	st := smocks.NewMockStore(t)
	// stale state left from before the key was last unwatched
	st.EXPECT().WatchListByKey(int64(1), model.EventPush, "main").Return([]*model.PipelineWatch{emailWatch(1, 1)}, nil)
	st.EXPECT().WatchStateList(int64(1), model.EventPush, "main").Return([]*model.WatchWorkflowState{state("a", true, 1)}, nil)
	st.EXPECT().GetRepo(int64(1)).Return(svcRepo, nil)
	st.EXPECT().GetPipelineList(svcRepo, mock.Anything, mock.Anything).Return([]*model.Pipeline{finishedPipeline(5, model.StatusSuccess)}, nil)
	st.EXPECT().WorkflowGetTree(mock.Anything).Return([]*model.Workflow{wf("a", model.StatusSuccess)}, nil)
	st.EXPECT().WatchStateReplace(int64(1), model.EventPush, "main", mock.MatchedBy(func(s []*model.WatchWorkflowState) bool {
		return len(s) == 1 && !s[0].Failing && s[0].PipelineNumber == 5
	})).Return(nil)
	assert.NoError(t, NewService(st, nil, "", "").Seed(t.Context(), testKey))
}

func TestSeed_ReseedClearsStaleStateWithoutRuns(t *testing.T) {
	st := smocks.NewMockStore(t)
	st.EXPECT().WatchListByKey(int64(1), model.EventPush, "main").Return([]*model.PipelineWatch{emailWatch(1, 1)}, nil)
	st.EXPECT().WatchStateList(int64(1), model.EventPush, "main").Return([]*model.WatchWorkflowState{state("a", true, 1)}, nil)
	st.EXPECT().GetRepo(int64(1)).Return(svcRepo, nil)
	st.EXPECT().GetPipelineList(svcRepo, mock.Anything, mock.Anything).Return(nil, nil)
	st.EXPECT().WatchStateReplace(int64(1), model.EventPush, "main", mock.MatchedBy(func(s []*model.WatchWorkflowState) bool {
		return len(s) == 0
	})).Return(nil)
	assert.NoError(t, NewService(st, nil, "", "").Seed(t.Context(), testKey))
}

func TestSeed_CronMatchesSender(t *testing.T) {
	st := smocks.NewMockStore(t)
	key := model.WatchKey{RepoID: 1, Event: model.EventCron, Target: "nightly"}
	cron := func(number int64, name string, status model.StatusValue) *model.Pipeline {
		return &model.Pipeline{ID: number * 100, Number: number, Status: status, Event: model.EventCron, Branch: "main", Sender: name}
	}
	st.EXPECT().WatchListByKey(int64(1), model.EventCron, "nightly").Return([]*model.PipelineWatch{emailWatch(1, 1)}, nil)
	st.EXPECT().WatchStateList(int64(1), model.EventCron, "nightly").Return(nil, nil)
	st.EXPECT().GetRepo(int64(1)).Return(svcRepo, nil)
	st.EXPECT().GetPipelineList(svcRepo, mock.MatchedBy(func(o *model.ListOptions) bool {
		return o.PerPage == seedLimit*seedScanFactor
	}), mock.MatchedBy(func(f *model.PipelineFilter) bool {
		return f.Branch == "" && len(f.Events) == 1 && f.Events[0] == model.EventCron
	})).Return([]*model.Pipeline{cron(3, "weekly", model.StatusSuccess), cron(2, "nightly", model.StatusFailure), cron(1, "weekly", model.StatusFailure)}, nil)
	st.EXPECT().WorkflowGetTree(mock.MatchedBy(func(p *model.Pipeline) bool { return p.Number == 2 })).
		Return([]*model.Workflow{wf("a", model.StatusFailure)}, nil).Once()
	st.EXPECT().WatchStateReplace(int64(1), model.EventCron, "nightly", mock.MatchedBy(func(s []*model.WatchWorkflowState) bool {
		return len(s) == 1 && s[0].Failing && s[0].PipelineNumber == 2
	})).Return(nil)
	assert.NoError(t, NewService(st, nil, "", "").Seed(t.Context(), key))
}

func TestSeed_DeploymentMatchesDeployToAndCapsMatches(t *testing.T) {
	st := smocks.NewMockStore(t)
	key := model.WatchKey{RepoID: 1, Event: model.EventDeploy, Target: "prod"}
	// newest first: 70 runs, every seventh one deploys to staging
	var pipelines []*model.Pipeline
	for n := int64(70); n > 0; n-- {
		env := "prod"
		if n%7 == 0 {
			env = "staging"
		}
		pipelines = append(pipelines, &model.Pipeline{ID: n * 100, Number: n, Status: model.StatusSuccess, Event: model.EventDeploy, DeployTo: env})
	}
	st.EXPECT().WatchListByKey(int64(1), model.EventDeploy, "prod").Return([]*model.PipelineWatch{emailWatch(1, 1)}, nil)
	st.EXPECT().WatchStateList(int64(1), model.EventDeploy, "prod").Return(nil, nil)
	st.EXPECT().GetRepo(int64(1)).Return(svcRepo, nil)
	st.EXPECT().GetPipelineList(svcRepo, mock.Anything, mock.MatchedBy(func(f *model.PipelineFilter) bool {
		return f.Branch == "" && len(f.Events) == 1 && f.Events[0] == model.EventDeploy
	})).Return(pipelines, nil)
	var applied []int64
	st.EXPECT().WorkflowGetTree(mock.Anything).RunAndReturn(func(p *model.Pipeline) ([]*model.Workflow, error) {
		assert.Equal(t, "prod", p.DeployTo)
		applied = append(applied, p.Number)
		return []*model.Workflow{wf("deploy", model.StatusSuccess)}, nil
	})
	st.EXPECT().WatchStateReplace(int64(1), model.EventDeploy, "prod", mock.MatchedBy(func(s []*model.WatchWorkflowState) bool {
		return len(s) == 1 && s[0].PipelineNumber == 69
	})).Return(nil)
	assert.NoError(t, NewService(st, nil, "", "").Seed(t.Context(), key))
	assert.Len(t, applied, seedLimit, "only the newest matches are applied")
	// the oldest applied run is the 50th newest prod deployment, applied first
	assert.Equal(t, int64(12), applied[0])
	assert.Equal(t, int64(69), applied[len(applied)-1])
}

func TestSeed_AppliesOldestFirst(t *testing.T) {
	st := smocks.NewMockStore(t)
	newest := finishedPipeline(2, model.StatusSuccess)
	oldest := finishedPipeline(1, model.StatusFailure)
	killed := finishedPipeline(3, model.StatusKilled)
	st.EXPECT().WatchListByKey(int64(1), model.EventPush, "main").Return([]*model.PipelineWatch{emailWatch(1, 1)}, nil)
	st.EXPECT().WatchStateList(int64(1), model.EventPush, "main").Return(nil, nil)
	st.EXPECT().GetRepo(int64(1)).Return(svcRepo, nil)
	st.EXPECT().GetPipelineList(svcRepo, mock.Anything, mock.MatchedBy(func(f *model.PipelineFilter) bool {
		return f.Branch == "main" && len(f.Events) == 1 && f.Events[0] == model.EventPush
	})).Return([]*model.Pipeline{killed, newest, oldest}, nil)
	st.EXPECT().WorkflowGetTree(mock.Anything).RunAndReturn(func(p *model.Pipeline) ([]*model.Workflow, error) {
		if p.Number == 1 {
			return []*model.Workflow{wf("a", model.StatusFailure)}, nil
		}
		return []*model.Workflow{wf("a", model.StatusSuccess)}, nil
	})
	st.EXPECT().WatchStateReplace(int64(1), model.EventPush, "main", mock.MatchedBy(func(s []*model.WatchWorkflowState) bool {
		return len(s) == 1 && !s[0].Failing && s[0].PipelineNumber == 2
	})).Return(nil)
	assert.NoError(t, NewService(st, nil, "", "").Seed(t.Context(), testKey))
}

func TestUnsubscribeURL(t *testing.T) {
	svc := NewService(smocks.NewMockStore(t), nil, "https://ci/", "secret")
	u := svc.UnsubscribeURL(&model.PipelineWatch{ID: 5, UserID: 7})
	assert.Contains(t, u, "https://ci/api/v1/notifications/unsubscribe?token=")
	assert.Empty(t, NewService(smocks.NewMockStore(t), nil, "https://ci", "").UnsubscribeURL(&model.PipelineWatch{ID: 5}))
}
