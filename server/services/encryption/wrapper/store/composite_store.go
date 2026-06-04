// Copyright 2024 Woodpecker Authors
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

package store

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/services/encryption/types"
	"codefloe.com/crowci/crow/v5/server/store"
)

// CompositeEncryptedStore combines all encryption wrappers into a single store.Store implementation.
// It delegates encrypted operations to the appropriate wrapper and passes through all other operations.
type CompositeEncryptedStore struct {
	baseStore      store.Store
	secretStore    *EncryptedSecretStore
	registryStore  *EncryptedRegistryStore
	userStore      *EncryptedUserStore
	forgeStore     *EncryptedForgeStore
	userForgeStore *EncryptedUserForgeStore
	encryption     types.EncryptionService
}

// Ensure CompositeEncryptedStore implements store.Store.
var _ store.Store = (*CompositeEncryptedStore)(nil)

// NewCompositeStore creates a new CompositeEncryptedStore with all wrappers.
func NewCompositeStore(
	baseStore store.Store,
	secretStore *EncryptedSecretStore,
	registryStore *EncryptedRegistryStore,
	userStore *EncryptedUserStore,
	forgeStore *EncryptedForgeStore,
	userForgeStore *EncryptedUserForgeStore,
) *CompositeEncryptedStore {
	return &CompositeEncryptedStore{
		baseStore:      baseStore,
		secretStore:    secretStore,
		registryStore:  registryStore,
		userStore:      userStore,
		forgeStore:     forgeStore,
		userForgeStore: userForgeStore,
	}
}

// SetEncryptionService configures encryption for all wrappers.
func (c *CompositeEncryptedStore) SetEncryptionService(service types.EncryptionService) error {
	if c.encryption != nil {
		return errors.New(errMessageInitSeveralTimes)
	}
	c.encryption = service

	if err := c.secretStore.SetEncryptionService(service); err != nil {
		return err
	}
	if err := c.registryStore.SetEncryptionService(service); err != nil {
		return err
	}
	if err := c.userStore.SetEncryptionService(service); err != nil {
		return err
	}
	if err := c.forgeStore.SetEncryptionService(service); err != nil {
		return err
	}
	if err := c.userForgeStore.SetEncryptionService(service); err != nil {
		return err
	}
	return nil
}

// EnableEncryption encrypts all existing data across all entity types.
func (c *CompositeEncryptedStore) EnableEncryption() error {
	log.Info().Msg("Enabling encryption for all sensitive data")

	if err := c.secretStore.EnableEncryption(); err != nil {
		return err
	}
	if err := c.registryStore.EnableEncryption(); err != nil {
		return err
	}
	if err := c.userStore.EnableEncryption(); err != nil {
		return err
	}
	if err := c.forgeStore.EnableEncryption(); err != nil {
		return err
	}
	if err := c.userForgeStore.EnableEncryption(); err != nil {
		return err
	}

	log.Info().Msg("All sensitive data encrypted successfully")
	return nil
}

// MigrateEncryption re-encrypts all data with a new encryption service.
func (c *CompositeEncryptedStore) MigrateEncryption(newEncryption types.EncryptionService) error {
	log.Info().Msg("Migrating encryption keys for all sensitive data")

	if err := c.secretStore.MigrateEncryption(newEncryption); err != nil {
		return err
	}
	if err := c.registryStore.MigrateEncryption(newEncryption); err != nil {
		return err
	}
	if err := c.userStore.MigrateEncryption(newEncryption); err != nil {
		return err
	}
	if err := c.forgeStore.MigrateEncryption(newEncryption); err != nil {
		return err
	}
	if err := c.userForgeStore.MigrateEncryption(newEncryption); err != nil {
		return err
	}

	c.encryption = newEncryption
	log.Info().Msg("Encryption migration completed successfully")
	return nil
}

// ============================================================================
// User operations - delegated to userStore
// ============================================================================.

func (c *CompositeEncryptedStore) GetUser(id int64) (*model.User, error) {
	return c.userStore.GetUser(id)
}

func (c *CompositeEncryptedStore) GetUserRemoteID(remoteID model.ForgeRemoteID, login string) (*model.User, error) {
	return c.userStore.GetUserRemoteID(remoteID, login)
}

func (c *CompositeEncryptedStore) GetUserLogin(login string) (*model.User, error) {
	return c.userStore.GetUserLogin(login)
}

func (c *CompositeEncryptedStore) GetUserList(p *model.ListOptions) ([]*model.User, error) {
	return c.userStore.GetUserList(p)
}

func (c *CompositeEncryptedStore) GetUserCount() (int64, error) {
	return c.userStore.GetUserCount()
}

func (c *CompositeEncryptedStore) CreateUser(user *model.User) error {
	return c.userStore.CreateUser(user)
}

func (c *CompositeEncryptedStore) UpdateUser(user *model.User) error {
	return c.userStore.UpdateUser(user)
}

func (c *CompositeEncryptedStore) DeleteUser(user *model.User) error {
	return c.userStore.DeleteUser(user)
}

func (c *CompositeEncryptedStore) UserListAll() ([]*model.User, error) {
	return c.userStore.UserListAll()
}

// ============================================================================
// Secret operations - delegated to secretStore
// ============================================================================.

func (c *CompositeEncryptedStore) SecretFind(repo *model.Repo, name string) (*model.Secret, error) {
	return c.secretStore.SecretFind(repo, name)
}

func (c *CompositeEncryptedStore) SecretFindByID(id int64) (*model.Secret, error) {
	return c.secretStore.SecretFindByID(id)
}

func (c *CompositeEncryptedStore) SecretList(repo *model.Repo, includeGlobalAndOrg bool, p *model.ListOptions) ([]*model.Secret, error) {
	return c.secretStore.SecretList(repo, includeGlobalAndOrg, p)
}

