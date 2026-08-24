import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';
import useReactivePipelines from './useReactivePipelines';

// Stubbed to keep the toast component (and its theme/localStorage access at
// import time) out of this test's module graph.
vi.mock('~/compositions/useNotifications', () => ({
  default: () => ({ notify: vi.fn() }),
}));

describe('useReactivePipelines filter query params', () => {
  let requestedUrls: string[];

  beforeEach(() => {
    setActivePinia(createPinia());
    requestedUrls = [];
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) => {
        requestedUrls.push(url);
        return {
          ok: true,
          status: 200,
          headers: new Headers({ 'Content-Type': 'application/json' }),
          json: async () => [],
        };
      }),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function lastQuery() {
    return new URL(requestedUrls.at(-1)!, 'http://localhost').searchParams;
  }

  // The server declares this filter as `event` (GetPipelinesInput in
  // server/api/pipeline.go) and huma silently ignores unknown query params, so
  // sending `events` turned the whole event filter into a no-op.
  it('sends selected events as the `event` param', async () => {
    useReactivePipelines({
      repoId: ref(3),
      filters: { events: ref(['push', 'pull_request']) },
    });
    await vi.waitFor(() => expect(requestedUrls).toHaveLength(1));

    expect(lastQuery().get('event')).toBe('push,pull_request');
    expect(lastQuery().has('events')).toBe(false);
  });

  it('omits the event param when nothing is selected', async () => {
    useReactivePipelines({ repoId: ref(3), filters: { events: ref([]) } });
    await vi.waitFor(() => expect(requestedUrls).toHaveLength(1));

    expect(lastQuery().has('event')).toBe(false);
  });
});
