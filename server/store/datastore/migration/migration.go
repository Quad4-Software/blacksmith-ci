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
	"errors"
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
		{ID: "crow-add-secret-targets", Migrate: createSecretTargets(driver)},
		{ID: "crow-add-proc-log-entries", Migrate: createProcLogEntries(driver)},
		{ID: "crow-repair-postgres-id-sequences", Migrate: repairPostgresIDSequences(driver)},
		{ID: "crow-repair-mysql-id-auto-increment", Migrate: repairMySQLAutoIncrementIDs(driver)},
		{ID: "crow-fix-proc-log-int64-columns", Migrate: fixProcLogInt64Columns(driver)},
		// v2 re-runs the same ALTER on databases where v1 landed as a silent no-op
		// because tableExists wrongly returned false for the empty proc_log_entries
		// table. With tableExists now fixed, v1 also works going forward, but the
		// migrations table on already-deployed clusters still has v1 marked applied —
		// only a new ID will trigger the widening.
		{ID: "crow-fix-proc-log-int64-columns-v2", Migrate: fixProcLogInt64Columns(driver)},
		{ID: "crow-add-workflow-approval", Migrate: addWorkflowApprovalColumns(driver)},
		{ID: "crow-create-workflow-approval-events", Migrate: createWorkflowApprovalEvents(driver)},
		{ID: "crow-agent-org-ids", Migrate: migrateAgentOrgIDs(driver)},
		{ID: "crow-add-log-entries-unique-index", Migrate: addLogEntriesUniqueIndex(driver)},
		{ID: "crow-add-pipeline-org-identity", Migrate: addPipelineOrgIdentity(driver)},
		{ID: "crow-backfill-workflow-agent-identity", Migrate: backfillWorkflowAgentIdentity(driver)},
		{ID: "crow-add-agent-host-resources", Migrate: addAgentHostResources},
		{ID: "crow-add-config-generated-from", Migrate: addConfigGeneratedFrom(driver)},
		{ID: "crow-add-user-identities", Migrate: createUserIdentities(driver)},
		{ID: "crow-add-user-roles", Migrate: addUserRoles},
		{ID: "crow-add-integration-secret-filters", Migrate: addIntegrationSecretFilters},
		{ID: "crow-add-build-caches", Migrate: createBuildCaches(driver)},
		{ID: "crow-add-agent-protocol-version", Migrate: addAgentProtocolVersion},
		{ID: "crow-add-agent-remote-update", Migrate: addAgentRemoteUpdate},
		{ID: "crow-add-provisioning-progress", Migrate: addProvisioningProgress},
		{ID: "crow-add-build-cache-management", Migrate: addBuildCacheManagement(driver)},
		{ID: "crow-add-pipeline-watches", Migrate: createPipelineWatches(driver)},
		{ID: "crow-drop-repo-notification-configs", Migrate: dropRepoNotificationConfigs},
	}

	if len(newMigrations) > 0 {
		if err := r.run(newMigrations); err != nil {
			return err
		}
	}

	if err := checkPostgresSequences(db, driver); err != nil {
		return err
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

// dropRepoNotificationConfigs removes the repo-level notification settings and
// per-repo unsubscribe records. Per-user pipeline watches replace them and the
// old data is intentionally not migrated.
func dropRepoNotificationConfigs(tx *sql.Tx) error {
	for _, table := range []string{"notification_configs", "email_unsubscribes"} {
		if _, err := tx.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			return err
		}
	}
	return nil
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

// addConfigGeneratedFrom adds the generated_from column to the configs table, which
// names the .jsonnet source a stored YAML document was evaluated from (empty for
// configs fetched from the forge verbatim).
// Skips gracefully if the configs table doesn't exist (fresh databases get the
// column from the baseline schema).
func addConfigGeneratedFrom(driver string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		if !tableExists(tx, "configs") {
			return nil
		}

		// MySQL before 8.0.13 rejects a DEFAULT on a TEXT column, so mirror the
		// VARCHAR the MySQL baseline schema uses.
		colType := "TEXT NOT NULL DEFAULT ''"
		if driver == "mysql" {
			colType = "VARCHAR(250) NOT NULL DEFAULT ''"
		}

		return addColumnIfNotExists("configs", "generated_from", colType)(tx)
	}
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

// addWorkflowApprovalColumns adds gate-tracking columns to the workflows table
// and an index used by the timeout sweeper.
// Skips gracefully if the workflows table is missing (fresh databases get the
// columns from the baseline schema).
func addWorkflowApprovalColumns(driver string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		if !tableExists(tx, "workflows") {
			return nil
		}

		cols := []struct{ name, typ string }{
			{"requires_approval", "BOOLEAN DEFAULT FALSE"},
			{"approval_users", "TEXT"},
			{"approval_teams", "TEXT"},
			// VARCHAR not TEXT: MySQL rejects DEFAULT on TEXT in strict mode, and
			// a sized VARCHAR also lets the composite index work without a prefix.
			{"approval_state", "VARCHAR(16) NOT NULL DEFAULT ''"},
			{"approval_deadline", "BIGINT DEFAULT 0"},
			{"approval_timeout_seconds", "BIGINT DEFAULT 0"},
		}
		for _, c := range cols {
			if err := addColumnIfNotExists("workflows", c.name, c.typ)(tx); err != nil {
				return fmt.Errorf("add column workflows.%s: %w", c.name, err)
			}
		}

		// Partial index would be nicer but not portable across all three DBs
		// at this codebase's compatibility floor — use a plain composite index.
		if driver == "mysql" {
			// MySQL does not support CREATE INDEX IF NOT EXISTS. Probe
			// INFORMATION_SCHEMA so this migration is idempotent on re-run.
			var count int
			if err := tx.QueryRow(
				`SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS` +
					` WHERE TABLE_SCHEMA = DATABASE()` +
					` AND TABLE_NAME = 'workflows'` +
					` AND INDEX_NAME = 'idx_workflows_approval'`,
			).Scan(&count); err != nil {
				return fmt.Errorf("probe idx_workflows_approval: %w", err)
			}
			if count == 0 {
				if _, err := tx.Exec(`CREATE INDEX idx_workflows_approval ON workflows (approval_state, approval_deadline)`); err != nil {
					return fmt.Errorf("create idx_workflows_approval: %w", err)
				}
			}
			return nil
		}
		idxStmt := "CREATE INDEX IF NOT EXISTS idx_workflows_approval ON workflows (approval_state, approval_deadline)"
		if _, err := tx.Exec(idxStmt); err != nil {
			return fmt.Errorf("create idx_workflows_approval: %w", err)
		}
		return nil
	}
}

