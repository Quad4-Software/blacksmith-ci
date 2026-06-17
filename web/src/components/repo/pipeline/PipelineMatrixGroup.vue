<template>
  <div class="rounded-md border border-border bg-card p-2 shadow-xs">
    <!-- Group Header Row -->
    <div
      class="grid grid-cols-[1.25rem_1.25rem_1fr_auto_4rem] gap-1 items-center transition-colors hover:bg-accent cursor-pointer"
      @click="
        groupExpanded = !groupExpanded;
        userToggledGroup = true;
      "
    >
      <!-- Chevron -->
      <span class="flex items-center justify-center h-8">
        <Icon
          name="chevron-right"
          class="transition-transform duration-150"
          :class="{ 'rotate-90 transform': groupExpanded }"
        />
      </span>
      <!-- Aggregated Status Icon -->
      <span class="flex items-center justify-center h-8">
        <PipelineStatusIcon :status="aggregatedStatus" class="h-4 w-4" />
      </span>
      <!-- Group Name -->
      <span class="min-w-0 font-medium truncate">{{ groupName }}</span>
      <!-- Spacer for agent/cancel column -->
      <span></span>
      <!-- Aggregated Duration -->
      <span class="flex justify-end pr-2">
        <PipelineStepDuration
          v-if="aggregatedDuration.started !== aggregatedDuration.finished"
          :workflow="aggregatedDuration as any"
        />
      </span>
    </div>

    <!-- Variant List (expanded) -->
    <div class="overflow-hidden transition-height duration-150" :class="{ 'max-h-0': !groupExpanded }">
      <!-- Filter combobox + Status chips for large groups -->
      <div v-if="isLargeGroup" class="px-1 pt-2 pb-1 space-y-1.5">
        <!-- Multi-select combobox with KEY=value suggestions -->
        <Combobox
          v-model="selectedAxisValues"
          v-model:search-term="searchTerm"
          multiple
          :filter-function="filterComboboxItems"
        >
          <ComboboxAnchor
            class="flex flex-wrap gap-1 items-center rounded-md border border-input bg-background px-1.5 py-0.5 w-full"
          >
            <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
            <span
              v-for="tag in selectedAxisValues"
              :key="tag"
              class="inline-flex items-center gap-0.5 rounded bg-primary/10 text-primary px-1.5 py-0.5 text-xs font-mono"
            >
              {{ tag }}
              <button class="ml-0.5 hover:text-destructive" @click.stop.prevent="removeSelectedValue(tag)">
                &times;
              </button>
            </span>
            <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
            <button
              v-if="selectedAxisValues.length > 1"
              class="shrink-0 rounded px-1.5 py-0.5 text-xs text-muted-foreground hover:text-destructive transition-colors"
              @click.stop.prevent="
                selectedAxisValues = [];
                searchTerm = '';
              "
            >
              {{ $t('clear') }}
            </button>
            <ComboboxInput
              :placeholder="selectedAxisValues.length === 0 ? $t('repo.pipeline.matrix.filter_placeholder') : ''"
              class="flex-1 min-w-[4rem] bg-transparent text-xs outline-hidden placeholder:text-muted-foreground"
            />
          </ComboboxAnchor>
          <ComboboxList class="w-[var(--reka-combobox-trigger-width)]">
            <ComboboxViewport class="max-h-[200px]">
              <ComboboxEmpty class="py-2 text-center text-xs">{{
                $t('repo.pipeline.matrix.no_matches')
              }}</ComboboxEmpty>
              <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
              <ComboboxGroup v-for="axis in matrixAxes" :key="axis.key" :heading="axis.key">
                <ComboboxItem
                  v-for="val in axis.values"
                  :key="`${axis.key}=${val}`"
                  :value="`${axis.key}=${val}`"
                  class="cursor-pointer text-xs font-mono"
                >
                  {{ axis.key }}={{ val }}
                  <ComboboxItemIndicator class="ml-auto">
                    <Icon name="lucide:check" class="h-3.5 w-3.5" />
                  </ComboboxItemIndicator>
                </ComboboxItem>
              </ComboboxGroup>
              <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
            </ComboboxViewport>
          </ComboboxList>
        </Combobox>
        <!-- Status filter -->
        <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
        <div class="flex flex-wrap gap-1">
          <button
            v-for="chip in statusChips"
            :key="chip.status"
            class="inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium transition-colors"
            :class="
              activeStatusFilter === chip.status
                ? 'bg-primary text-primary-foreground'
                : 'bg-muted text-muted-foreground hover:bg-accent'
            "
            @click="activeStatusFilter = activeStatusFilter === chip.status ? null : chip.status"
          >
            {{ chip.label }} ({{ chip.count }})
          </button>
        </div>
        <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
      </div>

      <!-- Variant rows -->
      <div class="border-t border-border mt-1">
        <template v-for="workflow in filteredWorkflows" :key="workflow.id">
          <!-- Variant Row -->
          <div
            class="flex items-center gap-1 h-7 px-1 transition-colors hover:bg-accent cursor-pointer border-b border-border last:border-b-0"
            :class="{ 'bg-accent/50': expandedVariantId === workflow.id }"
            @click="toggleVariant(workflow)"
          >
            <!-- Expand chevron -->
            <Icon
              name="chevron-right"
              class="h-3 w-3 shrink-0 text-muted-foreground transition-transform duration-150"
              :class="{ 'rotate-90 transform': expandedVariantId === workflow.id }"
            />
            <!-- Status Icon -->
            <PipelineStatusIcon :status="workflow.state" class="h-3.5 w-3.5 shrink-0 mr-0.5" />
            <!-- Environ values -->
            <span
              class="min-w-0 truncate text-xs text-muted-foreground font-mono"
              :title="formatEnviron(workflow.environ)"
            >
              {{ formatEnviron(workflow.environ) }}
            </span>
            <!-- Attempt dropdown -->
            <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
            <DropdownMenu v-if="workflowHasReruns(workflow)">
              <DropdownMenuTrigger as-child>
                <button
                  class="shrink-0 inline-flex items-center gap-0.5 rounded border border-primary/70 px-1 py-0.5 text-[10px] font-medium text-muted-foreground hover:bg-accent transition-colors"
                  @click.stop
                >
                  #{{ workflow.attempt }}
                  <Icon name="chevron-down" class="h-2.5 w-2.5" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start">
                <DropdownMenuItem
                  v-for="attempt in workflowAttemptsList(workflow)"
                  :key="attempt.id"
                  class="flex items-center gap-2 text-xs"
                  @click.stop="switchAttempt(attempt)"
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
            <TooltipProvider v-if="workflow.agent_id">
              <Tooltip>
                <TooltipTrigger as-child>
                  <router-link
                    :to="{ name: 'admin-agent-detail', params: { id: workflow.agent_id } }"
                    class="shrink-0 text-primary hover:text-primary/80 flex items-center gap-1"
                    @click.stop
                  >
                    <Icon name="agent" class="w-3.5 h-3.5" />
                    <span class="text-xs hidden lg:inline truncate max-w-[100px]">
                      {{ getAgentDisplay(workflow.agent_id) }}
                    </span>
                  </router-link>
                </TooltipTrigger>
                <TooltipContent>
                  <div class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                    <div class="col-span-2 font-medium mb-1">
                      {{ getAgent(workflow.agent_id)?.name ?? `#${workflow.agent_id}` }}
                    </div>
                    <template v-if="getAgent(workflow.agent_id)">
                      <span class="opacity-70">{{ $t('repo.pipeline.agent_tooltip.platform') }}</span>
                      <span>{{ getAgent(workflow.agent_id)!.platform }}</span>
                      <span class="opacity-70">{{ $t('repo.pipeline.agent_tooltip.backend') }}</span>
                      <span>{{ getAgent(workflow.agent_id)!.backend }}</span>
                      <span class="opacity-70">{{ $t('repo.pipeline.agent_tooltip.workflows') }}</span>
                      <span>{{
                        `${getAgent(workflow.agent_id)!.running_workflows} / ${getAgent(workflow.agent_id)!.capacity}`
                      }}</span>
                      <template v-if="getAgent(workflow.agent_id)!.backend === 'docker'">
                        <span class="opacity-70">{{ $t('repo.pipeline.agent_tooltip.limit_mem') }}</span>
                        <span>{{ formatBytes(getAgent(workflow.agent_id)!.limit_mem, $t) }}</span>
                        <span class="opacity-70">{{ $t('repo.pipeline.agent_tooltip.limit_cpu_quota') }}</span>
                        <span>{{ formatCPUQuota(getAgent(workflow.agent_id)!.limit_cpu_quota, $t) }}</span>
                      </template>
                    </template>
                  </div>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
            <!-- Rerun -->
            <IconButton
              v-if="hasPushPermission && workflow.state !== 'pending' && workflow.state !== 'running'"
              :is-loading="isRerunningWorkflow[workflow.id]"
              icon="solar:restart-bold"
              class="shrink-0 text-primary hover:text-primary/80 hover:bg-accent"
              :title="$t('repo.pipeline.actions.rerun_workflow')"
              @click.stop="$emit('rerunWorkflow', workflow.id)"
            />
            <!-- Cancel -->
            <IconButton
              v-if="hasPushPermission && (workflow.state === 'pending' || workflow.state === 'running')"
              :is-loading="isCancelingWorkflow[workflow.id]"
              icon="close"
              class="shrink-0 text-red-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20"
              :title="$t('repo.pipeline.actions.cancel')"
              size="1.1rem"
              @click.stop="$emit('cancelWorkflow', workflow.id)"
            />
            <!-- Duration -->
            <span class="shrink-0 text-xs text-right w-16 pr-1">
              <PipelineStepDuration
                v-if="workflow.started !== workflow.finished"
                :workflow="workflow"
                class="text-xs"
              />
            </span>
          </div>

          <!-- Canceled Message for this variant -->
          <div
            v-if="workflow.state === 'killed' && expandedVariantId === workflow.id"
            class="flex items-center gap-1 h-7 px-1"
          >
            <span class="shrink-0 w-3" />
            <span class="shrink-0 w-3.5" />
            <span class="italic text-xs text-destructive">
              {{ $t('repo.pipeline.actions.canceled_by_user') }}
            </span>
          </div>

          <!-- Steps for expanded variant -->
          <div
            class="overflow-hidden transition-height duration-150"
            :class="{ 'max-h-0': expandedVariantId !== workflow.id }"
          >
            <div
              v-for="node in getStepNodes(workflow)"
              :key="node.step.pid"
              class="flex items-center gap-1 h-7 px-1 transition-colors hover:bg-accent cursor-pointer border-b border-border last:border-b-0 bg-muted/30"
              :class="{ 'bg-accent!': node.step.pid === selectedStepId }"
              @click.stop="$emit('update:selectedStepId', node.step.pid)"
            >
              <!-- Base indentation spacer -->
              <span class="shrink-0 w-3" />
              <!-- Tree connectors for nested steps -->
              <template v-if="treeView && node.depth > 0">
                <span
                  v-for="d in node.depth"
                  :key="d"
                  class="step-tree-connector shrink-0 w-3 h-7 relative"
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
                class="h-3.5 w-3.5 shrink-0 mr-0.5"
              />
              <span class="min-w-0 truncate text-sm"
                >{{ node.step.name
                }}<span class="text-muted-foreground">{{ stepDupSuffix(workflow, node.step) }}</span></span
              >
              <!-- Multi-dependency tooltip -->
              <TooltipProvider v-if="node.effectiveDeps.length > 1">
                <Tooltip>
                  <TooltipTrigger as-child>
                    <span class="shrink-0 text-muted-foreground">
                      <Icon name="dependency" size="0.75rem" />
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
              <span class="flex-1" />
              <!-- Rerun step -->
              <IconButton
                v-if="hasPushPermission && isStepCompleted(node.step)"
                :is-loading="isRerunningStep[node.step.id]"
                icon="solar:restart-bold"
                class="shrink-0 text-primary hover:text-primary/80 hover:bg-accent"
                size="0.8rem"
                :title="$t('repo.pipeline.actions.rerun_step')"
                @click.stop="$emit('rerunStep', workflow.id, node.step.id)"
              />
              <span class="shrink-0 text-right w-16 pr-1">
                <PipelineStepDuration :step="node.step" class="text-xs" />
              </span>
            </div>
          </div>
        </template>

        <!-- No results message when filtering -->
        <div
          v-if="isLargeGroup && filteredWorkflows.length === 0"
          class="py-3 text-center text-xs text-muted-foreground"
        >
          {{ $t('repo.pipeline.matrix.no_matches') }}
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import Icon from '~/components/atomic/Icon.vue';
import IconButton from '~/components/atomic/IconButton.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import PipelineStepDuration from '~/components/repo/pipeline/PipelineStepDuration.vue';
import {
  Combobox,
  ComboboxAnchor,
  ComboboxEmpty,
  ComboboxGroup,
  ComboboxInput,
  ComboboxItem,
  ComboboxItemIndicator,
  ComboboxList,
  ComboboxViewport,
} from '~/components/ui/combobox';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';
import { requiredInject } from '~/compositions/useInjectProvide';
import { formatBytes, formatCPUQuota } from '~/lib/agent-utils';
import { StepType } from '~/lib/api/types';
import type { Agent, PipelineStatus, PipelineStep, PipelineWorkflow } from '~/lib/api/types';
import type { StepTreeNode } from '~/lib/pipeline-utils';
import {
  aggregateWorkflowDuration,
  aggregateWorkflowStatus,
  buildStepTree,
  duplicateStepNameIndexes,
  getWorkflowAttempts,
  hasReruns,
} from '~/lib/pipeline-utils';
import { computed, ref, watch } from 'vue';

