<template>
  <div v-if="pipelines || infiniteScroll" class="space-y-4 max-w-6xl mx-auto px-3 py-4 sm:px-4 overflow-x-hidden">
    <!-- Bulk actions overlay -->
    <Transition
      enter-active-class="transition-all duration-300 ease-out"
      enter-from-class="opacity-0 -translate-y-2"
      enter-to-class="opacity-100 translate-y-0"
      leave-active-class="transition-all duration-200 ease-in"
      leave-from-class="opacity-100 translate-y-0"
      leave-to-class="opacity-0 -translate-y-2"
    >
      <div
        v-if="selectedPipelines.length > 0 && repoPermissions?.push"
        key="bulk-actions-toolbar"
        class="sticky top-4 z-20 mb-2 backdrop-blur-md px-2"
      >
        <Card class="max-w-2xl mx-auto p-0">
          <CardContent class="p-3 relative">
            <!-- Close button in top-right corner -->
            <ShadcnButton
              variant="ghost"
              size="icon-sm"
              class="absolute top-2 right-2"
              :title="$t('repo.pipeline.clear_selection')"
              @click="clearSelection"
            >
              <Icon name="close" class="h-4 w-4" />
            </ShadcnButton>

            <div class="flex flex-col sm:flex-row items-center justify-between gap-3 pr-8">
              <div class="flex items-center w-full sm:w-auto">
                <span class="text-xs sm:text-sm font-medium whitespace-nowrap">
                  {{ $t('repo.pipeline.selected_count', { count: selectedPipelines.length }) }}
                </span>
              </div>
              <div class="flex gap-2 sm:gap-3 justify-center sm:justify-end">
                <ShadcnButton
                  v-if="canCancelSelected"
                  size="sm"
                  variant="outline"
                  :disabled="isCancelingBulk"
                  @click="cancelSelectedPipelines"
                >
                  <Icon
                    v-if="isCancelingBulk"
                    name="spinner"
                    class="mr-1 sm:mr-1.5 h-3 w-3 sm:h-4 sm:w-4 animate-spin"
                  />
                  <Icon v-else name="close" class="mr-1 sm:mr-1.5 h-3 w-3 sm:h-4 sm:w-4" />
                  <span class="hidden sm:inline">{{ $t('repo.pipeline.actions.cancel_selected') }}</span>
                  <span class="sm:hidden">{{ $t('repo.pipeline.actions.cancel') || 'Cancel' }}</span>
                </ShadcnButton>
                <ShadcnButton size="sm" :disabled="isRestartingBulk" @click="restartSelectedPipelines">
                  <Icon
                    v-if="isRestartingBulk"
                    name="spinner"
                    class="mr-1 sm:mr-1.5 h-3 w-3 sm:h-4 sm:w-4 animate-spin"
                  />
                  <Icon v-else name="play-outline" class="mr-1 sm:mr-1.5 h-3 w-3 sm:h-4 sm:w-4" />
                  <span class="hidden sm:inline">{{ $t('repo.pipeline.actions.restart_selected') }}</span>
                  <span class="sm:hidden">{{ $t('repo.pipeline.actions.restart') || 'Restart' }}</span>
                </ShadcnButton>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </Transition>

    <!-- Search input -->
    <SearchInput v-if="searchQuery !== null" v-model="searchQuery" :placeholder="t('search')" class="w-full" />

    <!-- Hidden pipelines toggle and multi-select header -->
    <div class="flex flex-wrap items-center justify-between gap-2 pb-2">
      <div
        v-if="isMultiselectMode && displayPipelines.length > 0 && repoPermissions?.push"
        class="flex items-center gap-2"
      >
        <div class="w-7 shrink-0 flex items-center justify-center">
          <Checkbox
            id="select-all"
            :model-value="isAllSelected ? true : isPartiallySelected ? 'indeterminate' : false"
            @update:model-value="toggleSelectAll"
          />
        </div>
        <Label
          for="select-all"
          class="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70 cursor-pointer"
        >
          {{ $t('repo.pipeline.select_all') }}
        </Label>
      </div>
      <div v-else />

      <!-- Hidden pipelines toggle -->
      <div v-if="hiddenCount > 0" class="flex flex-wrap items-center gap-1 sm:gap-2">
        <ShadcnButton
          variant="ghost"
          size="sm"
          class="h-8 text-muted-foreground text-xs sm:text-sm"
          @click="showHiddenPipelines = !showHiddenPipelines"
        >
          <Icon :name="showHiddenPipelines ? 'lucide:eye' : 'lucide:eye-off'" class="mr-1 sm:mr-1.5 h-4 w-4 shrink-0" />
          <span class="truncate">
            {{
              showHiddenPipelines
                ? $t('repo.pipeline.hide_hidden')
                : $t('repo.pipeline.show_hidden', { count: hiddenCount })
            }}
          </span>
        </ShadcnButton>
        <ShadcnButton
          v-if="showHiddenPipelines"
          variant="ghost"
          size="sm"
          class="h-8 text-muted-foreground text-xs sm:text-sm"
          @click="clearHiddenPipelines"
        >
          <Icon name="lucide:x" class="mr-1 sm:mr-1.5 h-4 w-4 shrink-0" />
          <span class="truncate">{{ $t('repo.pipeline.unhide_all') }}</span>
        </ShadcnButton>
      </div>
    </div>

    <!-- Pipeline list with hidden indicators -->
    <template
      v-for="item in pipelineDisplayItems"
      :key="item.type === 'pipeline' ? item.pipeline.id : `hidden-${item.pipelineIds[0]}`"
    >
      <PipelineItem
        v-if="item.type === 'pipeline'"
        :to="{
          name: 'repo-pipeline',
          params: { pipelineId: item.pipeline.number },
        }"
        :pipeline="item.pipeline"
        :is-selected="selectedPipelineIds.has(item.pipeline.id)"
        :is-hidden="hiddenPipelineIds.includes(item.pipeline.id)"
        :show-checkbox="isMultiselectMode && repoPermissions?.push"
        :class="{ 'opacity-50': showHiddenPipelines && hiddenPipelineIds.includes(item.pipeline.id) }"
        @toggle-select="togglePipelineSelection(item.pipeline)"
        @hide="hidePipeline"
        @unhide="unhidePipeline"
      />
      <!-- Hidden indicator (simplified on mobile) -->
      <div
        v-else
        class="flex items-center justify-center gap-2 py-2 text-sm text-muted-foreground cursor-pointer hover:text-foreground transition-colors"
        @click="showHiddenPipelines = true"
      >
        <div class="hidden sm:block flex-1 border-t border-dashed border-muted-foreground/30" />
        <Icon name="lucide:eye-off" class="h-4 w-4 shrink-0" />
        <span class="whitespace-nowrap">{{ $t('repo.pipeline.hidden_count', { count: item.count }) }}</span>
        <div class="hidden sm:block flex-1 border-t border-dashed border-muted-foreground/30" />
      </div>
    </template>

    <!-- Loading indicator for infinite scroll -->
    <div v-if="infiniteScroll && loading" class="flex justify-center items-center py-8">
      <div class="w-8 h-8">
        <PipelineProgressSpinner />
      </div>
      <span class="ml-3 text-foreground">{{ $t('repo.pipeline.loading') }}</span>
    </div>

    <!-- Scroll trigger element -->
    <div v-if="infiniteScroll && hasMore" ref="scrollTrigger" class="h-4" />

    <Panel v-if="visiblePipelines.length === 0 && hiddenCount === 0 && !loading">
      <span class="text-foreground">{{ $t('repo.pipeline.no_pipelines') }}</span>
    </Panel>
  </div>
