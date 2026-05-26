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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCheckPostgresSequences_NoOpOnNonPostgres ensures the check is a no-op
// for SQLite/MySQL where the issue doesn't apply.
func TestCheckPostgresSequences_NoOpOnNonPostgres(t *testing.T) {
	db := openSQLiteMemory(t)

	require.NoError(t, checkPostgresSequences(db, "sqlite3"))
	require.NoError(t, checkPostgresSequences(db, "mysql"))
}

// TestCheckPostgresSequences_HealthyTablePass verifies the check passes when
// id columns have a proper nextval default (BIGSERIAL).
func TestCheckPostgresSequences_HealthyTablePass(t *testing.T) {
	db, driver := testDB(t)
	if driver != "postgres" {
		t.Skipf("postgres-only check, got driver=%s", driver)
	}

	_, err := db.Exec(`CREATE TABLE autoscalers (id BIGSERIAL PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE agents (id BIGSERIAL PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)

	require.NoError(t, checkPostgresSequences(db, "postgres"))
}

// TestCheckPostgresSequences_BrokenTableFails reproduces the pgloader-style
// corruption from issue #1067: id is bigint NOT NULL but has no sequence/default.
// The check must surface the affected tables and include actionable repair SQL.
func TestCheckPostgresSequences_BrokenTableFails(t *testing.T) {
	db, driver := testDB(t)
	if driver != "postgres" {
		t.Skipf("postgres-only check, got driver=%s", driver)
	}

	// Healthy: agents has a sequence
	_, err := db.Exec(`CREATE TABLE agents (id BIGSERIAL PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)
	// Broken: autoscalers has bigint NOT NULL with no default (pgloader-style)
	_, err = db.Exec(`CREATE TABLE autoscalers (id BIGINT PRIMARY KEY NOT NULL, name TEXT)`)
	require.NoError(t, err)

	err = checkPostgresSequences(db, "postgres")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "autoscalers")
	// Must mention the table that's broken AND include the repair SQL keywords
	assert.Contains(t, err.Error(), "CREATE SEQUENCE")
	assert.Contains(t, err.Error(), "ALTER TABLE")
	// Must NOT incorrectly flag healthy tables
	assert.NotContains(t, err.Error(), "CREATE SEQUENCE IF NOT EXISTS agents_id_seq")
}

// TestCheckPostgresSequences_RepairRestores verifies that running the repair
// SQL embedded in the error message actually fixes the issue.
func TestCheckPostgresSequences_RepairRestores(t *testing.T) {
	db, driver := testDB(t)
	if driver != "postgres" {
		t.Skipf("postgres-only check, got driver=%s", driver)
	}

	_, err := db.Exec(`CREATE TABLE autoscalers (id BIGINT PRIMARY KEY NOT NULL, name TEXT)`)
	require.NoError(t, err)

	require.Error(t, checkPostgresSequences(db, "postgres"))

	// Apply the documented repair
	_, err = db.Exec(`CREATE SEQUENCE IF NOT EXISTS autoscalers_id_seq OWNED BY autoscalers.id`)
	require.NoError(t, err)
	_, err = db.Exec(`SELECT setval('autoscalers_id_seq', COALESCE((SELECT MAX(id) FROM autoscalers), 0) + 1, false)`)
	require.NoError(t, err)
	_, err = db.Exec(`ALTER TABLE autoscalers ALTER COLUMN id SET DEFAULT nextval('autoscalers_id_seq')`)
	require.NoError(t, err)

	require.NoError(t, checkPostgresSequences(db, "postgres"))
}

// TestCheckPostgresSequences_SkipsMissingTables verifies that the check
// ignores tables that don't exist (e.g. on a partial schema).
func TestCheckPostgresSequences_SkipsMissingTables(t *testing.T) {
	db, driver := testDB(t)
	if driver != "postgres" {
		t.Skipf("postgres-only check, got driver=%s", driver)
	}

	// No tables created — check should not error
	require.NoError(t, checkPostgresSequences(db, "postgres"))
}
