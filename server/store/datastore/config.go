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
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store/types"
)

func (s storage) ConfigsForPipeline(pipelineID int64) ([]*model.Config, error) {
	ctx := context.Background()
	configs := make([]*model.Config, 0, perPage)

	// Check if the pipeline_configs table exists by attempting the query
	// If the table doesn't exist, the query will fail
	err := s.db.NewSelect().
		Model(&configs).
		Join("LEFT JOIN pipeline_configs ON config.id = pipeline_configs.config_id").
		Where("pipeline_configs.pipeline_id = ?", pipelineID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("error querying pipeline configs: %w", err)
	}

	return configs, nil
}

func (s storage) configFindIdentical(ctx context.Context, db bun.IDB, repoID int64, hash, name string) (*model.Config, error) {
	conf := new(model.Config)
	if err := wrapGet(db.NewSelect().Model(conf).
		Where("repo_id = ?", repoID).
		Where("hash = ?", hash).
		Where("name = ?", name).
		Scan(ctx)); err != nil {
		return nil, err
	}
	return conf, nil
}

func (s storage) ConfigPersist(conf *model.Config) (*model.Config, error) {
	conf.Hash = fmt.Sprintf("%x", sha256.Sum256(conf.Data))

	var result *model.Config
	err := s.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		existingConfig, err := s.configFindIdentical(ctx, tx, conf.RepoID, conf.Hash, conf.Name)
		if err != nil && !errors.Is(err, types.RecordNotExist) {
			return err
		}
		if existingConfig != nil {
			result = existingConfig
			return nil
		}

		if err := s.configCreate(ctx, tx, conf); err != nil {
			return err
		}

		result = conf
		return nil
	})

	return result, err
}

func (s storage) configCreate(ctx context.Context, db bun.IDB, config *model.Config) error {
	// should never happen but just in case
	if config.Name == "" {
		return fmt.Errorf("insert config to store failed: 'Name' has to be set")
	}
	if config.Hash == "" {
		return fmt.Errorf("insert config to store failed: 'Hash' has to be set")
	}

	// only Insert set auto created ID back to object
	_, err := db.NewInsert().Model(config).Exec(ctx)
	return err
}

func (s storage) PipelineConfigCreate(config *model.PipelineConfig) error {
	// only Insert set auto created ID back to object
	_, err := s.db.NewInsert().Model(config).Exec(context.Background())
	return err
}
