<template>
  <div class="flex h-full w-full flex-col gap-1 pb-2 text-foreground">
    <div
      class="flex shrink-0 flex-wrap justify-between gap-1 rounded-md border border-border bg-card p-2 shadow-xs mb-1"
    >
      <div class="flex shrink-0 items-center space-x-1">
        <slot name="panel-chevron" />
        <div class="flex items-center">
          <Icon v-if="pipeline.event === 'cron'" name="stopwatch" />
          <UserAvatar
            v-else
            :avatar-url="pipeline.author_avatar"
            :username="pipeline.author"
            :forge-id="repo.forge_id"
            size="md"
          />
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

      <div class="flex shrink-0 items-center gap-1">
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
        <!-- Tree/flat view toggle -->
        <TooltipProvider v-if="hasAnyDependencies">
          <Tooltip>
            <TooltipTrigger as-child>
              <button
                class="inline-flex items-center justify-center rounded p-1 text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
                @click="treeView = !treeView"
              >
                <Icon :name="treeView ? 'tasks' : 'list'" size="1rem" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="bottom">
              {{ treeView ? $t('repo.pipeline.actions.flat_view') : $t('repo.pipeline.actions.tree_view') }}
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </div>
    </div>
    <Panel v-if="pipeline.workflows === undefined || pipeline.workflows.length === 0">
      <span>{{ $t('repo.pipeline.no_pipeline_steps') }}</span>
    </Panel>
    <div class="relative min-h-0 w-full grow">
      <ScrollArea class="h-full">
        <div class="flex flex-col gap-y-2">
          <template v-for="group in workflowGroups" :key="group.key">
            <!-- Matrix group: multiple workflows sharing same pipeline_name -->
            <PipelineMatrixGroup
              v-if="group.workflows.length > 1"
              :workflows="group.workflows"
              :all-workflows="allWorkflows"
              :group-name="group.key"
              :selected-step-id="selectedStepId"
              :agent-details="agentDetails"
              :is-canceling-workflow="isCancelingWorkflow"
              :is-rerunning-workflow="isRerunningWorkflow"
              :is-rerunning-step="isRerunningStep"
              :tree-view="treeView"
              @update:selected-step-id="selectStep"
              @cancel-workflow="cancelWorkflow($event)"
              @rerun-workflow="rerunWorkflow($event)"
              @rerun-step="(wfId: number, sId: number) => rerunStep(wfId, sId)"
            />
            <!-- Single workflow: render as before -->
            <div v-else class="rounded-md border border-border bg-card p-2 shadow-xs">
              <!-- Workflow Labels -->
              <div
                v-if="group.workflows[0].environ && Object.keys(group.workflows[0].environ).length > 0"
                class="flex flex-wrap justify-end gap-x-1 gap-y-2 pr-1 pt-1 text-xs pb-2"
              >
                <div v-for="(value, key) in group.workflows[0].environ" :key="key">
                  <Badge :label="key" :value="value" />
                </div>
              </div>
              <!-- Workflow Row -->
              <div
                class="flex items-center gap-1 h-8 px-1 transition-colors hover:bg-accent cursor-pointer"
                @click="toggleWorkflow(group.workflows[0].id)"
              >
                <!-- Chevron -->
                <Icon
                  name="chevron-right"
                  class="shrink-0 transition-transform duration-150"
                  :class="{ 'rotate-90 transform': !workflowsCollapsed[group.workflows[0].id] }"
                />
                <!-- Status Icon -->
                <PipelineStatusIcon :status="group.workflows[0].state" class="h-4 w-4 shrink-0 mr-0.5" />
                <!-- Name -->
                <span class="min-w-0 font-medium truncate">
                  {{ group.workflows[0].name }}
                </span>
                <!-- Workflow depends_on tooltip -->
                <TooltipProvider v-if="group.workflows[0].depends_on && group.workflows[0].depends_on.length > 0">
                  <Tooltip>
                    <TooltipTrigger as-child>
                      <span class="shrink-0 text-muted-foreground">
                        <Icon name="dependency" size="0.85rem" />
                      </span>
                    </TooltipTrigger>
                    <TooltipContent>
                      {{
                        $t('repo.pipeline.actions.workflow_depends_on_tooltip', {
                          names: group.workflows[0].depends_on.join(', '),
                        })
                      }}
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
                <!-- Attempt dropdown -->
                <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                <DropdownMenu v-if="workflowHasReruns(group.workflows[0])">
                  <DropdownMenuTrigger as-child>
                    <button
                      class="shrink-0 inline-flex items-center gap-0.5 rounded border border-primary/70 px-1 py-0.5 text-[10px] font-medium text-muted-foreground hover:bg-accent transition-colors"
                      @click.stop
                    >
                      #{{ group.workflows[0].attempt }}
                      <Icon name="chevron-down" class="h-2.5 w-2.5" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="start">
                    <DropdownMenuItem
                      v-for="attempt in workflowAttempts(group.workflows[0])"
                      :key="attempt.id"
                      class="flex items-center gap-2 text-xs"
                      @click.stop="switchAttempt(group.workflows[0], attempt)"
                    >
                      <PipelineStatusIcon :status="attempt.state" class="h-3.5 w-3.5" />
                      #{{ attempt.attempt }}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
                <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                <!-- Spacer -->
                <span class="flex-1" />
                <!-- Agent -->
                <TooltipProvider v-if="group.workflows[0].agent_id">
                  <Tooltip>
                    <TooltipTrigger as-child>
                      <a
                        :href="repoPermissions?.admin ? '/admin/agents/' : '/user/agents/'"
                        class="shrink-0 text-primary hover:text-primary/80 flex items-center gap-1.5"
                        @click.stop
                      >
                        <Icon name="agent" class="w-4 h-4" />
                        <span class="text-sm hidden lg:inline truncate max-w-[100px]">
                          {{ getAgentDisplay(group.workflows[0].agent_id) }}
                        </span>
                      </a>
                    </TooltipTrigger>
                    <TooltipContent>
                      <div class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                        <div class="col-span-2 font-medium mb-1">
                          {{ getAgent(group.workflows[0].agent_id)?.name ?? `#${group.workflows[0].agent_id}` }}
                        </div>
                        <template v-if="getAgent(group.workflows[0].agent_id)">
                          <span class="opacity-70">{{ $t('repo.pipeline.agent_tooltip.platform') }}</span>
                          <span>{{ getAgent(group.workflows[0].agent_id)!.platform }}</span>
                          <span class="opacity-70">{{ $t('repo.pipeline.agent_tooltip.backend') }}</span>
                          <span>{{ getAgent(group.workflows[0].agent_id)!.backend }}</span>
                          <span class="opacity-70">{{ $t('repo.pipeline.agent_tooltip.workflows') }}</span>
                          <span>{{
                            `${getAgent(group.workflows[0].agent_id)!.running_workflows} / ${getAgent(group.workflows[0].agent_id)!.capacity}`
                          }}</span>
                          <template v-if="getAgent(group.workflows[0].agent_id)!.backend === 'docker'">
                            <span class="opacity-70">{{ $t('repo.pipeline.agent_tooltip.limit_mem') }}</span>
                            <span>{{ formatBytes(getAgent(group.workflows[0].agent_id)!.limit_mem, $t) }}</span>
                            <span class="opacity-70">{{ $t('repo.pipeline.agent_tooltip.limit_cpu_quota') }}</span>
                            <span>{{
                              formatCPUQuota(getAgent(group.workflows[0].agent_id)!.limit_cpu_quota, $t)
                            }}</span>
                          </template>
                        </template>
                      </div>
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
                <!-- Rerun -->
                <IconButton
                  v-if="hasPushPermission && isWorkflowCompleted(group.workflows[0])"
                  :is-loading="isRerunningWorkflow[group.workflows[0].id]"
                  icon="solar:restart-bold"
                  class="shrink-0 text-primary hover:text-primary/80 hover:bg-accent"
                  :title="$t('repo.pipeline.actions.rerun_workflow')"
                  @click.stop="rerunWorkflow(group.workflows[0].id)"
                />
                <!-- Cancel -->
                <IconButton
                  v-if="
                    hasPushPermission &&
                    (group.workflows[0].state === 'pending' || group.workflows[0].state === 'running')
                  "
                  :is-loading="isCancelingWorkflow[group.workflows[0].id]"
                  icon="close"
                  class="shrink-0 text-red-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20"
                  :title="$t('repo.pipeline.actions.cancel')"
                  size="1.3rem"
                  @click.stop="cancelWorkflow(group.workflows[0].id)"
                />
                <!-- Duration -->
                <span class="shrink-0 text-right w-16 pr-1">
                  <PipelineStepDuration
                    v-if="group.workflows[0].started !== group.workflows[0].finished"
                    :workflow="group.workflows[0]"
                  />
                </span>
              </div>
              <!-- Canceled Message -->
              <div v-if="group.workflows[0].state === 'killed'" class="flex items-center gap-1 h-8 px-1">
                <span class="shrink-0 w-4" />
                <span class="shrink-0 w-4" />
                <span class="italic text-xs text-destructive">
                  {{ $t('repo.pipeline.actions.canceled_by_user') }}
                </span>
              </div>
              <!-- Steps -->
              <div
                class="overflow-hidden transition-height duration-150"
                :class="{ 'max-h-0': workflowsCollapsed[group.workflows[0].id] }"
              >
                <div
                  v-for="(node, nodeIdx) in getStepNodes(group.workflows[0])"
                  :key="node.step.pid"
                  class="flex items-center gap-1 h-8 px-1 transition-colors hover:bg-accent cursor-pointer border-b border-border last:border-b-0"
                  :class="{
                    'border-t border-border': nodeIdx === 0,
                    'bg-accent': node.step.pid === selectedStepId,
                  }"
                  @click="selectStep(node.step.pid)"
                >
                  <!-- Base indentation spacer -->
                  <span class="shrink-0 w-4" />
                  <!-- Tree connectors for nested steps -->
                  <template v-if="treeView && node.depth > 0">
                    <span
                      v-for="d in node.depth"
                      :key="d"
                      class="step-tree-connector shrink-0 w-4 h-8 relative"
                      :class="{
                        'connector-vertical': d < node.depth && node.connectorLines[d - 1],
                        'connector-branch': d === node.depth && !node.isLast,
                        'connector-last': d === node.depth && node.isLast,
                      }"
                    />
                  </template>
                  <PipelineStatusIcon
                    :service="node.step.type === StepType.Service"
                    :status="node.step.state"
                    class="h-4 w-4 shrink-0 mr-0.5"
                  />
                  <span class="min-w-0 truncate"
                    >{{ node.step.name
                    }}<span class="text-muted-foreground">{{
                      stepDupSuffix(group.workflows[0], node.step)
                    }}</span></span
                  >
                  <span v-if="node.step.state === 'preparing'" class="shrink-0 text-xs text-blue-500">
                    {{ $t('repo.pipeline.status.preparing') }}
                  </span>
                  <!-- Multi-dependency tooltip -->
                  <TooltipProvider v-if="node.effectiveDeps.length > 1">
                    <Tooltip>
                      <TooltipTrigger as-child>
                        <span class="shrink-0 text-muted-foreground">
                          <Icon name="dependency" size="0.85rem" />
                        </span>
                      </TooltipTrigger>
                      <TooltipContent>
                        {{
                          $t('repo.pipeline.actions.depends_on_tooltip', {
                            names: node.effectiveDeps.join(', '),
                          })
                        }}
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                  <!-- Step attempt dropdown -->
                  <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                  <DropdownMenu v-if="stepHasReruns(node.step)">
                    <DropdownMenuTrigger as-child>
                      <button
                        class="shrink-0 inline-flex items-center gap-0.5 rounded border border-primary/70 px-1 py-0.5 text-[10px] font-medium text-muted-foreground hover:bg-accent transition-colors"
                        @click.stop
                      >
                        #{{ node.step.attempt ?? 1 }}
                        <Icon name="chevron-down" class="h-2.5 w-2.5" />
                      </button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="start">
                      <DropdownMenuItem
                        v-for="attempt in stepAttempts(node.step)"
                        :key="attempt.id"
                        class="flex items-center gap-2 text-xs"
                        @click.stop="selectStep(attempt.pid)"
                      >
                        <PipelineStatusIcon :status="attempt.state" class="h-3.5 w-3.5" />
                        #{{ attempt.attempt ?? 1 }}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                  <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                  <span class="flex-1" />
                  <!-- Rerun step -->
                  <IconButton
                    v-if="hasPushPermission && isStepCompleted(node.step)"
                    :is-loading="isRerunningStep[node.step.id]"
                    icon="solar:restart-bold"
                    class="shrink-0 text-primary hover:text-primary/80 hover:bg-accent"
                    size="0.9rem"
                    :title="$t('repo.pipeline.actions.rerun_step')"
                    @click.stop="rerunStep(group.workflows[0].id, node.step.id)"
                  />
                  <span class="shrink-0 text-right w-16 pr-1">
                    <PipelineStepDuration :step="node.step" />
                  </span>
                </div>
              </div>
            </div>
          </template>
        </div>
      </ScrollArea>
    </div>
  </div>
