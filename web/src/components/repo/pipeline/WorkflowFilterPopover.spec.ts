import { flushPromises, mount } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Repo } from '~/lib/api/types';
import WorkflowFilterPopover from './WorkflowFilterPopover.vue';
import RunPipelineButton from './RunPipelineButton.vue';

const { getWorkflowNames, getRepoManualWorkflows, createPipeline, push, notify } = vi.hoisted(() => ({
  getWorkflowNames: vi.fn(),
  getRepoManualWorkflows: vi.fn(),
  createPipeline: vi.fn(),
  push: vi.fn(),
  notify: vi.fn(),
}));
vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({ getWorkflowNames, getRepoManualWorkflows, createPipeline }),
}));
vi.mock('~/compositions/useNotifications', () => ({ default: () => ({ notify }) }));
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }));
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }));
vi.mock('~/components/atomic/Icon.vue', () => ({ default: { template: '<span />' } }));

async function setup(canRun = true) {
  const wrapper = mount(RunPipelineButton, {
    props: { repo: { id: 1, default_branch: 'main' } as Repo, branch: 'feature', canRun },
    global: {
      stubs: {
        Popover: { template: '<div><slot /></div>' },
        PopoverTrigger: { template: '<div><slot /></div>' },
        PopoverContent: { template: '<div><slot /></div>' },
      },
    },
  });
  await flushPromises();
  return wrapper;
}

beforeEach(() => {
  vi.clearAllMocks();
  getWorkflowNames.mockResolvedValue(['lint']);
  getRepoManualWorkflows.mockResolvedValue({
    workflows: [{ name: 'lint', variables: [{ name: 'MODE', default: 'check' }] }],
  });
  createPipeline.mockResolvedValue({ number: 42 });
});

describe('separate workflow filters and quick runs', () => {
  it('runs only the clicked workflow on the selected branch with defaults without changing filters', async () => {
    const wrapper = await setup();
    await wrapper.get('button[aria-label="repo.manual_pipeline.run_workflow"]').trigger('click');
    await flushPromises();
    expect(createPipeline).toHaveBeenCalledWith(1, {
      branch: 'feature',
      workflows: ['lint'],
      variables: { MODE: 'check' },
    });
    expect(push).toHaveBeenCalledWith({ name: 'repo-pipeline', params: { repoId: 1, pipelineId: 42 } });
    expect(wrapper.emitted('update:modelValue')).toBeUndefined();
  });

  it('hides all run controls for read-only users', async () => {
    const wrapper = await setup(false);
    expect(wrapper.find('button').exists()).toBe(false);
    expect(createPipeline).not.toHaveBeenCalled();
  });

  it('opens the full configuration without starting a workflow', async () => {
    const wrapper = await setup();
    await wrapper.findAll('button')[0].trigger('click');
    expect(wrapper.emitted('configure')).toHaveLength(1);
    expect(createPipeline).not.toHaveBeenCalled();
  });

  it('keeps filtering separate from run controls', async () => {
    const wrapper = mount(WorkflowFilterPopover, {
      props: { repo: { id: 1 } as Repo, modelValue: [] },
      global: {
        stubs: {
          Popover: { template: '<div><slot /></div>' },
          PopoverTrigger: { template: '<div><slot /></div>' },
          PopoverContent: { template: '<div><slot /></div>' },
        },
      },
    });
    await flushPromises();
    expect(wrapper.find('button[aria-label]').exists()).toBe(false);
    await wrapper.get('[role="checkbox"]').trigger('click');
    expect(wrapper.emitted('update:modelValue')).toEqual([[['lint']]]);
    expect(createPipeline).not.toHaveBeenCalled();
  });

  it('disables repeat starts while pending and recovers after an error', async () => {
    let rejectRequest!: (error: Error) => void;
    createPipeline.mockImplementation(
      async () =>
        new Promise((_resolve, reject) => {
          rejectRequest = reject;
        }),
    );
    const wrapper = await setup();
    const button = wrapper.get('button[aria-label="repo.manual_pipeline.run_workflow"]');
    await button.trigger('click');
    await flushPromises();
    expect(button.attributes('disabled')).toBeDefined();
    rejectRequest(new Error('Required variable missing'));
    await flushPromises();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error', text: 'Required variable missing' }));
    expect(button.attributes('disabled')).toBeUndefined();
    expect(push).not.toHaveBeenCalled();
  });
});
