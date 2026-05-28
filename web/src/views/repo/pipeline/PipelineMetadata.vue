<!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
<template>
  <template v-if="repoPermissions && repoPermissions.push">
    <!-- Metadata Information Sections -->
    <div class="space-y-6">
      <!-- Pipeline Information -->
      <Card>
        <CardHeader>
          <CardTitle>{{ $t('repo.pipeline.metadata.pipeline_info') }}</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <MetadataField
              :label="$t('repo.pipeline.metadata.fields.number')"
              :value="metadata?.curr?.number || pipeline?.number"
            />
            <MetadataField
              :label="$t('repo.pipeline.metadata.fields.status')"
              :value="metadata?.curr?.status || pipeline?.status"
            />
            <MetadataField
              :label="$t('repo.pipeline.metadata.fields.event')"
              :value="metadata?.curr?.event || pipeline?.event"
            />
            <MetadataField
              :label="$t('repo.pipeline.metadata.fields.branch')"
              :value="metadata?.curr?.commit?.branch || pipeline?.branch"
            />
            <MetadataField :label="$t('repo.pipeline.metadata.fields.commit')">
              <a
                v-if="commitUrl"
                :href="commitUrl"
                target="_blank"
                class="text-primary hover:underline font-mono text-sm"
              >
                {{ commitHash }}
              </a>
              <span v-else class="font-mono text-sm">{{ commitHash }}</span>
            </MetadataField>
            <MetadataField
              :label="$t('repo.pipeline.metadata.fields.created')"
              :value="formatDate(metadata?.curr?.created || pipeline?.created)"
            />
            <MetadataField
              :label="$t('repo.pipeline.metadata.fields.started')"
              :value="formatDate(metadata?.curr?.started || pipeline?.started)"
            />
            <MetadataField
              :label="$t('repo.pipeline.metadata.fields.finished')"
              :value="formatDate(metadata?.curr?.finished || pipeline?.finished)"
            />
            <MetadataField
              :label="$t('repo.pipeline.metadata.fields.deployment')"
              :value="hasDeployment ? 'true' : 'false'"
            />
            <MetadataField
              v-if="hasDeployment"
              :label="$t('repo.pipeline.metadata.fields.environment')"
              :value="deploymentEnvironment"
            />
          </div>
        </CardContent>
      </Card>

      <!-- Repository Information -->
      <Card>
        <CardHeader>
          <CardTitle>{{ $t('repo.pipeline.metadata.repository_info') }}</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <MetadataField :label="$t('repo.pipeline.metadata.fields.id')" :value="metadata?.repo?.id || repo?.id" />
            <MetadataField
              :label="$t('repo.pipeline.metadata.fields.name')"
              :value="metadata?.repo?.name || repo?.name"
            />
            <MetadataField :label="$t('repo.pipeline.metadata.fields.url')">
              <a
                v-if="repositoryUrl"
                :href="repositoryUrl"
                target="_blank"
                class="text-primary hover:underline text-sm break-all"
              >
                {{ repositoryUrl }}
              </a>
              <span v-else class="text-sm">{{ $t('repo.pipeline.metadata.fields.na') }}</span>
            </MetadataField>
            <MetadataField :label="$t('repo.pipeline.metadata.fields.default_branch')" :value="defaultBranch" />
            <MetadataField
              :label="$t('repo.pipeline.metadata.fields.private')"
              :value="String(metadata?.repo?.private || repo?.private)"
            />
            <MetadataField :label="$t('repo.pipeline.metadata.fields.trusted')" :value="trustedDisplay" />
          </div>
        </CardContent>
      </Card>

      <!-- Pipeline Statistics -->
      <Card>
        <CardHeader>
          <CardTitle>{{ $t('repo.pipeline.metadata.statistics') }}</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
            <MetadataField :label="$t('repo.pipeline.metadata.fields.workflow_count')" :value="workflowCount" />
            <MetadataField :label="$t('repo.pipeline.metadata.fields.step_count')" :value="stepCount" />
            <MetadataField :label="$t('repo.pipeline.metadata.fields.unique_agents')" :value="uniqueAgentCount" />
          </div>
        </CardContent>
      </Card>

      <!-- Agent Information -->
      <Card v-if="agentInfo.length > 0">
        <CardHeader>
          <CardTitle>{{ $t('repo.pipeline.metadata.agent_info') }}</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="space-y-4">
            <Card v-for="agent in agentInfo" :key="agent.id" class="border-muted">
              <CardContent class="pt-6">
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <MetadataField :label="$t('repo.pipeline.metadata.fields.agent_id')">
                    <router-link
                      :to="{ name: 'admin-agent-detail', params: { id: agent.id } }"
                      class="text-primary hover:underline font-mono text-sm"
                    >
                      {{ agent.id }}
                    </router-link>
                  </MetadataField>
                  <MetadataField :label="$t('repo.pipeline.metadata.fields.agent_name')" :value="agent.name" />
                  <MetadataField :label="$t('repo.pipeline.metadata.fields.agent_platform')" :value="agent.platform" />
                  <MetadataField :label="$t('repo.pipeline.metadata.fields.agent_backend')" :value="agent.backend" />
                  <MetadataField
                    v-if="agent.priority && agent.priority > 0"
                    :label="$t('repo.pipeline.metadata.fields.agent_priority')"
                    :value="agent.priority"
                  />
                  <MetadataField
                    :label="$t('repo.pipeline.metadata.fields.workflow_count')"
                    :value="agent.workflows.length"
                  />
                  <div v-if="agent.custom_labels && Object.keys(agent.custom_labels).length > 0" class="col-span-full">
                    <div class="text-sm font-medium text-muted-foreground mb-2">
                      {{ $t('repo.pipeline.metadata.fields.agent_labels') }}
                    </div>
                    <div class="flex flex-wrap gap-2">
                      <Badge v-for="(value, key) in agent.custom_labels" :key="key" variant="secondary">
                        {{ key }}={{ value }}
                      </Badge>
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        </CardContent>
      </Card>

      <!-- Download Section -->
      <Card>
        <CardHeader>
          <CardTitle>{{ $t('repo.pipeline.metadata.download_metadata') }}</CardTitle>
        </CardHeader>
        <CardContent>
          <Button :disabled="isLoading" @click="downloadStructuredMetadata">
            <Icon v-if="isLoading" name="spinner" class="mr-2 h-4 w-4 animate-spin" />
            <Icon v-else name="download" class="mr-2 h-4 w-4" />
            {{ $t('repo.pipeline.metadata.download_metadata') }}
          </Button>
          <p class="text-sm text-muted-foreground mt-2">
            {{ downloadUrlTooltip }}
          </p>
        </CardContent>
      </Card>
    </div>
  </template>
  <div v-else class="flex h-full items-center justify-center p-8">
    <Card class="max-w-md">
      <CardContent class="pt-6 text-center">
        <Icon name="alert" class="h-12 w-12 mx-auto mb-4 text-destructive" />
        <p class="text-lg font-semibold">{{ $t('repo.pipeline.metadata.no_permission') }}</p>
      </CardContent>
    </Card>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import MetadataField from '~/components/repo/pipeline/MetadataField.vue';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '~/components/ui/card';
