// Copyright 2021 Woodpecker Authors
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

package proclog

import (
	"context"
	"sync"

	"codefloe.com/crowci/crow/v6/pipeline/rpc/proto"
)

// GrpcClient wraps a long-lived StreamProcLogs gRPC stream so the Shipper can call
// a simple SendBatch method. On send error the stream is closed and re-opened on
// the next call.
type GrpcClient struct {
	mu     sync.Mutex
	rpc    proto.CrowClient
	stream proto.Crow_StreamProcLogsClient
}

func NewGrpcClient(rpc proto.CrowClient) *GrpcClient {
	return &GrpcClient{rpc: rpc}
}

// SendBatch transmits one batch. If the underlying stream is closed or errored,
// it is reopened on the next call.
func (c *GrpcClient) SendBatch(ctx context.Context, batch *proto.ProcLogBatch) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stream == nil {
		s, err := c.rpc.StreamProcLogs(ctx)
		if err != nil {
			return err
		}
		c.stream = s
	}
	if err := c.stream.Send(batch); err != nil {
		// Close half-open stream so next call re-opens.
		_ = c.stream.CloseSend()
		c.stream = nil
		return err
	}
	return nil
}
