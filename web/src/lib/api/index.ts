import ApiClient, { encodeQueryString } from './client';
import type { RequestOptions } from './client';
import type {
  AccessToken,
  AccessTokenWithSecret,
  AdminMetrics,
  Agent,
  AgentWithToken,
  AgentWorkflow,
  Autoscaler,
  AutoscalerListOptions,
  AutoscalerWithToken,
  AvailableForge,
  AvailableScopesResponse,
  CreateAccessTokenRequest,
  Cron,
  CronWithRepo,
  Entity,
  EntityListOptions,
  Forge,
  ImportPreviewResponse,
  ImportSettingsRequest,
  ImportSettingsResponse,
  Integration,
  IntegrationTypeInfo,
  MaintenanceConfig,
  MaintenanceLog,
  MaintenanceOperationConfig,
  MaintenanceStats,
  ManualWorkflowsProgress,
  ManualWorkflowsResponse,
  MetricQuery,
  Metrics,
  MetricSeries,
  MetricsFilter,
  MetricsTimeFilter,
  CreateWatch,
  PipelineWatch,
  WatchOverviewItem,
  WatchSettings,
  WatchSuggestion,
  Org,
  OrgPermissions,
  Pipeline,
  PipelineConfig,
  PipelineFeed,
  PipelineLog,
  PipelineMetadata,
  PipelineWorkflow,
  PullRequest,
  QueueInfo,
  Registry,
  RegistryWithOrg,
  Repo,
  RepoCaches,
  BuildCacheBucket,
  BuildCacheBucketInput,
  BuildCacheCleanupResult,
  BuildCacheOrgDetail,
  AdminCaches,
  OrgCaches,
  RepoPermissions,
  RepoResponse,
  RepoSettings,
  Secret,
  SecretTarget,
  SecretWithOrg,
  SMTPStatus,
  SMTPTestResult,
  Task,
  UnlinkForgeResponse,
  UpdateAccessTokenRequest,
  User,
  UserForge,
  UserIdentity,
  UserPendingApprovalsResponse,
  WorkflowApprovalState,
} from './types';

interface AvailableReposOptions {
  forge_id?: number;
  only_unenabled?: boolean;
}

// PipelineRef is the branch or tag a manual pipeline runs against. Exactly one
// of the two is sent, since a branch and a tag may share a name.
export type PipelineRef = { branch: string; tag?: never } | { tag: string; branch?: never };

