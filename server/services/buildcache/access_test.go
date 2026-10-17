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

package buildcache

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
)

func TestAccessFor(t *testing.T) {
	repo := &model.Repo{ID: 7, Branch: "main"}
	tests := []struct {
		name     string
		pipeline *model.Pipeline
		want     Access
	}{
		{
			name:     "push to the default branch",
			pipeline: &model.Pipeline{Event: model.EventPush, Branch: "main", Ref: "refs/heads/main"},
			want:     Access{RepoID: 7, ReadScopes: []string{"refs/heads/main"}, WriteScope: "refs/heads/main"},
		},
		{
			name:     "push to a feature branch",
			pipeline: &model.Pipeline{Event: model.EventPush, Branch: "feature", Ref: "refs/heads/feature"},
			want:     Access{RepoID: 7, ReadScopes: []string{"refs/heads/feature", "refs/heads/main"}, WriteScope: "refs/heads/feature"},
		},
		{
			name:     "pull request reads its target branch",
			pipeline: &model.Pipeline{Event: model.EventPull, Branch: "release", Ref: "refs/pull/3/head"},
			want:     Access{RepoID: 7, ReadScopes: []string{"refs/pull/3/head", "refs/heads/release", "refs/heads/main"}, WriteScope: "refs/pull/3/head"},
		},
		{
			name:     "pull request from a fork is read-only",
			pipeline: &model.Pipeline{Event: model.EventPull, Branch: "main", Ref: "refs/pull/4/head", FromFork: true},
			want:     Access{RepoID: 7, ReadScopes: []string{"refs/pull/4/head", "refs/heads/main"}},
		},
		{
			name:     "tag",
			pipeline: &model.Pipeline{Event: model.EventTag, Ref: "refs/tags/v1.0.0"},
			want:     Access{RepoID: 7, ReadScopes: []string{"refs/tags/v1.0.0", "refs/heads/main"}, WriteScope: "refs/tags/v1.0.0"},
		},
		{
			name:     "missing ref falls back to the branch",
			pipeline: &model.Pipeline{Event: model.EventCron, Branch: "nightly"},
			want:     Access{RepoID: 7, ReadScopes: []string{"refs/heads/nightly", "refs/heads/main"}, WriteScope: "refs/heads/nightly"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AccessFor(repo, tt.pipeline))
		})
	}
}
