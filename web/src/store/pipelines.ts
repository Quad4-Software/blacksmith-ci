import { defineStore } from 'pinia';
import { computed, reactive, readonly, ref } from 'vue';
import type { Ref } from 'vue';

import useApiClient from '~/compositions/useApiClient';
import type { Pipeline, PipelineFeed, PipelineWorkflow } from '~/lib/api/types';
import { useRepoStore } from '~/store/repos';

/**
 * Pagination state for a repository's pipeline list.
 * Tracks infinite scroll state and SSE-delivered pipelines for deduplication.
 */
export interface PaginationState {
  page: number;
  hasMore: boolean;
  /** Pipeline IDs that have been fetched via API pagination */
  fetchedPipelineNumbers: Set<number>;
  /** Pipeline IDs that arrived via SSE (for deduplication with pagination) */
  ssePipelineNumbers: Set<number>;
}

/**
 * Compare two pipelines by creation timestamp.
 * @param {object} a - A pipeline.
 * @param {object} b - A pipeline.
 * @returns {number} 0 if created at the same time, < 0 if b was create before a, > 0 otherwise
 */
function comparePipelines(a: Pipeline, b: Pipeline): number {
  return (b.created || -1) - (a.created || -1);
}

/**
 * Compare two pipelines by the status.
 * Giving pending, running, or started higher priority than other status
 * @param {object} a - A pipeline.
 * @param {object} b - A pipeline.
 * @returns {number} 0 if status same priority, < 0 if b has higher priority, > 0 otherwise
 */
function comparePipelinesWithStatus(a: Pipeline, b: Pipeline): number {
  const bPriority = ['pending', 'running', 'started', 'blocked'].includes(b.status) ? 1 : 0;
  const aPriority = ['pending', 'running', 'started', 'blocked'].includes(a.status) ? 1 : 0;
  return bPriority - aPriority || comparePipelines(a, b);
}