import useApiClient from '~/compositions/useApiClient';
import { requiredInject } from '~/compositions/useInjectProvide';
import { useCrowTitle } from '~/compositions/useCrowTitle';

defineProps<{
  repoId: string;
  pipelineId: string;
}>();

const { t } = useI18n();
const apiClient = useApiClient();

const repo = requiredInject('repo');
const pipeline = requiredInject('pipeline');
const repoPermissions = requiredInject('repo-permissions');

const isLoading = ref(false);
const metadata = ref<any>(null);
const agentDetails = ref<Map<number, any>>(new Map());

// Computed properties for statistics and agent information
const workflowCount = computed(() => {
  return pipeline?.value?.workflows?.length || 0;
});

const stepCount = computed(() => {
  if (!pipeline?.value?.workflows) return 0;
  return pipeline.value.workflows.reduce((total, workflow) => {
    return total + (workflow.children?.length || 0);
  }, 0);
});

const agentInfo = computed(() => {
  if (!pipeline?.value?.workflows) return [];

  const agentMap = new Map<number, string[]>();

  pipeline.value.workflows.forEach((workflow) => {
    if (workflow.agent_id) {
      if (!agentMap.has(workflow.agent_id)) {
        agentMap.set(workflow.agent_id, []);
      }
      agentMap.get(workflow.agent_id)!.push(workflow.name);
    }
  });

  return Array.from(agentMap.entries(), ([id, workflows]) => {
    const details = agentDetails.value.get(id) || {};
    return {
      id,
      workflows,
      name: details.name,
      platform: details.platform,
      backend: details.backend,
      priority: details.priority,
      custom_labels: details.custom_labels,
    };
  });
});

const uniqueAgentCount = computed(() => {
  return agentInfo.value.length;
});

// Computed properties for improved fields
const commitHash = computed(() => {
  return metadata?.value?.curr?.commit?.sha || pipeline?.value?.commit || t('repo.pipeline.metadata.fields.na');
});

const commitUrl = computed(() => {
  const hash = metadata?.value?.curr?.commit?.sha || pipeline?.value?.commit;
  const forgeUrl = repo?.value?.forge_url || metadata?.value?.repo?.forge_url;

  if (!hash || !forgeUrl) return null;

  // Construct commit URL - this works for GitHub, GitLab, Gitea, etc.
  return `${forgeUrl}/commit/${hash}`;
});

const hasDeployment = computed(() => {
  return !!(metadata?.value?.curr?.deploy_to || pipeline?.value?.deploy_to);
});

const deploymentEnvironment = computed(() => {
  return metadata?.value?.curr?.deploy_to || pipeline?.value?.deploy_to || 'default';
});

