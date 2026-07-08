// Copyright 2022 Woodpecker Authors
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

	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v6/server/model"
)

func (s storage) getRedirection(ctx context.Context, db bun.IDB, fullName string) (*model.Redirection, error) {
	repo := new(model.Redirection)
	return repo, wrapGet(db.NewSelect().Model(repo).Where("repo_full_name = ?", fullName).Scan(ctx))
}

func (s storage) CreateRedirection(redirect *model.Redirection) error {
	return s.createRedirection(context.Background(), s.db, redirect)
}

func (s storage) createRedirection(ctx context.Context, db bun.IDB, redirect *model.Redirection) error {
	// only Insert set auto created ID back to object
	_, err := db.NewInsert().Model(redirect).Exec(ctx)
	return err
}

func (s storage) HasRedirectionForRepo(repoID int64, fullName string) (bool, error) {
	return s.db.NewSelect().Model((*model.Redirection)(nil)).
		Where("repo_id = ?", repoID).
		Where("repo_full_name = ?", fullName).
		Exists(context.Background())
}
