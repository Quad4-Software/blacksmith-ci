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
	"xorm.io/builder"

	"codeberg.org/crowci/crow/v5/server/model"
)

const orderSecretsBy = "name"

func (s storage) SecretFind(repo *model.Repo, name string) (*model.Secret, error) {
	secret := new(model.Secret)
	return secret, wrapGet(s.engine.Where(
		builder.Eq{"repo_id": repo.ID, "name": name},
	).Get(secret))
}

func (s storage) SecretList(repo *model.Repo, includeGlobalAndOrgSecrets bool, p *model.ListOptions) ([]*model.Secret, error) {
	var secrets []*model.Secret
	var cond builder.Cond = builder.Eq{"repo_id": repo.ID}
	if includeGlobalAndOrgSecrets {
		cond = cond.Or(builder.Eq{"org_id": repo.OrgID}).
			Or(builder.And(builder.Eq{"org_id": 0}, builder.Eq{"repo_id": 0}))
	}
	return secrets, s.paginate(p).Where(cond).OrderBy(orderSecretsBy).Find(&secrets)
}

func (s storage) SecretListAll() ([]*model.Secret, error) {
	var secrets []*model.Secret
	return secrets, s.engine.Find(&secrets)
}

func (s storage) SecretCreate(secret *model.Secret) error {
	// only Insert set auto created ID back to object
	_, err := s.engine.Insert(secret)
	return err
}

func (s storage) SecretUpdate(secret *model.Secret) error {
	_, err := s.engine.ID(secret.ID).AllCols().Update(secret)
	return err
}

func (s storage) SecretDelete(secret *model.Secret) error {
	return wrapDelete(s.engine.ID(secret.ID).Delete(new(model.Secret)))
}

func (s storage) OrgSecretFind(orgID int64, name string) (*model.Secret, error) {
	secret := new(model.Secret)
	return secret, wrapGet(s.engine.Where(
		builder.Eq{"org_id": orgID, "name": name},
	).Get(secret))
}

func (s storage) OrgSecretList(orgID int64, p *model.ListOptions) ([]*model.Secret, error) {
	secrets := make([]*model.Secret, 0)
	return secrets, s.paginate(p).Where("org_id = ?", orgID).OrderBy(orderSecretsBy).Find(&secrets)
}

func (s storage) GlobalSecretFind(name string) (*model.Secret, error) {
	secret := new(model.Secret)
	return secret, wrapGet(s.engine.Where(
		builder.Eq{"org_id": 0, "repo_id": 0, "name": name},
	).Get(secret))
}

func (s storage) GlobalSecretList(p *model.ListOptions) ([]*model.Secret, error) {
	secrets := make([]*model.Secret, 0)
	return secrets, s.paginate(p).Where(
		builder.Eq{"org_id": 0, "repo_id": 0},
	).OrderBy(orderSecretsBy).Find(&secrets)
}

func (s storage) UserSecretList(orgIDs []int64, repoIDs []int64, includeGlobal bool, searchTerm string, p *model.ListOptions) ([]*model.Secret, error) {
	secrets := make([]*model.Secret, 0)

	if len(orgIDs) == 0 && len(repoIDs) == 0 && !includeGlobal {
		return secrets, nil
	}

	// Build the main condition for org, repo, and global secrets
	var scopeConds []builder.Cond

	if len(orgIDs) > 0 {
		// Org-level secrets: org_id IN (orgIDs) AND repo_id = 0
		scopeConds = append(scopeConds, builder.And(
			builder.In("org_id", orgIDs),
			builder.Eq{"repo_id": 0},
		))
	}

	if len(repoIDs) > 0 {
		// Repo-level secrets: repo_id IN (repoIDs)
		scopeConds = append(scopeConds, builder.In("repo_id", repoIDs))
	}

	if includeGlobal {
		// Global secrets: org_id = 0 AND repo_id = 0
		scopeConds = append(scopeConds, builder.And(
			builder.Eq{"org_id": 0},
			builder.Eq{"repo_id": 0},
		))
	}

	// Combine scope conditions with OR
	var cond builder.Cond
	if len(scopeConds) == 1 {
		cond = scopeConds[0]
	} else if len(scopeConds) > 1 {
		cond = builder.Or(scopeConds...)
	}

	// Add search filter if provided
	if searchTerm != "" && cond != nil {
		cond = builder.And(cond, builder.Like{"name", "%" + searchTerm + "%"})
	}

	if cond == nil {
		return secrets, nil
	}

	return secrets, s.paginate(p).Where(cond).OrderBy(orderSecretsBy).Find(&secrets)
}
