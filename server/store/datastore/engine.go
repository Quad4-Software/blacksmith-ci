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
	"database/sql"

	"github.com/rs/zerolog"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/extra/bundebug"

	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/datastore/migration"
)

type storage struct {
	db     *bun.DB
	driver string
}

const perPage = 50

func NewEngine(opts *store.Opts) (store.Store, error) {
	sqldb, err := sql.Open(opts.Driver, opts.Config)
	if err != nil {
		return nil, err
	}

	// Apply driver-specific configuration (connection pool, SQLite pragmas)
	if err := configureSQL(sqldb, opts); err != nil {
		return nil, err
	}

	// Select Bun dialect based on driver (build-tag aware)
	db := newBunDB(sqldb, opts.Driver)

	// Set PostgreSQL search_path if schema is specified
	if opts.Schema != "" && opts.Driver == DriverPostgres {
		if _, err := db.Exec("SET search_path TO " + opts.Schema); err != nil {
			return nil, err
		}
	}

	// Add query logging hook
	if opts.DB.Log || opts.DB.ShowSQL {
		db.AddQueryHook(bundebug.NewQueryHook(
			bundebug.WithEnabled(true),
			bundebug.WithVerbose(zerolog.GlobalLevel() <= zerolog.DebugLevel),
		))
	}

	return &storage{
		db:     db,
		driver: opts.Driver,
	}, nil
}

func (s storage) Ping() error {
	return s.db.Ping()
}

// Migrate old storage or init new one.
func (s storage) Migrate(ctx context.Context, allowLong bool) error {
	return migration.Migrate(ctx, s.db.DB, s.driver, allowLong)
}

func (s storage) Close() error {
	return s.db.Close()
}

// DB returns the underlying *bun.DB for use in tests.
func (s storage) DB() *bun.DB {
	return s.db
}

// quoteIdent quotes an identifier (column/table name) for the current dialect.
func (s storage) quoteIdent(name string) string {
	switch s.driver {
	case DriverMysql:
		return "`" + name + "`"
	default:
		return `"` + name + `"`
	}
}
