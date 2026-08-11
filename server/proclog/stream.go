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
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
)

// defaultPollInterval is the default poll interval matching the existing pubsub pattern.
const defaultPollInterval = 100 * time.Millisecond

// defaultMaxFlushRows is the default cap on initial replay rows.
const defaultMaxFlushRows = 5000

// defaultChannelBuffer is the default per-subscriber channel buffer size.
const defaultChannelBuffer = 256

// livePollBatch is the number of rows fetched per live-poll tick.
const livePollBatch = 500

// StreamOpts controls Stream behavior.
type StreamOpts struct {
	// PollInterval is how often we re-query the store for new entries. 100ms matches the existing pubsub pattern.
	PollInterval time.Duration
	// MaxFlushRows caps the initial replay so a subscriber opening on a long-lived source doesn't block.
	MaxFlushRows int
	// ChannelBuffer is the per-subscriber channel buffer; full channel drops oldest unsent entries.
	ChannelBuffer int
}

// Stream provides per-source live subscriptions backed by DB polling.
// Mirrors the pattern in server/logging/distributed.go (streamSession).
type Stream struct {
	store store.Store
	opts  StreamOpts
}

func NewStream(s store.Store, opts StreamOpts) *Stream {
	if opts.PollInterval <= 0 {
		opts.PollInterval = defaultPollInterval
	}
	if opts.MaxFlushRows <= 0 {
		opts.MaxFlushRows = defaultMaxFlushRows
	}
	if opts.ChannelBuffer <= 0 {
		opts.ChannelBuffer = defaultChannelBuffer
	}
	return &Stream{store: s, opts: opts}
}

// Subscribe returns a channel that receives ProcLogEntry pointers in id order.
// On open, performs an initial flush of up to MaxFlushRows recent entries, then enters
// a poll loop at PollInterval. The channel closes when ctx is canceled.
func (s *Stream) Subscribe(
	ctx context.Context, sourceType string, sourceID int64,
) (<-chan *model.ProcLogEntry, error) {
	ch := make(chan *model.ProcLogEntry, s.opts.ChannelBuffer)

	// Initial flush: most-recent MaxFlushRows entries. Starting from the oldest
	// retained row instead would make a subscriber replay the whole retention
	// window at livePollBatch rows per tick before it ever reaches "now".
	initial, err := s.store.ProcLogTailRecent(ctx, sourceType, sourceID, s.opts.MaxFlushRows)
	if err != nil {
		close(ch)
		return nil, err
	}

	var lastID int64
	for _, e := range initial {
		if e.ID > lastID {
			lastID = e.ID
		}
	}

	go s.run(ctx, sourceType, sourceID, lastID, initial, ch)
	return ch, nil
}

func (s *Stream) run(
	ctx context.Context, sourceType string, sourceID int64, lastID int64,
	initial []*model.ProcLogEntry, ch chan<- *model.ProcLogEntry,
) {
	defer close(ch)

	// Drain initial flush with blocking sends so SSE clients receive
	// the full backfill window even if briefly slow to consume.
	// (Live polls below use sendOrDrop, which is correct for steady-state.)
	for _, e := range initial {
		select {
		case ch <- e:
		case <-ctx.Done():
			return
		}
	}

	ticker := time.NewTicker(s.opts.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rows, err := s.store.ProcLogTail(ctx, sourceType, sourceID, lastID, livePollBatch)
			if err != nil {
				log.Error().Err(err).
					Str("source_type", sourceType).Int64("source_id", sourceID).
					Msg("proclog tail failed")
				continue
			}
			for _, e := range rows {
				if e.ID > lastID {
					lastID = e.ID
				}
				if !sendOrDrop(ctx, ch, e) {
					return
				}
			}
		}
	}
}

// sendOrDrop sends e on ch. If the channel is full it drops e (back-pressure). Returns false if ctx is done.
func sendOrDrop(ctx context.Context, ch chan<- *model.ProcLogEntry, e *model.ProcLogEntry) bool {
	select {
	case ch <- e:
		return true
	case <-ctx.Done():
		return false
	default:
		// Subscriber is too slow; drop. (A more elaborate impl could emit a marker.)
		return true
	}
}
