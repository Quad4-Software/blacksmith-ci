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

package watch

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
	smocks "codefloe.com/crowci/crow/v6/server/services/mocks"
)

func TestCanPull(t *testing.T) {
	user := &model.User{ID: 1}
	tests := []struct {
		name string
		user *model.User
		vis  model.RepoVisibility
		perm *model.Perm
		want bool
	}{
		{name: "admin on private", user: &model.User{ID: 2, Admin: true}, vis: model.VisibilityPrivate, want: true},
		{name: "public repo", user: user, vis: model.VisibilityPublic, want: true},
		{name: "internal repo signed in", user: user, vis: model.VisibilityInternal, want: true},
		{name: "private with pull", user: user, vis: model.VisibilityPrivate, perm: &model.Perm{Pull: true}, want: true},
		{name: "private without pull", user: user, vis: model.VisibilityPrivate, perm: &model.Perm{Pull: false}, want: false},
		{name: "private without perm record", user: user, vis: model.VisibilityPrivate, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := smocks.NewMockStore(t)
			repo := &model.Repo{ID: 1, Visibility: tt.vis}
			if !tt.user.Admin && tt.vis == model.VisibilityPrivate {
				if tt.perm != nil {
					st.EXPECT().PermFind(tt.user, repo).Return(tt.perm, nil)
				} else {
					st.EXPECT().PermFind(tt.user, repo).Return(nil, assert.AnError)
				}
			}
			assert.Equal(t, tt.want, CanPull(st, tt.user, repo))
		})
	}
}
