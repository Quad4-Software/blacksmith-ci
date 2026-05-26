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
	"redirections",
	"registries",
	"repos",
	"secret_targets",
	"secrets",
	"steps",
	"user_forges",
	"users",
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
