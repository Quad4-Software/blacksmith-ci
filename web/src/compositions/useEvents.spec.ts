import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import useEvents from './useEvents';

const mocks = vi.hoisted(() => ({
  on: vi.fn(),
  load: vi.fn().mockResolvedValue(undefined),
  markSseReconnect: vi.fn(),
  auth: { isAuthenticated: true },
  refreshWatches: Object.assign(vi.fn(), { cancel: vi.fn() }),
  watchItems: { value: [] as { watch: { repo_id: number; event: string; target: string } }[] },
}));
vi.mock('./useApiClient', () => ({ default: () => ({ on: mocks.on }) }));
vi.mock('~/store/auth', () => ({ useAuthStore: () => mocks.auth }));
vi.mock('~/store/repos', () => ({ useRepoStore: () => ({ setRepo: vi.fn() }) }));
vi.mock('~/store/pipelines', () => ({
  usePipelineStore: () => ({
    markSseReconnect: mocks.markSseReconnect,
    setPipeline: vi.fn(),
    markAsSSEDelivered: vi.fn(),
  }),
}));
vi.mock('~/compositions/usePipelineFeed', () => ({
  default: () => ({ load: mocks.load, loadWorkflowApprovals: vi.fn() }),
}));
vi.mock('~/compositions/useWatches', () => ({
  refreshWatchesDebounced: mocks.refreshWatches,
  useWatches: () => ({ items: mocks.watchItems }),
}));

describe('event stream recovery', () => {
  let cleanup: (() => void) | undefined;
  let streams: (EventTarget & { close: ReturnType<typeof vi.fn> })[];
  let handlers: ((data: unknown) => void)[];

  function finish(repoId: number, pipeline: Record<string, unknown>) {
    handlers[0]({ repo: { id: repoId }, pipeline: { number: 1, status: 'failure', ...pipeline } });
  }

  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
    mocks.auth.isAuthenticated = true;
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
    streams = [];
    handlers = [];
    mocks.watchItems.value = [];
    mocks.on.mockImplementation((handler: (data: unknown) => void) => {
      const stream = Object.assign(new EventTarget(), { close: vi.fn() });
      streams.push(stream);
      handlers.push(handler);
      return stream;
    });
    cleanup = useEvents();
  });

  afterEach(() => {
    cleanup?.();
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it('refreshes the shared feed and current view on initial connect and reconnect', async () => {
    for (let count = 1; count <= 2; count++) {
      streams[0].dispatchEvent(new Event('open'));
      await vi.advanceTimersByTimeAsync(250);
      expect(mocks.load).toHaveBeenCalledTimes(count);
      expect(mocks.markSseReconnect).toHaveBeenCalledTimes(count);
    }
  });

  it('replaces a stale connection on tab return, coalescing focus and visibility events', async () => {
    document.dispatchEvent(new Event('visibilitychange'));
    window.dispatchEvent(new Event('focus'));
    await vi.advanceTimersByTimeAsync(500);
    expect(streams[0].close).toHaveBeenCalledOnce();
    expect(mocks.on).toHaveBeenCalledTimes(2);
    expect(mocks.load).toHaveBeenCalledOnce();
    streams[1].dispatchEvent(new Event('open'));
    await vi.advanceTimersByTimeAsync(250);
    expect(mocks.load).toHaveBeenCalledTimes(2);
  });

  it.each(['online', 'pageshow'])('recovers after %s', async (event) => {
    window.dispatchEvent(new Event(event));
    await vi.advanceTimersByTimeAsync(500);
    expect(mocks.on).toHaveBeenCalledTimes(2);
    expect(mocks.load).toHaveBeenCalledOnce();
  });

  it('does not reconnect in the background or after session expiration', async () => {
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden');
    document.dispatchEvent(new Event('visibilitychange'));
    await vi.advanceTimersByTimeAsync(500);
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
    mocks.auth.isAuthenticated = false;
    window.dispatchEvent(new Event('focus'));
    streams[0].dispatchEvent(new Event('open'));
    await vi.advanceTimersByTimeAsync(500);
    expect(mocks.on).toHaveBeenCalledOnce();
    expect(mocks.load).not.toHaveBeenCalled();
  });

  it('removes listeners and pending recovery on cleanup', async () => {
    window.dispatchEvent(new Event('focus'));
    cleanup?.();
    cleanup = undefined;
    window.dispatchEvent(new Event('online'));
    await vi.advanceTimersByTimeAsync(1000);
    expect(mocks.on).toHaveBeenCalledOnce();
    expect(mocks.load).not.toHaveBeenCalled();
    expect(streams[0].close).toHaveBeenCalledOnce();
  });

  it('refreshes watches on reconnect', async () => {
    streams[0].dispatchEvent(new Event('open'));
    await vi.advanceTimersByTimeAsync(250);
    expect(mocks.refreshWatches).toHaveBeenCalledOnce();
  });

  it('refreshes watches only for finished pipelines of a watched key', () => {
    // no watches loaded: no refetch for any pipeline in the fleet
    finish(1, { event: 'push', branch: 'main' });
    expect(mocks.refreshWatches).not.toHaveBeenCalled();

    mocks.watchItems.value = [{ watch: { repo_id: 1, event: 'push', target: 'main' } }];
    finish(1, { event: 'push', branch: 'dev' });
    finish(2, { event: 'push', branch: 'main' });
    finish(1, { event: 'pull_request', branch: 'main' });
    finish(1, { event: 'push', branch: 'main', status: 'running' });
    expect(mocks.refreshWatches).not.toHaveBeenCalled();

    finish(1, { event: 'push', branch: 'main' });
    expect(mocks.refreshWatches).toHaveBeenCalledOnce();
  });
});
