// Copyright 2026 Crow CI Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"codefloe.com/crowci/crow/v6/pipeline/rpc"
	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pubsub"
	"codefloe.com/crowci/crow/v6/server/queue"
	queue_mocks "codefloe.com/crowci/crow/v6/server/queue/mocks"
	"codefloe.com/crowci/crow/v6/server/services/ciquota"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func TestQuotaClass(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		want     string
	}{
		{name: "linux architecture", platform: "linux/amd64", want: "linux"},
		{name: "windows architecture", platform: "windows/amd64", want: "windows"},
		{name: "Darwin is macOS", platform: "darwin/arm64", want: "macos"},
		{name: "missing platform defaults to Linux", want: "linux"},
		{name: "unknown platform defaults to Linux", platform: "freebsd/amd64", want: "linux"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, quotaClass(test.platform))
		})
	}
}

type quotaCheckerFunc func(context.Context, string, string, string) (*ciquota.Decision, error)

type quotaTestReporter struct {
	t      *testing.T
	check  quotaCheckerFunc
	report func(ciquota.Usage) error
}

func (f quotaTestReporter) Check(ctx context.Context, repo, class, runnerID string) (*ciquota.Decision, error) {
	return f.check(ctx, repo, class, runnerID)
}

func (f quotaTestReporter) Report(_ context.Context, usage ciquota.Usage) error {
	if f.report != nil {
		return f.report(usage)
	}
	f.t.Error("unstarted workflows must not report usage")
	return nil
}

func setQuotaChecker(t *testing.T, checker ciquota.Reporter) {
	t.Helper()
	previous := server.Config.Services.CIQuotaReporter
	server.Config.Services.CIQuotaReporter = checker
	t.Cleanup(func() { server.Config.Services.CIQuotaReporter = previous })
}

func TestCheckCIQuotaRetry(t *testing.T) {
	for _, tc := range []struct {
		name    string
		recover bool
		cancel  bool
	}{
		{name: "temporary failure", recover: true},
		{name: "persistent failure"},
		{name: "canceled polling", cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := new(store_mocks.MockStore)
			q := new(queue_mocks.MockQueue)
			s := RPC{store: db, queue: q}
			db.On("WorkflowLoad", int64(42)).Return(&model.Workflow{Platform: "windows/amd64"}, nil)
			db.On("GetRepo", int64(7)).Return(&model.Repo{FullName: "org/repo"}, nil)
			q.On("Extend", mock.Anything, int64(1), "42").Return(nil)
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			attempts := 0
			setQuotaChecker(t, quotaTestReporter{t: t, check: func(_ context.Context, repo, class, runnerID string) (*ciquota.Decision, error) {
				attempts++
				assert.Equal(t, "org/repo", repo)
				assert.Equal(t, "public-agent", runnerID)
				assert.Equal(t, "windows", class)
				if tc.cancel {
					cancel(nil)
				}
				if tc.recover && attempts == 2 {
					return &ciquota.Decision{Allowed: true}, nil
				}
				return nil, errors.New("503 Service Unavailable")
			}})
			err := s.checkCIQuota(ctx, &model.Agent{ID: 1, Name: "public-agent"}, &model.Task{ID: "42", RepoID: 7})
			switch {
			case tc.cancel:
				require.ErrorIs(t, err, context.Canceled)
				assert.Equal(t, 1, attempts)
			case tc.recover:
				require.NoError(t, err)
				assert.Equal(t, 2, attempts)
			default:
				require.ErrorContains(t, err, "workflow not started")
				assert.Equal(t, 3, attempts)
			}
			db.AssertExpectations(t)
			q.AssertExpectations(t)
		})
	}
}