</template>

<script lang="ts" setup>
import Badge from '~/components/atomic/Badge.vue';
import Icon from '~/components/atomic/Icon.vue';
import IconButton from '~/components/atomic/IconButton.vue';
import Panel from '~/components/layout/Panel.vue';
import PipelineMatrixGroup from '~/components/repo/pipeline/PipelineMatrixGroup.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import PipelineStepDuration from '~/components/repo/pipeline/PipelineStepDuration.vue';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import { ScrollArea } from '~/components/ui/scroll-area';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';
import UserAvatar from '~/components/user/UserAvatar.vue';
import useApiClient from '~/compositions/useApiClient';
import { requiredInject } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import usePipeline from '~/compositions/usePipeline';
import { formatBytes, formatCPUQuota } from '~/lib/agent-utils';
import { StepType } from '~/lib/api/types';
import type { Agent, Pipeline, PipelineStep, PipelineWorkflow } from '~/lib/api/types';
import type { StepTreeNode } from '~/lib/pipeline-utils';
import {
  buildStepTree,
  duplicateStepNameIndexes,
  getLatestAttempts,
  getStepAttempts,
  getWorkflowAttempts,
  hasReruns,
  hasStepReruns,
  identifyMatrixGroups,
} from '~/lib/pipeline-utils';
import { usePipelineStore } from '~/store/pipelines';
import { computed, ref, toRef, watch } from 'vue';
import { useI18n } from 'vue-i18n';

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

