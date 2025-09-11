<template>
  <div v-if="pipelines || infiniteScroll" class="space-y-4">
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
        class="sticky top-4 z-20 mb-2 backdrop-blur-md px-2"
      >
        <div
          class="flex flex-col sm:flex-row items-center justify-between gap-3 rounded-xl border border-wp-background-400 bg-wp-background-300 p-3 shadow-sm max-w-2xl mx-auto relative"
        >
          <!-- Close button in top-right corner -->
          <button
            class="absolute top-2 right-2 p-1 rounded-lg hover:bg-wp-background-400 transition-colors"
            :title="$t('repo.pipeline.clear_selection')"
            @click="clearSelection"
          >
            <Icon name="close" class="h-4 w-4 text-wp-text-100" />
          </button>

          <div class="flex items-center w-full sm:w-auto pr-8">
            <span class="text-xs sm:text-sm font-medium text-wp-text-100 whitespace-nowrap">
              {{ $t('repo.pipeline.selected_count', { count: selectedPipelines.length }) }}
            </span>
          </div>
          <div class="flex gap-2 sm:gap-3 justify-center sm:justify-end">
            <Button
              v-if="canCancelSelected"
              size="small"
              variant="outline"
              class="border-wp-custom-100/30 hover:border-wp-custom-100/50 text-xs sm:text-sm px-3 sm:px-4"
              :is-loading="isCancelingBulk"
              @click="cancelSelectedPipelines"
            >
              <Icon name="close" class="mr-1 sm:mr-1.5 h-3 w-3 sm:h-4 sm:w-4" />
              <span class="hidden sm:inline">{{ $t('repo.pipeline.actions.cancel_selected') }}</span>
              <span class="sm:hidden">{{ $t('repo.pipeline.actions.cancel') || 'Cancel' }}</span>
            </Button>
            <Button
              size="small"
              class="bg-wp-custom-100 hover:bg-wp-custom-200 text-xs sm:text-sm px-3 sm:px-4"
              :is-loading="isRestartingBulk"
              @click="restartSelectedPipelines"
            >
              <Icon name="play-outline" class="mr-1 sm:mr-1.5 h-3 w-3 sm:h-4 sm:w-4" />
              <span class="hidden sm:inline">{{ $t('repo.pipeline.actions.restart_selected') }}</span>
              <span class="sm:hidden">{{ $t('repo.pipeline.actions.restart') || 'Restart' }}</span>
            </Button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- Multi-select header -->
    <div
      v-if="isMultiselectMode && displayPipelines.length > 0 && repoPermissions?.push"
      class="flex items-center pb-0"
    >
      <div class="w-7 flex-shrink-0">
        <input
          id="select-all"
          type="checkbox"
          class="h-4 w-4 rounded border-wp-custom-200 accent-wp-custom-highlight-100 focus:ring-2 focus:ring-wp-custom-highlight-100 focus:ring-offset-1 transition-all cursor-pointer"
          :checked="isAllSelected"
          :indeterminate="isPartiallySelected"
          @change="toggleSelectAll"
        />
      </div>
      <label
        for="select-all"
        class="text-md text-wp-text-100 cursor-pointer select-none hover:text-wp-custom-100 transition-colors ml-0.5"
      >
        {{ $t('repo.pipeline.select_all') }}
      </label>
    </div>

    <PipelineItem
      v-for="pipeline in displayPipelines"
      :key="pipeline.id"
      :to="{
        name: 'repo-pipeline',
        params: { pipelineId: pipeline.number },
      }"
      :pipeline="pipeline"
      :is-selected="selectedPipelineIds.has(pipeline.id)"
      :show-checkbox="isMultiselectMode && repoPermissions?.push"
      @toggle-select="togglePipelineSelection(pipeline)"
    />

    <!-- Loading indicator for infinite scroll -->
    <div v-if="infiniteScroll && loading" class="flex justify-center items-center py-8">
      <div class="w-8 h-8">
        <PipelineProgressSpinner />
      </div>
      <span class="ml-3 text-wp-text-100">{{ $t('repo.pipeline.loading') }}</span>
    </div>

    <!-- Scroll trigger element -->
    <div v-if="infiniteScroll && hasMore" ref="scrollTrigger" class="h-4" />

    <Panel v-if="displayPipelines.length === 0 && !loading">
      <span class="text-wp-text-100">{{ $t('repo.pipeline.no_pipelines') }}</span>
    </Panel>
  </div>
</template>

<script lang="ts" setup>
import type { Ref } from 'vue';
import { computed, inject, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import Button from '~/components/atomic/Button.vue';
import Icon from '~/components/atomic/Icon.vue';
import Panel from '~/components/layout/Panel.vue';
import PipelineItem from '~/components/repo/pipeline/PipelineItem.vue';
import PipelineProgressSpinner from '~/components/repo/pipeline/PipelineProgressSpinner.vue';
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
const injectedMultiselectMode = inject<Ref<boolean> | null>('multiselect-mode' as any, null);

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
const displayPipelines = computed(() => {
  if (props.infiniteScroll && infiniteScrollData) {
    return data.value;
  }
  return props.pipelines || [];
});

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
      selectedPipelineIds.value.clear();
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
    selectedPipelineIds.value.clear();
  } else {
    selectedPipelineIds.value = new Set(displayPipelines.value.map((p) => p.id));
  }
}

function clearSelection() {
  selectedPipelineIds.value.clear();
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
