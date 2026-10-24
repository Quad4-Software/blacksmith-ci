// Copyright 2026 Crow Authors
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

package migration

import (
	"database/sql"
	"os"
	"slices"
	"testing"

	// Import all database drivers so tests can run against any backend.
	_ "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

// TestMain registers the pgx driver as "postgres" (pgx only self-registers as
// "pgx" / "pgx/v5") and then runs the tests.
func TestMain(m *testing.M) {
	if !slices.Contains(sql.Drivers(), "postgres") {
		sql.Register("postgres", stdlib.GetDefaultDriver())
	}
	m.Run()
}

// testDB returns a database connection for testing.
// Uses CROW_DATABASE_DRIVER / CROW_DATABASE_DATASOURCE env vars if set,
// otherwise falls back to an in-memory SQLite database.
func testDB(t *testing.T) (*sql.DB, string) {
	t.Helper()

	driver := os.Getenv("CROW_DATABASE_DRIVER")
	dsn := os.Getenv("CROW_DATABASE_DATASOURCE")

	if driver == "" {
		driver = "sqlite3"
		dsn = ":memory:"
	}

	db, err := sql.Open(driver, dsn)
	require.NoError(t, err)

	if driver == "sqlite3" {
		db.SetMaxOpenConns(1) // SQLite in-memory requires single connection
	}

	t.Cleanup(func() {
		// Clean up tables created during tests (for shared database servers)
		if driver != "sqlite3" {
			cleanupTestTables(db)
		}
		db.Close()
	})

	return db, driver
}

// cleanupTestTables drops tables that migration tests create,
// so PostgreSQL/MySQL test databases stay clean between runs.
func cleanupTestTables(db *sql.DB) {
	// Drop tables with FK dependencies first, then the rest.
	tables := []string{
		"secret_targets", "pipeline_configs",
		"migrations",
		"migration",
		"users", "repos", "pipelines", "workflows", "steps", "tasks",
		"log_entries", "configs", "secrets", "registries",
		"perms", "agents", "autoscalers", "crons", "orgs", "forges",
		"server_configs", "access_tokens", "integrations",
		"approved_pull_requests", "distributed_locks", "pubsub_messages",
		"cache_entries", "maintenance_config", "maintenance_operation_config",
		"maintenance_stats", "maintenance_logs",
		"user_forges", "user_identities", "redirections", "build_caches",
		"pipeline_watches", "watch_workflow_states",
		"build_cache_settings", "build_cache_buckets", "build_cache_usage",
		"table_exists_probe",
	}
	for _, t := range tables {
		_, _ = db.Exec("DROP TABLE IF EXISTS " + t + " CASCADE")
	}
}
