<template>
  <div class="rounded-md border border-border bg-card p-2 shadow-xs">
    <!-- Group Header -->
    <div class="flex items-center gap-2 mb-1 w-full">
      <button
        class="flex items-center gap-2 flex-1 min-w-0 text-left hover:opacity-80 transition-opacity"
        @click="groupExpanded = !groupExpanded"
      >
        <Icon
          name="chevron-right"
          class="size-4 transition-transform duration-200 text-muted-foreground shrink-0"
          :class="{ 'rotate-90': groupExpanded }"
        />
        <PipelineStatusIcon :status="aggregatedStatus" class="h-4 w-4 shrink-0" />
        <span class="font-medium text-foreground truncate">{{ groupName }}</span>
      </button>
    </div>

    <!-- Variant List -->
    <div v-show="groupExpanded" class="mt-1">
      <!-- Filter combobox for large groups -->
      <div v-if="isLargeGroup" class="mb-2">
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
              class="flex-1 min-w-[4rem] bg-transparent text-base md:text-xs outline-hidden placeholder:text-muted-foreground"
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
                    <Icon name="ph:check" class="h-3.5 w-3.5" />
                  </ComboboxItemIndicator>
                </ComboboxItem>
              </ComboboxGroup>
              <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
            </ComboboxViewport>
          </ComboboxList>
        </Combobox>
      </div>

      <!-- Shared axes (constant across every variant) hoisted out of the rows -->
      <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
      <div v-if="classification.shared.length > 0" class="flex flex-wrap items-center gap-1 pb-2 pl-6">
        <span class="text-[10px] uppercase tracking-wide text-muted-foreground/70 mr-0.5">{{
          $t('repo.pipeline.matrix.shared')
        }}</span>
        <span
          v-for="axis in classification.shared"
          :key="axis.key"
          class="inline-flex items-center rounded-none border border-border bg-muted/50 px-1 py-px font-mono text-[11px] leading-tight text-muted-foreground"
        >
          <span class="text-muted-foreground/70">{{ axis.key }}=</span>
          <span :style="{ color: matrixValueColor(axis.value) }">{{ axis.value }}</span>
        </span>
      </div>
      <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->

      <div :class="{ 'max-h-[50vh] overflow-y-auto': isLargeGroup }">
        <div v-for="workflow in filteredWorkflows" :key="workflow.id" class="border-t border-border">
          <!-- Variant row (dynamic axis flow) -->
          <div class="flex items-start gap-2 py-1.5 px-1 w-full cursor-pointer" @click="toggleVariant(workflow.id)">
            <Icon
              name="chevron-right"
              class="size-3.5 mt-0.5 transition-transform duration-200 text-muted-foreground shrink-0"
              :class="{ 'rotate-90': expandedVariantId === workflow.id }"
            />
            <PipelineStatusIcon :status="workflow.state" class="h-3.5 w-3.5 mt-0.5 shrink-0" />
            <!-- Axis chips (flow: all axes, primary first, +N overflow) -->
            <MatrixVariantChips v-if="axisKeys.length > 0" :chips="axisChipsFor(workflow)" />
            <span v-else class="min-w-0 flex-1 mt-0.5 truncate font-mono text-xs text-muted-foreground">{{
              workflow.name
            }}</span>
            <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
            <DropdownMenu v-if="variantHasReruns(workflow)">
              <DropdownMenuTrigger as-child>
                <button
                  class="shrink-0 mt-0.5 inline-flex items-center gap-0.5 rounded border border-primary/70 px-1 py-0.5 text-[10px] font-medium text-muted-foreground hover:bg-accent transition-colors"
                  @click.stop
                >
                  #{{ workflow.attempt }}
                  <Icon name="chevron-down" class="h-2.5 w-2.5" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start">
                <DropdownMenuItem
                  v-for="attempt in variantAttempts(workflow)"
                  :key="attempt.id"
                  class="flex items-center gap-2 text-xs"
                  @click.stop="switchVariantAttempt(attempt)"
                >
                  <PipelineStatusIcon :status="attempt.state" class="h-3.5 w-3.5" />
                  #{{ attempt.attempt }}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
            <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
            <IconButton
              v-if="hasPushPermission && isVariantCompleted(workflow)"
              icon="solar:restart-bold"
              class="shrink-0 mt-0.5 text-primary hover:text-primary/80 hover:bg-accent"
              :title="$t('repo.pipeline.actions.rerun_workflow')"
              @click.stop="$emit('rerunWorkflow', workflow.id)"
            />
            <PipelineStepDuration
              v-if="workflow.started !== workflow.finished"
              :workflow="workflow"
              class="text-xs text-muted-foreground shrink-0 mt-0.5"
            />
          </div>

          <!-- Steps for expanded variant -->
          <div v-show="expandedVariantId === workflow.id" class="space-y-1 pb-1">
            <div
              v-for="step in workflow.children"
              :key="step.pid"
              class="flex items-center gap-3 p-2 rounded transition-colors hover:bg-accent cursor-pointer ml-6"
              @click="$emit('selectStep', step.pid)"
            >
              <PipelineStatusIcon :service="step.type === 'service'" :status="step.state" class="h-4 w-4" />
              <span class="flex-1 inline-flex items-center gap-1 min-w-0 text-sm text-foreground">
                <span class="truncate">{{ step.name }}</span>
                <PipelineCacheStepMarker v-if="step.type === 'cache'" />
              </span>
              <PipelineStepDuration :step="step" class="text-xs text-muted-foreground" />
            </div>
          </div>
        </div>

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
import { computed, ref, watch } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
import IconButton from '~/components/atomic/IconButton.vue';
import MatrixVariantChips from '~/components/repo/pipeline/MatrixVariantChips.vue';
import type { MatrixChip } from '~/components/repo/pipeline/MatrixVariantChips.vue';
import PipelineCacheStepMarker from '~/components/repo/pipeline/PipelineCacheStepMarker.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import PipelineStepDuration from '~/components/repo/pipeline/PipelineStepDuration.vue';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
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
import type { PipelineWorkflow } from '~/lib/api/types';
import {
  aggregateWorkflowStatus,
  classifyMatrixAxes,
  getWorkflowAttempts,
  hasReruns,
  matrixValueColor,
} from '~/lib/pipeline-utils';

