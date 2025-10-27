import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import AgentList from './AgentList.vue';

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
    const wrapper = mount(AgentList, {
      props: {
        agents: [sampleAgent],
        isDeleting: false,
        loading: false,
        isAdmin: true,
        userOrgId: 1,
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
