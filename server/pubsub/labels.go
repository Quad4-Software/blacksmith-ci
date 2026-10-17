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

package pubsub

import "codefloe.com/crowci/crow/v6/server/model"

const (
	// LabelRepo carries the full name of the repo a message is about.
	LabelRepo = "repo"
	// LabelVisibility carries the Crow visibility of the repo a message is about.
	LabelVisibility = "visibility"
)

// RepoLabels returns the labels of a message about repo.
// Every publisher of repo events uses it, so subscribers can rely on the
// labels when deciding who may receive a message.
func RepoLabels(repo *model.Repo) map[string]string {
	return map[string]string{
		LabelRepo:       repo.FullName,
		LabelVisibility: string(repo.Visibility),
	}
}

// VisibleWithoutRepoPerm reports whether a message labeled by RepoLabels may
// be delivered to a subscriber who has no permission on the repo.
// It mirrors the REST API: a public repo is visible to everyone and an
// internal repo to every logged-in user.
// A private repo, or a message without a known visibility, is not.
func VisibleWithoutRepoPerm(labels map[string]string, loggedIn bool) bool {
	switch model.RepoVisibility(labels[LabelVisibility]) {
	case model.VisibilityPublic:
		return true
	case model.VisibilityInternal:
		return loggedIn
	default:
		return false
	}
}
