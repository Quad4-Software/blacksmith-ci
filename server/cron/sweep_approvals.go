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

package cron

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/pipeline"
	"codefloe.com/crowci/crow/v6/server/queue"
	"codefloe.com/crowci/crow/v6/server/store"
)

// SweepExpiredApprovals scans for workflow approval gates whose deadline has
// passed and transitions them to the Expired state via pipeline.ExpireWorkflow.
// Safe to call repeatedly from a goroutine; the index on
// (approval_state, approval_deadline) makes each call cheap when there are no
// expired gates to process.
func SweepExpiredApprovals(ctx context.Context, s store.Store, q queue.Queue) {
	now := time.Now().Unix()
	wfs, err := s.WorkflowsAwaitingApprovalBefore(now)
	if err != nil {
		log.Error().Err(err).Msg("sweep approvals: query failed")
		return
	}
	for _, wf := range wfs {
		if err := pipeline.ExpireWorkflow(ctx, s, q, wf.ID); err != nil {
			log.Error().Err(err).Int64("workflow_id", wf.ID).Msg("sweep approvals: expire failed")
		}
	}
}

// RunSweeper runs SweepExpiredApprovals on the given interval until ctx is
// done. The first sweep fires immediately so a server that has been down
// across an approval deadline does not have to wait one full interval before
// expiring stale gates.
func RunSweeper(ctx context.Context, s store.Store, q queue.Queue, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		SweepExpiredApprovals(ctx, s, q)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