// createWorkflowApprovalEvents creates the audit table that records each
// approval/decline/expiry decision against a workflow gate.
//
// The index on workflow_id is a plain BIGINT index — MySQL has no TEXT-prefix
// requirement here — but `CREATE INDEX IF NOT EXISTS` is unsupported on MySQL
// versions older than MariaDB 10.5, so we probe INFORMATION_SCHEMA.STATISTICS
// for idempotency, mirroring the pattern used by createProcLogEntries and
// addWorkflowApprovalColumns above.
func createWorkflowApprovalEvents(driver string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		idType := "INTEGER PRIMARY KEY AUTOINCREMENT"
		switch driver {
		case "postgres":
			idType = "BIGSERIAL PRIMARY KEY"
		case "mysql":
			idType = "BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY"
		}

		stmt := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS workflow_approval_events (
			id          %s,
			workflow_id BIGINT NOT NULL,
			user_id     BIGINT NOT NULL DEFAULT 0,
			action      TEXT   NOT NULL,
			reason      TEXT,
			created_at  BIGINT NOT NULL
		)`, idType)
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("create workflow_approval_events: %w", err)
		}

		if driver == "mysql" {
			var count int
			if err := tx.QueryRow(
				`SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS` +
					` WHERE TABLE_SCHEMA = DATABASE()` +
					` AND TABLE_NAME = 'workflow_approval_events'` +
					` AND INDEX_NAME = 'idx_workflow_approval_events_workflow_id'`,
			).Scan(&count); err != nil {
				return fmt.Errorf("probe idx_workflow_approval_events_workflow_id: %w", err)
			}
			if count == 0 {
				if _, err := tx.Exec(
					"CREATE INDEX idx_workflow_approval_events_workflow_id ON workflow_approval_events (workflow_id)",
				); err != nil {
					return fmt.Errorf("create idx_workflow_approval_events_workflow_id: %w", err)
				}
			}
			return nil
		}

		if _, err := tx.Exec(
			"CREATE INDEX IF NOT EXISTS idx_workflow_approval_events_workflow_id ON workflow_approval_events (workflow_id)",
		); err != nil {
			return fmt.Errorf("create idx_workflow_approval_events_workflow_id: %w", err)
		}
		return nil
	}
}

// createUserIdentities creates the user_identities table, which links Crow users
// to accounts at an external identity provider (OIDC). The groups column is named
// idp_groups because GROUPS is a reserved word in MySQL 8. On MySQL the identity
// key columns are VARBINARY: every text collation ignores case or trailing
// spaces, which would let two distinct subjects resolve to the same identity.
func createUserIdentities(driver string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		var ddl string
		switch driver {
		case "postgres":
			ddl = `CREATE TABLE IF NOT EXISTS user_identities (
				id BIGSERIAL PRIMARY KEY,
				user_id INTEGER NOT NULL,
				provider VARCHAR(50) NOT NULL,
				issuer VARCHAR(255) NOT NULL DEFAULT '',
				subject VARCHAR(255) NOT NULL,
				email VARCHAR(500),
				username VARCHAR(250),
				idp_groups TEXT,
				linked_at INTEGER NOT NULL,
				last_login INTEGER,
				UNIQUE(provider, issuer, subject)
			)`
		case "mysql":
			ddl = `CREATE TABLE IF NOT EXISTS user_identities (
				id BIGINT PRIMARY KEY AUTO_INCREMENT,
				user_id INTEGER NOT NULL,
				provider VARBINARY(50) NOT NULL,
				issuer VARBINARY(255) NOT NULL DEFAULT '',
				subject VARBINARY(255) NOT NULL,
				email VARCHAR(500),
				username VARCHAR(250),
				idp_groups TEXT,
				linked_at INTEGER NOT NULL,
				last_login INTEGER,
				UNIQUE(provider, issuer, subject),
				INDEX idx_user_identities_user_id (user_id)
			)`
		default:
			ddl = `CREATE TABLE IF NOT EXISTS user_identities (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id INTEGER NOT NULL,
				provider VARCHAR(50) NOT NULL,
				issuer VARCHAR(255) NOT NULL DEFAULT '',
				subject VARCHAR(255) NOT NULL,
				email VARCHAR(500),
				username VARCHAR(250),
				idp_groups TEXT,
				linked_at INTEGER NOT NULL,
				last_login INTEGER,
				UNIQUE(provider, issuer, subject)
			)`
		}
		if _, err := tx.Exec(ddl); err != nil {
			return err
		}
		if driver == "mysql" {
			return nil
		}
		_, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_user_identities_user_id ON user_identities (user_id)`)
		return err
	}
}

