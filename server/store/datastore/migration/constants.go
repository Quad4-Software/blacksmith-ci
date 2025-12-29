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

// sharedMigrations contains migration IDs that are shared between
// Woodpecker CI and Crow CI. These migrations originated in Woodpecker
// and were inherited by Crow during the fork.
//
// Note: The order doesn't matter for detection - we only check presence.
var sharedMigrations = []string{
	"legacy-to-xormigrate",
	"add-org-id",
	"alter-table-tasks-update-type-of-task-data",
	"alter-table-config-update-type-of-config-data",
	"remove-plugin-only-option-from-secrets-table",
	"convert-to-new-pipeline-error-format",
	"rename-link-to-url",
	"clean-registry-pipeline",
	"set-forge-id",
	"unify-columns-tables",
	"alter-table-registries-fix-required-fields",
	"cron-without-sec",
	"rename-start-end-time",
	"fix-v31-registries",
	"remove-old-migrations-of-v1",
	"add-org-agents",
	"add-custom-labels-to-agent",
	"split-trusted",
	"correct-potential-corrupt-orgs-users-relation",
	"gated-to-require-approval",
	"remove-repo-netrc-only-trusted",
	"rename-token-fields",
	"set-new-defaults-for-require-approval",
	"remove-repo-scm",
	"unsanitize-org-and-user-names",
	"add-logs-deleted-to-pipelines",
	"add-logs-keep-to-repos",
	"replace-zero-forge-ids-in-orgs",
}

// crowMigrations contains migration IDs that are exclusive to Crow CI.
// These migrations implement Crow-specific features like HA mode,
// maintenance system, autoscalers, etc.
//
// When adding new migrations, use the "crow-" prefix to avoid future
// conflicts with Woodpecker migrations:
//
//	var crowNewFeature = xormigrate.Migration{
//		ID: "crow-new-feature",
//		// ...
//	}
var crowMigrations = []string{
	"add-depends-on-to-steps",
	"add-workflow-fields",
	"add-maintenance-tables",
	"add-distributed-locks",
	"add-pubsub-messages",
	"add-cache-entries",
	"add-kubernetes-maintenance-actions",
	"add-dry-run-to-maintenance-config",
	"add-priority-to-agents",
	"add-stale-agent-cleanup",
	"add-resource-limits-to-agents",
	"add-workflow-polling-health-to-agents",
	"add-autoscalers",
	"add-user-description-timezone",
	"add-secret-source",
	"add-timestamps",
	"add-access-tokens",
	"crow-add-agent-persistent-id",
}

// allKnownMigrations returns a combined list of all known migration IDs.
// Used to identify unknown migrations in the database.
func allKnownMigrations() []string {
	result := make([]string, 0, len(sharedMigrations)+len(crowMigrations))
	result = append(result, sharedMigrations...)
	result = append(result, crowMigrations...)
	return result
}
