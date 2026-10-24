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
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
)

// CanPull reports whether the user may read the repo, using the permissions
// stored in the database without resyncing from the forge.
func CanPull(s store.Store, user *model.User, repo *model.Repo) bool {
	if user == nil {
		return repo.Visibility == model.VisibilityPublic
	}
	if user.Admin {
		return true
	}
	switch repo.Visibility {
	case model.VisibilityPublic, model.VisibilityInternal:
		return true
	}
	perm, err := s.PermFind(user, repo)
	if err != nil || perm == nil {
		return false
	}
	return perm.Pull
}