const props = defineProps<{
  workflows: PipelineWorkflow[];
  allWorkflows: PipelineWorkflow[];
  groupName: string;
  selectedStepId?: number | null;
  agentDetails: Record<number, Agent>;
  isCancelingWorkflow: Record<number, boolean>;
  isRerunningWorkflow: Record<number, boolean>;
  isRerunningStep: Record<number, boolean>;
  treeView: boolean;
}>();

const emit = defineEmits<{
  (event: 'update:selectedStepId', stepId: number): void;
  (event: 'cancelWorkflow', workflowId: number): void;
  (event: 'rerunWorkflow', workflowId: number): void;
  (event: 'rerunStep', workflowId: number, stepId: number): void;
}>();

const LARGE_GROUP_THRESHOLD = 8;

const STATUS_SORT_ORDER: Record<PipelineStatus, number> = {
  error: 0,
  failure: 1,
  running: 2,
  started: 3,
  pending: 4,
  killed: 5,
  blocked: 6,
  declined: 7,
  success: 8,
  skipped: 9,
};

const repoPermissions = requiredInject('repo-permissions');
const hasPushPermission = computed(() => repoPermissions.value && repoPermissions.value.push);

const isLargeGroup = computed(() => props.workflows.length > LARGE_GROUP_THRESHOLD);

