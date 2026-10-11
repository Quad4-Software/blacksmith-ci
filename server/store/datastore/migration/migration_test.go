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
	assertIdentitySubjectsByteExact(t, db)

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
		"cache_entries", "maintenance_config", "user_forges", "user_identities", "redirections",
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
	// Use driver-appropriate auto-increment syntax so the post-migration
	// PostgreSQL sequence check (checkPostgresSequences) doesn't flag the
	// fixture tables as broken.
	var createAutoscalers, createTasks, createSteps string
	switch driver {
	case "mysql":
		createAutoscalers = `CREATE TABLE IF NOT EXISTS autoscalers (id BIGINT PRIMARY KEY AUTO_INCREMENT, pending_since INTEGER)`
		createTasks = `CREATE TABLE IF NOT EXISTS tasks (id VARCHAR(64) PRIMARY KEY, canceled INTEGER DEFAULT 0, canceled_at INTEGER DEFAULT 0)`
		createSteps = `CREATE TABLE IF NOT EXISTS steps (id BIGINT PRIMARY KEY AUTO_INCREMENT, original_step_id INTEGER DEFAULT 0, attempt INTEGER DEFAULT 1)`
	case "postgres":
		createAutoscalers = `CREATE TABLE IF NOT EXISTS autoscalers (id BIGSERIAL PRIMARY KEY, pending_since INTEGER)`
		createTasks = `CREATE TABLE IF NOT EXISTS tasks (id TEXT PRIMARY KEY, canceled INTEGER DEFAULT 0, canceled_at INTEGER DEFAULT 0)`
		createSteps = `CREATE TABLE IF NOT EXISTS steps (id BIGSERIAL PRIMARY KEY, original_step_id INTEGER DEFAULT 0, attempt INTEGER DEFAULT 1)`
	default:
		createAutoscalers = `CREATE TABLE IF NOT EXISTS autoscalers (id INTEGER PRIMARY KEY, pending_since INTEGER)`
		createTasks = `CREATE TABLE IF NOT EXISTS tasks (id INTEGER PRIMARY KEY, canceled INTEGER DEFAULT 0, canceled_at INTEGER DEFAULT 0)`
		createSteps = `CREATE TABLE IF NOT EXISTS steps (id INTEGER PRIMARY KEY, original_step_id INTEGER DEFAULT 0, attempt INTEGER DEFAULT 1)`
	}
	_, err = db.Exec(createAutoscalers)
	require.NoError(t, err)
	_, err = db.Exec(createTasks)
	require.NoError(t, err)
	_, err = db.Exec(createSteps)
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

	// Verify user_identities was created
	err = db.QueryRow("SELECT COUNT(*) FROM user_identities").Scan(&count)
	assert.NoError(t, err, "user_identities table should exist")
	assertIdentitySubjectsByteExact(t, db)

	// Verify pending_since column exists on autoscalers
	rows, err := db.Query("SELECT pending_since FROM autoscalers LIMIT 1")
	if assert.NoError(t, err, "pending_since column should exist on autoscalers") {
		defer rows.Close()
		_ = rows.Err()
	}
}

// assertIdentitySubjectsByteExact guards against identity confusion: OIDC
// subjects are compared byte by byte, so subjects that differ only in case or
// in trailing spaces must be distinct identities, including under MySQL's
// case-insensitive default collation and the PAD SPACE binary collations.
func assertIdentitySubjectsByteExact(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO user_identities (user_id, provider, issuer, subject, linked_at) VALUES (1, 'oidc', 'https://idp.example', 'Alice', 1)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO user_identities (user_id, provider, issuer, subject, linked_at) VALUES (2, 'oidc', 'https://idp.example', 'alice', 1)`)
	require.NoError(t, err, "subjects that differ only in case must not collide")
	_, err = db.Exec(`INSERT INTO user_identities (user_id, provider, issuer, subject, linked_at) VALUES (3, 'oidc', 'https://idp.example', 'alice ', 1)`)
	require.NoError(t, err, "subjects that differ only in trailing spaces must not collide")

	var userID int64
	require.NoError(t, db.QueryRow(`SELECT user_id FROM user_identities WHERE provider = 'oidc' AND issuer = 'https://idp.example' AND subject = 'alice'`).Scan(&userID))
	assert.Equal(t, int64(2), userID)
	require.NoError(t, db.QueryRow(`SELECT user_id FROM user_identities WHERE provider = 'oidc' AND issuer = 'https://idp.example' AND subject = 'alice '`).Scan(&userID))
	assert.Equal(t, int64(3), userID)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM user_identities WHERE issuer = 'https://IDP.example'`).Scan(&count))
	assert.Zero(t, count, "issuers must compare byte-exact")
}

