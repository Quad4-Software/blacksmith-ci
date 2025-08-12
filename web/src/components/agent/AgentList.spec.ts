import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import AgentList from './AgentList.vue';

// Create a simple i18n instance for testing
const i18n = createI18n({
  locale: 'en',
  messages: {
    en: {
      admin: {
        settings: {
          agents: {
            never: 'Never',
            edit_agent: 'Edit Agent',
            delete_agent: 'Delete Agent',
            org: { badge: 'Org' },
            type: { badge: 'Type' },
            custom_labels: { badge: 'Label' },
            platform: { badge: 'Platform' },
            backend: { badge: 'Backend' },
            capacity: { badge: 'Capacity' },
            none: 'No agents found',
            search: 'Search agents...',
            filter_type: 'All types',
            filter_platform: 'All platforms',
            filter_backend: 'All backends',
            clear_filters: 'Clear filters',
            no_match: 'No agents match your filters',
          },
        },
      },
    },
  },
});

// Mock the $t function for template usage
const mockT = (key: string) => key;

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
        plugins: [i18n],
        mocks: {
          $t: mockT,
        },
        stubs: {
          'router-link': true,
          Icon: { template: '<i></i>' },
          IconButton: { template: '<button></button>' },
          Badge: MockBadge,
          TextField: { template: '<input />' },
          SelectField: { template: '<select></select>' },
          Button: { template: '<button></button>' },
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
