<template>
  <div class="flex w-full flex-col gap-1 pb-2 text-foreground">
    <div class="flex shrink-0 flex-wrap justify-between gap-1 rounded-md border border-border bg-card p-2 shadow-xs">
      <div class="flex shrink-0 items-center space-x-1">
        <slot name="panel-chevron" />
        <div class="flex items-center">
          <Icon v-if="pipeline.event === 'cron'" name="stopwatch" />
          <UserAvatar v-else :avatar-url="pipeline.author_avatar" :username="pipeline.author" size="md" />
        </div>
        <span>{{ pipeline.author }}</span>
      </div>
      <a
        v-if="
          pipeline.event === 'pull_request' ||
          pipeline.event === 'pull_request_closed' ||
          pipeline.event === 'pull_request_edited'
        "
        class="flex min-w-0 items-center space-x-1 text-primary hover:text-primary/80"
        :href="pipeline.forge_url"
      >
        <Icon name="pull-request" size="1rem" /> <span class="truncate">{{ prettyRef }}</span>
      </a>
      <div
        v-else-if="pipeline.event === 'push' || pipeline.event === 'manual' || pipeline.event === 'deployment'"
        class="text-sm flex min-w-0 items-center space-x-1"
      >
        <Icon v-if="pipeline.event === 'manual'" size="1rem" name="manual-pipeline" />
        <Icon v-else-if="pipeline.event === 'push'" size="1rem" name="branch" />
        <Icon v-else-if="pipeline.event === 'deployment'" size="1rem" name="deployment" />
        <span class="truncate ml-1">{{ prettyRef }}</span>
      </div>
      <div v-else class="text-sm flex min-w-0 items-center space-x-1">
        <Icon v-if="pipeline.event === 'tag' || pipeline.event === 'release'" size="1rem" name="tag" />
        <span class="truncate ml-1">{{ prettyRef }}</span>
      </div>

      <div class="flex shrink-0 items-center">
        <template v-if="pipeline.event === 'pull_request'">
          <Icon name="commit" size="1rem" /> <span>{{ pipeline.commit.slice(0, 10) }}</span>
        </template>
        <a
          v-else
          class="text-sm flex items-center text-primary hover:text-primary/80"
          :href="pipeline.forge_url"
          target="_blank"
        >
          <Icon name="commit" size="1rem" /> <span class="truncate ml-1">{{ pipeline.commit.slice(0, 10) }}</span>
        </a>
      </div>
    </div>
    <Panel v-if="pipeline.workflows === undefined || pipeline.workflows.length === 0">
      <span>{{ $t('repo.pipeline.no_pipeline_steps') }}</span>
    </Panel>
    <div class="relative min-h-0 w-full grow">
      <ScrollArea class="h-full">
        <div class="flex flex-col gap-y-2">
          <div
            v-for="workflow in pipeline.workflows"
            :key="workflow.id"
            class="rounded-md border border-border bg-card p-2 shadow-xs"
          >
            <!-- Workflow Labels -->
            <div v-if="workflow.environ" class="flex flex-wrap justify-end gap-x-1 gap-y-2 pr-1 pt-1 text-xs pb-2">
              <div v-for="(value, key) in workflow.environ" :key="key"><Badge :label="key" :value="value" /></div>
            </div>
            <!-- Workflow Row -->
            <div
              class="grid grid-cols-[1.25rem_2rem_1fr_auto_4rem] gap-1 items-center transition-colors hover:bg-accent cursor-pointer"
              @click="toggleWorkflow(workflow.id)"
            >
              <!-- Chevron -->
              <span class="flex items-center justify-center h-8 w-8">
                <Icon
                  name="chevron-right"
                  class="transition-transform duration-150"
                  :class="{ 'rotate-90 transform': !workflowsCollapsed[workflow.id] }"
                />
              </span>
              <!-- Status Icon -->
              <span class="flex items-center justify-center h-8 w-8">
                <PipelineStatusIcon :status="workflow.state" class="h-4 w-4" />
              </span>
              <!-- Name -->
              <span class="min-w-0 font-medium truncate">
                {{ workflow.name }}
              </span>
              <!-- Agent Icon/Name and Cancel Button (combined column, right-aligned) -->
              <span class="flex items-center justify-end gap-1">
                <a
                  v-if="workflow.agent_id"
                  :href="repoPermissions?.admin ? '/admin/agents/' : '/user/agents/'"
                  class="text-primary hover:text-primary/80 shrink-0 flex items-center gap-1.5"
                  :title="getAgentDisplay(workflow.agent_id) || 'Agent'"
                  @click.stop
                >
                  <Icon name="agent" class="w-4 h-4 shrink-0" />
                  <span class="text-sm hidden lg:inline whitespace-nowrap">
                    {{ getAgentDisplay(workflow.agent_id) }}
                  </span>
                </a>
                <IconButton
                  v-if="hasPushPermission && (workflow.state === 'pending' || workflow.state === 'running')"
                  :is-loading="isCancelingWorkflow[workflow.id]"
                  icon="close"
                  class="text-red-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20"
                  :title="$t('repo.pipeline.actions.cancel')"
                  size="1.3rem"
                  @click.stop="cancelWorkflow(workflow.id)"
                />
              </span>
              <!-- Timer -->
              <span class="flex justify-end pr-2">
                <PipelineStepDuration v-if="workflow.started !== workflow.finished" :workflow="workflow" />
              </span>
            </div>
            <!-- Canceled Message -->
            <div v-if="workflow.state === 'killed'" class="grid grid-cols-[1.25rem_2rem_1fr_auto_4rem]">
              <span></span>
              <span class="flex items-center justify-center h-8 w-8"></span>
              <span
                class="italic flex items-center text-xs text-destructive ml-[0.5rem]"
                style="font-size: 0.85em; line-height: 1.5"
              >
                {{ $t('repo.pipeline.actions.canceled_by_user') }}
              </span>
              <span></span>
              <span></span>
            </div>
            <!-- Steps -->
            <div
              class="overflow-hidden transition-height duration-150"
              :class="{ 'max-h-0': workflowsCollapsed[workflow.id] }"
            >
              <div
                v-for="step in workflow.children"
                :key="step.pid"
                class="grid grid-cols-[1.25rem_2rem_1fr_auto_4rem] gap-1 items-center transition-colors hover:bg-accent cursor-pointer border-b border-border last:border-b-0"
                :class="{
                  'border-t border-border': step === workflow.children[0],
                  'bg-accent': step.pid === selectedStepId,
                }"
                @click="selectStep(step.pid)"
              >
                <span></span>
                <span class="flex items-center justify-center h-8 w-8">
                  <PipelineStatusIcon :service="step.type === StepType.Service" :status="step.state" class="h-4 w-4" />
                </span>
                <span class="min-w-0 truncate">{{ step.name }}</span>
                <span></span>
                <span class="flex justify-end pr-2">
                  <PipelineStepDuration :step="step" />
                </span>
              </div>
            </div>
          </div>
        </div>
      </ScrollArea>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { computed, ref, toRef, watch } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