func (c *CompositeEncryptedStore) SecretListAll() ([]*model.Secret, error) {
	return c.secretStore.SecretListAll()
}

func (c *CompositeEncryptedStore) SecretCreate(secret *model.Secret) error {
	return c.secretStore.SecretCreate(secret)
}

func (c *CompositeEncryptedStore) SecretUpdate(secret *model.Secret) error {
	return c.secretStore.SecretUpdate(secret)
}

func (c *CompositeEncryptedStore) SecretDelete(secret *model.Secret) error {
	return c.secretStore.SecretDelete(secret)
}

func (c *CompositeEncryptedStore) OrgSecretFind(orgID int64, name string) (*model.Secret, error) {
	return c.secretStore.OrgSecretFind(orgID, name)
}

func (c *CompositeEncryptedStore) OrgSecretList(orgID int64, p *model.ListOptions) ([]*model.Secret, error) {
	return c.secretStore.OrgSecretList(orgID, p)
}

func (c *CompositeEncryptedStore) GlobalSecretFind(name string) (*model.Secret, error) {
	return c.secretStore.GlobalSecretFind(name)
}

func (c *CompositeEncryptedStore) GlobalSecretList(p *model.ListOptions) ([]*model.Secret, error) {
	return c.secretStore.GlobalSecretList(p)
}

func (c *CompositeEncryptedStore) UserSecretList(orgIDs []int64, repoIDs []int64, includeGlobal bool, searchTerm string, p *model.ListOptions) ([]*model.Secret, error) {
	// UserSecretList is part of store.Store, not model.SecretStore.
	// Call baseStore and decrypt using secretStore's DecryptList.
	results, err := c.baseStore.UserSecretList(orgIDs, repoIDs, includeGlobal, searchTerm, p)
	if err != nil {
		return nil, err
	}
	if err := c.secretStore.DecryptList(results); err != nil {
		return nil, err
	}
	return results, nil
}

// ============================================================================
// Registry operations - delegated to registryStore
// ============================================================================.

func (c *CompositeEncryptedStore) RegistryFind(repo *model.Repo, addr string) (*model.Registry, error) {
	return c.registryStore.RegistryFind(repo, addr)
}

func (c *CompositeEncryptedStore) RegistryList(repo *model.Repo, includeGlobalAndOrg bool, p *model.ListOptions) ([]*model.Registry, error) {
	return c.registryStore.RegistryList(repo, includeGlobalAndOrg, p)
}

func (c *CompositeEncryptedStore) RegistryListAll() ([]*model.Registry, error) {
	return c.registryStore.RegistryListAll()
}

func (c *CompositeEncryptedStore) RegistryCreate(registry *model.Registry) error {
	return c.registryStore.RegistryCreate(registry)
}

func (c *CompositeEncryptedStore) RegistryUpdate(registry *model.Registry) error {
	return c.registryStore.RegistryUpdate(registry)
}

func (c *CompositeEncryptedStore) RegistryDelete(registry *model.Registry) error {
	return c.registryStore.RegistryDelete(registry)
}

func (c *CompositeEncryptedStore) OrgRegistryFind(orgID int64, name string) (*model.Registry, error) {
	return c.registryStore.OrgRegistryFind(orgID, name)
}

func (c *CompositeEncryptedStore) OrgRegistryList(orgID int64, p *model.ListOptions) ([]*model.Registry, error) {
	return c.registryStore.OrgRegistryList(orgID, p)
}

func (c *CompositeEncryptedStore) GlobalRegistryFind(name string) (*model.Registry, error) {
	return c.registryStore.GlobalRegistryFind(name)
}

func (c *CompositeEncryptedStore) GlobalRegistryList(p *model.ListOptions) ([]*model.Registry, error) {
	return c.registryStore.GlobalRegistryList(p)
}

func (c *CompositeEncryptedStore) UserRegistryList(orgIDs []int64, includeGlobal bool, searchTerm string, p *model.ListOptions) ([]*model.Registry, error) {
	return c.registryStore.UserRegistryList(orgIDs, includeGlobal, searchTerm, p)
}

// ============================================================================
// Forge operations - delegated to forgeStore
// ============================================================================.

func (c *CompositeEncryptedStore) ForgeCreate(forge *model.Forge) error {
	return c.forgeStore.ForgeCreate(forge)
}

func (c *CompositeEncryptedStore) ForgeGet(id int64) (*model.Forge, error) {
	return c.forgeStore.ForgeGet(id)
}

func (c *CompositeEncryptedStore) ForgeList(p *model.ListOptions) ([]*model.Forge, error) {
	return c.forgeStore.ForgeList(p)
}

func (c *CompositeEncryptedStore) ForgeListAll() ([]*model.Forge, error) {
	return c.forgeStore.ForgeListAll()
}

func (c *CompositeEncryptedStore) ForgeUpdate(forge *model.Forge) error {
	return c.forgeStore.ForgeUpdate(forge)
}

func (c *CompositeEncryptedStore) ForgeDelete(forge *model.Forge) error {
	return c.forgeStore.ForgeDelete(forge)
}

// ============================================================================
// Pass-through operations - delegated directly to baseStore
// ============================================================================.

// Repos.
func (c *CompositeEncryptedStore) GetRepo(id int64) (*model.Repo, error) {
	return c.baseStore.GetRepo(id)
}

func (c *CompositeEncryptedStore) GetRepoForgeID(remoteID model.ForgeRemoteID) (*model.Repo, error) {
	return c.baseStore.GetRepoForgeID(remoteID)
}

func (c *CompositeEncryptedStore) GetRepoByForgeAndRemoteID(forgeID int64, remoteID model.ForgeRemoteID) (*model.Repo, error) {
	return c.baseStore.GetRepoByForgeAndRemoteID(forgeID, remoteID)
}

