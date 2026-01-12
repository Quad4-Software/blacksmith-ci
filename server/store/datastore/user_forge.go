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
	"time"

	"codeberg.org/crowci/crow/v5/server/model"
)

func (s storage) UserForgeList(userID int64) ([]*model.UserForge, error) {
	var userForges []*model.UserForge
	return userForges, s.engine.Where("user_id = ?", userID).OrderBy("is_primary DESC, linked_at ASC").Find(&userForges)
}

func (s storage) UserForgeGet(userID, forgeID int64) (*model.UserForge, error) {
	userForge := new(model.UserForge)
	return userForge, wrapGet(s.engine.Where("user_id = ? AND forge_id = ?", userID, forgeID).Get(userForge))
}

func (s storage) UserForgeGetByRemoteID(forgeID int64, remoteID model.ForgeRemoteID) (*model.UserForge, error) {
	userForge := new(model.UserForge)
	return userForge, wrapGet(s.engine.Where("forge_id = ? AND forge_remote_id = ?", forgeID, remoteID).Get(userForge))
}

func (s storage) UserForgeGetPrimary(userID int64) (*model.UserForge, error) {
	userForge := new(model.UserForge)
	return userForge, wrapGet(s.engine.Where("user_id = ? AND is_primary = ?", userID, true).Get(userForge))
}

func (s storage) UserForgeCreate(userForge *model.UserForge) error {
	if userForge.LinkedAt == 0 {
		userForge.LinkedAt = time.Now().Unix()
	}
	_, err := s.engine.Insert(userForge)
	return err
}

func (s storage) UserForgeUpdate(userForge *model.UserForge) error {
	userForge.LastUsed = time.Now().Unix()
	_, err := s.engine.ID(userForge.ID).AllCols().Update(userForge)
	return err
}

func (s storage) UserForgeDelete(userForge *model.UserForge) error {
	return wrapDelete(s.engine.ID(userForge.ID).Delete(new(model.UserForge)))
}

func (s storage) UserForgeListAll() ([]*model.UserForge, error) {
	var userForges []*model.UserForge
	return userForges, s.engine.Find(&userForges)
}
