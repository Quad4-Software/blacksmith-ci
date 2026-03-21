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
	"slices"

	"github.com/rs/zerolog/log"
)

// DatabaseOrigin indicates where the database was originally created.
type DatabaseOrigin string

const (
	OriginNew        DatabaseOrigin = "new"
	OriginWoodpecker DatabaseOrigin = "woodpecker"
	OriginCrow       DatabaseOrigin = "crow"
	OriginUnknown    DatabaseOrigin = "unknown"
)

// MigrationInfo contains analysis results from database migration inspection.
type MigrationInfo struct {
	Origin               DatabaseOrigin
	AppliedMigrations    []string
	UnknownMigrations    []string
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

// detectDatabaseOriginSQL analyzes the migrations table to determine
// whether the database was originally created by Woodpecker or Crow.
func detectDatabaseOriginSQL(db *sql.DB) (*MigrationInfo, error) {
	info := &MigrationInfo{
		Origin:            OriginNew,
		AppliedMigrations: []string{},
		UnknownMigrations: []string{},
	}

	// Try the new "migrations" table first, then fall back to the legacy
	// "migration" table (used by xormigrate in XORM-based releases).
	rows, err := db.Query("SELECT id FROM migrations")
	if err != nil {
		rows, err = db.Query("SELECT id FROM migration")
		if err != nil {
			// Neither table exists — fresh database
			return info, nil //nolint:nilerr
		}
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		info.AppliedMigrations = append(info.AppliedMigrations, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(info.AppliedMigrations) == 0 {
		return info, nil
	}

	hasCrowMigrations := hasAnyMigrationFrom(info.AppliedMigrations, crowMigrations)
	hasSharedMigrations := hasAnyMigrationFrom(info.AppliedMigrations, sharedMigrations)

	switch {
	case hasCrowMigrations:
		info.Origin = OriginCrow
	case hasSharedMigrations:
		info.Origin = OriginWoodpecker
		info.IsFirstCrowMigration = true
	default:
		info.Origin = OriginUnknown
	}

	knownMigrations := allKnownMigrations()
	for _, id := range info.AppliedMigrations {
		if !slices.Contains(knownMigrations, id) {
			info.UnknownMigrations = append(info.UnknownMigrations, id)
		}
	}

	return info, nil
}

// logMigrationInfo logs diagnostic information about database origin.
func logMigrationInfo(info *MigrationInfo) {
	switch info.Origin {
	case OriginNew:
		log.Info().Msg("Initializing new Crow database")
	case OriginWoodpecker:
		log.Warn().Msg("╔════════════════════════════════════════════════════════════════╗")
		log.Warn().Msg("║  WOODPECKER DATABASE DETECTED                                  ║")
		log.Warn().Msg("║  Migrating from Woodpecker CI to Crow CI                       ║")
		log.Warn().Msg("║  Your data will be preserved and upgraded automatically.       ║")
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

	if len(info.UnknownMigrations) > 0 {
		log.Warn().
			Strs("unknown_migrations", info.UnknownMigrations).
			Msg("Unknown migration IDs detected in database. These may be from a different version or fork. Migration will continue but review is recommended.")
	}
}