</template>

<script lang="ts" setup>
import { useStorage } from '@vueuse/core';
import type { Ref } from 'vue';
import { computed, inject, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import Checkbox from '~/components/ui/checkbox/Checkbox.vue';
import Panel from '~/components/layout/Panel.vue';
import SearchInput from '~/components/ui/input/SearchInput.vue';
import PipelineItem from '~/components/repo/pipeline/PipelineItem.vue';
import PipelineProgressSpinner from '~/components/repo/pipeline/PipelineProgressSpinner.vue';
import { Button as ShadcnButton } from '~/components/ui/button';
import { Card, CardContent } from '~/components/ui/card';
import { Label } from '~/components/ui/label';
import { usePagination } from '~/compositions/usePaginate';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useApiClient from '~/compositions/useApiClient';
import useNotifications from '~/compositions/useNotifications';
import { requiredInject } from '~/compositions/useInjectProvide';
import type { Pipeline, Repo } from '~/lib/api/types';
import { usePipelineStore } from '~/store/pipelines';

const props = defineProps<{
  pipelines?: Pipeline[] | undefined;
  repo?: Repo;
  infiniteScroll?: boolean;
}>();

const { t } = useI18n();
const apiClient = useApiClient();
const pipelineStore = usePipelineStore();
const notifications = useNotifications();
const scrollTrigger = ref<HTMLElement | null>(null);
// Try to get repo permissions, may not be available in all contexts
let repoPermissions;
try {
  repoPermissions = requiredInject('repo-permissions');
} catch {
  repoPermissions = null;
}

// Try to get multiselect mode from parent
const injectedMultiselectMode = inject<Ref<boolean> | null>('multiselect-mode', null);

// Try to get search and date filters
const searchQuery = inject<Ref<string> | null>('search-query', null);
const dateFrom = inject<Ref<string> | null>('date-from', null);
const dateTo = inject<Ref<string> | null>('date-to', null);
const selectedEvents = inject<Ref<string[]> | null>('selected-events', null);
const branchFilter = inject<Ref<string> | null>('branch-filter', null);

// Hidden pipelines state - stored per repo
const storageKey = computed(() =>
  props.repo ? `crow:hidden-pipelines:${props.repo.id}` : 'crow:hidden-pipelines:global',
);
const hiddenPipelineIds = useStorage<number[]>(storageKey.value, []);
const showHiddenPipelines = ref(false);

// Update storage key when repo changes
watch(
  () => props.repo?.id,
  () => {
    if (props.repo) {
      const newKey = `crow:hidden-pipelines:${props.repo.id}`;
      const stored = localStorage.getItem(newKey);
      hiddenPipelineIds.value = stored ? JSON.parse(stored) : [];
    }
  },
);

function hidePipeline(pipelineId: number) {
  if (!hiddenPipelineIds.value.includes(pipelineId)) {
    hiddenPipelineIds.value = [...hiddenPipelineIds.value, pipelineId];
  }
}

function unhidePipeline(pipelineId: number) {
  hiddenPipelineIds.value = hiddenPipelineIds.value.filter((id) => id !== pipelineId);
}

function clearHiddenPipelines() {
  hiddenPipelineIds.value = [];
}

// Use infinite scroll if enabled and repo is provided
const infiniteScrollData =
  props.infiniteScroll && props.repo
    ? usePagination<Pipeline>(
        async (page) => {
          try {
            const pipelines = await apiClient.getPipelineList(props.repo!.id, { page, perPage: 25 });
            // Store pipelines in the store for consistency
            pipelines.forEach((pipeline) => {
              pipelineStore.setPipeline(props.repo!.id, pipeline);
            });
            return pipelines;
          } catch (error) {
            notifications.notify({ title: 'Failed to load pipelines', type: 'error' });
            console.error('Error loading pipelines:', error);
            return [];
          }
        },
        () => true,
        { scrollElement: scrollTrigger, pageSize: 25 },
      )
    : null;

const { data, loading, hasMore } = infiniteScrollData || { data: ref([]), loading: ref(false), hasMore: ref(false) };

// Display either provided pipelines or infinite scroll data
const allPipelines = computed(() => {
  if (props.infiniteScroll && infiniteScrollData) {
    return data.value;
  }
  return props.pipelines || [];
});

// Apply search and date filters
const displayPipelines = computed(() => {
  let filtered = allPipelines.value;

  // Apply search filter
  if (searchQuery?.value) {
    const query = searchQuery.value.toLowerCase();
    filtered = filtered.filter((pipeline) => {
      return (
        pipeline.number?.toString().includes(query) ||
        pipeline.message?.toLowerCase().includes(query) ||
        pipeline.branch?.toLowerCase().includes(query) ||
        pipeline.author?.toLowerCase().includes(query) ||
        pipeline.event?.toLowerCase().includes(query)
      );
    });
  }

  // Apply event filter (multi-select)
  if (selectedEvents?.value && selectedEvents.value.length > 0) {
    filtered = filtered.filter((pipeline) => pipeline.event && selectedEvents.value.includes(pipeline.event));
  }

  // Apply branch filter
  if (branchFilter?.value) {
    filtered = filtered.filter((pipeline) => {
      // Match exact branch or ref
      return pipeline.branch === branchFilter.value || pipeline.ref === branchFilter.value;
    });
  }

  // Apply date range filter (only if values are set)
  const today = new Date().toISOString().split('T')[0];
  const hasDateFrom = dateFrom?.value && dateFrom.value.trim() !== '';
  const hasDateTo = dateTo?.value && dateTo.value.trim() !== '' && dateTo.value !== today;

  if (hasDateFrom || hasDateTo) {
    filtered = filtered.filter((pipeline) => {
      const pipelineDate = new Date(pipeline.created * 1000); // Convert Unix timestamp to Date
      const fromDate = hasDateFrom ? new Date(dateFrom.value) : null;
      const toDate = hasDateTo ? new Date(`${dateTo.value}T23:59:59`) : null; // End of day

      if (fromDate && pipelineDate < fromDate) return false;
      if (toDate && pipelineDate > toDate) return false;
      return true;
    });
  }

  return filtered;
});

// Count of hidden pipelines in current filtered view
const hiddenCount = computed(() => {
  return displayPipelines.value.filter((p) => hiddenPipelineIds.value.includes(p.id)).length;
});

// Pipeline display items with hidden indicators
type PipelineDisplayItem =
  | { type: 'pipeline'; pipeline: Pipeline }
  | { type: 'hidden-indicator'; count: number; pipelineIds: number[] };

const pipelineDisplayItems = computed<PipelineDisplayItem[]>(() => {
  if (showHiddenPipelines.value) {
    // Show all pipelines when toggle is on
    return displayPipelines.value.map((p) => ({ type: 'pipeline' as const, pipeline: p }));
  }

  const items: PipelineDisplayItem[] = [];
  let hiddenGroup: number[] = [];

  for (const pipeline of displayPipelines.value) {
    if (hiddenPipelineIds.value.includes(pipeline.id)) {
      hiddenGroup.push(pipeline.id);
    } else {
      // Flush any accumulated hidden pipelines as an indicator
      if (hiddenGroup.length > 0) {
        items.push({ type: 'hidden-indicator', count: hiddenGroup.length, pipelineIds: [...hiddenGroup] });
        hiddenGroup = [];
      }
      items.push({ type: 'pipeline', pipeline });
    }
  }

  // Don't forget trailing hidden pipelines
  if (hiddenGroup.length > 0) {
    items.push({ type: 'hidden-indicator', count: hiddenGroup.length, pipelineIds: [...hiddenGroup] });
  }

  return items;
});

const visiblePipelines = computed(() =>
  pipelineDisplayItems.value.filter((item) => item.type === 'pipeline').map((item) => (item as any).pipeline),
);

// Multi-select state - use injected or local
const localMultiselectMode = ref(false);
const isMultiselectMode = injectedMultiselectMode || localMultiselectMode;
const selectedPipelineIds = ref<Set<number>>(new Set());
const selectedPipelines = computed(() => displayPipelines.value.filter((p) => selectedPipelineIds.value.has(p.id)));

// Multi-select computed properties
const isAllSelected = computed(
  () => displayPipelines.value.length > 0 && selectedPipelineIds.value.size === displayPipelines.value.length,
);

const isPartiallySelected = computed(
  () => selectedPipelineIds.value.size > 0 && selectedPipelineIds.value.size < displayPipelines.value.length,
);

const canCancelSelected = computed(() =>
  selectedPipelines.value.some((p) => p.status === 'pending' || p.status === 'running'),
);

// Multi-select functions
// Watch for multiselect mode changes to clear selection when exiting
if (injectedMultiselectMode) {
  watch(injectedMultiselectMode, (newValue) => {
    if (!newValue) {
      // Clear selection when exiting multiselect mode
      selectedPipelineIds.value = new Set();
    }
  });
}

function togglePipelineSelection(pipeline: Pipeline) {
  if (selectedPipelineIds.value.has(pipeline.id)) {
    selectedPipelineIds.value.delete(pipeline.id);
  } else {
    selectedPipelineIds.value.add(pipeline.id);
  }
  // Trigger reactivity
  selectedPipelineIds.value = new Set(selectedPipelineIds.value);
}

function toggleSelectAll() {
  if (isAllSelected.value) {
    selectedPipelineIds.value = new Set();
  } else {
    selectedPipelineIds.value = new Set(displayPipelines.value.map((p) => p.id));
  }
}

function clearSelection() {
  selectedPipelineIds.value = new Set();
  // Only toggle local mode if not using injected
  if (!injectedMultiselectMode) {
    localMultiselectMode.value = false;
  } else {
    // Clear the injected mode
    injectedMultiselectMode.value = false;
  }
}

// Bulk actions
const { doSubmit: cancelSelectedPipelines, isLoading: isCancelingBulk } = useAsyncAction(async () => {
  if (!props.repo) {
    throw new Error('Repository is required for bulk operations');
  }

  const cancelablePipelines = selectedPipelines.value.filter((p) => p.status === 'pending' || p.status === 'running');

  await Promise.all(cancelablePipelines.map((pipeline) => apiClient.cancelPipeline(props.repo!.id, pipeline.number)));

  notifications.notify({
    title: t('notify_title_success'),
    text: t('repo.pipeline.actions.notify_text_cancel_bulk_success', { count: cancelablePipelines.length }),
    type: 'success',
    duration: 6000,
  });

  clearSelection();

  // Reload the page to show updated pipeline statuses
  window.location.reload();
});

const { doSubmit: restartSelectedPipelines, isLoading: isRestartingBulk } = useAsyncAction(async () => {
  if (!props.repo) {
    throw new Error('Repository is required for bulk operations');
  }

  // Run restarts sequentially to avoid race conditions
  const results = [];
  for (const pipeline of selectedPipelines.value) {
    try {
      const result = await apiClient.restartPipeline(props.repo!.id, String(pipeline.number), { fork: true });
      results.push({ status: 'fulfilled', value: result });
    } catch (error) {
      results.push({ status: 'rejected', reason: error });
    }
  }

  // Count successful restarts
  const successCount = results.filter((r) => r.status === 'fulfilled').length;
  const failedCount = results.filter((r) => r.status === 'rejected').length;

  if (successCount > 0) {
    notifications.notify({
      title: t('notify_title_success'),
      text: t('repo.pipeline.actions.notify_text_restart_bulk_success', { count: successCount }),
      type: 'success',
      duration: 6000,
    });
  }

  if (failedCount > 0) {
    notifications.notify({
      title: t('notify_title_error'),
      text: `Failed to restart ${failedCount} pipeline(s)`,
      type: 'error',
      duration: 6000,
    });
  }

  clearSelection();

  // Reload the page to show the new pipeline runs
  setTimeout(() => {
    window.location.reload();
  }, 1000);
});
</script>