// Exercise the actual dispatch and completion paths: macOS must never be
// returned to an agent, but independent Linux work must remain runnable.
func TestNextCIQuotaDenial(t *testing.T) {
	for _, tc := range []struct {
		name         string
		allowLinux   bool
		checkFailure bool
	}{
		{name: "class cap leaves independent Linux runnable", allowLinux: true},
		{name: "total cap refuses every workflow"},
		{name: "unavailable service never dispatches work", checkFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := new(store_mocks.MockStore)
			q := new(queue_mocks.MockQueue)
			agent := &model.Agent{ID: 1, Name: "public-agent", LastWork: time.Now().Unix()}
			mac := &model.Workflow{ID: 42, PipelineID: 8, Name: "mac", Platform: "darwin/arm64", State: model.StatusPending}
			linux := &model.Workflow{ID: 43, PipelineID: 8, Name: "linux", Platform: "linux/amd64", State: model.StatusPending}
			pl := &model.Pipeline{ID: 8, RepoID: 7, Status: model.StatusPending}
			repo := &model.Repo{ID: 7, FullName: "org/repo"}
			db.On("AgentFind", int64(1)).Return(agent, nil)
			db.On("TaskGetAgentID", mock.Anything, int64(1)).Return(true, nil)
			db.On("WorkflowLoad", int64(42)).Return(mac, nil)
			db.On("WorkflowLoad", int64(43)).Return(linux, nil)
			db.On("GetRepo", int64(7)).Return(repo, nil)
			db.On("GetPipeline", int64(8)).Return(pl, nil)
			db.On("StepListFromWorkflowFind", mock.Anything).Return([]*model.Step{}, nil)
			db.On("WorkflowUpdate", mock.Anything).Run(func(args mock.Arguments) {
				updated, ok := args.Get(0).(*model.Workflow)
				require.True(t, ok)
				if updated.ID == mac.ID {
					*mac = *updated
				} else {
					*linux = *updated
				}
			}).Return(nil)
			db.On("WorkflowGetTree", mock.Anything).Return([]*model.Workflow{mac, linux}, nil)
			db.On("TaskListPending").Return([]*model.Task{}, nil)
			// Forge status delivery is outside this scheduling regression.
			db.On("GetUser", int64(0)).Return((*model.User)(nil), errors.New("no forge user"))
			q.On("Extend", mock.Anything, int64(1), mock.Anything).Return(nil)
			q.On("Error", mock.Anything, "42", mock.Anything).Return(nil).Once()
			q.On("Info", mock.Anything).Return(queue.InfoT{})
			for _, id := range []string{"42", "43"} {
				data, err := json.Marshal(&rpc.Workflow{ID: id})
				require.NoError(t, err)
				q.On("Poll", mock.Anything, int64(1), mock.Anything).Return(&model.Task{ID: id, RepoID: 7, Data: data, RunOn: []string{"success"}}, nil).Once()
			}
			if !tc.allowLinux {
				q.On("Error", mock.Anything, "43", mock.Anything).Return(nil).Once()
				db.On("UpdatePipeline", mock.Anything).Run(func(args mock.Arguments) {
					updated, ok := args.Get(0).(*model.Pipeline)
					require.True(t, ok)
					*pl = *updated
				}).Return(nil).Once()
			}
			classes := []string{}
			setQuotaChecker(t, quotaTestReporter{t: t, check: func(_ context.Context, repo, class, runnerID string) (*ciquota.Decision, error) {
				assert.Equal(t, "org/repo", repo)
				assert.Equal(t, "public-agent", runnerID)
				classes = append(classes, class)
				if tc.checkFailure {
					return nil, errors.New("quota service unavailable")
				}
				return &ciquota.Decision{Allowed: tc.allowLinux && class == "linux", Reason: "quota is used up"}, nil
			}})
			s := RPC{store: db, queue: q, pubsub: pubsub.New(), metrics: newTestGRPCMetrics()}
			ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("agent_id", "1"))
			// Each poll RPC finishes at most one refused workflow so a quota
			// outage cannot hold a single Next call open past proxy timeouts.
			wf, err := s.Next(ctx, rpc.Filter{Labels: map[string]string{}})
			require.NoError(t, err)
			assert.Nil(t, wf)
			if tc.checkFailure {
				assert.Equal(t, []string{"macos", "macos", "macos"}, classes)
			} else {
				assert.Equal(t, []string{"macos"}, classes)
			}
			wf, err = s.Next(ctx, rpc.Filter{Labels: map[string]string{}})
			require.NoError(t, err)
			if tc.checkFailure {
				assert.Equal(t, []string{"macos", "macos", "macos", "linux", "linux", "linux"}, classes)
			} else {
				assert.Equal(t, []string{"macos", "linux"}, classes)
			}
			assert.Equal(t, model.StatusFailure, mac.State)
			if tc.checkFailure {
				assert.Contains(t, mac.Error, "workflow not started")
			} else {
				assert.Contains(t, mac.Error, "quota is used up")
			}
			assert.Zero(t, mac.Started)
			if tc.allowLinux {
				require.NotNil(t, wf)
				assert.Equal(t, "43", wf.ID)
				assert.Equal(t, model.StatusPending, linux.State)
			} else {
				assert.Nil(t, wf)
				assert.Equal(t, model.StatusFailure, linux.State)
				assert.Equal(t, model.StatusFailure, pl.Status)
				assert.Zero(t, linux.Started)
			}
			db.AssertExpectations(t)
			q.AssertExpectations(t)
		})
	}
}

