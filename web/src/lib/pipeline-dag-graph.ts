import { Position } from '@vue-flow/core';
import type { Edge, Node } from '@vue-flow/core';
import type { PipelineStatus, PipelineStep, PipelineWorkflow } from '~/lib/api/types';
import { identifyMatrixGroupsMap } from '~/lib/pipeline-utils';
import { isFailingStep, stepSkipReasons } from '~/lib/step-skip-reason';
import type { SkipReason } from '~/lib/step-skip-reason';

export const STEP_NODE_TYPE = 'pipeline-step';
export const GROUP_NODE_TYPE = 'workflow-group';

export interface StepNodeData {
  step: PipelineStep;
  workflow: PipelineWorkflow;
  // The id of the group node the step is drawn in.
  groupId: string;
  skipReason?: SkipReason;
}

/**
 * A box around the steps of one workflow, or of all workflows of a matrix, which
 * share the box and the name of the pipeline they were expanded from.
 */
export interface GroupNodeData {
  // The workflow shown in the header: the workflow itself, or for a matrix its
  // first workflow under the shared name and with the aggregated state.
  workflow: PipelineWorkflow;
  isMatrixGroup: boolean;
  matrixWorkflows: PipelineWorkflow[];
  width: number;
  height: number;
}

export interface PipelineGraph {
  nodes: Node[];
  edges: Edge[];
}

export function stepNodeId(step: PipelineStep): string {
  return `step-${step.pid}`;
}

export function groupNodeId(groupKey: string): string {
  return `workflow-group-${groupKey}`;
}

// Higher wins when the workflows of a matrix are summarized in one state.
const STATE_PRIORITY: Record<PipelineStatus, number> = {
  error: 7,
  failure: 6,
  killed: 5,
  running: 4,
  preparing: 4,
  started: 4,
  pending: 3,
  success: 2,
  skipped: 1,
  blocked: 0,
  declined: 0,
};

/** Summarizes the states of the workflows of a matrix, the most severe first. */
export function aggregateWorkflowState(workflows: PipelineWorkflow[]): PipelineStatus {
  if (workflows.length === 1) {
    return workflows[0].state;
  }
  let result: PipelineStatus = 'success';
  let highest = 0;
  for (const workflow of workflows) {
    const priority = STATE_PRIORITY[workflow.state] ?? 0;
    if (priority > highest) {
      highest = priority;
      result = workflow.state;
    }
  }
  return result;
}

function groupData(groupKey: string, workflows: PipelineWorkflow[], representative: PipelineWorkflow): GroupNodeData {
  return {
    workflow: { ...representative, name: groupKey, state: aggregateWorkflowState(workflows) },
    isMatrixGroup: workflows.length > 1,
    matrixWorkflows: workflows,
    width: 0,
    height: 0,
  };
}

/**
 * Builds the nodes and edges of the pipeline graph: a node per step, a group
 * node per workflow or matrix, edges between the steps of a workflow and edges
 * between the groups of dependent workflows. Every node starts at the origin;
 * the layout positions them once their sizes are known.
 */
export function buildPipelineGraph(workflows: PipelineWorkflow[], pipelineStatus: PipelineStatus): PipelineGraph {
  const matrixGroups = identifyMatrixGroupsMap(workflows);
  const groupKeyOfWorkflow = new Map<string, string>();
  for (const [groupKey, groupWorkflows] of matrixGroups) {
    for (const workflow of groupWorkflows) {
      groupKeyOfWorkflow.set(workflow.name, groupKey);
    }
  }

  const groupNodes: Node[] = [];
  const stepNodes: Node[] = [];
  const edges: Edge[] = [];

  for (const [groupKey, groupWorkflows] of matrixGroups) {
    const withSteps = groupWorkflows.filter((workflow) => (workflow.children ?? []).length > 0);
    if (withSteps.length === 0) {
      continue;
    }
    const groupId = groupNodeId(groupKey);
    groupNodes.push({
      id: groupId,
      type: GROUP_NODE_TYPE,
      position: { x: 0, y: 0 },
      data: groupData(groupKey, groupWorkflows, withSteps[0]) satisfies GroupNodeData,
      zIndex: 100,
      selectable: false,
      draggable: false,
    });

    for (const workflow of withSteps) {
      const skipReasons = stepSkipReasons(workflow, pipelineStatus);
      for (const step of workflow.children) {
        stepNodes.push({
          id: stepNodeId(step),
          type: STEP_NODE_TYPE,
          position: { x: 0, y: 0 },
          data: { step, workflow, groupId, skipReason: skipReasons.get(step.pid) } satisfies StepNodeData,
          sourcePosition: Position.Right,
          targetPosition: Position.Left,
          // Above the group boxes and the edges.
          zIndex: 2000,
        });
      }
      edges.push(...createStepDependencyEdges(workflow, skipReasons));
    }
  }

  const groupIds = new Set(groupNodes.map((node) => node.id));
  edges.push(...createWorkflowDependencyEdges(workflows, groupKeyOfWorkflow, groupIds));

  return { nodes: [...groupNodes, ...stepNodes], edges };
}

