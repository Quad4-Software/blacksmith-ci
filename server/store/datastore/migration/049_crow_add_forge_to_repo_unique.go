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
	"fmt"

	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// crowAddForgeToRepoUnique updates unique constraints on repos and redirections
// tables to include forge_id. This is required for multi-forge support where
// the same repository name (e.g., owner/repo) can exist on different forges.
var crowAddForgeToRepoUnique = xormigrate.Migration{
	ID: "crow-add-forge-to-repo-unique",
	MigrateSession: func(sess *xorm.Session) error {
		dialect := sess.Engine().Dialect().URI().DBType

		// Update repos table unique constraints
		if err := updateReposUniqueConstraints(sess, dialect); err != nil {
			return fmt.Errorf("failed to update repos unique constraints: %w", err)
		}

		// Add forge_id to redirections table and update unique constraint
		if err := updateRedirectionsTable(sess, dialect); err != nil {
			return fmt.Errorf("failed to update redirections table: %w", err)
		}

		return nil
	},
}

func updateReposUniqueConstraints(sess *xorm.Session, dialect schemas.DBType) error {
	switch dialect {
	case schemas.SQLITE:
		return updateReposUniqueConstraintsSQLite(sess)
	case schemas.POSTGRES:
		return updateReposUniqueConstraintsPostgres(sess)
	case schemas.MYSQL:
		return updateReposUniqueConstraintsMySQL(sess)
	default:
		return fmt.Errorf("dialect '%s' not supported", dialect)
	}
}

func updateReposUniqueConstraintsSQLite(sess *xorm.Session) error {
	// SQLite requires recreating the table to change constraints
	// Get current table schema
	var result []struct {
		SQL string `xorm:"sql"`
	}
	if err := sess.SQL("SELECT sql FROM sqlite_master WHERE type='table' AND name='repos'").Find(&result); err != nil {
		return err
	}
	if len(result) == 0 {
		return fmt.Errorf("repos table not found")
	}

	// Drop old unique indexes if they exist
	indexes := []string{
		"UQE_repos_full_name",
		"UQE_repos_name",
	}
	for _, idx := range indexes {
		if _, err := sess.Exec(fmt.Sprintf("DROP INDEX IF EXISTS `%s`", idx)); err != nil {
			return err
		}
	}

	// Create new composite unique indexes
	if _, err := sess.Exec("CREATE UNIQUE INDEX IF NOT EXISTS `UQE_repos_forge` ON `repos` (`forge_id`, `owner`, `name`)"); err != nil {
		return err
	}
	if _, err := sess.Exec("CREATE UNIQUE INDEX IF NOT EXISTS `UQE_repos_forge_full_name` ON `repos` (`forge_id`, `full_name`)"); err != nil {
		return err
	}

	return nil
}

func updateReposUniqueConstraintsPostgres(sess *xorm.Session) error {
	// Drop old constraints
	constraints := []string{
		"repos_owner_name_key",
		"repos_full_name_key",
		"UQE_repos_name",
		"UQE_repos_full_name",
	}
	for _, c := range constraints {
		// Use IF EXISTS to handle cases where constraint may not exist
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE repos DROP CONSTRAINT IF EXISTS %s", c)); err != nil {
			return err
		}
	}

	// Drop old indexes
	indexes := []string{
		"UQE_repos_name",
		"UQE_repos_full_name",
	}
	for _, idx := range indexes {
		if _, err := sess.Exec(fmt.Sprintf("DROP INDEX IF EXISTS %s", idx)); err != nil {
			return err
		}
	}

	// Create new composite unique constraints
	if _, err := sess.Exec("CREATE UNIQUE INDEX IF NOT EXISTS UQE_repos_forge ON repos (forge_id, owner, name)"); err != nil {
		return err
	}
	if _, err := sess.Exec("CREATE UNIQUE INDEX IF NOT EXISTS UQE_repos_forge_full_name ON repos (forge_id, full_name)"); err != nil {
		return err
	}

	return nil
}

func updateReposUniqueConstraintsMySQL(sess *xorm.Session) error {
	// Check and drop existing indexes
	indexes := []string{
		"UQE_repos_name",
		"UQE_repos_full_name",
		"full_name",
		"name",
	}

	for _, idx := range indexes {
		// Check if index exists
		var count int64
		countSQL := fmt.Sprintf("SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'repos' AND index_name = '%s'", idx)
		if _, err := sess.SQL(countSQL).Get(&count); err != nil {
			return err
		}
		if count > 0 {
			if _, err := sess.Exec(fmt.Sprintf("DROP INDEX `%s` ON `repos`", idx)); err != nil {
				return err
			}
		}
	}

	// Create new composite unique indexes
	if _, err := sess.Exec("CREATE UNIQUE INDEX `UQE_repos_forge` ON `repos` (`forge_id`, `owner`, `name`)"); err != nil {
		return err
	}
	if _, err := sess.Exec("CREATE UNIQUE INDEX `UQE_repos_forge_full_name` ON `repos` (`forge_id`, `full_name`)"); err != nil {
		return err
	}

	return nil
}

