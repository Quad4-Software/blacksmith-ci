// Copyright 2023 Woodpecker Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
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

func (s storage) ServerConfigGet(key string) (string, error) {
	config := new(model.ServerConfig)
	err := wrapGet(s.db.NewSelect().Model(config).Where("\"key\" = ?", key).Scan(context.Background()))
	if err != nil {
		return "", err
	}

	return config.Value, nil
}

func (s storage) ServerConfigSet(key, value string) error {
	return s.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		count, err := tx.NewSelect().Model((*model.ServerConfig)(nil)).Where("\"key\" = ?", key).Count(ctx)
		if err != nil {
			return err
		}

		config := &model.ServerConfig{
			Key:   key,
			Value: value,
		}

		if count == 0 {
			_, err = tx.NewInsert().Model(config).Exec(ctx)
		} else {
			_, err = tx.NewUpdate().Model(config).Column("value").Where("\"key\" = ?", key).Exec(ctx)
		}
		return err
	})
}

func (s storage) ServerConfigDelete(key string) error {
	return wrapDelete(s.db.NewDelete().Model((*model.ServerConfig)(nil)).Where("\"key\" = ?", key).Exec(context.Background()))
}
