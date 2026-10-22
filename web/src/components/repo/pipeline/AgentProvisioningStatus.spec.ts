import { mount } from '@vue/test-utils';
import type { PipelineWorkflow } from '~/lib/api/types';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import AgentProvisioningStatus from './AgentProvisioningStatus.vue';

const { loadPipeline } = vi.hoisted(() => ({ loadPipeline: vi.fn() }));
vi.mock('~/store/pipelines', () => ({ usePipelineStore: () => ({ loadPipeline }) }));

function workflow(overrides: Partial<PipelineWorkflow> = {}): PipelineWorkflow {
  const now = Math.floor(Date.now() / 1000);
  return {
    id: 1,
    pipeline_id: 1,
    pid: 1,
    name: 'windows',
    pipeline_name: 'windows',
    state: 'pending',
    original_workflow_id: 0,
    attempt: 1,
    children: [],
    autoscaler_provisioning: true,
    autoscaler_provisioning_since: now - 600,
    autoscaler_provision_timeout: 1800,
    provisioning_agents: [
      {
        id: 5,
        name: 'crowci-azure-hyperv-abcd',
        phases: [
          { phase: 'Installing Hyper-V', at: now - 500 },
          { phase: 'Building the base image', at: now - 100 },
        ],
      },
      { id: 6, name: 'crowci-azure-hyperv-efgh', phases: [] },
    ],
    ...overrides,
  };
}

function mountStatus(wf: PipelineWorkflow) {
  return mount(AgentProvisioningStatus, {
    props: { workflow: wf, repoId: 34, pipelineNumber: 11692 },
    global: { stubs: { Icon: { template: '<span />' } } },
  });
}

describe('agentProvisioningStatus', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    loadPipeline.mockReset();
    loadPipeline.mockResolvedValue(undefined);
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('shows elapsed and expected time and the phases of agents that reported any', () => {
    const wrapper = mountStatus(workflow());
    expect(wrapper.find('[data-testid="provisioning-elapsed"]').exists()).toBe(true);
    const agents = wrapper.findAll('[data-testid="provisioning-agent"]');
    expect(agents).toHaveLength(1);
    expect(agents[0].text()).toContain('crowci-azure-hyperv-abcd');
    expect(agents[0].text()).toContain('Installing Hyper-V');
    expect(agents[0].text()).toContain('Building the base image');
  });

  it('works without progress from older autoscalers', () => {
    const wrapper = mountStatus(workflow({ autoscaler_provisioning_since: undefined, provisioning_agents: undefined }));
    expect(wrapper.find('[data-testid="provisioning-elapsed"]').exists()).toBe(false);
    expect(wrapper.findAll('[data-testid="provisioning-agent"]')).toHaveLength(0);
  });

  it('reloads the pipeline while shown and stops once removed', () => {
    const wrapper = mountStatus(workflow());
    vi.advanceTimersByTime(15_000);
    expect(loadPipeline).toHaveBeenCalledWith(34, 11692);
    wrapper.unmount();
    vi.advanceTimersByTime(60_000);
    expect(loadPipeline).toHaveBeenCalledTimes(1);
  });
});
