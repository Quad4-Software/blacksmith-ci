import { debounce } from 'lodash';

import usePipelineFeed from '~/compositions/usePipelineFeed';
import { refreshWatchesDebounced, useWatches } from '~/compositions/useWatches';
import type { Pipeline } from '~/lib/api/types';
import { watchKeyFor } from '~/lib/watchKey';
import { useAuthStore } from '~/store/auth';
import { usePipelineStore } from '~/store/pipelines';
import { useRepoStore } from '~/store/repos';
import useApiClient from './useApiClient';

const apiClient = useApiClient();
let initialized = false;

const terminalStatuses = new Set(['success', 'failure', 'error', 'killed', 'declined', 'skipped']);

export default () => {
  if (initialized) {
    return;
  }
  const authStore = useAuthStore();
  const repoStore = useRepoStore();
  const pipelineStore = usePipelineStore();
  const pipelineFeed = usePipelineFeed();
  const { items: watchItems } = useWatches();

  initialized = true;

  // Whether the pipeline belongs to a key the user watches. Without this check
  // every finished pipeline the stream delivers would refetch the watches.
  const isWatched = (repoId: number, pipeline: Pipeline): boolean => {
    if (watchItems.value.length === 0) return false;
    const key = watchKeyFor(pipeline);
    if (!key) return false;
    return watchItems.value.some(
      (i) => i.watch.repo_id === repoId && i.watch.event === key.event && i.watch.target === key.target,
    );
  };

  // The navbar approval-gate badge counts pending gates via a separate endpoint
  // that SSE does not drive. A gate transition (block / approve / decline /
  // expire) publishes a pipeline event, so refresh the count whenever a pipeline
  // event arrives — debounced so a burst of step updates coalesces into one
  // refetch. Without this the badge stays stale until a full page reload.
  const refreshApprovals = debounce(() => {
    void pipelineFeed.loadWorkflowApprovals();
  }, 1000);

  const connect = () =>
    apiClient.on((data) => {
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

      // Watch state only changes when a watched pipeline reaches a terminal status.
      if (terminalStatuses.has(pipeline.status) && isWatched(repo.id, pipeline)) {
        refreshWatchesDebounced();
      }
    });

  // The server does not replay missed events. Refresh shared state as well as
  // the current view on every connection, including the initial one.
  const refresh = debounce(() => {
    if (!authStore.isAuthenticated) return;
    pipelineStore.markSseReconnect();
    void pipelineFeed.load().catch(console.error);
    refreshWatchesDebounced();
  }, 250);

  let events = connect();
  events.addEventListener('open', refresh);

  // A suspended tab can retain an apparently OPEN but dead connection. Replace
  // it when returning to the tab or network instead of trusting readyState.
  const resume = debounce(() => {
    if (document.visibilityState === 'hidden' || !authStore.isAuthenticated) return;
    events.removeEventListener('open', refresh);
    events.close();
    events = connect();
    events.addEventListener('open', refresh);
    // Recover the snapshot even if the streaming endpoint is unavailable.
    refresh();
  }, 250);

  document.addEventListener('visibilitychange', resume);
  window.addEventListener('focus', resume);
  window.addEventListener('online', resume);
  window.addEventListener('pageshow', resume);

  return () => {
    document.removeEventListener('visibilitychange', resume);
    window.removeEventListener('focus', resume);
    window.removeEventListener('online', resume);
    window.removeEventListener('pageshow', resume);
    resume.cancel();
    refresh.cancel();
    refreshApprovals.cancel();
    refreshWatchesDebounced.cancel();
    events.removeEventListener('open', refresh);
    events.close();
    initialized = false;
  };
};
