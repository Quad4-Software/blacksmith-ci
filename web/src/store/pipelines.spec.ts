import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import useApiClient from '~/compositions/useApiClient';
import type { Pipeline } from '~/lib/api/types';
import { useRepoStore } from './repos';

import { usePipelineStore } from './pipelines';

describe('pipelineStore SSE reconnect counter', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('starts at 0', () => {
    const store = usePipelineStore();
    expect(store.sseConnectionId).toBe(0);
  });

  it('increments on each markSseReconnect call', () => {
    const store = usePipelineStore();
    store.markSseReconnect();
    expect(store.sseConnectionId).toBe(1);
    store.markSseReconnect();
    expect(store.sseConnectionId).toBe(2);
  });
});

describe('pipeline feed recovery', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
    vi.spyOn(useRepoStore(), 'loadRepos').mockResolvedValue(undefined);
  });

  it('reconciles active pipelines that have fallen out of the bounded feed', async () => {
    const store = usePipelineStore();
    const api = useApiClient();
    const old = { number: 1, status: 'running', reviewed: 0 } as Pipeline;
    store.setPipeline(7, old);
    vi.spyOn(api, 'getPipelineFeed').mockResolvedValue([{ ...old, repo_id: 7, number: 2, status: 'running' }]);
    const getPipeline = vi.spyOn(api, 'getPipeline').mockResolvedValue({ ...old, status: 'success' });

    await store.loadPipelineFeed();

    expect(getPipeline).toHaveBeenCalledWith(7, 1);
    expect(store.activePipelines.map((p) => p.number)).toEqual([2]);
    expect(store.pipelines.get(7)?.get(1)?.status).toBe('success');
  });

  it('does not refetch entries included in the feed or regress a newer SSE update', async () => {
    const store = usePipelineStore();
    const old = { number: 1, status: 'running', reviewed: 0 } as Pipeline;
    store.setPipeline(7, old);
    vi.spyOn(useApiClient(), 'getPipelineFeed').mockImplementation(async () => {
      store.setPipeline(7, { ...old, status: 'success' });
      return [{ ...old, repo_id: 7 }];
    });
    const getPipeline = vi.spyOn(useApiClient(), 'getPipeline');

    await store.loadPipelineFeed();

    expect(getPipeline).not.toHaveBeenCalled();
    expect(store.activePipelines).toEqual([]);
  });
});
