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
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"

	"codefloe.com/crowci/crow/v6/pipeline/rpc/proto"
)

// burstFactor is the multiplier applied to RateLimit to set the token-bucket burst capacity.
const burstFactor = 2

// ShipperClient is the slim interface the shipper needs from the gRPC client.
// In production, T15's wrapper around proto.CrowClient's StreamProcLogs stream;
// in tests, a fake.
type ShipperClient interface {
	SendBatch(ctx context.Context, batch *proto.ProcLogBatch) error
}

// ShipperOpts configures the Shipper.
type ShipperOpts struct {
	BatchInterval time.Duration
	BatchSize     int
	RateLimit     int // tokens-per-second; bucket capacity = burstFactor * RateLimit
}

// Shipper drains the Sink's channel, rate-limits, buffers into the Ring, and
// sends batches via a ShipperClient. On send failure the batch is re-buffered
// for retry on the next flush. Drops above the rate limit collapse into a
// synthetic "N dropped (rate limit)" line emitted at most once per second.
type Shipper struct {
	sink    *Sink
	ring    *Ring
	client  ShipperClient
	opts    ShipperOpts
	limiter *rate.Limiter

	markerLastEmitted time.Time
	pendingDropped    int
}

const (
	defaultBatchInterval = time.Second
	defaultBatchSize     = 100
	defaultRateLimit     = 100
)

// NewShipper constructs a Shipper with sane defaults for zero-value opts fields.
func NewShipper(sink *Sink, ring *Ring, client ShipperClient, opts ShipperOpts) *Shipper {
	if opts.BatchInterval <= 0 {
		opts.BatchInterval = defaultBatchInterval
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = defaultBatchSize
	}
	if opts.RateLimit <= 0 {
		opts.RateLimit = defaultRateLimit
	}
	return &Shipper{
		sink:    sink,
		ring:    ring,
		client:  client,
		opts:    opts,
		limiter: rate.NewLimiter(rate.Limit(opts.RateLimit), burstFactor*opts.RateLimit),
	}
}

// Run starts the ship loop. It returns when ctx is canceled or the sink
// channel is closed.
func (s *Shipper) Run(ctx context.Context) {
	tick := time.NewTicker(s.opts.BatchInterval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-s.sink.Out():
			if !ok {
				return
			}
			s.intake(line)
			if s.ring.Len() >= s.opts.BatchSize {
				s.flush(ctx)
			}
		case <-tick.C:
			s.flush(ctx)
		}
	}
}

// intake applies the rate limiter to a single line before pushing it into the ring.
func (s *Shipper) intake(line SinkLine) {
	if !s.limiter.Allow() {
		s.pendingDropped++
		return
	}
	s.ring.Push(line)

	if dropped := s.sink.DropCount(); dropped > 0 {
		s.pendingDropped += int(dropped)
	}
}

// flush drains up to BatchSize entries from the ring and ships them. On
// failure the lines are re-buffered so they will be retried on the next tick.
func (s *Shipper) flush(ctx context.Context) {
	if s.pendingDropped > 0 && time.Since(s.markerLastEmitted) >= time.Second {
		s.ring.Push(SinkLine{
			Time:  time.Now().UnixNano(),
			Level: "warn",
			Line:  fmt.Sprintf("%d lines dropped (rate limit)", s.pendingDropped),
		})
		s.pendingDropped = 0
		s.markerLastEmitted = time.Now()
	}

	if s.ring.Len() == 0 {
		return
	}
	lines := s.ring.DrainN(s.opts.BatchSize)
	batch := &proto.ProcLogBatch{Entries: make([]*proto.ProcLogEntry, 0, len(lines))}
	for _, l := range lines {
		batch.Entries = append(batch.Entries, &proto.ProcLogEntry{
			Time:  l.Time,
			Level: l.Level,
			Line:  l.Line,
		})
	}
	if err := s.client.SendBatch(ctx, batch); err != nil {
		log.Debug().Err(err).Int("entries", len(lines)).Msg("proclog batch send failed; re-buffering for retry")
		for i := range lines {
			s.ring.Push(lines[i])
		}
	}
}
