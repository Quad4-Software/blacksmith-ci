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

import "github.com/uptrace/bun"

// PipelineWatch is a user's subscription to the outcomes of one
// (repo, event, target) key, for example pushes to a branch or a named cron.
type PipelineWatch struct {
	bun.BaseModel `bun:"table:pipeline_watches"`

	ID               int64                      `json:"id"                 bun:"id,pk,autoincrement"`
	UserID           int64                      `json:"-"                  bun:"user_id,notnull,unique:watch_key"`
	RepoID           int64                      `json:"repo_id"            bun:"repo_id,notnull,unique:watch_key"`
	Event            WebhookEvent               `json:"event"              bun:"watch_event,notnull,unique:watch_key"`
	Target           string                     `json:"target"             bun:"watch_target,notnull,unique:watch_key"`
	InApp            bool                       `json:"in_app"             bun:"in_app"`
	Email            bool                       `json:"email"              bun:"email_enabled"`
	EmailEvents      JSONSliceNotificationEvent `json:"email_events"       bun:"email_events,type:text"`
	LastReadPipeline int64                      `json:"last_read_pipeline" bun:"last_read_pipeline,notnull"`
	CreatedAt        int64                      `json:"created_at"         bun:"created_at"`
	UpdatedAt        int64                      `json:"updated_at"         bun:"updated_at"`
} //	@name	PipelineWatch

// WatchWorkflowState tracks whether a workflow is currently failing for a
// watch key, shared by every user watching that key.
type WatchWorkflowState struct {
	bun.BaseModel `bun:"table:watch_workflow_states"`

	ID             int64        `bun:"id,pk,autoincrement"`
	RepoID         int64        `bun:"repo_id,notnull,unique:watch_state_key"`
	Event          WebhookEvent `bun:"watch_event,notnull,unique:watch_state_key"`
	Target         string       `bun:"watch_target,notnull,unique:watch_state_key"`
	WorkflowName   string       `bun:"workflow_name,notnull,unique:watch_state_key"` // "" = pipeline-level (config error)
	Failing        bool         `bun:"failing"`
	PipelineID     int64        `bun:"pipeline_id,notnull"`
	PipelineNumber int64        `bun:"pipeline_number,notnull"`
	RunsSinceSeen  int64        `bun:"runs_since_seen,notnull"`
	UpdatedAt      int64        `bun:"updated_at"`
}

// WatchKey identifies what a watch follows within a repo.
type WatchKey struct {
	RepoID int64
	Event  WebhookEvent
	Target string
}

// WatchKeyFor derives the watch key of a pipeline. Ok is false for events
// that cannot be watched, such as pull request events.
func WatchKeyFor(p *Pipeline) (event WebhookEvent, target string, ok bool) {
	if !IsWatchableEvent(p.Event) {
		return "", "", false
	}
	switch p.Event {
	case EventPush, EventManual:
		target = p.Branch
	case EventCron:
		target = p.Sender
	case EventDeploy:
		target = p.DeployTo
	}
	return p.Event, target, true
}

// IsWatchableEvent reports whether pipelines of the event can be watched.
func IsWatchableEvent(e WebhookEvent) bool {
	switch e {
	case EventPush, EventManual, EventCron, EventDeploy, EventTag, EventRelease:
		return true
	default:
		return false
	}
}

// IsGenuineOutcome reports whether the status is a real result of a pipeline
// run. Only these change watch state or send email; error counts as failure.
func IsGenuineOutcome(s StatusValue) bool {
	switch s {
	case StatusSuccess, StatusFailure, StatusError:
		return true
	default:
		return false
	}
}
