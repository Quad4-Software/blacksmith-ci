import type { PipelineStatus, PipelineStep, PipelineWorkflow } from '~/lib/api/types';

/**
 * Why a step was skipped. The agent decides this while it schedules the
 * workflow but does not report it, so it is derived here from the states of the
 * step's dependencies, its workflow and its pipeline.
 */
export type SkipReason =
  // A step this one depends on, directly or transitively, failed.
  | { kind: 'dependency_failed'; dependency: string }
  // The pipeline or workflow was cancelled before the step ran.
  | { kind: 'cancelled' }
  // The whole workflow was skipped, e.g. by its `when` filter or a failed workflow dependency.
  | { kind: 'workflow_skipped' }
  // The workflow's approval gate was declined.
  | { kind: 'workflow_declined' }
  // The workflow stopped before the step ran, without any step failing.
  | { kind: 'workflow_stopped' }
  // Everything the step depends on succeeded, and it only runs on failure.
  | { kind: 'only_on_failure' };

/** Reports whether a step's failure fails its lane; `failure: ignore` steps do not. */
export function isFailingStep(step: PipelineStep): boolean {
  return (step.state === 'failure' || step.state === 'error') && step.failure !== 'ignore';
}

/**
 * Walks the dependencies of a step breadth-first and returns the nearest one
 * matching the predicate, so the reason names the step closest to the skip.
 */
function findAncestor(
  step: PipelineStep,
  stepsByName: Map<string, PipelineStep>,
  predicate: (dependency: PipelineStep) => boolean,
): PipelineStep | undefined {
  const seen = new Set<string>([step.name]);
  let queue = step.depends_on ?? [];
  while (queue.length > 0) {
    const next: string[] = [];
    for (const name of queue) {
      if (seen.has(name)) {
        continue;
      }
      seen.add(name);
      const dependency = stepsByName.get(name);
      if (!dependency) {
        continue;
      }
      if (predicate(dependency)) {
        return dependency;
      }
      next.push(...(dependency.depends_on ?? []));
    }
    queue = next;
  }
  return undefined;
}

/**
 * Derives the skip reason of every skipped step of a workflow, keyed by step pid.
 */
export function stepSkipReasons(workflow: PipelineWorkflow, pipelineStatus: PipelineStatus): Map<number, SkipReason> {
  const reasons = new Map<number, SkipReason>();
  const steps = workflow.children ?? [];
  const stepsByName = new Map(steps.map((step) => [step.name, step]));
  const cancelled = pipelineStatus === 'killed' || workflow.state === 'killed';
  const anyStepFailed = steps.some(isFailingStep);

  for (const step of steps) {
    if (step.state !== 'skipped') {
      continue;
    }

    const failed = findAncestor(step, stepsByName, isFailingStep);
    if (failed) {
      reasons.set(step.pid, { kind: 'dependency_failed', dependency: failed.name });
    } else if (cancelled || findAncestor(step, stepsByName, (dependency) => dependency.state === 'killed')) {
      reasons.set(step.pid, { kind: 'cancelled' });
    } else if (workflow.state === 'skipped') {
      reasons.set(step.pid, { kind: 'workflow_skipped' });
    } else if (workflow.state === 'declined') {
      reasons.set(step.pid, { kind: 'workflow_declined' });
    } else if ((workflow.state === 'failure' || workflow.state === 'error') && !anyStepFailed) {
      reasons.set(step.pid, { kind: 'workflow_stopped' });
    } else {
      reasons.set(step.pid, { kind: 'only_on_failure' });
    }
  }
  return reasons;
}
