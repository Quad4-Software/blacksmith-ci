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
	"fmt"
	"slices"

	"github.com/rs/zerolog/log"
	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

// DatabaseOrigin indicates where the database was originally created.
type DatabaseOrigin string

const (
	// OriginNew indicates a fresh database with no prior migrations.
	OriginNew DatabaseOrigin = "new"

	// OriginWoodpecker indicates the database was created by Woodpecker CI.
	OriginWoodpecker DatabaseOrigin = "woodpecker"

	// OriginCrow indicates the database was created by Crow CI.
	OriginCrow DatabaseOrigin = "crow"

	// OriginUnknown indicates the database has migrations but we can't
	// determine the origin (possibly from a modified fork).
	OriginUnknown DatabaseOrigin = "unknown"
)

// MigrationInfo contains analysis results from database migration inspection.
type MigrationInfo struct {
	// Origin indicates where the database came from.
	Origin DatabaseOrigin

	// AppliedMigrations lists all migration IDs found in the database.
	AppliedMigrations []string

	// UnknownMigrations lists migration IDs that are not recognized.
	// These may be from a newer version or a different fork.
	UnknownMigrations []string

	// IsFirstCrowMigration is true when this is the first time Crow
	// is running against a Woodpecker database.
	IsFirstCrowMigration bool
}

// hasAnyMigrationFrom checks if any migration from the given set exists
// in the list of applied migrations.
func hasAnyMigrationFrom(appliedMigrations, migrationSet []string) bool {
	for _, id := range appliedMigrations {
		if slices.Contains(migrationSet, id) {
			return true
		}
	}
	return false
}

// detectDatabaseOrigin analyzes the migrations table to determine
// whether the database was originally created by Woodpecker or Crow.
//
// Detection logic:
//   - Empty/no migrations table -> OriginNew.
//   - Has Crow-specific migrations -> OriginCrow.
//   - Has shared migrations but no Crow ones -> OriginWoodpecker.
//   - Has unknown migrations only -> OriginUnknown.
func detectDatabaseOrigin(e *xorm.Engine) (*MigrationInfo, error) {
	info := &MigrationInfo{
		Origin:            OriginNew,
		AppliedMigrations: []string{},
		UnknownMigrations: []string{},
	}

	// Check if xormigrate's migrations table exists
	// Note: xormigrate uses the Migration struct directly as the table
	exists, err := e.IsTableExist(&xormigrate.Migration{})
	if err != nil {
		return nil, fmt.Errorf("failed to check migrations table: %w", err)
	}

	if !exists {
		// No migrations table means fresh database
		return info, nil
	}

	// Query all applied migrations
	var migrations []xormigrate.Migration
	if err := e.Find(&migrations); err != nil {
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}

	if len(migrations) == 0 {
		// Empty table also means fresh database
		return info, nil
	}

	// Extract migration IDs
	for _, m := range migrations {
		info.AppliedMigrations = append(info.AppliedMigrations, m.ID)
	}

	// Check for Crow-exclusive and shared migrations
	hasCrowMigrations := hasAnyMigrationFrom(info.AppliedMigrations, crowMigrations)
	hasSharedMigrations := hasAnyMigrationFrom(info.AppliedMigrations, sharedMigrations)

	// Determine origin based on migration presence
	switch {
	case hasCrowMigrations:
		// Has Crow migrations - this is a Crow database
		info.Origin = OriginCrow
	case hasSharedMigrations:
		// Has shared migrations but no Crow ones - this is Woodpecker
		info.Origin = OriginWoodpecker
		info.IsFirstCrowMigration = true
	default:
		// Has migrations but none we recognize
		info.Origin = OriginUnknown
	}

	// Find unknown migrations (present in DB but not in our lists)
	knownMigrations := allKnownMigrations()
	for _, id := range info.AppliedMigrations {
		if !slices.Contains(knownMigrations, id) {
			info.UnknownMigrations = append(info.UnknownMigrations, id)
		}
	}

	return info, nil
}

// logMigrationInfo logs diagnostic information about database origin
// and any detected issues. Uses prominent formatting for important messages.
func logMigrationInfo(info *MigrationInfo) {
	switch info.Origin {
	case OriginNew:
		log.Info().Msg("Initializing new Crow database")

	case OriginWoodpecker:
		// Prominent banner for Woodpecker migration
		log.Warn().Msg("╔════════════════════════════════════════════════════════════════╗")
		log.Warn().Msg("║  WOODPECKER DATABASE DETECTED                                  ║")
		log.Warn().Msg("║                                                                ║")
		log.Warn().Msg("║  Migrating from Woodpecker CI to Crow CI                       ║")
		log.Warn().Msg("║  This is a ONE-TIME migration process.                         ║")
		log.Warn().Msg("║                                                                ║")
		log.Warn().Msg("║  Your data will be preserved and upgraded automatically.       ║")
		log.Warn().Msg("║  For best results, ensure you have a database backup.          ║")
		log.Warn().Msg("╚════════════════════════════════════════════════════════════════╝")

	case OriginCrow:
		log.Debug().
			Int("migrations", len(info.AppliedMigrations)).
			Msg("Continuing with existing Crow database")

	case OriginUnknown:
		log.Warn().
			Strs("applied", info.AppliedMigrations).
			Msg("Database origin unknown - proceeding with caution")
	}

	// Warn about unknown migrations
	if len(info.UnknownMigrations) > 0 {
		log.Warn().
			Strs("unknown_migrations", info.UnknownMigrations).
			Msg("Unknown migration IDs detected in database. " +
				"These may be from a different version or fork. " +
				"Migration will continue but review is recommended.")
	}
}