const allWorkflows = computed(() => pipeline.value.workflows || []);
const workflowGroups = computed(() => identifyMatrixGroups(getLatestAttempts(allWorkflows.value)));

const treeView = ref(true);

const hasAnyDependencies = computed(() =>
  allWorkflows.value.some(
    (wf) =>
      (wf.depends_on && wf.depends_on.length > 0) || wf.children.some((s) => s.depends_on && s.depends_on.length > 0),
  ),
);

const stepNameDupIndexes = computed(() => {
  const map = new Map<number, Map<number, number>>();
  for (const wf of allWorkflows.value) {
    map.set(wf.id, duplicateStepNameIndexes(wf.children));
  }
  return map;
});

function stepDupSuffix(workflow: PipelineWorkflow, step: PipelineStep): string {
  const index = stepNameDupIndexes.value.get(workflow.id)?.get(step.pid);
  return index ? ` (${index})` : '';
}

function getStepNodes(workflow: PipelineWorkflow): StepTreeNode[] {
  if (treeView.value) {
    return buildStepTree(workflow.children);
  }
  const dagRootNames = new Set(
    workflow.children.filter((s) => !s.depends_on || s.depends_on.length === 0).map((s) => s.name),
  );
  return workflow.children.map((step) => ({
    step,
    depth: 0,
    isLast: true,
    connectorLines: [],
    effectiveDeps: (step.depends_on ?? []).filter((d) => !dagRootNames.has(d)),
  }));
}

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
const isRerunningWorkflow = ref<Record<number, boolean>>({});
const isRerunningStep = ref<Record<number, boolean>>({});
const agentDetails = ref<Record<number, Agent>>({});

