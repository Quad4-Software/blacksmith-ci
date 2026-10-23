import { flushPromises, mount } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';

import type { BuildCacheOrgDetail, OrgCaches as OrgCachesResponse } from '~/lib/api/types';

import OrgCaches from './OrgCaches.vue';

const getOrgCaches = vi.fn();
const putOrgCacheBucket = vi.fn();
const deleteOrgCacheBucket = vi.fn();
const deleteOrgCaches = vi.fn();
const notify = vi.fn();

vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({ getOrgCaches, putOrgCacheBucket, deleteOrgCacheBucket, deleteOrgCaches }),
}));

vi.mock('~/compositions/useInjectProvide', () => ({
  requiredInject: () => ref({ id: 7, name: 'acme' }),
}));

vi.mock('~/compositions/useNotifications', () => ({
  default: () => ({ notify }),
}));

vi.mock('~/compositions/useCrowTitle', () => ({ useCrowTitle: () => {} }));

const GiB = 1024 ** 3;
const counts = { hits: 6, partial_hits: 2, misses: 1, denied: 1, saves: 3, egress: 5 * GiB, ingress: GiB };

const detail: BuildCacheOrgDetail = {
  org_id: 7,
  name: 'acme',
  is_user: false,
  size: GiB,
  entries: 4,
  repos: 2,
  bucket_size: 0,
  bucket_entries: 0,
  usage: counts,
  egress_month: 15 * GiB,
  bucket_egress_month: 0,
  settings: { org_id: 7, max_size: 0, max_egress: 0, ttl: 0, updated: 0 },
  limits: { max_size: 2 * GiB, max_egress: 10 * GiB, ttl: 604800 },
  repo_list: [{ repo_id: 1, full_name: 'acme/web', size: GiB, entries: 4, usage: counts }],
  daily: [],
};

const enabled: OrgCachesResponse = {
  enabled: true,
  allow_buckets: true,
  defaults: { max_size: 2 * GiB, max_egress: 10 * GiB, ttl: 604800 },
  detail,
};

async function mountCaches() {
  const wrapper = mount(OrgCaches, {
    global: {
      stubs: {
        Settings: { template: '<div><slot name="headerActions" /><slot /></div>' },
        CacheDailyChart: true,
      },
    },
  });
  await flushPromises();
  return wrapper;
}

describe('org settings caches', () => {
  beforeEach(() => {
    getOrgCaches.mockReset();
    putOrgCacheBucket.mockReset();
    notify.mockReset();
  });

  it('explains that caching is disabled', async () => {
    getOrgCaches.mockResolvedValue({ enabled: false, allow_buckets: false, defaults: detail.limits });
    const wrapper = await mountCaches();
    expect(wrapper.find('[data-testid="org-caches-disabled"]').exists()).toBe(true);
  });

  it('shows the hit rate, the egress over its limit and the repositories', async () => {
    getOrgCaches.mockResolvedValue(enabled);
    const wrapper = await mountCaches();

    expect(wrapper.get('[data-testid="cache-hit-rate"]').text()).toBe('80%');
    // 15 of 10 GiB egress: the meter turns red.
    expect(wrapper.get('[data-testid="cache-usage-tiles"]').find('.bg-destructive').exists()).toBe(true);
    expect(wrapper.get('[data-testid="cache-repo-table"]').text()).toContain('acme/web');
    expect(wrapper.get('[data-testid="org-cache-retention"]').text()).toContain('caches.org.limits_by_admin');
  });

  it('keeps the reason a bucket could not be connected next to the form', async () => {
    getOrgCaches.mockResolvedValue(enabled);
    putOrgCacheBucket.mockRejectedValue(new Error('invalid cache bucket: access denied'));
    const wrapper = await mountCaches();

    await wrapper.get('#bucket-endpoint').setValue('s3.example.com');
    await wrapper.get('#bucket-name').setValue('cache');
    await wrapper.get('form').trigger('submit');
    await flushPromises();

    expect(putOrgCacheBucket).toHaveBeenCalledWith(
      7,
      expect.objectContaining({ endpoint: 's3.example.com', bucket: 'cache', ssl: true }),
    );
    expect(wrapper.get('[data-testid="bucket-error"]').text()).toContain('access denied');
    expect(notify).not.toHaveBeenCalled();
  });

  it('connects a bucket and reloads', async () => {
    getOrgCaches.mockResolvedValue(enabled);
    putOrgCacheBucket.mockResolvedValue({});
    const wrapper = await mountCaches();

    await wrapper.get('#bucket-endpoint').setValue('s3.example.com');
    await wrapper.get('#bucket-name').setValue('cache');
    await wrapper.get('form').trigger('submit');
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'success' }));
    expect(getOrgCaches).toHaveBeenCalledTimes(2);
  });

  it('shows and keeps the limits the owner set for its bucket', async () => {
    getOrgCaches.mockResolvedValue({
      ...enabled,
      detail: {
        ...detail,
        bucket_size: GiB,
        bucket_egress_month: 6 * GiB,
        bucket: {
          id: 1,
          org_id: 7,
          endpoint: 's3.example.com',
          bucket: 'cache',
          region: '',
          prefix: '',
          access_key: 'key',
          ssl: true,
          path_style: false,
          max_size: 0,
          max_egress: 5 * GiB,
          ttl: -1,
          created: 0,
          updated: 0,
        },
      },
    });
    putOrgCacheBucket.mockResolvedValue({});
    const wrapper = await mountCaches();

    // 6 of the 5 GiB the owner allows per month.
    expect(wrapper.get('[data-testid="cache-usage-tiles"]').find('.bg-destructive').exists()).toBe(true);
    expect(wrapper.get('[data-testid="org-cache-retention"]').text()).toContain('caches.org.limits_by_owner');
    expect(wrapper.get('[data-testid="bucket-max-egress-amount"]').element).toHaveProperty('value', '5');

    await wrapper.get('form').trigger('submit');
    await flushPromises();
    expect(putOrgCacheBucket).toHaveBeenCalledWith(
      7,
      expect.objectContaining({ max_size: 0, max_egress: 5 * GiB, ttl: -1, secret_key: '' }),
    );
  });
});