// Group expansion state
const groupExpanded = ref(true);
const userToggledGroup = ref(false);
const expandedVariantId = ref<number | null>(null);

// Filter state (large groups only)
const selectedAxisValues = ref<string[]>([]);
const searchTerm = ref('');
const activeStatusFilter = ref<string | null>(null);

// Aggregated values
const aggregatedStatus = computed(() => aggregateWorkflowStatus(props.workflows));
const aggregatedDuration = computed(() => aggregateWorkflowDuration(props.workflows));

// Extract matrix axis keys and their unique values (preserving insertion order)
const matrixAxes = computed(() => {
  const axisMap = new Map<string, Set<string>>();
  for (const w of props.workflows) {
    if (!w.environ) continue;
    for (const [key, value] of Object.entries(w.environ)) {
      if (!axisMap.has(key)) {
        axisMap.set(key, new Set());
      }
      axisMap.get(key)!.add(value);
    }
  }
  return Array.from(axisMap.entries(), ([key, values]) => ({
    key,
    values: [...values],
  }));
});

// Parse selected combobox values into a map of key -> Set<value> for filtering
const parsedAxisFilters = computed(() => {
  const map = new Map<string, Set<string>>();
  for (const entry of selectedAxisValues.value) {
    const eqIdx = entry.indexOf('=');
    if (eqIdx === -1) continue;
    const key = entry.substring(0, eqIdx);
    const value = entry.substring(eqIdx + 1);
    if (!map.has(key)) {
      map.set(key, new Set());
    }
    map.get(key)!.add(value);
  }
  return map;
});