func (c *CompositeEncryptedStore) GetRepoByForgeOwnerName(forgeID int64, owner, name string) (*model.Repo, error) {
	return c.baseStore.GetRepoByForgeOwnerName(forgeID, owner, name)
}

func (c *CompositeEncryptedStore) GetRepoNameFallback(forgeID int64, remoteID model.ForgeRemoteID, fullName string) (*model.Repo, error) {
	return c.baseStore.GetRepoNameFallback(forgeID, remoteID, fullName)
}

func (c *CompositeEncryptedStore) GetRepoName(name string) (*model.Repo, error) {
	return c.baseStore.GetRepoName(name)
}

func (c *CompositeEncryptedStore) GetRepoCount() (int64, error) {
	return c.baseStore.GetRepoCount()
}

func (c *CompositeEncryptedStore) CreateRepo(repo *model.Repo) error {
	return c.baseStore.CreateRepo(repo)
}

func (c *CompositeEncryptedStore) UpdateRepo(repo *model.Repo) error {
	return c.baseStore.UpdateRepo(repo)
}

func (c *CompositeEncryptedStore) DeleteRepo(repo *model.Repo) error {
	return c.baseStore.DeleteRepo(repo)
}

func (c *CompositeEncryptedStore) RepoListOrphanedOwner() ([]*model.Repo, error) {
	return c.baseStore.RepoListOrphanedOwner()
}

// Redirections.
func (c *CompositeEncryptedStore) CreateRedirection(redirection *model.Redirection) error {
	return c.baseStore.CreateRedirection(redirection)
}

func (c *CompositeEncryptedStore) HasRedirectionForRepo(repoID int64, fullName string) (bool, error) {
	return c.baseStore.HasRedirectionForRepo(repoID, fullName)
}

// Pipelines.
func (c *CompositeEncryptedStore) GetPipeline(id int64) (*model.Pipeline, error) {
	return c.baseStore.GetPipeline(id)
}

func (c *CompositeEncryptedStore) GetPipelineNumber(repo *model.Repo, number int64) (*model.Pipeline, error) {
	return c.baseStore.GetPipelineNumber(repo, number)
}

func (c *CompositeEncryptedStore) GetPipelineBadge(repo *model.Repo, branch string) (*model.Pipeline, error) {
	return c.baseStore.GetPipelineBadge(repo, branch)
}

func (c *CompositeEncryptedStore) GetPipelineLast(repo *model.Repo, branch string) (*model.Pipeline, error) {
	return c.baseStore.GetPipelineLast(repo, branch)
}

func (c *CompositeEncryptedStore) GetPipelineLastBefore(repo *model.Repo, branch string, number int64) (*model.Pipeline, error) {
	return c.baseStore.GetPipelineLastBefore(repo, branch, number)
}

func (c *CompositeEncryptedStore) GetPipelineList(repo *model.Repo, p *model.ListOptions, filter *model.PipelineFilter) ([]*model.Pipeline, error) {
	return c.baseStore.GetPipelineList(repo, p, filter)
}

func (c *CompositeEncryptedStore) GetRepoLatestPipelines(repoIDs []int64) ([]*model.Pipeline, error) {
	return c.baseStore.GetRepoLatestPipelines(repoIDs)
}

func (c *CompositeEncryptedStore) GetActivePipelineList(repo *model.Repo) ([]*model.Pipeline, error) {
	return c.baseStore.GetActivePipelineList(repo)
}

func (c *CompositeEncryptedStore) GetPipelineListKeyset(repo *model.Repo, startID int64, beforeDate int64, limit int) ([]*model.Pipeline, error) {
	return c.baseStore.GetPipelineListKeyset(repo, startID, beforeDate, limit)
}

func (c *CompositeEncryptedStore) GetPipelineQueue() ([]*model.Feed, error) {
	return c.baseStore.GetPipelineQueue()
}

func (c *CompositeEncryptedStore) GetPipelineCount() (int64, error) {
	return c.baseStore.GetPipelineCount()
}

func (c *CompositeEncryptedStore) GetPipelineCountByRepo(repoID int64) (int64, error) {
	return c.baseStore.GetPipelineCountByRepo(repoID)
}

func (c *CompositeEncryptedStore) CreatePipeline(pipeline *model.Pipeline, steps ...*model.Step) error {
	return c.baseStore.CreatePipeline(pipeline, steps...)
}

func (c *CompositeEncryptedStore) UpdatePipeline(pipeline *model.Pipeline) error {
	return c.baseStore.UpdatePipeline(pipeline)
}

func (c *CompositeEncryptedStore) DeletePipeline(pipeline *model.Pipeline) error {
	return c.baseStore.DeletePipeline(pipeline)
}

func (c *CompositeEncryptedStore) SetPipelineLogPurged(pipeline *model.Pipeline) error {
	return c.baseStore.SetPipelineLogPurged(pipeline)
}

func (c *CompositeEncryptedStore) GetPipelineLogPurged(pipeline *model.Pipeline) (bool, error) {
	return c.baseStore.GetPipelineLogPurged(pipeline)
}

// Feeds.
func (c *CompositeEncryptedStore) UserFeed(user *model.User) ([]*model.Feed, error) {
	return c.baseStore.UserFeed(user)
}

// Repositories.
func (c *CompositeEncryptedStore) RepoList(user *model.User, owned, active bool) ([]*model.Repo, error) {
	return c.baseStore.RepoList(user, owned, active)
}

func (c *CompositeEncryptedStore) RepoListLatest(user *model.User) ([]*model.Feed, error) {
	return c.baseStore.RepoListLatest(user)
}

func (c *CompositeEncryptedStore) RepoListAll(active bool, p *model.ListOptions) ([]*model.Repo, error) {
	return c.baseStore.RepoListAll(active, p)
}

func (c *CompositeEncryptedStore) RepoListPublic(p *model.ListOptions) ([]*model.Repo, error) {
	return c.baseStore.RepoListPublic(p)
}

