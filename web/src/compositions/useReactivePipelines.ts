import type { ComputedRef, Ref } from 'vue';
import { computed, ref, watch } from 'vue';

import useNotifications from '~/compositions/useNotifications';
import type { Pipeline } from '~/lib/api/types';
import { usePipelineStore } from '~/store/pipelines';

export interface UseReactivePipelinesOptions {
  repoId: Ref<number>;
  pageSize?: number;
  filters?: {
    search?: Ref<string | null> | Ref<string> | null;
    events?: Ref<string[] | null> | Ref<string[]> | null;
    branch?: Ref<string | null> | Ref<string> | null;
    dateFrom?: Ref<string | null> | Ref<string> | null;
    dateTo?: Ref<string | null> | Ref<string> | null;
    workflow?: Ref<string | null> | Ref<string> | null;
  };
}

export interface UseReactivePipelinesReturn {
  /** Reactive computed pipelines from store (automatically updates on SSE events) */
  pipelines: ComputedRef<Pipeline[]>;
  /** Whether data is currently being loaded */
  loading: Ref<boolean>;
  /** Whether there are more pages to load */
  hasMore: ComputedRef<boolean>;
  /** Load the next page of pipelines */
  loadMore: () => Promise<void>;
  /** Reset pagination and reload from page 1 (e.g., when filters change) */
  reset: () => Promise<void>;
}

/**
 * Composable for reactive pipeline data with infinite scroll support.
 *
 * This composable bridges the pipeline store (which receives SSE updates)
 * with component views, providing:
 * - Reactive pipelines that update automatically when SSE events arrive
 * - Infinite scroll pagination
 * - Server-side filtering
 *
 * @example
 * ```ts
 * const { pipelines, loading, hasMore, loadMore } = useReactivePipelines({
 *   repoId: computed(() => props.repo.id),
 *   filters: {
 *     search: searchQuery,
 *     events: selectedEvents,
 *   },
 * });
 * ```
 */
export default function useReactivePipelines(options: UseReactivePipelinesOptions): UseReactivePipelinesReturn {
  const { repoId, pageSize = 25, filters } = options;
  const pipelineStore = usePipelineStore();
  const notifications = useNotifications();

  const loading = ref(false);
  const localHasMore = ref(true);

  // Get all pipelines for this repo from store (reactive to SSE updates)
  const allRepoPipelines = pipelineStore.getRepoPipelines(repoId);

  // Get pagination state from store
  const paginationState = computed(() => pipelineStore.getPaginationState(repoId.value));

  /**
   * Reactive pipelines computed from the store.
   *
   * This filters the store's pipelines to only include those that have been
   * fetched via pagination OR arrived via SSE. This ensures:
   * 1. SSE updates appear immediately (new pipelines at top, status updates)
   * 2. Pagination still works (only shows fetched + SSE pipelines)
   * 3. No duplicates (store merges by pipeline number)
   */
  const pipelines = computed(() => {
    const state = paginationState.value;
    if (!state) {
      // No pagination state yet - return all from store
      return allRepoPipelines.value;
    }

    // Include pipelines that were fetched via API OR arrived via SSE
    return allRepoPipelines.value.filter(
      (p) => state.fetchedPipelineNumbers.has(p.number) || state.ssePipelineNumbers.has(p.number),
    );
  });

  const hasMore = computed(() => {
    const state = paginationState.value;
    return state?.hasMore ?? localHasMore.value;
  });

  /**
   * Load the next page of pipelines.
   */
  async function loadMore(): Promise<void> {
    if (loading.value || !hasMore.value) return;

    loading.value = true;
    try {
      const state = pipelineStore.initPagination(repoId.value);
      const nextPage = state.page + 1;

      // Build API options from filters
      const searchValue = filters?.search?.value;
      const eventsValue = filters?.events?.value;
      const dateFromValue = filters?.dateFrom?.value;
      const dateToValue = filters?.dateTo?.value;
      const branchValue = filters?.branch?.value;
      const workflowValue = filters?.workflow?.value;

      const opts: Parameters<typeof pipelineStore.loadRepoPipelines>[1] = {
        page: nextPage,
        perPage: pageSize,
        search: searchValue !== null && searchValue !== undefined && searchValue !== '' ? searchValue : undefined,
        events:
          eventsValue !== null && eventsValue !== undefined && eventsValue.length > 0
            ? eventsValue.join(',')
            : undefined,
        after:
          dateFromValue !== null && dateFromValue !== undefined && dateFromValue !== ''
            ? new Date(dateFromValue).toISOString()
            : undefined,
        before:
          dateToValue !== null && dateToValue !== undefined && dateToValue !== ''
            ? new Date(`${dateToValue}T23:59:59`).toISOString()
            : undefined,
      };

      // Handle branch/ref filter
      if (branchValue !== null && branchValue !== undefined && branchValue !== '') {
        if (branchValue.startsWith('refs/')) {
          opts.ref = branchValue;
        } else {
          opts.branch = branchValue;
        }
      }

      if (workflowValue !== null && workflowValue !== undefined && workflowValue !== '') {
        opts.workflow = workflowValue;
      }

      const fetchedPipelines = await pipelineStore.loadRepoPipelines(repoId.value, opts);
      localHasMore.value = fetchedPipelines.length >= pageSize;
    } catch (error) {
      notifications.notify({ title: 'Failed to load pipelines', type: 'error' });
      console.error('Error loading pipelines:', error);
    } finally {
      loading.value = false;
    }
  }

  /**
   * Reset pagination and reload from page 1.
   * Called automatically when filters change.
   */
  async function reset(): Promise<void> {
    pipelineStore.resetPagination(repoId.value);
    localHasMore.value = true;
    await loadMore();
  }

  // Initial load - only if we have a valid repoId
  // Note: We use a flag to prevent duplicate loads from watchers on mount
  let isInitialized = false;
  if (repoId.value) {
    void loadMore().then(() => {
      isInitialized = true;
      return undefined;
    });
  } else {
    isInitialized = true;
  }

  // Watch filters and reset pagination when they change
  // We skip the first call using isInitialized to prevent double-loading
  if (filters) {
    const filterRefs = [
      filters.search ?? ref(null),
      filters.events ?? ref(null),
      filters.branch ?? ref(null),
      filters.dateFrom ?? ref(null),
      filters.dateTo ?? ref(null),
      filters.workflow ?? ref(null),
    ];

    watch(
      filterRefs,
      () => {
        if (isInitialized) {
          void reset();
        }
      },
      { deep: true },
    );
  }

  // Watch for repoId changes (e.g., navigating to different repo)
  watch(repoId, (newValue, oldValue) => {
    // Only reset if repo actually changed (not on initial mount)
    if (oldValue !== undefined && newValue !== oldValue) {
      void reset();
    }
  });

  return {
    pipelines,
    loading,
    hasMore,
    loadMore,
    reset,
  };
}
