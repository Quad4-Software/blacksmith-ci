import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import useEvents from './useEvents';

const mocks = vi.hoisted(() => ({
  on: vi.fn(),
  load: vi.fn().mockResolvedValue(undefined),
  markSseReconnect: vi.fn(),
  auth: { isAuthenticated: true },
}));
vi.mock('./useApiClient', () => ({ default: () => ({ on: mocks.on }) }));
vi.mock('~/store/auth', () => ({ useAuthStore: () => mocks.auth }));
vi.mock('~/store/repos', () => ({ useRepoStore: () => ({}) }));
vi.mock('~/store/pipelines', () => ({
  usePipelineStore: () => ({ markSseReconnect: mocks.markSseReconnect }),
}));
vi.mock('~/compositions/usePipelineFeed', () => ({ default: () => ({ load: mocks.load }) }));

describe('event stream recovery', () => {
  let cleanup: (() => void) | undefined;
  let streams: (EventTarget & { close: ReturnType<typeof vi.fn> })[];

  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
    mocks.auth.isAuthenticated = true;
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
    streams = [];
    mocks.on.mockImplementation(() => {
      const stream = Object.assign(new EventTarget(), { close: vi.fn() });
      streams.push(stream);
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
});
