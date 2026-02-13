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
	"xorm.io/xorm"

	"codefloe.com/crowci/crow/v5/server/store"
)

// configureEngine applies driver-specific configuration.
// For SQLite (CGO builds), it applies optimizations to prevent locking issues.
func configureEngine(engine *xorm.Engine, opts *store.Opts) error {
	// SQLite works best with a single connection to avoid locking issues.
	// Multiple connections don't help since SQLite serializes writes anyway.
	if opts.Driver == DriverSqlite {
		engine.SetMaxOpenConns(1)
		engine.SetMaxIdleConns(1)
		engine.SetConnMaxLifetime(0) // Don't close the connection

		if err := applySQLitePragmas(engine); err != nil {
			return err
		}
	} else {
		engine.SetMaxOpenConns(opts.XORM.MaxOpenConns)
		engine.SetMaxIdleConns(opts.XORM.MaxIdleConns)
		engine.SetConnMaxLifetime(opts.XORM.ConnMaxLifetime)
	}

	return nil
}

// applySQLitePragmas sets recommended SQLite pragmas for better concurrency.
func applySQLitePragmas(engine *xorm.Engine) error {
	pragmas := []string{
		"PRAGMA busy_timeout = 5000;",  // Wait 5s before returning SQLITE_BUSY
		"PRAGMA journal_mode = WAL;",   // Write-Ahead Logging for better concurrency
		"PRAGMA synchronous = NORMAL;", // Good balance of safety and performance with WAL
	}

	for _, pragma := range pragmas {
		if _, err := engine.Exec(pragma); err != nil {
			return err
		}
	}

	return nil
}
