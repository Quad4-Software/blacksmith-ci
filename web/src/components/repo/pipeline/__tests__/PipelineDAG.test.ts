// Copyright 2025 Crow authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import PipelineDAG from '../PipelineDAG.vue';
import type { Pipeline, PipelineStatus } from '~/lib/api/types';
import { WebhookEvents } from '~/lib/api/types';

// Mock the vue-flow components
vi.mock('@vue-flow/core', () => ({
  VueFlow: {
    name: 'VueFlow',
    template: '<div><slot /></div>',
    props: ['nodes', 'edges'],
    emits: ['nodes-initialized', 'node-click'],
  },
  Handle: {
    name: 'Handle',
    template: '<div />',
  },
  Position: {
    Left: 'left',
    Right: 'right',
  },
  useVueFlow: vi.fn(() => ({
    fitView: vi.fn(),
    findNode: vi.fn(),
  })),
}));

vi.mock('@vue-flow/background', () => ({
  Background: {
    name: 'Background',
    template: '<div />',
  },
}));

vi.mock('@vue-flow/controls', () => ({
  Controls: {
    name: 'Controls',
    template: '<div />',
  },
}));

vi.mock('~/compositions/useLayout', () => ({
  useLayout: () => ({
    layout: vi.fn((nodes: any[]) => nodes),
  }),
}));

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}));

