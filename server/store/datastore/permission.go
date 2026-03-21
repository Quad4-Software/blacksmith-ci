// Copyright 2021 Woodpecker Authors
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
	"fmt"

	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v5/server/model"
)

func (s storage) PermFind(user *model.User, repo *model.Repo) (*model.Perm, error) {
	perm := new(model.Perm)
	return perm, wrapGet(s.db.NewSelect().Model(perm).
		Where("user_id = ? AND repo_id = ?", user.ID, repo.ID).
		Scan(context.Background()))
}

func (s storage) PermUpsert(perm *model.Perm) error {
	return s.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		return s.permUpsert(ctx, tx, perm)
	})
}

func (s storage) permUpsert(ctx context.Context, db bun.IDB, perm *model.Perm) error {
	if perm.RepoID == 0 && perm.Repo == nil {
		return fmt.Errorf("could not determine repo for permission: %v", perm)
	}

	// lookup repo based on name or forge ID if possible
	if perm.RepoID == 0 && perm.Repo != nil {
		r, err := s.getRepoNameFallback(ctx, db, perm.Repo.ForgeRemoteID, perm.Repo.FullName)
		if err != nil {
			return err
		}
		perm.RepoID = r.ID
	}

	exists, err := db.NewSelect().Model((*model.Perm)(nil)).
		Where("user_id = ? AND repo_id = ?", perm.UserID, perm.RepoID).
		Exists(ctx)
	if err != nil {
		return err
	}

	if exists {
		_, err = db.NewUpdate().Model(perm).
			Where("user_id = ? AND repo_id = ?", perm.UserID, perm.RepoID).
			Exec(ctx)
	} else {
		_, err = db.NewInsert().Model(perm).Exec(ctx)
	}
	return err
}
