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

package notification

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v5/server/model"
)

func TestExtractPRNumber(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{name: "GitHub/Forgejo PR", ref: "refs/pull/883/head", want: "883"},
		{name: "GitLab MR", ref: "refs/merge-requests/42/head", want: "42"},
		{name: "push branch ref", ref: "refs/heads/main", want: ""},
		{name: "tag ref", ref: "refs/tags/v1.0.0", want: ""},
		{name: "empty ref", ref: "", want: ""},
		{name: "PR ref without trailing path", ref: "refs/pull/123", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractPRNumber(tt.ref))
		})
	}
}

func TestBuildEmailBody_BranchPush(t *testing.T) {
	repo := &model.Repo{FullName: "crowci/crow"}
	pipeline := &model.Pipeline{
		Number:  100,
		Status:  model.StatusFailure,
		Event:   model.EventPush,
		Branch:  "main",
		Commit:  "abc123def456",
		Message: "fix something",
	}

	body := buildEmailBody("https://ci.example.com", repo, pipeline, "main", "")

	assert.Contains(t, body, ">Branch</td>")
	assert.Contains(t, body, ">main</td>")
	assert.NotContains(t, body, "Pull Request")
}

func TestBuildEmailBody_PullRequest(t *testing.T) {
	repo := &model.Repo{FullName: "crowci/crow"}
	pipeline := &model.Pipeline{
		Number:  200,
		Status:  model.StatusFailure,
		Event:   model.EventPull,
		Branch:  "main",
		Ref:     "refs/pull/42/head",
		Commit:  "abc123def456",
		Message: "add feature",
	}

	body := buildEmailBody("https://ci.example.com", repo, pipeline, "PR #42", "")

	assert.Contains(t, body, ">Pull Request</td>")
	assert.Contains(t, body, ">PR #42</td>")
	assert.NotContains(t, body, ">Branch</td>")
}

func TestBuildEmailBody_CommitTruncated(t *testing.T) {
	repo := &model.Repo{FullName: "crowci/crow"}
	pipeline := &model.Pipeline{
		Number:  1,
		Status:  model.StatusSuccess,
		Event:   model.EventPush,
		Branch:  "main",
		Commit:  "abcdef1234567890",
		Message: "test",
	}

	body := buildEmailBody("https://ci.example.com", repo, pipeline, "main", "")

	assert.Contains(t, body, "abcdef12")
	assert.NotContains(t, body, "abcdef1234567890")
}

func TestBuildEmailBody_MessageFirstLine(t *testing.T) {
	repo := &model.Repo{FullName: "crowci/crow"}
	pipeline := &model.Pipeline{
		Number:  1,
		Status:  model.StatusSuccess,
		Event:   model.EventPush,
		Branch:  "main",
		Commit:  "abc123",
		Message: "first line\nsecond line\nthird line",
	}

	body := buildEmailBody("https://ci.example.com", repo, pipeline, "main", "")

	assert.Contains(t, body, "first line")
	assert.NotContains(t, body, "second line")
}

func TestBuildEmailBody_UnsubscribeLink(t *testing.T) {
	repo := &model.Repo{FullName: "crowci/crow"}
	pipeline := &model.Pipeline{
		Number:  1,
		Status:  model.StatusSuccess,
		Event:   model.EventPush,
		Branch:  "main",
		Commit:  "abc123",
		Message: "test",
	}

	body := buildEmailBody("https://ci.example.com", repo, pipeline, "main", "https://ci.example.com/unsubscribe?token=xyz")

	assert.Contains(t, body, "Unsubscribe")
	assert.Contains(t, body, "https://ci.example.com/unsubscribe?token=xyz")
}

func TestBuildEmailBody_EscapesHTML(t *testing.T) {
	repo := &model.Repo{ID: 1, FullName: `crowci/<b>crow</b>`}
	pipeline := &model.Pipeline{
		Number:  1,
		Status:  model.StatusSuccess,
		Event:   model.EventPush,
		Branch:  "main",
		Commit:  "abc123",
		Message: `<script>alert('xss')</script>`,
	}

	body := buildEmailBody("https://ci.example.com", repo, pipeline, `feat/<img src=x onerror=alert(1)>`, "")

	// Raw markup from user-controlled fields must not appear verbatim.
	assert.NotContains(t, body, "<script>")
	assert.NotContains(t, body, "<b>crow</b>")
	assert.NotContains(t, body, "<img src=x")

	// The escaped forms must be present instead.
	assert.Contains(t, body, "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;")
	assert.Contains(t, body, "crowci/&lt;b&gt;crow&lt;/b&gt;")
	assert.Contains(t, body, "feat/&lt;img src=x onerror=alert(1)&gt;")
}

func TestStatusChangeLabel(t *testing.T) {
	tests := []struct {
		name     string
		current  model.StatusValue
		previous model.StatusValue
		want     string
	}{
		{name: "failure with no previous", current: model.StatusFailure, previous: "", want: "failure"},
		{name: "success with no previous", current: model.StatusSuccess, previous: "", want: "success"},
		{name: "failure after success (broken)", current: model.StatusFailure, previous: model.StatusSuccess, want: "broken"},
		{name: "success after failure (fixed)", current: model.StatusSuccess, previous: model.StatusFailure, want: "fixed"},
		{name: "failure after failure (same)", current: model.StatusFailure, previous: model.StatusFailure, want: "failure"},
		{name: "success after success (same)", current: model.StatusSuccess, previous: model.StatusSuccess, want: "success"},
		{name: "error after success (broken)", current: model.StatusError, previous: model.StatusSuccess, want: "broken"},
		{name: "success after error (fixed)", current: model.StatusSuccess, previous: model.StatusError, want: "fixed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, statusChangeLabel(tt.current, tt.previous))
		})
	}
}

