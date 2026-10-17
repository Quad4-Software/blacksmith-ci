import { flushPromises, mount } from '@vue/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';

import type { RepoCaches } from '~/lib/api/types';

import Caches from './Caches.vue';

const getRepoCaches = vi.fn();
const deleteRepoCache = vi.fn();
const deleteRepoCaches = vi.fn();
const notify = vi.fn();

vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({ getRepoCaches, deleteRepoCache, deleteRepoCaches }),
}));

vi.mock('~/compositions/useInjectProvide', () => ({
  requiredInject: () => ref({ id: 3, full_name: 'owner/repo' }),
}));

vi.mock('~/compositions/useNotifications', () => ({
  default: () => ({ notify }),
}));

vi.mock('~/compositions/useCrowTitle', () => ({ useCrowTitle: () => {} }));

const NOW = 1760000600;

const populated: RepoCaches = {
  enabled: true,
  size: 3 * 1024 * 1024,
  entries: [
    {
      id: 1,
      repo_id: 3,
      scope: 'refs/heads/main',
      key: 'go-abc123',
      size: 2 * 1024 * 1024,
      committed: true,
      created: NOW - 600,
      last_used: NOW - 100,
    },
    {
      id: 2,
      repo_id: 3,
      scope: 'refs/pull/12/head',
      key: 'node-def456',
      size: 1024 * 1024,
      committed: false,
      created: NOW - 50,
      last_used: NOW - 50,
    },
  ],
};

async function mountCaches() {
  const wrapper = mount(Caches, {
    attachTo: document.body,
    // Render the header slot, where the delete-all action lives.
    global: { stubs: { Settings: { template: '<div><slot name="headerActions" /><slot /></div>' } } },
  });
  await flushPromises();
  return wrapper;
}

// AlertDialog content is teleported to the body, so it is looked up there.
function findInBody(selector: string): HTMLElement | null {
  return document.body.querySelector<HTMLElement>(selector);
}

describe('repo settings caches', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(NOW * 1000);
    getRepoCaches.mockReset();
    deleteRepoCache.mockReset().mockResolvedValue(undefined);
    deleteRepoCaches.mockReset().mockResolvedValue(undefined);
    notify.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
    document.body.innerHTML = '';
  });

  it('shows a loading state until the caches are loaded', async () => {
    let resolveCaches: (value: RepoCaches) => void = () => {};
    getRepoCaches.mockReturnValue(
      new Promise<RepoCaches>((resolve) => {
        resolveCaches = resolve;
      }),
    );
    const wrapper = mount(Caches, {
      global: { stubs: { Settings: { template: '<div><slot name="headerActions" /><slot /></div>' } } },
    });
    await flushPromises();
    expect(wrapper.find('[data-testid="caches-loading"]').exists()).toBe(true);

    resolveCaches({ enabled: true, size: 0, entries: [] });
    await flushPromises();
    expect(wrapper.find('[data-testid="caches-loading"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="caches-empty"]').exists()).toBe(true);
  });

  it('explains that caching is disabled on the server', async () => {
    getRepoCaches.mockResolvedValue({ enabled: false, size: 0, entries: [] });
    const wrapper = await mountCaches();
    expect(wrapper.find('[data-testid="caches-disabled"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="delete-all-caches"]').exists()).toBe(false);
  });

  it('treats a null entry list as empty', async () => {
    getRepoCaches.mockResolvedValue({ enabled: true, size: 0, entries: null });
    const wrapper = await mountCaches();
    expect(wrapper.find('[data-testid="caches-empty"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="delete-all-caches"]').exists()).toBe(false);
  });

  it('lists entries with readable scope, size and a pending badge', async () => {
    getRepoCaches.mockResolvedValue(populated);
    const wrapper = await mountCaches();

    const rows = wrapper.findAll('[data-testid="cache-entry"]');
    expect(rows).toHaveLength(2);

    expect(rows[0].text()).toContain('go-abc123');
    expect(rows[0].text()).toContain('main');
    expect(rows[0].text()).not.toContain('refs/heads/');
    expect(rows[0].text()).toContain('2 MB');
    expect(rows[0].text()).not.toContain('repo.settings.caches.pending');

    expect(rows[1].text()).toContain('#12');
    expect(rows[1].text()).toContain('repo.settings.caches.pending');
  });

  it('deletes all caches after confirmation and reloads', async () => {
    getRepoCaches.mockResolvedValueOnce(populated).mockResolvedValueOnce({ enabled: true, size: 0, entries: [] });
    const wrapper = await mountCaches();

    await wrapper.find('[data-testid="delete-all-caches"]').trigger('click');
    await flushPromises();
    expect(deleteRepoCaches).not.toHaveBeenCalled();

    findInBody('[data-testid="confirm-delete-all-caches"]')!.click();
    await flushPromises();

    expect(deleteRepoCaches).toHaveBeenCalledWith(3);
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'success' }));
    expect(getRepoCaches).toHaveBeenCalledTimes(2);
    expect(wrapper.find('[data-testid="caches-empty"]').exists()).toBe(true);
  });
});
