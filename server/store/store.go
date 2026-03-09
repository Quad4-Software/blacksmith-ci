// Copyright 2018 Drone.IO Inc.
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

	"codefloe.com/crowci/crow/v5/server/model"
)

// TODO: CreateX func should return new object to not indirect let storage change an existing object (alter ID etc...)

type Store interface {
	// Users
	// GetUser gets a user by unique ID.
	GetUser(int64) (*model.User, error)
	// GetUserRemoteID gets a user by remote ID with fallback to login name.
	GetUserRemoteID(model.ForgeRemoteID, string) (*model.User, error)
	// GetUserLogin gets a user by unique Login name.
	GetUserLogin(string) (*model.User, error)
	// GetUserList gets a list of all users in the system.
	GetUserList(p *model.ListOptions) ([]*model.User, error)
	// GetUserCount gets a count of all users in the system.
	GetUserCount() (int64, error)
	// CreateUser creates a new user account.
	CreateUser(*model.User) error
	// UpdateUser updates a user account.
	UpdateUser(*model.User) error
	// DeleteUser deletes a user account.
	DeleteUser(*model.User) error
	// UserListAll gets all users in the system (for encryption migration).
	UserListAll() ([]*model.User, error)

	// User Forges (multi-forge support)
	// UserForgeList returns all forge connections for a user.
	UserForgeList(userID int64) ([]*model.UserForge, error)
	// UserForgeGet returns a user's forge connection for a specific forge.
	UserForgeGet(userID, forgeID int64) (*model.UserForge, error)
	// UserForgeGetByRemoteID finds a user forge connection by forge ID and forge remote ID.
	UserForgeGetByRemoteID(forgeID int64, remoteID model.ForgeRemoteID) (*model.UserForge, error)
	// UserForgeGetPrimary returns the user's primary forge connection.
	UserForgeGetPrimary(userID int64) (*model.UserForge, error)
	// UserForgeCreate creates a new user forge connection.
	UserForgeCreate(*model.UserForge) error
	// UserForgeUpdate updates a user forge connection.
	UserForgeUpdate(*model.UserForge) error
	// UserForgeDelete deletes a user forge connection.
	UserForgeDelete(*model.UserForge) error
	// UserForgeListAll returns all user forge connections (for encryption migration).
	UserForgeListAll() ([]*model.UserForge, error)

	// Repos
	// GetRepo gets a repo by unique ID.
	GetRepo(int64) (*model.Repo, error)
	// GetRepoForgeID gets a repo by its forge remote ID.
	GetRepoForgeID(model.ForgeRemoteID) (*model.Repo, error)
	// GetRepoByForgeAndRemoteID gets a repo by both forge ID and forge remote ID.
	// This is the preferred method for multi-forge setups as forge_remote_id alone is not unique.
	GetRepoByForgeAndRemoteID(forgeID int64, remoteID model.ForgeRemoteID) (*model.Repo, error)
	// GetRepoNameFallback gets the repo by its forge ID and if this doesn't exist by its full name.
	GetRepoNameFallback(remoteID model.ForgeRemoteID, fullName string) (*model.Repo, error)
	// GetRepoName gets a repo by its full name.
	GetRepoName(string) (*model.Repo, error)
	// GetRepoCount gets a count of all repositories in the system.
	GetRepoCount() (int64, error)
	// CreateRepo creates a new repository.
	CreateRepo(*model.Repo) error
	// UpdateRepo updates a user repository.
	UpdateRepo(*model.Repo) error
	// DeleteRepo deletes a user repository.
	DeleteRepo(*model.Repo) error

	// Redirections
	// CreateRedirection creates a redirection
	CreateRedirection(redirection *model.Redirection) error
	// HasRedirectionForRepo checks if there's a redirection for the given repo and full name
	HasRedirectionForRepo(int64, string) (bool, error)

	// Pipelines
	// GetPipeline gets a pipeline by unique ID.
	GetPipeline(int64) (*model.Pipeline, error)
	// GetPipelineNumber gets a pipeline by number.
	GetPipelineNumber(*model.Repo, int64) (*model.Pipeline, error)
	// GetPipelineBadge gets the last relevant pipeline for the badge.
	GetPipelineBadge(*model.Repo, string) (*model.Pipeline, error)
	// GetPipelineLast gets the last pipeline for the branch.
	GetPipelineLast(*model.Repo, string) (*model.Pipeline, error)
	// GetPipelineLastBefore gets the last pipeline before pipeline number N.
	GetPipelineLastBefore(*model.Repo, string, int64) (*model.Pipeline, error)
	// GetPipelineList gets a list of pipelines for the repository
	GetPipelineList(*model.Repo, *model.ListOptions, *model.PipelineFilter) ([]*model.Pipeline, error)
	// GetRepoLatestPipelines gets the latest pipelines for the given repo IDs.
	GetRepoLatestPipelines([]int64) ([]*model.Pipeline, error)
	// GetActivePipelineList gets a list of the active pipelines for the repository
	GetActivePipelineList(repo *model.Repo) ([]*model.Pipeline, error)
	// GetPipelineListKeyset gets a list of pipelines for the repository using keyset pagination.
	GetPipelineListKeyset(repo *model.Repo, startID int64, beforeDate int64, limit int) ([]*model.Pipeline, error)
	// GetPipelineQueue gets a list of pipelines in queue.
	GetPipelineQueue() ([]*model.Feed, error)
	// GetPipelineCount gets a count of all pipelines in the system.
	GetPipelineCount() (int64, error)
	// GetPipelineCountByRepo gets a count of all pipelines for a specific repo.
	GetPipelineCountByRepo(int64) (int64, error)
	// CreatePipeline creates a new pipeline and steps.
	CreatePipeline(*model.Pipeline, ...*model.Step) error
	// UpdatePipeline updates a pipeline.
	UpdatePipeline(*model.Pipeline) error
	// DeletePipeline deletes a pipeline.
	DeletePipeline(*model.Pipeline) error
	// Set indicator that pipelineLogs were purged
	SetPipelineLogPurged(*model.Pipeline) error
	// Get indicator if pipelineLogs were purged
	GetPipelineLogPurged(*model.Pipeline) (bool, error)

	// Feeds
	UserFeed(*model.User) ([]*model.Feed, error)

	// Repositories
	RepoList(user *model.User, owned, active bool) ([]*model.Repo, error)
	RepoListLatest(*model.User) ([]*model.Feed, error)
	RepoListAll(active bool, p *model.ListOptions) ([]*model.Repo, error)
	// RepoListPublic returns all active public repos (for anonymous browsing)
	RepoListPublic(p *model.ListOptions) ([]*model.Repo, error)
	// RepoListByUserAndForge returns all active repos owned by a user from a specific forge
	RepoListByUserAndForge(userID, forgeID int64) ([]*model.Repo, error)

	// Permissions
	PermFind(user *model.User, repo *model.Repo) (*model.Perm, error)
	PermUpsert(perm *model.Perm) error

	// Configs
	ConfigsForPipeline(pipelineID int64) ([]*model.Config, error)
	ConfigPersist(*model.Config) (*model.Config, error)
	PipelineConfigCreate(*model.PipelineConfig) error

	// Secrets
	SecretFind(*model.Repo, string) (*model.Secret, error)
	SecretList(*model.Repo, bool, *model.ListOptions) ([]*model.Secret, error)
	SecretListAll() ([]*model.Secret, error)
	SecretCreate(*model.Secret) error
	SecretUpdate(*model.Secret) error
	SecretDelete(*model.Secret) error
	OrgSecretFind(int64, string) (*model.Secret, error)
	OrgSecretList(int64, *model.ListOptions) ([]*model.Secret, error)
	GlobalSecretFind(string) (*model.Secret, error)
	GlobalSecretList(*model.ListOptions) ([]*model.Secret, error)
	UserSecretList(orgIDs []int64, repoIDs []int64, includeGlobal bool, searchTerm string, p *model.ListOptions) ([]*model.Secret, error)

	// Registries
	RegistryFind(*model.Repo, string) (*model.Registry, error)
	RegistryList(*model.Repo, bool, *model.ListOptions) ([]*model.Registry, error)
	RegistryListAll() ([]*model.Registry, error)
	RegistryCreate(*model.Registry) error
	RegistryUpdate(*model.Registry) error
	RegistryDelete(*model.Registry) error
	OrgRegistryFind(int64, string) (*model.Registry, error)
	OrgRegistryList(int64, *model.ListOptions) ([]*model.Registry, error)
	GlobalRegistryFind(string) (*model.Registry, error)
	GlobalRegistryList(*model.ListOptions) ([]*model.Registry, error)
	UserRegistryList(orgIDs []int64, includeGlobal bool, searchTerm string, p *model.ListOptions) ([]*model.Registry, error)

	// Steps
	StepLoad(int64) (*model.Step, error)
	StepFind(*model.Pipeline, int) (*model.Step, error)
	StepByUUID(string) (*model.Step, error)
	StepChild(*model.Pipeline, int, string) (*model.Step, error)
	StepList(*model.Pipeline) ([]*model.Step, error)
	StepUpdate(*model.Step) error
	StepListFromWorkflowFind(*model.Workflow) ([]*model.Step, error)

	// Logs
	LogFind(*model.Step) ([]*model.LogEntry, error)
	LogFindAfter(stepID int64, afterLogID int64) ([]*model.LogEntry, error)
	LogAppend(*model.Step, []*model.LogEntry) error
	LogDelete(*model.Step) error

	// Tasks
	// TaskList TODO: paginate & opt filter
	TaskList() ([]*model.Task, error)
	// TaskListPending returns only tasks whose corresponding workflows are still pending or running
	TaskListPending() ([]*model.Task, error)
	TaskInsert(*model.Task) error
	TaskUpdate(*model.Task) error
	TaskDelete(string) error
	// TaskAtomicAssign atomically assigns a task to an agent if it's currently unassigned
	TaskAtomicAssign(taskID string, agentID int64) (bool, error)
	// TaskAtomicClaim atomically claims a task for adoption if it was previously assigned to this agent.
	// Used by restarted agents to reclaim orphaned workflows. Returns the task if claimed, nil if not.
	TaskAtomicClaim(taskID string, agentID int64) (*model.Task, error)

	// ServerConfig
	ServerConfigGet(string) (string, error)
	ServerConfigSet(string, string) error
	ServerConfigDelete(string) error

	// Cron
	CronCreate(*model.Cron) error
	CronFind(*model.Repo, int64) (*model.Cron, error)
	CronList(*model.Repo, *model.ListOptions) ([]*model.Cron, error)
	CronUpdate(*model.Repo, *model.Cron) error
	CronDelete(*model.Repo, int64) error
	CronListNextExecute(int64, int64) ([]*model.Cron, error)
	CronGetLock(*model.Cron, int64) (bool, error)
	UserCronList(repoIDs []int64, searchTerm string, p *model.ListOptions) ([]*model.Cron, error)

	// Forge
	ForgeCreate(*model.Forge) error
	ForgeGet(int64) (*model.Forge, error)
	ForgeList(p *model.ListOptions) ([]*model.Forge, error)
	// ForgeListAll gets all forges in the system (for encryption migration).
	ForgeListAll() ([]*model.Forge, error)
	ForgeUpdate(*model.Forge) error
	ForgeDelete(*model.Forge) error

	// Agent
	AgentCreate(*model.Agent) error
	AgentFind(int64) (*model.Agent, error)
	AgentFindByToken(string) (*model.Agent, error)
	AgentList(opts *model.AgentListOptions) ([]*model.Agent, error)
	AgentUpdate(*model.Agent) error
	AgentDelete(*model.Agent) error
	AgentListForOrg(orgID int64, opt *model.ListOptions) ([]*model.Agent, error)

	// Access Tokens
	AccessTokenCreate(*model.AccessToken) error
	AccessTokenFind(int64) (*model.AccessToken, error)
	AccessTokenFindByHash(string) (*model.AccessToken, error)
	AccessTokenList(userID int64, p *model.ListOptions) ([]*model.AccessToken, error)
	AccessTokenUpdate(*model.AccessToken) error
	AccessTokenDelete(*model.AccessToken) error
	AccessTokenUpdateLastUsed(int64, int64) error

	// Autoscaler
	AutoscalerCreate(*model.Autoscaler) error
	AutoscalerFind(int64) (*model.Autoscaler, error)
	AutoscalerFindByName(string) (*model.Autoscaler, error)
	AutoscalerFindByToken(string) (*model.Autoscaler, error)
	AutoscalerList(opts *model.AutoscalerListOptions) ([]*model.Autoscaler, error)
	AutoscalerUpdate(*model.Autoscaler) error
	AutoscalerDelete(*model.Autoscaler) error
	AutoscalerUpdateHeartbeat(id int64, activeAgents, pendingAgents int32, version string) error

	// Integrations
	IntegrationFind(int64) (*model.Integration, error)
	IntegrationFindByName(userID int64, name string) (*model.Integration, error)
	IntegrationList(userID int64, opts *model.ListOptions) ([]*model.Integration, error)
	IntegrationListAll() ([]*model.Integration, error)
	IntegrationListAccessible(userID int64, orgIDs []int64, repoIDs []int64, opts *model.ListOptions) ([]*model.Integration, error)
	IntegrationCreate(*model.Integration) error
	IntegrationUpdate(*model.Integration) error
	IntegrationDelete(*model.Integration) error

	// Notification Configs
	NotificationConfigFind(repoID int64) (*model.NotificationConfig, error)
	NotificationConfigUpsert(*model.NotificationConfig) error
	NotificationConfigDelete(repoID int64) error

	// Maintenance
	MaintenanceConfigGet() (*model.MaintenanceConfig, error)
	MaintenanceConfigUpdate(*model.MaintenanceConfig) error
	MaintenanceConfigGetByActionType(actionType string) (*model.MaintenanceConfig, error)
	MaintenanceConfigUpdateByActionType(actionType string, config *model.MaintenanceConfig) error
	MaintenanceStatsGet() (*model.MaintenanceStats, error)
	MaintenanceStatsUpdate(*model.MaintenanceStats) error
	MaintenanceLogCreate(*model.MaintenanceLog) error
	MaintenanceLogList(limit int) ([]*model.MaintenanceLog, error)
	MaintenanceRun(context.Context) error

	// Approved Pull Requests
	// ApprovedPullRequestFind finds an always-approved PR by repo ID and ref.
	ApprovedPullRequestFind(repoID int64, ref string) (*model.ApprovedPullRequest, error)
	// ApprovedPullRequestCreate creates a new always-approved PR record.
	ApprovedPullRequestCreate(*model.ApprovedPullRequest) error
	// ApprovedPullRequestDelete deletes an always-approved PR record by repo ID and ref.
	ApprovedPullRequestDelete(repoID int64, ref string) error

	// Workflow
	WorkflowGetTree(*model.Pipeline) ([]*model.Workflow, error)
	WorkflowsCreate([]*model.Workflow) error
	WorkflowsReplace(*model.Pipeline, []*model.Workflow) error
	WorkflowLoad(int64) (*model.Workflow, error)
	WorkflowUpdate(*model.Workflow) error

	// Org
	OrgCreate(*model.Org) error
	OrgGet(int64) (*model.Org, error)
	OrgFindByName(string, int64) (*model.Org, error)
	// OrgFindByNameAllForges returns all orgs with the given name across all forges.
	OrgFindByNameAllForges(string) ([]*model.Org, error)
	OrgUpdate(*model.Org) error
	OrgDelete(int64) error
	OrgList(*model.ListOptions) ([]*model.Org, error)

	// Org repos
	OrgRepoList(*model.Org, *model.ListOptions) ([]*model.Repo, error)

	// Metrics
	// GetOrgCount gets a count of all organizations in the system.
	GetOrgCount() (int64, error)
	// GetWorkflowCountByAgent gets a count of workflows executed by a specific agent with optional time filter.
	GetWorkflowCountByAgent(agentID int64, after, before int64) (int64, error)
	// GetPipelineCountByOrg gets a count of pipelines for a specific org with optional time filter.
	GetPipelineCountByOrg(orgID int64, after, before int64) (int64, error)
	// GetAveragePipelineDuration gets the average duration of pipelines with optional time and repo filter.
	// If repoIDs is nil or empty, returns metrics for all repos.
	GetAveragePipelineDuration(after, before int64, repoIDs []int64) (float64, error)
	// GetAverageStepsPerWorkflow gets the average number of steps per workflow with optional time and repo filter.
	// If repoIDs is nil or empty, returns metrics for all repos.
	GetAverageStepsPerWorkflow(after, before int64, repoIDs []int64) (float64, error)
	// GetWorkflowCountsByAgent returns workflow counts grouped by agent ID with optional time and repo filter.
	// If repoIDs is nil or empty, returns metrics for all repos.
	// Note: For better tracking across agent ID changes, prefer GetWorkflowCountsByAgentPersistentID.
	// This method is still useful for legacy data without persistent IDs.
	GetWorkflowCountsByAgent(after, before int64, repoIDs []int64) (map[int64]int64, error)
	// GetWorkflowCountsByAgentPersistentID returns workflow counts grouped by agent persistent ID.
	// This method uses the persistent ID stored on workflows, allowing metrics aggregation even
	// after agent cleanup/ID changes. For workflows without a persistent ID (legacy data),
	// use GetWorkflowCountsByAgent as fallback.
	GetWorkflowCountsByAgentPersistentID(after, before int64, repoIDs []int64) ([]model.AgentMetricData, error)
	// GetPipelineCountsByRepo returns pipeline counts grouped by repo ID with optional time and repo filter.
	// If repoIDs is nil or empty, returns metrics for all repos.
	GetPipelineCountsByRepo(after, before int64, repoIDs []int64) (map[int64]int64, error)
	// GetPipelineCountsByOrg returns pipeline counts grouped by org ID with optional time and repo filter.
	// If repoIDs is nil or empty, returns metrics for all repos.
	GetPipelineCountsByOrg(after, before int64, repoIDs []int64) (map[int64]int64, error)
	// GetBuildTimeStatsByRepo returns average and total build times grouped by repo ID with optional time and repo filter.
	// If repoIDs is nil or empty, returns metrics for all repos.
	GetBuildTimeStatsByRepo(after, before int64, repoIDs []int64) (map[int64]map[string]float64, error)
	// GetBuildTimeStatsByOrg returns average and total build times grouped by org ID with optional time and repo filter.
	// If repoIDs is nil or empty, returns metrics for all repos.
	GetBuildTimeStatsByOrg(after, before int64, repoIDs []int64) (map[int64]map[string]float64, error)

	// Store operations
	Ping() error
	Close() error
	Migrate(context.Context, bool) error

	// CleanupOrphanedWorkflows checks for workflows that exist in the queue but not in the database
	// This helps prevent "sql: no rows in result set" errors
	CleanupOrphanedWorkflows(workflowIDs []string) error

	// Distributed Locking (for HA coordination)
	// DistributedLockAcquire attempts to acquire a distributed lock
	DistributedLockAcquire(lockName string, instanceID string, ttlSeconds int64) (bool, error)
	// DistributedLockRelease releases a distributed lock held by this instance
	DistributedLockRelease(lockName string, instanceID string) error
	// DistributedLockExtend extends the TTL of a lock held by this instance
	DistributedLockExtend(lockName string, instanceID string, ttlSeconds int64) (bool, error)
	// DistributedLockCleanup removes expired locks
	DistributedLockCleanup() error
	// DistributedLockList lists all currently held locks
	DistributedLockList() ([]*model.DistributedLock, error)
	// DistributedLockInit initializes known locks to prevent race conditions
	DistributedLockInit() error

	// Distributed Pub/Sub (for HA log streaming and events)
	// PubSubMessageInsert stores a message for distributed pub/sub
	PubSubMessageInsert(msg *model.DistributedMessage) error
	// PubSubMessageListAfter retrieves messages created after the given timestamp
	PubSubMessageListAfter(timestamp int64) ([]*model.DistributedMessage, error)
	// PubSubMessageCleanupOlderThan removes messages older than the given timestamp
	PubSubMessageCleanupOlderThan(timestamp int64) error

	// Distributed Cache (for HA membership and permissions caching)
	// CacheGet retrieves a cache entry by key
	CacheGet(key string) (*model.CacheEntry, error)
	// CacheSet stores a cache entry
	CacheSet(entry *model.CacheEntry) error
	// CacheDelete removes a cache entry
	CacheDelete(key string) error
	// CacheCleanupExpired removes expired cache entries
	CacheCleanupExpired(cutoff int64) error
}
