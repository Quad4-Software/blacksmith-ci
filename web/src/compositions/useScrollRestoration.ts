import { onBeforeUnmount, onMounted, unref } from 'vue';
import type { MaybeRef, Ref } from 'vue';

/**
 * Saved offsets for the lifetime of the SPA session, keyed by the caller's key
 * plus the history entry the offset was recorded on.
 */
const savedOffsets = new Map<string, number>();

/** Gestures that mean the user took over, so we must stop steering the container. */
const USER_SCROLL_EVENTS = ['wheel', 'touchstart', 'keydown'] as const;

/** How long, and how often, we keep re-applying the offset while content is still arriving. */
const RETRY_INTERVAL_MS = 50;
const RETRY_BUDGET_MS = 2000;

/**
 * Position of the current history entry.
 *
 * `createWebHistory` keeps a monotonic `position` in the state it pushes, so an
 * entry we come back to reports the same number it did when we left it, while a
 * fresh navigation to the same route reports a new one.
 */
function historyPosition(): number {
  const { position } = (window.history.state ?? {}) as { position?: unknown };
  return typeof position === 'number' ? position : -1;
}

function offsetKey(key: string, position: number): string {
  return `${key}@${position}`;
}

/**
 * Remember the scroll offset of a scrollable element and restore it when the
 * user returns to the same history entry via back/forward.
 *
 * Navigating to the route afresh lands on a new history entry and therefore
 * starts at the top, matching how browsers restore scroll for full page loads.
 *
 * @param el - the element that actually scrolls (the one with `overflow-y: auto`)
 * @param key - identifies the list, so two different repos don't share an offset
 */
export function useScrollRestoration(el: Ref<HTMLElement | null>, key: MaybeRef<string>): void {
  let node: HTMLElement | null = null;
  let restoreTarget: number | null = null;
  let retryTimer: ReturnType<typeof setInterval> | undefined;
  let retriesLeft = 0;

  function stopRestoring() {
    restoreTarget = null;
    if (retryTimer !== undefined) {
      clearInterval(retryTimer);
      retryTimer = undefined;
    }
  }

  function save() {
    // While a restore is in flight the offsets are ours, not the user's.
    if (node === null || restoreTarget !== null) {
      return;
    }
    savedOffsets.set(offsetKey(unref(key), historyPosition()), node.scrollTop);
  }

  function applyOnce() {
    if (node === null || restoreTarget === null) {
      return;
    }
    node.scrollTop = restoreTarget;
    // The browser clamps to the content height, so falling short means the rest
    // of the list hasn't rendered yet and we try again on the next tick.
    if (node.scrollTop === restoreTarget) {
      stopRestoring();
    }
  }

  onMounted(() => {
    node = el.value;
    if (node === null) {
      return;
    }

    node.addEventListener('scroll', save, { passive: true });
    for (const event of USER_SCROLL_EVENTS) {
      node.addEventListener(event, stopRestoring, { passive: true });
    }

    const saved = savedOffsets.get(offsetKey(unref(key), historyPosition()));
    if (saved === undefined || saved === 0) {
      return;
    }

    restoreTarget = saved;
    retriesLeft = Math.ceil(RETRY_BUDGET_MS / RETRY_INTERVAL_MS);
    applyOnce();
    if (restoreTarget === null) {
      return;
    }

    retryTimer = setInterval(() => {
      retriesLeft -= 1;
      if (retriesLeft <= 0) {
        stopRestoring();
        return;
      }
      applyOnce();
    }, RETRY_INTERVAL_MS);
  });

  onBeforeUnmount(() => {
    // No final save here: by the time we unmount the router has already pushed
    // the next history entry, so `historyPosition()` would file the offset under
    // the page we are leaving for. The scroll listener has kept it current.
    stopRestoring();
    if (node === null) {
      return;
    }
    node.removeEventListener('scroll', save);
    for (const event of USER_SCROLL_EVENTS) {
      node.removeEventListener(event, stopRestoring);
    }
    node = null;
  });
}

/** Test helper: drop every remembered offset. */
export function clearScrollRestorationCache(): void {
  savedOffsets.clear();
}