import Panel from '~/components/layout/Panel.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import UserAvatar from '~/components/user/UserAvatar.vue';
import PipelineStepDuration from '~/components/repo/pipeline/PipelineStepDuration.vue';
import { ScrollArea } from '~/components/ui/scroll-area';
import { requiredInject } from '~/compositions/useInjectProvide';
import usePipeline from '~/compositions/usePipeline';
import { StepType } from '~/lib/api/types';
import type { Pipeline, PipelineStep } from '~/lib/api/types';
import IconButton from '~/components/atomic/IconButton.vue';
import useApiClient from '~/compositions/useApiClient';
import useNotifications from '~/compositions/useNotifications';
import { usePipelineStore } from '~/store/pipelines';
import { useI18n } from 'vue-i18n';
import Badge from '~/components/atomic/Badge.vue';

const props = defineProps<{
  pipeline: Pipeline;
  selectedStepId?: number | null;
}>();

const emit = defineEmits<{
  (event: 'update:selected-step-id', selectedStepId: number): void;
}>();

const pipeline = toRef(props, 'pipeline');
const selectedStepId = toRef(props, 'selectedStepId');
const { prettyRef } = usePipeline(pipeline);
const repo = requiredInject('repo');
const repoPermissions = requiredInject('repo-permissions');
const apiClient = useApiClient();
const notifications = useNotifications();
const pipelineStore = usePipelineStore();
const i18n = useI18n();

