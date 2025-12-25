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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

func TestHasAnyMigrationFrom(t *testing.T) {
	tests := []struct {
		name              string
		appliedMigrations []string
		migrationSet      []string
		expected          bool
	}{
		{
			name:              "empty applied, empty set",
			appliedMigrations: []string{},
			migrationSet:      []string{},
			expected:          false,
		},
		{
			name:              "empty applied, non-empty set",
			appliedMigrations: []string{},
			migrationSet:      []string{"add-org-id", "set-forge-id"},
			expected:          false,
		},
		{
			name:              "non-empty applied, empty set",
			appliedMigrations: []string{"add-org-id", "set-forge-id"},
			migrationSet:      []string{},
			expected:          false,
		},
		{
			name:              "no match",
			appliedMigrations: []string{"unknown-migration-1", "unknown-migration-2"},
			migrationSet:      []string{"add-org-id", "set-forge-id"},
			expected:          false,
		},
		{
			name:              "first element matches",
			appliedMigrations: []string{"add-org-id", "unknown"},
			migrationSet:      []string{"add-org-id", "set-forge-id"},
			expected:          true,
		},
		{
			name:              "last element matches",
			appliedMigrations: []string{"unknown", "set-forge-id"},
			migrationSet:      []string{"add-org-id", "set-forge-id"},
			expected:          true,
		},
		{
			name:              "multiple matches",
			appliedMigrations: []string{"add-org-id", "set-forge-id", "add-autoscalers"},
			migrationSet:      []string{"add-org-id", "set-forge-id"},
			expected:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAnyMigrationFrom(tt.appliedMigrations, tt.migrationSet)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectDatabaseOrigin_FreshDatabase(t *testing.T) {
	engine, err := xorm.NewEngine("sqlite3", ":memory:")
	require.NoError(t, err)
	defer engine.Close()

	info, err := detectDatabaseOrigin(engine)
	require.NoError(t, err)

	assert.Equal(t, OriginNew, info.Origin)
	assert.Empty(t, info.AppliedMigrations)
	assert.Empty(t, info.UnknownMigrations)
	assert.False(t, info.IsFirstCrowMigration)
}

func TestDetectDatabaseOrigin_EmptyMigrationsTable(t *testing.T) {
	engine, err := xorm.NewEngine("sqlite3", ":memory:")
	require.NoError(t, err)
	defer engine.Close()

	// Create empty migrations table
	err = engine.Sync(&xormigrate.Migration{})
	require.NoError(t, err)

	info, err := detectDatabaseOrigin(engine)
	require.NoError(t, err)

	assert.Equal(t, OriginNew, info.Origin)
	assert.Empty(t, info.AppliedMigrations)
	assert.Empty(t, info.UnknownMigrations)
	assert.False(t, info.IsFirstCrowMigration)
}

func TestDetectDatabaseOrigin_WoodpeckerDatabase(t *testing.T) {
	engine, err := xorm.NewEngine("sqlite3", ":memory:")
	require.NoError(t, err)
	defer engine.Close()

	// Create migrations table and insert Woodpecker migrations (shared only)
	err = engine.Sync(&xormigrate.Migration{})
	require.NoError(t, err)

	woodpeckerMigrations := []xormigrate.Migration{
		{ID: "legacy-to-xormigrate"},
		{ID: "add-org-id"},
		{ID: "set-forge-id"},
		{ID: "unify-columns-tables"},
	}
	for _, m := range woodpeckerMigrations {
		_, err := engine.Insert(&m)
		require.NoError(t, err)
	}

	info, err := detectDatabaseOrigin(engine)
	require.NoError(t, err)

	assert.Equal(t, OriginWoodpecker, info.Origin)
	assert.Len(t, info.AppliedMigrations, 4)
	assert.Empty(t, info.UnknownMigrations)
	assert.True(t, info.IsFirstCrowMigration)
}

func TestDetectDatabaseOrigin_CrowDatabase(t *testing.T) {
	engine, err := xorm.NewEngine("sqlite3", ":memory:")
	require.NoError(t, err)
	defer engine.Close()

	// Create migrations table and insert both shared and Crow migrations
	err = engine.Sync(&xormigrate.Migration{})
	require.NoError(t, err)

	migrations := []xormigrate.Migration{
		// Shared migrations
		{ID: "legacy-to-xormigrate"},
		{ID: "add-org-id"},
		// Crow-specific migrations
		{ID: "add-distributed-locks"},
		{ID: "add-autoscalers"},
	}
	for _, m := range migrations {
		_, err := engine.Insert(&m)
		require.NoError(t, err)
	}

	info, err := detectDatabaseOrigin(engine)
	require.NoError(t, err)

	assert.Equal(t, OriginCrow, info.Origin)
	assert.Len(t, info.AppliedMigrations, 4)
	assert.Empty(t, info.UnknownMigrations)
	assert.False(t, info.IsFirstCrowMigration)
}

func TestDetectDatabaseOrigin_UnknownOrigin(t *testing.T) {
	engine, err := xorm.NewEngine("sqlite3", ":memory:")
	require.NoError(t, err)
	defer engine.Close()

	// Create migrations table with only unknown migrations
	err = engine.Sync(&xormigrate.Migration{})
	require.NoError(t, err)

	unknownMigrations := []xormigrate.Migration{
		{ID: "completely-unknown-migration-1"},
		{ID: "completely-unknown-migration-2"},
	}
	for _, m := range unknownMigrations {
		_, err := engine.Insert(&m)
		require.NoError(t, err)
	}

	info, err := detectDatabaseOrigin(engine)
	require.NoError(t, err)

	assert.Equal(t, OriginUnknown, info.Origin)
	assert.Len(t, info.AppliedMigrations, 2)
	assert.Len(t, info.UnknownMigrations, 2)
	assert.False(t, info.IsFirstCrowMigration)
}

func TestDetectDatabaseOrigin_MixedWithUnknownMigrations(t *testing.T) {
	engine, err := xorm.NewEngine("sqlite3", ":memory:")
	require.NoError(t, err)
	defer engine.Close()

	// Create migrations table with known and unknown migrations
	err = engine.Sync(&xormigrate.Migration{})
	require.NoError(t, err)

	migrations := []xormigrate.Migration{
		// Shared migrations
		{ID: "add-org-id"},
		// Unknown migrations (e.g., from newer Woodpecker)
		{ID: "woodpecker-new-feature"},
		{ID: "woodpecker-another-feature"},
	}
	for _, m := range migrations {
		_, err := engine.Insert(&m)
		require.NoError(t, err)
	}

	info, err := detectDatabaseOrigin(engine)
	require.NoError(t, err)

	// Should be detected as Woodpecker (has shared, no Crow)
	assert.Equal(t, OriginWoodpecker, info.Origin)
	assert.Len(t, info.AppliedMigrations, 3)
	// Should have 2 unknown migrations
	assert.Len(t, info.UnknownMigrations, 2)
	assert.Contains(t, info.UnknownMigrations, "woodpecker-new-feature")
	assert.Contains(t, info.UnknownMigrations, "woodpecker-another-feature")
	assert.True(t, info.IsFirstCrowMigration)
}

func TestDetectDatabaseOrigin_CrowWithUnknownMigrations(t *testing.T) {
	engine, err := xorm.NewEngine("sqlite3", ":memory:")
	require.NoError(t, err)
	defer engine.Close()

	// Create migrations table with Crow and unknown migrations
	err = engine.Sync(&xormigrate.Migration{})
	require.NoError(t, err)

	migrations := []xormigrate.Migration{
		// Shared migrations
		{ID: "add-org-id"},
		// Crow-specific migrations
		{ID: "add-autoscalers"},
		// Unknown migrations
		{ID: "future-crow-feature"},
	}
	for _, m := range migrations {
		_, err := engine.Insert(&m)
		require.NoError(t, err)
	}

	info, err := detectDatabaseOrigin(engine)
	require.NoError(t, err)

	// Should be detected as Crow (has Crow migrations)
	assert.Equal(t, OriginCrow, info.Origin)
	assert.Len(t, info.AppliedMigrations, 3)
	// Should have 1 unknown migration
	assert.Len(t, info.UnknownMigrations, 1)
	assert.Contains(t, info.UnknownMigrations, "future-crow-feature")
	assert.False(t, info.IsFirstCrowMigration)
}

func TestAllKnownMigrations(t *testing.T) {
	known := allKnownMigrations()

	// Should contain both shared and Crow migrations
	assert.Contains(t, known, "add-org-id")            // shared
	assert.Contains(t, known, "set-forge-id")          // shared
	assert.Contains(t, known, "add-autoscalers")       // Crow
	assert.Contains(t, known, "add-distributed-locks") // Crow

	// Should be the sum of both lists
	assert.Equal(t, len(sharedMigrations)+len(crowMigrations), len(known))
}

func TestConstantsConsistency(t *testing.T) {
	// Verify that shared and Crow migrations don't overlap
	for _, shared := range sharedMigrations {
		assert.NotContains(t, crowMigrations, shared,
			"Migration %q should not be in both shared and Crow lists", shared)
	}

	// Verify some expected migrations are in the right lists
	assert.Contains(t, sharedMigrations, "legacy-to-xormigrate")
	assert.Contains(t, sharedMigrations, "add-org-id")
	assert.Contains(t, crowMigrations, "add-distributed-locks")
	assert.Contains(t, crowMigrations, "add-autoscalers")
}
