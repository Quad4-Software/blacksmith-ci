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
    agentProtocol: { version: 21, min_agent: 20 },
  }),
}));

// Mock useTheme - must be mocked before importing components that use it
vi.mock('~/compositions/useTheme', () => ({
  useTheme: () => ({
    theme: ref('light'),
    storeTheme: ref('auto'),
    themeName: ref('blacksmith'),
    currentTheme: ref({ id: 'blacksmith', name: 'Blacksmith CI', light: {}, dark: {} }),
    availableThemes: ref([]),
  }),
}));

const sampleAgent = {
  id: 1,
  name: 'Test Agent',
  owner_id: 1,
  org_ids: [1],
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
  host_cpus: 0,
  host_mem: 0,
  protocol_version: 21,
  remote_update: '',
  restart_requested: false,
  restart_queued: false,
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

  describe('host resources', () => {
    it('renders the per-workflow limit against the host total', () => {
      const wrapper = mountWithAgent({
        ...sampleAgent,
        limit_mem: 4 * 1024 ** 3,
        limit_cpu_quota: 200000,
        host_cpus: 16,
        host_mem: 64 * 1024 ** 3,
      });
      expect(wrapper.text()).toContain('4 GB / 64 GB');
      expect(wrapper.text()).toContain('2 / 16 admin.settings.agents.cores');
    });

    it('renders host totals for a backend without limits', () => {
      const wrapper = mountWithAgent({
        ...sampleAgent,
        backend: 'local',
        host_cpus: 8,
        host_mem: 16 * 1024 ** 3,
      });
      expect(wrapper.text()).toContain('16 GB');
      expect(wrapper.text()).toContain('8 admin.settings.agents.cores');
    });

    it('omits the resource section when nothing is known', () => {
      const wrapper = mountWithAgent({ ...sampleAgent, backend: 'kubernetes' });
      expect(wrapper.text()).not.toContain('admin.settings.agents.resource_mem');
    });
  });

  describe('version compatibility', () => {
    function compatibility(wrapper: ReturnType<typeof mountWithAgent>) {
      return wrapper.find('[data-testid="agent-compatibility"]');
    }

    it('shows nothing for an agent on the server release', () => {
      expect(compatibility(mountWithAgent(sampleAgent)).exists()).toBe(false);
    });

    it('flags an agent on an older protocol as outdated', () => {
      const icon = compatibility(mountWithAgent({ ...sampleAgent, version: '0.9.0', protocol_version: 20 }));
      expect(icon.attributes('data-compatibility')).toBe('outdated');
      expect(icon.attributes('title')).toBe('admin.settings.agents.version.outdated');
    });

    it('flags an agent below the server minimum as incompatible', () => {
      const icon = compatibility(mountWithAgent({ ...sampleAgent, version: '0.8.0', protocol_version: 19 }));
      expect(icon.attributes('data-compatibility')).toBe('incompatible');
    });

    it('shows a pending restart', () => {
      expect(mountWithAgent(sampleAgent).find('[data-testid="agent-restart-pending"]').exists()).toBe(false);
      const wrapper = mountWithAgent({ ...sampleAgent, restart_requested: true });
      expect(wrapper.find('[data-testid="agent-restart-pending"]').exists()).toBe(true);
    });

    it('shows a queued restart', () => {
      expect(mountWithAgent(sampleAgent).find('[data-testid="agent-restart-queued"]').exists()).toBe(false);
      const wrapper = mountWithAgent({ ...sampleAgent, restart_queued: true });
      expect(wrapper.find('[data-testid="agent-restart-queued"]').exists()).toBe(true);
    });

    it('marks a different release on the same protocol as informational', () => {
      const icon = compatibility(mountWithAgent({ ...sampleAgent, version: '1.0.1' }));
      expect(icon.attributes('data-compatibility')).toBe('release_differs');
    });
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

  describe('instance roles', () => {
    // Agents.vue passes is-admin = can('agents:write'), which an auditor lacks and an operator holds.
    it('hides edit/delete buttons for an auditor on agents outside their org', () => {
      const global = mountWithAgent({ ...sampleAgent, org_ids: [] }, { isAdmin: false, userOrgId: 1 });
      const otherOrg = mountWithAgent({ ...sampleAgent, org_ids: [42] }, { isAdmin: false, userOrgId: 1 });
      expect(hasManagementButtons(global)).toBe(false);
      expect(hasManagementButtons(otherOrg)).toBe(false);
    });

    it('shows edit/delete buttons for an operator on every agent', () => {
      const global = mountWithAgent({ ...sampleAgent, org_ids: [] }, { isAdmin: true, userOrgId: 1 });
      const otherOrg = mountWithAgent({ ...sampleAgent, org_ids: [42] }, { isAdmin: true, userOrgId: 1 });
      expect(hasManagementButtons(global)).toBe(true);
      expect(hasManagementButtons(otherOrg)).toBe(true);
    });
  });

  describe('missing org_ids', () => {
    // Servers predating the empty-array normalization send org_ids: null for
    // global agents, which used to throw on `.length` and blank the whole list.
    const agentWithoutOrgIDs = { ...sampleAgent, org_ids: null as unknown as number[] };

    it('renders a global agent whose org_ids is null', () => {
      const wrapper = mountWithAgent(agentWithoutOrgIDs, { isAdmin: true, userOrgId: 1 });
      expect(wrapper.text()).toContain('Test Agent');
      expect(wrapper.text()).toContain('admin.settings.agents.type.global');
    });

    it('hides edit/delete buttons for a regular user when org_ids is null', () => {
      const wrapper = mountWithAgent(agentWithoutOrgIDs, { isAdmin: false, userOrgId: 1 });
      expect(hasManagementButtons(wrapper)).toBe(false);
    });
  });
});