// createBuildCaches creates the build_caches table holding the metadata of
// workflow cache archives. On MySQL scope and cache_key are VARBINARY: every
// text collation ignores case or trailing spaces, which would make two distinct
// cache keys collide.
func createBuildCaches(driver string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		var ddl string
		switch driver {
		case "postgres":
			ddl = `CREATE TABLE IF NOT EXISTS build_caches (
				id BIGSERIAL PRIMARY KEY,
				repo_id BIGINT NOT NULL,
				scope VARCHAR(255) NOT NULL,
				cache_key VARCHAR(255) NOT NULL,
				size BIGINT NOT NULL DEFAULT 0,
				committed BOOLEAN NOT NULL DEFAULT false,
				created BIGINT NOT NULL DEFAULT 0,
				last_used BIGINT NOT NULL DEFAULT 0,
				UNIQUE(repo_id, scope, cache_key)
			)`
		case "mysql":
			ddl = `CREATE TABLE IF NOT EXISTS build_caches (
				id BIGINT PRIMARY KEY AUTO_INCREMENT,
				repo_id BIGINT NOT NULL,
				scope VARBINARY(255) NOT NULL,
				cache_key VARBINARY(255) NOT NULL,
				size BIGINT NOT NULL DEFAULT 0,
				committed BOOLEAN NOT NULL DEFAULT false,
				created BIGINT NOT NULL DEFAULT 0,
				last_used BIGINT NOT NULL DEFAULT 0,
				UNIQUE(repo_id, scope, cache_key),
				INDEX idx_build_caches_last_used (last_used)
			)`
		default:
			ddl = `CREATE TABLE IF NOT EXISTS build_caches (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				repo_id INTEGER NOT NULL,
				scope VARCHAR(255) NOT NULL,
				cache_key VARCHAR(255) NOT NULL,
				size INTEGER NOT NULL DEFAULT 0,
				committed BOOLEAN NOT NULL DEFAULT false,
				created INTEGER NOT NULL DEFAULT 0,
				last_used INTEGER NOT NULL DEFAULT 0,
				UNIQUE(repo_id, scope, cache_key)
			)`
		}
		if _, err := tx.Exec(ddl); err != nil {
			return err
		}
		if driver == "mysql" {
			return nil
		}
		_, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_build_caches_last_used ON build_caches (last_used)`)
		return err
	}
}

// createPipelineWatches creates the tables of per-user pipeline watches and the
// per-key workflow states they share. On MySQL watch_target and workflow_name
// are VARBINARY: every text collation ignores case or trailing spaces, which
// would merge distinct branch or cron names. The email column is named
// email_enabled and not email to keep it distinct from user emails.
func createPipelineWatches(driver string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		var ddls []string
		switch driver {
		case "postgres":
			ddls = []string{
				`CREATE TABLE IF NOT EXISTS pipeline_watches (
					id BIGSERIAL PRIMARY KEY,
					user_id BIGINT NOT NULL,
					repo_id BIGINT NOT NULL,
					watch_event VARCHAR(50) NOT NULL,
					watch_target VARCHAR(255) NOT NULL,
					in_app BOOLEAN NOT NULL,
					email_enabled BOOLEAN NOT NULL,
					email_events TEXT,
					last_read_pipeline BIGINT NOT NULL DEFAULT 0,
					created_at BIGINT,
					updated_at BIGINT,
					UNIQUE(user_id, repo_id, watch_event, watch_target)
				)`,
				`CREATE TABLE IF NOT EXISTS watch_workflow_states (
					id BIGSERIAL PRIMARY KEY,
					repo_id BIGINT NOT NULL,
					watch_event VARCHAR(50) NOT NULL,
					watch_target VARCHAR(255) NOT NULL,
					workflow_name VARCHAR(255) NOT NULL,
					failing BOOLEAN NOT NULL,
					pipeline_id BIGINT NOT NULL,
					pipeline_number BIGINT NOT NULL,
					runs_since_seen BIGINT NOT NULL DEFAULT 0,
					updated_at BIGINT,
					UNIQUE(repo_id, watch_event, watch_target, workflow_name)
				)`,
			}
		case "mysql":
			ddls = []string{
				`CREATE TABLE IF NOT EXISTS pipeline_watches (
					id BIGINT PRIMARY KEY AUTO_INCREMENT,
					user_id BIGINT NOT NULL,
					repo_id BIGINT NOT NULL,
					watch_event VARCHAR(50) NOT NULL,
					watch_target VARBINARY(255) NOT NULL,
					in_app BOOLEAN NOT NULL,
					email_enabled BOOLEAN NOT NULL,
					email_events TEXT,
					last_read_pipeline BIGINT NOT NULL DEFAULT 0,
					created_at BIGINT,
					updated_at BIGINT,
					UNIQUE(user_id, repo_id, watch_event, watch_target),
					INDEX idx_pipeline_watches_key (repo_id, watch_event, watch_target)
				)`,
				`CREATE TABLE IF NOT EXISTS watch_workflow_states (
					id BIGINT PRIMARY KEY AUTO_INCREMENT,
					repo_id BIGINT NOT NULL,
					watch_event VARCHAR(50) NOT NULL,
					watch_target VARBINARY(255) NOT NULL,
					workflow_name VARBINARY(255) NOT NULL,
					failing BOOLEAN NOT NULL,
					pipeline_id BIGINT NOT NULL,
					pipeline_number BIGINT NOT NULL,
					runs_since_seen BIGINT NOT NULL DEFAULT 0,
					updated_at BIGINT,
					UNIQUE(repo_id, watch_event, watch_target, workflow_name)
				)`,
			}
		default:
			ddls = []string{
				`CREATE TABLE IF NOT EXISTS pipeline_watches (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					user_id INTEGER NOT NULL,
					repo_id INTEGER NOT NULL,
					watch_event VARCHAR(50) NOT NULL,
					watch_target VARCHAR(255) NOT NULL,
					in_app BOOLEAN NOT NULL,
					email_enabled BOOLEAN NOT NULL,
					email_events TEXT,
					last_read_pipeline INTEGER NOT NULL DEFAULT 0,
					created_at INTEGER,
					updated_at INTEGER,
					UNIQUE(user_id, repo_id, watch_event, watch_target)
				)`,
				`CREATE TABLE IF NOT EXISTS watch_workflow_states (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					repo_id INTEGER NOT NULL,
					watch_event VARCHAR(50) NOT NULL,
					watch_target VARCHAR(255) NOT NULL,
					workflow_name VARCHAR(255) NOT NULL,
					failing BOOLEAN NOT NULL,
					pipeline_id INTEGER NOT NULL,
					pipeline_number INTEGER NOT NULL,
					runs_since_seen INTEGER NOT NULL DEFAULT 0,
					updated_at INTEGER,
					UNIQUE(repo_id, watch_event, watch_target, workflow_name)
				)`,
			}
		}
		for _, ddl := range ddls {
			if _, err := tx.Exec(ddl); err != nil {
				return err
			}
		}
		if driver == "mysql" {
			return nil
		}
		_, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_pipeline_watches_key ON pipeline_watches (repo_id, watch_event, watch_target)`)
		return err
	}
}

