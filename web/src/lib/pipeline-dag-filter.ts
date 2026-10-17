import type { Node } from '@vue-flow/core';
import type { PipelineWorkflow } from '~/lib/api/types';
import { GROUP_NODE_TYPE, STEP_NODE_TYPE } from '~/lib/pipeline-dag-graph';
import type { GroupNodeData, StepNodeData } from '~/lib/pipeline-dag-graph';

export interface PipelineGraphFilter {
  // Matched case-insensitively against step and workflow names and matrix values.
  query: string;
  // A step state, or 'all'.
  state: string;
  // A workflow pid, or 'all'.
  workflow: string;
}

export function isFilterActive(filter: PipelineGraphFilter): boolean {
  return filter.query !== '' || filter.state !== 'all' || filter.workflow !== 'all';
}

function workflowMatches(workflow: PipelineWorkflow, query: string): boolean {
  return (
    workflow.name.toLowerCase().includes(query) ||
    Object.entries(workflow.environ ?? {}).some(([key, value]) => `${key}=${value}`.toLowerCase().includes(query))
  );
}

/**
 * Returns the nodes that pass the filter. A step passes when the query matches
 * the step, its workflow or its group, and its state and workflow are the
 * selected ones. A group is shown while any of its steps is.
 */
export function filterPipelineGraph(nodes: Node[], filter: PipelineGraphFilter): Node[] {
  if (!isFilterActive(filter)) {
    return nodes;
  }
  const query = filter.query.toLowerCase();
  const groupNames = new Map<string, string>();
  for (const node of nodes) {
    if (node.type === GROUP_NODE_TYPE) {
      groupNames.set(node.id, (node.data as GroupNodeData).workflow.name.toLowerCase());
    }
  }

  const visibleGroups = new Set<string>();
  const visibleSteps = new Set<string>();
  for (const node of nodes) {
    if (node.type !== STEP_NODE_TYPE) {
      continue;
    }
    const { step, workflow, groupId } = node.data as StepNodeData;
    const matchesQuery =
      query === '' ||
      step.name.toLowerCase().includes(query) ||
      workflowMatches(workflow, query) ||
      (groupNames.get(groupId)?.includes(query) ?? false);
    const matchesState = filter.state === 'all' || step.state === filter.state;
    const matchesWorkflow = filter.workflow === 'all' || workflow.pid === Number.parseInt(filter.workflow, 10);
    if (matchesQuery && matchesState && matchesWorkflow) {
      visibleSteps.add(node.id);
      visibleGroups.add(groupId);
    }
  }

  return nodes.filter((node) => visibleSteps.has(node.id) || visibleGroups.has(node.id));
}
