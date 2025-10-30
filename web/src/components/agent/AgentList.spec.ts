import { describe, it, expect, vi } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import AgentList from './AgentList.vue';

// Mock vue-i18n
vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}));

// Mock compositions
vi.mock('~/compositions/useDate', () => ({
  useDate: () => ({
    timeAgo: (_timestamp: number) => 'Never',
  }),
}));

vi.mock('~/compositions/useConfig', () => ({
  default: () => ({
    version: '1.0.0',
  }),
}));

const sampleAgent = {
  id: 1,
  name: 'Test Agent',
  owner_id: 1,
  org_id: 1,
  token: '',
  created: 0,
  updated: 0,
  last_contact: 0,
  platform: 'linux',
  backend: 'docker',
  capacity: 3,
  version: '1.0.0',
  no_schedule: false,
  priority: 0,
  custom_labels: { foo: 'bar' },
  running_workflows: 2,
};

describe('agentList', () => {
  it('renders agent metadata and aligns columns', () => {
    const wrapper = shallowMount(AgentList, {
      props: {
        agents: [sampleAgent],
        isDeleting: false,
        loading: false,
        isAdmin: true,
        userOrgId: 1,
        agentFilters: {
          search: '',
          platform: '',
          backend: '',
          sort: 'status',
          order: 'asc',
        },
        currentPage: 1,
        hasMore: false,
        isJumpingToLast: false,
      },
    });
    // Check agent name
    expect(wrapper.text()).toContain('Test Agent');
    // Check running/capacity (note: there are spaces around the /)
    expect(wrapper.text()).toContain('2 / 3');
    // Check custom label (rendered as "foo:bar" without space due to margin-based spacing)
    expect(wrapper.text()).toContain('foo:bar');
    // Check that the info grid exists
    const infoGrid = wrapper.find('.grid');
    expect(infoGrid.exists()).toBe(true);
  });
});
