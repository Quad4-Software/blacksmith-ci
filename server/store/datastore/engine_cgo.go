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

//go:build cgo

package datastore

import (
	"database/sql"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"

	"codefloe.com/crowci/crow/v5/server/store"
)

// newBunDB creates a *bun.DB with the appropriate dialect for the driver.
func newBunDB(sqldb *sql.DB, driver string) *bun.DB {
	switch driver {
	case DriverSqlite:
		return bun.NewDB(sqldb, sqlitedialect.New())
	case DriverMysql:
		return bun.NewDB(sqldb, mysqldialect.New())
	case DriverPostgres:
		return bun.NewDB(sqldb, pgdialect.New())
	default:
		return bun.NewDB(sqldb, pgdialect.New())
	}
}

// configureSQL applies driver-specific configuration to the underlying sql.DB.
// For SQLite (CGO builds), it applies optimizations to prevent locking issues.
func configureSQL(sqldb *sql.DB, opts *store.Opts) error {
	if opts.Driver == DriverSqlite {
		sqldb.SetMaxOpenConns(1)
		sqldb.SetMaxIdleConns(1)
		sqldb.SetConnMaxLifetime(0)

		if err := applySQLitePragmas(sqldb); err != nil {
			return err
		}
	} else {
		sqldb.SetMaxOpenConns(opts.DB.MaxOpenConns)
		sqldb.SetMaxIdleConns(opts.DB.MaxIdleConns)
		sqldb.SetConnMaxLifetime(opts.DB.ConnMaxLifetime)
	}

	return nil
}

// applySQLitePragmas sets recommended SQLite pragmas for better concurrency.
func applySQLitePragmas(sqldb *sql.DB) error {
	pragmas := []string{
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
	}

	for _, pragma := range pragmas {
		if _, err := sqldb.Exec(pragma); err != nil {
			return err
		}
	}

	return nil
}
