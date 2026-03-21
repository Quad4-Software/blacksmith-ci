// Copyright 2025 Crow Authors
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

package migration

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func createMigrationsTable(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS migrations (id TEXT NULL)`)
	require.NoError(t, err)
}

func insertMigrations(t *testing.T, db *sql.DB, ids ...string) {
	t.Helper()
	for _, id := range ids {
		_, err := db.Exec("INSERT INTO migrations (id) VALUES (?)", id)
		require.NoError(t, err)
	}
}

func TestHasAnyMigrationFrom(t *testing.T) {
	tests := []struct {
		name              string
		appliedMigrations []string
		migrationSet      []string
		expected          bool
	}{
		{"empty applied, empty set", []string{}, []string{}, false},
		{"empty applied, non-empty set", []string{}, []string{"add-org-id"}, false},
		{"no match", []string{"unknown-1"}, []string{"add-org-id"}, false},
		{"match", []string{"add-org-id", "unknown"}, []string{"add-org-id", "set-forge-id"}, true},
		{"multiple matches", []string{"add-org-id", "set-forge-id"}, []string{"add-org-id", "set-forge-id"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, hasAnyMigrationFrom(tt.appliedMigrations, tt.migrationSet))
		})
	}
}

func TestDetectDatabaseOrigin_FreshDatabase(t *testing.T) {
	db := openTestDB(t)

	info, err := detectDatabaseOriginSQL(db)
	require.NoError(t, err)
	assert.Equal(t, OriginNew, info.Origin)
	assert.Empty(t, info.AppliedMigrations)
	assert.False(t, info.IsFirstCrowMigration)
}

func TestDetectDatabaseOrigin_EmptyMigrationsTable(t *testing.T) {
	db := openTestDB(t)
	createMigrationsTable(t, db)

	info, err := detectDatabaseOriginSQL(db)
	require.NoError(t, err)
	assert.Equal(t, OriginNew, info.Origin)
	assert.Empty(t, info.AppliedMigrations)
}

func TestDetectDatabaseOrigin_WoodpeckerDatabase(t *testing.T) {
	db := openTestDB(t)
	createMigrationsTable(t, db)
	insertMigrations(t, db, "legacy-to-xormigrate", "add-org-id", "set-forge-id")

	info, err := detectDatabaseOriginSQL(db)
	require.NoError(t, err)
	assert.Equal(t, OriginWoodpecker, info.Origin)
	assert.Len(t, info.AppliedMigrations, 3)
	assert.Empty(t, info.UnknownMigrations)
	assert.True(t, info.IsFirstCrowMigration)
}

func TestDetectDatabaseOrigin_CrowDatabase(t *testing.T) {
	db := openTestDB(t)
	createMigrationsTable(t, db)
	insertMigrations(t, db, "add-org-id", "add-distributed-locks", "add-autoscalers")

	info, err := detectDatabaseOriginSQL(db)
	require.NoError(t, err)
	assert.Equal(t, OriginCrow, info.Origin)
	assert.Len(t, info.AppliedMigrations, 3)
	assert.Empty(t, info.UnknownMigrations)
	assert.False(t, info.IsFirstCrowMigration)
}

func TestDetectDatabaseOrigin_UnknownOrigin(t *testing.T) {
	db := openTestDB(t)
	createMigrationsTable(t, db)
	insertMigrations(t, db, "completely-unknown-1", "completely-unknown-2")

	info, err := detectDatabaseOriginSQL(db)
	require.NoError(t, err)
	assert.Equal(t, OriginUnknown, info.Origin)
	assert.Len(t, info.UnknownMigrations, 2)
}

func TestDetectDatabaseOrigin_MixedWithUnknownMigrations(t *testing.T) {
	db := openTestDB(t)
	createMigrationsTable(t, db)
	insertMigrations(t, db, "add-org-id", "woodpecker-new-feature")

	info, err := detectDatabaseOriginSQL(db)
	require.NoError(t, err)
	assert.Equal(t, OriginWoodpecker, info.Origin)
	assert.Contains(t, info.UnknownMigrations, "woodpecker-new-feature")
	assert.True(t, info.IsFirstCrowMigration)
}

func TestDetectDatabaseOrigin_CrowWithUnknownMigrations(t *testing.T) {
	db := openTestDB(t)
	createMigrationsTable(t, db)
	insertMigrations(t, db, "add-org-id", "add-autoscalers", "future-crow-feature")

	info, err := detectDatabaseOriginSQL(db)
	require.NoError(t, err)
	assert.Equal(t, OriginCrow, info.Origin)
	assert.Contains(t, info.UnknownMigrations, "future-crow-feature")
}

func TestAllKnownMigrations(t *testing.T) {
	known := allKnownMigrations()
	assert.Contains(t, known, "add-org-id")
	assert.Contains(t, known, "add-autoscalers")
	assert.Equal(t, len(sharedMigrations)+len(crowMigrations), len(known))
}

func TestConstantsConsistency(t *testing.T) {
	for _, shared := range sharedMigrations {
		assert.NotContains(t, crowMigrations, shared)
	}
	assert.Contains(t, sharedMigrations, "legacy-to-xormigrate")
	assert.Contains(t, crowMigrations, "add-distributed-locks")
}
