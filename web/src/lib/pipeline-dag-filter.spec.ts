import type { PipelineStatus, PipelineStep, PipelineWorkflow } from '~/lib/api/types';
import { describe, expect, it } from 'vitest';
import { filterPipelineGraph } from './pipeline-dag-filter';
import { buildPipelineGraph } from './pipeline-dag-graph';

function step(pid: number, name: string, state: PipelineStatus): PipelineStep {
  return {
    id: pid,
    uuid: `step-${pid}`,
    pipeline_id: 1,
    pid,
    ppid: 1,
    name,
    state,
    exit_code: 0,
    original_step_id: 0,
    attempt: 1,
  };
}

function workflow(pid: number, name: string, children: PipelineStep[], environ?: Record<string, string>) {
  return {
    id: pid,
    pipeline_id: 1,
    pid,
    name,
    pipeline_name: name,
    state: 'success',
    environ,
    original_workflow_id: 0,
    attempt: 1,
    children,
  } satisfies PipelineWorkflow;
}

const { nodes } = buildPipelineGraph(
  [
    workflow(1, 'build', [step(11, 'clone', 'success'), step(12, 'compile', 'failure')]),
    workflow(2, 'lint', [step(21, 'eslint', 'success')], { NODE: '24' }),
  ],
  'failure',
);

function visible(query: string, state = 'all', workflowPid = 'all'): string[] {
  return filterPipelineGraph(nodes, { query, state, workflow: workflowPid }).map((node) => node.id);
}

describe('filterPipelineGraph', () => {
  it('keeps every node without a filter', () => {
    expect(visible('')).toHaveLength(nodes.length);
  });

  it('shows a matching step together with its group', () => {
    expect(visible('COMPILE')).toEqual(['workflow-group-build', 'step-12']);
  });

  it('shows all steps of a workflow whose name or matrix values match', () => {
    expect(visible('build')).toEqual(['workflow-group-build', 'step-11', 'step-12']);
    expect(visible('node=24')).toEqual(['workflow-group-lint', 'step-21']);
  });

  it('hides a group none of whose steps pass the filter', () => {
    expect(visible('', 'failure')).toEqual(['workflow-group-build', 'step-12']);
    expect(visible('', 'all', '2')).toEqual(['workflow-group-lint', 'step-21']);
  });
});
