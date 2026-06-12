import { debounce } from 'lodash';

import usePipelineFeed from '~/compositions/usePipelineFeed';
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
  const pipelineFeed = usePipelineFeed();

  initialized = true;

  // The navbar approval-gate badge counts pending gates via a separate endpoint
  // that SSE does not drive. A gate transition (block / approve / decline /
  // expire) publishes a pipeline event, so refresh the count whenever a pipeline
  // event arrives — debounced so a burst of step updates coalesces into one
  // refetch. Without this the badge stays stale until a full page reload.
  const refreshApprovals = debounce(() => {
    void pipelineFeed.loadWorkflowApprovals();
  }, 1000);

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

    refreshApprovals();
  });

  // Refetch on every SSE (re)connect because the server has no replay protocol — any
  // Update events fired while the EventSource was still CONNECTING (initial mount) or
  // while it was reconnecting after a transient failure are silently dropped. Bumping
  // a generation counter signals to views rendering live pipeline state to refetch
  // and catch up to the true server state.
  events.addEventListener('open', () => {
    pipelineStore.markSseReconnect();
  });
};