func TestNextCIQuotaDisabled(t *testing.T) {
	setQuotaChecker(t, nil)
	db := new(store_mocks.MockStore)
	q := new(queue_mocks.MockQueue)
	db.On("AgentFind", int64(1)).Return(&model.Agent{ID: 1}, nil)
	data, err := json.Marshal(&rpc.Workflow{ID: "42"})
	require.NoError(t, err)
	q.On("Poll", mock.Anything, int64(1), mock.Anything).Return(&model.Task{ID: "42", Data: data, RunOn: []string{"success"}}, nil)
	s := RPC{store: db, queue: q}
	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("agent_id", "1"))
	wf, err := s.Next(ctx, rpc.Filter{Labels: map[string]string{}})
	require.NoError(t, err)
	assert.Equal(t, "42", wf.ID)
	db.AssertExpectations(t)
	q.AssertExpectations(t)
}

// A poll canceled while the quota is being checked never had a quota
// decision, so the workflow must keep its claim instead of being failed.
func TestNextCIQuotaCanceledPollKeepsClaim(t *testing.T) {
	db := new(store_mocks.MockStore)
	q := new(queue_mocks.MockQueue)
	db.On("AgentFind", int64(1)).Return(&model.Agent{ID: 1}, nil)
	db.On("WorkflowLoad", int64(42)).Return(&model.Workflow{ID: 42, Platform: "linux/amd64"}, nil)
	db.On("GetRepo", int64(7)).Return(&model.Repo{ID: 7, FullName: "org/repo"}, nil)
	q.On("Extend", mock.Anything, int64(1), "42").Return(nil)
	data, err := json.Marshal(&rpc.Workflow{ID: "42"})
	require.NoError(t, err)
	q.On("Poll", mock.Anything, int64(1), mock.Anything).Return(&model.Task{ID: "42", RepoID: 7, Data: data, RunOn: []string{"success"}}, nil).Once()
	ctx, cancel := context.WithCancelCause(metadata.NewIncomingContext(t.Context(), metadata.Pairs("agent_id", "1")))
	defer cancel(nil)
	setQuotaChecker(t, quotaTestReporter{t: t, check: func(context.Context, string, string, string) (*ciquota.Decision, error) {
		cancel(nil)
		return nil, errors.New("connection reset")
	}})
	s := RPC{store: db, queue: q}
	wf, err := s.Next(ctx, rpc.Filter{Labels: map[string]string{}})
	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, wf)
	db.AssertNotCalled(t, "WorkflowUpdate", mock.Anything)
	q.AssertNotCalled(t, "Error", mock.Anything, mock.Anything, mock.Anything)
	db.AssertExpectations(t)
	q.AssertExpectations(t)
}

// The whole retry loop must fit inside one poll budget so an unreachable quota
// service cannot hold the agent's Next RPC open past reverse-proxy timeouts.
func TestCheckCIQuotaBoundsTotalTime(t *testing.T) {
	db := new(store_mocks.MockStore)
	q := new(queue_mocks.MockQueue)
	s := RPC{store: db, queue: q}
	db.On("WorkflowLoad", int64(42)).Return(&model.Workflow{Platform: "linux/amd64"}, nil)
	db.On("GetRepo", int64(7)).Return(&model.Repo{FullName: "org/repo"}, nil)
	q.On("Extend", mock.Anything, int64(1), "42").Return(nil)
	started := time.Now()
	setQuotaChecker(t, quotaTestReporter{t: t, check: func(ctx context.Context, _, _, _ string) (*ciquota.Decision, error) {
		deadline, ok := ctx.Deadline()
		require.True(t, ok, "check context must carry a deadline")
		assert.WithinDuration(t, started.Add(ciQuotaCheckTimeout), deadline, 2*time.Second)
		assert.Less(t, ciQuotaCheckTimeout, 30*time.Second, "budget must stay below the queue poll timeout")
		return &ciquota.Decision{Allowed: true}, nil
	}})
	require.NoError(t, s.checkCIQuota(t.Context(), &model.Agent{ID: 1, Name: "public-agent"}, &model.Task{ID: "42", RepoID: 7}))
}