const hasPushPermission = computed(() => repoPermissions.value && repoPermissions.value.push);

function toggleWorkflow(id: number) {
  workflowsCollapsed.value[id] = !workflowsCollapsed.value[id];
}

function getAgentDisplay(agentId: number): string {
  const agent = agentDetails.value[agentId];
  return agent?.name || '';
}

function getAgent(agentId: number): Agent | undefined {
  return agentDetails.value[agentId];
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
  const details: Record<number, Agent> = {};
  for (const agentId of agentIds) {
    try {
      const agent = await apiClient.getAgent(agentId, { silent: true });
      details[agentId] = agent;
    } catch (error) {
      console.warn(`Failed to load agent ${agentId}:`, error);
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

async function rerunWorkflow(workflowId: number) {
  if (!repo.value || !pipeline.value?.number) return;
  isRerunningWorkflow.value = { ...isRerunningWorkflow.value, [workflowId]: true };
  try {
    await apiClient.rerunWorkflow(repo.value.id, pipeline.value.number, workflowId);
    notifications.notify({
      title: i18n.t('notify_title_success'),
      text: i18n.t('repo.pipeline.actions.notify_text_rerun_success'),
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
    isRerunningWorkflow.value = { ...isRerunningWorkflow.value, [workflowId]: false };
  }
}

function workflowAttempts(workflow: PipelineWorkflow) {
  return getWorkflowAttempts(allWorkflows.value, workflow);
}

function workflowHasReruns(workflow: PipelineWorkflow) {
  return hasReruns(allWorkflows.value, workflow);
}

function isWorkflowCompleted(workflow: PipelineWorkflow) {
  return workflow.state !== 'pending' && workflow.state !== 'running';
}

function isStepCompleted(step: PipelineStep) {
  return step.state !== 'pending' && step.state !== 'preparing' && step.state !== 'running';
}

function stepAttempts(step: PipelineStep) {
  return getStepAttempts(allWorkflows.value, step);
}

function stepHasReruns(step: PipelineStep) {
  return hasStepReruns(allWorkflows.value, step);
}

async function rerunStep(workflowId: number, stepId: number) {
  if (!repo.value || !pipeline.value?.number) return;
  isRerunningStep.value = { ...isRerunningStep.value, [stepId]: true };
  try {
    await apiClient.rerunStep(repo.value.id, pipeline.value.number, workflowId, stepId);
    notifications.notify({
      title: i18n.t('notify_title_success'),
      text: i18n.t('repo.pipeline.actions.notify_text_step_rerun_success'),
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
    isRerunningStep.value = { ...isRerunningStep.value, [stepId]: false };
  }
}

function switchAttempt(_workflow: PipelineWorkflow, targetAttempt: PipelineWorkflow) {
  // When switching to a different attempt, expand it and select the first step
  workflowsCollapsed.value[targetAttempt.id] = false;
  if (targetAttempt.children.length > 0) {
    selectStep(targetAttempt.children[0].pid);
  }
}

function selectStep(stepId: number) {
  emit('update:selected-step-id', stepId);
}
</script>

<style scoped>
.step-tree-connector {
  --connector-color: var(--color-border);
}

/* Vertical line through the full height (ancestor still has children below) */
.connector-vertical::before {
  content: '';
  position: absolute;
  left: 50%;
  top: 0;
  bottom: 0;
  width: 1px;
  background: var(--connector-color);
}

/* Branch connector: vertical line + horizontal branch (├──) */
.connector-branch::before {
  content: '';
  position: absolute;
  left: 50%;
  top: 0;
  bottom: 0;
  width: 1px;
  background: var(--connector-color);
}
.connector-branch::after {
  content: '';
  position: absolute;
  left: 50%;
  top: 50%;
  width: 50%;
  height: 1px;
  background: var(--connector-color);
}

/* Last child connector: vertical line from top to middle + horizontal branch (└──) */
.connector-last::before {
  content: '';
  position: absolute;
  left: 50%;
  top: 0;
  height: 50%;
  width: 1px;
  background: var(--connector-color);
}
.connector-last::after {
  content: '';
  position: absolute;
  left: 50%;
  top: 50%;
  width: 50%;
  height: 1px;
  background: var(--connector-color);
}
</style>
