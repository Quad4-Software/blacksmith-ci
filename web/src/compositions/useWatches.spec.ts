import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { WatchOverviewItem } from '~/lib/api/types';

const getUserWatches = vi.fn();
const markWatchRead = vi.fn();
const markAllWatchesRead = vi.fn();
const deleteWatch = vi.fn();

vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({ getUserWatches, markWatchRead, markAllWatchesRead, deleteWatch }),
}));

const { useWatches } = await import('./useWatches');

function item(id: number, failing: boolean, unread: boolean, inApp = true): WatchOverviewItem {
  return {
    watch: { id, in_app: inApp } as WatchOverviewItem['watch'],
    repo_full_name: `o/r${id}`,
    failing,
    unread,
    failing_workflows: [],
  };
}

describe('useWatches', () => {
  beforeEach(() => {
    getUserWatches.mockReset();
    markWatchRead.mockReset();
    markAllWatchesRead.mockReset();
    deleteWatch.mockReset();
    getUserWatches.mockResolvedValue([item(1, true, false), item(2, true, true), item(3, false, false)]);
  });

  it('counts unread failing watches and orders failing unread first', async () => {
    const w = useWatches();
    await w.load();
    expect(w.unreadCount.value).toBe(1);
    expect(w.failing.value.map((i) => i.watch.id)).toEqual([2, 1]);
  });

  it('marks a watch read without reloading', async () => {
    markWatchRead.mockResolvedValue(undefined);
    const w = useWatches();
    await w.load();
    await w.markRead(2);
    expect(w.unreadCount.value).toBe(0);
    expect(getUserWatches).toHaveBeenCalledTimes(1);
  });

  it('marks all read and unwatches locally', async () => {
    markAllWatchesRead.mockResolvedValue(undefined);
    deleteWatch.mockResolvedValue(undefined);
    const w = useWatches();
    await w.load();
    await w.unwatch(1);
    expect(w.items.value.map((i) => i.watch.id)).toEqual([2, 3]);
    await w.markAllRead();
    expect(w.unreadCount.value).toBe(0);
  });

  it('keeps email-only watches out of the failing list and the badge', async () => {
    // The server already clears the flags; an edited watch must not linger until the next reload.
    getUserWatches.mockResolvedValue([item(1, true, true, false), item(2, true, true), item(3, false, false, false)]);
    const w = useWatches();
    await w.load();
    expect(w.failing.value.map((i) => i.watch.id)).toEqual([2]);
    expect(w.unreadCount.value).toBe(1);
    expect(w.items.value.map((i) => i.watch.id)).toEqual([1, 2, 3]);
  });
});