const workflowsCollapsed = ref<Record<PipelineStep['id'], boolean>>(
  pipeline.value.workflows && pipeline.value.workflows.length > 1
    ? (pipeline.value.workflows || []).reduce(
        (collapsed, workflow) => ({
          ...collapsed,
          [workflow.id]:
            ['success', 'skipped', 'blocked'].includes(workflow.state) &&
            !workflow.children.some((child) => child.pid === selectedStepId.value),
        }),
        {},
      )
    : {},
);

const isCancelingWorkflow = ref<Record<number, boolean>>({});
const agentDetails = ref<Record<number, { name?: string }>>({});

const hasPushPermission = computed(() => repoPermissions.value && repoPermissions.value.push);

function toggleWorkflow(id: number) {
  workflowsCollapsed.value[id] = !workflowsCollapsed.value[id];
}

function getAgentDisplay(agentId: number): string {
  const agent = agentDetails.value[agentId];
  return agent?.name || '';
}

async function loadAgentDetails() {
  if (!pipeline.value.workflows) {
    return;
  }

  const agentIds = new Set<number>();

  // Collect all unique agent IDs from workflows
  pipeline.value.workflows.forEach((workflow) => {
    if (workflow.agent_id) {
      agentIds.add(workflow.agent_id);
    }
  });

  // Nothing to load
  if (agentIds.size === 0) {
    return;
  }

  // Load details for each agent
  const details: Record<number, { name?: string }> = {};
  for (const agentId of agentIds) {
    try {
      const agent = await apiClient.getAgent(agentId);
      details[agentId] = agent;
    } catch (error) {
      console.warn(`Failed to load agent ${agentId}:`, error);
      details[agentId] = {};
    }
  }

  // Update all at once to ensure reactivity
  agentDetails.value = details;
}

// Load agent details when component mounts and when workflows change
watch(
  () => pipeline.value.workflows,
  (workflows) => {
    if (workflows && workflows.length > 0) {
      loadAgentDetails();
    }
  },
  { immediate: true },
);

async function cancelWorkflow(workflowId: number) {
  if (!repo.value || !pipeline.value?.number) return;
  isCancelingWorkflow.value = { ...isCancelingWorkflow.value, [workflowId]: true };
  try {
    await apiClient.cancelWorkflow(repo.value.id, pipeline.value.number, workflowId);
    // Add a small delay to ensure the backend has time to process the cancel request
    await new Promise((resolve) => setTimeout(resolve, 500));
    // Reload the pipeline from the backend to get the real workflow state
    await pipelineStore.loadPipeline(repo.value.id, pipeline.value.number);
    notifications.notify({
      title: i18n.t('notify_title_success'),
      text: i18n.t('repo.pipeline.actions.notify_text_cancel_success'),
      type: 'success',
      duration: 6000,
    });
  } catch {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      text: i18n.t('unknown_error'),
      type: 'error',
      duration: 9000,
    });
  } finally {
    isCancelingWorkflow.value = { ...isCancelingWorkflow.value, [workflowId]: false };
  }
}

function selectStep(stepId: number) {
  emit('update:selected-step-id', stepId);
}
</script>
