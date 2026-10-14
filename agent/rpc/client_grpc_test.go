// Copyright 2023 Woodpecker Authors
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

package rpc

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"codefloe.com/crowci/crow/v6/pipeline/rpc/proto"
)

func TestWaitRetriesUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result error
	}{
		{"completion", nil},
		{"cancellation", status.Error(codes.Unknown, "queue: task canceled")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := &waitRetryClient{result: tc.result}
			c := &client{client: peer}
			err := c.Wait(t.Context(), "42")
			assert.ErrorIs(t, err, tc.result)
			assert.Equal(t, 2, peer.calls)
		})
	}
}

type waitRetryClient struct {
	proto.CrowClient
	calls  int
	result error
}

func (c *waitRetryClient) Wait(_ context.Context, _ *proto.WaitRequest, _ ...grpc.CallOption) (*proto.Empty, error) {
	c.calls++
	if c.calls == 1 {
		return nil, status.Error(codes.Unavailable, "too many connections")
	}
	return &proto.Empty{}, c.result
}

// TestProcessLogsFlushesUnderSteadyOutput guards against the flush timer being
// restarted by every incoming entry, which starved it under steady log output.
func TestProcessLogsFlushesUnderSteadyOutput(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		peer := &logRecordingClient{}
		c := &client{
			client:      peer,
			logs:        make(chan *proto.LogEntry),
			logsDrained: make(chan struct{}),
		}
		go c.processLogs(t.Context())

		// Emit an entry every half period for three periods.
		for range 6 {
			c.logs <- &proto.LogEntry{Data: []byte("line")}
			time.Sleep(maxLogFlushPeriod / 2)
		}
		synctest.Wait()

		assert.GreaterOrEqual(t, peer.calls, 2)

		close(c.logs)
		<-c.logsDrained
	})
}

type logRecordingClient struct {
	proto.CrowClient
	calls int
}

func (c *logRecordingClient) Log(_ context.Context, _ *proto.LogRequest, _ ...grpc.CallOption) (*proto.Empty, error) {
	c.calls++
	return &proto.Empty{}, nil
}