func (c *CompositeEncryptedStore) RepoListByUserAndForge(userID, forgeID int64) ([]*model.Repo, error) {
	return c.baseStore.RepoListByUserAndForge(userID, forgeID)
}

// Permissions.
func (c *CompositeEncryptedStore) PermFind(user *model.User, repo *model.Repo) (*model.Perm, error) {
	return c.baseStore.PermFind(user, repo)
}

func (c *CompositeEncryptedStore) PermUpsert(perm *model.Perm) error {
	return c.baseStore.PermUpsert(perm)
}

// Configs.
func (c *CompositeEncryptedStore) ConfigsForPipeline(pipelineID int64) ([]*model.Config, error) {
	return c.baseStore.ConfigsForPipeline(pipelineID)
}

func (c *CompositeEncryptedStore) ConfigPersist(config *model.Config) (*model.Config, error) {
	return c.baseStore.ConfigPersist(config)
}

func (c *CompositeEncryptedStore) PipelineConfigCreate(pc *model.PipelineConfig) error {
	return c.baseStore.PipelineConfigCreate(pc)
}

// Steps.
func (c *CompositeEncryptedStore) StepLoad(id int64) (*model.Step, error) {
	return c.baseStore.StepLoad(id)
}

func (c *CompositeEncryptedStore) StepFind(pipeline *model.Pipeline, pid int) (*model.Step, error) {
	return c.baseStore.StepFind(pipeline, pid)
}

func (c *CompositeEncryptedStore) StepByUUID(uuid string) (*model.Step, error) {
	return c.baseStore.StepByUUID(uuid)
}

func (c *CompositeEncryptedStore) StepChild(pipeline *model.Pipeline, ppid int, name string) (*model.Step, error) {
	return c.baseStore.StepChild(pipeline, ppid, name)
}

func (c *CompositeEncryptedStore) StepList(pipeline *model.Pipeline) ([]*model.Step, error) {
	return c.baseStore.StepList(pipeline)
}

func (c *CompositeEncryptedStore) StepUpdate(step *model.Step) error {
	return c.baseStore.StepUpdate(step)
}

func (c *CompositeEncryptedStore) StepListFromWorkflowFind(workflow *model.Workflow) ([]*model.Step, error) {
	return c.baseStore.StepListFromWorkflowFind(workflow)
}

// Logs.
func (c *CompositeEncryptedStore) LogFind(step *model.Step) ([]*model.LogEntry, error) {
	return c.baseStore.LogFind(step)
}

func (c *CompositeEncryptedStore) LogFindAfter(stepID int64, afterLogID int64) ([]*model.LogEntry, error) {
	return c.baseStore.LogFindAfter(stepID, afterLogID)
}

func (c *CompositeEncryptedStore) LogAppend(step *model.Step, entries []*model.LogEntry) error {
	return c.baseStore.LogAppend(step, entries)
}

func (c *CompositeEncryptedStore) LogDelete(step *model.Step) error {
	return c.baseStore.LogDelete(step)
}

// ProcLogs.
func (c *CompositeEncryptedStore) ProcLogInsert(ctx context.Context, entries []*model.ProcLogEntry) error {
	return c.baseStore.ProcLogInsert(ctx, entries)
}

func (c *CompositeEncryptedStore) ProcLogTail(ctx context.Context, sourceType string, sourceID int64, sinceID int64, limit int) ([]*model.ProcLogEntry, error) {
	return c.baseStore.ProcLogTail(ctx, sourceType, sourceID, sinceID, limit)
}

func (c *CompositeEncryptedStore) ProcLogTrim(ctx context.Context, sourceType string, sourceID int64, keepLastN int) error {
	return c.baseStore.ProcLogTrim(ctx, sourceType, sourceID, keepLastN)
}

func (c *CompositeEncryptedStore) ProcLogListSources(ctx context.Context) ([]model.ProcLogSourceKey, error) {
	return c.baseStore.ProcLogListSources(ctx)
}

// Tasks.
func (c *CompositeEncryptedStore) TaskList() ([]*model.Task, error) {
	return c.baseStore.TaskList()
}

func (c *CompositeEncryptedStore) TaskListPending() ([]*model.Task, error) {
	return c.baseStore.TaskListPending()
}

func (c *CompositeEncryptedStore) TaskInsert(task *model.Task) error {
	return c.baseStore.TaskInsert(task)
}

func (c *CompositeEncryptedStore) TaskUpdate(task *model.Task) error {
	return c.baseStore.TaskUpdate(task)
}

func (c *CompositeEncryptedStore) TaskDelete(taskID string) error {
	return c.baseStore.TaskDelete(taskID)
}

func (c *CompositeEncryptedStore) TaskAtomicAssign(taskID string, agentID int64) (bool, error) {
	return c.baseStore.TaskAtomicAssign(taskID, agentID)
}

func (c *CompositeEncryptedStore) TaskAtomicClaim(taskID string, agentID int64) (*model.Task, error) {
	return c.baseStore.TaskAtomicClaim(taskID, agentID)
}

func (c *CompositeEncryptedStore) TaskAtomicResetOrphan(taskID string, expectedAgentID int64) (bool, error) {
	return c.baseStore.TaskAtomicResetOrphan(taskID, expectedAgentID)
}

func (c *CompositeEncryptedStore) TaskGetCancelled(id string) (bool, bool, error) {
	return c.baseStore.TaskGetCancelled(id)
}

func (c *CompositeEncryptedStore) TaskSetCancelled(id string) error {
	return c.baseStore.TaskSetCancelled(id)
}

func (c *CompositeEncryptedStore) TaskGetAgentID(taskID string, agentID int64) (bool, error) {
	return c.baseStore.TaskGetAgentID(taskID, agentID)
}

