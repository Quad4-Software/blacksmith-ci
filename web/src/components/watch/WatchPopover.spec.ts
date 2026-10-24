import { flushPromises, mount } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { defineComponent } from 'vue';

import type { Pipeline } from '~/lib/api/types';

let smtpConfigured = true;

const getRepoWatches = vi.fn();
const createRepoWatch = vi.fn();
const updateWatch = vi.fn();
const deleteWatch = vi.fn();
const load = vi.fn();

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}));
vi.mock('~/compositions/useConfig', () => ({
  default: () => ({ smtpConfigured }),
}));
vi.mock('~/store/auth', () => ({
  useAuthStore: () => ({ user: { email: 'a@b.c' } }),
}));
vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({ getRepoWatches, createRepoWatch, updateWatch, deleteWatch }),
}));
vi.mock('~/compositions/useWatches', () => ({
  useWatches: () => ({ load }),
}));
vi.mock('~/compositions/useNotifications', () => ({
  default: () => ({ notify: vi.fn() }),
}));

const passthrough = defineComponent({ template: '<div><slot /></div>' });
const { default: WatchPopover } = await import('./WatchPopover.vue');

function pipeline(p: Partial<Pipeline>): Pipeline {
  return { event: 'push', branch: 'main', sender: 'bob', deploy_to: '', ...p } as Pipeline;
}

function mountPopover(p: Pipeline) {
  return mount(WatchPopover, {
    props: { repoId: 7, pipeline: p },
    global: {
      mocks: { $t: (k: string) => k },
      stubs: {
        Popover: passthrough,
        PopoverTrigger: passthrough,
        PopoverContent: passthrough,
        Icon: true,
      },
    },
  });
}

function existing(settings: Record<string, unknown> = {}) {
  return {
    watch: {
      id: 42,
      repo_id: 7,
      event: 'push',
      target: 'main',
      in_app: true,
      email: false,
      email_events: ['failure'],
      ...settings,
    },
    repo_full_name: 'o/r',
    failing: false,
    unread: false,
    failing_workflows: [],
  };
}

describe('watchPopover', () => {
  beforeEach(() => {
    smtpConfigured = true;
    for (const m of [getRepoWatches, createRepoWatch, updateWatch, deleteWatch, load]) m.mockReset();
    getRepoWatches.mockResolvedValue([]);
    createRepoWatch.mockResolvedValue({ id: 1 });
    deleteWatch.mockResolvedValue(undefined);
  });

  it('renders nothing for a pull_request pipeline', async () => {
    const w = mountPopover(pipeline({ event: 'pull_request' as Pipeline['event'] }));
    await flushPromises();
    expect(w.find('[data-testid="watch-popover-trigger"]').exists()).toBe(false);
    expect(getRepoWatches).not.toHaveBeenCalled();
  });

  it('creates a watch with the defaults', async () => {
    const w = mountPopover(pipeline({}));
    await flushPromises();
    expect(w.get('[data-testid="watch-popover-trigger"]').text()).toContain('repo.watch.watch');
    await w.get('[data-testid="watch-save"]').trigger('click');
    await flushPromises();
    expect(createRepoWatch).toHaveBeenCalledWith(7, {
      event: 'push',
      target: 'main',
      in_app: true,
      email: false,
      email_events: ['failure'],
    });
    expect(load).toHaveBeenCalled();
    expect(w.find('[data-testid="watch-unwatch"]').exists()).toBe(true);
  });

  it('shows Watching for an existing watch and unwatches it', async () => {
    getRepoWatches.mockResolvedValue([existing(), existing({ id: 9, target: 'dev' })]);
    const w = mountPopover(pipeline({}));
    await flushPromises();
    expect(w.get('[data-testid="watch-popover-trigger"]').text()).toContain('repo.watch.watching');
    await w.get('[data-testid="watch-unwatch"]').trigger('click');
    await flushPromises();
    expect(deleteWatch).toHaveBeenCalledWith(42);
    expect(w.find('[data-testid="watch-unwatch"]').exists()).toBe(false);
  });

  it('updates an existing watch instead of creating', async () => {
    getRepoWatches.mockResolvedValue([existing()]);
    updateWatch.mockResolvedValue({ id: 42 });
    const w = mountPopover(pipeline({}));
    await flushPromises();
    await w.get('[data-testid="watch-save"]').trigger('click');
    await flushPromises();
    expect(updateWatch).toHaveBeenCalledWith(42, { in_app: true, email: false, email_events: ['failure'] });
    expect(createRepoWatch).not.toHaveBeenCalled();
  });

  it('disables the email checkbox when smtp is not configured', async () => {
    smtpConfigured = false;
    const w = mountPopover(pipeline({}));
    await flushPromises();
    expect(w.get('[data-testid="watch-email"]').attributes('disabled')).toBeDefined();
  });

  it('disables Save when both channels are off', async () => {
    getRepoWatches.mockResolvedValue([existing({ in_app: false, email: false })]);
    const w = mountPopover(pipeline({}));
    await flushPromises();
    expect(w.get('[data-testid="watch-save"]').attributes('disabled')).toBeDefined();
  });
});
