import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { ref } from 'vue';
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

// Mock useTheme - must be mocked before importing components that use it
vi.mock('~/compositions/useTheme', () => ({
  useTheme: () => ({
    theme: ref('light'),
    storeTheme: ref('auto'),
    themeName: ref('crow-ci'),
    currentTheme: ref({ id: 'crow-ci', name: 'Crow CI', light: {}, dark: {} }),
    availableThemes: ref([]),
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
  workflow_polling_healthy: true,
  platform: 'linux',
  backend: 'docker',
  capacity: 3,
  version: '1.0.0',
  build_date: '2024-01-15',
  no_schedule: false,
  priority: 0,
  custom_labels: { foo: 'bar' },
  limit_mem: 0,
  limit_cpu_quota: 0,
  running_workflows: 2,
};

describe('agentList', () => {
  it('renders agent metadata and aligns columns', () => {
    const wrapper = mount(AgentList, {
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