// TestMigrate_RepairsBrokenMySQLAutoIncrement simulates the MySQL analog of
// the bug in TestMigrate_RepairsBrokenPostgresIDSequence: the buggy migration
// window left email_unsubscribes as `id INT PRIMARY KEY` (no AUTO_INCREMENT).
// The new `crow-repair-mysql-id-auto-increment` migration must restore the
// AUTO_INCREMENT attribute so inserts without an explicit id succeed and the
// counter advances past the existing row.
func TestMigrate_RepairsBrokenMySQLAutoIncrement(t *testing.T) {
	db, driver := testDB(t)
	if driver != "mysql" {
		t.Skip("repair is a no-op outside MySQL")
	}

	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS migrations (id TEXT NULL)`)
	require.NoError(t, err)
	for _, id := range allLegacyMigrationIDs {
		_, err = db.Exec(fmt.Sprintf("INSERT INTO migrations (id) VALUES ('%s')", id))
		require.NoError(t, err)
	}
	_, err = db.Exec("INSERT INTO migrations (id) VALUES ('crow-add-email-unsubscribes')")
	require.NoError(t, err)

	// Fixtures other new migrations expect.
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS autoscalers (id BIGINT PRIMARY KEY AUTO_INCREMENT)`)
	require.NoError(t, err)

	// Recreate the broken historical schema on MySQL: id is INT PRIMARY KEY
	// with no AUTO_INCREMENT, no default, NOT NULL by virtue of PRIMARY KEY.
	_, err = db.Exec(`CREATE TABLE email_unsubscribes (
		id INT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		repo_id INTEGER NOT NULL,
		email TEXT NOT NULL,
		created_at INTEGER,
		UNIQUE(user_id, repo_id)
	)`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO email_unsubscribes (id, user_id, repo_id, email) VALUES (42, 1, 1, 'seed@example.com')`)
	require.NoError(t, err)

	// Sanity check: in the broken state, INSERT without an id fails because
	// the column has no AUTO_INCREMENT and no default.
	_, err = db.Exec(`INSERT INTO email_unsubscribes (user_id, repo_id, email) VALUES (2, 2, 'pre@example.com')`)
	require.Error(t, err, "broken table must reject inserts without an explicit id")

	err = Migrate(t.Context(), db, driver, true)
	require.NoError(t, err)

	// Post-repair: INSERT without id works and the counter advances past 42.
	res, err := db.Exec(`INSERT INTO email_unsubscribes (user_id, repo_id, email) VALUES (3, 3, 'post@example.com')`)
	require.NoError(t, err)
	newID, err := res.LastInsertId()
	require.NoError(t, err)
	assert.Greater(t, newID, int64(42), "repaired AUTO_INCREMENT must skip past existing max id")
}

// TestMigrate_RepairsBrokenPostgresIDSequence simulates the production failure
// mode from May 2026: an existing PostgreSQL database where
// `crow-add-email-unsubscribes` was applied during the window when it created
// the table with `id INTEGER PRIMARY KEY` (no sequence). The new
// `crow-repair-postgres-id-sequences` migration must restore the sequence so
// inserts without an explicit id succeed and the post-migration sequence
// check passes.
func TestMigrate_RepairsBrokenPostgresIDSequence(t *testing.T) {
	db, driver := testDB(t)
	if driver != "postgres" {
		t.Skip("repair is a no-op outside PostgreSQL")
	}

	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS migrations (id TEXT NULL)`)
	require.NoError(t, err)
	for _, id := range allLegacyMigrationIDs {
		_, err = db.Exec(fmt.Sprintf("INSERT INTO migrations (id) VALUES ('%s')", id))
		require.NoError(t, err)
	}
	// Pretend the buggy email_unsubscribes migration already ran.
	_, err = db.Exec("INSERT INTO migrations (id) VALUES ('crow-add-email-unsubscribes')")
	require.NoError(t, err)

	// Fixtures other new migrations need.
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS autoscalers (id BIGSERIAL PRIMARY KEY)`)
	require.NoError(t, err)

	// Recreate the broken historical schema: id is a plain INTEGER PRIMARY KEY
	// with no DEFAULT and no backing sequence.
	_, err = db.Exec(`CREATE TABLE email_unsubscribes (
		id INTEGER PRIMARY KEY,
		user_id INTEGER NOT NULL,
		repo_id INTEGER NOT NULL,
		email TEXT NOT NULL,
		created_at INTEGER,
		UNIQUE(user_id, repo_id)
	)`)
	require.NoError(t, err)

	// Seed with an explicit id so the repair has a row to skip past.
	_, err = db.Exec(`INSERT INTO email_unsubscribes (id, user_id, repo_id, email) VALUES (42, 1, 1, 'seed@example.com')`)
	require.NoError(t, err)

	// Sanity check: in the broken state, INSERT without an id violates NOT NULL.
	_, err = db.Exec(`INSERT INTO email_unsubscribes (user_id, repo_id, email) VALUES (2, 2, 'pre@example.com')`)
	require.Error(t, err, "broken table must reject inserts without an explicit id")

	err = Migrate(t.Context(), db, driver, true)
	require.NoError(t, err)

	// Post-repair: INSERT without id works and the sequence advances past the seeded row.
	var newID int
	err = db.QueryRow(`INSERT INTO email_unsubscribes (user_id, repo_id, email) VALUES (3, 3, 'post@example.com') RETURNING id`).Scan(&newID)
	require.NoError(t, err)
	assert.Greater(t, newID, 42, "repaired sequence must skip past existing max id")
}

// TestTableExists_EmptyTable guards against a regression where tableExists
// treated a Scan returning sql.ErrNoRows (empty table) as "table does not
// exist". That bug silently turned crow-fix-proc-log-int64-columns into a
// no-op on every deployed cluster, because proc_log_entries was always empty
// at migration time.
func TestTableExists_EmptyTable(t *testing.T) {
	db, _ := testDB(t)

	_, err := db.Exec(`CREATE TABLE table_exists_probe (id INTEGER PRIMARY KEY)`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	assert.True(t, tableExists(tx, "table_exists_probe"), "empty table must report as existing")
	assert.False(t, tableExists(tx, "definitely_does_not_exist"), "unknown table must report as missing")
}

// TestAddLogEntriesUniqueIndex_DedupsAndIndexes verifies that the migration
// removes only the surplus duplicate (step_id, line) rows (keeping the lowest
// id per group), leaves unique rows untouched, and then successfully creates
// the UNIQUE index. It also guards against the regression where the dedup used
// a whole-table `id NOT IN (SELECT MIN(id) ... GROUP BY ...)` anti-join that the
// planner executed as O(n^2), hanging startup on large log_entries tables.
func TestAddLogEntriesUniqueIndex_DedupsAndIndexes(t *testing.T) {
	db, driver := testDB(t)

	_, err := db.Exec(`CREATE TABLE log_entries (
		id INTEGER PRIMARY KEY,
		step_id INTEGER,
		line INTEGER,
		data TEXT
	)`)
	require.NoError(t, err)

	// Two duplicate groups plus unique rows. The lowest id in each duplicate
	// group must survive; the rest must be deleted.
	_, err = db.Exec(`INSERT INTO log_entries (id, step_id, line, data) VALUES
		(1, 10, 0, 'keep'),
		(2, 10, 0, 'dup'),
		(3, 10, 0, 'dup'),
		(4, 10, 1, 'unique'),
		(5, 20, 0, 'keep'),
		(6, 20, 0, 'dup'),
		(7, 20, 1, 'unique')`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)

	require.NoError(t, addLogEntriesUniqueIndex(driver)(tx))
	require.NoError(t, tx.Commit())

	// Surviving ids are the lowest per (step_id, line) group.
	rows, err := db.Query(`SELECT id FROM log_entries ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()
	var got []int
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		got = append(got, id)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []int{1, 4, 5, 7}, got, "only surplus duplicates should be deleted")

	// The UNIQUE index now exists and enforces (step_id, line).
	_, err = db.Exec(`INSERT INTO log_entries (id, step_id, line, data) VALUES (99, 10, 0, 'conflict')`)
	assert.Error(t, err, "unique index must reject a duplicate (step_id, line)")
}

