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

package datastore

import (
	"codefloe.com/crowci/crow/v5/server/model"
)

func (s storage) ApprovedPullRequestFind(repoID int64, ref string) (*model.ApprovedPullRequest, error) {
	apr := new(model.ApprovedPullRequest)
	return apr, wrapGet(s.engine.Where("repo_id = ? AND ref = ?", repoID, ref).Get(apr))
}

func (s storage) ApprovedPullRequestCreate(apr *model.ApprovedPullRequest) error {
	_, err := s.engine.Insert(apr)
	return err
}

func (s storage) ApprovedPullRequestDelete(repoID int64, ref string) error {
	return wrapDelete(s.engine.Where("repo_id = ? AND ref = ?", repoID, ref).Delete(new(model.ApprovedPullRequest)))
}
