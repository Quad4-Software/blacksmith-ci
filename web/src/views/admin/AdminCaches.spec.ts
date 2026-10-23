import { flushPromises, mount } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { AdminCaches as AdminCachesResponse, BuildCacheOrgSummary } from '~/lib/api/types';

import AdminCaches from './AdminCaches.vue';

const getAdminCaches = vi.fn();
const runAdminCacheCleanup = vi.fn();
const notify = vi.fn();

vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({ getAdminCaches, runAdminCacheCleanup }),
}));

vi.mock('~/compositions/usePermissions', () => ({
  default: () => ({ can: (p: string) => p === 'maintenance:write' || p === 'maintenance:read' }),
}));

vi.mock('~/compositions/useNotifications', () => ({
  default: () => ({ notify }),
}));

vi.mock('~/compositions/useCrowTitle', () => ({ useCrowTitle: () => {} }));

const GiB = 1024 ** 3;
const usage = { hits: 3, partial_hits: 0, misses: 1, denied: 0, saves: 1, egress: GiB, ingress: GiB };

function org(overrides: Partial<BuildCacheOrgSummary>): BuildCacheOrgSummary {
  return {
    org_id: 1,
    name: 'acme',
    is_user: false,
    size: GiB,
    entries: 2,
    repos: 1,
    bucket_size: 0,
    bucket_entries: 0,
    usage,
    egress_month: GiB,
    bucket_egress_month: 0,
    settings: { org_id: 1, max_size: 0, max_egress: 0, ttl: 0, updated: 0 },
    limits: { max_size: 2 * GiB, max_egress: 10 * GiB, ttl: 604800 },
    ...overrides,
  };
}

const response: AdminCachesResponse = {
  enabled: true,
  overview: {
    defaults: { max_size: 2 * GiB, max_egress: 10 * GiB, ttl: 604800 },
    max_repo_size: 10 * GiB,
    max_total_size: 0,
    allow_buckets: true,
    size: 2 * GiB,
    entries: 3,
    bucket_size: 0,
    bucket_entries: 0,
    usage,
    egress_month: 12 * GiB,
    orgs: [
      org({}),
      org({
        org_id: 2,
        name: 'jdoe',
        is_user: true,
        egress_month: 12 * GiB,
        settings: { org_id: 2, max_size: 0, max_egress: 0, ttl: 3600, updated: 1 },
        limits: { max_size: 2 * GiB, max_egress: 10 * GiB, ttl: 3600 },
      }),
    ],
  },
};

async function mountCaches() {
  const wrapper = mount(AdminCaches, {
    global: {
      stubs: { Settings: { template: '<div><slot name="headerActions" /><slot /></div>' }, CacheDailyChart: true },
    },
  });
  await flushPromises();
  return wrapper;
}

describe('admin caches', () => {
  beforeEach(() => {
    getAdminCaches.mockReset();
    runAdminCacheCleanup.mockReset();
    notify.mockReset();
  });

  it('explains that caching is disabled', async () => {
    getAdminCaches.mockResolvedValue({ enabled: false });
    const wrapper = await mountCaches();
    expect(wrapper.find('[data-testid="admin-caches-disabled"]').exists()).toBe(true);
  });

  it('lists every owner with its hit rate, retention and custom limits', async () => {
    getAdminCaches.mockResolvedValue(response);
    const wrapper = await mountCaches();

    const rows = wrapper.findAll('[data-testid="cache-org-row"]');
    expect(rows).toHaveLength(2);
    expect(rows[0].text()).toContain('acme');
    expect(rows[0].text()).toContain('75%');
    expect(rows[0].text()).toContain('7d');
    expect(rows[1].text()).toContain('caches.admin.custom_limits');
    expect(rows[1].text()).toContain('1h');
    expect(rows[1].find('.text-destructive').text()).toContain('12 GB');
  });

  it('runs the cleanup and reports what it removed', async () => {
    getAdminCaches.mockResolvedValue(response);
    runAdminCacheCleanup.mockResolvedValue({ removed: 4, freed: GiB });
    const wrapper = await mountCaches();

    await wrapper.get('[data-testid="run-cache-cleanup"]').trigger('click');
    await flushPromises();

    expect(runAdminCacheCleanup).toHaveBeenCalled();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'success' }));
    expect(getAdminCaches).toHaveBeenCalledTimes(2);
  });

  it('offers no limits or purge for repositories without an owner', async () => {
    const overview = response.overview!;
    getAdminCaches.mockResolvedValue({ ...response, overview: { ...overview, orgs: [org({ org_id: 0, name: '' })] } });
    const wrapper = await mountCaches();

    const row = wrapper.get('[data-testid="cache-org-row"]');
    expect(row.find('[data-testid="edit-cache-limits"]').exists()).toBe(false);
    expect(row.findAll('button')).toHaveLength(1);
  });
});