function filterComboboxItems(items: string[], term: string): string[] {
  if (!term) return items;
  const lower = term.toLowerCase();
  return items.filter((item) => item.toLowerCase().includes(lower));
}

function removeSelectedValue(tag: string) {
  selectedAxisValues.value = selectedAxisValues.value.filter((v) => v !== tag);
}

// Status chips for large groups
const statusChips = computed(() => {
  const counts = new Map<string, number>();
  for (const w of props.workflows) {
    counts.set(w.state, (counts.get(w.state) || 0) + 1);
  }

  const chips: { status: string; label: string; count: number }[] = [];
  const labelMap: Record<string, string> = {
    error: 'Error',
    failure: 'Failed',
    running: 'Running',
    pending: 'Pending',
    success: 'Passed',
    killed: 'Canceled',
    skipped: 'Skipped',
    blocked: 'Blocked',
    started: 'Started',
    declined: 'Declined',
  };

  // Show "All" chip first
  chips.push({ status: 'all', label: 'All', count: props.workflows.length });

  for (const [status, count] of counts) {
    if (count > 0) {
      chips.push({ status, label: labelMap[status] || status, count });
    }
  }

  return chips;
});

// Sorted + filtered workflows
const filteredWorkflows = computed(() => {
  let result = [...props.workflows];

  // Sort by status priority (worst first), then by duration descending
  result.sort((a, b) => {
    const statusDiff = (STATUS_SORT_ORDER[a.state] ?? 99) - (STATUS_SORT_ORDER[b.state] ?? 99);
    if (statusDiff !== 0) return statusDiff;

    const durationA = (a.finished ?? 0) - (a.started ?? 0);
    const durationB = (b.finished ?? 0) - (b.started ?? 0);
    return durationB - durationA;
  });

  if (!isLargeGroup.value) return result;

  // Apply status filter
  if (activeStatusFilter.value && activeStatusFilter.value !== 'all') {
    result = result.filter((w) => w.state === activeStatusFilter.value);
  }

  // Apply axis filters: AND across keys, OR within same key
  if (parsedAxisFilters.value.size > 0) {
    result = result.filter((w) => {
      if (!w.environ) return false;
      for (const [key, allowedValues] of parsedAxisFilters.value) {
        if (!allowedValues.has(w.environ![key])) return false;
      }
      return true;
    });
  }

  // Apply free-text search as additional substring filter
  if (searchTerm.value) {
    const query = searchTerm.value.toLowerCase();
    result = result.filter((w) => {
      if (!w.environ) return false;
      return Object.entries(w.environ).some(
        ([key, value]) =>
          key.toLowerCase().includes(query) ||
          value.toLowerCase().includes(query) ||
          `${key}=${value}`.toLowerCase().includes(query),
      );
    });
  }

  return result;
});