func (c *CompositeEncryptedStore) TaskDeleteCancelledBefore(cancelledBefore int64) error {
	return c.baseStore.TaskDeleteCancelledBefore(cancelledBefore)
}

func (c *CompositeEncryptedStore) TaskDeleteStale() (int64, error) {
	return c.baseStore.TaskDeleteStale()
}

// ServerConfig.
func (c *CompositeEncryptedStore) ServerConfigGet(key string) (string, error) {
	return c.baseStore.ServerConfigGet(key)
}

func (c *CompositeEncryptedStore) ServerConfigSet(key, value string) error {
	return c.baseStore.ServerConfigSet(key, value)
}

func (c *CompositeEncryptedStore) ServerConfigDelete(key string) error {
	return c.baseStore.ServerConfigDelete(key)
}

// Cron.
func (c *CompositeEncryptedStore) CronCreate(cron *model.Cron) error {
	return c.baseStore.CronCreate(cron)
}

func (c *CompositeEncryptedStore) CronFind(repo *model.Repo, id int64) (*model.Cron, error) {
	return c.baseStore.CronFind(repo, id)
}

func (c *CompositeEncryptedStore) CronList(repo *model.Repo, p *model.ListOptions) ([]*model.Cron, error) {
	return c.baseStore.CronList(repo, p)
}

func (c *CompositeEncryptedStore) CronUpdate(repo *model.Repo, cron *model.Cron) error {
	return c.baseStore.CronUpdate(repo, cron)
}

func (c *CompositeEncryptedStore) CronDelete(repo *model.Repo, id int64) error {
	return c.baseStore.CronDelete(repo, id)
}

func (c *CompositeEncryptedStore) CronListNextExecute(now int64, limit int64) ([]*model.Cron, error) {
	return c.baseStore.CronListNextExecute(now, limit)
}

func (c *CompositeEncryptedStore) CronGetLock(cron *model.Cron, now int64) (bool, error) {
	return c.baseStore.CronGetLock(cron, now)
}

func (c *CompositeEncryptedStore) UserCronList(repoIDs []int64, searchTerm string, p *model.ListOptions) ([]*model.Cron, error) {
	return c.baseStore.UserCronList(repoIDs, searchTerm, p)
}

func (c *CompositeEncryptedStore) CronListOrphaned() ([]*model.Cron, error) {
	return c.baseStore.CronListOrphaned()
}

func (c *CompositeEncryptedStore) CronListOrphanedOwner() ([]*model.Cron, error) {
	return c.baseStore.CronListOrphanedOwner()
}

func (c *CompositeEncryptedStore) CronDeleteByID(id int64) error {
	return c.baseStore.CronDeleteByID(id)
}

func (c *CompositeEncryptedStore) CronDisableOrphaned(id int64, msg string) error {
	return c.baseStore.CronDisableOrphaned(id, msg)
}

// Agent.
func (c *CompositeEncryptedStore) AgentCreate(agent *model.Agent) error {
	return c.baseStore.AgentCreate(agent)
}

func (c *CompositeEncryptedStore) AgentFind(id int64) (*model.Agent, error) {
	return c.baseStore.AgentFind(id)
}

func (c *CompositeEncryptedStore) AgentFindByToken(token string) (*model.Agent, error) {
	return c.baseStore.AgentFindByToken(token)
}

func (c *CompositeEncryptedStore) AgentList(opts *model.AgentListOptions) ([]*model.Agent, error) {
	return c.baseStore.AgentList(opts)
}

func (c *CompositeEncryptedStore) AgentUpdate(agent *model.Agent) error {
	return c.baseStore.AgentUpdate(agent)
}

func (c *CompositeEncryptedStore) AgentDelete(agent *model.Agent) error {
	return c.baseStore.AgentDelete(agent)
}

func (c *CompositeEncryptedStore) AgentListForOrg(orgID int64, opt *model.ListOptions) ([]*model.Agent, error) {
	return c.baseStore.AgentListForOrg(orgID, opt)
}

// Access Tokens.
func (c *CompositeEncryptedStore) AccessTokenCreate(token *model.AccessToken) error {
	return c.baseStore.AccessTokenCreate(token)
}

func (c *CompositeEncryptedStore) AccessTokenFind(id int64) (*model.AccessToken, error) {
	return c.baseStore.AccessTokenFind(id)
}

func (c *CompositeEncryptedStore) AccessTokenFindByHash(hash string) (*model.AccessToken, error) {
	return c.baseStore.AccessTokenFindByHash(hash)
}

func (c *CompositeEncryptedStore) AccessTokenList(userID int64, p *model.ListOptions) ([]*model.AccessToken, error) {
	return c.baseStore.AccessTokenList(userID, p)
}

func (c *CompositeEncryptedStore) AccessTokenUpdate(token *model.AccessToken) error {
	return c.baseStore.AccessTokenUpdate(token)
}

func (c *CompositeEncryptedStore) AccessTokenDelete(token *model.AccessToken) error {
	return c.baseStore.AccessTokenDelete(token)
}

func (c *CompositeEncryptedStore) AccessTokenUpdateLastUsed(tokenID int64, now int64) error {
	return c.baseStore.AccessTokenUpdateLastUsed(tokenID, now)
}

// Autoscaler.
func (c *CompositeEncryptedStore) AutoscalerCreate(autoscaler *model.Autoscaler) error {
	return c.baseStore.AutoscalerCreate(autoscaler)
}

func (c *CompositeEncryptedStore) AutoscalerFind(id int64) (*model.Autoscaler, error) {
	return c.baseStore.AutoscalerFind(id)
}

func (c *CompositeEncryptedStore) AutoscalerFindByName(name string) (*model.Autoscaler, error) {
	return c.baseStore.AutoscalerFindByName(name)
}

