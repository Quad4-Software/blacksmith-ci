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
  org_ids: [1],
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

function mountWithAgent(agent: typeof sampleAgent, opts: { isAdmin?: boolean; userOrgId?: number } = {}) {
  return mount(AgentList, {
    props: {
      agents: [agent],
      isDeleting: false,
      loading: false,
      isAdmin: opts.isAdmin,
      userOrgId: opts.userOrgId,
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
}

function hasManagementButtons(wrapper: ReturnType<typeof mountWithAgent>) {
  return (
    wrapper.find('button[title="admin.settings.agents.edit_agent"]').exists() &&
    wrapper.find('button[title="admin.settings.agents.delete_agent"]').exists()
  );
}

describe('agentList', () => {
  it('renders agent metadata and aligns columns', () => {
    const wrapper = mountWithAgent(sampleAgent, { isAdmin: true, userOrgId: 1 });
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

  describe('management button visibility', () => {
    it('shows edit/delete buttons for admin on a global agent', () => {
      const wrapper = mountWithAgent({ ...sampleAgent, org_ids: [] }, { isAdmin: true });
      expect(hasManagementButtons(wrapper)).toBe(true);
    });

    it('shows edit/delete buttons for admin on a user/org agent in another org', () => {
      const wrapper = mountWithAgent({ ...sampleAgent, org_ids: [42] }, { isAdmin: true, userOrgId: 1 });
      expect(hasManagementButtons(wrapper)).toBe(true);
    });

    it('shows edit/delete buttons for a regular user on their own org agent', () => {
      const wrapper = mountWithAgent({ ...sampleAgent, org_ids: [1] }, { isAdmin: false, userOrgId: 1 });
      expect(hasManagementButtons(wrapper)).toBe(true);
    });

    it('hides edit/delete buttons for a regular user on a global agent', () => {
      const wrapper = mountWithAgent({ ...sampleAgent, org_ids: [] }, { isAdmin: false, userOrgId: 1 });
      expect(hasManagementButtons(wrapper)).toBe(false);
    });

    it("hides edit/delete buttons for a regular user on another org's agent", () => {
      const wrapper = mountWithAgent({ ...sampleAgent, org_ids: [42] }, { isAdmin: false, userOrgId: 1 });
      expect(hasManagementButtons(wrapper)).toBe(false);
    });
  });
});