// Initialize collapse state (only auto-collapse before user has interacted)
watch(
  () => props.workflows,
  (workflows) => {
    if (!userToggledGroup.value) {
      const allDone = workflows.every((w) => ['success', 'skipped', 'blocked'].includes(w.state));
      const containsSelected = workflows.some((w) => w.children?.some((s) => s.pid === props.selectedStepId));

      if (allDone && !containsSelected) {
        groupExpanded.value = false;
      }
    }

    // Default status filter to 'failure' if any failures exist (large groups only)
    if (isLargeGroup.value && workflows.some((w) => w.state === 'failure' || w.state === 'error')) {
      activeStatusFilter.value = workflows.some((w) => w.state === 'failure') ? 'failure' : 'error';
    }
  },
  { immediate: true },
);

// Auto-expand group if selected step is inside it
watch(
  () => props.selectedStepId,
  (stepId) => {
    if (stepId === null || stepId === undefined) return;

    for (const workflow of props.workflows) {
      if (workflow.children?.some((s) => s.pid === stepId)) {
        groupExpanded.value = true;
        expandedVariantId.value = workflow.id;
        return;
      }
    }
  },
  { immediate: true },
);

function toggleVariant(workflow: PipelineWorkflow) {
  if (expandedVariantId.value === workflow.id) {
    expandedVariantId.value = null;
  } else {
    expandedVariantId.value = workflow.id;
  }
}

function formatEnviron(environ?: Record<string, string>): string {
  if (!environ) return '';
  return Object.entries(environ)
    .map(([k, v]) => `${k}=${v}`)
    .join(' ');
}

function workflowHasReruns(workflow: PipelineWorkflow): boolean {
  return hasReruns(props.allWorkflows, workflow);
}

function workflowAttemptsList(workflow: PipelineWorkflow): PipelineWorkflow[] {
  return getWorkflowAttempts(props.allWorkflows, workflow);
}

function switchAttempt(targetAttempt: PipelineWorkflow) {
  // Select the first step of the target attempt's workflow
  if (targetAttempt.children && targetAttempt.children.length > 0) {
    emit('update:selectedStepId', targetAttempt.children[0].pid);
  }
}

function getAgentDisplay(agentId: number): string {
  return props.agentDetails[agentId]?.name || '';
}

function getAgent(agentId: number): Agent | undefined {
  return props.agentDetails[agentId];
}

function isStepCompleted(step: PipelineStep) {
  return step.state !== 'pending' && step.state !== 'running';
}

const stepNameDupIndexes = computed(() => {
  const map = new Map<number, Map<number, number>>();
  for (const wf of props.workflows) {
    map.set(wf.id, duplicateStepNameIndexes(wf.children));
  }
  return map;
});

function stepDupSuffix(workflow: PipelineWorkflow, step: PipelineStep): string {
  const index = stepNameDupIndexes.value.get(workflow.id)?.get(step.pid);
  return index ? ` (${index})` : '';
}

function getStepNodes(workflow: PipelineWorkflow): StepTreeNode[] {
  if (props.treeView) {
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
</script>

<style scoped>
:deep([data-slot='command-input-wrapper']) {
  border-bottom: none;
  height: auto;
  padding: 0;
}

:deep([data-slot='command-input-wrapper'] > svg) {
  display: none;
}

.step-tree-connector {
  --connector-color: var(--color-border);
}

.connector-vertical::before {
  content: '';
  position: absolute;
  left: 50%;
  top: 0;
  bottom: 0;
  width: 1px;
  background: var(--connector-color);
}

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
