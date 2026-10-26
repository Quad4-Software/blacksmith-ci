import { debounce } from 'lodash';
import { ref } from 'vue';

import useApiClient from '~/compositions/useApiClient';
import type { StuckPipeline } from '~/lib/api/types';

// Module-scoped so every bell instance shares one list.
const stuck = ref<StuckPipeline[]>([]);

async function load(): Promise<void> {
  stuck.value = await useApiClient().getUserStuckPipelines({ silent: true });
}

// Whether the pipeline is listed as stuck, so only its own events refetch.
function isStuck(repoId: number, pipelineNumber: number): boolean {
  return stuck.value.some((p) => p.repo_id === repoId && p.pipeline_number === pipelineNumber);
}

export function useStuckPipelines() {
  return { stuck, load, isStuck };
}

export const refreshStuckPipelinesDebounced = debounce(() => {
  void load().catch(console.error);
}, 1000);
