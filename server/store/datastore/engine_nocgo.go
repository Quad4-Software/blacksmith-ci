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

//go:build !cgo

package datastore

import (
	"database/sql"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"

	"codefloe.com/crowci/crow/v6/server/store"
)

// newBunDB creates a *bun.DB with the appropriate dialect for the driver.
func newBunDB(sqldb *sql.DB, driver string) *bun.DB {
	switch driver {
	case DriverMysql:
		return bun.NewDB(sqldb, mysqldialect.New())
	case DriverPostgres:
		return bun.NewDB(sqldb, pgdialect.New())
	default:
		return bun.NewDB(sqldb, pgdialect.New())
	}
}

// configureSQL applies driver-specific configuration.
// For non-CGO builds (no SQLite support), just apply standard connection settings.
func configureSQL(sqldb *sql.DB, opts *store.Opts) error {
	sqldb.SetMaxOpenConns(opts.DB.MaxOpenConns)
	sqldb.SetMaxIdleConns(opts.DB.MaxIdleConns)
	sqldb.SetConnMaxLifetime(opts.DB.ConnMaxLifetime)

	return nil
}
