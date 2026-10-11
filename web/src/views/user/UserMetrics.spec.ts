import { flushPromises, shallowMount } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import UserMetrics from './UserMetrics.vue';

const getMetrics = vi.fn();
let mockUser: { admin: boolean; permissions: string[] } = { admin: false, permissions: [] };

vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({ getActiveRepos: vi.fn().mockResolvedValue([]), getMetrics }),
}));

vi.mock('~/compositions/useAuthentication', () => ({
  default: () => ({ user: mockUser }),
}));

vi.mock('~/compositions/useCrowTitle', () => ({ useCrowTitle: () => {} }));

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace: vi.fn() }),
}));

async function mountMetrics() {
  const wrapper = shallowMount(UserMetrics, {
    // Render the header slot, where the instance-wide toggle lives.
    global: { stubs: { Settings: { template: '<div><slot name="headerActions" /><slot /></div>' } } },
  });
  await flushPromises();
  return wrapper;
}

describe('userMetrics instance-wide view', () => {
  beforeEach(() => {
    getMetrics.mockReset().mockResolvedValue({});
    // The instance-wide toggle was left on from an earlier visit.
    localStorage.setItem('crow:metrics-show-all', 'true');
  });

  it('shows the toggle and requests all metrics with metrics:all', async () => {
    mockUser = { admin: false, permissions: ['metrics:all'] };
    const wrapper = await mountMetrics();
    expect(wrapper.find('#admin-scope').exists()).toBe(true);
    expect(getMetrics).toHaveBeenCalledWith(expect.objectContaining({ all: true }));
  });

  it('hides the toggle and never requests all metrics without metrics:all', async () => {
    mockUser = { admin: false, permissions: [] };
    const wrapper = await mountMetrics();
    expect(wrapper.find('#admin-scope').exists()).toBe(false);
    expect(getMetrics).not.toHaveBeenCalledWith(expect.objectContaining({ all: true }));
  });
});
