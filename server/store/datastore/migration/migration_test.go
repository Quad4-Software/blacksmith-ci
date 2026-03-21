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
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrate_FreshDatabase(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1) // SQLite in-memory requires single connection

	err = Migrate(t.Context(), db, "sqlite3", true)
	require.NoError(t, err)

	// Verify that baseline tables were created
	tables := []string{
		"users", "repos", "pipelines", "workflows", "steps", "tasks",
		"log_entries", "configs", "pipeline_configs", "secrets", "registries",
		"perms", "agents", "autoscalers", "crons", "orgs", "forges",
		"server_configs", "access_tokens", "integrations", "notification_configs",
		"approved_pull_requests", "distributed_locks", "pubsub_messages",
		"cache_entries", "maintenance_config", "user_forges", "redirections",
	}
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

func TestMigrate_LegacyXormTable(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)

	// Simulate an XORM-based database: legacy "migration" table (singular),
	// no "migrations" table, and existing data tables.
	_, err = db.Exec(`CREATE TABLE migration (id TEXT NULL)`)
	require.NoError(t, err)
	for _, id := range allLegacyMigrationIDs {
		_, err = db.Exec("INSERT INTO migration (id) VALUES (?)", id)
		require.NoError(t, err)
	}
	// Create a data table to simulate existing database
	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO users (id) VALUES (1)`)
	require.NoError(t, err)

	// isFreshDatabase must return false
	r := newRunner(db)
	assert.False(t, r.isFreshDatabase(), "database with legacy migration table should not be detected as fresh")

	// Migrate should succeed and copy legacy entries to the new table
	err = Migrate(t.Context(), db, "sqlite3", true)
	require.NoError(t, err)

	// Verify entries were copied to the new "migrations" table
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM migrations").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, len(allLegacyMigrationIDs), count)
}

func TestMigrate_ExistingDatabase_AllLegacyApplied(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)

	// Simulate an existing database with ALL legacy migrations applied
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS migrations (id TEXT NULL)`)
	require.NoError(t, err)
	for _, id := range allLegacyMigrationIDs {
		_, err = db.Exec("INSERT INTO migrations (id) VALUES (?)", id)
		require.NoError(t, err)
	}

	err = Migrate(t.Context(), db, "sqlite3", true)
	require.NoError(t, err)
}

func TestMigrate_ExistingDatabase_IncompleteLegacy(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)

	// Simulate a database with only some legacy migrations applied
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS migrations (id TEXT NULL)`)
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO migrations (id) VALUES (?)", "legacy-to-xormigrate")
	require.NoError(t, err)

	// Should fail with an error telling the user to upgrade to v5.6 first
	err = Migrate(t.Context(), db, "sqlite3", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upgrade to v5.6 first")
}
