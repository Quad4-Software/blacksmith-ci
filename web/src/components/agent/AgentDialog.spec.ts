import { flushPromises, mount } from '@vue/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Agent } from '~/lib/api/types';

import AgentDialog from './AgentDialog.vue';

const permissions = new Set<string>();

vi.mock('~/compositions/usePermissions', () => ({
  default: () => ({ can: (p: string) => permissions.has(p) }),
}));

vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({ getOrgs: async () => [{ id: 5, name: 'acme' }] }),
}));

vi.mock('~/compositions/useDate', () => ({
  useDate: () => ({ timeAgo: () => 'Never' }),
}));

const agent = {
  id: 1,
  name: 'runner',
  org_ids: [5],
  no_schedule: false,
  priority: 0,
  token: '',
  last_contact: 0,
} as unknown as Agent;

function mountDialog() {
  return mount(AgentDialog, { props: { open: true, agent, scopeEditable: true }, attachTo: document.body });
}

describe('agentDialog organization scope', () => {
  beforeEach(() => {
    permissions.clear();
  });

  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('shows the organization scope with agents:register', async () => {
    permissions.add('agents:register');
    const wrapper = mountDialog();
    await flushPromises();

    expect(document.body.textContent).toContain('admin.settings.agents.scope.label');

    wrapper.unmount();
  });

  it('hides the organization scope without agents:register and saves the current organizations', async () => {
    permissions.add('agents:write');
    const wrapper = mountDialog();
    await flushPromises();

    expect(document.body.textContent).not.toContain('admin.settings.agents.scope.label');

    document.querySelector<HTMLButtonElement>('button[type="submit"]')?.click();
    await flushPromises();

    expect(wrapper.emitted('save')?.[0]?.[0]).toMatchObject({ id: 1, org_ids: [5] });

    wrapper.unmount();
  });
});
