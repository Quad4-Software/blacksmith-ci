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

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
)

func TestRepoLabelsUseCrowVisibility(t *testing.T) {
	// A repo that is public on the forge but private in Crow.
	repo := &model.Repo{FullName: "octo/app", IsSCMPrivate: false, Visibility: model.VisibilityPrivate}

	labels := RepoLabels(repo)

	assert.Equal(t, map[string]string{LabelRepo: "octo/app", LabelVisibility: "private"}, labels)
	assert.False(t, VisibleWithoutRepoPerm(labels, false))
	assert.False(t, VisibleWithoutRepoPerm(labels, true))
}

func TestVisibleWithoutRepoPerm(t *testing.T) {
	tests := []struct {
		visibility model.RepoVisibility
		anonymous  bool
		loggedIn   bool
	}{
		{visibility: model.VisibilityPublic, anonymous: true, loggedIn: true},
		{visibility: model.VisibilityInternal, anonymous: false, loggedIn: true},
		{visibility: model.VisibilityPrivate, anonymous: false, loggedIn: false},
		{visibility: "", anonymous: false, loggedIn: false},
		{visibility: "bogus", anonymous: false, loggedIn: false},
	}
	for _, tt := range tests {
		t.Run(string(tt.visibility), func(t *testing.T) {
			labels := RepoLabels(&model.Repo{FullName: "octo/app", Visibility: tt.visibility})
			assert.Equal(t, tt.anonymous, VisibleWithoutRepoPerm(labels, false), "anonymous")
			assert.Equal(t, tt.loggedIn, VisibleWithoutRepoPerm(labels, true), "logged in")
		})
	}

	// A message without labels, such as one from a publisher that does not
	// use RepoLabels, is never broadcast.
	assert.False(t, VisibleWithoutRepoPerm(nil, true))
}
