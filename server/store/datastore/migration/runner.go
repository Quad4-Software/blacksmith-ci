// Copyright 2026 Crow Authors
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
	"fmt"

	"github.com/rs/zerolog/log"
)

// migration represents a single schema migration.
type migration struct {
	ID      string
	Migrate func(tx *sql.Tx) error
}

// runner manages migrations tracked in the "migrations" table.
// It is compatible with the table schema used by the legacy xormigrate runner.
type runner struct {
	db *sql.DB
}

func newRunner(db *sql.DB) *runner {
	return &runner{db: db}
}

// run executes all pending migrations in order.
func (r *runner) run(migrations []*migration) error {
	if err := r.ensureMigrationsTable(); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	applied, err := r.appliedMigrations()
	if err != nil {
		return fmt.Errorf("list applied migrations: %w", err)
	}

	for _, m := range migrations {
		if applied[m.ID] {
			continue
		}

		log.Info().Str("id", m.ID).Msg("running migration")

		tx, err := r.db.Begin()
		if err != nil {
			return fmt.Errorf("begin tx for migration %s: %w", m.ID, err)
		}

		if err := m.Migrate(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %s failed: %w", m.ID, err)
		}

		if _, err := tx.Exec(fmt.Sprintf("INSERT INTO migrations (id) VALUES ('%s')", m.ID)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", m.ID, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", m.ID, err)
		}
	}

	return nil
}

func (r *runner) appliedMigrations() (map[string]bool, error) {
	rows, err := r.db.Query("SELECT id FROM migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		applied[id] = true
	}
	return applied, rows.Err()
}

// markApplied marks a list of migration IDs as applied without running them.
// Used when bootstrapping a fresh database with a baseline schema.
func (r *runner) markApplied(ids []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	for _, id := range ids {
		// Use string formatting instead of parameterized query because
		// ? placeholders are not portable across database drivers (PostgreSQL
		// requires $1). Migration IDs are trusted internal constants.
		if _, err := tx.Exec(fmt.Sprintf("INSERT INTO migrations (id) VALUES ('%s')", id)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("mark migration %s: %w", id, err)
		}
	}
	return tx.Commit()
}

// migrateLegacyTable copies entries from the legacy "migration" table (used by
// xormigrate) to the new "migrations" table. This is a one-time operation
// during the first upgrade from an XORM-based release.
func (r *runner) migrateLegacyTable() error {
	// Check if the legacy table exists
	rows, err := r.db.Query("SELECT id FROM migration")
	if err != nil {
		return nil // legacy table doesn't exist, nothing to migrate
	}
	defer rows.Close()

	// Collect legacy IDs
	var legacyIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		legacyIDs = append(legacyIDs, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if len(legacyIDs) == 0 {
		return nil
	}

	// Get already-applied IDs in the new table
	applied, err := r.appliedMigrations()
	if err != nil {
		return err
	}

	// Copy missing entries
	var toInsert []string
	for _, id := range legacyIDs {
		if !applied[id] {
			toInsert = append(toInsert, id)
		}
	}

	if len(toInsert) > 0 {
		log.Info().Int("count", len(toInsert)).Msg("migrating legacy migration records to new table")
		if err := r.markApplied(toInsert); err != nil {
			return err
		}
	}

	return nil
}

// addColumnIfNotExists returns a migration function that adds a column to a table,
// skipping if the column already exists. Works across SQLite, PostgreSQL, and MySQL
// by using a savepoint to isolate the column-existence probe so a failed SELECT
// doesn't poison the PostgreSQL transaction.
func addColumnIfNotExists(table, column, colType string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		_, _ = tx.Exec("SAVEPOINT add_col_check")
		rows, err := tx.Query(fmt.Sprintf("SELECT %s FROM %s LIMIT 1", column, table)) //nolint:gosec
		if err == nil {
			defer rows.Close()
			_ = rows.Err()
			_, _ = tx.Exec("RELEASE SAVEPOINT add_col_check")
			return nil // column already exists
		}
		_, _ = tx.Exec("ROLLBACK TO SAVEPOINT add_col_check")
		_, err = tx.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colType)) //nolint:gosec
		return err
	}
}

func (r *runner) ensureMigrationsTable() error {
	_, err := r.db.Exec(`CREATE TABLE IF NOT EXISTS migrations (id TEXT NULL)`)
	return err
}

// isFreshDatabase returns true only if no application tables exist at all.
// It checks for the legacy "migration" table (used by xormigrate), the new
// "migrations" table, and core data tables like "users" and "repos".
// This prevents a false "fresh database" detection when upgrading from
// an XORM-based release.
func (r *runner) isFreshDatabase() bool {
	// Check the new migrations table first
	var count int
	if err := r.db.QueryRow("SELECT COUNT(*) FROM migrations").Scan(&count); err == nil && count > 0 {
		return false
	}

	// Check the legacy xormigrate table (named "migration", singular)
	if err := r.db.QueryRow("SELECT COUNT(*) FROM migration").Scan(&count); err == nil && count > 0 {
		return false
	}

	// Final safety check: if any core data table exists with data, this is not fresh
	for _, table := range []string{"users", "repos", "pipelines"} {
		if err := r.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err == nil && count > 0 { //nolint:gosec
			return false
		}
	}

	return true
}
