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
	"database/sql"
	"errors"

	"codefloe.com/crowci/crow/v5/server/model"
)

func (s storage) NotificationConfigFind(repoID int64) (*model.NotificationConfig, error) {
	config := new(model.NotificationConfig)
	return config, wrapGet(s.db.NewSelect().Model(config).Where("repo_id = ?", repoID).Scan(context.Background()))
}

func (s storage) NotificationConfigUpsert(config *model.NotificationConfig) error {
	existing := new(model.NotificationConfig)
	err := s.db.NewSelect().Model(existing).Where("repo_id = ?", config.RepoID).Scan(context.Background())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if err == nil {
		config.ID = existing.ID
		_, err = s.db.NewUpdate().Model(config).WherePK().Exec(context.Background())
		return err
	}

	_, err = s.db.NewInsert().Model(config).Exec(context.Background())
	return err
}

func (s storage) NotificationConfigDelete(repoID int64) error {
	return wrapDelete(s.db.NewDelete().Model((*model.NotificationConfig)(nil)).Where("repo_id = ?", repoID).Exec(context.Background()))
}
