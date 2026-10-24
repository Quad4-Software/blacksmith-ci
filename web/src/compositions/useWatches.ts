import { debounce } from 'lodash';
import { computed, ref } from 'vue';

import useApiClient from '~/compositions/useApiClient';
import type { WatchOverviewItem } from '~/lib/api/types';

// Module-scoped state so the header bell, the overview page and the watch
// popovers all see the same list.
const items = ref<WatchOverviewItem[]>([]);
const loading = ref(false);

// Only watches with the in-app channel feed the bell and the Failing tab. The
// server already reports failing/unread as false for email-only watches; the
// in_app check keeps a locally edited watch consistent until the next load.
function showsInApp(i: WatchOverviewItem): boolean {
  return i.failing && i.watch.in_app;
}

const failing = computed(() => items.value.filter(showsInApp).toSorted((a, b) => Number(b.unread) - Number(a.unread)));
const unreadCount = computed(() => items.value.filter((i) => showsInApp(i) && i.unread).length);

async function load(): Promise<void> {
  loading.value = true;
  try {
    items.value = await useApiClient().getUserWatches({ silent: true });
  } finally {
    loading.value = false;
  }
}

async function markRead(id: number): Promise<void> {
  await useApiClient().markWatchRead(id);
  items.value = items.value.map((i) => (i.watch.id === id ? { ...i, unread: false } : i));
}

async function markAllRead(): Promise<void> {
  await useApiClient().markAllWatchesRead();
  items.value = items.value.map((i) => ({ ...i, unread: false }));
}

async function unwatch(id: number): Promise<void> {
  await useApiClient().deleteWatch(id);
  items.value = items.value.filter((i) => i.watch.id !== id);
}

export function useWatches() {
  return { items, failing, unreadCount, loading, load, markRead, markAllRead, unwatch };
}

export const refreshWatchesDebounced = debounce(() => {
  void load().catch(console.error);
}, 1000);
