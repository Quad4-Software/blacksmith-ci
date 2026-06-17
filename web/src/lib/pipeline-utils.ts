import type { PipelineStatus, PipelineStep, PipelineWorkflow } from '~/lib/api/types';

/**
 * Group workflows by their pipeline_name field to identify matrix groups.
 * Returns an ordered list of groups preserving the first-seen order.
 */
export interface WorkflowGroup {
  key: string;
  workflows: PipelineWorkflow[];
}

export function identifyMatrixGroups(workflows: PipelineWorkflow[]): WorkflowGroup[] {
  const sorted = topoSortWorkflows(workflows);
  const groupMap = new Map<string, PipelineWorkflow[]>();
  const order: string[] = [];

  for (const workflow of sorted) {
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
 * Topologically sort workflows so that dependencies appear before dependents.
 * Workflows without depends_on come first, preserving original order as tiebreaker.
 */
function topoSortWorkflows(workflows: PipelineWorkflow[]): PipelineWorkflow[] {
  // Build a map from workflow name to its original index for stable ordering
  const nameToIndex = new Map<string, number>();
  for (let i = 0; i < workflows.length; i++) {
    const name = workflows[i].pipeline_name || workflows[i].name;
    if (!nameToIndex.has(name)) {
      nameToIndex.set(name, i);
    }
  }

  // Compute in-degree per workflow name (how many deps it has that exist in this set)
  const names = new Set(nameToIndex.keys());
  const inDegree = new Map<string, number>();
  const dependents = new Map<string, string[]>(); // dep -> list of names that depend on it

  for (const name of names) {
    inDegree.set(name, 0);
  }

  for (const wf of workflows) {
    const name = wf.pipeline_name || wf.name;
    if (!wf.depends_on) continue;
    for (const dep of wf.depends_on) {
      if (names.has(dep) && dep !== name) {
        inDegree.set(name, (inDegree.get(name) ?? 0) + 1);
        if (!dependents.has(dep)) {
          dependents.set(dep, []);
        }
        if (!dependents.get(dep)!.includes(name)) {
          dependents.get(dep)!.push(name);
        }
      }
    }
  }

  // Kahn's algorithm with stable ordering (use original index as tiebreaker)
  const queue = [...names]
    .filter((n) => (inDegree.get(n) ?? 0) === 0)
    .sort((a, b) => (nameToIndex.get(a) ?? 0) - (nameToIndex.get(b) ?? 0));
  const sortedNames: string[] = [];

  while (queue.length > 0) {
    const name = queue.shift()!;
    sortedNames.push(name);

    for (const dep of dependents.get(name) ?? []) {
      const newDeg = (inDegree.get(dep) ?? 1) - 1;
      inDegree.set(dep, newDeg);
      if (newDeg === 0) {
        // Insert in sorted position by original index for stability
        const idx = nameToIndex.get(dep) ?? 0;
        let insertAt = queue.length;
        for (let i = 0; i < queue.length; i++) {
          if ((nameToIndex.get(queue[i]) ?? 0) > idx) {
            insertAt = i;
            break;
          }
        }
        queue.splice(insertAt, 0, dep);
      }
    }
  }

  // Re-order workflows by sorted name order
  const nameOrder = new Map<string, number>();
  for (let i = 0; i < sortedNames.length; i++) {
    nameOrder.set(sortedNames[i], i);
  }

  return [...workflows].sort((a, b) => {
    const nameA = a.pipeline_name || a.name;
    const nameB = b.pipeline_name || b.name;
    const orderDiff = (nameOrder.get(nameA) ?? 0) - (nameOrder.get(nameB) ?? 0);
    if (orderDiff !== 0) return orderDiff;
    // Within the same workflow group, preserve original order
    return workflows.indexOf(a) - workflows.indexOf(b);
  });
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

  return [...latest.values()];
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

/**
 * Get all attempts for a given step across all workflow attempts for the same variant.
 * Returns steps with matching name, sorted by attempt number.
 */
export function getStepAttempts(allWorkflows: PipelineWorkflow[], step: PipelineStep): PipelineStep[] {
  // Find the parent workflow of this step
  const parentWorkflow = allWorkflows.find((wf) => wf.children.some((s) => s.id === step.id));
  if (!parentWorkflow) return [step];

  // Get all workflow attempts for this variant
  const workflowAttempts = getWorkflowAttempts(allWorkflows, parentWorkflow);

  // Collect all steps with the same name across all workflow attempts
  const attempts: PipelineStep[] = [];
  for (const wf of workflowAttempts) {
    for (const s of wf.children) {
      if (s.name === step.name) {
        attempts.push(s);
      }
    }
  }

  return attempts.toSorted((a, b) => (a.attempt ?? 1) - (b.attempt ?? 1));
}

/**
 * Check if a step has reruns (more than one attempt exists across workflow attempts).
 */
export function hasStepReruns(allWorkflows: PipelineWorkflow[], step: PipelineStep): boolean {
  return getStepAttempts(allWorkflows, step).length > 1;
}

/**
 * For steps whose name occurs more than once within the same workflow, return a
 * 1-based occurrence index keyed by step pid. Steps with a unique name are omitted.
 * Lets the UI append a disambiguating suffix (e.g. "deploy (2)") so identically
 * named steps can be told apart.
 */
export function duplicateStepNameIndexes(steps: PipelineStep[]): Map<number, number> {
  const totals = new Map<string, number>();
  for (const s of steps) {
    totals.set(s.name, (totals.get(s.name) ?? 0) + 1);
  }

  const seen = new Map<string, number>();
  const result = new Map<number, number>();
  for (const s of steps) {
    if ((totals.get(s.name) ?? 0) > 1) {
      const index = (seen.get(s.name) ?? 0) + 1;
      seen.set(s.name, index);
      result.set(s.pid, index);
    }
  }
  return result;
}

/**
 * A flattened tree node representing a step with its nesting metadata.
 * Used to render steps as a tree in the step list when depends_on forms a single-parent chain.
 */
export interface StepTreeNode {
  step: PipelineStep;
  /** Nesting depth (0 = root level) */
  depth: number;
  /** Whether this is the last child of its parent */
  isLast: boolean;
  /**
   * For each ancestor depth 0..depth-1, whether a vertical connector line should be drawn.
   * true = ancestor at that depth has more siblings below, so draw a vertical line.
   */
  connectorLines: boolean[];
  /** depends_on names with clone steps filtered out */
  effectiveDeps: string[];
}

/**
 * Build a tree structure from a flat list of steps based on depends_on relationships.
 *
 * - DAG root steps (steps with no depends_on, e.g. clone steps) are always at root level
 *   and don't cause indentation for their direct dependents.
 * - Steps with exactly one non-root depends_on are nested under their parent.
 * - Steps with zero or multiple non-root depends_on remain at root level (depth 0).
 */
export function buildStepTree(steps: PipelineStep[]): StepTreeNode[] {
  // Identify DAG root step names (steps with no depends_on).
  // These are entry points (clone steps, custom clone steps, etc.) that shouldn't cause nesting.
  const dagRootNames = new Set(steps.filter((s) => !s.depends_on || s.depends_on.length === 0).map((s) => s.name));

  const visible = steps;

  // Build a name -> step index for visible steps
  const nameIndex = new Map<string, number>();
  for (let i = 0; i < visible.length; i++) {
    nameIndex.set(visible[i].name, i);
  }

  // Determine effective dependencies (excluding DAG root steps)
  const effectiveDeps: string[][] = visible.map((s) => {
    if (!s.depends_on) return [];
    return s.depends_on.filter((dep) => !dagRootNames.has(dep));
  });

  // Build children map: parentIndex -> childIndices (only for single-dep steps)
  const children = new Map<number, number[]>();
  const nestedIndices = new Set<number>();

  for (let i = 0; i < visible.length; i++) {
    const deps = effectiveDeps[i];
    if (deps.length === 1) {
      const parentIdx = nameIndex.get(deps[0]);
      if (parentIdx !== undefined) {
        if (!children.has(parentIdx)) {
          children.set(parentIdx, []);
        }
        children.get(parentIdx)!.push(i);
        nestedIndices.add(i);
      }
    }
  }

  // DFS to flatten tree
  const result: StepTreeNode[] = [];

  function walk(idx: number, depth: number, connectorLines: boolean[], isLast: boolean) {
    result.push({
      step: visible[idx],
      depth,
      isLast,
      connectorLines: [...connectorLines],
      effectiveDeps: effectiveDeps[idx],
    });

    const kids = children.get(idx);
    if (kids) {
      for (let i = 0; i < kids.length; i++) {
        const childIsLast = i === kids.length - 1;
        // For the next level, carry forward whether the current node has more siblings below
        const nextLines = [...connectorLines, !isLast];
        walk(kids[i], depth + 1, nextLines, childIsLast);
      }
    }
  }

  // Root nodes: not nested under anyone
  const roots = visible.map((_, i) => i).filter((i) => !nestedIndices.has(i));

  for (let i = 0; i < roots.length; i++) {
    walk(roots[i], 0, [], i === roots.length - 1);
  }

  return result;
}