const defaultBranch = computed(() => {
  return metadata?.value?.repo?.branch || repo?.value?.default_branch || 'main';
});

const repositoryUrl = computed(() => {
  return metadata?.value?.repo?.forge_url || repo?.value?.forge_url;
});

const trustedDisplay = computed(() => {
  const trusted = metadata?.value?.repo?.trusted || repo?.value?.trusted;
  if (!trusted) return 'false';

  const enabledFeatures: string[] = [];
  if (trusted.network) enabledFeatures.push('network');
  if (trusted.volumes) enabledFeatures.push('volumes');
  if (trusted.security) enabledFeatures.push('security');

  if (enabledFeatures.length === 0) return 'false';
  return enabledFeatures.join(', ');
});

const metadataFileName = computed(
  () => `${repo?.value.full_name.replaceAll('/', '-')}-pipeline-${pipeline?.value.number}-structured-metadata.json`,
);

const downloadUrlTooltip = computed(() => {
  return `/api/repos/${repo?.value?.id}/pipelines/${pipeline?.value?.number}/metadata`;
});

function formatDate(timestamp: number | string | null | undefined): string {
  if (!timestamp || timestamp === 0) return t('repo.pipeline.metadata.fields.na');
  // Convert Unix timestamp (seconds) to JavaScript Date (milliseconds)
  const date = new Date(typeof timestamp === 'string' ? Number.parseInt(timestamp) * 1000 : timestamp * 1000);
  return date.toLocaleString();
}

async function loadMetadata() {
  if (!repo?.value || !pipeline?.value || !repoPermissions?.value?.push) {
    return;
  }

  try {
    metadata.value = await apiClient.getPipelineMetadata(repo.value.id, pipeline.value.number);

    // Load agent details for all agents used in this pipeline
    if (pipeline.value.workflows) {
      const agentIds = [...new Set(pipeline.value.workflows.map((w) => w.agent_id).filter(Boolean))];

      for (const agentId of agentIds) {
        try {
          const agent = await apiClient.getAgent(agentId as number);
          agentDetails.value.set(agentId as number, agent);
        } catch (error) {
          console.warn(`Failed to load agent ${agentId}:`, error);
        }
      }
    }
  } catch (error) {
    console.error('Error loading metadata:', error);
  }
}

// Create structured metadata format
function createStructuredMetadata() {
  const structuredData = {
    pipeline: {
      id: pipeline?.value?.id,
      number: pipeline?.value?.number,
      status: pipeline?.value?.status,
      event: pipeline?.value?.event,
      branch: pipeline?.value?.branch,
      commit: pipeline?.value?.commit,
      created: pipeline?.value?.created,
      started: pipeline?.value?.started,
      finished: pipeline?.value?.finished,
      deploy_to: pipeline?.value?.deploy_to,
    },
    repository: {
      id: repo?.value?.id,
      name: repo?.value?.name,
      full_name: repo?.value?.full_name,
      owner: repo?.value?.owner,
      scm: repo?.value?.scm,
      forge_url: repo?.value?.forge_url,
      clone_url: repo?.value?.clone_url,
      default_branch: repo?.value?.default_branch,
      private: repo?.value?.private,
      trusted: repo?.value?.trusted,
    },
    statistics: {
      workflow_count: workflowCount.value,
      step_count: stepCount.value,
      unique_agents: uniqueAgentCount.value,
    },
    workflows:
      pipeline?.value?.workflows?.map((workflow) => ({
        id: workflow.id,
        name: workflow.name,
        state: workflow.state,
        agent_id: workflow.agent_id,
        started: workflow.started,
        finished: workflow.finished,
        environ: workflow.environ,
        depends_on: workflow.depends_on,
        step_count: workflow.children?.length || 0,
      })) || [],
    agents: agentInfo.value.map((agent) => ({
      id: agent.id,
      name: agent.name,
      platform: agent.platform,
      backend: agent.backend,
      priority: agent.priority,
      custom_labels: agent.custom_labels,
      workflows: agent.workflows,
    })),
    api_metadata: metadata.value,
    generated_at: new Date().toISOString(),
    generated_by: 'crow-ci-web-ui',
  };

  return structuredData;
}

async function downloadStructuredMetadata() {
  if (!repo?.value || !pipeline?.value || !repoPermissions?.value?.push) {
    return;
  }

  isLoading.value = true;
  try {
    const structuredData = createStructuredMetadata();

    // Create a Blob with the structured JSON data
    const blob = new Blob([JSON.stringify(structuredData, null, 2)], { type: 'application/json' });

    // Create a download link and trigger the download
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = metadataFileName.value;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
  } catch (error) {
    console.error('Error creating structured metadata:', error);
  } finally {
    isLoading.value = false;
  }
}

onMounted(() => {
  loadMetadata();
});

useCrowTitle(
  computed(() => [
    t('repo.pipeline.metadata.title'),
    t('repo.pipeline.pipeline', { pipelineId: pipeline.value.id }),
    repo.value.full_name,
  ]),
);
</script>