func (c *CompositeEncryptedStore) AutoscalerFindByToken(token string) (*model.Autoscaler, error) {
	return c.baseStore.AutoscalerFindByToken(token)
}

func (c *CompositeEncryptedStore) AutoscalerList(opts *model.AutoscalerListOptions) ([]*model.Autoscaler, error) {
	return c.baseStore.AutoscalerList(opts)
}

func (c *CompositeEncryptedStore) AutoscalerUpdate(autoscaler *model.Autoscaler) error {
	return c.baseStore.AutoscalerUpdate(autoscaler)
}

func (c *CompositeEncryptedStore) AutoscalerDelete(autoscaler *model.Autoscaler) error {
	return c.baseStore.AutoscalerDelete(autoscaler)
}

func (c *CompositeEncryptedStore) AutoscalerUpdateHeartbeat(id int64, activeAgents, pendingAgents int32, pendingSince int64, version string) error {
	return c.baseStore.AutoscalerUpdateHeartbeat(id, activeAgents, pendingAgents, pendingSince, version)
}

// Maintenance.
func (c *CompositeEncryptedStore) MaintenanceConfigGet() (*model.MaintenanceConfig, error) {
	return c.baseStore.MaintenanceConfigGet()
}

func (c *CompositeEncryptedStore) MaintenanceConfigUpdate(config *model.MaintenanceConfig) error {
	return c.baseStore.MaintenanceConfigUpdate(config)
}

func (c *CompositeEncryptedStore) MaintenanceConfigGetByActionType(actionType string) (*model.MaintenanceConfig, error) {
	return c.baseStore.MaintenanceConfigGetByActionType(actionType)
}

func (c *CompositeEncryptedStore) MaintenanceConfigUpdateByActionType(actionType string, config *model.MaintenanceConfig) error {
	return c.baseStore.MaintenanceConfigUpdateByActionType(actionType, config)
}

func (c *CompositeEncryptedStore) MaintenanceStatsGet() (*model.MaintenanceStats, error) {
	return c.baseStore.MaintenanceStatsGet()
}

func (c *CompositeEncryptedStore) MaintenanceStatsUpdate(stats *model.MaintenanceStats) error {
	return c.baseStore.MaintenanceStatsUpdate(stats)
}

func (c *CompositeEncryptedStore) MaintenanceLogCreate(log *model.MaintenanceLog) error {
	return c.baseStore.MaintenanceLogCreate(log)
}

func (c *CompositeEncryptedStore) MaintenanceLogList(limit int) ([]*model.MaintenanceLog, error) {
	return c.baseStore.MaintenanceLogList(limit)
}

func (c *CompositeEncryptedStore) MaintenanceRun(ctx context.Context) error {
	return c.baseStore.MaintenanceRun(ctx)
}

// Workflow.
func (c *CompositeEncryptedStore) WorkflowGetTree(pipeline *model.Pipeline) ([]*model.Workflow, error) {
	return c.baseStore.WorkflowGetTree(pipeline)
}

func (c *CompositeEncryptedStore) WorkflowsCreate(workflows []*model.Workflow) error {
	return c.baseStore.WorkflowsCreate(workflows)
}

func (c *CompositeEncryptedStore) WorkflowsReplace(pipeline *model.Pipeline, workflows []*model.Workflow) error {
	return c.baseStore.WorkflowsReplace(pipeline, workflows)
}

func (c *CompositeEncryptedStore) WorkflowLoad(id int64) (*model.Workflow, error) {
	return c.baseStore.WorkflowLoad(id)
}

func (c *CompositeEncryptedStore) WorkflowUpdate(workflow *model.Workflow) error {
	return c.baseStore.WorkflowUpdate(workflow)
}

func (c *CompositeEncryptedStore) WorkflowNamesByRepo(repoID int64) ([]string, error) {
	return c.baseStore.WorkflowNamesByRepo(repoID)
}

func (c *CompositeEncryptedStore) WorkflowsAwaitingApprovalBefore(now int64) ([]*model.Workflow, error) {
	return c.baseStore.WorkflowsAwaitingApprovalBefore(now)
}

func (c *CompositeEncryptedStore) WorkflowApprovalEventCreate(ev *model.WorkflowApprovalEvent) error {
	return c.baseStore.WorkflowApprovalEventCreate(ev)
}

func (c *CompositeEncryptedStore) WorkflowApprovalEventsForWorkflow(workflowID int64) ([]*model.WorkflowApprovalEvent, error) {
	return c.baseStore.WorkflowApprovalEventsForWorkflow(workflowID)
}

func (c *CompositeEncryptedStore) WorkflowsPendingApprovalForUser(userID int64) ([]*model.Workflow, error) {
	return c.baseStore.WorkflowsPendingApprovalForUser(userID)
}

// Org.
func (c *CompositeEncryptedStore) OrgCreate(org *model.Org) error {
	return c.baseStore.OrgCreate(org)
}

func (c *CompositeEncryptedStore) OrgGet(id int64) (*model.Org, error) {
	return c.baseStore.OrgGet(id)
}

func (c *CompositeEncryptedStore) OrgFindByName(name string, forgeID int64) (*model.Org, error) {
	return c.baseStore.OrgFindByName(name, forgeID)
}

func (c *CompositeEncryptedStore) OrgFindByNameAllForges(name string) ([]*model.Org, error) {
	return c.baseStore.OrgFindByNameAllForges(name)
}

func (c *CompositeEncryptedStore) OrgUpdate(org *model.Org) error {
	return c.baseStore.OrgUpdate(org)
}

func (c *CompositeEncryptedStore) OrgDelete(orgID int64) error {
	return c.baseStore.OrgDelete(orgID)
}

func (c *CompositeEncryptedStore) OrgList(p *model.ListOptions) ([]*model.Org, error) {
	return c.baseStore.OrgList(p)
}