// addBuildCacheManagement adds build_caches.bucket_id and the tables holding
// per-org cache limits, buckets orgs connected and daily cache usage.
func addBuildCacheManagement(driver string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		if tableExists(tx, "build_caches") {
			if err := addColumnIfNotExists("build_caches", "bucket_id", "BIGINT NOT NULL DEFAULT 0")(tx); err != nil {
				return fmt.Errorf("add build_caches.bucket_id: %w", err)
			}
		}
		var ddls []string
		switch driver {
		case "postgres":
			ddls = []string{
				`CREATE TABLE IF NOT EXISTS build_cache_settings (
					org_id BIGINT PRIMARY KEY,
					max_size BIGINT NOT NULL DEFAULT 0,
					max_egress BIGINT NOT NULL DEFAULT 0,
					ttl BIGINT NOT NULL DEFAULT 0,
					updated BIGINT NOT NULL DEFAULT 0
				)`,
				`CREATE TABLE IF NOT EXISTS build_cache_buckets (
					id BIGSERIAL PRIMARY KEY,
					org_id BIGINT NOT NULL,
					endpoint VARCHAR(255) NOT NULL,
					bucket VARCHAR(255) NOT NULL,
					region VARCHAR(255),
					prefix VARCHAR(255),
					access_key VARCHAR(255),
					secret_key TEXT,
					use_ssl BOOLEAN NOT NULL DEFAULT true,
					path_style BOOLEAN NOT NULL DEFAULT false,
					max_size BIGINT NOT NULL DEFAULT 0,
					max_egress BIGINT NOT NULL DEFAULT 0,
					ttl BIGINT NOT NULL DEFAULT 0,
					created BIGINT NOT NULL DEFAULT 0,
					updated BIGINT NOT NULL DEFAULT 0,
					UNIQUE(org_id)
				)`,
				`CREATE TABLE IF NOT EXISTS build_cache_usage (
					id BIGSERIAL PRIMARY KEY,
					repo_id BIGINT NOT NULL,
					org_id BIGINT NOT NULL,
					bucket_id BIGINT NOT NULL DEFAULT 0,
					day BIGINT NOT NULL,
					hits BIGINT NOT NULL DEFAULT 0,
					partial_hits BIGINT NOT NULL DEFAULT 0,
					misses BIGINT NOT NULL DEFAULT 0,
					denied BIGINT NOT NULL DEFAULT 0,
					saves BIGINT NOT NULL DEFAULT 0,
					egress BIGINT NOT NULL DEFAULT 0,
					ingress BIGINT NOT NULL DEFAULT 0,
					UNIQUE(repo_id, bucket_id, day)
				)`,
			}
		case "mysql":
			ddls = []string{
				`CREATE TABLE IF NOT EXISTS build_cache_settings (
					org_id BIGINT PRIMARY KEY,
					max_size BIGINT NOT NULL DEFAULT 0,
					max_egress BIGINT NOT NULL DEFAULT 0,
					ttl BIGINT NOT NULL DEFAULT 0,
					updated BIGINT NOT NULL DEFAULT 0
				)`,
				`CREATE TABLE IF NOT EXISTS build_cache_buckets (
					id BIGINT PRIMARY KEY AUTO_INCREMENT,
					org_id BIGINT NOT NULL,
					endpoint VARCHAR(255) NOT NULL,
					bucket VARCHAR(255) NOT NULL,
					region VARCHAR(255),
					prefix VARCHAR(255),
					access_key VARCHAR(255),
					secret_key TEXT,
					use_ssl BOOLEAN NOT NULL DEFAULT true,
					path_style BOOLEAN NOT NULL DEFAULT false,
					max_size BIGINT NOT NULL DEFAULT 0,
					max_egress BIGINT NOT NULL DEFAULT 0,
					ttl BIGINT NOT NULL DEFAULT 0,
					created BIGINT NOT NULL DEFAULT 0,
					updated BIGINT NOT NULL DEFAULT 0,
					UNIQUE(org_id)
				)`,
				`CREATE TABLE IF NOT EXISTS build_cache_usage (
					id BIGINT PRIMARY KEY AUTO_INCREMENT,
					repo_id BIGINT NOT NULL,
					org_id BIGINT NOT NULL,
					bucket_id BIGINT NOT NULL DEFAULT 0,
					day BIGINT NOT NULL,
					hits BIGINT NOT NULL DEFAULT 0,
					partial_hits BIGINT NOT NULL DEFAULT 0,
					misses BIGINT NOT NULL DEFAULT 0,
					denied BIGINT NOT NULL DEFAULT 0,
					saves BIGINT NOT NULL DEFAULT 0,
					egress BIGINT NOT NULL DEFAULT 0,
					ingress BIGINT NOT NULL DEFAULT 0,
					UNIQUE(repo_id, bucket_id, day),
					INDEX idx_build_cache_usage_org_day (org_id, day)
				)`,
			}
		default:
			ddls = []string{
				`CREATE TABLE IF NOT EXISTS build_cache_settings (
					org_id INTEGER PRIMARY KEY,
					max_size INTEGER NOT NULL DEFAULT 0,
					max_egress INTEGER NOT NULL DEFAULT 0,
					ttl INTEGER NOT NULL DEFAULT 0,
					updated INTEGER NOT NULL DEFAULT 0
				)`,
				`CREATE TABLE IF NOT EXISTS build_cache_buckets (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					org_id INTEGER NOT NULL,
					endpoint VARCHAR(255) NOT NULL,
					bucket VARCHAR(255) NOT NULL,
					region VARCHAR(255),
					prefix VARCHAR(255),
					access_key VARCHAR(255),
					secret_key TEXT,
					use_ssl BOOLEAN NOT NULL DEFAULT true,
					path_style BOOLEAN NOT NULL DEFAULT false,
					max_size INTEGER NOT NULL DEFAULT 0,
					max_egress INTEGER NOT NULL DEFAULT 0,
					ttl INTEGER NOT NULL DEFAULT 0,
					created INTEGER NOT NULL DEFAULT 0,
					updated INTEGER NOT NULL DEFAULT 0,
					UNIQUE(org_id)
				)`,
				`CREATE TABLE IF NOT EXISTS build_cache_usage (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					repo_id INTEGER NOT NULL,
					org_id INTEGER NOT NULL,
					bucket_id INTEGER NOT NULL DEFAULT 0,
					day INTEGER NOT NULL,
					hits INTEGER NOT NULL DEFAULT 0,
					partial_hits INTEGER NOT NULL DEFAULT 0,
					misses INTEGER NOT NULL DEFAULT 0,
					denied INTEGER NOT NULL DEFAULT 0,
					saves INTEGER NOT NULL DEFAULT 0,
					egress INTEGER NOT NULL DEFAULT 0,
					ingress INTEGER NOT NULL DEFAULT 0,
					UNIQUE(repo_id, bucket_id, day)
				)`,
			}
		}
		for _, ddl := range ddls {
			if _, err := tx.Exec(ddl); err != nil {
				return err
			}
		}
		if driver == "mysql" {
			return nil
		}
		_, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_build_cache_usage_org_day ON build_cache_usage (org_id, day)`)
		return err
	}
}

// createSecretTargets creates the secret_targets junction table for multi-scope secret sharing.
// Skips gracefully if the secrets table doesn't exist (fresh databases get this
// table from the baseline schema).
func createSecretTargets(driver string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		if !tableExists(tx, "secrets") {
			return nil
		}

		var ddl string
		switch driver {
		case "postgres":
			ddl = `CREATE TABLE IF NOT EXISTS secret_targets (
				id BIGSERIAL PRIMARY KEY,
				secret_id BIGINT NOT NULL REFERENCES secrets(id) ON DELETE CASCADE,
				org_id INTEGER NOT NULL DEFAULT 0,
				repo_id INTEGER NOT NULL DEFAULT 0,
				created_at INTEGER,
				UNIQUE(secret_id, org_id, repo_id)
			)`
		case "mysql":
			ddl = `CREATE TABLE IF NOT EXISTS secret_targets (
				id BIGINT PRIMARY KEY AUTO_INCREMENT,
				secret_id BIGINT NOT NULL,
				org_id INTEGER NOT NULL DEFAULT 0,
				repo_id INTEGER NOT NULL DEFAULT 0,
				created_at INTEGER,
				UNIQUE(secret_id, org_id, repo_id),
				FOREIGN KEY (secret_id) REFERENCES secrets(id) ON DELETE CASCADE
			)`
		default:
			ddl = `CREATE TABLE IF NOT EXISTS secret_targets (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				secret_id INTEGER NOT NULL REFERENCES secrets(id) ON DELETE CASCADE,
				org_id INTEGER NOT NULL DEFAULT 0,
				repo_id INTEGER NOT NULL DEFAULT 0,
				created_at INTEGER,
				UNIQUE(secret_id, org_id, repo_id)
			)`
		}
		_, err := tx.Exec(ddl)
		return err
	}
}

// createProcLogEntries creates the proc_log_entries table for streaming process
// logs from agents and autoscalers into the database. `time` holds Unix
// nanoseconds and `source_id` mirrors model int64 IDs, so both must be 64-bit
// on PostgreSQL and MySQL — a plain INTEGER (int32) overflows on every insert.
func createProcLogEntries(driver string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		var ddl string
		switch driver {
		case "postgres":
			ddl = `CREATE TABLE IF NOT EXISTS proc_log_entries (
				id BIGSERIAL PRIMARY KEY,
				source_type TEXT NOT NULL,
				source_id BIGINT NOT NULL,
				time BIGINT NOT NULL,
				level TEXT NOT NULL,
				line TEXT NOT NULL
			)`
		case "mysql":
			ddl = `CREATE TABLE IF NOT EXISTS proc_log_entries (
				id BIGINT PRIMARY KEY AUTO_INCREMENT,
				source_type TEXT NOT NULL,
				source_id BIGINT NOT NULL,
				time BIGINT NOT NULL,
				level TEXT NOT NULL,
				line TEXT NOT NULL
			)`
		default:
			ddl = `CREATE TABLE IF NOT EXISTS proc_log_entries (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				source_type TEXT NOT NULL,
				source_id INTEGER NOT NULL,
				time INTEGER NOT NULL,
				level TEXT NOT NULL,
				line TEXT NOT NULL
			)`
		}
		if _, err := tx.Exec(ddl); err != nil {
			return err
		}
		if driver == "mysql" {
			// MySQL does not allow indexing TEXT columns without a prefix length,
			// and does not support CREATE INDEX IF NOT EXISTS. Check INFORMATION_SCHEMA
			// first so this migration is idempotent on re-run or across shared test DBs.
			var count int
			if err := tx.QueryRow(
				`SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS` +
					` WHERE TABLE_SCHEMA = DATABASE()` +
					` AND TABLE_NAME = 'proc_log_entries'` +
					` AND INDEX_NAME = 'idx_proc_log_entries_source_id'`,
			).Scan(&count); err != nil {
				return err
			}
			if count == 0 {
				if _, err := tx.Exec(`CREATE INDEX idx_proc_log_entries_source_id ON proc_log_entries (source_type(32), source_id, id)`); err != nil {
					return err
				}
			}
			return nil
		}
		_, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_proc_log_entries_source_id ON proc_log_entries (source_type, source_id, id)`)
		return err
	}
}

