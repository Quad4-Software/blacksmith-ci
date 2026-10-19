import { flushPromises, mount } from '@vue/test-utils';
import { describe, expect, it, vi } from 'vitest';

import AgentRestartAllAction from './AgentRestartAllAction.vue';

const queueAgentRestarts = vi.fn();
const notify = vi.fn();
const t = vi.fn((key: string, arg?: unknown) => (arg === undefined ? key : `${key}:${JSON.stringify(arg)}`));

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t }),
}));

vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({ queueAgentRestarts }),
}));

vi.mock('~/compositions/useNotifications', () => ({
  default: () => ({ notify }),
}));

vi.mock('~/compositions/useConfig', () => ({
  default: () => ({ version: '6.12.0', agentAutoRestart: { policy: 'off', max_parallel: 2 } }),
}));

function mountAction() {
  return mount(AgentRestartAllAction, {
    global: {
      mocks: { $t: t },
      stubs: {
        // Render the dialog inline so the confirm button can be clicked.
        AlertDialog: { template: '<div><slot /></div>' },
        AlertDialogContent: { template: '<div><slot /></div>' },
        AlertDialogHeader: { template: '<div><slot /></div>' },
        AlertDialogTitle: { template: '<div><slot /></div>' },
        AlertDialogDescription: { template: '<div data-testid="description"><slot /></div>' },
        AlertDialogFooter: { template: '<div><slot /></div>' },
        AlertDialogCancel: { template: '<button><slot /></button>' },
        AlertDialogAction: { template: '<button v-bind="$attrs"><slot /></button>' },
      },
    },
  });
}

describe('agentRestartAllAction', () => {
  it('names the server release and the parallel limit in the confirmation', () => {
    const wrapper = mountAction();
    expect(wrapper.get('[data-testid="description"]').text()).toBe(
      'admin.settings.agents.restart.all_confirm_text:{"version":"6.12.0","count":2}',
    );
  });

  it('queues the restarts after confirmation and reports how many', async () => {
    queueAgentRestarts.mockResolvedValueOnce({ queued: 3 });
    const wrapper = mountAction();

    await wrapper.get('[data-testid="agent-restart-all"]').trigger('click');
    await wrapper.get('[data-testid="agent-restart-all-confirm"]').trigger('click');
    await flushPromises();

    expect(queueAgentRestarts).toHaveBeenCalledOnce();
    expect(wrapper.emitted('queued')?.[0]).toEqual([3]);
    expect(notify).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'success', text: 'admin.settings.agents.restart.all_queued:3' }),
    );
  });
});
