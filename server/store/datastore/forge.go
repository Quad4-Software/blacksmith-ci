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

	"codefloe.com/crowci/crow/v5/server/model"
)

func (s storage) ForgeGet(id int64) (*model.Forge, error) {
	forge := new(model.Forge)
	return forge, wrapGet(s.db.NewSelect().Model(forge).Where("id = ?", id).Scan(context.Background()))
}

func (s storage) ForgeList(p *model.ListOptions) ([]*model.Forge, error) {
	forges := make([]*model.Forge, 0, 10)
	return forges, s.paginate(s.db.NewSelect().Model(&forges), p).Scan(context.Background())
}

func (s storage) ForgeListAll() ([]*model.Forge, error) {
	var forges []*model.Forge
	return forges, s.db.NewSelect().Model(&forges).Scan(context.Background())
}

func (s storage) ForgeCreate(forge *model.Forge) error {
	// only Insert set auto created ID back to object
	_, err := s.db.NewInsert().Model(forge).Exec(context.Background())
	return err
}

func (s storage) ForgeUpdate(forge *model.Forge) error {
	_, err := s.db.NewUpdate().Model(forge).WherePK().Exec(context.Background())
	return err
}

func (s storage) ForgeDelete(forge *model.Forge) error {
	return s.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		return wrapDelete(tx.NewDelete().Model((*model.Forge)(nil)).Where("id = ?", forge.ID).Exec(ctx))
	})
}