// fixProcLogInt64Columns widens proc_log_entries.time and .source_id from 32-bit
// to 64-bit on existing PostgreSQL and MySQL databases. The original migration
// declared these as INTEGER, which overflows on every insert because `time`
// stores Unix nanoseconds (~1.78e18 today). SQLite's INTEGER is dynamic and
// already stores int64, so no action is needed there.
func fixProcLogInt64Columns(driver string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		if !tableExists(tx, "proc_log_entries") {
			return nil
		}
		switch driver {
		case "postgres":
			_, err := tx.Exec(`ALTER TABLE proc_log_entries
				ALTER COLUMN time TYPE BIGINT,
				ALTER COLUMN source_id TYPE BIGINT`)
			return err
		case "mysql":
			_, err := tx.Exec(`ALTER TABLE proc_log_entries
				MODIFY time BIGINT NOT NULL,
				MODIFY source_id BIGINT NOT NULL`)
			return err
		}
		return nil
	}
}

// tableExists checks whether a table exists in the database using a savepoint
// so that a failed probe doesn't poison the surrounding transaction.
// Uses QueryRow+Scan to fully consume the result inline, avoiding open result
// sets that break MySQL's single-result-set-per-connection constraint.
//
// An empty table returns sql.ErrNoRows from Scan but the table still exists,
// so treat that as "exists" — mirroring addColumnIfNotExists. Treating empty
// tables as missing previously caused crow-fix-proc-log-int64-columns to
// silently no-op on every deployment, since proc_log_entries had no rows yet.
func tableExists(tx *sql.Tx, table string) bool {
	_, _ = tx.Exec("SAVEPOINT table_check")
	var dummy int
	err := tx.QueryRow(fmt.Sprintf("SELECT 1 FROM %s LIMIT 1", table)).Scan(&dummy) //nolint:gosec
	if err == nil || errors.Is(err, sql.ErrNoRows) {
		_, _ = tx.Exec("RELEASE SAVEPOINT table_check")
		return true
	}
	_, _ = tx.Exec("ROLLBACK TO SAVEPOINT table_check")
	return false
}

