import { mount } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Pipeline, PipelineStep, PipelineWorkflow, Repo, RepoPermissions } from '~/lib/api/types';
import { StepType } from '~/lib/api/types';
import { ref } from 'vue';
import PipelineStepList from './PipelineStepList.vue';

const { getAgent, notify, loadPipeline } = vi.hoisted(() => ({
  getAgent: vi.fn(),
  notify: vi.fn(),
  loadPipeline: vi.fn(),
}));
vi.mock('~/compositions/useApiClient', () => ({ default: () => ({ getAgent }) }));
vi.mock('~/compositions/useNotifications', () => ({ default: () => ({ notify }) }));
vi.mock('~/store/pipelines', () => ({ usePipelineStore: () => ({ loadPipeline }) }));

function step(overrides: Partial<PipelineStep> & Pick<PipelineStep, 'id' | 'pid' | 'ppid' | 'name'>): PipelineStep {
  return {
    uuid: `uuid-${overrides.id}`,
    pipeline_id: 1,
    state: 'success',
    exit_code: 0,
    original_step_id: 0,
    attempt: 1,
    ...overrides,
  };
}

// Mirrors issue #1520: a single "build-base" workflow that failed on its first
// attempt and succeeded after a rerun, so both attempts exist side by side.
function pipelineWithTwoAttempts(): Pipeline {
  const attempt1: PipelineWorkflow = {
    id: 10,
    pipeline_id: 1,
    pid: 1,
    name: 'build-base',
    pipeline_name: 'build-base',
    state: 'failure',
    original_workflow_id: 0,
    attempt: 1,
    children: [
      step({ id: 100, pid: 2, ppid: 1, name: 'clone', type: StepType.Clone }),
      step({ id: 101, pid: 3, ppid: 1, name: 'build-ubuntu', state: 'failure', exit_code: 1 }),
    ],
  };
  const attempt2: PipelineWorkflow = {
    id: 11,
    pipeline_id: 1,
    pid: 4,
    name: 'build-base',
    pipeline_name: 'build-base',
    state: 'success',
    original_workflow_id: 10,
    attempt: 2,
    children: [
      step({ id: 102, pid: 5, ppid: 4, name: 'clone', type: StepType.Clone, original_step_id: 100, attempt: 2 }),
      step({ id: 103, pid: 6, ppid: 4, name: 'build-ubuntu', original_step_id: 101, attempt: 2 }),
    ],
  };

  return {
    id: 1,
    number: 1,
    parent: 0,
    event: 'push',
    status: 'success',
    created: 0,
    updated: 0,
    started: 0,
    finished: 0,
    deploy_to: '',
    commit: 'abcdef0123456789',
    branch: 'main',
    message: 'commit',
    timestamp: 0,
    ref: 'refs/heads/main',
    refspec: '',
    clone_url: '',
    title: '',
    sender: 'someone',
    author: 'someone',
    author_avatar: '',
    author_email: '',
    forge_url: '',
    reviewed_by: '',
    reviewed: 0,
    logs_deleted: 0,
    workflows: [attempt1, attempt2],
  } as Pipeline;
}

function mountList(selectedStepId: number) {
  return mount(PipelineStepList, {
    props: { pipeline: pipelineWithTwoAttempts(), selectedStepId },
    global: {
      provide: {
        repo: ref({ id: 1, forge_id: 1 } as Repo),
        'repo-permissions': ref({ pull: true, push: true, admin: false } as RepoPermissions),
      },
      stubs: {
        Icon: { template: '<span />' },
        UserAvatar: { template: '<span />' },
        PipelineStatusIcon: { template: '<span />' },
        PipelineStepDuration: { template: '<span />' },
        ScrollArea: { template: '<div><slot /></div>' },
        TooltipProvider: { template: '<div><slot /></div>' },
        Tooltip: { template: '<div><slot /></div>' },
        TooltipTrigger: { template: '<div><slot /></div>' },
        TooltipContent: { template: '<div />' },
        DropdownMenu: { template: '<div><slot /></div>' },
        DropdownMenuTrigger: { template: '<div data-test="attempt-trigger"><slot /></div>' },
        DropdownMenuContent: { template: '<div><slot /></div>' },
        DropdownMenuItem: { template: '<div data-test="attempt-item"><slot /></div>' },
      },
    },
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  getAgent.mockResolvedValue({ id: 1, name: 'agent-1' });
});

describe('workflow attempt switching (issue #1520)', () => {
  it('shows the picked attempt after selecting a previous run', async () => {
    // The latest attempt (#2) is shown initially, with its build-ubuntu step selected.
    const wrapper = mountList(6);
    expect(wrapper.get('[data-test="attempt-trigger"]').text()).toContain('#2');

    // Open the workflow attempt dropdown and pick the first (failed) run.
    const items = wrapper.findAll('[data-test="attempt-item"]');
    const firstRun = items.find((item) => item.text().includes('#1'));
    expect(firstRun).toBeDefined();
    await firstRun!.trigger('click');

    // The parent routes the emitted step id into the URL and passes it back down.
    const emitted = wrapper.emitted('update:selected-step-id');
    expect(emitted).toBeTruthy();
    const newStepId = emitted!.at(-1)![0] as number;
    expect(newStepId).toBe(3); // build-ubuntu of attempt #1: same step, previous run
    await wrapper.setProps({ selectedStepId: newStepId });

    // The step list must now show attempt #1 and highlight the selected step.
    expect(wrapper.get('[data-test="attempt-trigger"]').text()).toContain('#1');
    expect(wrapper.findAll('[data-test="attempt-trigger"]').map((t) => t.text())).toEqual(['#1', '#1', '#1']);
    expect(wrapper.findAll('.bg-accent').length).toBeGreaterThan(0);
  });

  it('keeps showing the latest attempt while a step of it is selected', async () => {
    const wrapper = mountList(5); // clone of attempt #2
    expect(wrapper.findAll('[data-test="attempt-trigger"]').map((t) => t.text())).toEqual(['#2', '#2', '#2']);
    expect(wrapper.findAll('.bg-accent').length).toBeGreaterThan(0);
  });

  it('deep-links into a previous run when the route points at one of its steps', async () => {
    const wrapper = mountList(2); // clone of attempt #1
    expect(wrapper.get('[data-test="attempt-trigger"]').text()).toContain('#1');
    expect(wrapper.findAll('.bg-accent').length).toBeGreaterThan(0);
  });
});
