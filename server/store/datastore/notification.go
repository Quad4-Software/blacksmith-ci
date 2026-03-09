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
	"codefloe.com/crowci/crow/v5/server/model"
)

func (s storage) NotificationConfigFind(repoID int64) (*model.NotificationConfig, error) {
	config := new(model.NotificationConfig)
	return config, wrapGet(s.engine.Where("repo_id = ?", repoID).Get(config))
}

func (s storage) NotificationConfigUpsert(config *model.NotificationConfig) error {
	existing := new(model.NotificationConfig)
	has, err := s.engine.Where("repo_id = ?", config.RepoID).Get(existing)
	if err != nil {
		return err
	}

	if has {
		config.ID = existing.ID
		_, err = s.engine.ID(config.ID).AllCols().Update(config)
		return err
	}

	_, err = s.engine.Insert(config)
	return err
}

func (s storage) NotificationConfigDelete(repoID int64) error {
	return wrapDelete(s.engine.Where("repo_id = ?", repoID).Delete(new(model.NotificationConfig)))
}