// columnExists reports whether a column exists, using a savepoint so a failed
// probe doesn't poison the surrounding transaction (mirrors tableExists).
func columnExists(tx *sql.Tx, table, column string) bool {
	_, _ = tx.Exec("SAVEPOINT col_check")
	var dummy any
	err := tx.QueryRow(fmt.Sprintf("SELECT %s FROM %s LIMIT 1", column, table)).Scan(&dummy) //nolint:gosec
	if err == nil || errors.Is(err, sql.ErrNoRows) {
		_, _ = tx.Exec("RELEASE SAVEPOINT col_check")
		return true
	}
	_, _ = tx.Exec("ROLLBACK TO SAVEPOINT col_check")
	return false
}

// dropIndexIfExists drops an index in a driver-portable, idempotent way.
func dropIndexIfExists(tx *sql.Tx, driver, index, table string) error {
	if driver == "mysql" {
		var count int
		if err := tx.QueryRow(
			`SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS`+
				` WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?`,
			table, index,
		).Scan(&count); err != nil {
			return fmt.Errorf("probe index %s: %w", index, err)
		}
		if count == 0 {
			return nil
		}
		if _, err := tx.Exec(fmt.Sprintf("DROP INDEX %s ON %s", index, table)); err != nil {
			return fmt.Errorf("drop index %s: %w", index, err)
		}
		return nil
	}
	if _, err := tx.Exec("DROP INDEX IF EXISTS " + index); err != nil {
		return fmt.Errorf("drop index %s: %w", index, err)
	}
	return nil
}

// migrateAgentOrgIDs adds the org_ids list column, backfills it from the legacy
// single org_id column (-1 -> global/empty list, otherwise [org_id]), and drops
// the old org_id column and its index. Fresh databases get org_ids from the
// baseline schema and skip this (tableExists guard plus the runner's applied set).
func migrateAgentOrgIDs(driver string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		if !tableExists(tx, "agents") {
			return nil
		}

		if err := addColumnIfNotExists("agents", "org_ids", "TEXT")(tx); err != nil {
			return fmt.Errorf("add agents.org_ids: %w", err)
		}

		// A fresh-but-migrated DB may already lack org_id (baseline). If the
		// legacy column is gone there is nothing to backfill or drop.
		if !columnExists(tx, "agents", "org_id") {
			return nil
		}

		rows, err := tx.Query("SELECT id, org_id FROM agents")
		if err != nil {
			return fmt.Errorf("read agents for org_ids backfill: %w", err)
		}
		defer rows.Close()
		type agentRow struct{ id, orgID int64 }
		var pending []agentRow
		for rows.Next() {
			var r agentRow
			if err := rows.Scan(&r.id, &r.orgID); err != nil {
				return fmt.Errorf("scan agent row: %w", err)
			}
			pending = append(pending, r)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		rows.Close()

		// Postgres uses $N placeholders; SQLite and MySQL use ?.
		updateStmt := "UPDATE agents SET org_ids = ? WHERE id = ?"
		if driver == "postgres" {
			updateStmt = "UPDATE agents SET org_ids = $1 WHERE id = $2"
		}
		for _, r := range pending {
			orgIDs := "[]"
			if r.orgID >= 0 {
				orgIDs = fmt.Sprintf("[%d]", r.orgID)
			}
			if _, err := tx.Exec(updateStmt, orgIDs, r.id); err != nil {
				return fmt.Errorf("backfill agent %d org_ids: %w", r.id, err)
			}
		}

		if err := dropIndexIfExists(tx, driver, "idx_agents_org_id", "agents"); err != nil {
			return err
		}
		if _, err := tx.Exec("ALTER TABLE agents DROP COLUMN org_id"); err != nil {
			return fmt.Errorf("drop agents.org_id: %w", err)
		}
		return nil
	}
}

