import { flushPromises, mount } from '@vue/test-utils';
import { describe, expect, it, vi } from 'vitest';

import type { Agent } from '~/lib/api/types';

import AgentRestartAction from './AgentRestartAction.vue';

const requestAgentRestart = vi.fn();
const cancelAgentRestart = vi.fn();
const notify = vi.fn();

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}));

vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({ requestAgentRestart, cancelAgentRestart }),
}));

vi.mock('~/compositions/useNotifications', () => ({
  default: () => ({ notify }),
}));

const baseAgent = {
  id: 7,
  remote_update: 'restart',
  restart_requested: false,
  restart_queued: false,
} as Agent;

function mountAction(agent: Partial<Agent> = {}) {
  return mount(AgentRestartAction, {
    props: { agent: { ...baseAgent, ...agent } },
    global: {
      stubs: {
        // Render the dialog inline so the confirm button can be clicked.
        AlertDialog: { template: '<div><slot /></div>' },
        AlertDialogContent: { template: '<div><slot /></div>' },
        AlertDialogHeader: { template: '<div><slot /></div>' },
        AlertDialogTitle: { template: '<div><slot /></div>' },
        AlertDialogDescription: { template: '<div><slot /></div>' },
        AlertDialogFooter: { template: '<div><slot /></div>' },
        AlertDialogCancel: { template: '<button><slot /></button>' },
        AlertDialogAction: { template: '<button v-bind="$attrs"><slot /></button>' },
      },
    },
  });
}

describe('agentRestartAction', () => {
  it('disables the request for agents that ignore restart requests', () => {
    const wrapper = mountAction({ remote_update: '' });
    const button = wrapper.get('[data-testid="agent-restart-request"]');
    expect(button.attributes('disabled')).toBeDefined();
    expect(wrapper.get('span[title]').attributes('title')).toBe('admin.settings.agents.restart.not_accepted');
  });

  it('requests a restart after confirmation and emits the updated agent', async () => {
    requestAgentRestart.mockResolvedValueOnce({ ...baseAgent, restart_requested: true });
    const wrapper = mountAction();

    await wrapper.get('[data-testid="agent-restart-request"]').trigger('click');
    await wrapper.get('[data-testid="agent-restart-confirm"]').trigger('click');
    await flushPromises();

    expect(requestAgentRestart).toHaveBeenCalledWith(7);
    expect(wrapper.emitted('updated')?.[0]?.[0]).toMatchObject({ restart_requested: true });
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'success' }));
  });

  it('offers to cancel a pending request', async () => {
    cancelAgentRestart.mockResolvedValueOnce({ ...baseAgent, restart_requested: false });
    const wrapper = mountAction({ restart_requested: true });

    expect(wrapper.find('[data-testid="agent-restart-request"]').exists()).toBe(false);
    await wrapper.get('[data-testid="agent-restart-cancel"]').trigger('click');
    await flushPromises();

    expect(cancelAgentRestart).toHaveBeenCalledWith(7);
    expect(wrapper.emitted('updated')?.[0]?.[0]).toMatchObject({ restart_requested: false });
  });

  it('offers to cancel a queued restart', () => {
    const wrapper = mountAction({ restart_queued: true });
    expect(wrapper.find('[data-testid="agent-restart-request"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="agent-restart-cancel"]').exists()).toBe(true);
  });
});
