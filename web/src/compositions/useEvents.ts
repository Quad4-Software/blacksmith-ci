import { usePipelineStore } from '~/store/pipelines';
import { useRepoStore } from '~/store/repos';

import useApiClient from './useApiClient';

const apiClient = useApiClient();
let initialized = false;

export default () => {
  if (initialized) {
    return;
  }
  const repoStore = useRepoStore();
  const pipelineStore = usePipelineStore();

  initialized = true;

  const events = apiClient.on((data) => {
    // contains repo update
    if (!data.repo) {
      return;
    }
    const { repo } = data;
    repoStore.setRepo(repo);

    // contains pipeline update
    if (!data.pipeline) {
      return;
    }
    const { pipeline } = data;
    pipelineStore.setPipeline(repo.id, pipeline);

    // Mark as SSE-delivered for deduplication with pagination
    // This ensures the pipeline appears in the list even if pagination hasn't fetched it yet
    pipelineStore.markAsSSEDelivered(repo.id, pipeline.number);
  });

  // The browser's EventSource auto-reconnects on transient failures, but any events
  // published while the connection is down are lost — there is no replay protocol on
  // the server. Detect the error → open transition and bump a generation counter so
  // views currently rendering live pipeline state can refetch to catch up.
  let sawError = false;
  events.addEventListener('error', () => {
    sawError = true;
  });
  events.addEventListener('open', () => {
    if (sawError) {
      sawError = false;
      pipelineStore.markSseReconnect();
    }
  });
};