const props = defineProps<{
  workflows: PipelineWorkflow[];
  allWorkflows: PipelineWorkflow[];
  groupName: string;
  hasPushPermission?: boolean;
}>();

const emit = defineEmits<{
  (event: 'selectStep', stepId: number): void;
  (event: 'rerunWorkflow', workflowId: number): void;
}>();

const LARGE_GROUP_THRESHOLD = 8;

const isLargeGroup = computed(() => props.workflows.length > LARGE_GROUP_THRESHOLD);
const aggregatedStatus = computed(() => aggregateWorkflowStatus(props.workflows));

const groupExpanded = ref(true);
const expandedVariantId = ref<number | null>(null);

function variantHasReruns(workflow: PipelineWorkflow) {
  return hasReruns(props.allWorkflows, workflow);
}

function variantAttempts(workflow: PipelineWorkflow) {
  return getWorkflowAttempts(props.allWorkflows, workflow);
}

function isVariantCompleted(workflow: PipelineWorkflow) {
  return workflow.state !== 'pending' && workflow.state !== 'running';
}

function switchVariantAttempt(targetAttempt: PipelineWorkflow) {
  expandedVariantId.value = targetAttempt.id;
  if (targetAttempt.children.length > 0) {
    emit('selectStep', targetAttempt.children[0].pid);
  }
}

const selectedAxisValues = ref<string[]>([]);
const searchTerm = ref('');

// Extract matrix axis keys and their unique values
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

// Parse selected combobox values into a map of key -> Set<value>
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

const filteredWorkflows = computed(() => {
  let result = [...props.workflows];

  if (!isLargeGroup.value) return result;

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

watch(
  () => props.workflows,
  (workflows) => {
    const allDone = workflows.every((w) => ['success', 'skipped', 'blocked'].includes(w.state));
    if (allDone) {
      groupExpanded.value = false;
    }
  },
  { immediate: true },
);

function toggleVariant(workflowId: number) {
  expandedVariantId.value = expandedVariantId.value === workflowId ? null : workflowId;
}

// Classify the group's matrix axes: shared axes are hoisted to the header; the
// rest (primary first) flow as chips on each variant row.
const classification = computed(() => classifyMatrixAxes(props.workflows));
const axisKeys = computed(() => {
  const c = classification.value;
  return [c.primary, ...c.secondary].filter((k): k is string => k !== undefined);
});

function axisChipsFor(workflow: PipelineWorkflow): MatrixChip[] {
  return axisKeys.value.map((key) => ({
    key,
    value: workflow.environ?.[key],
    present: !!workflow.environ && key in workflow.environ,
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
</style>
