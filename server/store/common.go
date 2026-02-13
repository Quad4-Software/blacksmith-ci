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

package store

import (
	"time"

	"codefloe.com/crowci/crow/v5/server/model"
)

type XORM struct {
	Log             bool
	ShowSQL         bool
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
}

// Opts are options for a new database connection.
type Opts struct {
	Driver string
	Config string
	Schema string // Database schema name (PostgreSQL only)
	XORM   XORM
}

type PipelineListKeyset interface {
	GetPipelineListKeyset(repo *model.Repo, lastID, beforeDate int64, limit int) ([]*model.Pipeline, error)
	GetPipelineCount(repo *model.Repo) (int64, error)
}
