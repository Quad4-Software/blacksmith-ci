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
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store/types"
)

func (s storage) GetUser(id int64) (*model.User, error) {
	user := new(model.User)
	return user, wrapGet(s.db.NewSelect().Model(user).Where("id = ?", id).Scan(context.Background()))
}

func (s storage) GetUserRemoteID(remoteID model.ForgeRemoteID, login string) (*model.User, error) {
	user := new(model.User)
	err := wrapGet(s.db.NewSelect().Model(user).Where("forge_remote_id = ?", remoteID).Scan(context.Background()))
	if err != nil {
		return s.getUserLogin(s.db, login)
	}
	return user, err
}

func (s storage) GetUserLogin(login string) (*model.User, error) {
	return s.getUserLogin(s.db, login)
}

func (s storage) getUserLogin(db bun.IDB, login string) (*model.User, error) {
	user := new(model.User)
	return user, wrapGet(db.NewSelect().Model(user).Where("login = ?", login).Scan(context.Background()))
}

func (s storage) GetUserList(p *model.ListOptions) ([]*model.User, error) {
	var users []*model.User
	return users, s.paginate(s.db.NewSelect().Model(&users).Order("login"), p).Scan(context.Background())
}

func (s storage) GetUserCount() (int64, error) {
	n, err := s.db.NewSelect().Model((*model.User)(nil)).Count(context.Background())
	return int64(n), err
}

func (s storage) CreateUser(user *model.User) error {
	org := &model.Org{
		Name:    user.Login,
		ForgeID: user.ForgeID,
		IsUser:  true,
	}

	existingOrg, err := s.orgFindByName(s.db, org.Name, user.ForgeID)
	if err != nil && !errors.Is(err, types.RecordNotExist) {
		return fmt.Errorf("failed to check if org exists: %w", err)
	}

	if !errors.Is(err, types.RecordNotExist) {
		org = existingOrg
		org.IsUser = true
		org.Name = user.Login
		err = s.orgUpdate(s.db, org)
		if err != nil {
			return fmt.Errorf("failed to update existing org: %w", err)
		}
	} else {
		err = s.orgCreate(s.db, org)
		if err != nil {
			return fmt.Errorf("failed to create new org: %w", err)
		}
	}
	user.OrgID = org.ID
	// only Insert set auto created ID back to object
	_, err = s.db.NewInsert().Model(user).Exec(context.Background())
	return err
}

func (s storage) UpdateUser(user *model.User) error {
	_, err := s.db.NewUpdate().Model(user).WherePK().Exec(context.Background())
	return err
}

func (s storage) UserListAll() ([]*model.User, error) {
	var users []*model.User
	return users, s.db.NewSelect().Model(&users).Scan(context.Background())
}

func (s storage) DeleteUser(user *model.User) error {
	return s.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		if err := s.orgDelete(ctx, &tx, user.OrgID); err != nil {
			return fmt.Errorf("failed to delete org: %w", err)
		}

		res, err := tx.NewDelete().Model((*model.User)(nil)).Where("id = ?", user.ID).Exec(ctx)
		if err := wrapDelete(res, err); err != nil {
			return fmt.Errorf("failed to delete user: %w", err)
		}

		if _, err := tx.NewDelete().Model((*model.Perm)(nil)).Where("user_id = ?", user.ID).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete perms: %w", err)
		}

		if _, err := tx.NewDelete().Model((*model.UserForge)(nil)).Where("user_id = ?", user.ID).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user forges: %w", err)
		}

		return nil
	})
}