func updateRedirectionsTable(sess *xorm.Session, dialect schemas.DBType) error {
	// First, add forge_id column to redirections table
	// Note: If repo_full_name column doesn't exist yet (fresh database), skip index creation.
	// The syncAll() at the end of migration will create the column with correct constraints.
	switch dialect {
	case schemas.SQLITE:
		// Check what columns exist
		var columns []struct {
			Name string `xorm:"name"`
		}
		if err := sess.SQL("PRAGMA table_info(redirections)").Find(&columns); err != nil {
			return err
		}
		hasForgeID := false
		hasRepoFullName := false
		for _, col := range columns {
			if col.Name == "forge_id" {
				hasForgeID = true
			}
			if col.Name == "repo_full_name" {
				hasRepoFullName = true
			}
		}
		if !hasForgeID {
			if _, err := sess.Exec("ALTER TABLE `redirections` ADD COLUMN `forge_id` INTEGER NOT NULL DEFAULT 1"); err != nil {
				return err
			}
		}

		// Only update indexes if repo_full_name column exists
		// If it doesn't exist, syncAll will create the table with correct schema
		if hasRepoFullName {
			// Drop old unique index and create new one
			if _, err := sess.Exec("DROP INDEX IF EXISTS `UQE_redirections_repo_full_name`"); err != nil {
				return err
			}
			if _, err := sess.Exec("CREATE UNIQUE INDEX IF NOT EXISTS `UQE_redirections_forge` ON `redirections` (`forge_id`, `repo_full_name`)"); err != nil {
				return err
			}
		}

	case schemas.POSTGRES:
		// Check if columns exist
		var forgeIDCount int64
		if _, err := sess.SQL("SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'redirections' AND column_name = 'forge_id'").Get(&forgeIDCount); err != nil {
			return err
		}
		if forgeIDCount == 0 {
			if _, err := sess.Exec("ALTER TABLE redirections ADD COLUMN forge_id BIGINT NOT NULL DEFAULT 1"); err != nil {
				return err
			}
		}

		var repoFullNameCount int64
		if _, err := sess.SQL("SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'redirections' AND column_name = 'repo_full_name'").Get(&repoFullNameCount); err != nil {
			return err
		}

		// Only update indexes if repo_full_name column exists
		if repoFullNameCount > 0 {
			// Drop old constraint/index and create new one
			if _, err := sess.Exec("DROP INDEX IF EXISTS UQE_redirections_repo_full_name"); err != nil {
				return err
			}
			if _, err := sess.Exec("ALTER TABLE redirections DROP CONSTRAINT IF EXISTS redirections_repo_full_name_key"); err != nil {
				return err
			}
			if _, err := sess.Exec("CREATE UNIQUE INDEX IF NOT EXISTS UQE_redirections_forge ON redirections (forge_id, repo_full_name)"); err != nil {
				return err
			}
		}

	case schemas.MYSQL:
		// Check if columns exist
		var forgeIDCount int64
		if _, err := sess.SQL("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'redirections' AND column_name = 'forge_id'").Get(&forgeIDCount); err != nil {
			return err
		}
		if forgeIDCount == 0 {
			if _, err := sess.Exec("ALTER TABLE `redirections` ADD COLUMN `forge_id` BIGINT NOT NULL DEFAULT 1"); err != nil {
				return err
			}
		}

		var repoFullNameCount int64
		if _, err := sess.SQL("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'redirections' AND column_name = 'repo_full_name'").Get(&repoFullNameCount); err != nil {
			return err
		}

		// Only update indexes if repo_full_name column exists
		if repoFullNameCount > 0 {
			// Check and drop old index
			var idxCount int64
			if _, err := sess.SQL("SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'redirections' AND index_name = 'UQE_redirections_repo_full_name'").Get(&idxCount); err != nil {
				return err
			}
			if idxCount > 0 {
				if _, err := sess.Exec("DROP INDEX `UQE_redirections_repo_full_name` ON `redirections`"); err != nil {
					return err
				}
			}

			// Create new composite unique index
			if _, err := sess.Exec("CREATE UNIQUE INDEX `UQE_redirections_forge` ON `redirections` (`forge_id`, `repo_full_name`)"); err != nil {
				return err
			}
		}

	default:
		return fmt.Errorf("dialect '%s' not supported", dialect)
	}

	return nil
}
