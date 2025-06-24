import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import AgentList from './AgentList.vue';

// Mock the $t function
const mockT = (key: string) => {
  const translations: Record<string, string> = {
    'admin.settings.agents.never': 'Never',
    'admin.settings.agents.edit_agent': 'Edit Agent',
    'admin.settings.agents.delete_agent': 'Delete Agent',
    'admin.settings.agents.org.badge': 'Org',
    'admin.settings.agents.type.badge': 'Type',
    'admin.settings.agents.custom_labels.badge': 'Label',
    'admin.settings.agents.platform.badge': 'Platform',
    'admin.settings.agents.backend.badge': 'Backend',
    'admin.settings.agents.capacity.badge': 'Capacity',
    'admin.settings.agents.none': 'No agents found',
  };
  return translations[key] || key;
};

// Mock Badge component
const MockBadge = {
  template: '<span>{{ label }}: {{ value }}</span>',
  props: ['label', 'value'],
};

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
  custom_labels: { foo: 'bar' },
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
      },
      global: {
        mocks: {
          $t: mockT,
        },
        stubs: {
          'router-link': true,
          Icon: true,
          IconButton: true,
          Badge: MockBadge,
        },
      },
    });
    // Check agent name
    expect(wrapper.text()).toContain('Test Agent');
    // Check running/capacity
    expect(wrapper.text()).toContain('2/3');
    // Check custom label
    expect(wrapper.text()).toContain('foo: bar');
    // Check left alignment by finding the div with text-left class
    const metaCol = wrapper.find('.text-left');
    expect(metaCol.exists()).toBe(true);
    // Check no px-4 on mobile by finding the grid div
    const infoRow = wrapper.find('.grid');
    expect(infoRow.exists()).toBe(true);
    expect(infoRow.classes()).not.toContain('px-4');
  });
});
