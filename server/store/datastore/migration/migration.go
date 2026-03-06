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
	"fmt"
	"reflect"

	"github.com/rs/zerolog/log"
	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"

	"codefloe.com/crowci/crow/v5/server/model"
)

// migrationTasks contains all database migrations in execution order.
// They are executed sequentially and if one fails, xormigrate will try to rollback
// that specific migration and exit.
//
// ADDING NEW MIGRATIONS:
// 1. Create a new file: NNN_crow_description.go (e.g., 043_crow_new_feature.go)
// 2. Use the "crow-" prefix for the migration ID to avoid conflicts with Woodpecker:
//
//	var crowNewFeature = xormigrate.Migration{
//	    ID: "crow-new-feature",
//	    MigrateSession: func(sess *xorm.Session) error {
//	        // migration logic
//	        return nil
//	    },
//	}
//
// 3. Add the migration to this slice below.
// 4. Update constants.go crowMigrations list with the new ID.
var migrationTasks = []*xormigrate.Migration{
	&legacyToXormigrate,
	&addOrgID,
	&alterTableTasksUpdateColumnTaskDataType,
	&alterTableConfigUpdateColumnConfigDataType,
	&removePluginOnlyOptionFromSecretsTable,
	&convertToNewPipelineErrorFormat,
	&renameLinkToURL,
	&cleanRegistryPipeline,
	&setForgeID,
	&unifyColumnsTables,
	&alterTableRegistriesFixRequiredFields,
	&cronWithoutSec,
	&renameStartEndTime,
	&fixV31Registries,
	&removeOldMigrationsOfV1,
	&addOrgAgents,
	&addCustomLabelsToAgent,
	&splitTrusted,
	&correctPotentialCorruptOrgsUsersRelation,
	&gatedToRequireApproval,
	&removeRepoNetrcOnlyTrusted,
	&renameTokenFields,
	&setNewDefaultsForRequireApproval,
	&removeRepoScm,
	&unsanitizeOrgAndUserNames,
	&addLogsDeletedToPipelines,
	&addLogsKeepToRepos,
	&replaceZeroForgeIDsInOrgs,
	&addDependsOnToSteps,
	&addWorkflowFields,
	&addMaintenanceTables,
	&addDistributedLocks,
	&addPubSubMessages,
	&addCacheEntries,
	&addKubernetesMaintenanceActions,
	&addDryRunToMaintenanceConfig,
	&addPriorityToAgents,
	&addStaleAgentCleanup,
	&addResourceLimitsToAgents,
	&addWorkflowPollingHealthToAgents,
	&addAutoscalers,
	&addUserDescriptionTimezone,
	&addSecretSource,
	&addTimestamps,
	&addAccessTokens,
	&addAgentPersistentID,
	&crowAddDeployTeamToRepos,
	&crowAddIntegrations,
	&crowAddAgentHostname,
	&crowAddUserForges,
	&crowAddForgeIcon,
	&crowAddForgeToRepoUnique,
	&crowAddAllowManualToRepos,
	&crowAddWorkflowRerunFields,
	&crowAddApprovedPullRequests,
}

var allBeans = []any{
	new(model.Agent),
	new(model.Pipeline),
	new(model.PipelineConfig),
	new(model.Config),
	new(model.LogEntry),
	new(model.Perm),
	new(model.Step),
	new(model.Registry),
	new(model.Repo),
	new(model.Secret),
	new(model.Task),
	new(model.User),
	new(model.ServerConfig),
	new(model.Cron),
	new(model.Redirection),
	new(model.Forge),
	new(model.Workflow),
	new(model.Org),
	new(model.MaintenanceConfig),
	new(model.MaintenanceStats),
	new(model.MaintenanceLog),
	new(model.DistributedLock),
	new(model.DistributedMessage),
	new(model.CacheEntry),
	new(model.Autoscaler),
	new(model.AccessToken),
	new(model.Integration),
	new(model.UserForge),
	new(model.ApprovedPullRequest),
}

// TODO: make xormigrate context aware
func Migrate(_ context.Context, e *xorm.Engine, allowLong bool) error {
	e.SetDisableGlobalCache(true)

	m := xormigrate.New(e, migrationTasks)
	m.AllowLong(allowLong)

	oldExist, err := e.IsTableExist("migrations")
	if err != nil {
		return err
	}

	oldEmpty := false
	if oldExist {
		oldEmpty, err = e.IsTableEmpty("migrations")
		if err != nil {
			return err
		}
	}

	if !oldExist || oldEmpty {
		// allow new schema initialization if old migrations table is empty or it does not exist (err != nil)
		// schema initialization will always run if we call `InitSchema`
		m.InitSchema(func(_ *xorm.Engine) error {
			// do nothing on schema init, models are synced in any case below
			return nil
		})
	}

	m.SetLogger(&xormigrateLogger{})

	// Detect database origin (Woodpecker vs Crow) and log migration info
	migrationInfo, err := detectDatabaseOrigin(e)
	if err != nil {
		return fmt.Errorf("failed to detect database origin: %w", err)
	}
	logMigrationInfo(migrationInfo)

	if err := m.Migrate(); err != nil {
		return err
	}

	// Log completion message for Woodpecker migrations
	if migrationInfo.IsFirstCrowMigration {
		log.Info().Msg("Successfully migrated Woodpecker database to Crow. Your data has been preserved.")
	}

	e.SetDisableGlobalCache(false)

	if err := syncAll(e); err != nil {
		return fmt.Errorf("msg: %w", err)
	}

	return nil
}

func syncAll(sess *xorm.Engine) error {
	for _, bean := range allBeans {
		if err := sess.Sync(bean); err != nil {
			return fmt.Errorf("sync error '%s': %w", reflect.TypeOf(bean), err)
		}
	}
	return nil
}
