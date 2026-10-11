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

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

func (s storage) GetUser(id int64) (*model.User, error) {
	user := new(model.User)
	return user, wrapGet(s.db.NewSelect().Model(user).Where("id = ?", id).Scan(context.Background()))
}

// MainForgeID is the ID of the forge a single-forge installation uses. Rows
// written before multi-forge support may carry an unset forge_id; they belong
// to this forge.
const MainForgeID = 1

// GetUserByForgeRemoteID looks up the legacy `users` row for an account on the
// given forge. It is only used to adopt users that predate the `user_forges`
// table.
//
// Every lookup is scoped to forgeID. Forge remote IDs are per-forge sequences
// and login names are only unique within a forge, so an unscoped lookup would
// let an account on one forge resolve to a user registered on another and take
// it over.
func (s storage) GetUserByForgeRemoteID(forgeID int64, remoteID model.ForgeRemoteID, login string) (*model.User, error) {
	user := new(model.User)
	err := wrapGet(s.forgeScope(s.db.NewSelect().Model(user), forgeID).
		Where("forge_remote_id = ?", remoteID).
		Scan(context.Background()))
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, types.RecordNotExist) {
		return nil, err
	}

	// Rows created before remote IDs were tracked have no forge_remote_id, so
	// fall back to the login name for those only. Users registered through an
	// identity provider also have no forge_remote_id, but their login is not
	// forge-verified: a forge account of the same name must never adopt them.
	user = new(model.User)
	return user, wrapGet(s.forgeScope(s.db.NewSelect().Model(user), forgeID).
		Where("login = ?", login).
		Where("(forge_remote_id IS NULL OR forge_remote_id = '')").
		Where("NOT EXISTS (SELECT 1 FROM user_identities WHERE user_identities.user_id = ?TableAlias.id)").
		Scan(context.Background()))
}

// forgeScope restricts a `users` query to a single forge, counting rows with an
// unset forge_id as belonging to the main forge.
func (storage) forgeScope(q *bun.SelectQuery, forgeID int64) *bun.SelectQuery {
	if forgeID == MainForgeID {
		return q.Where("(forge_id = ? OR forge_id = 0 OR forge_id IS NULL)", forgeID)
	}
	return q.Where("forge_id = ?", forgeID)
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
	return n, err
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

		if _, err := tx.NewDelete().Model((*model.UserIdentity)(nil)).Where("user_id = ?", user.ID).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user identities: %w", err)
		}

		return nil
	})
}