// Org repos.
func (c *CompositeEncryptedStore) OrgRepoList(org *model.Org, p *model.ListOptions) ([]*model.Repo, error) {
	return c.baseStore.OrgRepoList(org, p)
}

// Metrics.
func (c *CompositeEncryptedStore) GetOrgCount() (int64, error) {
	return c.baseStore.GetOrgCount()
}

func (c *CompositeEncryptedStore) GetWorkflowCountByAgent(agentID int64, after, before int64) (int64, error) {
	return c.baseStore.GetWorkflowCountByAgent(agentID, after, before)
}

func (c *CompositeEncryptedStore) GetPipelineCountByOrg(orgID int64, after, before int64) (int64, error) {
	return c.baseStore.GetPipelineCountByOrg(orgID, after, before)
}

func (c *CompositeEncryptedStore) GetAveragePipelineDuration(after, before int64, repoIDs []int64) (float64, error) {
	return c.baseStore.GetAveragePipelineDuration(after, before, repoIDs)
}

func (c *CompositeEncryptedStore) GetAverageStepsPerWorkflow(after, before int64, repoIDs []int64) (float64, error) {
	return c.baseStore.GetAverageStepsPerWorkflow(after, before, repoIDs)
}

func (c *CompositeEncryptedStore) GetWorkflowCountsByAgent(after, before int64, repoIDs []int64) (map[int64]int64, error) {
	return c.baseStore.GetWorkflowCountsByAgent(after, before, repoIDs)
}

func (c *CompositeEncryptedStore) GetWorkflowCountsByAgentPersistentID(after, before int64, repoIDs []int64) ([]model.AgentMetricData, error) {
	return c.baseStore.GetWorkflowCountsByAgentPersistentID(after, before, repoIDs)
}

func (c *CompositeEncryptedStore) GetPipelineCountsByRepo(after, before int64, repoIDs []int64) (map[int64]int64, error) {
	return c.baseStore.GetPipelineCountsByRepo(after, before, repoIDs)
}

func (c *CompositeEncryptedStore) GetPipelineCountsByOrg(after, before int64, repoIDs []int64) (map[int64]int64, error) {
	return c.baseStore.GetPipelineCountsByOrg(after, before, repoIDs)
}

func (c *CompositeEncryptedStore) GetBuildTimeStatsByRepo(after, before int64, repoIDs []int64) (map[int64]map[string]float64, error) {
	return c.baseStore.GetBuildTimeStatsByRepo(after, before, repoIDs)
}

func (c *CompositeEncryptedStore) GetBuildTimeStatsByOrg(after, before int64, repoIDs []int64) (map[int64]map[string]float64, error) {
	return c.baseStore.GetBuildTimeStatsByOrg(after, before, repoIDs)
}

// Store operations.
func (c *CompositeEncryptedStore) Ping() error {
	return c.baseStore.Ping()
}

func (c *CompositeEncryptedStore) Close() error {
	return c.baseStore.Close()
}

func (c *CompositeEncryptedStore) Migrate(ctx context.Context, allowLong bool) error {
	return c.baseStore.Migrate(ctx, allowLong)
}

func (c *CompositeEncryptedStore) CleanupOrphanedWorkflows(workflowIDs []string) error {
	return c.baseStore.CleanupOrphanedWorkflows(workflowIDs)
}

// Distributed Locking.
func (c *CompositeEncryptedStore) DistributedLockAcquire(lockName string, instanceID string, ttlSeconds int64) (bool, error) {
	return c.baseStore.DistributedLockAcquire(lockName, instanceID, ttlSeconds)
}

func (c *CompositeEncryptedStore) DistributedLockRelease(lockName string, instanceID string) error {
	return c.baseStore.DistributedLockRelease(lockName, instanceID)
}

func (c *CompositeEncryptedStore) DistributedLockExtend(lockName string, instanceID string, ttlSeconds int64) (bool, error) {
	return c.baseStore.DistributedLockExtend(lockName, instanceID, ttlSeconds)
}

func (c *CompositeEncryptedStore) DistributedLockCleanup() error {
	return c.baseStore.DistributedLockCleanup()
}

func (c *CompositeEncryptedStore) DistributedLockList() ([]*model.DistributedLock, error) {
	return c.baseStore.DistributedLockList()
}

func (c *CompositeEncryptedStore) DistributedLockInit() error {
	return c.baseStore.DistributedLockInit()
}

// Distributed Pub/Sub.
func (c *CompositeEncryptedStore) PubSubMessageInsert(msg *model.DistributedMessage) error {
	return c.baseStore.PubSubMessageInsert(msg)
}

func (c *CompositeEncryptedStore) PubSubMessageListAfter(timestamp int64) ([]*model.DistributedMessage, error) {
	return c.baseStore.PubSubMessageListAfter(timestamp)
}

func (c *CompositeEncryptedStore) PubSubMessageCleanupOlderThan(timestamp int64) error {
	return c.baseStore.PubSubMessageCleanupOlderThan(timestamp)
}

// Distributed Cache.
func (c *CompositeEncryptedStore) CacheGet(key string) (*model.CacheEntry, error) {
	return c.baseStore.CacheGet(key)
}

func (c *CompositeEncryptedStore) CacheSet(entry *model.CacheEntry) error {
	return c.baseStore.CacheSet(entry)
}

func (c *CompositeEncryptedStore) CacheDelete(key string) error {
	return c.baseStore.CacheDelete(key)
}

func (c *CompositeEncryptedStore) CacheCleanupExpired(cutoff int64) error {
	return c.baseStore.CacheCleanupExpired(cutoff)
}

// Secret targets - pass through to base store.
func (c *CompositeEncryptedStore) SecretTargetCreate(target *model.SecretTarget) error {
	return c.baseStore.SecretTargetCreate(target)
}

func (c *CompositeEncryptedStore) SecretTargetDelete(target *model.SecretTarget) error {
	return c.baseStore.SecretTargetDelete(target)
}

