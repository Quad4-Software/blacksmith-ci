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

package migration

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

//go:embed baseline_schema.sql
var baselineSchema string

// allLegacyMigrationIDs contains the IDs of all migrations that existed before
// the switch to the Bun-based migration runner. For fresh databases, the baseline
// schema is used instead and these IDs are marked as applied.
var allLegacyMigrationIDs = append(
	append([]string{"SCHEMA_INIT"}, sharedMigrations[1:]...), // skip duplicate "SCHEMA_INIT" if present
	crowMigrations...,
)

// Migrate runs all pending database migrations.
// For fresh databases, it creates the full schema from the baseline DDL.
// For existing databases, it runs only new migrations that haven't been applied yet.
func Migrate(_ context.Context, db *sql.DB, _ string, _ bool) error {
	r := newRunner(db)

	if r.isFreshDatabase() {
		log.Info().Msg("fresh database detected, creating baseline schema")
		if err := r.ensureMigrationsTable(); err != nil {
			return fmt.Errorf("ensure migrations table: %w", err)
		}
		if err := execStatements(db, baselineSchema); err != nil {
			return fmt.Errorf("create baseline schema: %w", err)
		}
		if err := r.markApplied(allLegacyMigrationIDs); err != nil {
			return fmt.Errorf("mark legacy migrations: %w", err)
		}
		log.Info().Msg("baseline schema created successfully")
		return nil
	}

	// Ensure the new "migrations" table exists
	if err := r.ensureMigrationsTable(); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	// Migrate entries from the legacy "migration" table (xormigrate) to
	// the new "migrations" table, if the legacy table exists.
	if err := r.migrateLegacyTable(); err != nil {
		return fmt.Errorf("migrate legacy migration table: %w", err)
	}

	// For existing databases, detect origin and log info
	migrationInfo, err := detectDatabaseOriginSQL(db)
	if err != nil {
		log.Warn().Err(err).Msg("could not detect database origin")
	} else {
		logMigrationInfo(migrationInfo)
	}

	// Check that all legacy migrations have been applied.
	// If not, the user needs to first upgrade to the last XORM-based release (v5.6)
	// which will run the legacy migrations, then upgrade to this version.
	if migrationInfo != nil {
		if err := checkLegacyMigrationsComplete(migrationInfo); err != nil {
			return err
		}
	}

	// Run any new migrations (added after the baseline).
	// To add a new migration, append it to this slice.
	newMigrations := []*migration{
		{ID: "crow-add-autoscaler-pending-since", Migrate: addColumnIfNotExists(
			"autoscalers", "pending_since", "INTEGER",
		)},
	}

	if len(newMigrations) > 0 {
		if err := r.run(newMigrations); err != nil {
			return err
		}
	}

	return nil
}

// checkLegacyMigrationsComplete verifies that all legacy migrations have been applied.
// If any are missing, the user must first upgrade to v5.6 (the last XORM-based release)
// before upgrading to this version.
func checkLegacyMigrationsComplete(info *MigrationInfo) error {
	applied := make(map[string]bool, len(info.AppliedMigrations))
	for _, id := range info.AppliedMigrations {
		applied[id] = true
	}

	var missing []string
	for _, id := range allLegacyMigrationIDs {
		if !applied[id] {
			missing = append(missing, id)
		}
	}

	if len(missing) > 0 {
		log.Error().
			Strs("missing_migrations", missing).
			Msg("Database has unapplied legacy migrations. " +
				"Please first upgrade to Crow v5.6, run the server once to apply all migrations, " +
				"then upgrade to this version.")
		return fmt.Errorf("database has %d unapplied legacy migrations — upgrade to v5.6 first", len(missing))
	}

	return nil
}

// execStatements splits SQL by semicolons and executes each statement.
// Comment-only lines (starting with --) within a statement are stripped.
func execStatements(db *sql.DB, rawSQL string) error {
	for _, segment := range strings.Split(rawSQL, ";") {
		// Strip comment lines
		var lines []string
		for _, line := range strings.Split(segment, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "--") {
				lines = append(lines, line)
			}
		}
		stmt := strings.TrimSpace(strings.Join(lines, "\n"))
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			const truncLen = 80
			return fmt.Errorf("exec %q: %w", truncate(stmt, truncLen), err)
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