export const usePipelineStore = defineStore('pipelines', () => {
  const apiClient = useApiClient();
  const repoStore = useRepoStore();

  const pipelines: Map<number, Map<number, Pipeline>> = reactive(new Map());

  /** Pagination state per repository (keyed by repoId) */
  const paginationStates: Map<number, PaginationState> = reactive(new Map());

  /**
   * Initialize or get pagination state for a repository.
   */
  function initPagination(repoId: number): PaginationState {
    if (!paginationStates.has(repoId)) {
      paginationStates.set(repoId, {
        page: 0,
        hasMore: true,
        fetchedPipelineNumbers: new Set(),
        ssePipelineNumbers: new Set(),
      });
    }
    return paginationStates.get(repoId)!;
  }

  /**
   * Update pagination state for a repository.
   */
  function updatePagination(
    repoId: number,
    updates: Partial<Omit<PaginationState, 'fetchedPipelineNumbers' | 'ssePipelineNumbers'>>,
  ): void {
    const state = initPagination(repoId);
    Object.assign(state, updates);
  }

  /**
   * Reset pagination state for a repository (e.g., when filters change).
   */
  function resetPagination(repoId: number): void {
    paginationStates.set(repoId, {
      page: 0,
      hasMore: true,
      fetchedPipelineNumbers: new Set(),
      ssePipelineNumbers: new Set(),
    });
  }

  /**
   * Mark a pipeline as delivered via SSE (for deduplication with pagination).
   */
  function markAsSSEDelivered(repoId: number, pipelineNumber: number): void {
    const state = initPagination(repoId);
    state.ssePipelineNumbers.add(pipelineNumber);
  }

  /**
   * Mark pipelines as fetched via API pagination.
   */
  function markAsFetched(repoId: number, pipelineNumbers: number[]): void {
    const state = initPagination(repoId);
    pipelineNumbers.forEach((num) => state.fetchedPipelineNumbers.add(num));
  }

  /**
   * Get pagination state for a repository (readonly).
   */
  function getPaginationState(repoId: number): PaginationState | undefined {
    return paginationStates.get(repoId);
  }

  function setPipeline(repoId: number, pipeline: Pipeline) {
    const repoPipelines = pipelines.get(repoId) ?? new Map<number, Pipeline>();
    repoPipelines.set(pipeline.number, {
      ...(repoPipelines.get(pipeline.number) ?? {}),
      ...pipeline,
    });

    // Update last pipeline number for the repo
    const repo = repoStore.repos.get(repoId);
    if (repo?.last_pipeline_number !== undefined && repo.last_pipeline_number < pipeline.number) {
      repo.last_pipeline_number = pipeline.number;
      repoStore.setRepo(repo);
    }

    pipelines.set(repoId, repoPipelines);
  }

  function getRepoPipelines(repoId: Ref<number>) {
    return computed(() => [...(pipelines.get(repoId.value)?.values() ?? [])].sort(comparePipelines));
  }

  function getPipeline(repoId: Ref<number>, _pipelineNumber: Ref<string | number>) {
    return computed(() => {
      if (typeof _pipelineNumber.value === 'string') {
        const pipelineNumber = Number.parseInt(_pipelineNumber.value, 10);
        return pipelines.get(repoId.value)?.get(pipelineNumber);
      }

      return pipelines.get(repoId.value)?.get(_pipelineNumber.value);
    });
  }

  function setWorkflow(repoId: number, pipelineNumber: number, workflow: PipelineWorkflow) {
    const pipeline = getPipeline(ref(repoId), ref(pipelineNumber.toString())).value;
    if (!pipeline) {
      throw new Error("Can't find pipeline");
    }

    if (!pipeline.workflows) {
      pipeline.workflows = [];
    }

    pipeline.workflows = [...pipeline.workflows.filter((p) => p.pid !== workflow.pid), workflow];
    setPipeline(repoId, pipeline);
  }

  /**
   * Load pipelines for a repository with optional filtering.
   * Updates both the pipeline store and pagination state.
   */
  async function loadRepoPipelines(
    repoId: number,
    opts?: {
      page?: number;
      perPage?: number;
      before?: string;
      after?: string;
      ref?: string;
      branch?: string;
      events?: string;
      search?: string;
      workflow?: string;
    },
  ): Promise<Pipeline[]> {
    const _pipelines = await apiClient.getPipelineList(repoId, opts);

    // Store all fetched pipelines
    _pipelines.forEach((pipeline) => {
      setPipeline(repoId, pipeline);
    });

    // Track pagination state if page was specified
    if (opts?.page !== undefined) {
      const pageSize = opts.perPage ?? 25;
      markAsFetched(
        repoId,
        _pipelines.map((p) => p.number),
      );
      updatePagination(repoId, {
        page: opts.page,
        hasMore: _pipelines.length >= pageSize,
      });
    }

    return _pipelines;
  }

  async function loadPipeline(repoId: number, pipelinesNumber: number) {
    const pipeline = await apiClient.getPipeline(repoId, pipelinesNumber);
    setPipeline(repoId, pipeline);
  }

  const pipelineFeed = computed(() =>
    [...pipelines.entries()]
      .reduce<PipelineFeed[]>((acc, [_repoId, repoPipelines]) => {
        const repoPipelinesArray = Array.from(
          repoPipelines.entries(),
          ([_pipelineNumber, pipeline]) =>
            <PipelineFeed>{
              ...pipeline,
              repo_id: _repoId,
              number: _pipelineNumber,
            },
        );
        return [...acc, ...repoPipelinesArray];
      }, [])
      .sort(comparePipelinesWithStatus)
      .filter((pipeline) => {
        // eslint-disable-next-line ts/no-unsafe-assignment
        const ownedRepoIdsRef = repoStore.ownedRepoIds as any;
        // eslint-disable-next-line ts/no-unsafe-assignment, ts/no-unsafe-member-access
        const ownedIds: number[] | null = ownedRepoIdsRef?.value ?? ownedRepoIdsRef ?? null;
        if (ownedIds == null || ownedIds.length === 0) {
          return true;
        }
        return ownedIds.includes(pipeline.repo_id);
      }),
  );

  const activePipelines = computed(() =>
    pipelineFeed.value.filter((pipeline) => ['pending', 'running', 'started', 'blocked'].includes(pipeline.status)),
  );

  async function loadPipelineFeed() {
    await repoStore.loadRepos();

    const _pipelines = await apiClient.getPipelineFeed();
    _pipelines.forEach((pipeline) => {
      setPipeline(pipeline.repo_id, pipeline);
    });
  }

  return {
    pipelines,
    setPipeline,
    setWorkflow,
    getRepoPipelines,
    getPipeline,
    loadRepoPipelines,
    loadPipeline,
    activePipelines,
    pipelineFeed,
    loadPipelineFeed,
    // Pagination state management
    paginationStates: readonly(paginationStates),
    initPagination,
    updatePagination,
    resetPagination,
    getPaginationState,
    markAsSSEDelivered,
    markAsFetched,
  };
});