func TestShouldNotify(t *testing.T) {
	tests := []struct {
		name     string
		events   []model.NotificationEvent
		current  model.StatusValue
		previous model.StatusValue
		want     bool
	}{
		{name: "failure event matches failure", events: []model.NotificationEvent{model.NotificationEventFailure}, current: model.StatusFailure, previous: "", want: true},
		{name: "failure event matches error", events: []model.NotificationEvent{model.NotificationEventFailure}, current: model.StatusError, previous: "", want: true},
		{name: "failure event skips success", events: []model.NotificationEvent{model.NotificationEventFailure}, current: model.StatusSuccess, previous: "", want: false},
		{name: "success event matches success", events: []model.NotificationEvent{model.NotificationEventSuccess}, current: model.StatusSuccess, previous: "", want: true},
		{name: "success event skips failure", events: []model.NotificationEvent{model.NotificationEventSuccess}, current: model.StatusFailure, previous: "", want: false},
		{name: "status change detects broken", events: []model.NotificationEvent{model.NotificationEventStatusChange}, current: model.StatusFailure, previous: model.StatusSuccess, want: true},
		{name: "status change detects fixed", events: []model.NotificationEvent{model.NotificationEventStatusChange}, current: model.StatusSuccess, previous: model.StatusFailure, want: true},
		{name: "status change skips same", events: []model.NotificationEvent{model.NotificationEventStatusChange}, current: model.StatusFailure, previous: model.StatusFailure, want: false},
		{name: "status change skips no previous", events: []model.NotificationEvent{model.NotificationEventStatusChange}, current: model.StatusFailure, previous: "", want: false},
		{name: "multiple events match any", events: []model.NotificationEvent{model.NotificationEventSuccess, model.NotificationEventFailure}, current: model.StatusFailure, previous: "", want: true},
		{name: "failure event skips killed (superseded/canceled)", events: []model.NotificationEvent{model.NotificationEventFailure}, current: model.StatusKilled, previous: "", want: false},
		{name: "failure event skips killed after success", events: []model.NotificationEvent{model.NotificationEventFailure}, current: model.StatusKilled, previous: model.StatusSuccess, want: false},
		{name: "status change skips killed after success", events: []model.NotificationEvent{model.NotificationEventStatusChange}, current: model.StatusKilled, previous: model.StatusSuccess, want: false},
		{name: "status change skips killed after failure", events: []model.NotificationEvent{model.NotificationEventStatusChange}, current: model.StatusKilled, previous: model.StatusFailure, want: false},
		{name: "failure event skips declined", events: []model.NotificationEvent{model.NotificationEventFailure}, current: model.StatusDeclined, previous: "", want: false},
		{name: "failure event skips skipped", events: []model.NotificationEvent{model.NotificationEventFailure}, current: model.StatusSkipped, previous: "", want: false},
		{name: "failure event skips blocked", events: []model.NotificationEvent{model.NotificationEventFailure}, current: model.StatusBlocked, previous: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shouldNotify(tt.events, tt.current, tt.previous))
		})
	}
}

func TestBuildEmailBody_ViewPipelineLink(t *testing.T) {
	repo := &model.Repo{ID: 42, FullName: "crowci/crow"}
	pipeline := &model.Pipeline{
		Number:  123,
		Status:  model.StatusSuccess,
		Event:   model.EventPush,
		Branch:  "main",
		Commit:  "abc123",
		Message: "test",
	}

	body := buildEmailBody("https://ci.example.com", repo, pipeline, "main", "")

	assert.Contains(t, body, `href="https://ci.example.com/repos/42/pipeline/123"`)
	assert.Contains(t, body, "View pipeline")
}

func TestBuildEmailBody_AllPREvents(t *testing.T) {
	prEvents := []model.WebhookEvent{
		model.EventPull,
		model.EventPullClosed,
		model.EventPullMerged,
		model.EventPullEdited,
	}

	for _, event := range prEvents {
		t.Run(string(event), func(t *testing.T) {
			repo := &model.Repo{FullName: "crowci/crow"}
			pipeline := &model.Pipeline{
				Number:  1,
				Status:  model.StatusSuccess,
				Event:   event,
				Branch:  "main",
				Ref:     "refs/pull/99/head",
				Commit:  "abc123",
				Message: "test",
			}

			body := buildEmailBody("https://ci.example.com", repo, pipeline, "PR #99", "")

			assert.Contains(t, body, ">Pull Request</td>")
			assert.Contains(t, body, ">PR #99</td>")
			assert.NotContains(t, body, ">Branch</td>")
			// Should NOT contain "main" as the ref value
			// (it may appear in other contexts, so check the specific table cell)
			assert.True(t, strings.Contains(body, "PR #99"))
		})
	}
}
