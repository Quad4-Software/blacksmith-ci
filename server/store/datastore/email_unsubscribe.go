// Copyright 2026 Crow Authors
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

	"codefloe.com/crowci/crow/v5/server/model"
)

func (s storage) EmailUnsubscribeFind(userID, repoID int64) (*model.EmailUnsubscribe, error) {
	unsub := new(model.EmailUnsubscribe)
	return unsub, wrapGet(s.db.NewSelect().Model(unsub).
		Where("user_id = ?", userID).
		Where("repo_id = ?", repoID).
		Scan(context.Background()))
}

func (s storage) EmailUnsubscribeCreate(unsub *model.EmailUnsubscribe) error {
	_, err := s.db.NewInsert().Model(unsub).
		On("CONFLICT (user_id, repo_id) DO NOTHING").
		Exec(context.Background())
	return err
}

func (s storage) EmailUnsubscribeDelete(userID, repoID int64) error {
	return wrapDelete(s.db.NewDelete().Model((*model.EmailUnsubscribe)(nil)).
		Where("user_id = ?", userID).
		Where("repo_id = ?", repoID).
		Exec(context.Background()))
}
