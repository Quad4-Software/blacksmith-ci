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

	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v5/server/model"
)

const orderSecretsBy = "name ASC"

func (s storage) SecretFind(repo *model.Repo, name string) (*model.Secret, error) {
	secret := new(model.Secret)
	return secret, wrapGet(s.db.NewSelect().Model(secret).
		Where("repo_id = ? AND name = ?", repo.ID, name).
		Scan(context.Background()))
}

func (s storage) SecretFindByID(id int64) (*model.Secret, error) {
	secret := new(model.Secret)
	return secret, wrapGet(s.db.NewSelect().Model(secret).
		Where("id = ?", id).
		Scan(context.Background()))
}

func (s storage) SecretList(repo *model.Repo, includeGlobalAndOrgSecrets bool, p *model.ListOptions) ([]*model.Secret, error) {
	var secrets []*model.Secret
	q := s.db.NewSelect().Model(&secrets)
	if includeGlobalAndOrgSecrets {
		q = q.Where(
			"repo_id = ? OR org_id = ? OR (org_id = 0 AND repo_id = 0)"+
				" OR id IN (SELECT secret_id FROM secret_targets WHERE repo_id = ? OR org_id = ?)",
			repo.ID, repo.OrgID, repo.ID, repo.OrgID,
		)
	} else {
		q = q.Where("repo_id = ?", repo.ID)
	}
	return secrets, s.paginate(q.Order(orderSecretsBy), p).Scan(context.Background())
}

func (s storage) SecretListAll() ([]*model.Secret, error) {
	var secrets []*model.Secret
	return secrets, s.db.NewSelect().Model(&secrets).Scan(context.Background())
}

func (s storage) SecretCreate(secret *model.Secret) error {
	// only Insert set auto created ID back to object
	_, err := s.db.NewInsert().Model(secret).Exec(context.Background())
	return err
}

func (s storage) SecretUpdate(secret *model.Secret) error {
	_, err := s.db.NewUpdate().Model(secret).WherePK().Exec(context.Background())
	return err
}

func (s storage) SecretDelete(secret *model.Secret) error {
	return wrapDelete(s.db.NewDelete().Model((*model.Secret)(nil)).Where("id = ?", secret.ID).Exec(context.Background()))
}

func (s storage) OrgSecretFind(orgID int64, name string) (*model.Secret, error) {
	secret := new(model.Secret)
	return secret, wrapGet(s.db.NewSelect().Model(secret).
		Where("org_id = ? AND name = ?", orgID, name).
		Scan(context.Background()))
}

func (s storage) OrgSecretList(orgID int64, p *model.ListOptions) ([]*model.Secret, error) {
	secrets := make([]*model.Secret, 0)
	return secrets, s.paginate(
		s.db.NewSelect().Model(&secrets).
			Where("org_id = ? OR id IN (SELECT secret_id FROM secret_targets WHERE org_id = ?)", orgID, orgID).
			Order(orderSecretsBy), p,
	).Scan(context.Background())
}

func (s storage) GlobalSecretFind(name string) (*model.Secret, error) {
	secret := new(model.Secret)
	return secret, wrapGet(s.db.NewSelect().Model(secret).
		Where("org_id = 0 AND repo_id = 0 AND name = ?", name).
		Scan(context.Background()))
}

func (s storage) GlobalSecretList(p *model.ListOptions) ([]*model.Secret, error) {
	secrets := make([]*model.Secret, 0)
	return secrets, s.paginate(s.db.NewSelect().Model(&secrets).
		Where("org_id = 0 AND repo_id = 0").
		Order(orderSecretsBy), p).Scan(context.Background())
}

func (s storage) UserSecretList(orgIDs []int64, repoIDs []int64, includeGlobal bool, searchTerm string, p *model.ListOptions) ([]*model.Secret, error) {
	secrets := make([]*model.Secret, 0)

	if len(orgIDs) == 0 && len(repoIDs) == 0 && !includeGlobal {
		return secrets, nil
	}

	q := s.db.NewSelect().Model(&secrets)

	// Build the scope condition as a WhereGroup so all parts are OR'd together
	q = q.WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
		if len(orgIDs) > 0 {
			sq = sq.WhereOr("(org_id IN (?) AND repo_id = 0)", bun.List(orgIDs))
			sq = sq.WhereOr("id IN (SELECT secret_id FROM secret_targets WHERE org_id IN (?))", bun.List(orgIDs))
		}
		if len(repoIDs) > 0 {
			sq = sq.WhereOr("repo_id IN (?)", bun.List(repoIDs))
			sq = sq.WhereOr("id IN (SELECT secret_id FROM secret_targets WHERE repo_id IN (?))", bun.List(repoIDs))
		}
		if includeGlobal {
			sq = sq.WhereOr("(org_id = 0 AND repo_id = 0)")
		}
		return sq
	})

	// Add search filter if provided
	if searchTerm != "" {
		q = q.Where("name LIKE ?", "%"+searchTerm+"%")
	}

	return secrets, s.paginate(q.Order(orderSecretsBy), p).Scan(context.Background())
}
