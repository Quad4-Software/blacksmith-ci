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
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

func testDriverConfig() (driver, config string) {
	driver = "sqlite3"
	config = ":memory:"

	if os.Getenv("WOODPECKER_DATABASE_DRIVER") != "" {
		driver = os.Getenv("WOODPECKER_DATABASE_DRIVER")
		config = os.Getenv("WOODPECKER_DATABASE_DATASOURCE")
	}
	return driver, config
}

// newTestStore creates a new database connection for testing purposes.
// The database driver and connection string are provided by
// environment variables, with fallback to in-memory sqlite.
func newTestStore(t *testing.T, tables ...any) (*storage, func()) {
	driver, dsn := testDriverConfig()

	sqldb, err := sql.Open(driver, dsn)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	db := bun.NewDB(sqldb, sqlitedialect.New())

	ctx := context.Background()
	for _, table := range tables {
		_, err := db.NewCreateTable().Model(table).IfNotExists().Exec(ctx)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
	}

	return &storage{
			db:     db,
			driver: driver,
		}, func() {
			ctx := context.Background()
			for _, table := range tables {
				_, _ = db.NewDropTable().Model(table).IfExists().Cascade().Exec(ctx)
			}
			if err := db.Close(); err != nil {
				t.Error(err)
				t.FailNow()
			}

			if driver == "mysql" || driver == "postgres" {
				time.Sleep(10 * time.Millisecond)
			}
		}
}
