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

const orderRegistriesBy = "id ASC"

func (s storage) RegistryFind(repo *model.Repo, addr string) (*model.Registry, error) {
	reg := new(model.Registry)
	return reg, wrapGet(s.db.NewSelect().Model(reg).
		Where("repo_id = ? AND address = ?", repo.ID, addr).
		Scan(context.Background()))
}

func (s storage) RegistryList(repo *model.Repo, includeGlobalAndOrg bool, p *model.ListOptions) ([]*model.Registry, error) {
	var regs []*model.Registry
	q := s.db.NewSelect().Model(&regs)
	if includeGlobalAndOrg {
		q = q.Where("repo_id = ? OR org_id = ? OR (org_id = 0 AND repo_id = 0)", repo.ID, repo.OrgID)
	} else {
		q = q.Where("repo_id = ?", repo.ID)
	}
	return regs, s.paginate(q.Order(orderRegistriesBy), p).Scan(context.Background())
}

func (s storage) RegistryListAll() ([]*model.Registry, error) {
	var registries []*model.Registry
	return registries, s.db.NewSelect().Model(&registries).Scan(context.Background())
}

func (s storage) RegistryCreate(registry *model.Registry) error {
	// only Insert set auto created ID back to object
	_, err := s.db.NewInsert().Model(registry).Exec(context.Background())
	return err
}

func (s storage) RegistryUpdate(registry *model.Registry) error {
	_, err := s.db.NewUpdate().Model(registry).WherePK().Exec(context.Background())
	return err
}

func (s storage) RegistryDelete(registry *model.Registry) error {
	return wrapDelete(s.db.NewDelete().Model((*model.Registry)(nil)).Where("id = ?", registry.ID).Exec(context.Background()))
}

func (s storage) OrgRegistryFind(orgID int64, name string) (*model.Registry, error) {
	registry := new(model.Registry)
	return registry, wrapGet(s.db.NewSelect().Model(registry).
		Where("org_id = ? AND address = ?", orgID, name).
		Scan(context.Background()))
}

func (s storage) OrgRegistryList(orgID int64, p *model.ListOptions) ([]*model.Registry, error) {
	registries := make([]*model.Registry, 0)
	return registries, s.paginate(s.db.NewSelect().Model(&registries).Where("org_id = ?", orgID).Order(orderRegistriesBy), p).Scan(context.Background())
}

func (s storage) GlobalRegistryFind(name string) (*model.Registry, error) {
	registry := new(model.Registry)
	return registry, wrapGet(s.db.NewSelect().Model(registry).
		Where("org_id = 0 AND repo_id = 0 AND address = ?", name).
		Scan(context.Background()))
}

func (s storage) GlobalRegistryList(p *model.ListOptions) ([]*model.Registry, error) {
	registries := make([]*model.Registry, 0)
	return registries, s.paginate(s.db.NewSelect().Model(&registries).
		Where("org_id = 0 AND repo_id = 0").
		Order(orderRegistriesBy), p).Scan(context.Background())
}

func (s storage) UserRegistryList(orgIDs []int64, includeGlobal bool, searchTerm string, p *model.ListOptions) ([]*model.Registry, error) {
	registries := make([]*model.Registry, 0)

	if len(orgIDs) == 0 && !includeGlobal {
		return registries, nil
	}

	q := s.db.NewSelect().Model(&registries)

	// Build the scope condition as a WhereGroup so all parts are OR'd together
	q = q.WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
		if len(orgIDs) > 0 {
			sq = sq.WhereOr("(org_id IN (?) AND repo_id = 0)", bun.List(orgIDs))
		}
		if includeGlobal {
			sq = sq.WhereOr("(org_id = 0 AND repo_id = 0)")
		}
		return sq
	})

	// Add search filter if provided
	if searchTerm != "" {
		q = q.Where("address LIKE ?", "%"+searchTerm+"%")
	}

	return registries, s.paginate(q.Order(orderRegistriesBy), p).Scan(context.Background())
}
