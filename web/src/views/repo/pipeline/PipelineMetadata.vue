<!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
<template>
  <template v-if="repoPermissions && repoPermissions.push">
    <!-- Metadata Information Sections -->
    <div class="space-y-4">
      <!-- Pipeline Information -->
      <Panel>
        <MetadataInputField :label="$t('repo.pipeline.metadata.pipeline_info')">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.number') }}:</strong>
              {{ metadata?.curr?.number || pipeline?.number || $t('repo.pipeline.metadata.fields.na') }}
            </div>
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.status') }}:</strong>
              {{ metadata?.curr?.status || pipeline?.status || $t('repo.pipeline.metadata.fields.na') }}
            </div>
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.event') }}:</strong>
              {{ metadata?.curr?.event || pipeline?.event || $t('repo.pipeline.metadata.fields.na') }}
            </div>
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.branch') }}:</strong>
              {{ metadata?.curr?.commit?.branch || pipeline?.branch || $t('repo.pipeline.metadata.fields.na') }}
            </div>
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.commit') }}: </strong>
              <a
                v-if="commitUrl"
                :href="commitUrl"
                target="_blank"
                class="text-wp-link-100 hover:text-wp-link-200 underline"
              >
                {{ commitHash }}
              </a>
              <span v-else>{{ commitHash }}</span>
            </div>
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.created') }}:</strong>
              {{ formatDate(metadata?.curr?.created || pipeline?.created) }}
            </div>
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.started') }}:</strong>
              {{ formatDate(metadata?.curr?.started || pipeline?.started) }}
            </div>
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.finished') }}:</strong>
              {{ formatDate(metadata?.curr?.finished || pipeline?.finished) }}
            </div>
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.deployment') }}:</strong>
              {{ hasDeployment ? 'true' : 'false' }}
            </div>
            <div v-if="hasDeployment">
              <strong>{{ $t('repo.pipeline.metadata.fields.environment') }}:</strong>
              {{ deploymentEnvironment }}
            </div>
          </div>
        </MetadataInputField>
      </Panel>

      <!-- Repository Information -->
      <Panel>
        <MetadataInputField :label="$t('repo.pipeline.metadata.repository_info')">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.id') }}:</strong>
              {{ metadata?.repo?.id || repo?.id || $t('repo.pipeline.metadata.fields.na') }}
            </div>
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.name') }}:</strong>
              {{ metadata?.repo?.name || repo?.name || $t('repo.pipeline.metadata.fields.na') }}
            </div>
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.url') }}: </strong>
              <a
                v-if="repositoryUrl"
                :href="repositoryUrl"
                target="_blank"
                class="text-wp-link-100 hover:text-wp-link-200 underline"
              >
                {{ repositoryUrl }}
              </a>
              <span v-else>{{ $t('repo.pipeline.metadata.fields.na') }}</span>
            </div>
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.default_branch') }}:</strong>
              {{ defaultBranch }}
            </div>
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.private') }}:</strong>
              {{ metadata?.repo?.private || repo?.private || $t('repo.pipeline.metadata.fields.na') }}
            </div>
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.trusted') }}:</strong>
              {{ JSON.stringify(metadata?.repo?.trusted) || repo?.trusted || $t('repo.pipeline.metadata.fields.na') }}
            </div>
          </div>
        </MetadataInputField>
      </Panel>

      <!-- Pipeline Statistics -->
      <Panel>
        <MetadataInputField :label="$t('repo.pipeline.metadata.statistics')">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.workflow_count') }}:</strong> {{ workflowCount }}
            </div>
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.step_count') }}:</strong> {{ stepCount }}
            </div>
            <div>
              <strong>{{ $t('repo.pipeline.metadata.fields.unique_agents') }}:</strong> {{ uniqueAgentCount }}
            </div>
          </div>
        </MetadataInputField>
      </Panel>

      <!-- Agent Information -->
      <Panel v-if="agentInfo.length > 0">
        <MetadataInputField :label="$t('repo.pipeline.metadata.agent_info')">
          <div class="space-y-4">
            <div
              v-for="agent in agentInfo"
              :key="agent.id"
              class="border border-wp-background-300 dark:border-wp-background-400 rounded p-3"
            >
              <div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
                <div>
                  <strong>{{ $t('repo.pipeline.metadata.fields.agent_id') }}: </strong>
                  <router-link :to="agentPageRoute" class="text-wp-link-100 hover:text-wp-link-200 underline">
                    {{ agent.id }}
                  </router-link>
                </div>
                <div>
                  <strong>{{ $t('repo.pipeline.metadata.fields.agent_name') }}:</strong>
                  {{ agent.name || $t('repo.pipeline.metadata.fields.na') }}
                </div>
                <div>
                  <strong>{{ $t('repo.pipeline.metadata.fields.agent_platform') }}:</strong>
                  {{ agent.platform || $t('repo.pipeline.metadata.fields.na') }}
                </div>
                <div>
                  <strong>{{ $t('repo.pipeline.metadata.fields.agent_backend') }}:</strong>
                  {{ agent.backend || $t('repo.pipeline.metadata.fields.na') }}
                </div>
                <div v-if="agent.priority && agent.priority > 0">
                  <strong>{{ $t('repo.pipeline.metadata.fields.agent_priority') }}:</strong>
                  {{ agent.priority }}
                </div>
                <div>
                  <strong>{{ $t('repo.pipeline.metadata.fields.workflow_count') }}:</strong>
                  {{ agent.workflows.length }}
                </div>
                <div v-if="agent.custom_labels && Object.keys(agent.custom_labels).length > 0" class="col-span-full">
                  <strong>{{ $t('repo.pipeline.metadata.fields.agent_labels') }}:</strong>
                  <span
                    v-for="(value, key) in agent.custom_labels"
                    :key="key"
                    class="ml-2 inline-block bg-wp-background-200 dark:bg-wp-background-300 px-2 py-1 rounded text-xs"
                  >
                    {{ key }}={{ value }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </MetadataInputField>
      </Panel>

      <!-- Download Section -->
      <Panel>
        <MetadataInputField label="Download">
          <div class="flex items-center space-x-4">
            <Button
              :is-loading="isLoading"
              :text="$t('repo.pipeline.metadata.download_metadata')"
              :title="downloadUrlTooltip"
              @click="downloadStructuredMetadata"
            />
          </div>
        </MetadataInputField>
      </Panel>
    </div>
  </template>
  <div v-else class="flex h-full items-center justify-center">
    <div class="rounded-lg bg-wp-error-100 p-8 text-center shadow-lg dark:bg-wp-error-200">
      <p class="text-2xl font-bold text-white">{{ $t('repo.pipeline.metadata.no_permission') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';

import Button from '~/components/atomic/Button.vue';
import MetadataInputField from '~/components/form/MetadataInputField.vue';
import Panel from '~/components/layout/Panel.vue';
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

  return Array.from(agentMap.entries()).map(([id, workflows]) => {
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

const metadataFileName = computed(
  () => `${repo?.value.full_name.replaceAll('/', '-')}-pipeline-${pipeline?.value.number}-structured-metadata.json`,
);

const downloadUrlTooltip = computed(() => {
  return `/api/repos/${repo?.value?.id}/pipelines/${pipeline?.value?.number}/metadata`;
});

const agentPageRoute = computed(() => {
  // Navigate to admin agents page if user has admin permissions, otherwise user agents page
  if (repoPermissions?.value?.admin) {
    return { name: 'admin-settings-agents' };
  }
  return { name: 'user-agents' };
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
