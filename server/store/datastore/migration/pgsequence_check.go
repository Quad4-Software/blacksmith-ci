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
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

// Every table listed here must have a sequence-backed default on its id column per the baseline schema.
// Generic SQLite-to-PostgreSQL copy tools such as pgloader drop the sequence for empty tables, leaving id bigint NOT NULL and breaking subsequent INSERTs with SQLSTATE 23502.
// Keep in sync with baseline_schema_postgres.sql.
var expectedBigSerialTables = []string{
	"access_tokens",
	"agents",
	"approved_pull_requests",
	"autoscalers",
	"configs",
	"crons",
	"distributed_locks",
	"email_unsubscribes",
	"forges",
	"integrations",
	"log_entries",
	"maintenance_config",
	"maintenance_logs",
	"maintenance_operation_config",
	"maintenance_stats",
	"notification_configs",
	"orgs",
	"pipelines",
	"proc_log_entries",
	"redirections",
	"registries",
	"repos",
	"secret_targets",
	"secrets",
	"steps",
	"user_forges",
	"users",
	"workflow_approval_events",
	"workflows",
}

// checkPostgresSequences verifies that every table expected to have a
// BIGSERIAL `id` column actually has a sequence-backed default. It catches
// pgloader-style migrations (issue #1067) where the schema was copied from
// SQLite without translating implicit AUTOINCREMENT into a PostgreSQL
// sequence, leaving INSERTs to fail at runtime with NOT NULL violations.
//
// No-op on non-PostgreSQL drivers.
func checkPostgresSequences(db *sql.DB, driver string) error {
	if driver != "postgres" {
		return nil
	}

	placeholders := make([]string, len(expectedBigSerialTables))
	args := make([]any, len(expectedBigSerialTables))
	for i, name := range expectedBigSerialTables {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = name
	}

	query := fmt.Sprintf(`
		SELECT table_name
		FROM information_schema.columns
		WHERE table_schema = 'public'
		AND column_name = 'id'
		AND column_default IS NULL
		AND table_name IN (%s)
		ORDER BY table_name
	`, strings.Join(placeholders, ", "))

	rows, err := db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("check id-column sequences: %w", err)
	}
	defer rows.Close()

	var broken []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan broken-sequence row: %w", err)
		}
		broken = append(broken, name)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate broken-sequence rows: %w", err)
	}

	if len(broken) == 0 {
		return nil
	}

	log.Error().
		Strs("tables", broken).
		Msg("PostgreSQL tables are missing the sequence-backed DEFAULT on their `id` column. " +
			"This typically results from a pgloader-style SQLite→PostgreSQL migration that " +
			"did not translate implicit AUTOINCREMENT into a sequence. " +
			"Any INSERT into these tables will fail with SQLSTATE 23502 (null value in column \"id\"). " +
			"See https://codefloe.com/crowci/crow/issues/1067 for context.")

	return fmt.Errorf(
		"%d table(s) missing id-column sequence on PostgreSQL: %s\n\n"+
			"Run the following SQL against your database to repair them, then restart Crow:\n\n%s",
		len(broken),
		strings.Join(broken, ", "),
		repairSQLFor(broken),
	)
}

// repairSQLFor returns a copy-pasteable SQL block that recreates the
// `<table>_id_seq` sequence, advances it past existing rows, and re-attaches
// it as the column default. Safe to run repeatedly.
func repairSQLFor(tables []string) string {
	var b strings.Builder
	for _, t := range tables {
		fmt.Fprintf(&b,
			"CREATE SEQUENCE IF NOT EXISTS %s_id_seq OWNED BY %s.id;\n"+
				"SELECT setval('%s_id_seq', COALESCE((SELECT MAX(id) FROM %s), 0) + 1, false);\n"+
				"ALTER TABLE %s ALTER COLUMN id SET DEFAULT nextval('%s_id_seq');\n\n",
			t, t, t, t, t, t,
		)
	}
	return b.String()
}