// PipelineOptions is the data for creating a new pipeline
type PipelineOptions = PipelineRef & {
  variables: Record<string, string>;
  workflows?: string[]; // specific workflows to run (if empty, all matching workflows are run)
};

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
  // Returns the user's active repositories with last pipeline info populated.
  async getActiveRepos(opts?: RequestOptions): Promise<Repo[]> {
    return this._get(`/api/v1/user/repos/active`, opts) as Promise<Repo[]>;
  }

  // Returns repositories visible from linked forges. Already-enabled repos are
  // flagged with active=true; their last_pipeline field is not populated.
  async getAvailableRepos(opts?: AvailableReposOptions): Promise<Repo[]> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/user/repos/available?${query}`) as Promise<Repo[]>;
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

  async getRepoBranches(repoId: number, opts?: PaginationOptions, reqOpts?: RequestOptions): Promise<string[]> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/repos/${repoId}/branches?${query}`, reqOpts) as Promise<string[]>;
  }

  // Returns 501 on forges without tag support, which callers are expected to
  // treat as "this repo has no selectable tags" rather than as a failure.
  async getRepoTags(repoId: number, opts?: PaginationOptions, reqOpts?: RequestOptions): Promise<string[]> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/repos/${repoId}/tags?${query}`, reqOpts) as Promise<string[]>;
  }

  async getRepoPullRequests(
    repoId: number,
    opts?: PaginationOptions,
    reqOpts?: RequestOptions,
  ): Promise<PullRequest[]> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/repos/${repoId}/pull_requests?${query}`, reqOpts) as Promise<PullRequest[]>;
  }

  async getRepoManualWorkflows(repoId: number, ref: PipelineRef): Promise<ManualWorkflowsResponse> {
    const query = encodeQueryString({ ...ref, filter: 'manual' });
    return this._get(`/api/v1/repos/${repoId}/configs?${query}`) as Promise<ManualWorkflowsResponse>;
  }

  async getRepoManualWorkflowsWithProgress(
    repoId: number,
    ref: PipelineRef,
    onProgress: (progress: ManualWorkflowsProgress) => void,
  ): Promise<ManualWorkflowsResponse> {
    const query = encodeQueryString({ ...ref, filter: 'manual' });

    return new Promise((resolve, reject) => {
      let settled = false;
      const events = this._subscribe<ManualWorkflowsProgress>(
        `/api/v1/repos/${repoId}/configs/stream?${query}`,
        (progress) => {
          onProgress(progress);
          if (progress.stage === 'complete' && progress.result) {
            settled = true;
            events.close();
            resolve(progress.result);
          } else if (progress.stage === 'error') {
            settled = true;
            events.close();
            reject(new Error(progress.error ?? 'Failed to fetch workflows'));
          }
        },
        { reconnect: false },
      );

      events.addEventListener('error', () => {
        if (!settled) {
          settled = true;
          events.close();
          reject(new Error('Workflow progress stream disconnected'));
        }
      });
    });
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
      /** comma-separated webhook events; must match the server's `event` query param */
      event?: string;
      search?: string;
      workflow?: string;
    },
    reqOpts?: RequestOptions,
  ): Promise<Pipeline[]> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/repos/${repoId}/pipelines?${query}`, reqOpts) as Promise<Pipeline[]>;
  }

  async getWorkflowNames(repoId: number, opts?: RequestOptions): Promise<string[]> {
    return this._get(`/api/v1/repos/${repoId}/workflows/names`, opts) as Promise<string[]>;
  }

  async getPipeline(repoId: number, pipelineNumber: number | 'latest'): Promise<Pipeline> {
    return this._get(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}`) as Promise<Pipeline>;
  }

  async getPipelineConfig(repoId: number, pipelineNumber: number): Promise<PipelineConfig[]> {
    return this._get(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/config`) as Promise<PipelineConfig[]>;
  }

  async getPipelineDiff(repoId: number, pipelineNumber: number): Promise<{ diff: string }> {
    return this._get(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/diff`, { silent: true }) as Promise<{
      diff: string;
    }>;
  }

  async getPipelineMetadata(repoId: number, pipelineNumber: number, opts?: RequestOptions): Promise<PipelineMetadata> {
    return this._get(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/metadata`, opts) as Promise<PipelineMetadata>;
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

  async rerunStep(repoId: number, pipelineNumber: number, workflowId: number, stepId: number): Promise<Pipeline> {
    return this._post(
      `/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/workflows/${workflowId}/steps/${stepId}/rerun`,
    ) as Promise<Pipeline>;
  }

  async approvePipeline(repoId: number, pipelineNumber: string): Promise<Pipeline> {
    return this._post(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/approve`) as Promise<Pipeline>;
  }

  async approveAlwaysPipeline(repoId: number, pipelineNumber: string): Promise<Pipeline> {
    return this._post(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/approve-always`) as Promise<Pipeline>;
  }

  async revokeAlwaysApproval(repoId: number, pipelineNumber: string): Promise<unknown> {
    return this._delete(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/approve-always`);
  }

  async declinePipeline(repoId: number, pipelineNumber: string): Promise<unknown> {
    return this._post(`/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/decline`);
  }

  // Workflow-level approval gate API.
  async workflowApprove(
    repoId: number,
    pipelineNumber: number,
    workflowId: number,
    reason?: string,
  ): Promise<PipelineWorkflow> {
    const body = reason !== undefined && reason !== '' ? { reason } : undefined;
    return this._post(
      `/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/workflows/${workflowId}/approve`,
      body,
    ) as Promise<PipelineWorkflow>;
  }

  async workflowDecline(
    repoId: number,
    pipelineNumber: number,
    workflowId: number,
    reason?: string,
  ): Promise<PipelineWorkflow> {
    const body = reason !== undefined && reason !== '' ? { reason } : undefined;
    return this._post(
      `/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/workflows/${workflowId}/decline`,
      body,
    ) as Promise<PipelineWorkflow>;
  }

  async getWorkflowApproval(
    repoId: number,
    pipelineNumber: number,
    workflowId: number,
    opts?: RequestOptions,
  ): Promise<WorkflowApprovalState> {
    return this._get(
      `/api/v1/repos/${repoId}/pipelines/${pipelineNumber}/workflows/${workflowId}/approval`,
      opts,
    ) as Promise<WorkflowApprovalState>;
  }

  async getUserPendingApprovals(opts?: RequestOptions): Promise<UserPendingApprovalsResponse> {
    return this._get('/api/v1/user/approvals/pending', opts) as Promise<UserPendingApprovalsResponse>;
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
    return this._patch(`/api/v1/repos/${repoId}/registries/${encodeURIComponent(registry.address ?? '')}`, registry);
  }

  async deleteRegistry(repoId: number, registryAddress: string): Promise<unknown> {
    return this._delete(`/api/v1/repos/${repoId}/registries/${encodeURIComponent(registryAddress)}`);
  }

  async getRepoCaches(repoId: number): Promise<RepoCaches> {
    return this._get(`/api/v1/repos/${repoId}/caches`) as Promise<RepoCaches>;
  }

  async deleteRepoCache(repoId: number, cacheId: number): Promise<unknown> {
    return this._delete(`/api/v1/repos/${repoId}/caches/${cacheId}`);
  }

  async deleteRepoCaches(repoId: number): Promise<unknown> {
    return this._delete(`/api/v1/repos/${repoId}/caches`);
  }

  async getOrgCaches(orgId: number): Promise<OrgCaches> {
    return this._get(`/api/v1/orgs/${orgId}/caches`) as Promise<OrgCaches>;
  }

  async deleteOrgCaches(orgId: number): Promise<BuildCacheCleanupResult> {
    return this._delete(`/api/v1/orgs/${orgId}/caches`) as Promise<BuildCacheCleanupResult>;
  }

  async putOrgCacheBucket(orgId: number, bucket: BuildCacheBucketInput): Promise<BuildCacheBucket> {
    return this._put(`/api/v1/orgs/${orgId}/caches/bucket`, bucket) as Promise<BuildCacheBucket>;
  }

  async deleteOrgCacheBucket(orgId: number): Promise<unknown> {
    return this._delete(`/api/v1/orgs/${orgId}/caches/bucket`);
  }

  async getAdminCaches(): Promise<AdminCaches> {
    return this._get('/api/v1/admin/caches') as Promise<AdminCaches>;
  }

  async getAdminCacheOrg(orgId: number): Promise<BuildCacheOrgDetail> {
    return this._get(`/api/v1/admin/caches/orgs/${orgId}`) as Promise<BuildCacheOrgDetail>;
  }

  async putAdminCacheOrg(
    orgId: number,
    settings: { max_size: number; max_egress: number; ttl: number },
  ): Promise<BuildCacheOrgDetail> {
    return this._put(`/api/v1/admin/caches/orgs/${orgId}`, settings) as Promise<BuildCacheOrgDetail>;
  }

  async deleteAdminCacheOrg(orgId: number): Promise<BuildCacheCleanupResult> {
    return this._delete(`/api/v1/admin/caches/orgs/${orgId}`) as Promise<BuildCacheCleanupResult>;
  }

  async runAdminCacheCleanup(): Promise<BuildCacheCleanupResult> {
    return this._post('/api/v1/admin/caches/cleanup') as Promise<BuildCacheCleanupResult>;
  }

  async getOrgRegistryList(orgId: number, opts?: PaginationOptions): Promise<Registry[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/orgs/${orgId}/registries?${query}`) as Promise<Registry[] | null>;
  }

  async createOrgRegistry(orgId: number, registry: Partial<Registry>): Promise<unknown> {
    return this._post(`/api/v1/orgs/${orgId}/registries`, registry);
  }

  async updateOrgRegistry(orgId: number, registry: Partial<Registry>): Promise<unknown> {
    return this._patch(`/api/v1/orgs/${orgId}/registries/${encodeURIComponent(registry.address ?? '')}`, registry);
  }

  async deleteOrgRegistry(orgId: number, registryAddress: string): Promise<unknown> {
    return this._delete(`/api/v1/orgs/${orgId}/registries/${encodeURIComponent(registryAddress)}`);
  }

  async getGlobalRegistryList(opts?: PaginationOptions): Promise<Registry[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/registries?${query}`) as Promise<Registry[] | null>;
  }

  async createGlobalRegistry(registry: Partial<Registry>): Promise<unknown> {
    return this._post(`/api/v1/registries`, registry);
  }

  async updateGlobalRegistry(registry: Partial<Registry>): Promise<unknown> {
    return this._patch(`/api/v1/registries/${encodeURIComponent(registry.address ?? '')}`, registry);
  }

  async deleteGlobalRegistry(registryAddress: string): Promise<unknown> {
    return this._delete(`/api/v1/registries/${encodeURIComponent(registryAddress)}`);
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

  async getSecretTargets(secretId: number, opts?: RequestOptions): Promise<SecretTarget[]> {
    return this._get(`/api/v1/secret-targets/${secretId}/targets`, opts) as Promise<SecretTarget[]>;
  }

  async createSecretTarget(secretId: number, target: { org_id: number; repo_id: number }): Promise<SecretTarget> {
    return this._post(`/api/v1/secret-targets/${secretId}/targets`, target) as Promise<SecretTarget>;
  }

  async deleteSecretTarget(secretId: number, targetId: number): Promise<unknown> {
    return this._delete(`/api/v1/secret-targets/${secretId}/targets/${targetId}`);
  }

  async transferSecretOwner(
    secretId: number,
    data: { org_id: number; repo_id: number; keep_as_target: boolean },
  ): Promise<unknown> {
    return this._patch(`/api/v1/secret-targets/${secretId}/owner`, data);
  }

  async getSelf(): Promise<unknown> {
    return this._get('/api/v1/user');
  }

  async updateSelf(profile: Pick<User, 'description' | 'timezone'>): Promise<User> {
    return this._patch('/api/v1/user', profile) as Promise<User>;
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
  async getUserForges(opts?: RequestOptions): Promise<UserForge[]> {
    return this._get('/api/v1/user/forges', opts) as Promise<UserForge[]>;
  }

  async getUserForge(forgeId: number): Promise<UserForge> {
    return this._get(`/api/v1/user/forges/${forgeId}`) as Promise<UserForge>;
  }

  async getUserIdentities(): Promise<UserIdentity[] | null> {
    return this._get('/api/v1/user/identities') as Promise<UserIdentity[] | null>;
  }

  async linkLdapIdentity(username: string, password: string): Promise<UserIdentity> {
    return this._post(
      '/api/v1/user/identities/ldap',
      { username, password },
      { silent: true },
    ) as Promise<UserIdentity>;
  }

  async deleteUserIdentity(identityId: number): Promise<unknown> {
    return this._delete(`/api/v1/user/identities/${identityId}`);
  }

  async getAvailableForges(): Promise<AvailableForge[]> {
    return this._get('/api/v1/user/forges/available') as Promise<AvailableForge[]>;
  }

  async getForgeAffectedRepos(forgeId: number, opts?: RequestOptions): Promise<Repo[]> {
    return this._get(`/api/v1/user/forges/${forgeId}/affected-repos`, opts) as Promise<Repo[]>;
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

  async getAgent(agentId: Agent['id'], opts?: RequestOptions): Promise<Agent> {
    return this._get(`/api/v1/agents/${agentId}`, opts) as Promise<Agent>;
  }

  async getAgentTasks(agentId: Agent['id']): Promise<Task[] | null> {
    return this._get(`/api/v1/agents/${agentId}/tasks`) as Promise<Task[] | null>;
  }

  async getAgentWorkflows(agentId: Agent['id'], opts?: PaginationOptions): Promise<AgentWorkflow[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/agents/${agentId}/workflows?${query}`) as Promise<AgentWorkflow[] | null>;
  }

  async createAgent(agent: Partial<Agent>): Promise<AgentWithToken> {
    return this._post('/api/v1/agents', agent) as Promise<AgentWithToken>;
  }

  async updateAgent(agent: Partial<Agent>): Promise<Agent> {
    return this._patch(`/api/v1/agents/${agent.id}`, {
      name: agent.name,
      no_schedule: agent.no_schedule,
      priority: agent.priority,
      org_ids: agent.org_ids,
    }) as Promise<Agent>;
  }

  async deleteAgent(agent: Agent): Promise<unknown> {
    return this._delete(`/api/v1/agents/${agent.id}`);
  }

  async requestAgentRestart(agentId: Agent['id']): Promise<Agent> {
    return this._post(`/api/v1/agents/${agentId}/restart`) as Promise<Agent>;
  }

  async cancelAgentRestart(agentId: Agent['id']): Promise<Agent> {
    return this._delete(`/api/v1/agents/${agentId}/restart`) as Promise<Agent>;
  }

  async queueAgentRestarts(): Promise<{ queued: number }> {
    return this._post('/api/v1/agents/restarts') as Promise<{ queued: number }>;
  }

  async getOrgAgents(orgId: number, opts?: PaginationOptions): Promise<Agent[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/orgs/${orgId}/agents?${query}`) as Promise<Agent[] | null>;
  }

  async createOrgAgent(orgId: number, agent: Partial<Agent>): Promise<AgentWithToken> {
    return this._post(`/api/v1/orgs/${orgId}/agents`, agent) as Promise<AgentWithToken>;
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

  async getForges(opts?: PaginationOptions, reqOpts?: RequestOptions): Promise<Forge[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/forges?${query}`, reqOpts) as Promise<Forge[] | null>;
  }

  async getForge(forgeId: Forge['id'], opts?: RequestOptions): Promise<Forge> {
    return this._get(`/api/v1/forges/${forgeId}`, opts) as Promise<Forge>;
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

  async cancelQueueTask(taskId: Task['id']): Promise<unknown> {
    return this._post(`/api/v1/queue/tasks/${taskId}/cancel`);
  }

  async getUsers(opts?: PaginationOptions, reqOpts?: RequestOptions): Promise<User[] | null> {
    const query = encodeQueryString(opts);
    return this._get(`/api/v1/users?${query}`, reqOpts) as Promise<User[] | null>;
  }

  async getUser(username: string, opts?: RequestOptions): Promise<User> {
    return this._get(`/api/v1/users/${username}`, opts) as Promise<User>;
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

  async getUserOrgs(opts?: RequestOptions): Promise<Org[]> {
    return this._get(`/api/v1/user/orgs`, opts) as Promise<Org[]>;
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
    action: 'vacuum' | 'kubernetes_cleanup' | 'stale_agent_cleanup' | 'docker_cleanup' | 'orphaned_data_cleanup',
    opts?: RequestOptions,
  ): Promise<MaintenanceConfig> {
    return this._get(`/api/v1/admin/maintenance/config/${action}`, opts) as Promise<MaintenanceConfig>;
  }

  async updateMaintenanceConfigByAction(
    action: 'vacuum' | 'kubernetes_cleanup' | 'stale_agent_cleanup' | 'docker_cleanup' | 'orphaned_data_cleanup',
    config: Partial<MaintenanceConfig>,
  ): Promise<MaintenanceConfig> {
    return this._put(`/api/v1/admin/maintenance/config/${action}`, config) as Promise<MaintenanceConfig>;
  }

  async getMaintenanceStats(opts?: RequestOptions): Promise<MaintenanceStats> {
    return this._get('/api/v1/admin/maintenance/stats', opts) as Promise<MaintenanceStats>;
  }

  async getMaintenanceLogs(limit?: number, opts?: RequestOptions): Promise<MaintenanceLog[]> {
    const query = limit !== undefined && limit > 0 ? `?limit=${limit}` : '';
    return this._get(`/api/v1/admin/maintenance/logs${query}`, opts) as Promise<MaintenanceLog[]>;
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

  // Chart builder: run a single flexible metrics aggregation.
  async queryMetrics(query: MetricQuery): Promise<MetricSeries> {
    const qs = encodeQueryString(query);
    return this._get(`/api/v1/metrics/query?${qs}`) as Promise<MetricSeries>;
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

  // Watch API methods
  async getUserWatches(opts?: RequestOptions): Promise<WatchOverviewItem[]> {
    return ((await this._get('/api/v1/user/watches', opts)) as WatchOverviewItem[] | null) ?? [];
  }

  async markWatchRead(id: number): Promise<void> {
    return this._post(`/api/v1/user/watches/${id}/read`) as Promise<void>;
  }

  async markAllWatchesRead(): Promise<void> {
    return this._post('/api/v1/user/watches/read') as Promise<void>;
  }

  async updateWatch(id: number, settings: WatchSettings): Promise<PipelineWatch> {
    return this._patch(`/api/v1/user/watches/${id}`, settings) as Promise<PipelineWatch>;
  }

  async deleteWatch(id: number): Promise<void> {
    return this._delete(`/api/v1/user/watches/${id}`) as Promise<void>;
  }

  async getRepoWatches(repoId: number): Promise<WatchOverviewItem[]> {
    return ((await this._get(`/api/v1/repos/${repoId}/watches`)) as WatchOverviewItem[] | null) ?? [];
  }

  async createRepoWatch(repoId: number, body: CreateWatch): Promise<PipelineWatch> {
    return this._post(`/api/v1/repos/${repoId}/watches`, body) as Promise<PipelineWatch>;
  }

  async getRepoWatchSuggestions(repoId: number): Promise<WatchSuggestion[]> {
    return ((await this._get(`/api/v1/repos/${repoId}/watches/suggestions`)) as WatchSuggestion[] | null) ?? [];
  }

  async getAdminSMTPStatus(opts?: RequestOptions): Promise<SMTPStatus> {
    return this._get('/api/v1/admin/notifications/smtp', opts) as Promise<SMTPStatus>;
  }

  async testAdminSMTP(): Promise<SMTPTestResult> {
    return this._post('/api/v1/admin/notifications/test') as Promise<SMTPTestResult>;
  }
}