// A reporter that returns neither a decision nor an error is a protocol bug,
// never a permission to run and never a crash of the poll RPC.
func TestCheckCIQuotaEmptyDecision(t *testing.T) {
	db := new(store_mocks.MockStore)
	q := new(queue_mocks.MockQueue)
	s := RPC{store: db, queue: q}
	db.On("WorkflowLoad", int64(42)).Return(&model.Workflow{Platform: "linux/amd64"}, nil)
	db.On("GetRepo", int64(7)).Return(&model.Repo{FullName: "org/repo"}, nil)
	q.On("Extend", mock.Anything, int64(1), "42").Return(nil)
	setQuotaChecker(t, quotaTestReporter{t: t, check: func(context.Context, string, string, string) (*ciquota.Decision, error) {
		return nil, nil
	}})
	err := s.checkCIQuota(t.Context(), &model.Agent{ID: 1, Name: "public-agent"}, &model.Task{ID: "42", RepoID: 7})
	require.ErrorContains(t, err, "workflow not started")
	require.ErrorContains(t, err, "empty decision")
}

func TestNextCIQuotaSelfHostedNameCollision(t *testing.T) {
	setQuotaChecker(t, quotaTestReporter{t: t, check: func(context.Context, string, string, string) (*ciquota.Decision, error) {
		t.Error("organization-scoped agents must not need platform quota")
		return nil, errors.New("quota service unavailable")
	}})
	db := new(store_mocks.MockStore)
	q := new(queue_mocks.MockQueue)
	db.On("AgentFind", int64(1)).Return(&model.Agent{
		ID: 1, Name: "public-agent", OrgIDs: model.JSONSliceInt64{7},
	}, nil)
	// Next confirms an org-scoped agent may access the workflow's repo.
	db.On("WorkflowLoad", int64(42)).Return(&model.Workflow{ID: 42, PipelineID: 8}, nil)
	db.On("GetPipeline", int64(8)).Return(&model.Pipeline{ID: 8, RepoID: 3}, nil)
	db.On("GetRepo", int64(3)).Return(&model.Repo{ID: 3, OrgID: 7, FullName: "org/repo"}, nil)
	data, err := json.Marshal(&rpc.Workflow{ID: "42"})
	require.NoError(t, err)
	q.On("Poll", mock.Anything, int64(1), mock.Anything).Return(&model.Task{ID: "42", Data: data, RunOn: []string{"success"}}, nil)
	s := RPC{store: db, queue: q}
	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("agent_id", "1"))
	wf, err := s.Next(ctx, rpc.Filter{Labels: map[string]string{}})
	require.NoError(t, err)
	require.NotNil(t, wf)
	assert.Equal(t, "42", wf.ID)
	db.AssertExpectations(t)
	q.AssertExpectations(t)
}

func TestReportCIQuotaUsageAgentIdentity(t *testing.T) {
	for _, tc := range []struct {
		name       string
		orgIDs     model.JSONSliceInt64
		started    int64
		wantReport bool
	}{
		{name: "global agent", started: 1_700_000_000, wantReport: true},
		{name: "organization agent with same name", orgIDs: model.JSONSliceInt64{7}, started: 1_700_000_000},
		{name: "unstarted platform workflow"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reports := make(chan ciquota.Usage, 1)
			setQuotaChecker(t, quotaTestReporter{t: t, report: func(usage ciquota.Usage) error {
				reports <- usage
				return nil
			}})
			s := RPC{}
			workflow := &model.Workflow{ID: 42, Platform: "darwin/arm64", Started: tc.started, Finished: tc.started + 90}
			s.reportCIQuotaUsage(&model.Repo{FullName: "org/repo"}, workflow, &model.Agent{
				ID: 123, Name: "public-agent", OrgIDs: tc.orgIDs,
			}, zerolog.Nop())
			if !tc.wantReport {
				assert.Empty(t, reports)
				return
			}
			select {
			case usage := <-reports:
				assert.Equal(t, ciquota.Usage{
					RunnerID: "public-agent", Repository: "org/repo", ExternalRef: "crow:workflow:42",
					Started: time.Unix(tc.started, 0).UTC(), Stopped: time.Unix(tc.started+90, 0).UTC(),
					Seconds: 90, Class: "macos",
				}, usage)
			case <-time.After(5 * time.Second):
				t.Fatal("workflow usage was not reported")
			}
		})
	}
}
