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
	"fmt"
	"strings"

	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v6/server/model"
)

func (s storage) OrgCreate(org *model.Org) error {
	return s.orgCreate(s.db, org)
}

func (s storage) orgCreate(db bun.IDB, org *model.Org) error {
	if org.Name == "" {
		return fmt.Errorf("org name is empty")
	}
	// insert
	_, err := db.NewInsert().Model(org).Exec(context.Background())
	return err
}

func (s storage) OrgGet(id int64) (*model.Org, error) {
	org := new(model.Org)
	return org, wrapGet(s.db.NewSelect().Model(org).Where("id = ?", id).Scan(context.Background()))
}

func (s storage) OrgUpdate(org *model.Org) error {
	return s.orgUpdate(s.db, org)
}

func (s storage) orgUpdate(db bun.IDB, org *model.Org) error {
	_, err := db.NewUpdate().Model(org).WherePK().Exec(context.Background())
	return err
}

func (s storage) OrgDelete(id int64) error {
	return s.orgDelete(context.Background(), s.db, id)
}

func (s storage) orgDelete(ctx context.Context, db bun.IDB, id int64) error {
	if _, err := db.NewDelete().Model((*model.Secret)(nil)).Where("org_id = ?", id).Exec(ctx); err != nil {
		return err
	}

	var repos []*model.Repo
	if err := db.NewSelect().Model(&repos).Where("org_id = ?", id).Scan(ctx); err != nil {
		return err
	}

	for _, repo := range repos {
		if err := s.deleteRepo(ctx, db, repo); err != nil {
			return err
		}
	}

	return wrapDelete(db.NewDelete().Model((*model.Org)(nil)).Where("id = ?", id).Exec(ctx))
}

func (s storage) OrgFindByName(name string, forgeID int64) (*model.Org, error) {
	return s.orgFindByName(s.db, name, forgeID)
}

func (s storage) orgFindByName(db bun.IDB, name string, forgeID int64) (*model.Org, error) {
	org := new(model.Org)
	return org, wrapGet(db.NewSelect().Model(org).Where("LOWER(name) = ? AND forge_id = ?", strings.ToLower(name), forgeID).Scan(context.Background()))
}

// OrgFindByNameAllForges returns all orgs with the given name across all forges.
func (s storage) OrgFindByNameAllForges(name string) ([]*model.Org, error) {
	var orgs []*model.Org
	return orgs, s.db.NewSelect().Model(&orgs).Where("LOWER(name) = ?", strings.ToLower(name)).Scan(context.Background())
}

func (s storage) OrgRepoList(org *model.Org, p *model.ListOptions) ([]*model.Repo, error) {
	var repos []*model.Repo
	return repos, s.paginate(s.db.NewSelect().Model(&repos).Where("org_id = ?", org.ID).Order("id"), p).Scan(context.Background())
}

func (s storage) OrgList(p *model.ListOptions) ([]*model.Org, error) {
	var orgs []*model.Org
	return orgs, s.paginate(s.db.NewSelect().Model(&orgs).Where("is_user = ?", false).Order("id"), p).Scan(context.Background())
}