// repairPostgresIDSequences returns a migration that auto-applies the same
// repair operators were previously asked to run by hand: for every table in
// expectedBigSerialTables whose `id` column has no sequence-backed default,
// create the `<table>_id_seq` sequence, advance it past existing rows, and
// attach it as the column default. No-op on non-PostgreSQL drivers and on
// tables that are either absent or already healthy, so a clean DB sees
// nothing happen.
//
// Affects existing deployments that ran the buggy `crow-add-email-unsubscribes`
// migration before da8da2fdf (when it created the table with `id INTEGER
// PRIMARY KEY` on Postgres) and pgloader-style SQLite→Postgres migrations
// (issue #1067).
func repairPostgresIDSequences(driver string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		if driver != "postgres" {
			return nil
		}

		for _, table := range expectedBigSerialTables {
			var hasDefault bool
			err := tx.QueryRow(`
				SELECT column_default IS NOT NULL
				FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = $1 AND column_name = 'id'
			`, table).Scan(&hasDefault)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return fmt.Errorf("inspect %s.id default: %w", table, err)
			}
			if hasDefault {
				continue
			}

			log.Warn().Str("table", table).Msg("repairing missing PostgreSQL id sequence")

			seq := table + "_id_seq"
			if _, err := tx.Exec(fmt.Sprintf(
				`CREATE SEQUENCE IF NOT EXISTS %s OWNED BY %s.id`, seq, table,
			)); err != nil { //nolint:gosec
				return fmt.Errorf("create sequence %s: %w", seq, err)
			}
			if _, err := tx.Exec(fmt.Sprintf(
				`SELECT setval('%s', COALESCE((SELECT MAX(id) FROM %s), 0) + 1, false)`, seq, table,
			)); err != nil { //nolint:gosec
				return fmt.Errorf("setval %s: %w", seq, err)
			}
			if _, err := tx.Exec(fmt.Sprintf(
				`ALTER TABLE %s ALTER COLUMN id SET DEFAULT nextval('%s')`, table, seq,
			)); err != nil { //nolint:gosec
				return fmt.Errorf("set default on %s.id: %w", table, err)
			}
		}

		return nil
	}
}

// repairMySQLAutoIncrementIDs returns a migration that ensures every table in
// expectedBigSerialTables has an AUTO_INCREMENT id column on MySQL. For tables
// whose id column is missing AUTO_INCREMENT (a `column_type` lookup whose
// `extra` does not contain "auto_increment"), it issues an
// `ALTER TABLE ... MODIFY COLUMN id <existing_type> NOT NULL AUTO_INCREMENT`.
// The existing column type is preserved so other tables' FK columns continue
// to match. MySQL automatically advances the auto-increment counter past
// existing rows when AUTO_INCREMENT is added.
//
// No-op on non-MySQL drivers and on already-healthy columns. Affects MySQL
// deployments that ran the buggy `crow-add-email-unsubscribes` migration
// before da8da2fdf (when it created the table with `id INTEGER PRIMARY KEY`,
// i.e. INT with no AUTO_INCREMENT on MySQL).
func repairMySQLAutoIncrementIDs(driver string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		if driver != "mysql" {
			return nil
		}

		for _, table := range expectedBigSerialTables {
			var columnType, extra string
			err := tx.QueryRow(`
				SELECT column_type, extra
				FROM information_schema.columns
				WHERE table_schema = DATABASE() AND table_name = ? AND column_name = 'id'
			`, table).Scan(&columnType, &extra)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return fmt.Errorf("inspect %s.id on MySQL: %w", table, err)
			}
			if strings.Contains(strings.ToLower(extra), "auto_increment") {
				continue
			}

			log.Warn().Str("table", table).Msg("repairing missing MySQL AUTO_INCREMENT on id column")

			stmt := fmt.Sprintf(
				`ALTER TABLE %s MODIFY COLUMN id %s NOT NULL AUTO_INCREMENT`,
				table, columnType,
			)
			if _, err := tx.Exec(stmt); err != nil { //nolint:gosec
				return fmt.Errorf("add AUTO_INCREMENT to %s.id: %w", table, err)
			}
		}

		return nil
	}
}
