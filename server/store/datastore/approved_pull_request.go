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
	"context"

	"codefloe.com/crowci/crow/v6/server/model"
)

func (s storage) ApprovedPullRequestFind(repoID int64, ref string) (*model.ApprovedPullRequest, error) {
	apr := new(model.ApprovedPullRequest)
	err := wrapGet(s.db.NewSelect().Model(apr).Where("repo_id = ? AND ref = ?", repoID, ref).Scan(context.Background()))
	if err != nil {
		return nil, err
	}
	return apr, nil
}

func (s storage) ApprovedPullRequestCreate(apr *model.ApprovedPullRequest) error {
	_, err := s.db.NewInsert().Model(apr).Exec(context.Background())
	return err
}

func (s storage) ApprovedPullRequestDelete(repoID int64, ref string) error {
	return wrapDelete(s.db.NewDelete().Model((*model.ApprovedPullRequest)(nil)).Where("repo_id = ? AND ref = ?", repoID, ref).Exec(context.Background()))
}