describe('pipelineDAG', () => {
  it('should handle matrix workflow depends_on connections correctly', async () => {
    // Create mock pipeline data with matrix workflows
    const mockPipeline: Pipeline = {
      id: 1,
      number: 1,
      parent: 0,
      event: WebhookEvents.Push,
      status: 'success',
      created: Date.now(),
      updated: Date.now(),
      started: Date.now(),
      finished: Date.now(),
      deploy_to: '',
      commit: 'abc123',
      branch: 'main',
      message: 'Test commit',
      timestamp: Date.now(),
      ref: 'refs/heads/main',
      refspec: '',
      clone_url: 'https://github.com/test/repo.git',
      title: 'Test Pipeline',
      sender: 'test-user',
      author: 'test-user',
      author_avatar: '',
      author_email: 'test@example.com',
      forge_url: 'https://github.com/test/repo',
      reviewed_by: '',
      reviewed: 0,
      logs_deleted: 0,
      workflows: [
        // Matrix workflow variant 1
        {
          id: 1,
          pipeline_id: 1,
          pid: 1,
          name: 'test-go-1.19',
          pipeline_name: 'Test Pipeline 1',
          state: 'success',
          original_workflow_id: 0,
          attempt: 1,
          environ: { GO_VERSION: '1.19' },
          children: [
            {
              id: 1,
              uuid: 'uuid-1',
              pipeline_id: 1,
              pid: 1,
              ppid: 1,
              name: 'clone',
              state: 'success',
              exit_code: 0,
              depends_on: [],
              original_step_id: 0,
              attempt: 1,
            },
            {
              id: 2,
              uuid: 'uuid-2',
              pipeline_id: 1,
              pid: 2,
              ppid: 1,
              name: 'test',
              state: 'success',
              exit_code: 0,
              depends_on: ['clone'],
              original_step_id: 0,
              attempt: 1,
            },
          ],
        },
        // Matrix workflow variant 2
        {
          id: 2,
          pipeline_id: 1,
          pid: 2,
          name: 'test-go-1.20',
          pipeline_name: 'Test Pipeline 2',
          state: 'success',
          original_workflow_id: 0,
          attempt: 1,
          environ: { GO_VERSION: '1.20' },
          children: [
            {
              id: 3,
              uuid: 'uuid-3',
              pipeline_id: 1,
              pid: 3,
              ppid: 2,
              name: 'clone',
              state: 'success',
              exit_code: 0,
              depends_on: [],
              original_step_id: 0,
              attempt: 1,
            },
            {
              id: 4,
              uuid: 'uuid-4',
              pipeline_id: 1,
              pid: 4,
              ppid: 2,
              name: 'test',
              state: 'success',
              exit_code: 0,
              depends_on: ['clone'],
              original_step_id: 0,
              attempt: 1,
            },
          ],
        },
      ],
    };

    const wrapper = mount(PipelineDAG, {
      props: {
        pipeline: mockPipeline,
      },
      global: {
        provide: {
          pipeline: mockPipeline,
        },
      },
    });

    // Wait for component to initialize
    await wrapper.vm.$nextTick();

    // Access the internal nodes and edges (not on the public type, but exposed by vue-flow)
    // @ts-expect-error nodes/edges are internal shallowRefs not on ComponentPublicInstance
    const { nodes, edges } = wrapper.vm;

    // Verify that all nodes are created (includes workflow group nodes)
    expect(nodes).toHaveLength(6); // 2 workflows x 2 steps each + 2 workflow group nodes

    // Verify that each workflow has its own clone and test nodes
    const workflow1CloneNode = nodes.find((node: any) => node.data.step?.pid === 1 && node.data.step?.name === 'clone');
    const workflow1TestNode = nodes.find((node: any) => node.data.step?.pid === 2 && node.data.step?.name === 'test');
    const workflow2CloneNode = nodes.find((node: any) => node.data.step?.pid === 3 && node.data.step?.name === 'clone');
    const workflow2TestNode = nodes.find((node: any) => node.data.step?.pid === 4 && node.data.step?.name === 'test');

    expect(workflow1CloneNode).toBeDefined();
    expect(workflow1TestNode).toBeDefined();
    expect(workflow2CloneNode).toBeDefined();
    expect(workflow2TestNode).toBeDefined();

    // Verify that edges are created correctly within each workflow
    expect(edges).toHaveLength(2); // 2 workflows x 1 edge each (test depends on clone)

    // Verify workflow 1 edge: test depends on clone
    const workflow1Edge = edges.find(
      (edge: any) => edge.source === workflow1CloneNode?.id && edge.target === workflow1TestNode?.id,
    );
    expect(workflow1Edge).toBeDefined();

    // Verify workflow 2 edge: test depends on clone
    const workflow2Edge = edges.find(
      (edge: any) => edge.source === workflow2CloneNode?.id && edge.target === workflow2TestNode?.id,
    );
    expect(workflow2Edge).toBeDefined();

    // Verify that there are no cross-workflow edges
    // (workflow 1 test should not depend on workflow 2 clone)
    const crossWorkflowEdge1 = edges.find(
      (edge: any) => edge.source === workflow2CloneNode?.id && edge.target === workflow1TestNode?.id,
    );
    const crossWorkflowEdge2 = edges.find(
      (edge: any) => edge.source === workflow1CloneNode?.id && edge.target === workflow2TestNode?.id,
    );

    expect(crossWorkflowEdge1).toBeUndefined();
    expect(crossWorkflowEdge2).toBeUndefined();
  });

  it('should handle empty workflows gracefully', async () => {
    const mockPipeline: Pipeline = {
      id: 1,
      number: 1,
      parent: 0,
      event: WebhookEvents.Push,
      status: 'success',
      created: Date.now(),
      updated: Date.now(),
      started: Date.now(),
      finished: Date.now(),
      deploy_to: '',
      commit: 'abc123',
      branch: 'main',
      message: 'Test commit',
      timestamp: Date.now(),
      ref: 'refs/heads/main',
      refspec: '',
      clone_url: 'https://github.com/test/repo.git',
      title: 'Test Pipeline',
      sender: 'test-user',
      author: 'test-user',
      author_avatar: '',
      author_email: 'test@example.com',
      forge_url: 'https://github.com/test/repo',
      reviewed_by: '',
      reviewed: 0,
      logs_deleted: 0,
      workflows: [],
    };

    const wrapper = mount(PipelineDAG, {
      props: {
        pipeline: mockPipeline,
      },
      global: {
        provide: {
          pipeline: mockPipeline,
        },
      },
    });

    await wrapper.vm.$nextTick();

    // @ts-expect-error nodes/edges are internal shallowRefs not on ComponentPublicInstance
    const { nodes, edges } = wrapper.vm;

    expect(nodes).toHaveLength(0);
    expect(edges).toHaveLength(0);
  });

  it('should handle workflows with missing depends_on gracefully', async () => {
    const mockPipeline: Pipeline = {
      id: 1,
      number: 1,
      parent: 0,
      event: WebhookEvents.Push,
      status: 'success',
      created: Date.now(),
      updated: Date.now(),
      started: Date.now(),
      finished: Date.now(),
      deploy_to: '',
      commit: 'abc123',
      branch: 'main',
      message: 'Test commit',
      timestamp: Date.now(),
      ref: 'refs/heads/main',
      refspec: '',
      clone_url: 'https://github.com/test/repo.git',
      title: 'Test Pipeline',
      sender: 'test-user',
      author: 'test-user',
      author_avatar: '',
      author_email: 'test@example.com',
      forge_url: 'https://github.com/test/repo',
      reviewed_by: '',
      reviewed: 0,
      logs_deleted: 0,
      workflows: [
        {
          id: 1,
          pipeline_id: 1,
          pid: 1,
          name: 'test-workflow',
          pipeline_name: 'Test Pipeline',
          original_workflow_id: 0,
          attempt: 1,
          state: 'success',
          children: [
            {
              id: 1,
              uuid: 'uuid-1',
              pipeline_id: 1,
              pid: 1,
              ppid: 1,
              name: 'standalone-step',
              state: 'success',
              exit_code: 0,
              original_step_id: 0,
              attempt: 1,
              // No depends_on field
            },
          ],
        },
      ],
    };

    const wrapper = mount(PipelineDAG, {
      props: {
        pipeline: mockPipeline,
      },
      global: {
        provide: {
          pipeline: mockPipeline,
        },
      },
    });

    await wrapper.vm.$nextTick();

    // @ts-expect-error nodes/edges are internal shallowRefs not on ComponentPublicInstance
    const { nodes, edges } = wrapper.vm;

    expect(nodes).toHaveLength(2); // 1 step + 1 workflow group node
    expect(edges).toHaveLength(0); // No dependencies, so no edges
  });

  it('does not connect a step to a same-named step of another workflow', async () => {
    const step = (id: number, ppid: number, name: string, depends_on: string[], state: PipelineStatus = 'success') => ({
      id,
      uuid: `uuid-${id}`,
      pipeline_id: 1,
      pid: id,
      ppid,
      name,
      state,
      exit_code: 0,
      depends_on,
      original_step_id: 0,
      attempt: 1,
    });
    const workflow = (pid: number, name: string, children: ReturnType<typeof step>[]) => ({
      id: pid,
      pipeline_id: 1,
      pid,
      name,
      pipeline_name: name,
      state: 'success' as const,
      original_workflow_id: 0,
      attempt: 1,
      children,
    });
    // A step rerun keeps the target step but drops the steps it depends on, so
    // `publish` names a `compile` that only exists in the other workflow.
    const pipelineWith = (publishState: PipelineStatus): Pipeline =>
      ({
        id: 1,
        number: 1,
        status: 'running',
        workflows: [
          workflow(1, 'build', [step(1, 1, 'clone', []), step(2, 1, 'compile', ['clone'])]),
          workflow(2, 'release', [step(3, 2, 'publish', ['compile'], publishState)]),
        ],
      }) as unknown as Pipeline;

    const wrapper = mount(PipelineDAG, { props: { pipeline: pipelineWith('pending') } });
    await wrapper.vm.$nextTick();

    const edgeIds = () =>
      // @ts-expect-error edges is an internal shallowRef not on ComponentPublicInstance
      (wrapper.vm.edges as { source: string; target: string }[]).map((edge) => `${edge.source}->${edge.target}`);
    expect(edgeIds()).toEqual(['step-1->step-2']);

    // A state change takes the updateNodeStates path, which rebuilds the edges.
    await wrapper.setProps({ pipeline: pipelineWith('running') });
    await wrapper.vm.$nextTick();
    expect(edgeIds()).toEqual(['step-1->step-2']);
  });
});
