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

package model

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// setTimestamps fills in a model's created and updated times for an insert or
// update query. Pass nil for a time the model does not have.
//
// XORM filled these from its `created` and `updated` struct tags. Bun has no
// equivalent, so since the move to Bun (#877) any store that did not set them
// itself wrote zero. For agents that disabled the autoscaler's inactivity
// timeout for every agent that never connected: an agent with no creation time
// cannot be told apart from one created a moment ago, so it was never removed
// and its VM ran indefinitely.
//
// A creation time the caller already set is kept, so stores that set their own
// keep working.
func setTimestamps(query bun.Query, created, updated *int64) {
	now := time.Now().Unix()
	switch query.(type) {
	case *bun.InsertQuery:
		if created != nil && *created == 0 {
			*created = now
		}
		if updated != nil && *updated == 0 {
			*updated = now
		}
	case *bun.UpdateQuery:
		if updated != nil {
			*updated = now
		}
	}
}

var (
	_ bun.BeforeAppendModelHook = (*Agent)(nil)
	_ bun.BeforeAppendModelHook = (*Autoscaler)(nil)
	_ bun.BeforeAppendModelHook = (*ApprovedPullRequest)(nil)
	_ bun.BeforeAppendModelHook = (*Cron)(nil)
	_ bun.BeforeAppendModelHook = (*LogEntry)(nil)
	_ bun.BeforeAppendModelHook = (*Perm)(nil)
	_ bun.BeforeAppendModelHook = (*Pipeline)(nil)
	_ bun.BeforeAppendModelHook = (*Registry)(nil)
	_ bun.BeforeAppendModelHook = (*Secret)(nil)
	_ bun.BeforeAppendModelHook = (*AccessToken)(nil)
	_ bun.BeforeAppendModelHook = (*Integration)(nil)
	_ bun.BeforeAppendModelHook = (*PipelineWatch)(nil)
	_ bun.BeforeAppendModelHook = (*WatchWorkflowState)(nil)
)

func (a *Agent) BeforeAppendModel(_ context.Context, query bun.Query) error {
	setTimestamps(query, &a.Created, &a.Updated)
	return nil
}

func (a *Autoscaler) BeforeAppendModel(_ context.Context, query bun.Query) error {
	setTimestamps(query, &a.Created, &a.Updated)
	return nil
}

func (a *ApprovedPullRequest) BeforeAppendModel(_ context.Context, query bun.Query) error {
	setTimestamps(query, &a.Created, nil)
	return nil
}

func (c *Cron) BeforeAppendModel(_ context.Context, query bun.Query) error {
	setTimestamps(query, &c.Created, nil)
	return nil
}

func (l *LogEntry) BeforeAppendModel(_ context.Context, query bun.Query) error {
	setTimestamps(query, &l.Created, nil)
	return nil
}

func (p *Perm) BeforeAppendModel(_ context.Context, query bun.Query) error {
	setTimestamps(query, &p.Created, &p.Updated)
	return nil
}

func (p *Pipeline) BeforeAppendModel(_ context.Context, query bun.Query) error {
	setTimestamps(query, &p.Created, &p.Updated)
	return nil
}

func (r *Registry) BeforeAppendModel(_ context.Context, query bun.Query) error {
	setTimestamps(query, &r.CreatedAt, &r.UpdatedAt)
	return nil
}

func (s *Secret) BeforeAppendModel(_ context.Context, query bun.Query) error {
	setTimestamps(query, &s.CreatedAt, &s.UpdatedAt)
	return nil
}

func (t *AccessToken) BeforeAppendModel(_ context.Context, query bun.Query) error {
	setTimestamps(query, &t.CreatedAt, &t.UpdatedAt)
	return nil
}

func (i *Integration) BeforeAppendModel(_ context.Context, query bun.Query) error {
	setTimestamps(query, &i.CreatedAt, &i.UpdatedAt)
	return nil
}

func (w *PipelineWatch) BeforeAppendModel(_ context.Context, query bun.Query) error {
	setTimestamps(query, &w.CreatedAt, &w.UpdatedAt)
	return nil
}

func (w *WatchWorkflowState) BeforeAppendModel(_ context.Context, query bun.Query) error {
	setTimestamps(query, nil, &w.UpdatedAt)
	return nil
}
