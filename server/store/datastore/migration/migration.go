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
var baselineSchemaSQLite string

//go:embed baseline_schema_postgres.sql
var baselineSchemaPostgres string

//go:embed baseline_schema_mysql.sql
var baselineSchemaMySQL string

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
func Migrate(_ context.Context, db *sql.DB, driver string, _ bool) error {
	r := newRunner(db)

	if r.isFreshDatabase() {
		log.Info().Str("driver", driver).Msg("fresh database detected, creating baseline schema")
		if err := r.ensureMigrationsTable(); err != nil {
			return fmt.Errorf("ensure migrations table: %w", err)
		}
		schema := baselineSchemaForDriver(driver)
		if err := execStatements(db, schema); err != nil {
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
	// IMPORTANT: New migration IDs must NOT be in crowMigrations (constants.go).
	// That list is for already-applied migrations. Add the ID there only after
	// it has run on production. The checkLegacyMigrationsComplete check runs
	// before these migrations, so any ID in crowMigrations that isn't in the
	// DB yet will cause a fatal "upgrade to v5.6 first" error.
	newMigrations := []*migration{
		{ID: "crow-add-autoscaler-pending-since", Migrate: addColumnIfNotExists(
			"autoscalers", "pending_since", "INTEGER",
		)},
		{ID: "crow-add-email-unsubscribes", Migrate: createEmailUnsubscribes(driver)},
		{ID: "crow-add-task-canceled", Migrate: addTaskCancelledColumns},
		{ID: "crow-add-step-rerun-fields", Migrate: addStepRerunFields},
	}

	if len(newMigrations) > 0 {
		if err := r.run(newMigrations); err != nil {
			return err
		}
	}

	return nil
}

// checkLegacyMigrationsComplete verifies that Crow-specific migrations have been applied.
// Only checks crowMigrations (not shared/Woodpecker migrations) because databases created
// with InitSchema or very old Woodpecker versions may not have every shared migration ID
// recorded, even though the schema is up to date.
// If Crow migrations are missing, the user must first upgrade to v5.6 (the last XORM-based
// release) before upgrading to this version.
func checkLegacyMigrationsComplete(info *MigrationInfo) error {
	applied := make(map[string]bool, len(info.AppliedMigrations))
	for _, id := range info.AppliedMigrations {
		applied[id] = true
	}

	var missing []string
	for _, id := range crowMigrations {
		if !applied[id] {
			missing = append(missing, id)
		}
	}

	if len(missing) > 0 {
		log.Error().
			Strs("missing_migrations", missing).
			Msg("Database has unapplied Crow migrations. " +
				"Please first upgrade to Crow v5.6, run the server once to apply all migrations, " +
				"then upgrade to this version.")
		return fmt.Errorf("database has %d unapplied Crow migrations — upgrade to v5.6 first", len(missing))
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

// baselineSchemaForDriver returns the embedded baseline DDL for the given database driver.
func baselineSchemaForDriver(driver string) string {
	switch driver {
	case "postgres":
		return baselineSchemaPostgres
	case "mysql":
		return baselineSchemaMySQL
	default:
		return baselineSchemaSQLite
	}
}

// createEmailUnsubscribes returns a migration function that creates the email_unsubscribes
// table with the correct auto-increment syntax for the given driver.
func createEmailUnsubscribes(driver string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		var ddl string
		switch driver {
		case "postgres":
			ddl = `CREATE TABLE IF NOT EXISTS email_unsubscribes (
				id BIGSERIAL PRIMARY KEY,
				user_id INTEGER NOT NULL,
				repo_id INTEGER NOT NULL,
				email TEXT NOT NULL,
				created_at INTEGER,
				UNIQUE(user_id, repo_id)
			)`
		case "mysql":
			ddl = `CREATE TABLE IF NOT EXISTS email_unsubscribes (
				id BIGINT PRIMARY KEY AUTO_INCREMENT,
				user_id INTEGER NOT NULL,
				repo_id INTEGER NOT NULL,
				email TEXT NOT NULL,
				created_at INTEGER,
				UNIQUE(user_id, repo_id)
			)`
		default:
			ddl = `CREATE TABLE IF NOT EXISTS email_unsubscribes (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id INTEGER NOT NULL,
				repo_id INTEGER NOT NULL,
				email TEXT NOT NULL,
				created_at INTEGER,
				UNIQUE(user_id, repo_id)
			)`
		}
		_, err := tx.Exec(ddl)
		return err
	}
}

// addTaskCancelledColumns adds canceled and canceled_at columns to the tasks table
// for cross-instance cancel propagation in HA mode.
// Skips gracefully if the tasks table doesn't exist (fresh databases get these
// columns from the baseline schema).
func addTaskCancelledColumns(tx *sql.Tx) error {
	if !tableExists(tx, "tasks") {
		return nil
	}

	addCanceled := addColumnIfNotExists("tasks", "canceled", "INTEGER DEFAULT 0")
	if err := addCanceled(tx); err != nil {
		return err
	}
	addCanceledAt := addColumnIfNotExists("tasks", "canceled_at", "INTEGER DEFAULT 0")
	return addCanceledAt(tx)
}

// addStepRerunFields adds original_step_id and attempt columns to the steps table
// for step-level rerun tracking.
// Skips gracefully if the steps table doesn't exist (fresh databases get these
// columns from the baseline schema).
func addStepRerunFields(tx *sql.Tx) error {
	if !tableExists(tx, "steps") {
		return nil
	}

	if err := addColumnIfNotExists("steps", "original_step_id", "INTEGER DEFAULT 0")(tx); err != nil {
		return err
	}
	return addColumnIfNotExists("steps", "attempt", "INTEGER DEFAULT 1")(tx)
}

// tableExists checks whether a table exists in the database using a savepoint
// so that a failed probe doesn't poison the surrounding transaction.
// The rows are closed before returning, ensuring no open result sets leak to callers.
func tableExists(tx *sql.Tx, table string) bool {
	_, _ = tx.Exec("SAVEPOINT table_check")
	rows, err := tx.Query(fmt.Sprintf("SELECT 1 FROM %s LIMIT 1", table)) //nolint:gosec
	if err != nil {
		_, _ = tx.Exec("ROLLBACK TO SAVEPOINT table_check")
		return false
	}
	defer rows.Close()
	_ = rows.Err()
	_, _ = tx.Exec("RELEASE SAVEPOINT table_check")
	return true
}
