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

package migration

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- SQLite-only tests ---
// These test migration logic (legacy table handling, completeness checks)
// and use SQLite because they need to simulate specific migration table states.

func openSQLiteMemory(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrate_FreshDatabase(t *testing.T) {
	db := openSQLiteMemory(t)

	err := Migrate(t.Context(), db, "sqlite3", true)
	require.NoError(t, err)

	// Verify that baseline tables were created
	tables := freshDatabaseTables()
	for _, table := range tables {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		assert.NoError(t, err, "table %s should exist", table)
	}

	// Verify all legacy migration IDs were marked as applied
	var migrationCount int
	err = db.QueryRow("SELECT COUNT(*) FROM migrations").Scan(&migrationCount)
	require.NoError(t, err)
	assert.Greater(t, migrationCount, 0, "legacy migrations should be marked as applied")
}

// TestMigrate_FreshDatabase_MultiDriver runs the full baseline schema and
// migrations against whichever database is configured via env vars.
// On MySQL/Postgres this validates that every CREATE TABLE statement —
// including FK constraints — uses compatible column types.
// This is the test that would have caught issue #945.
func TestMigrate_FreshDatabase_MultiDriver(t *testing.T) {
	db, driver := testDB(t)

	err := Migrate(t.Context(), db, driver, true)
	require.NoError(t, err)

	tables := freshDatabaseTables()
	for _, table := range tables {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		assert.NoError(t, err, "table %s should exist after fresh migration on %s", table, driver)
	}

	var migrationCount int
	err = db.QueryRow("SELECT COUNT(*) FROM migrations").Scan(&migrationCount)
	require.NoError(t, err)
	assert.Greater(t, migrationCount, 0, "legacy migrations should be marked as applied")
}

func freshDatabaseTables() []string {
	return []string{
		"users", "repos", "pipelines", "workflows", "steps", "tasks",
		"log_entries", "configs", "pipeline_configs", "secrets", "secret_targets",
		"registries", "perms", "agents", "autoscalers", "crons", "orgs", "forges",
		"server_configs", "access_tokens", "integrations", "notification_configs",
		"approved_pull_requests", "distributed_locks", "pubsub_messages",
		"cache_entries", "maintenance_config", "user_forges", "redirections",
		"email_unsubscribes", "maintenance_operation_config",
		"maintenance_stats", "maintenance_logs",
	}
}

func TestMigrate_LegacyXormTable(t *testing.T) {
	db := openSQLiteMemory(t)

	// Simulate an XORM-based database: legacy "migration" table (singular),
	// no "migrations" table, and existing data tables.
	_, err := db.Exec(`CREATE TABLE migration (id TEXT NULL)`)
	require.NoError(t, err)
	for _, id := range allLegacyMigrationIDs {
		_, err = db.Exec("INSERT INTO migration (id) VALUES (?)", id)
		require.NoError(t, err)
	}
	// Create data tables to simulate existing database
	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO users (id) VALUES (1)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE autoscalers (id INTEGER PRIMARY KEY)`)
	require.NoError(t, err)

	// isFreshDatabase must return false
	r := newRunner(db)
	assert.False(t, r.isFreshDatabase(), "database with legacy migration table should not be detected as fresh")

	// Migrate should succeed and copy legacy entries to the new table
	err = Migrate(t.Context(), db, "sqlite3", true)
	require.NoError(t, err)

	// Verify entries were copied to the new "migrations" table
	// (legacy IDs + any new migrations that ran)
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM migrations").Scan(&count)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, len(allLegacyMigrationIDs))
}

func TestMigrate_ExistingDatabase_AllLegacyApplied(t *testing.T) {
	db := openSQLiteMemory(t)

	// Simulate an existing database with ALL legacy migrations applied
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS migrations (id TEXT NULL)`)
	require.NoError(t, err)
	for _, id := range allLegacyMigrationIDs {
		_, err = db.Exec("INSERT INTO migrations (id) VALUES (?)", id)
		require.NoError(t, err)
	}
	// Create tables that new migrations will alter
	_, err = db.Exec(`CREATE TABLE autoscalers (id INTEGER PRIMARY KEY)`)
	require.NoError(t, err)

	err = Migrate(t.Context(), db, "sqlite3", true)
	require.NoError(t, err)
}

func TestMigrate_ExistingDatabase_MissingSharedMigrations(t *testing.T) {
	db := openSQLiteMemory(t)

	// Simulate a v5.6 database created via InitSchema: it has SCHEMA_INIT and
	// all crow migrations, but some shared (Woodpecker-era) migration IDs like
	// "add-polling-instances" were never recorded because InitSchema skipped them.
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS migrations (id TEXT NULL)`)
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO migrations (id) VALUES (?)", "SCHEMA_INIT")
	require.NoError(t, err)
	// Add all crow migrations but skip some shared ones
	for _, id := range crowMigrations {
		_, err = db.Exec("INSERT INTO migrations (id) VALUES (?)", id)
		require.NoError(t, err)
	}
	// Create tables that new migrations will alter
	_, err = db.Exec(`CREATE TABLE autoscalers (id INTEGER PRIMARY KEY)`)
	require.NoError(t, err)

	// Should succeed — missing shared migrations are not checked
	err = Migrate(t.Context(), db, "sqlite3", true)
	require.NoError(t, err)
}

func TestMigrate_ExistingDatabase_IncompleteLegacy(t *testing.T) {
	db := openSQLiteMemory(t)

	// Simulate a database with only some legacy migrations applied
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS migrations (id TEXT NULL)`)
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO migrations (id) VALUES (?)", "legacy-to-xormigrate")
	require.NoError(t, err)

	// Should fail with an error telling the user to upgrade to v5.6 first
	err = Migrate(t.Context(), db, "sqlite3", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upgrade to v5.6 first")
}

