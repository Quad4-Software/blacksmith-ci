<template>
  <div class="rounded-md border border-border bg-card p-2 shadow-xs">
    <!-- Group Header -->
    <button
      class="flex items-center gap-2 mb-1 w-full text-left hover:opacity-80 transition-opacity"
      @click="groupExpanded = !groupExpanded"
    >
      <Icon
        name="chevron-right"
        class="size-4 transition-transform duration-200 text-muted-foreground"
        :class="{ 'rotate-90': groupExpanded }"
      />
      <div class="flex items-center gap-3">
        <PipelineStatusIcon :status="aggregatedStatus" class="h-4 w-4" />
        <span class="font-medium text-foreground">{{ groupName }}</span>
      </div>
    </button>

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
      </div>

      <div :class="{ 'max-h-[50vh] overflow-y-auto': isLargeGroup }">
        <div v-for="workflow in filteredWorkflows" :key="workflow.id" class="border-t border-border">
          <!-- Variant row -->
          <button
            class="flex items-center gap-2 py-1.5 px-1 w-full text-left hover:opacity-80 transition-opacity"
            @click="toggleVariant(workflow.id)"
          >
            <Icon
              name="chevron-right"
              class="size-3.5 transition-transform duration-200 text-muted-foreground shrink-0"
              :class="{ 'rotate-90': expandedVariantId === workflow.id }"
            />
            <PipelineStatusIcon :status="workflow.state" class="h-3.5 w-3.5 shrink-0" />
            <span
              class="flex-1 text-xs text-muted-foreground font-mono truncate"
              :title="formatEnviron(workflow.environ)"
            >
              {{ formatEnviron(workflow.environ) }}
            </span>
            <PipelineStepDuration
              v-if="workflow.started !== workflow.finished"
              :workflow="workflow"
              class="text-xs text-muted-foreground shrink-0"
            />
          </button>

          <!-- Steps for expanded variant -->
          <div v-show="expandedVariantId === workflow.id" class="space-y-1 pb-1">
            <div
              v-for="step in workflow.children"
              :key="step.pid"
              class="flex items-center gap-3 p-2 rounded transition-colors hover:bg-accent cursor-pointer ml-6"
              @click="$emit('selectStep', step.pid)"
            >
              <PipelineStatusIcon :service="step.type === 'service'" :status="step.state" class="h-4 w-4" />
              <span class="flex-1 text-sm text-foreground">{{ step.name }}</span>
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
import type { PipelineWorkflow } from '~/lib/api/types';
import { aggregateWorkflowStatus } from '~/lib/pipeline-utils';

const props = defineProps<{
  workflows: PipelineWorkflow[];
  groupName: string;
}>();

defineEmits<{
  (event: 'selectStep', stepId: number): void;
}>();

const LARGE_GROUP_THRESHOLD = 8;

const isLargeGroup = computed(() => props.workflows.length > LARGE_GROUP_THRESHOLD);
const aggregatedStatus = computed(() => aggregateWorkflowStatus(props.workflows));

const groupExpanded = ref(true);
const expandedVariantId = ref<number | null>(null);
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
  return Array.from(axisMap.entries()).map(([key, values]) => ({
    key,
    values: Array.from(values),
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

function formatEnviron(environ?: Record<string, string>): string {
  if (!environ) return '';
  return Object.entries(environ)
    .map(([k, v]) => `${k}=${v}`)
    .join(' ');
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