func TestAddPipelineOrgIdentity_Backfill(t *testing.T) {
	db, driver := testDB(t)

	_, err := db.Exec(`CREATE TABLE repos (id INTEGER PRIMARY KEY, org_id INTEGER, owner TEXT, full_name TEXT)`)
	require.NoError(t, err)
	// Pre-migration pipelines table: no org identity columns yet.
	_, err = db.Exec(`CREATE TABLE pipelines (id INTEGER PRIMARY KEY, repo_id INTEGER, created INTEGER)`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO repos (id, org_id, owner, full_name) VALUES (1, 5, 'acme', 'acme/test')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO pipelines (id, repo_id) VALUES (1, 1), (2, 1), (3, 999)`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, addPipelineOrgIdentity(driver)(tx))
	require.NoError(t, tx.Commit())

	var orgID sql.NullInt64
	var orgName, fullName sql.NullString
	require.NoError(t, db.QueryRow(`SELECT org_id, org_name, repo_full_name FROM pipelines WHERE id = 1`).Scan(&orgID, &orgName, &fullName))
	assert.Equal(t, int64(5), orgID.Int64)
	assert.Equal(t, "acme", orgName.String)
	assert.Equal(t, "acme/test", fullName.String)

	// The orphaned pipeline (no matching repo) keeps NULL identity.
	require.NoError(t, db.QueryRow(`SELECT org_id FROM pipelines WHERE id = 3`).Scan(&orgID))
	assert.False(t, orgID.Valid, "orphaned pipeline must not be backfilled")
}

func TestBackfillWorkflowAgentIdentity(t *testing.T) {
	db, driver := testDB(t)

	_, err := db.Exec(`CREATE TABLE agents (id INTEGER PRIMARY KEY, name TEXT, persistent_id TEXT, platform TEXT)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE workflows (id INTEGER PRIMARY KEY, agent_id INTEGER, agent_persistent_id TEXT, agent_name TEXT, platform TEXT)`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO agents (id, name, persistent_id, platform) VALUES (1, 'artemis', 'pid-1', 'linux/amd64')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO workflows (id, agent_id, agent_persistent_id, agent_name, platform) VALUES
		(1, 1, '', '', ''),
		(2, 2, '', '', ''),
		(3, 1, 'pid-existing', 'already', 'plat')`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, backfillWorkflowAgentIdentity(driver)(tx))
	require.NoError(t, tx.Commit())

	var name, pid string
	// Legacy workflow with an existing agent is backfilled.
	require.NoError(t, db.QueryRow(`SELECT agent_name, agent_persistent_id FROM workflows WHERE id = 1`).Scan(&name, &pid))
	assert.Equal(t, "artemis", name)
	assert.Equal(t, "pid-1", pid)

	// Legacy workflow whose agent is gone cannot be recovered.
	require.NoError(t, db.QueryRow(`SELECT agent_name FROM workflows WHERE id = 2`).Scan(&name))
	assert.Equal(t, "", name)

	// A workflow that already carries a persistent id is left untouched.
	require.NoError(t, db.QueryRow(`SELECT agent_name FROM workflows WHERE id = 3`).Scan(&name))
	assert.Equal(t, "already", name)
}