// --- Multi-driver tests ---
// These run against whichever database is configured via env vars (SQLite, PostgreSQL, MySQL)
// and verify that new migration DDL is portable across all drivers.

// TestMigrate_NewMigrations_IdempotentRerun verifies that running migrations a
// second time (columns/tables already exist) doesn't cause "bad connection" errors.
// This is the exact scenario that broke MySQL: tableExists and addColumnIfNotExists
// opened result sets via tx.Query but issued further tx.Exec calls before closing
// them, triggering MySQL's single-open-result-set constraint.
func TestMigrate_NewMigrations_IdempotentRerun(t *testing.T) {
	db, driver := testDB(t)

	// Set up the migrations table and mark all legacy migrations as applied.
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS migrations (id TEXT NULL)`)
	require.NoError(t, err)
	for _, id := range allLegacyMigrationIDs {
		_, err = db.Exec(fmt.Sprintf("INSERT INTO migrations (id) VALUES ('%s')", id))
		require.NoError(t, err)
	}

	// Create tables with the columns that new migrations would add.
	// This simulates a database that already ran these migrations successfully.
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS autoscalers (id INTEGER PRIMARY KEY, pending_since INTEGER)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS tasks (id INTEGER PRIMARY KEY, canceled INTEGER DEFAULT 0, canceled_at INTEGER DEFAULT 0)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS steps (id INTEGER PRIMARY KEY, original_step_id INTEGER DEFAULT 0, attempt INTEGER DEFAULT 1)`)
	require.NoError(t, err)

	// First run — migrations execute but columns already exist, so
	// addColumnIfNotExists takes the "column exists" path.
	err = Migrate(t.Context(), db, driver, true)
	require.NoError(t, err)

	// Second run — all migration IDs are already recorded, nothing executes.
	// This confirms the migrations were properly recorded.
	err = Migrate(t.Context(), db, driver, true)
	require.NoError(t, err)
}

// TestMigrate_NewMigrations_PortableSQL verifies that new migrations use
// SQL syntax compatible with all supported databases.
// It simulates an existing database with all legacy migrations applied,
// then runs only the new migrations — this is the actual upgrade path
// that must work on PostgreSQL and MySQL.
func TestMigrate_NewMigrations_PortableSQL(t *testing.T) {
	db, driver := testDB(t)

	// Set up the migrations table and mark all legacy migrations as applied.
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS migrations (id TEXT NULL)`)
	require.NoError(t, err)
	for _, id := range allLegacyMigrationIDs {
		_, err = db.Exec(fmt.Sprintf("INSERT INTO migrations (id) VALUES ('%s')", id))
		require.NoError(t, err)
	}
	// Create tables that new migrations will alter or reference with FKs.
	// Use BIGINT for PKs to match the real schema — MySQL requires FK column
	// types to match the referenced PK type exactly.
	var createAutoscalers, createSecrets string
	switch driver {
	case "mysql":
		createAutoscalers = `CREATE TABLE IF NOT EXISTS autoscalers (id BIGINT PRIMARY KEY AUTO_INCREMENT)`
		createSecrets = `CREATE TABLE IF NOT EXISTS secrets (id BIGINT PRIMARY KEY AUTO_INCREMENT, org_id INTEGER NOT NULL DEFAULT 0, repo_id INTEGER NOT NULL DEFAULT 0, name VARCHAR(250) NOT NULL)`
	case "postgres":
		createAutoscalers = `CREATE TABLE IF NOT EXISTS autoscalers (id BIGSERIAL PRIMARY KEY)`
		createSecrets = `CREATE TABLE IF NOT EXISTS secrets (id BIGSERIAL PRIMARY KEY, org_id INTEGER NOT NULL DEFAULT 0, repo_id INTEGER NOT NULL DEFAULT 0, name TEXT NOT NULL)`
	default:
		createAutoscalers = `CREATE TABLE IF NOT EXISTS autoscalers (id INTEGER PRIMARY KEY)`
		createSecrets = `CREATE TABLE IF NOT EXISTS secrets (id INTEGER PRIMARY KEY, org_id INTEGER NOT NULL DEFAULT 0, repo_id INTEGER NOT NULL DEFAULT 0, name TEXT NOT NULL)`
	}
	_, err = db.Exec(createAutoscalers)
	require.NoError(t, err)
	_, err = db.Exec(createSecrets)
	require.NoError(t, err)
	// Insert a row so tableExists() sees a non-empty table and the
	// createSecretTargets migration actually runs the FK DDL.
	_, err = db.Exec(`INSERT INTO secrets (org_id, repo_id, name) VALUES (0, 1, 'test')`)
	require.NoError(t, err)

	// Run Migrate — should only execute new migrations, not the baseline
	err = Migrate(t.Context(), db, driver, true)
	require.NoError(t, err)

	// Verify new migration tables/columns were created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM email_unsubscribes").Scan(&count)
	assert.NoError(t, err, "email_unsubscribes table should exist")

	// Verify secret_targets was created with proper FK
	err = db.QueryRow("SELECT COUNT(*) FROM secret_targets").Scan(&count)
	assert.NoError(t, err, "secret_targets table should exist")

	// Verify pending_since column exists on autoscalers
	rows, err := db.Query("SELECT pending_since FROM autoscalers LIMIT 1")
	if assert.NoError(t, err, "pending_since column should exist on autoscalers") {
		defer rows.Close()
		_ = rows.Err()
	}
}
