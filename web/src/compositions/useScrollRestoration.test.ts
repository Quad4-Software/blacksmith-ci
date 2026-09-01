import { shallowMount } from '@vue/test-utils';
import type { VueWrapper } from '@vue/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { h, ref } from 'vue';
import { clearScrollRestorationCache, useScrollRestoration } from './useScrollRestoration';

/**
 * A stand-in for the scroll container. happy-dom has no layout, so `scrollTop`
 * is clamped by hand against a content height the test can grow, which is what
 * makes the "content is still loading" retry observable.
 */
function createScroller(contentHeight = 5000, viewportHeight = 500) {
  const node = document.createElement('div');
  let top = 0;
  let maxScroll = contentHeight - viewportHeight;

  Object.defineProperty(node, 'scrollTop', {
    get: () => top,
    set: (value: number) => {
      top = Math.min(Math.max(value, 0), maxScroll);
      node.dispatchEvent(new Event('scroll'));
    },
  });

  return {
    node,
    grow(newContentHeight: number) {
      maxScroll = newContentHeight - viewportHeight;
    },
  };
}

function goToHistoryPosition(position: number) {
  window.history.replaceState({ position }, '');
}

function mountScroller(node: HTMLElement, key = 'pipelines:1'): VueWrapper {
  return shallowMount({
    setup() {
      useScrollRestoration(ref(node), key);
      return () => h('div');
    },
  });
}

describe('useScrollRestoration', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    clearScrollRestorationCache();
    goToHistoryPosition(0);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('restores the offset when returning to the same history entry', () => {
    const first = createScroller();
    goToHistoryPosition(4);
    const list = mountScroller(first.node);
    first.node.scrollTop = 1200;
    list.unmount();

    // Forward to a detail page, then back to the list's own entry.
    goToHistoryPosition(5);
    goToHistoryPosition(4);
    const second = createScroller();
    mountScroller(second.node);

    expect(second.node.scrollTop).toBe(1200);
  });

  it('starts at the top when the route is opened on a fresh history entry', () => {
    const first = createScroller();
    goToHistoryPosition(4);
    const list = mountScroller(first.node);
    first.node.scrollTop = 1200;
    list.unmount();

    goToHistoryPosition(9);
    const second = createScroller();
    mountScroller(second.node);

    expect(second.node.scrollTop).toBe(0);
  });

  it('keeps offsets of different lists apart', () => {
    const first = createScroller();
    goToHistoryPosition(4);
    const list = mountScroller(first.node, 'pipelines:1');
    first.node.scrollTop = 1200;
    list.unmount();

    const second = createScroller();
    mountScroller(second.node, 'pipelines:2');

    expect(second.node.scrollTop).toBe(0);
  });

  it('re-applies the offset as more content arrives', () => {
    const first = createScroller();
    goToHistoryPosition(4);
    const list = mountScroller(first.node);
    first.node.scrollTop = 1200;
    list.unmount();

    // Coming back, only the first page has rendered so far.
    const second = createScroller(900);
    mountScroller(second.node);
    expect(second.node.scrollTop).toBe(400);

    second.grow(5000);
    vi.advanceTimersByTime(50);
    expect(second.node.scrollTop).toBe(1200);

    // Once the target is reached we stop touching the container.
    second.node.scrollTop = 0;
    vi.advanceTimersByTime(1000);
    expect(second.node.scrollTop).toBe(0);
  });

  it('gives up re-applying after the retry budget', () => {
    const first = createScroller();
    goToHistoryPosition(4);
    const list = mountScroller(first.node);
    first.node.scrollTop = 1200;
    list.unmount();

    const second = createScroller(900);
    mountScroller(second.node);

    vi.advanceTimersByTime(2000);
    second.grow(5000);
    vi.advanceTimersByTime(1000);

    expect(second.node.scrollTop).toBe(400);
  });

  it('stops restoring once the user scrolls themselves', () => {
    const first = createScroller();
    goToHistoryPosition(4);
    const list = mountScroller(first.node);
    first.node.scrollTop = 1200;
    list.unmount();

    const second = createScroller(900);
    mountScroller(second.node);

    second.node.dispatchEvent(new Event('wheel'));
    second.grow(5000);
    vi.advanceTimersByTime(1000);

    expect(second.node.scrollTop).toBe(400);
  });

  it('records the offset the user scrolls to after a restore', () => {
    const first = createScroller();
    goToHistoryPosition(4);
    const list = mountScroller(first.node);
    first.node.scrollTop = 1200;
    list.unmount();

    goToHistoryPosition(4);
    const second = createScroller();
    const restored = mountScroller(second.node);
    second.node.scrollTop = 2400;
    restored.unmount();

    goToHistoryPosition(4);
    const third = createScroller();
    mountScroller(third.node);

    expect(third.node.scrollTop).toBe(2400);
  });
});
