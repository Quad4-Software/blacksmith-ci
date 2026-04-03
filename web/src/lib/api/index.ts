import ApiClient, { encodeQueryString } from './client';
import type {
  AccessToken,
  AccessTokenWithSecret,
  AvailableScopesResponse,
  CreateAccessTokenRequest,
  UpdateAccessTokenRequest,
  AdminMetrics,
  Agent,
  Autoscaler,
  AutoscalerListOptions,
  AutoscalerWithToken,
  AvailableForge,
  Cron,
  CronWithRepo,
  Entity,
  EntityListOptions,
  Forge,
  Integration,
  IntegrationTypeInfo,
  MaintenanceConfig,
  MaintenanceLog,
  MaintenanceOperationConfig,
  MaintenanceStats,
  Metrics,
  MetricsFilter,
  MetricsTimeFilter,
  Org,
  OrgPermissions,
  Pipeline,
  PipelineConfig,
  PipelineFeed,
  PipelineLog,
  PullRequest,
  QueueInfo,
  Registry,
  RegistryWithOrg,
  Repo,
  RepoPermissions,
  RepoResponse,
  RepoSettings,
  ImportSettingsRequest,
  ImportSettingsResponse,
  ImportPreviewResponse,
  Secret,
  SecretWithOrg,
  NotificationConfig,
  SMTPStatus,
  SMTPTestResult,
  UnlinkForgeResponse,
  User,
  UserForge,
  ManualWorkflowsResponse,
} from './types';

interface RepoListOptions {
  all?: boolean;
}

// PipelineOptions is the data for creating a new pipeline
interface PipelineOptions {
  branch: string;
  variables: Record<string, string>;
  workflows?: string[]; // specific workflows to run (if empty, all matching workflows are run)
}

interface DeploymentOptions {
  id: string;
  environment: string;
  task: string;
  variables: Record<string, string>;
}

interface PaginationOptions {
  page?: number;
  perPage?: number;
}

interface UserListOptions extends PaginationOptions {
  scope?: string;
  search?: string;
}

interface AgentListOptions extends PaginationOptions {
  search?: string;
  platform?: string;
  backend?: string;
  sort?: string;
  order?: string;
}

