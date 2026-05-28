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

	"codefloe.com/crowci/crow/v5/server/store"
)

const (
	defaultKeepPerSource = 5000
	defaultInterval      = 5 * time.Minute
)

type JanitorOpts struct {
	// KeepPerSource is the maximum number of rows kept per (source_type, source_id) after a run.
	KeepPerSource int
	// Interval is how often the janitor wakes up.
	Interval time.Duration
}

// Janitor periodically trims proc_log_entries so each source is bounded.
type Janitor struct {
	store store.Store
	opts  JanitorOpts
}

func NewJanitor(s store.Store, opts JanitorOpts) *Janitor {
	if opts.KeepPerSource <= 0 {
		opts.KeepPerSource = defaultKeepPerSource
	}
	if opts.Interval <= 0 {
		opts.Interval = defaultInterval
	}
	return &Janitor{store: s, opts: opts}
}

// Run blocks until ctx is canceled, performing RunOnce on each tick.
func (j *Janitor) Run(ctx context.Context) error {
	ticker := time.NewTicker(j.opts.Interval)
	defer ticker.Stop()

	// Run immediately on start so we don't wait `Interval` after boot before the first cleanup.
	if err := j.RunOnce(ctx); err != nil {
		log.Warn().Err(err).Msg("proclog janitor initial run failed")
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := j.RunOnce(ctx); err != nil {
				log.Warn().Err(err).Msg("proclog janitor run failed")
			}
		}
	}
}

// RunOnce performs a single trim pass over all known sources.
func (j *Janitor) RunOnce(ctx context.Context) error {
	keys, err := j.store.ProcLogListSources(ctx)
	if err != nil {
		return err
	}
	for _, k := range keys {
		if err := j.store.ProcLogTrim(ctx, k.SourceType, k.SourceID, j.opts.KeepPerSource); err != nil {
			log.Warn().Err(err).
				Str("source_type", k.SourceType).Int64("source_id", k.SourceID).
				Msg("proclog trim failed for source")
		}
	}
	return nil
}