func (c *CompositeEncryptedStore) SecretTargetList(secretID int64) ([]*model.SecretTarget, error) {
	return c.baseStore.SecretTargetList(secretID)
}

func (c *CompositeEncryptedStore) SecretTargetFind(secretID int64, orgID int64, repoID int64) (*model.SecretTarget, error) {
	return c.baseStore.SecretTargetFind(secretID, orgID, repoID)
}

func (c *CompositeEncryptedStore) SecretTargetDeleteBySecretID(secretID int64) error {
	return c.baseStore.SecretTargetDeleteBySecretID(secretID)
}

func (c *CompositeEncryptedStore) SecretTargetCheckConflict(secretID int64, secretName string, orgID int64, repoID int64) error {
	return c.baseStore.SecretTargetCheckConflict(secretID, secretName, orgID, repoID)
}

// Integration operations - pass through to base store.
// Note: Integration configs should be encrypted at the application layer if needed.
func (c *CompositeEncryptedStore) IntegrationFind(id int64) (*model.Integration, error) {
	return c.baseStore.IntegrationFind(id)
}

func (c *CompositeEncryptedStore) IntegrationFindByName(userID int64, name string) (*model.Integration, error) {
	return c.baseStore.IntegrationFindByName(userID, name)
}

func (c *CompositeEncryptedStore) IntegrationList(userID int64, opts *model.ListOptions) ([]*model.Integration, error) {
	return c.baseStore.IntegrationList(userID, opts)
}

func (c *CompositeEncryptedStore) IntegrationListAll() ([]*model.Integration, error) {
	return c.baseStore.IntegrationListAll()
}

func (c *CompositeEncryptedStore) IntegrationListAccessible(userID int64, orgIDs []int64, repoIDs []int64, opts *model.ListOptions) ([]*model.Integration, error) {
	return c.baseStore.IntegrationListAccessible(userID, orgIDs, repoIDs, opts)
}

func (c *CompositeEncryptedStore) IntegrationCreate(integration *model.Integration) error {
	return c.baseStore.IntegrationCreate(integration)
}

func (c *CompositeEncryptedStore) IntegrationUpdate(integration *model.Integration) error {
	return c.baseStore.IntegrationUpdate(integration)
}

func (c *CompositeEncryptedStore) IntegrationDelete(integration *model.Integration) error {
	return c.baseStore.IntegrationDelete(integration)
}

// ============================================================================
// UserForge operations - delegated to userForgeStore
// ============================================================================.

func (c *CompositeEncryptedStore) UserForgeList(userID int64) ([]*model.UserForge, error) {
	return c.userForgeStore.UserForgeList(userID)
}

func (c *CompositeEncryptedStore) UserForgeGet(userID, forgeID int64) (*model.UserForge, error) {
	return c.userForgeStore.UserForgeGet(userID, forgeID)
}

func (c *CompositeEncryptedStore) UserForgeGetByRemoteID(forgeID int64, remoteID model.ForgeRemoteID) (*model.UserForge, error) {
	return c.userForgeStore.UserForgeGetByRemoteID(forgeID, remoteID)
}

func (c *CompositeEncryptedStore) UserForgeGetPrimary(userID int64) (*model.UserForge, error) {
	return c.userForgeStore.UserForgeGetPrimary(userID)
}

func (c *CompositeEncryptedStore) UserForgeCreate(userForge *model.UserForge) error {
	return c.userForgeStore.UserForgeCreate(userForge)
}

func (c *CompositeEncryptedStore) UserForgeUpdate(userForge *model.UserForge) error {
	return c.userForgeStore.UserForgeUpdate(userForge)
}

func (c *CompositeEncryptedStore) UserForgeDelete(userForge *model.UserForge) error {
	return c.userForgeStore.UserForgeDelete(userForge)
}

func (c *CompositeEncryptedStore) UserForgeListAll() ([]*model.UserForge, error) {
	return c.userForgeStore.UserForgeListAll()
}

// Notification Configs.
func (c *CompositeEncryptedStore) NotificationConfigFind(repoID int64) (*model.NotificationConfig, error) {
	return c.baseStore.NotificationConfigFind(repoID)
}

func (c *CompositeEncryptedStore) NotificationConfigUpsert(config *model.NotificationConfig) error {
	return c.baseStore.NotificationConfigUpsert(config)
}

func (c *CompositeEncryptedStore) NotificationConfigDelete(repoID int64) error {
	return c.baseStore.NotificationConfigDelete(repoID)
}

// Email Unsubscribes.
func (c *CompositeEncryptedStore) EmailUnsubscribeFind(userID, repoID int64) (*model.EmailUnsubscribe, error) {
	return c.baseStore.EmailUnsubscribeFind(userID, repoID)
}

func (c *CompositeEncryptedStore) EmailUnsubscribeCreate(unsub *model.EmailUnsubscribe) error {
	return c.baseStore.EmailUnsubscribeCreate(unsub)
}

func (c *CompositeEncryptedStore) EmailUnsubscribeDelete(userID, repoID int64) error {
	return c.baseStore.EmailUnsubscribeDelete(userID, repoID)
}

// Approved Pull Requests.
func (c *CompositeEncryptedStore) ApprovedPullRequestFind(repoID int64, ref string) (*model.ApprovedPullRequest, error) {
	return c.baseStore.ApprovedPullRequestFind(repoID, ref)
}

func (c *CompositeEncryptedStore) ApprovedPullRequestCreate(apr *model.ApprovedPullRequest) error {
	return c.baseStore.ApprovedPullRequestCreate(apr)
}

func (c *CompositeEncryptedStore) ApprovedPullRequestDelete(repoID int64, ref string) error {
	return c.baseStore.ApprovedPullRequestDelete(repoID, ref)
}
