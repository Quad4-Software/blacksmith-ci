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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWatchKeyFor(t *testing.T) {
	tests := []struct {
		name       string
		pipeline   *Pipeline
		wantEvent  WebhookEvent
		wantTarget string
		wantOK     bool
	}{
		{"push uses branch", &Pipeline{Event: EventPush, Branch: "main"}, EventPush, "main", true},
		{"manual uses branch", &Pipeline{Event: EventManual, Branch: "release/2.x"}, EventManual, "release/2.x", true},
		{"cron uses cron name", &Pipeline{Event: EventCron, Sender: "nightly", Branch: "main"}, EventCron, "nightly", true},
		{"deployment uses target", &Pipeline{Event: EventDeploy, DeployTo: "production", Branch: "main"}, EventDeploy, "production", true},
		{"tag has empty target", &Pipeline{Event: EventTag, Branch: "v1"}, EventTag, "", true},
		{"release has empty target", &Pipeline{Event: EventRelease, Branch: "v1"}, EventRelease, "", true},
		{"pull_request not watchable", &Pipeline{Event: EventPull, Branch: "main"}, "", "", false},
		{"pull_request_closed not watchable", &Pipeline{Event: EventPullClosed}, "", "", false},
		{"pull_request_merged not watchable", &Pipeline{Event: EventPullMerged}, "", "", false},
		{"pull_request_edited not watchable", &Pipeline{Event: EventPullEdited}, "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, target, ok := WatchKeyFor(tt.pipeline)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantEvent, event)
			assert.Equal(t, tt.wantTarget, target)
			assert.Equal(t, tt.wantOK, IsWatchableEvent(tt.pipeline.Event))
		})
	}
}

func TestIsGenuineOutcome(t *testing.T) {
	for _, s := range []StatusValue{StatusSuccess, StatusFailure, StatusError} {
		assert.True(t, IsGenuineOutcome(s), s)
	}
	for _, s := range []StatusValue{StatusKilled, StatusDeclined, StatusSkipped, StatusBlocked, StatusPending, StatusRunning} {
		assert.False(t, IsGenuineOutcome(s), s)
	}
}
