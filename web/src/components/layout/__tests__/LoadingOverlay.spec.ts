import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { nextTick } from 'vue';

import LoadingOverlay from '../LoadingOverlay.vue';
import { __resetLoadingOverlay, useLoadingOverlay } from '~/compositions/useLoadingOverlay';

vi.mock('~/components/atomic/Icon.vue', () => ({
  default: {
    name: 'Icon',
    props: ['name'],
    template: '<i :data-icon="name" />',
  },
}));

describe('loadingOverlay', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    __resetLoadingOverlay();
  });

  afterEach(() => {
    vi.useRealTimers();
    document.body.style.overflow = '';
  });

  it('renders nothing when state is hidden', () => {
    const wrapper = mount(LoadingOverlay, {
      attachTo: document.body,
      global: {
        stubs: {
          Transition: { template: '<slot />' },
        },
      },
    });
    expect(document.body.querySelector('[role="status"]')).toBeNull();
    wrapper.unmount();
  });

  it('renders spinner + text when state becomes visible', async () => {
    const wrapper = mount(LoadingOverlay, {
      attachTo: document.body,
      global: {
        stubs: {
          Transition: { template: '<slot />' },
        },
      },
    });
    const { withOverlay } = useLoadingOverlay();

    let resolveAction: (() => void) | undefined;
    const actionPromise = new Promise<void>((resolve) => {
      resolveAction = resolve;
    });

    const pending = withOverlay('Creating pipeline...', async () => actionPromise);

    await vi.advanceTimersByTimeAsync(200);
    await nextTick();

    const card = document.body.querySelector('[role="status"]');
    expect(card).not.toBeNull();
    expect(card?.textContent).toContain('Creating pipeline...');
    expect(card?.querySelector('[data-icon="spinner"]')).not.toBeNull();

    resolveAction!();
    await pending;
    await nextTick();
    expect(document.body.querySelector('[role="status"]')).toBeNull();

    wrapper.unmount();
  });

  it('locks body scroll while visible and releases on hide', async () => {
    const wrapper = mount(LoadingOverlay, {
      attachTo: document.body,
      global: {
        stubs: {
          Transition: { template: '<slot />' },
        },
      },
    });
    const { withOverlay } = useLoadingOverlay();

    let resolveAction: (() => void) | undefined;
    const actionPromise = new Promise<void>((resolve) => {
      resolveAction = resolve;
    });

    const pending = withOverlay('working', async () => actionPromise);

    await vi.advanceTimersByTimeAsync(200);
    await nextTick();
    expect(document.body.style.overflow).toBe('hidden');

    resolveAction!();
    await pending;
    await nextTick();
    expect(document.body.style.overflow).not.toBe('hidden');

    wrapper.unmount();
  });

  it('prevents default and stops propagation on ESC keydown while visible', async () => {
    const wrapper = mount(LoadingOverlay, {
      attachTo: document.body,
      global: {
        stubs: {
          Transition: { template: '<slot />' },
        },
      },
    });
    const { withOverlay } = useLoadingOverlay();

    let resolveAction: (() => void) | undefined;
    const actionPromise = new Promise<void>((resolve) => {
      resolveAction = resolve;
    });

    const pending = withOverlay('working', async () => actionPromise);

    await vi.advanceTimersByTimeAsync(200);
    await nextTick();

    const event = new KeyboardEvent('keydown', { key: 'Escape', cancelable: true });
    const stopPropagationSpy = vi.spyOn(event, 'stopPropagation');
    window.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(true);
    expect(stopPropagationSpy).toHaveBeenCalled();

    resolveAction!();
    await pending;
    wrapper.unmount();
  });

  it('does not prevent default on ESC keydown when overlay is hidden', () => {
    const wrapper = mount(LoadingOverlay, {
      attachTo: document.body,
      global: {
        stubs: {
          Transition: { template: '<slot />' },
        },
      },
    });

    const event = new KeyboardEvent('keydown', { key: 'Escape', cancelable: true });
    window.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(false);

    wrapper.unmount();
  });
});