// addPipelineOrgIdentity adds the denormalized org_id/org_name/repo_full_name
// columns to the pipelines table and backfills them from the owning repo. This
// lets metrics resolve org/repo names directly from the pipeline row, so they
// survive repo/org renames and deletion (and no longer render personal-account
// orgs as "Unknown"). Fresh databases get the columns from the baseline schema
// and skip this via the tableExists guard plus the runner's applied set.
// Adds the host resource totals reported by agents.
func addAgentHostResources(tx *sql.Tx) error {
	if !tableExists(tx, "agents") {
		return nil
	}

	if err := addColumnIfNotExists("agents", "host_cpus", "INTEGER")(tx); err != nil {
		return fmt.Errorf("add agents.host_cpus: %w", err)
	}
	// BIGINT: a byte count exceeds the 32-bit INTEGER range of Postgres and MySQL.
	if err := addColumnIfNotExists("agents", "host_mem", "BIGINT")(tx); err != nil {
		return fmt.Errorf("add agents.host_mem: %w", err)
	}

	return nil
}

// addAgentProtocolVersion adds agents.protocol_version, the agent protocol
// version reported on registration, so the UI can flag outdated agents.
func addAgentProtocolVersion(tx *sql.Tx) error {
	if !tableExists(tx, "agents") {
		return nil
	}
	return addColumnIfNotExists("agents", "protocol_version", "INTEGER")(tx)
}

// addAgentRemoteUpdate adds agents.remote_update, the agent's reported remote
// update mode, agents.restart_requested, set when the agent is asked to
// restart onto a newer release, agents.restart_queued, set while a restart
// waits for a free slot, and agents.auto_restart_version, the server release
// under which the automatic restart policy last asked.
func addAgentRemoteUpdate(tx *sql.Tx) error {
	if !tableExists(tx, "agents") {
		return nil
	}
	if err := addColumnIfNotExists("agents", "remote_update", "VARCHAR(20)")(tx); err != nil {
		return fmt.Errorf("add agents.remote_update: %w", err)
	}
	if err := addColumnIfNotExists("agents", "restart_requested", "BOOLEAN")(tx); err != nil {
		return fmt.Errorf("add agents.restart_requested: %w", err)
	}
	if err := addColumnIfNotExists("agents", "restart_queued", "BOOLEAN")(tx); err != nil {
		return fmt.Errorf("add agents.restart_queued: %w", err)
	}
	if err := addColumnIfNotExists("agents", "auto_restart_version", "VARCHAR(255)")(tx); err != nil {
		return fmt.Errorf("add agents.auto_restart_version: %w", err)
	}
	return nil
}

// addProvisioningProgress adds agents.provisioning, the phases a host reported
// while being set up, and autoscalers.pending_agent_ids and
// autoscalers.provision_timeout, reported in autoscaler heartbeats.
func addProvisioningProgress(tx *sql.Tx) error {
	if tableExists(tx, "agents") {
		if err := addColumnIfNotExists("agents", "provisioning", "TEXT")(tx); err != nil {
			return fmt.Errorf("add agents.provisioning: %w", err)
		}
	}
	if tableExists(tx, "autoscalers") {
		if err := addColumnIfNotExists("autoscalers", "pending_agent_ids", "TEXT")(tx); err != nil {
			return fmt.Errorf("add autoscalers.pending_agent_ids: %w", err)
		}
		if err := addColumnIfNotExists("autoscalers", "provision_timeout", "BIGINT")(tx); err != nil {
			return fmt.Errorf("add autoscalers.provision_timeout: %w", err)
		}
	}
	return nil
}

// addUserRoles adds users.roles, the JSON list of instance roles besides admin.
// The column is nullable because MySQL rejects a DEFAULT on TEXT; NULL reads
// as no roles.
func addUserRoles(tx *sql.Tx) error {
	if !tableExists(tx, "users") {
		return nil
	}
	return addColumnIfNotExists("users", "roles", "TEXT")(tx)
}

// addIntegrationSecretFilters adds integrations.events and integrations.images,
// the JSON events and images filters applied to secrets fetched through an
// integration. The columns are nullable because MySQL rejects a DEFAULT on TEXT;
// NULL events read as the default event set, NULL images as no image filter.
func addIntegrationSecretFilters(tx *sql.Tx) error {
	if !tableExists(tx, "integrations") {
		return nil
	}
	if err := addColumnIfNotExists("integrations", "events", "TEXT")(tx); err != nil {
		return err
	}
	return addColumnIfNotExists("integrations", "images", "TEXT")(tx)
}

