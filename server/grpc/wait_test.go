// Copyright 2022 crow Authors
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
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"codefloe.com/crowci/crow/v6/pipeline/rpc/proto"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/queue"
	queue_mocks "codefloe.com/crowci/crow/v6/server/queue/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func TestWaitDatabaseErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		state string
		code  codes.Code
	}{
		{"53300", codes.Unavailable},
		{"08006", codes.Unavailable},
		{"57P01", codes.Unavailable},
		{"57P02", codes.Unavailable},
		{"57P03", codes.Unavailable},
		{"28P01", codes.Unknown},
		{"42501", codes.Unknown},
	} {
		t.Run(tc.state, func(t *testing.T) {
			store := store_mocks.NewMockStore(t)
			dbErr := fmt.Errorf("load agent: %w", &pgconn.PgError{Code: tc.state})
			store.EXPECT().AgentFind(int64(1)).Return(nil, dbErr).Once()
			server := &CrowServer{peer: RPC{store: store}}
			ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("agent_id", "1"))
			_, err := server.Wait(ctx, &proto.WaitRequest{Id: "2"})
			assert.Equal(t, tc.code, status.Code(err))
		})
	}
}

func TestWaitAuthorizationAndQueueErrors(t *testing.T) {
	for _, stage := range []string{"workflow", "pipeline", "repo", "queue", "cancel", "complete", "permission"} {
		t.Run(stage, func(t *testing.T) {
			store := store_mocks.NewMockStore(t)
			q := queue_mocks.NewMockQueue(t)
			server := &CrowServer{peer: RPC{store: store, queue: q}}
			ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("agent_id", "1"))
			dbErr := &pgconn.PgError{Code: "53300"}
			stageError := func(name string) error {
				if stage == name {
					return dbErr
				}
				return nil
			}
			agent := &model.Agent{ID: 1}
			if stage == "permission" {
				agent.OrgIDs = []int64{99}
			}
			store.EXPECT().AgentFind(int64(1)).Return(agent, nil).Once()
			store.EXPECT().WorkflowLoad(int64(2)).Return(&model.Workflow{PipelineID: 3}, stageError("workflow")).Once()
			if stage != "workflow" {
				store.EXPECT().GetPipeline(int64(3)).Return(&model.Pipeline{RepoID: 4}, stageError("pipeline")).Once()
			}
			if stage != "workflow" && stage != "pipeline" {
				store.EXPECT().GetRepo(int64(4)).Return(&model.Repo{ID: 4}, stageError("repo")).Once()
			}
			expected := codes.Unavailable
			switch stage {
			case "queue":
				q.EXPECT().Wait(ctx, "2").Return(dbErr).Once()
			case "cancel":
				q.EXPECT().Wait(ctx, "2").Return(queue.ErrCancel).Once()
				expected = codes.Unknown
			case "complete":
				q.EXPECT().Wait(ctx, "2").Return(nil).Once()
				expected = codes.OK
			case "permission":
				expected = codes.Unknown
			}
			_, err := server.Wait(ctx, &proto.WaitRequest{Id: "2"})
			assert.Equal(t, expected, status.Code(err))
			if stage == "cancel" {
				assert.ErrorIs(t, err, queue.ErrCancel)
			}
		})
	}
}
