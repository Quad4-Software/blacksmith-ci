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
	"slices"

	"codefloe.com/crowci/crow/v6/server/model"
)

const branchRefPrefix = "refs/heads/"

// Access is what one running workflow may do with the cache of its repo.
type Access struct {
	RepoID int64
	// OrgID owns the repo; its limits apply to the cache.
	OrgID int64
	// BucketID is where the cache is kept: 0 for the server's storage,
	// otherwise the bucket the org connected.
	BucketID int64
	// ReadScopes are restored from, in order of preference.
	ReadScopes []string
	// WriteScope is the only scope saved to; empty when the workflow may only
	// restore.
	WriteScope string
}

// CanRead reports whether entries of scope may be restored.
func (a Access) CanRead(scope string) bool {
	return slices.Contains(a.ReadScopes, scope)
}

// AccessFor derives the cache access of a pipeline. Every pipeline is scoped
// to its git ref: it reads its own ref first, then the target branch of a pull
// request, then the default branch. It writes only its own ref, so a branch or
// pull request can never plant a cache that another branch restores. Pull
// requests from forks run untrusted code and may only restore.
func AccessFor(repo *model.Repo, pipeline *model.Pipeline) Access {
	own := pipeline.Ref
	if own == "" {
		own = branchRefPrefix + pipeline.Branch
	}

	scopes := []string{own}
	add := func(scope string) {
		if scope != branchRefPrefix && !slices.Contains(scopes, scope) {
			scopes = append(scopes, scope)
		}
	}
	if isPullRequest(pipeline.Event) {
		add(branchRefPrefix + pipeline.Branch)
	}
	add(branchRefPrefix + repo.Branch)

	access := Access{RepoID: repo.ID, OrgID: repo.OrgID, ReadScopes: scopes, WriteScope: own}
	// A ref longer than the scope column cannot be stored, so it only restores.
	if pipeline.FromFork || len(own) > MaxKeyLength {
		access.WriteScope = ""
	}
	return access
}

func isPullRequest(event model.WebhookEvent) bool {
	switch event {
	case model.EventPull, model.EventPullClosed, model.EventPullMerged, model.EventPullEdited:
		return true
	default:
		return false
	}
}
