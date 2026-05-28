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
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v5/pipeline/rpc/proto"
)

type fakeClient struct {
	mu      sync.Mutex
	batches []*proto.ProcLogBatch
	failN   int
}

func (c *fakeClient) SendBatch(_ context.Context, b *proto.ProcLogBatch) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failN > 0 {
		c.failN--
		return errors.New("simulated network failure")
	}
	c.batches = append(c.batches, b)
	return nil
}

func (c *fakeClient) Received() [][]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([][]string, len(c.batches))
	for i, b := range c.batches {
		for _, e := range b.Entries {
			out[i] = append(out[i], e.Line)
		}
	}
	return out
}

func TestShipperFlushesOnTick(t *testing.T) {
	sink := NewSink(16)
	ring := NewRing(16)
	client := &fakeClient{}

	s := NewShipper(sink, ring, client, ShipperOpts{
		BatchInterval: 20 * time.Millisecond,
		BatchSize:     100,
		RateLimit:     1000,
	})

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	go s.Run(ctx)

	for _, l := range []string{"a", "b", "c"} {
		sink.out <- SinkLine{Time: 1, Level: "info", Line: l}
	}

	require.Eventually(t, func() bool {
		return len(client.Received()) > 0
	}, time.Second, 5*time.Millisecond)

	assert.Equal(t, []string{"a", "b", "c"}, client.Received()[0])
}

func TestShipperFlushesOnBatchSize(t *testing.T) {
	sink := NewSink(16)
	ring := NewRing(16)
	client := &fakeClient{}

	s := NewShipper(sink, ring, client, ShipperOpts{
		BatchInterval: time.Hour,
		BatchSize:     3,
		RateLimit:     1000,
	})

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	go s.Run(ctx)

	for _, l := range []string{"a", "b", "c", "d"} {
		sink.out <- SinkLine{Time: 1, Level: "info", Line: l}
	}

	require.Eventually(t, func() bool {
		batches := client.Received()
		return len(batches) >= 1 && len(batches[0]) == 3
	}, time.Second, 5*time.Millisecond)
}

func TestShipperRateLimitDropsAndEmitsMarker(t *testing.T) {
	sink := NewSink(1000)
	ring := NewRing(1000)
	client := &fakeClient{}

	s := NewShipper(sink, ring, client, ShipperOpts{
		BatchInterval: 20 * time.Millisecond,
		BatchSize:     1000,
		RateLimit:     10,
	})

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	go s.Run(ctx)

	for i := 0; i < 200; i++ {
		sink.out <- SinkLine{Time: 1, Level: "info", Line: "x"}
	}

	require.Eventually(t, func() bool {
		for _, b := range client.Received() {
			for _, l := range b {
				if strings.Contains(l, "dropped (rate limit)") {
					return true
				}
			}
		}
		return false
	}, 2*time.Second, 10*time.Millisecond)
}

func TestShipperBackfillsOnReconnect(t *testing.T) {
	sink := NewSink(16)
	ring := NewRing(16)
	client := &fakeClient{failN: 1}

	s := NewShipper(sink, ring, client, ShipperOpts{
		BatchInterval: 20 * time.Millisecond,
		BatchSize:     100,
		RateLimit:     1000,
	})

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	go s.Run(ctx)

	for _, l := range []string{"a", "b"} {
		sink.out <- SinkLine{Time: 1, Level: "info", Line: l}
	}

	require.Eventually(t, func() bool {
		got := client.Received()
		if len(got) == 0 {
			return false
		}
		return len(got[0]) == 2 && got[0][0] == "a" && got[0][1] == "b"
	}, 2*time.Second, 5*time.Millisecond)
}