/**
 * Connects each step to the steps it depends on. The compiler resolves
 * depends_on within a workflow, so a dependency is only looked up among the
 * steps of the same workflow. A name that is not there, such as a step that a
 * rerun left out, gets no edge rather than one to a same-named step of another
 * workflow. The edges a failure travelled along to skip its dependents are red.
 */
function createStepDependencyEdges(workflow: PipelineWorkflow, skipReasons: Map<number, SkipReason>): Edge[] {
  const edges: Edge[] = [];
  const stepsByName = new Map(workflow.children.map((step) => [step.name, step]));
  const skippedByFailure = (step: PipelineStep) => skipReasons.get(step.pid)?.kind === 'dependency_failed';

  for (const step of workflow.children) {
    const target = stepNodeId(step);
    for (const dependency of step.depends_on ?? []) {
      const dependencyStep = stepsByName.get(dependency);
      if (!dependencyStep) {
        continue;
      }
      const source = stepNodeId(dependencyStep);
      // A failure travels along an edge when it skips the target and comes from
      // a failed step or from a step that the same failure skipped.
      const failurePath = skippedByFailure(step) && (isFailingStep(dependencyStep) || skippedByFailure(dependencyStep));
      edges.push({
        id: `edge-${source}-${target}`,
        source,
        target,
        sourceHandle: `${source}-source`,
        targetHandle: `${target}-target`,
        type: 'smoothstep',
        animated: step.state === 'running',
        // red-600, matching the border of a failed step
        style: failurePath ? { stroke: '#dc2626', strokeDasharray: '6,4' } : undefined,
        // Vue Flow's marker types do not know the custom platform markers.
        // eslint-disable-next-line ts/no-unsafe-assignment
        markerEnd: {
          type: 'platform',
          color: failurePath ? '#dc2626' : '#6b7280',
        } as any,
      });
    }
  }
  return edges;
}

/**
 * Connects the group of each workflow to the groups of the workflows it depends
 * on. A dependency names a workflow or the shared name of a matrix; the workflows
 * of a matrix that depend on the same group share one edge.
 */
function createWorkflowDependencyEdges(
  workflows: PipelineWorkflow[],
  groupKeyOfWorkflow: Map<string, string>,
  groupIds: Set<string>,
): Edge[] {
  const edges = new Map<string, Edge>();
  for (const workflow of workflows) {
    const target = groupNodeId(groupKeyOfWorkflow.get(workflow.name) ?? workflow.name);
    for (const dependency of workflow.depends_on ?? []) {
      const source = groupNodeId(groupKeyOfWorkflow.get(dependency) ?? dependency);
      const id = `workflow-dependency-${source}-${target}`;
      if (source === target || !groupIds.has(source) || !groupIds.has(target) || edges.has(id)) {
        continue;
      }
      edges.set(id, {
        id,
        source,
        target,
        sourceHandle: `${source}-source`,
        targetHandle: `${target}-target`,
        type: 'smoothstep',
        animated: false,
        style: {
          strokeDasharray: '8,8',
          strokeWidth: 3,
          // gray-500 for a consistent appearance across themes
          stroke: '#6b7280',
          strokeOpacity: 0.8,
        },
        // Vue Flow's marker types do not know the custom platform markers.
        // eslint-disable-next-line ts/no-unsafe-assignment
        markerEnd: {
          type: 'platform-large',
          color: '#6b7280',
        } as any,
        zIndex: 5,
      });
    }
  }
  return [...edges.values()];
}
