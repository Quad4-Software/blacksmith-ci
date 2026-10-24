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

package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"codefloe.com/crowci/crow/v6/pipeline/rpc"
	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/logging"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pubsub"
	"codefloe.com/crowci/crow/v6/server/queue"
	queue_mocks "codefloe.com/crowci/crow/v6/server/queue/mocks"
	watch_mocks "codefloe.com/crowci/crow/v6/server/services/watch/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

// Done on the last workflow of a pipeline hands the finished pipeline to the
// watch service exactly once, with its final status.
func TestRPC_Done_LastWorkflowEvaluatesWatchesOnce(t *testing.T) {
	// not parallel: swaps the global watch service
	watchSvc := watch_mocks.NewMockService(t)
	orig := server.Config.Services.WatchService
	server.Config.Services.WatchService = watchSvc
	t.Cleanup(func() { server.Config.Services.WatchService = orig })

	const (
		agentID    int64 = 1
		workflowID int64 = 42
		pipelineID int64 = 8
		repoID     int64 = 7
	)
	agent := &model.Agent{ID: agentID, LastWork: time.Now().Unix()}
	repo := &model.Repo{ID: repoID, UserID: 3, FullName: "o/r"}
	pipeline := &model.Pipeline{ID: pipelineID, RepoID: repoID, Number: 5, Status: model.StatusRunning, Event: model.EventPush, Branch: "main"}
	step := &model.Step{ID: 9, PipelineID: pipelineID, State: model.StatusFailure, Started: 10, Finished: 20}
	workflow := &model.Workflow{ID: workflowID, PipelineID: pipelineID, Name: "build", State: model.StatusRunning, AgentID: agentID}
	other := &model.Workflow{ID: 43, PipelineID: pipelineID, Name: "lint", State: model.StatusSuccess}

	st := store_mocks.NewMockStore(t)
	st.EXPECT().WorkflowLoad(workflowID).Return(workflow, nil)
	st.EXPECT().StepListFromWorkflowFind(mock.Anything).Return([]*model.Step{step}, nil)
	st.EXPECT().GetPipeline(pipelineID).Return(pipeline, nil)
	st.EXPECT().GetRepo(repoID).Return(repo, nil)
	st.EXPECT().AgentFind(agentID).Return(agent, nil)
	st.EXPECT().TaskGetAgentID("42", agentID).Return(true, nil)
	st.EXPECT().WorkflowUpdate(mock.Anything).Return(nil)
	st.EXPECT().WorkflowGetTree(mock.Anything).Return([]*model.Workflow{
		{ID: workflowID, PipelineID: pipelineID, Name: "build", State: model.StatusFailure},
		other,
	}, nil)
	st.EXPECT().UpdatePipeline(mock.Anything).Return(nil)
	// the forge status update stops early without a repo owner
	st.EXPECT().GetUser(repo.UserID).Return(nil, errors.New("no owner"))
	// nothing depends on the failed workflow
	st.EXPECT().TaskListPending().Return(nil, nil)

	q := queue_mocks.NewMockQueue(t)
	q.EXPECT().Error(mock.Anything, "42", mock.Anything).Return(nil)
	q.EXPECT().Info(mock.Anything).Return(queue.InfoT{})

	finished := make(chan *model.Pipeline, 2)
	watchSvc.EXPECT().PipelineFinished(mock.Anything, repo, mock.Anything).RunAndReturn(func(_ context.Context, _ *model.Repo, p *model.Pipeline) error {
		finished <- p
		return nil
	}).Once()

	s := &RPC{store: st, queue: q, pubsub: pubsub.New(), logger: logging.New(), metrics: newTestGRPCMetrics()}
	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("agent_id", "1"))
	require.NoError(t, s.Done(ctx, "42", rpc.WorkflowState{Started: 10, Finished: 20, Error: "exit code 1"}))

	select {
	case p := <-finished:
		assert.Equal(t, pipelineID, p.ID)
		assert.Equal(t, model.StatusFailure, p.Status)
	case <-time.After(2 * time.Second):
		t.Fatal("watch service was not called after the last workflow finished")
	}
	// give a duplicate call the chance to show up
	select {
	case <-finished:
		t.Fatal("watch service was called more than once")
	case <-time.After(100 * time.Millisecond):
	}
}