func addPipelineOrgIdentity(driver string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		if !tableExists(tx, "pipelines") {
			return nil
		}

		if err := addColumnIfNotExists("pipelines", "org_id", "INTEGER")(tx); err != nil {
			return fmt.Errorf("add pipelines.org_id: %w", err)
		}
		if err := addColumnIfNotExists("pipelines", "org_name", "TEXT")(tx); err != nil {
			return fmt.Errorf("add pipelines.org_name: %w", err)
		}
		if err := addColumnIfNotExists("pipelines", "repo_full_name", "TEXT")(tx); err != nil {
			return fmt.Errorf("add pipelines.repo_full_name: %w", err)
		}

		// Backfill from repos using correlated subqueries (portable across
		// SQLite, MySQL and Postgres — avoids the UPDATE...FROM dialect split).
		// The WHERE EXISTS guard leaves any pre-existing orphaned pipeline
		// (no matching repo) untouched instead of nulling its identity.
		if _, err := tx.Exec(`UPDATE pipelines
			SET org_id         = (SELECT r.org_id    FROM repos r WHERE r.id = pipelines.repo_id),
			org_name       = (SELECT r.owner     FROM repos r WHERE r.id = pipelines.repo_id),
			repo_full_name = (SELECT r.full_name FROM repos r WHERE r.id = pipelines.repo_id)
			WHERE EXISTS (SELECT 1 FROM repos r WHERE r.id = pipelines.repo_id)`); err != nil {
			return fmt.Errorf("backfill pipelines org identity: %w", err)
		}

		// Index used by the org-grouped metrics aggregations.
		const indexName = "idx_pipelines_org_id"
		if driver == "mysql" {
			var count int
			if err := tx.QueryRow(
				`SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS`+
					` WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pipelines' AND INDEX_NAME = ?`,
				indexName,
			).Scan(&count); err != nil {
				return fmt.Errorf("probe %s: %w", indexName, err)
			}
			if count == 0 {
				if _, err := tx.Exec(`CREATE INDEX ` + indexName + ` ON pipelines (org_id)`); err != nil {
					return fmt.Errorf("create %s: %w", indexName, err)
				}
			}
			return nil
		}
		if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS ` + indexName + ` ON pipelines (org_id)`); err != nil {
			return fmt.Errorf("create %s: %w", indexName, err)
		}
		return nil
	}
}

// backfillWorkflowAgentIdentity copies agent_name/agent_persistent_id/platform
// onto legacy workflow rows (those written before the agent identity was
// denormalized) from the still-existing agent row. Workflows whose agent has
// already been deleted cannot be recovered and keep their empty identity; the
// metrics layer labels those "Agent #<id>" instead of collapsing them into a
// single "Unknown" bucket.
func backfillWorkflowAgentIdentity(_ string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		if !tableExists(tx, "workflows") || !tableExists(tx, "agents") {
			return nil
		}

		if _, err := tx.Exec(`UPDATE workflows
			SET agent_name          = (SELECT a.name          FROM agents a WHERE a.id = workflows.agent_id),
			agent_persistent_id = (SELECT a.persistent_id FROM agents a WHERE a.id = workflows.agent_id),
			platform            = (SELECT a.platform      FROM agents a WHERE a.id = workflows.agent_id)
			WHERE (agent_persistent_id IS NULL OR agent_persistent_id = '')
			AND agent_id > 0
			AND EXISTS (SELECT 1 FROM agents a WHERE a.id = workflows.agent_id)`); err != nil {
			return fmt.Errorf("backfill workflow agent identity: %w", err)
		}
		return nil
	}
}

// addLogEntriesUniqueIndex removes any pre-existing duplicate (step_id, line)
// rows (the old racy SELECT-then-INSERT dedup could leave some) and then adds a
// UNIQUE index on (step_id, line). LogAppend relies on this index for atomic
// INSERT ... ON CONFLICT DO NOTHING deduplication. Fresh databases already get
// the index from the baseline schema and skip this via the runner's applied set.
func addLogEntriesUniqueIndex(driver string) func(tx *sql.Tx) error {
	return func(tx *sql.Tx) error {
		if !tableExists(tx, "log_entries") {
			return nil
		}

		// Delete duplicate rows, keeping the lowest id per (step_id, line).
		//
		// Only rows in a group that actually has duplicates are touched: the inner
		// GROUP BY ... HAVING COUNT(*) > 1 yields just the duplicated (step_id, line)
		// keys (typically a handful even on huge tables), and the join deletes the
		// non-minimum ids within those groups. This is a single pass over the table
		// plus a tiny hash join, versus the old `id NOT IN (SELECT MIN(id) ... GROUP
		// BY ...)` form which the planner executed as a per-row anti-join against a
		// materialized set of every group — effectively O(n^2) and multi-minute on a
		// table with millions of log lines, blowing the startup window.
		//
		// The extra derived table (aliased "x") materializes the id list before the
		// DELETE so MySQL does not reject reading the delete target in a subquery
		// (error 1093). NULL step_id/line rows never match the self-join equality, so
		// they are left alone — correct, since a UNIQUE index treats NULLs as
		// distinct across SQLite, PostgreSQL and MySQL.
		if _, err := tx.Exec(`DELETE FROM log_entries
			WHERE id IN (
				SELECT del_id FROM (
					SELECT le.id AS del_id
					FROM log_entries le
					JOIN (
						SELECT step_id, line, MIN(id) AS keep_id
						FROM log_entries
						GROUP BY step_id, line
						HAVING COUNT(*) > 1
					) dup ON le.step_id = dup.step_id AND le.line = dup.line
					WHERE le.id <> dup.keep_id
				) AS x
			)`); err != nil {
			return fmt.Errorf("dedup log_entries before unique index: %w", err)
		}

		const indexName = "idx_log_entries_step_id_line"
		if driver == "mysql" {
			// MySQL has no CREATE UNIQUE INDEX IF NOT EXISTS; probe first.
			var count int
			if err := tx.QueryRow(
				`SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS`+
					` WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'log_entries' AND INDEX_NAME = ?`,
				indexName,
			).Scan(&count); err != nil {
				return fmt.Errorf("probe %s: %w", indexName, err)
			}
			if count == 0 {
				if _, err := tx.Exec(`CREATE UNIQUE INDEX ` + indexName + ` ON log_entries (step_id, line)`); err != nil {
					return fmt.Errorf("create %s: %w", indexName, err)
				}
			}
			return nil
		}

		if _, err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS ` + indexName + ` ON log_entries (step_id, line)`); err != nil {
			return fmt.Errorf("create %s: %w", indexName, err)
		}
		return nil
	}
}
