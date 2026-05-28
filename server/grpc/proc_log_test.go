// Copyright 2024 Crow Authors
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
	"io"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"codefloe.com/crowci/crow/v5/pipeline/rpc/proto"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/proclog"
	storeMocks "codefloe.com/crowci/crow/v5/server/store/mocks"
)

// fakeStreamProcLogsServer implements proto.Crow_StreamProcLogsServer.
type fakeStreamProcLogsServer struct {
	grpc.ServerStream
	batches []*proto.ProcLogBatch
	pos     int
	ctx     context.Context
	ack     *proto.ProcLogAck
}

func (f *fakeStreamProcLogsServer) Context() context.Context { return f.ctx }

func (f *fakeStreamProcLogsServer) Recv() (*proto.ProcLogBatch, error) {
	if f.pos >= len(f.batches) {
		return nil, io.EOF
	}
	b := f.batches[f.pos]
	f.pos++
	return b, nil
}

func (f *fakeStreamProcLogsServer) SendAndClose(a *proto.ProcLogAck) error {
	f.ack = a
	return nil
}

func TestStreamProcLogsForwardsToReceiver(t *testing.T) {
	t.Parallel()

	storeMock := storeMocks.NewMockStore(t)

	// getAgentFromContext calls AgentFind with the ID parsed from metadata.
	agentID := int64(7)
	storeAgent := &model.Agent{ID: agentID}
	storeMock.On("AgentFind", agentID).Return(storeAgent, nil)

	var captured []*model.ProcLogEntry
	storeMock.On("ProcLogInsert", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			entries, ok := args.Get(1).([]*model.ProcLogEntry)
			require.True(t, ok)
			captured = append(captured, entries...)
		}).Return(nil)

	receiver := proclog.NewReceiver(storeMock)

	rpcServer := &RPC{
		store:           storeMock,
		procLogReceiver: receiver,
		metrics:         newTestGRPCMetrics(),
	}

	stream := &fakeStreamProcLogsServer{
		batches: []*proto.ProcLogBatch{
			{Entries: []*proto.ProcLogEntry{{Time: 1, Level: "info", Line: "a"}}},
			{Entries: []*proto.ProcLogEntry{{Time: 2, Level: "warn", Line: "b"}}},
		},
		ctx: metadata.NewIncomingContext(
			t.Context(),
			metadata.Pairs("agent_id", "7"),
		),
	}

	require.NoError(t, rpcServer.StreamProcLogs(stream))

	require.NotNil(t, stream.ack)
	require.Equal(t, int64(2), stream.ack.Received)

	require.Len(t, captured, 2)
	require.Equal(t, "a", captured[0].Line)
	require.Equal(t, "b", captured[1].Line)
	require.Equal(t, model.ProcLogSourceTypeAgent, captured[0].SourceType)
	require.Equal(t, agentID, captured[0].SourceID)
}

func TestStreamProcLogsDisabledWhenReceiverNil(t *testing.T) {
	t.Parallel()

	rpcServer := &RPC{
		metrics: newTestGRPCMetrics(),
	}

	stream := &fakeStreamProcLogsServer{
		ctx: t.Context(),
	}

	err := rpcServer.StreamProcLogs(stream)
	require.Error(t, err)
	require.Contains(t, err.Error(), "process log ingest is disabled")
}
