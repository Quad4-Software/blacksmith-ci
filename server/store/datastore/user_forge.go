// Copyright 2025 Crow Authors
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
	"time"

	"codefloe.com/crowci/crow/v6/server/model"
)

func (s storage) UserForgeList(userID int64) ([]*model.UserForge, error) {
	var userForges []*model.UserForge
	return userForges, s.db.NewSelect().Model(&userForges).
		Where("user_id = ?", userID).
		Order("is_primary DESC", "linked_at ASC").
		Scan(context.Background())
}

func (s storage) UserForgeGet(userID, forgeID int64) (*model.UserForge, error) {
	userForge := new(model.UserForge)
	return userForge, wrapGet(s.db.NewSelect().Model(userForge).
		Where("user_id = ? AND forge_id = ?", userID, forgeID).
		Scan(context.Background()))
}

func (s storage) UserForgeGetByRemoteID(forgeID int64, remoteID model.ForgeRemoteID) (*model.UserForge, error) {
	userForge := new(model.UserForge)
	return userForge, wrapGet(s.db.NewSelect().Model(userForge).
		Where("forge_id = ? AND forge_remote_id = ?", forgeID, remoteID).
		Scan(context.Background()))
}

func (s storage) UserForgeGetPrimary(userID int64) (*model.UserForge, error) {
	userForge := new(model.UserForge)
	return userForge, wrapGet(s.db.NewSelect().Model(userForge).
		Where("user_id = ? AND is_primary = ?", userID, true).
		Scan(context.Background()))
}

func (s storage) UserForgeCreate(userForge *model.UserForge) error {
	if userForge.LinkedAt == 0 {
		userForge.LinkedAt = time.Now().Unix()
	}
	_, err := s.db.NewInsert().Model(userForge).Exec(context.Background())
	return err
}

func (s storage) UserForgeUpdate(userForge *model.UserForge) error {
	userForge.LastUsed = time.Now().Unix()
	_, err := s.db.NewUpdate().Model(userForge).WherePK().Exec(context.Background())
	return err
}

func (s storage) UserForgeDelete(userForge *model.UserForge) error {
	return wrapDelete(s.db.NewDelete().Model((*model.UserForge)(nil)).Where("id = ?", userForge.ID).Exec(context.Background()))
}

func (s storage) UserForgeListAll() ([]*model.UserForge, error) {
	var userForges []*model.UserForge
	return userForges, s.db.NewSelect().Model(&userForges).Scan(context.Background())
}
