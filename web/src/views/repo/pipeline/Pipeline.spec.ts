import { mount } from '@vue/test-utils';
import { computed, ref } from 'vue';
import { describe, expect, it, vi } from 'vitest';
import type { Pipeline as PipelineData, PipelineWorkflow } from '~/lib/api/types';
import Pipeline from './Pipeline.vue';

vi.mock('~/compositions/useApiClient', () => ({ default: () => ({}) }));
vi.mock('~/compositions/useNotifications', () => ({ default: () => ({ notify: vi.fn() }) }));
vi.mock('~/compositions/useCrowTitle', () => ({ useCrowTitle: vi.fn() }));
vi.mock('~/store/pipelines', () => ({ usePipelineStore: () => ({}) }));
vi.mock('~/compositions/usePipelineNavigation', () => ({
  usePipelineNavigation: ({ stepIdProp }: { stepIdProp: { value: string } }) => ({
    selectedStepId: computed(() => Number(stepIdProp.value)),
    previousStepId: ref(null),
    nextStepId: ref(null),
    autoFollowEnabled: ref(false),
    handleAutoFollowUpdate: vi.fn(),
  }),
}));

function workflow(id: number, state: PipelineWorkflow['state'], gated = false): PipelineWorkflow {
  return {
    id,
    pid: id,
    pipeline_id: 1,
    name: `workflow-${id}`,
    pipeline_name: `workflow-${id}`,
    state,
    original_workflow_id: 0,
    attempt: 1,
    approval_state: gated ? (state === 'declined' ? 'declined' : 'pending') : '',
    children: [{ id: id * 10, pid: id * 10, name: 'step', state: state === 'declined' ? 'skipped' : state }],
  } as PipelineWorkflow;
}

function mountPipeline(status: PipelineData['status'], workflows: PipelineWorkflow[]) {
  return mount(Pipeline, {
    shallow: true,
    props: { stepId: '10' },
    global: {
      renderStubDefaultSlot: true,
      provide: {
        pipeline: ref({ id: 1, number: 1, status, workflows }),
        repo: ref({ id: 1, full_name: 'owner/repo' }),
        'repo-permissions': ref({ push: true }),
      },
    },
  });
}

describe('workflow approval display', () => {
  it.each(['blocked', 'declined'] as const)('keeps completed logs mounted when a sibling is %s', async (status) => {
    const wrapper = mountPipeline(status, [workflow(1, 'success'), workflow(2, status, true)]);
    expect(wrapper.findComponent({ name: 'PipelineLog' }).exists()).toBe(true);
    expect(wrapper.findComponent({ name: 'PipelineLogMobile' }).exists()).toBe(true);
    expect(wrapper.text()).not.toContain('repo.pipeline.protected.awaits');
    expect(wrapper.text()).not.toContain('repo.pipeline.protected.declined');

    await wrapper.setProps({ stepId: '20' });
    expect(wrapper.findComponent({ name: 'WorkflowApprovalPanel' }).exists()).toBe(true);
    // Mobile step navigation stays mounted even on a declined workflow.
    expect(wrapper.findComponent({ name: 'PipelineLogMobile' }).exists()).toBe(true);
    wrapper.unmount();
  });

  it('retains approval controls for a pipeline-level block', () => {
    const wrapper = mountPipeline('blocked', [workflow(1, 'blocked')]);
    expect(wrapper.text()).toContain('repo.pipeline.protected.awaits');
    expect(wrapper.findComponent({ name: 'PipelineLog' }).exists()).toBe(false);
    wrapper.unmount();
  });
});