export default class CrowClient extends ApiClient {
  async getRepoList(opts?: RepoListOptions): Promise<Repo[]> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/user/repos?${query}`) as Promise<Repo[]>;
  }

  async getPublicRepos(opts?: PaginationOptions): Promise<Repo[]> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/repos/public?${query}`) as Promise<Repo[]>;
  }

  async lookupRepo(owner: string, name: string): Promise<Repo | undefined> {
    return this._get(`/api/v1/repos/lookup/${owner}/${name}`) as Promise<Repo | undefined>;
  }

  async getRepo(repoId: number): Promise<Repo> {
    return this._get(`/api/v1/repos/${repoId}`) as Promise<Repo>;
  }

  async getRepoPermissions(repoId: number): Promise<RepoPermissions> {
    return this._get(`/api/v1/repos/${repoId}/permissions`) as Promise<RepoPermissions>;
  }

  async getRepoBranches(repoId: number, opts?: PaginationOptions): Promise<string[]> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/repos/${repoId}/branches?${query}`) as Promise<string[]>;
  }

  async getRepoPullRequests(repoId: number, opts?: PaginationOptions): Promise<PullRequest[]> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/repos/${repoId}/pull_requests?${query}`) as Promise<PullRequest[]>;
  }

  async getRepoManualWorkflows(repoId: number, branch: string): Promise<ManualWorkflowsResponse> {
    const query = encodeQueryString({ branch, filter: 'manual' });
    return this._get(`/api/v1/repos/${repoId}/configs?${query}`) as Promise<ManualWorkflowsResponse>;
  }

  async activateRepo(forgeRemoteId: string, forgeId?: number): Promise<RepoResponse> {
    const params: Record<string, string> = { forge_remote_id: forgeRemoteId };
    if (forgeId !== undefined) {
      params.forge_id = String(forgeId);
    }
    const query = encodeQueryString(params);
    return this._post(`/api/v1/repos?${query}`) as Promise<RepoResponse>;
  }

  async updateRepo(repoId: number, repoSettings: RepoSettings): Promise<unknown> {
    return this._patch(`/api/v1/repos/${repoId}`, repoSettings);
  }

  async deleteRepo(repoId: number, remove = true): Promise<unknown> {
    const query = encodeQueryString({ remove });
    return this._delete(`/api/v1/repos/${repoId}?${query}`);
  }

  async repairRepo(repoId: number): Promise<unknown> {
    return this._post(`/api/v1/repos/${repoId}/repair`);
  }

  // Get repos that can be used as import source for a target repo
  async getImportableRepos(targetRepoId: number): Promise<Repo[]> {
    return this._get(`/api/v1/repos/${targetRepoId}/importable`) as Promise<Repo[]>;
  }

  // Get import preview for a source repo
  async getImportPreview(targetRepoId: number, sourceRepoId: number): Promise<ImportPreviewResponse> {
    return this._get(`/api/v1/repos/${targetRepoId}/import-preview/${sourceRepoId}`) as Promise<ImportPreviewResponse>;
  }

  // Import settings from another repo
  async importRepoSettings(targetRepoId: number, request: ImportSettingsRequest): Promise<ImportSettingsResponse> {
    return this._post(`/api/v1/repos/${targetRepoId}/import-settings`, request) as Promise<ImportSettingsResponse>;
  }

  async createPipeline(repoId: number, options: PipelineOptions): Promise<Pipeline> {
    return this._post(`/api/v1/repos/${repoId}/pipelines`, options) as Promise<Pipeline>;
  }

  // Deploy triggers a deployment for an existing pipeline using the
  // specified target environment and task.
  async deployPipeline(repoId: number, pipelineNumber: string, options: DeploymentOptions): Promise<Pipeline> {
    const vars = {
      ...options.variables,
      event: 'deployment',
      deploy_to: options.environment,
      deploy_task: options.task,
    };
    const query = encodeQueryString(vars);
    return this._post(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}?${query}`) as Promise<Pipeline>;
  }

  async getPipelineList(
    repoId: number,
    opts?: PaginationOptions & {
      before?: string;
      after?: string;
      ref?: string;
      branch?: string;
      events?: string;
      search?: string;
      workflow?: string;
    },
  ): Promise<Pipeline[]> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/repos/${repoId}/pipelines?${query}`) as Promise<Pipeline[]>;
  }

  async getWorkflowNames(repoId: number): Promise<string[]> {
    return this._get(`/api/v1/repos/${repoId}/workflows/names`) as Promise<string[]>;
  }

  async getPipeline(repoId: number, pipelineNumber: number | 'latest'): Promise<Pipeline> {
    return this._get(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}`) as Promise<Pipeline>;
  }

  async getPipelineConfig(repoId: number, pipelineNumber: number): Promise<PipelineConfig[]> {
    return this._get(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/config`) as Promise<PipelineConfig[]>;
  }

  async getPipelineMetadata(repoId: number, pipelineNumber: number): Promise<any> {
    return this._get(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/metadata`) as Promise<any>;
  }

  async getPipelineFeed(): Promise<PipelineFeed[]> {
    return this._get(`/api/v1/user/feed`) as Promise<PipelineFeed[]>;
  }

  async cancelPipeline(repoId: number, pipelineNumber: number): Promise<unknown> {
    return this._post(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/cancel`);
  }

  async cancelWorkflow(repoId: number, pipelineNumber: number, workflowId: number): Promise<unknown> {
    return this._post(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/workflows/${workflowId}/cancel`);
  }

  async rerunWorkflow(repoId: number, pipelineNumber: number, workflowId: number): Promise<Pipeline> {
    return this._post(
      `/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/workflows/${workflowId}/rerun`,
    ) as Promise<Pipeline>;
  }

  async approvePipeline(repoId: number, pipelineNumber: string): Promise<unknown> {
    return this._post(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/approve`);
  }

  async approveAlwaysPipeline(repoId: number, pipelineNumber: string): Promise<unknown> {
    return this._post(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/approve-always`);
  }

  async revokeAlwaysApproval(repoId: number, pipelineNumber: string): Promise<unknown> {
    return this._delete(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/approve-always`);
  }

  async declinePipeline(repoId: number, pipelineNumber: string): Promise<unknown> {
    return this._post(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/decline`);
  }

  async restartPipeline(
    repoId: number,
    pipeline: string,
    opts?: { event?: string; deploy_to?: string; fork?: boolean },
  ): Promise<Pipeline> {
    const query = encodeQueryString(opts);
    return this._post(`/api/v1/repos/${repoId}/pipelines/${pipeline}?${query}`) as Promise<Pipeline>;
  }

  async getLogs(repoId: number, pipeline: number, step: number): Promise<PipelineLog[]> {
    return this._get(`/api/v1/repos/${repoId}/logs/${pipeline}/${step}`) as Promise<PipelineLog[]>;
  }

  async deleteLogs(repoId: number, pipeline: number, step: number): Promise<unknown> {
    return this._delete(`/api/v1/repos/${repoId}/logs/${pipeline}/${step}`);
  }

  async getSecretList(repoId: number, opts?: PaginationOptions): Promise<Secret[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/repos/${repoId}/secrets?${query}`) as Promise<Secret[] | null>;
  }

  async createSecret(repoId: number, secret: Partial<Secret>): Promise<unknown> {
    return this._post(`/api/v1/repos/${repoId}/secrets`, secret);
  }

  async updateSecret(repoId: number, secret: Partial<Secret>): Promise<unknown> {
    const secretName = encodeURIComponent(secret.name ?? '');
    return this._patch(`/api/v1/repos/${repoId}/secrets/${secretName}`, secret);
  }

  async deleteSecret(repoId: number, secretName: string): Promise<unknown> {
    const name = encodeURIComponent(secretName);
    return this._delete(`/api/v1/repos/${repoId}/secrets/${name}`);
  }

  async getSecretAbbreviated(repoId: number, secretName: string): Promise<Secret> {
    const name = encodeURIComponent(secretName);
    return this._get(`/api/v1/repos/${repoId}/secrets/${name}/abbreviated`) as Promise<Secret>;
  }

  async getRegistryList(repoId: number, opts?: PaginationOptions): Promise<Registry[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/repos/${repoId}/registries?${query}`) as Promise<Registry[] | null>;
  }

  async createRegistry(repoId: number, registry: Partial<Registry>): Promise<unknown> {
    return this._post(`/api/v1/repos/${repoId}/registries`, registry);
  }

  async updateRegistry(repoId: number, registry: Partial<Registry>): Promise<unknown> {
    return this._patch(`/api/v1/repos/${repoId}/registries/${registry.address}`, registry);
  }

  async deleteRegistry(repoId: number, registryAddress: string): Promise<unknown> {
    return this._delete(`/api/v1/repos/${repoId}/registries/${registryAddress}`);
  }

  async getOrgRegistryList(orgId: number, opts?: PaginationOptions): Promise<Registry[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/orgs/${orgId}/registries?${query}`) as Promise<Registry[] | null>;
  }

  async createOrgRegistry(orgId: number, registry: Partial<Registry>): Promise<unknown> {
    return this._post(`/api/v1/orgs/${orgId}/registries`, registry);
  }

  async updateOrgRegistry(orgId: number, registry: Partial<Registry>): Promise<unknown> {
    return this._patch(`/api/v1/orgs/${orgId}/registries/${registry.address}`, registry);
  }

  async deleteOrgRegistry(orgId: number, registryAddress: string): Promise<unknown> {
    return this._delete(`/api/v1/orgs/${orgId}/registries/${registryAddress}`);
  }

  async getGlobalRegistryList(opts?: PaginationOptions): Promise<Registry[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/registries?${query}`) as Promise<Registry[] | null>;
  }

  async createGlobalRegistry(registry: Partial<Registry>): Promise<unknown> {
    return this._post(`/api/v1/registries`, registry);
  }

  async updateGlobalRegistry(registry: Partial<Registry>): Promise<unknown> {
    return this._patch(`/api/v1/registries/${registry.address}`, registry);
  }

  async deleteGlobalRegistry(registryAddress: string): Promise<unknown> {
    return this._delete(`/api/v1/registries/${registryAddress}`);
  }

  async getCronList(repoId: number, opts?: PaginationOptions): Promise<Cron[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/repos/${repoId}/cron?${query}`) as Promise<Cron[] | null>;
  }

  async getUserCronList(opts?: UserListOptions): Promise<CronWithRepo[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/user/crons?${query}`) as Promise<CronWithRepo[] | null>;
  }

  async createCron(repoId: number, cron: Partial<Cron>): Promise<Cron> {
    return this._post(`/api/v1/repos/${repoId}/cron`, cron) as Promise<Cron>;
  }

  async updateCron(repoId: number, cron: Partial<Cron>): Promise<Cron> {
    return this._patch(`/api/v1/repos/${repoId}/cron/${cron.id}`, cron) as Promise<Cron>;
  }

  async deleteCron(repoId: number, cronId: number): Promise<unknown> {
    return this._delete(`/api/v1/repos/${repoId}/cron/${cronId}`);
  }

  async runCron(repoId: number, cronId: number): Promise<Pipeline> {
    return this._post(`/api/v1/repos/${repoId}/cron/${cronId}`) as Promise<Pipeline>;
  }

  async getOrg(orgId: number): Promise<Org> {
    return this._get(`/api/v1/orgs/${orgId}`) as Promise<Org>;
  }

  async lookupOrg(name: string): Promise<Org> {
    return this._get(`/api/v1/orgs/lookup/${name}`) as Promise<Org>;
  }

  async getOrgPermissions(orgId: number): Promise<OrgPermissions> {
    return this._get(`/api/v1/orgs/${orgId}/permissions`) as Promise<OrgPermissions>;
  }

  async getOrgSecretList(orgId: number, opts?: PaginationOptions): Promise<Secret[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/orgs/${orgId}/secrets?${query}`) as Promise<Secret[] | null>;
  }

  async createOrgSecret(orgId: number, secret: Partial<Secret>): Promise<unknown> {
    return this._post(`/api/v1/orgs/${orgId}/secrets`, secret);
  }

  async updateOrgSecret(orgId: number, secret: Partial<Secret>): Promise<unknown> {
    const secretName = encodeURIComponent(secret.name ?? '');
    return this._patch(`/api/v1/orgs/${orgId}/secrets/${secretName}`, secret);
  }

  async deleteOrgSecret(orgId: number, secretName: string): Promise<unknown> {
    const name = encodeURIComponent(secretName);
    return this._delete(`/api/v1/orgs/${orgId}/secrets/${name}`);
  }

  async getOrgSecretAbbreviated(orgId: number, secretName: string): Promise<Secret> {
    const name = encodeURIComponent(secretName);
    return this._get(`/api/v1/orgs/${orgId}/secrets/${name}/abbreviated`) as Promise<Secret>;
  }

  async getGlobalSecretList(opts?: PaginationOptions): Promise<Secret[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/secrets?${query}`) as Promise<Secret[] | null>;
  }

  async createGlobalSecret(secret: Partial<Secret>): Promise<unknown> {
    return this._post(`/api/v1/secrets`, secret);
  }

  async updateGlobalSecret(secret: Partial<Secret>): Promise<unknown> {
    const secretName = encodeURIComponent(secret.name ?? '');
    return this._patch(`/api/v1/secrets/${secretName}`, secret);
  }

  async deleteGlobalSecret(secretName: string): Promise<unknown> {
    const name = encodeURIComponent(secretName);
    return this._delete(`/api/v1/secrets/${name}`);
  }

  async getGlobalSecretAbbreviated(secretName: string): Promise<Secret> {
    const name = encodeURIComponent(secretName);
    return this._get(`/api/v1/secrets/${name}/abbreviated`) as Promise<Secret>;
  }

  async getSelf(): Promise<unknown> {
    return this._get('/api/v1/user');
  }

  async getToken(): Promise<string> {
    return this._post('/api/v1/user/token') as Promise<string>;
  }

  // Access Tokens
  async getAccessTokenList(opts?: PaginationOptions): Promise<AccessToken[]> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/user/access-tokens?${query}`) as Promise<AccessToken[]>;
  }

  async getAccessToken(tokenId: number): Promise<AccessToken> {
    return this._get(`/api/v1/user/access-tokens/${tokenId}`) as Promise<AccessToken>;
  }

  async createAccessToken(data: CreateAccessTokenRequest): Promise<AccessTokenWithSecret> {
    return this._post('/api/v1/user/access-tokens', data) as Promise<AccessTokenWithSecret>;
  }

  async updateAccessToken(tokenId: number, data: UpdateAccessTokenRequest): Promise<AccessToken> {
    return this._patch(`/api/v1/user/access-tokens/${tokenId}`, data) as Promise<AccessToken>;
  }

  async deleteAccessToken(tokenId: number): Promise<void> {
    await this._delete(`/api/v1/user/access-tokens/${tokenId}`);
  }

  async deleteAccessTokenBatch(ids: number[]): Promise<void> {
    await this._delete('/api/v1/user/access-tokens/batch', { ids });
  }

  async getAvailableScopes(): Promise<AvailableScopesResponse> {
    return this._get('/api/v1/user/access-tokens/scopes') as Promise<AvailableScopesResponse>;
  }

  // User Forges (multi-forge support)
  async getUserForges(): Promise<UserForge[]> {
    return this._get('/api/v1/user/forges') as Promise<UserForge[]>;
  }

  async getUserForge(forgeId: number): Promise<UserForge> {
    return this._get(`/api/v1/user/forges/${forgeId}`) as Promise<UserForge>;
  }

  async getAvailableForges(): Promise<AvailableForge[]> {
    return this._get('/api/v1/user/forges/available') as Promise<AvailableForge[]>;
  }

  async getForgeAffectedRepos(forgeId: number): Promise<Repo[]> {
    return this._get(`/api/v1/user/forges/${forgeId}/affected-repos`) as Promise<Repo[]>;
  }

  async unlinkForge(forgeId: number): Promise<UnlinkForgeResponse> {
    return this._delete(`/api/v1/user/forges/${forgeId}`) as Promise<UnlinkForgeResponse>;
  }

  async getUserSecretList(opts?: UserListOptions): Promise<SecretWithOrg[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/user/secrets?${query}`) as Promise<SecretWithOrg[] | null>;
  }

  async getUserRegistryList(opts?: UserListOptions): Promise<RegistryWithOrg[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/user/registries?${query}`) as Promise<RegistryWithOrg[] | null>;
  }

  async getAgents(opts?: AgentListOptions): Promise<Agent[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/agents?${query}`) as Promise<Agent[] | null>;
  }

  async getAgent(agentId: Agent['id']): Promise<Agent> {
    return this._get(`/api/v1/agents/${agentId}`) as Promise<Agent>;
  }

  async createAgent(agent: Partial<Agent>): Promise<Agent> {
    return this._post('/api/v1/agents', agent) as Promise<Agent>;
  }

  async updateAgent(agent: Partial<Agent>): Promise<Agent> {
    return this._patch(`/api/v1/agents/${agent.id}`, {
      name: agent.name,
      no_schedule: agent.no_schedule,
      priority: agent.priority,
    }) as Promise<Agent>;
  }

  async deleteAgent(agent: Agent): Promise<unknown> {
    return this._delete(`/api/v1/agents/${agent.id}`);
  }

  async getOrgAgents(orgId: number, opts?: PaginationOptions): Promise<Agent[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/orgs/${orgId}/agents?${query}`) as Promise<Agent[] | null>;
  }

  async createOrgAgent(orgId: number, agent: Partial<Agent>): Promise<Agent> {
    return this._post(`/api/v1/orgs/${orgId}/agents`, agent) as Promise<Agent>;
  }

  async updateOrgAgent(orgId: number, agentId: number, agent: Partial<Agent>): Promise<Agent> {
    return this._patch(`/api/v1/orgs/${orgId}/agents/${agentId}`, agent) as Promise<Agent>;
  }

  async deleteOrgAgent(orgId: number, agentId: number): Promise<unknown> {
    return this._delete(`/api/v1/orgs/${orgId}/agents/${agentId}`);
  }

  async getAutoscalers(opts?: AutoscalerListOptions): Promise<Autoscaler[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/autoscalers?${query}`) as Promise<Autoscaler[] | null>;
  }

  async getAutoscaler(autoscalerId: number): Promise<Autoscaler> {
    return this._get(`/api/v1/autoscalers/${autoscalerId}`) as Promise<Autoscaler>;
  }

  async createAutoscaler(autoscaler: Partial<Autoscaler>): Promise<AutoscalerWithToken> {
    return this._post('/api/v1/autoscalers', autoscaler) as Promise<AutoscalerWithToken>;
  }

  async updateAutoscaler(autoscaler: Partial<Autoscaler>): Promise<Autoscaler> {
    return this._patch(`/api/v1/autoscalers/${autoscaler.id}`, { enabled: autoscaler.enabled }) as Promise<Autoscaler>;
  }

  async deleteAutoscaler(autoscaler: Autoscaler): Promise<unknown> {
    return this._delete(`/api/v1/autoscalers/${autoscaler.id}`);
  }

  async regenerateAutoscalerToken(autoscalerId: number): Promise<{ token: string }> {
    return this._post(`/api/v1/autoscalers/${autoscalerId}/token`) as Promise<{ token: string }>;
  }

  async getForges(opts?: PaginationOptions): Promise<Forge[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/forges?${query}`) as Promise<Forge[] | null>;
  }

  async getForge(forgeId: Forge['id']): Promise<Forge> {
    return this._get(`/api/v1/forges/${forgeId}`) as Promise<Forge>;
  }

  async createForge(forge: Partial<Forge>): Promise<Forge> {
    return this._post('/api/v1/forges', forge) as Promise<Forge>;
  }

  async updateForge(forge: Partial<Forge>): Promise<unknown> {
    return this._patch(`/api/v1/forges/${forge.id}`, forge);
  }

  async deleteForge(forge: Forge): Promise<unknown> {
    return this._delete(`/api/v1/forges/${forge.id}`);
  }

  async getQueueInfo(): Promise<QueueInfo> {
    return this._get('/api/v1/queue/info') as Promise<QueueInfo>;
  }

  async pauseQueue(): Promise<unknown> {
    return this._post('/api/v1/queue/pause');
  }

  async resumeQueue(): Promise<unknown> {
    return this._post('/api/v1/queue/resume');
  }

  async getUsers(opts?: PaginationOptions): Promise<User[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/users?${query}`) as Promise<User[] | null>;
  }

  async getUser(username: string): Promise<User> {
    return this._get(`/api/v1/users/${username}`) as Promise<User>;
  }

  async createUser(user: Partial<User>): Promise<User> {
    return this._post('/api/v1/users', user) as Promise<User>;
  }

  async updateUser(user: Partial<User>): Promise<unknown> {
    return this._patch(`/api/v1/users/${user.login}`, user);
  }

  async deleteUser(user: User): Promise<unknown> {
    return this._delete(`/api/v1/users/${user.login}`);
  }

  async getTimezones(): Promise<string[]> {
    return this._get('/api/v1/timezones') as Promise<string[]>;
  }

  async resetToken(): Promise<string> {
    return this._delete('/api/v1/user/token') as Promise<string>;
  }

  async getOrgs(opts?: PaginationOptions): Promise<Org[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/orgs?${query}`) as Promise<Org[] | null>;
  }

  async getUserOrgs(): Promise<Org[]> {
    return this._get(`/api/v1/user/orgs`) as Promise<Org[]>;
  }

  async deleteOrg(org: Org): Promise<unknown> {
    return this._delete(`/api/v1/orgs/${org.id}`);
  }

  async getEntities(opts?: EntityListOptions): Promise<Entity[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/entities?${query}`) as Promise<Entity[] | null>;
  }

  async getAllRepos(opts?: PaginationOptions): Promise<Repo[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/repos?${query}`) as Promise<Repo[] | null>;
  }

  async repairAllRepos(): Promise<unknown> {
    return this._post(`/api/v1/repos/repair`);
  }

  // eslint-disable-next-line promise/prefer-await-to-callbacks
  on(callback: (data: { pipeline?: Pipeline; repo?: Repo }) => void): EventSource {
    return this._subscribe('/api/v1/stream/events', callback, {
      reconnect: true,
    });
  }

  streamLogs(
    repoId: number,
    pipeline: number,
    step: number,
    // eslint-disable-next-line promise/prefer-await-to-callbacks
    callback: (data: PipelineLog) => void,
  ): EventSource {
    return this._subscribe(`/api/v1/stream/logs/${repoId}/${pipeline}/${step}`, callback, {
      reconnect: true,
    });
  }

  // Maintenance API methods
  async getMaintenanceConfig(): Promise<MaintenanceConfig> {
    return this._get('/api/v1/admin/maintenance/config') as Promise<MaintenanceConfig>;
  }

  async updateMaintenanceConfig(config: Partial<MaintenanceConfig>): Promise<MaintenanceConfig> {
    return this._put('/api/v1/admin/maintenance/config', config) as Promise<MaintenanceConfig>;
  }

  // Action-specific maintenance config methods
  async getMaintenanceConfigByAction(
    action: 'vacuum' | 'kubernetes_cleanup' | 'stale_agent_cleanup' | 'docker_cleanup',
  ): Promise<MaintenanceConfig> {
    return this._get(`/api/v1/admin/maintenance/config/${action}`) as Promise<MaintenanceConfig>;
  }

  async updateMaintenanceConfigByAction(
    action: 'vacuum' | 'kubernetes_cleanup' | 'stale_agent_cleanup' | 'docker_cleanup',
    config: Partial<MaintenanceConfig>,
  ): Promise<MaintenanceConfig> {
    return this._put(`/api/v1/admin/maintenance/config/${action}`, config) as Promise<MaintenanceConfig>;
  }

  async getMaintenanceStats(): Promise<MaintenanceStats> {
    return this._get('/api/v1/admin/maintenance/stats') as Promise<MaintenanceStats>;
  }

  async getMaintenanceLogs(limit?: number): Promise<MaintenanceLog[]> {
    const query = limit !== undefined && limit > 0 ? `?limit=${limit}` : '';
    return this._get(`/api/v1/admin/maintenance/logs${query}`) as Promise<MaintenanceLog[]>;
  }

  async runMaintenanceNow(job: string, dryRun?: boolean): Promise<{ message: string; job: string }> {
    const payload: { job: string; dry_run?: boolean } = { job };
    if (dryRun !== undefined) {
      payload.dry_run = dryRun;
    }
    return this._post('/api/v1/admin/maintenance/run', payload) as Promise<{ message: string; job: string }>;
  }

  async getMaintenanceOperationConfigs(): Promise<MaintenanceOperationConfig[]> {
    return this._get('/api/v1/admin/maintenance/operations') as Promise<MaintenanceOperationConfig[]>;
  }

  async updateMaintenanceOperationConfig(
    config: Partial<MaintenanceOperationConfig>,
  ): Promise<MaintenanceOperationConfig> {
    return this._put(
      `/api/v1/admin/maintenance/operations/${config.operation}`,
      config,
    ) as Promise<MaintenanceOperationConfig>;
  }

  // Metrics API methods
  async getMetrics(filter?: MetricsFilter): Promise<Metrics> {
    const query = encodeQueryString(filter);
    return this._get(`/api/v1/metrics?${query}`) as Promise<Metrics>;
  }

  // Deprecated: use getMetrics instead
  async getAdminMetrics(filter?: MetricsTimeFilter): Promise<AdminMetrics> {
    return this.getMetrics(filter);
  }

  // Integration API methods
  async getIntegrationList(opts?: PaginationOptions): Promise<Integration[]> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/user/integrations?${query}`) as Promise<Integration[]>;
  }

  async getAccessibleIntegrations(opts?: PaginationOptions): Promise<Integration[]> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/user/integrations/accessible?${query}`) as Promise<Integration[]>;
  }

  async getIntegration(integrationId: number): Promise<Integration> {
    return this._get(`/api/v1/user/integrations/${integrationId}`) as Promise<Integration>;
  }

  async createIntegration(integration: Partial<Integration>): Promise<Integration> {
    return this._post('/api/v1/user/integrations', integration) as Promise<Integration>;
  }

  async updateIntegration(integrationId: number, integration: Partial<Integration>): Promise<Integration> {
    return this._patch(`/api/v1/user/integrations/${integrationId}`, integration) as Promise<Integration>;
  }

  async deleteIntegration(integrationId: number): Promise<void> {
    return this._delete(`/api/v1/user/integrations/${integrationId}`) as Promise<void>;
  }

  async testIntegration(integrationId: number): Promise<{ status: string }> {
    return this._post(`/api/v1/user/integrations/${integrationId}/test`) as Promise<{ status: string }>;
  }

  async getIntegrationTypes(): Promise<IntegrationTypeInfo[]> {
    return this._get('/api/v1/integrations/types') as Promise<IntegrationTypeInfo[]>;
  }

  // Notification API methods
  async getRepoNotificationConfig(repoId: number): Promise<NotificationConfig> {
    return this._get(`/api/v1/repos/${repoId}/notification`) as Promise<NotificationConfig>;
  }

  async saveRepoNotificationConfig(
    repoId: number,
    config: Pick<NotificationConfig, 'is_enabled' | 'events'>,
  ): Promise<NotificationConfig> {
    return this._put(`/api/v1/repos/${repoId}/notification`, config) as Promise<NotificationConfig>;
  }

  async deleteRepoNotificationConfig(repoId: number): Promise<void> {
    return this._delete(`/api/v1/repos/${repoId}/notification`) as Promise<void>;
  }

  async getAdminSMTPStatus(): Promise<SMTPStatus> {
    return this._get('/api/v1/admin/notifications/smtp') as Promise<SMTPStatus>;
  }

  async testAdminSMTP(): Promise<SMTPTestResult> {
    return this._post('/api/v1/admin/notifications/test') as Promise<SMTPTestResult>;
  }
}
