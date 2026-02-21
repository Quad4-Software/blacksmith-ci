import type { PipelineStatus, PipelineWorkflow } from '~/lib/api/types';

/**
 * Group workflows by their pipeline_name field to identify matrix groups.
 * Returns an ordered list of groups preserving the first-seen order.
 */
export interface WorkflowGroup {
  key: string;
  workflows: PipelineWorkflow[];
}

export function identifyMatrixGroups(workflows: PipelineWorkflow[]): WorkflowGroup[] {
  const groupMap = new Map<string, PipelineWorkflow[]>();
  const order: string[] = [];

  for (const workflow of workflows) {
    const key = workflow.pipeline_name || workflow.name;
    if (!groupMap.has(key)) {
      groupMap.set(key, []);
      order.push(key);
    }
    groupMap.get(key)!.push(workflow);
  }

  return order.map((key) => ({ key, workflows: groupMap.get(key)! }));
}

/**
 * Same grouping logic but returns a Map for backward compatibility with PipelineDAG.
 */
export function identifyMatrixGroupsMap(workflows: PipelineWorkflow[]): Map<string, PipelineWorkflow[]> {
  const matrixGroups = new Map<string, PipelineWorkflow[]>();

  for (const workflow of workflows) {
    const key = workflow.pipeline_name || workflow.name;
    if (!matrixGroups.has(key)) {
      matrixGroups.set(key, []);
    }
    matrixGroups.get(key)!.push(workflow);
  }

  return matrixGroups;
}

const STATUS_PRIORITY: PipelineStatus[] = [
  'error',
  'failure',
  'running',
  'started',
  'pending',
  'killed',
  'blocked',
  'declined',
  'success',
  'skipped',
];

/**
 * Compute the aggregate status for a set of workflows.
 * Returns the highest-priority (worst) status found.
 */
export function aggregateWorkflowStatus(workflows: PipelineWorkflow[]): PipelineStatus {
  for (const status of STATUS_PRIORITY) {
    if (workflows.some((w) => w.state === status)) return status;
  }
  return 'pending';
}

/**
 * Compute the maximum duration across workflows.
 * Returns a synthetic object with started/finished suitable for PipelineStepDuration.
 */
export function aggregateWorkflowDuration(workflows: PipelineWorkflow[]): { started?: number; finished?: number } {
  let earliest: number | undefined;
  let latest: number | undefined;

  for (const w of workflows) {
    if (w.started !== undefined) {
      if (earliest === undefined || w.started < earliest) earliest = w.started;
    }
    if (w.finished !== undefined) {
      if (latest === undefined || w.finished > latest) latest = w.finished;
    }
  }

  return { started: earliest, finished: latest };
}

/**
 * Compute a variant key for a workflow: name + sorted environ entries.
 * This uniquely identifies a matrix variant regardless of original_workflow_id values.
 */
function variantKey(wf: PipelineWorkflow): string {
  if (!wf.environ || Object.keys(wf.environ).length === 0) {
    return wf.name;
  }
  const envStr = Object.entries(wf.environ)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([k, v]) => `${k}=${v}`)
    .join(',');
  return `${wf.name}|${envStr}`;
}

/**
 * Get the latest attempt of each workflow, grouping by name+environ.
 * This correctly handles matrix variants even if original_workflow_id is corrupted.
 */
export function getLatestAttempts(workflows: PipelineWorkflow[]): PipelineWorkflow[] {
  const latest = new Map<string, PipelineWorkflow>();

  for (const wf of workflows) {
    const key = variantKey(wf);
    const existing = latest.get(key);
    if (existing == null || wf.attempt > existing.attempt) {
      latest.set(key, wf);
    }
  }

  return Array.from(latest.values());
}

/**
 * Get all attempts for a given workflow (matched by name+environ), sorted by attempt number.
 */
export function getWorkflowAttempts(workflows: PipelineWorkflow[], workflow: PipelineWorkflow): PipelineWorkflow[] {
  const targetKey = variantKey(workflow);
  return workflows.filter((wf) => variantKey(wf) === targetKey).sort((a, b) => a.attempt - b.attempt);
}

/**
 * Check if a workflow has reruns (more than one attempt exists).
 */
export function hasReruns(workflows: PipelineWorkflow[], workflow: PipelineWorkflow): boolean {
  return getWorkflowAttempts(workflows, workflow).length > 1;
}
