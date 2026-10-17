import type { PipelineStatus, PipelineStep, PipelineWorkflow } from '~/lib/api/types';
import { describe, expect, it } from 'vitest';
import { aggregateWorkflowState, buildPipelineGraph, GROUP_NODE_TYPE } from './pipeline-dag-graph';
import type { GroupNodeData, StepNodeData } from './pipeline-dag-graph';

function step(pid: number, ppid: number, name: string, state: PipelineStatus = 'success', dependsOn: string[] = []) {
  return {
    id: pid,
    uuid: `step-${pid}`,
    pipeline_id: 1,
    pid,
    ppid,
    name,
    state,
    exit_code: 0,
    depends_on: dependsOn,
    original_step_id: 0,
    attempt: 1,
  } satisfies PipelineStep;
}

function workflow(
  pid: number,
  name: string,
  children: PipelineStep[],
  extra: Partial<PipelineWorkflow> = {},
): PipelineWorkflow {
  return {
    id: pid,
    pipeline_id: 1,
    pid,
    name,
    pipeline_name: name,
    state: 'success',
    original_workflow_id: 0,
    attempt: 1,
    children,
    ...extra,
  };
}

describe('buildPipelineGraph', () => {
  // A matrix `test` of two workflows, both depending on `build`.
  const workflows = [
    workflow(1, 'build', [step(11, 1, 'clone'), step(12, 1, 'compile', 'success', ['clone'])]),
    workflow(2, 'test-1', [step(21, 2, 'unit', 'failure')], {
      pipeline_name: 'test',
      depends_on: ['build'],
      state: 'failure',
    }),
    workflow(3, 'test-2', [step(31, 3, 'unit')], { pipeline_name: 'test', depends_on: ['build'] }),
    workflow(4, 'empty', []),
  ];
  const graph = buildPipelineGraph(workflows, 'failure');
  const groups = graph.nodes.filter((node) => node.type === GROUP_NODE_TYPE);

  it('draws one group per workflow or matrix that has steps', () => {
    expect(groups.map((node) => node.id)).toEqual(['workflow-group-build', 'workflow-group-test']);
    const matrix = groups[1].data as GroupNodeData;
    expect(matrix.isMatrixGroup).toBe(true);
    expect(matrix.workflow.name).toBe('test');
    expect(matrix.workflow.state).toBe('failure');
  });

  it('puts the steps of every workflow of a matrix into its group', () => {
    const stepGroups = graph.nodes
      .filter((node) => node.type !== GROUP_NODE_TYPE)
      .map((node) => [node.id, (node.data as StepNodeData).groupId]);
    expect(stepGroups).toEqual([
      ['step-11', 'workflow-group-build'],
      ['step-12', 'workflow-group-build'],
      ['step-21', 'workflow-group-test'],
      ['step-31', 'workflow-group-test'],
    ]);
  });

  it('draws one edge per dependency between groups', () => {
    expect(graph.edges.map((edge) => `${edge.source}->${edge.target}`)).toEqual([
      'step-11->step-12',
      'workflow-group-build->workflow-group-test',
    ]);
  });
});

describe('aggregateWorkflowState', () => {
  it('reports the most severe state of a matrix', () => {
    const states = (...values: PipelineStatus[]) =>
      aggregateWorkflowState(values.map((state, i) => workflow(i, `w${i}`, [], { state })));
    expect(states('success', 'running', 'skipped')).toBe('running');
    expect(states('success', 'failure', 'error')).toBe('error');
    expect(states('skipped')).toBe('skipped');
  });
});
